package referral

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"referral-system/env"
	"referral-system/src/contracts"

	"referral-system/src/utils"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/viper"
)

type App struct {
	Db              *sql.DB
	MarginTokenInfo []DbMarginTokenInfo
	PaymentExecutor PayExec
	Rpc             []string
	RpcClient       *ethclient.Client
	MultipayCtrct   *contracts.MultiPay
	RS              ReferralSystem
}

type ReferralSystem interface {
	ProcessPayments(app *App, rows *sql.Rows, scale map[uint32]float64, batchTs string)
	OpenPay(rows *sql.Rows, app *App, traderAddr string) (utils.APIResponseOpenEarnings, error)
	PreProcessPayments(rpc *ethclient.Client) error
	LoadConfig(fileName string, chainId int) error
	SetBrokerAddr(addr string)
	GetBrokerAddr() string
	GetCronSchedule() string
	GetMultiPayAddr() string
	GetPaymentLookBackDays() int
	SetDb(db *sql.DB)
	SettingsToDb() error
	GetType() string
}

type Rpc struct {
	ChainId int      `json:"chainId"`
	Rpc     []string `json:"HTTP"`
}

type DbReferralChainOfChild struct {
	Parent     string  `json:"parent"`
	Child      string  `json:"child"`
	PassOn     float64 `json:"passOnDec"` // rel. pass on (0.2 for 20%)
	Lvl        uint8   `json:"level"`
	ParentPay  float64 `json:"parentPayDec"`  // rel. fraction of total payment to parent
	ChildAvail float64 `json:"childAvailDec"` // rel. fraction of total payment that child can redistribute
}

type DbReferralCode struct {
	Code             string
	ReferrerAddr     string
	CreatedOn        time.Time
	Expiry           time.Time
	TraderRebatePerc float32
}

type DbMarginTokenInfo struct {
	PoolId        uint32
	TokenAddr     string
	TokenName     string
	TokenDecimals int8
}

type PaymentLog struct {
	BatchTimestamp int
	Code           string
	PoolId         uint32
	TokenAddr      string
	BrokerAddr     string
	TxHash         string
	BlockNumber    uint64
	BlockTs        uint64
	PayeeAddr      []common.Address
	AmountDecN     []*big.Int
}

type PaymentExecution struct {
	Code       string
	PoolId     uint32
	TokenAddr  string
	PayeeAddr  []common.Address
	AmountDecN []*big.Int
}

type DbPayment struct {
	TraderAddr   string
	PayeeAddr    string
	Code         string
	Level        int
	PoolId       int
	BatchTs      time.Time
	PaidAmountCC string
	TxHash       string
	BlockNr      uint64
	BlockTs      time.Time
	TxConfirmed  bool
}

// New intantiates a referral app
func (a *App) New(viper *viper.Viper) error {

	slog.Info("Loading RPC configuration")
	rpcs, err := loadRPCConfig(viper)
	if err != nil {
		return err
	}
	a.Rpc = rpcs

	r := viper.GetString(env.REFERRAL_SYS_TYPE)
	if r == "CODE_REFERRAL" {
		slog.Info("Code referral system")
		a.RS = &CodeSystem{}
	} else {
		slog.Info("Social referral system")
		tbearer := viper.GetString("TWITTER_AUTH_BEARER")
		a.RS = NewSocialSystem(tbearer)
	}
	// load config
	slog.Info("Loading configuration")
	fileName := viper.GetString(env.CONFIG_PATH)
	targetChain := viper.GetInt(env.CHAIN_ID)
	err = a.RS.LoadConfig(fileName, targetChain)
	if err != nil {
		return err
	}

	// connect db
	connStr := viper.GetString(env.DATABASE_DSN_HISTORY)
	err = a.ConnectDB(connStr)
	if err != nil {
		return err
	}

	// settings to database
	slog.Info("Writing settings to DB")
	err = a.SettingsToDB()
	if err != nil {
		return err
	}

	slog.Info("Create RPC Client")
	err = a.CreateRpcClient()
	if err != nil {
		return err
	}
	slog.Info("Create Multipay Instance")
	err = a.CreateMultipayInstance()
	if err != nil {
		return err
	}

	a.PaymentExecutor = &RemotePayExec{}
	slog.Info("Init PaymentExecutor")
	err = a.PaymentExecutor.Init(viper, a.RS.GetMultiPayAddr())
	if err != nil {
		return err
	}

	return nil
}

// loadRPCConfig loads the RPC list for the
// configured chain
func loadRPCConfig(v *viper.Viper) ([]string, error) {
	fileName := v.GetString(env.RPC_URL_PATH)
	chainId := v.GetInt(env.CHAIN_ID)
	var r []Rpc
	data, err := os.ReadFile(fileName)
	if err != nil {
		return []string{}, err
	}
	err = json.Unmarshal(data, &r)
	if err != nil {
		return []string{}, err
	}
	for k := 0; k < len(r); k++ {
		if r[k].ChainId == chainId {
			return r[k].Rpc, nil
		}
	}
	return []string{}, errors.New("No RPC for chainId " + strconv.Itoa(chainId))
}

// SettingsToDB stores relevant settings to the DB
func (a *App) SettingsToDB() error {
	query := `
	INSERT INTO referral_settings (property, value)
	VALUES ($1, $2)
	ON CONFLICT (property) DO UPDATE SET value = EXCLUDED.value`
	_, err := a.Db.Exec(query, "payment_max_lookback_days", a.RS.GetPaymentLookBackDays())

	if err != nil {
		return err
	}

	// broker address
	addr := a.PaymentExecutor.GetBrokerAddr().String()
	addr = strings.ToLower(addr)
	a.RS.SetBrokerAddr(addr)
	query = `
	INSERT INTO referral_settings (property, value)
	VALUES ($1, $2)
	ON CONFLICT (property) DO UPDATE SET value = EXCLUDED.value`
	_, err = a.Db.Exec(query, "broker_addr", addr)
	if err != nil {
		return err
	}
	// referral system specific settings
	return a.RS.SettingsToDb()
}

// ConnectDB connects to the database and assigns the connection to the app struct
func (a *App) ConnectDB(connStr string) error {
	// Connect to database
	// From documentation: "The returned DB is safe for concurrent use by multiple goroutines and
	// maintains its own pool of idle connections. Thus, the Open function should be called just once.
	// It is rarely necessary to close a DB."
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return err
	}
	a.Db = db
	return nil
}

// DbGetMarginTkn sets the margin token info in the app-struct
func (a *App) DbGetMarginTkn() error {
	if a.Db == nil {
		return errors.New("Db not initialized")
	}
	var row DbMarginTokenInfo
	query := "select pool_id, token_addr, token_name, token_decimals from margin_token_info;"
	rows, err := a.Db.Query(query)
	defer rows.Close()
	if err != nil {
		return err
	}
	a.MarginTokenInfo = nil
	for rows.Next() {
		rows.Scan(&row.PoolId, &row.TokenAddr, &row.TokenName, &row.TokenDecimals)
		a.MarginTokenInfo = append(a.MarginTokenInfo, row)
		fmt.Println(row)
	}
	return nil
}

// Queries payment transactions which have not been confirmed
// for the latest batch, and asks the RPC for the tx status
// On success we set the confirm flag in the db, on failure,
// we delete and add to failed payments, if the query ends
// without success, we will retry unless we are already
// 3/4 days after execution
func (a *App) ConfirmPaymentTxs() {
	txs, ts := a.DBGetUnconfirmedPayTxForLastBatch()
	var fail []string
	var success []string
	var doPurge bool = false
	var hasTxNotFound bool = false
	tsNow := time.Now().Unix()

	// Create a token bucket with a limit of 5 tokens and a refill rate of 3 tokens per second
	// to throttle the RPC queries
	bucket := TokenBucket{
		tokens:     5,
		capacity:   5,
		refillRate: 3,
		lastRefill: time.Now(),
	}

	for _, tx := range txs {
		bucket.WaitForToken("ConfirmPaymentTxs")
		status := QueryTxStatus(a.RpcClient, tx)
		if status == TxFailed {
			fail = append(fail, tx)
			continue
		}
		if status == TxConfirmed {
			success = append(success, tx)
			continue
		}
		hasTxNotFound = true
		// tx could not be found. Not on chain? RPC issues?
		// only delete 2/3 days after batch ts
		doPurge = tsNow-ts > 86400*2/3
	}

	a.DBSetPayTxsConfirmed(success)
	numUnknown := strconv.Itoa(len(txs) - len(success) - len(fail))
	slog.Info("Payment confirmation total : " + strconv.Itoa(len(txs)) + ", success: " + strconv.Itoa(len(success)) + ", fail: " + strconv.Itoa(len(fail)) + ", unknown :" + numUnknown)
	if doPurge {
		slog.Info("Deleting all unconfirmed payments")
		a.PurgeUnconfirmedPayments(nil)
	} else if len(fail) > 0 {
		slog.Info("Deleting failed payments")
		a.PurgeUnconfirmedPayments(fail)
	}
	if !doPurge && hasTxNotFound {
		slog.Info("Re-scheduling ConfirmPaymentTxs")
		// we have to try again
		time.Sleep(time.Hour)
		a.ConfirmPaymentTxs()
	} else {
		slog.Info("ConfirmPaymentTxs completed")
	}
}

// DBGetUnconfirmedPayTxForLastBatch assembles an array of tx-ids ([]string)
// for payments that have not been confirmed (in table referral_payment)
// returns the tx-id string array and the timestamp of the batch
func (a *App) DBGetUnconfirmedPayTxForLastBatch() ([]string, int64) {
	query := `SELECT distinct(tx_hash),
				rp.batch_ts
				FROM referral_payment rp 
				WHERE rp.tx_confirmed = false 
				AND batch_ts IN (
					SELECT max(rp2.batch_ts) as max_ts
					FROM referral_payment rp2 
				)`
	rows, err := a.Db.Query(query)
	defer rows.Close()
	if err != nil {
		slog.Error("Error in DBGetUnconfirmedPayTxForLastBatch:" + err.Error())
		return []string{}, 0
	}
	var unTxs []string
	var unixTimestamp int64
	for rows.Next() {
		var tx string
		var batchTimestamp time.Time
		rows.Scan(&tx, &batchTimestamp)
		unixTimestamp = batchTimestamp.Unix()
		unTxs = append(unTxs, tx)
	}
	return unTxs, unixTimestamp
}

// DBSetPayTxsConfirmed sets the confirmed flag in the DB table referral_payment for
// a list of transactions to true
func (a *App) DBSetPayTxsConfirmed(txHash []string) {
	query := `UPDATE referral_payment
			SET tx_confirmed = true
			WHERE tx_hash = $1;`
	for _, h := range txHash {
		_, err := a.Db.Exec(query, h)
		if err != nil {
			slog.Error("Failed to set tx confirmed for tx=" + h + " error: " + err.Error())
		} else {
			slog.Info("Setting tx " + h + " to confirmed")
		}
	}
}

// SavePayments gets the payment events from on-chain and
// updates or inserts database entries
func (a *App) SavePayments() error {
	tsStart := time.Now().Unix() - int64(a.RS.GetPaymentLookBackDays()*86400)
	lookBackBlock, _, err := contracts.FindBlockWithTs(a.RpcClient, uint64(tsStart))
	if err != nil {
		lookBackBlock = 0
	}
	payments, err := FilterPayments(a.MultipayCtrct, a.RpcClient, lookBackBlock, 0)
	if err != nil {
		return err
	}
	for _, p := range payments {
		// key = trader_addr, payee_addr, pool_id, batch_timestamp
		traderAddr := p.PayeeAddr[0].String()
		for k, payee := range p.PayeeAddr {
			result := p.AmountDecN[k].Cmp(big.NewInt(0))
			if result != 0 {
				a.writeDbPayment(traderAddr, payee.String(), p, k)
			}
		}

	}
	return nil
}

// PurgeUnconfirmedPayments deletes records from the database that could not be
// confirmed on-chain and are in txs list, adds them to failed payments table
// If txs is nil, all unconfirmed payments (tx_confirmed = false) will be deleted
func (a *App) PurgeUnconfirmedPayments(txs []string) error {
	query := `select trader_addr, payee_addr, 
				code, level, pool_id, batch_ts, paid_amount_cc, 
				tx_hash, block_ts
				from referral_payment rp 
			where tx_confirmed = false;`
	rows, err := a.Db.Query(query)
	defer rows.Close()
	if err != nil {
		return err
	}
	var row DbPayment
	var idx = 0
	for rows.Next() {
		inDeleteList := txs == nil
		rows.Scan(&row.TraderAddr, &row.PayeeAddr, &row.Code, &row.Level, &row.PoolId, &row.BatchTs, &row.PaidAmountCC,
			&row.TxHash, &row.BlockTs)
		for _, tx := range txs {
			if tx == row.TxHash {
				inDeleteList = true
				break
			}
		}
		if !inDeleteList {
			continue
		}
		slog.Info("Moving to failed payments tx hash = " + row.TxHash)
		query = `INSERT INTO referral_failed_payment
			(trader_addr, payee_addr, code, level, pool_id, batch_ts, paid_amount_cc, tx_hash, ts)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
		_, err := a.Db.Exec(query, row.TraderAddr, row.PayeeAddr, row.Code, row.Level, row.PoolId, row.BatchTs, row.PaidAmountCC,
			row.TxHash, row.BlockTs)
		idx++
		if err != nil {
			slog.Error("Could not insert tx to failed tx " + row.TxHash + ": " + err.Error())
		}
		query = `DELETE FROM referral_payment rp
			where tx_hash=$1`
		_, err = a.Db.Query(query, row.TxHash)
		if err != nil {
			slog.Error("Could not delete unconfirmed payments:" + err.Error())
		}
	}

	return nil
}

// DbWriteTx write info about the payment transaction into referral_payment
func (a *App) DbWriteTx(traderAddr string, code string, amounts []*big.Int, payees []common.Address, batchTs string, poolId uint32, tx string) {
	slog.Info("Inserting Payment TX in DB")
	t, _ := strconv.Atoi(batchTs)
	ts := time.Unix(int64(t), 0)
	query := `INSERT INTO referral_payment (trader_addr, payee_addr, code, level, pool_id, batch_ts, paid_amount_cc, tx_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	for k := 0; k < len(payees); k++ {
		if amounts[k].BitLen() == 0 {
			// we don't write 0 amounts to the database
			continue
		}
		_, err := a.Db.Exec(query, traderAddr, payees[k].String(), code, k, poolId, ts, amounts[k].String(), tx)
		if err != nil {
			slog.Error("Could not insert tx to db for trader " + traderAddr + ": " + err.Error())
		}
	}
}

// writeDbPayment writes data from multipay contract into the database
// if there is already an entry for a given record which is not confirmed, it sets the confirmed flag to true
// db keys are trader_addr, payee_addr, pool_id and batch_ts
func (a *App) writeDbPayment(traderAddr string, payeeAddr string, p PaymentLog, payIdx int) error {
	if a.Db == nil {
		return errors.New("Db not initialized")
	}
	utcBatchTime := time.Unix(int64(p.BatchTimestamp), 0)
	utcBlockTime := time.Unix(int64(p.BlockTs), 0)
	query := `SELECT tx_confirmed FROM referral_payment 
			  WHERE lower(trader_addr) = lower($1) 
			  	AND lower(payee_addr) = lower($2) 
				AND batch_ts = $3 
				AND pool_id=$4
				AND level=$5`
	var isConfirmed bool
	err := a.Db.QueryRow(query, traderAddr, payeeAddr, utcBatchTime, p.PoolId, payIdx).Scan(&isConfirmed)
	if err == sql.ErrNoRows {
		// insert
		query = `INSERT INTO referral_payment (trader_addr, payee_addr, code, level, pool_id, batch_ts, paid_amount_cc, tx_hash, block_nr, block_ts, tx_confirmed)
          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
		_, err := a.Db.Exec(query, traderAddr, payeeAddr, p.Code, payIdx, p.PoolId, utcBatchTime, p.AmountDecN[payIdx].String(), p.TxHash, p.BlockNumber, utcBlockTime, true)
		if err != nil {
			return errors.New("Failed to insert data: " + err.Error())
		}
	} else if err != nil {
		return err
	} else if !isConfirmed {
		// set tx confirmed to true
		query = `UPDATE referral_payment 
				SET tx_confirmed = true, block_nr = $4, block_ts = $5
				WHERE lower(trader_addr) = lower($1) 
					AND lower(payee_addr) = lower($2) 
					AND batch_ts = $3
					AND level = $4`
		_, err := a.Db.Exec(query, traderAddr, payeeAddr, utcBatchTime, payIdx, p.BlockNumber, utcBlockTime)
		if err != nil {
			return errors.New("Failed to insert data: " + err.Error())
		}
	}
	return nil
}

// HistoricEarnings calculates historic earnings of any participant
// (broker-payout address, agent, referrer, trader)
func (a *App) HistoricEarnings(addr string) ([]utils.APIResponseHistEarnings, error) {

	var history []utils.APIResponseHistEarnings
	query := `SELECT rp.pool_id, 
				CASE
					WHEN rp.trader_addr = rp.payee_addr THEN 1 -- True if trader_addr is equal to payee_addr
					ELSE 0 -- False otherwise
				END AS as_trader,
				rp.code, sum(paid_amount_cc/pow(10, mti.token_decimals)) as earnings, mti.token_name 
			FROM referral_payment rp
			JOIN margin_token_info mti
				on mti.pool_id = rp.pool_id
			where LOWER(payee_addr)=$1
				and rp.tx_confirmed = TRUE
			group by as_trader, rp.payee_addr, rp.pool_id, rp.code, mti.token_name;`
	rows, err := a.Db.Query(query, addr)
	defer rows.Close()
	if err != nil {
		slog.Error("Error for historic earnings" + err.Error())
		return []utils.APIResponseHistEarnings{}, errors.New("Unable to retreive earnings")
	}
	var el utils.APIResponseHistEarnings
	for rows.Next() {
		rows.Scan(&el.PoolId, &el.AsTrader, &el.Code, &el.Earnings, &el.TokenName)
		history = append(history, el)
	}
	return history, nil
}

// DbSetPaymentExecFinished sets the batch number and value for hasFinished
// When a payment execution starts, we set a new batch number and set the
// batch_finished status to false. Once done, we set the status to true
func (a *App) DbSetPaymentExecFinished(batchTs string, hasFinished bool) error {
	hasFinishedStr := strconv.FormatBool(hasFinished)
	query := `INSERT INTO referral_settings (property, value)
	VALUES ($1, $2),
		   ($3, $4)
	ON CONFLICT (property) DO UPDATE SET value = EXCLUDED.value`
	_, err := a.Db.Exec(query, "batch_timestamp", batchTs, "batch_finished", hasFinishedStr)
	if err != nil {
		slog.Error("DbSetPaymentExecFinished:" + err.Error())
		return err
	}
	return nil
}

func (a *App) DbGetPaymentExecHasFinished() (bool, error) {
	query := `SELECT value FROM referral_settings rs
			WHERE rs.property='batch_finished'`
	var hasFinished string
	err := a.Db.QueryRow(query).Scan(&hasFinished)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		slog.Error("DbGetPaymentExecHasFinished:" + err.Error())
		return true, err
	}
	return hasFinished == "true", nil
}
