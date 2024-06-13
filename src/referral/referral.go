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

	"github.com/D8-X/d8x-futures-go-sdk/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/viper"
)

type App struct {
	Db              *sql.DB
	MarginTokenInfo []DbMarginTokenInfo
	PaymentExecutor PayExec
	Settings        Settings
	Rpc             []string
	RpcClient       *ethclient.Client
	MultipayCtrct   *contracts.MultiPay
	BrokerAddr      string
}

type Settings struct {
	ChainId                int    `json:"chainId"`
	PaymentMaxLookBackDays int    `json:"paymentMaxLookBackDays"`
	PayCronSchedule        string `json:"paymentScheduleCron"`
	TokenX                 struct {
		Address  string `json:"address"`
		Decimals uint8  `json:"decimals"`
	} `json:"tokenX"`
	ReferrerCut      [][]float64    `json:"referrerCutPercentForTokenXHolding"`
	BrokerPayoutAddr common.Address `json:"brokerPayoutAddr"`
	BrokerId         string         `json:"brokerId"`
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

	// load settings
	slog.Info("Loading configuration")
	settings, err := loadConfig(viper)
	if err != nil {
		return err
	}
	a.Settings = settings

	slog.Info("Loading RPC configuration")
	rpcs, err := loadRPCConfig(viper)
	if err != nil {
		return err
	}
	a.Rpc = rpcs

	a.PaymentExecutor = &RemotePayExec{}
	slog.Info("Init PaymentExecutor")
	mAddr, err := config.GetMultiPayAddr(int64(a.Settings.ChainId))
	if err != nil {
		return errors.New("multipay not found in sdk " + err.Error())
	}
	fmt.Println("Multipay contract = " + mAddr)
	err = a.PaymentExecutor.Init(viper, mAddr)
	if err != nil {
		return err
	}

	if strings.EqualFold(a.PaymentExecutor.GetExecutorAddrHex(), a.PaymentExecutor.GetBrokerAddr().String()) {
		return errors.New("executor address and broker address must differ")
	}
	if strings.EqualFold(a.Settings.BrokerPayoutAddr.String(), a.PaymentExecutor.GetBrokerAddr().String()) {
		return errors.New("broker address and broker-payout address must differ")
	}

	// connect db
	connStr := viper.GetString(env.DATABASE_DSN_HISTORY)
	err = a.ConnectDB(connStr)
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

	slog.Info("Checking Token X")
	tkn, err := a.CreateErc20Instance(a.Settings.TokenX.Address)
	if err != nil {
		return err
	}
	for trial := 0; trial < 4; trial++ {
		_, err = a.QueryTokenBalance(tkn, a.Settings.BrokerPayoutAddr.Hex())
		if err == nil {
			break
		}
		slog.Info("failed to query Token X:" + err.Error())
		time.Sleep(time.Duration(2*(trial+1)) * time.Second)
	}
	if err != nil {
		return fmt.Errorf("TokenX address %s not valid, edit referralSettings: %s", a.Settings.TokenX.Address, err.Error())
	}
	slog.Info("Token X ok")
	return nil
}

// loadConfig loads the configuration file from the file system
// and returns the Setting struct
func loadConfig(v *viper.Viper) (Settings, error) {
	fileName := v.GetString(env.CONFIG_PATH)
	var settings []Settings
	data, err := os.ReadFile(fileName)
	if err != nil {
		return Settings{}, err
	}
	err = json.Unmarshal(data, &settings)
	if err != nil {
		return Settings{}, err
	}
	// pick correct setting by chain id
	var setting Settings = Settings{}
	targetChain := v.GetInt(env.CHAIN_ID)
	for k := 0; k < len(settings); k++ {
		if settings[k].ChainId == targetChain {
			setting = settings[k]
			if setting.BrokerId == "" {
				return Settings{}, errors.New("brokerId missing in referralSettings.json")
			}
			break
		}
	}
	if setting.ChainId != targetChain {
		return Settings{}, errors.New("No setting found for chain id " + strconv.Itoa(targetChain))
	}
	setting.TokenX.Address = strings.ToLower(setting.TokenX.Address)
	return setting, nil
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
	INSERT INTO referral_settings (property, value, broker_id)
	VALUES ($1, $2, $3)
	ON CONFLICT (property, broker_id) DO UPDATE SET value = EXCLUDED.value`
	_, err := a.Db.Exec(query, "payment_max_lookback_days", a.Settings.PaymentMaxLookBackDays, a.Settings.BrokerId)

	if err != nil {
		return err
	}

	// broker address
	addr := a.PaymentExecutor.GetBrokerAddr().String()
	addr = strings.ToLower(addr)
	a.BrokerAddr = addr
	_, err = a.Db.Exec(query, "broker_addr", addr, a.Settings.BrokerId)
	if err != nil {
		return err
	}
	err = a.DbCleanBrokerChain(addr)
	if err != nil {
		return err
	}
	// referral cut based on token holdings
	dec := a.Settings.TokenX.Decimals
	tkn := a.Settings.TokenX.Address

	query = `DELETE FROM referral_setting_cut WHERE broker_id=$1;`
	_, err = a.Db.Exec(query, a.Settings.BrokerId)
	if err != nil {
		slog.Error(err.Error())
	}

	for k := 0; k < len(a.Settings.ReferrerCut); k++ {
		perc := a.Settings.ReferrerCut[k][0]
		holding := a.Settings.ReferrerCut[k][1]
		holdingDecN := utils.FloatToDecN(holding, dec)
		query = `
		INSERT INTO referral_setting_cut (cut_perc, holding_amount_dec_n, token_addr, broker_id)
		VALUES ($1, $2, $3, $4)`
		_, err = a.Db.Exec(query, perc, holdingDecN.String(), tkn, a.Settings.BrokerId)
		if err != nil {
			slog.Error("could not insert referral setting:" + err.Error())
			continue
		}
	}

	return nil
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
		return errors.New("db not initialized")
	}
	var row DbMarginTokenInfo
	query := "select pool_id, token_addr, token_name, token_decimals from margin_token_info;"
	rows, err := a.Db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()
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
	for {
		slog.Info("Confirming payment transactions...")
		if a.confirmPaymentTrial() {
			slog.Info("ConfirmPaymentTxs completed")
			break
		}
		slog.Info("ConfirmPaymentTxs needs to be revisited")
		time.Sleep(time.Hour)
		a.confirmPaymentTrial()
	}
}

// confirmPaymentTrial returns true if all transactions could
// be processed, false if the function has to be restarted
func (a *App) confirmPaymentTrial() bool {
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
		bucket.WaitForToken("ConfirmPaymentTxs", false)
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
	}
	// tx could not be found. Not on chain? RPC issues?
	// only delete 2/3 days after batch ts
	doPurge = tsNow-ts > (86400*2)/3

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
	// if !doPurge && hasTxNotFound we need to retry
	// -> not(!doPurge && hasTxNotFound)=doPurge or !hasTxNotFound
	return doPurge || !hasTxNotFound
}

// DBGetUnconfirmedPayTxForLastBatch assembles an array of tx-ids ([]string)
// for payments that have not been confirmed (in table referral_payment)
// returns the tx-id string array and the timestamp of the batch
func (a *App) DBGetUnconfirmedPayTxForLastBatch() ([]string, int64) {
	query := `SELECT distinct(tx_hash),
				rp.batch_ts
				FROM referral_payment rp 
				WHERE rp.tx_confirmed = false 
				AND broker_id=$1
				AND batch_ts IN (
					SELECT max(rp2.batch_ts) as max_ts
					FROM referral_payment rp2 
					WHERE broker_id=$1
				)`
	rows, err := a.Db.Query(query, a.Settings.BrokerId)
	if err != nil {
		slog.Error("Error in DBGetUnconfirmedPayTxForLastBatch:" + err.Error())
		return []string{}, 0
	}
	defer rows.Close()
	var unTxs []string
	var unixTimestamp int64
	for rows.Next() {
		var tx string
		var batchTimestamp time.Time
		if err := rows.Scan(&tx, &batchTimestamp); err != nil {
			slog.Error("error scanning rows in DBGetUnconfirmedPayTxForLastBatch" + err.Error())
			continue
		}
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
	tsStart := time.Now().Unix() - int64(a.Settings.PaymentMaxLookBackDays*86400)
	lookBackBlock, _, err := contracts.FindBlockWithTs(a.RpcClient, uint64(tsStart))
	if err != nil {
		lookBackBlock = 0
	}
	payments, err := FilterPayments(a.MultipayCtrct, a.RpcClient, lookBackBlock, 0)
	if err != nil {
		slog.Error("reading onchain payments aborted with error " + err.Error())
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
	query := `SELECT trader_addr, payee_addr, 
					code, level, pool_id, batch_ts, paid_amount_cc, 
					tx_hash, block_ts
				FROM referral_payment rp 
				WHERE tx_confirmed = false
				AND broker_id=$1;`
	rows, err := a.Db.Query(query, a.Settings.BrokerId)
	if err != nil {
		return err
	}
	defer rows.Close()
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
			(trader_addr, payee_addr, code, level, pool_id, batch_ts, paid_amount_cc, tx_hash, ts, broker_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
		_, err := a.Db.Exec(query, row.TraderAddr, row.PayeeAddr, row.Code, row.Level, row.PoolId, row.BatchTs, row.PaidAmountCC,
			row.TxHash, row.BlockTs, a.Settings.BrokerId)
		idx++
		if err != nil {
			slog.Error("could not insert tx to failed tx " + row.TxHash + ": " + err.Error())
		}
		query = `DELETE FROM referral_payment rp
			WHERE tx_hash=$1`
		_, err = a.Db.Query(query, row.TxHash)
		if err != nil {
			slog.Error("Could not delete unconfirmed payments:" + err.Error())
		}
	}

	return nil
}

// DbGetReferralChainFromChild returns the percentage of trader
// fees earned by an agency.
// The fees for pure referrers (no agency connection)
// are calculated assuming they had "holdings" amount of tokens. If holdings
// is nil, the actual holdings stored in the DB are used
func (a *App) DbGetReferralChainFromChild(child string, holdings *big.Int) ([]DbReferralChainOfChild, bool, error) {
	child = strings.ToLower(child)
	var chain []DbReferralChainOfChild
	isAg, _ := a.IsAgency(child)
	if isAg {
		var row DbReferralChainOfChild
		query := `WITH RECURSIVE child_to_root AS 
			( 
				SELECT child, parent, pass_on, 1 AS lvl 
				FROM referral_chain 
					WHERE lower(child) = $1 
					AND broker_id=$2
				UNION ALL
				SELECT c.child, c.parent, c.pass_on, cr.lvl + 1
				FROM referral_chain c
				INNER JOIN child_to_root cr 
					ON lower(cr.parent) = lower(c.child)
					AND broker_id=$2
			)
			SELECT parent, child, pass_on, lvl
			FROM child_to_root
			ORDER BY -lvl;`
		rows, err := a.Db.Query(query, child, a.Settings.BrokerId)
		if err != nil {
			return []DbReferralChainOfChild{}, isAg, err
		}
		defer rows.Close()
		var currentPassOn float64 = 1.0
		for rows.Next() {
			rows.Scan(&row.Parent, &row.Child, &row.PassOn, &row.Lvl)
			row.PassOn = row.PassOn / 100.0
			row.ParentPay = currentPassOn * (1.0 - row.PassOn)
			currentPassOn = currentPassOn * row.PassOn
			row.ChildAvail = currentPassOn
			chain = append(chain, row)
			fmt.Println(row)
		}
		return chain, isAg, nil
	}
	var hParam interface{}
	if holdings == nil {
		hParam = nil // SQL NULL
	} else {
		hParam = holdings.String()
	}
	// referrer without agency, we calculate the rebate based
	// on token holdings
	query := `SELECT MAX(cut_perc) as max_cut
			FROM referral_setting_cut rsc
			LEFT join referral_token_holdings rth 
			on lower(rth.referrer_addr) = $1
			WHERE LOWER(rsc.token_addr)= $2
			AND rsc.holding_amount_dec_n<=coalesce($3, rth.holding_amount_dec_n, 0)
			AND rsc.broker_id=$4`
	var cut float64
	err := a.Db.QueryRow(query, child, a.Settings.TokenX.Address, hParam, a.Settings.BrokerId).Scan(&cut)
	if err != nil {
		slog.Error("Error for CutPercentageAgency address " + child)
		return []DbReferralChainOfChild{}, isAg, errors.New("could not get percentage")
	}
	cut = cut / 100
	var el = DbReferralChainOfChild{
		Parent:     a.BrokerAddr,
		Child:      child,
		PassOn:     cut,
		ParentPay:  1.0 - cut,
		ChildAvail: cut,
		Lvl:        1,
	}
	chain = append(chain, el)

	return chain, isAg, nil
}

// Cut percentage returns how much % (1% is represented as 1) of the broker fees
// trickle down to this agency or referrer address. For referrers without agency,
// we calculate the cut assuming they had "holdings" amount of tokens (can be nil
// to use the actual holding).
func (a *App) CutPercentageAgency(addr string, holdings *big.Int) (float64, bool, error) {
	addr = strings.ToLower(addr)
	chain, isAg, err := a.DbGetReferralChainFromChild(addr, holdings)
	if err != nil {
		slog.Error("Error for CutPercentageAgency address " + addr)
		return 0, false, errors.New("could not get percentage")
	}
	if len(chain) == 0 && isAg {
		// broker
		return 100, true, nil
	}
	return 100 * chain[len(chain)-1].ChildAvail, isAg, nil

}

// CutPercentageCode calculates the percent (1% -> 1) rebate on broker trading fees,
// when selecting this code
// Code has to be "cleaned" outside this function
func (a *App) CutPercentageCode(code string) (float64, error) {
	query := `SELECT referrer_addr, trader_rebate_perc 
			 FROM referral_code 
			 WHERE code=$1
			 AND broker_id=$2`
	var refAddr string
	var traderCut float64
	err := a.Db.QueryRow(query, code, a.Settings.BrokerId).Scan(&refAddr, &traderCut)
	if err != nil {
		//no log
		return 0, errors.New("could not identify code")
	}
	passOnCut, _, err := a.CutPercentageAgency(refAddr, nil)
	if err != nil {
		slog.Error("Error for CutPercentageCode code " + code)
		return 0, errors.New("could not identify cut")
	}
	return passOnCut * traderCut / 100, nil
}

// IsAgency returns true if the address is either the broker,
// or a child in the referral chain (hence an agency)
// The second parameter is true if it is the broker
func (a *App) IsAgency(addr string) (bool, bool) {
	query := `SELECT LOWER(child), false as is_broker
		FROM referral_chain 
		WHERE LOWER(child)=$1 AND broker_id=$2
		UNION SELECT value as child, true as is_broker
		FROM referral_settings 
		WHERE property='broker_addr' 
		AND LOWER(value)=$1
		AND broker_id=$2`
	var dbAddr string
	var isBroker bool
	err := a.Db.QueryRow(query, addr, a.Settings.BrokerId).Scan(&dbAddr, &isBroker)
	return err != sql.ErrNoRows, isBroker
}

// dbWriteTx write info about the payment transaction into referral_payment
func (a *App) dbWriteTx(traderAddr string, code string, amounts []*big.Int, payees []common.Address, batchTs string, poolId uint32, tx string) {
	slog.Info("Inserting Payment TX in DB")
	t, _ := strconv.Atoi(batchTs)
	ts := time.Unix(int64(t), 0)
	query := `INSERT INTO referral_payment (trader_addr, payee_addr, code, level, pool_id, batch_ts, paid_amount_cc, tx_hash, broker_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	for k := 0; k < len(payees); k++ {
		if amounts[k].BitLen() == 0 {
			// we don't write 0 amounts to the database
			continue
		}
		_, err := a.Db.Exec(query,
			strings.ToLower(traderAddr),
			strings.ToLower(payees[k].String()),
			code,
			k,
			poolId,
			ts,
			amounts[k].String(),
			tx,
			a.Settings.BrokerId)
		if err != nil {
			slog.Error(fmt.Sprintf("dbWriteTx: could not insert tx to db for trader %s, lvl %d, code %s: %s", traderAddr, k, code, err.Error()))
		}
	}
}

// writeDbPayment writes data from multipay contract into the database
// if there is already an entry for a given record which is not confirmed, it sets the confirmed flag to true
// db keys are trader_addr, payee_addr, pool_id and batch_ts
func (a *App) writeDbPayment(traderAddr string, payeeAddr string, p PaymentLog, payIdx int) error {
	if a.Db == nil {
		return errors.New("db not initialized")
	}
	utcBatchTime := time.Unix(int64(p.BatchTimestamp), 0)
	utcBlockTime := time.Unix(int64(p.BlockTs), 0)
	query := `SELECT tx_confirmed FROM referral_payment 
			  WHERE lower(trader_addr) = lower($1) 
			  	AND lower(payee_addr) = lower($2) 
				AND batch_ts = $3 
				AND pool_id=$4
				AND level=$5
				AND broker_id=$6`
	var isConfirmed bool
	err := a.Db.QueryRow(query, traderAddr, payeeAddr, utcBatchTime, p.PoolId, payIdx, a.Settings.BrokerId).Scan(&isConfirmed)
	if err == sql.ErrNoRows {
		// insert
		query = `INSERT INTO referral_payment 
			(trader_addr, payee_addr, code, level, 
			 pool_id, batch_ts, paid_amount_cc, tx_hash, 
			 block_nr, block_ts, tx_confirmed, broker_id)
          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
		_, err := a.Db.Exec(query,
			traderAddr,
			payeeAddr,
			p.Code,
			payIdx,
			p.PoolId,
			utcBatchTime,
			p.AmountDecN[payIdx].String(),
			p.TxHash,
			p.BlockNumber,
			utcBlockTime,
			true,
			a.Settings.BrokerId)
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
					AND level = $4
					AND broker_id = $5`
		_, err := a.Db.Exec(query, traderAddr, payeeAddr, utcBatchTime, payIdx, p.BlockNumber, utcBlockTime, a.Settings.BrokerId)
		if err != nil {
			return errors.New("Failed to insert data: " + err.Error())
		}
	}
	return nil
}

// HasLoopOnChainAddition returns true if when adding the new child to the
// referral chain, there would be a loop
func (a *App) HasLoopOnChainAddition(parent string, newChild string) (bool, error) {

	chain, _, err := a.DbGetReferralChainFromChild(parent, nil)
	if err != nil {
		return true, err
	}
	newChild = strings.ToLower(newChild)
	for _, el := range chain {
		if strings.ToLower(el.Parent) == newChild || strings.ToLower(el.Child) == newChild {
			return true, nil
		}
	}
	return false, nil
}

// SelectCode tries to select a given code for a trader. Future trades will
// be using this code.
// Signature must have been checked
// before. The error message returned (if any) is exposed to the API
func (a *App) SelectCode(csp utils.APICodeSelectionPayload) error {
	csp.TraderAddr = strings.ToLower(csp.TraderAddr)
	timeNow := time.Now().Unix()
	// code exists?
	query := `SELECT expiry
		FROM referral_code
		WHERE code=$1
		AND broker_id=$2`
	var ts time.Time
	err := a.Db.QueryRow(query, csp.Code, a.Settings.BrokerId).Scan(&ts)
	if err != sql.ErrNoRows && err != nil {
		slog.Info("Failed to search for code:" + err.Error())
		return errors.New("Failed")
	} else if err == sql.ErrNoRows {
		slog.Info("Code does not exist")
		return errors.New("Failed")
	}
	if ts != (time.Time{}) && ts.Before(time.Unix(timeNow, 0)) {
		slog.Info("Code " + csp.Code + " expired")
		return errors.New("code expired")
	}

	// first reset valid until for code
	type SQLResponse struct {
		TraderAddr string
		Code       string
		ValidFrom  time.Time
		ValidTo    time.Time
	}
	var latestCode SQLResponse
	query = `
		SELECT trader_addr, code, valid_from, valid_to
		FROM referral_code_usage
		WHERE LOWER(trader_addr)=$1
		AND broker_id=$2
		ORDER BY valid_to DESC
		LIMIT 1`
	err = a.Db.QueryRow(query, csp.TraderAddr, a.Settings.BrokerId).Scan(
		&latestCode.TraderAddr, &latestCode.Code, &latestCode.ValidFrom, &latestCode.ValidTo)
	if err != sql.ErrNoRows && err != nil {
		slog.Info("Failed to query latest code:" + err.Error())
		return errors.New("Failed")
	}

	if latestCode.Code != "" {
		if latestCode.Code == csp.Code {
			return errors.New("code already selected")
		}
		// update valid-to of old code
		query = `UPDATE referral_code_usage
		SET valid_to=to_timestamp($1)
		WHERE LOWER(trader_addr)=$2
			AND code=$3
			AND valid_to=$4
			AND broker_id=$5`
		_, err := a.Db.Exec(query, timeNow, csp.TraderAddr, latestCode.Code, latestCode.ValidTo, a.Settings.BrokerId)
		if err != nil {
			slog.Error("Failed to insert data: " + err.Error())
			return errors.New("failed updating existing code")
		}
	}
	// now insert new code
	query = `INSERT INTO referral_code_usage (trader_addr, valid_from, code, broker_id) VALUES ($1, to_timestamp($2), $3, $4)`
	_, err = a.Db.Exec(query, csp.TraderAddr, timeNow, csp.Code, a.Settings.BrokerId)
	if err != nil {
		slog.Error("Failed to insert data: " + err.Error())
		return errors.New("failed inserting new code")
	}
	return nil
}

// UpsertCode inserts new codes and updates the code rebate
func (a *App) UpsertCode(csp utils.APICodePayload) error {
	var passOn float32 = float32(csp.PassOnPercTDF) / 100.0
	// check whether code exists
	query := `SELECT referrer_addr 
		FROM referral_code
		WHERE code=$1
		AND broker_id=$2`
	var refAddr string
	err := a.Db.QueryRow(query, csp.Code, a.Settings.BrokerId).Scan(&refAddr)
	if err != sql.ErrNoRows && err != nil {
		slog.Info("Failed to query latest code:" + err.Error())
		return errors.New("Failed")
	} else if err == sql.ErrNoRows {
		// not found, we can insert
		query = `INSERT INTO referral_code (code, referrer_addr, trader_rebate_perc, broker_id)
          VALUES ($1, $2, $3, $4)`
		_, err := a.Db.Exec(query, csp.Code, csp.ReferrerAddr, passOn, a.Settings.BrokerId)
		if err != nil {
			slog.Error("Failed to insert code" + err.Error())
			return errors.New("failed to insert code")
		}
		return nil
	}
	// found, we check whether the referral addr is correct
	if strings.ToLower(refAddr) != csp.ReferrerAddr {
		return errors.New("not code owner")
	}
	query = `UPDATE referral_code SET trader_rebate_perc = $1
			 WHERE code = $2 AND broker_id=$3`
	_, err = a.Db.Exec(query, passOn, csp.Code, a.Settings.BrokerId)
	if err != nil {
		return errors.New("Failed to insert data: " + err.Error())
	}
	return nil
}

// Refer handles new referral requests (checks and insert into db)
func (a *App) Refer(rpl utils.APIReferPayload) error {
	var passOn float32 = float32(rpl.PassOnPercTDF) / 100.0
	rpl.ParentAddr = strings.ToLower(rpl.ParentAddr)
	// parent can only refer if they are the broker or a child
	if isAg, _ := a.IsAgency(rpl.ParentAddr); !isAg {
		return errors.New("not an agency")
	}
	rpl.ReferToAddr = strings.ToLower(rpl.ReferToAddr)
	h, err := a.HasLoopOnChainAddition(rpl.ParentAddr, rpl.ReferToAddr)
	if err != nil {
		slog.Error("HasLoopOnChainAddition failed")
		return errors.New("failed")
	}
	if h {
		return errors.New("referral already in chain")
	}
	query := "SELECT child from referral_chain WHERE LOWER(child)=$1 AND broker_id=$2"
	var addr string
	err = a.Db.QueryRow(query, rpl.ReferToAddr, a.Settings.BrokerId).Scan(&addr)
	if err != sql.ErrNoRows {
		return errors.New("refer to addr already in use")
	}
	// referral chain length
	chain, _, err := a.DbGetReferralChainFromChild(rpl.ParentAddr, nil)
	if err == nil && len(chain) > env.MAX_REFERRAL_CHAIN_LEN {
		slog.Info("Max referral chain length reached for " + rpl.ParentAddr)
		return errors.New("reached maximum number of referrals")
	}
	// now safe to insert
	query = `INSERT INTO referral_chain (parent, child, pass_on, broker_id)
          	 VALUES ($1, $2, $3, $4)`
	_, err = a.Db.Exec(query, rpl.ParentAddr, rpl.ReferToAddr, passOn, a.Settings.BrokerId)
	if err != nil {
		slog.Error("Failed to insert referral" + err.Error())
		return errors.New("failed to insert referral")
	}
	return nil
}

// DbUpdateTokenHoldings queries balances of TokenX from the blockchain
// and updates the holdings for active referrers in the database
func (a *App) DbUpdateTokenHoldings() error {
	// select referrers that are no agency (not in referral chain)
	refAddr, lastUpdate, err := a.DbGetActiveReferrers()
	if err != nil {
		return err
	}
	tkn, err := a.CreateErc20Instance(a.Settings.TokenX.Address)
	if err != nil {
		return err
	}
	nowTime := time.Now()
	for k := 0; k < len(refAddr); k++ {
		currReferrerAddr := refAddr[k]
		if lastUpdate[k] != (time.Time{}) && nowTime.Sub(lastUpdate[k]).Hours() < env.REFERRER_TOKENX_BAL_FREQ_H {
			slog.Info("No balance update required for referrer " + currReferrerAddr)
			continue
		}
		slog.Info("Updating balance for referrer" + currReferrerAddr)

		holdings, err := a.QueryTokenBalance(tkn, currReferrerAddr)
		if err != nil {
			slog.Error("Error when trying to get token balance:" + err.Error())
			continue
		}
		query := `
		INSERT INTO referral_token_holdings (referrer_addr, holding_amount_dec_n, token_addr)
		VALUES ($1, $2, $3)
		ON CONFLICT (referrer_addr, token_addr) DO UPDATE SET holding_amount_dec_n = EXCLUDED.holding_amount_dec_n`
		_, err = a.Db.Exec(query, currReferrerAddr, holdings.String(), a.Settings.TokenX.Address)
		if err != nil {
			slog.Error("Error when trying to upsert token balance:" + err.Error())
			continue
		}
	}

	return nil
}

// HistoricEarnings calculates historic earnings of any participant
// (broker-payout address, agent, referrer, trader). If multiple brokers are
// using this database, then those earnings are aggregated here.
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
			where LOWER(payee_addr)=LOWER($1)
				and rp.tx_confirmed = TRUE
			group by as_trader, rp.payee_addr, rp.pool_id, rp.code, mti.token_name;`
	rows, err := a.Db.Query(query, addr)
	if err != nil {
		slog.Error("Error for historic earnings" + err.Error())
		return []utils.APIResponseHistEarnings{}, errors.New("unable to retreive earnings")
	}
	defer rows.Close()
	var el utils.APIResponseHistEarnings
	for rows.Next() {
		rows.Scan(&el.PoolId, &el.AsTrader, &el.Code, &el.Earnings, &el.TokenName)
		history = append(history, el)
	}
	return history, nil
}

// DbGetActiveReferrers returns a list of addresses that are
// (1) not an agency, (2) have a code which is used in the
// view referral_aggr_fees_per_trader. The func also returns the last
// token balance update time
func (a *App) DbGetActiveReferrers() ([]string, []time.Time, error) {
	query := `SELECT distinct(lower(rc.referrer_addr)), rth.last_updated 
			  FROM referral_aggr_fees_per_trader rafpt 
			  JOIN referral_code rc 
				ON rc.code = rafpt.code
				AND LOWER(rc.referrer_addr) NOT IN (SELECT LOWER(rc2.child) FROM referral_chain rc2)
			  LEFT JOIN referral_token_holdings rth 
				ON LOWER(rth.referrer_addr) = LOWER(rc.referrer_addr)
			  WHERE rafpt.broker_id=$1`
	rows, err := a.Db.Query(query, a.Settings.BrokerId)
	if err != nil {
		msg := ("Error getting DbGetActiveReferrers" + err.Error())
		return []string{}, []time.Time{}, errors.New(msg)
	}
	defer rows.Close()
	var refAddr []string
	var lastUpdts []time.Time
	for rows.Next() {
		var addr string
		var ts time.Time
		rows.Scan(&addr, &ts)
		refAddr = append(refAddr, addr)
		lastUpdts = append(lastUpdts, ts)
	}
	return refAddr, lastUpdts, nil
}

func (a *App) DbGetMyReferrals(addr string) ([]utils.APIResponseMyReferrals, error) {
	// as agency select downstream partners and pass-on
	query := `SELECT child, pass_on as pass_on_perc
			   FROM referral_chain rc 
			  WHERE lower(rc.parent) = $1
			  AND broker_id=$2
				UNION -- as referrer:
			  SELECT code as child, trader_rebate_perc as pass_on_perc
				FROM referral_code 
				WHERE lower(referrer_addr) = $1
				AND broker_id=$2`
	rows, err := a.Db.Query(query, addr, a.Settings.BrokerId)
	if err != nil {
		slog.Error("Error in DbGetMyReferrals: " + err.Error())
		return []utils.APIResponseMyReferrals{}, errors.New("failed to get referrals")
	}
	defer rows.Close()
	var res []utils.APIResponseMyReferrals
	for rows.Next() {
		var el utils.APIResponseMyReferrals
		rows.Scan(&el.Referral, &el.PassOnPerc)
		res = append(res, el)
	}
	return res, nil
}

func (a *App) DbGetMyCodeSelection(addr string) (string, error) {
	query := `SELECT code
			  FROM referral_code_usage rcu 
			  WHERE lower(rcu.trader_addr) = $1
			  AND valid_to>NOW() AND valid_from<NOW()
			  AND broker_id=$2`
	var code string
	err := a.Db.QueryRow(query, addr, a.Settings.BrokerId).Scan(&code)
	if err == sql.ErrNoRows {
		return "", nil
	} else if err != nil {
		slog.Error("DbMyCodeSelection failed:" + err.Error())
		return "", errors.New("code retrieval failed")
	}
	return code, nil
}

// DbSetPaymentExecFinished sets the batch number and value for hasFinished
// When a payment execution starts, we set a new batch number and set the
// batch_finished status to false. Once done, we set the status to true
func (a *App) DbSetPaymentExecFinished(batchTs string, hasFinished bool) error {
	hasFinishedStr := strconv.FormatBool(hasFinished)
	query := `INSERT INTO referral_settings (property, value, broker_id)
	VALUES ($1, $2, $5),
		   ($3, $4, $5)
	ON CONFLICT (broker_id, property) DO UPDATE SET value = EXCLUDED.value`
	_, err := a.Db.Exec(query, "batch_timestamp", batchTs, "batch_finished", hasFinishedStr, a.Settings.BrokerId)
	if err != nil {
		slog.Error("DbSetPaymentExecFinished:" + err.Error())
		return err
	}
	return nil
}

func (a *App) DbGetTokenInfo() (utils.APIResponseTokenHoldings, error) {
	query := `SELECT cut_perc, holding_amount_dec_n/power(10, $1) as holding, token_addr 
	          FROM referral_setting_cut rsc
			  WHERE broker_id=$2`
	rows, err := a.Db.Query(query, a.Settings.TokenX.Decimals, a.Settings.BrokerId)
	if err != nil {
		slog.Error("Error in DbGetTokenInfo: " + err.Error())
		return utils.APIResponseTokenHoldings{}, errors.New("failed to get token info")
	}
	defer rows.Close()
	var res utils.APIResponseTokenHoldings
	for rows.Next() {
		var el utils.APIRebate
		rows.Scan(&el.CutPerc, &el.Holding, &res.TokenAddr)
		res.Rebates = append(res.Rebates, el)
	}
	return res, nil
}

// Find parent in referral chain that is never a child. This
// should be the broker. If it is not, this means the broker
// was switched out and we update codes and chain with the
// current broker address
func (a *App) DbCleanBrokerChain(brokerAddr string) error {
	// find parent that is never a child (broker!)
	query := `SELECT DISTINCT lower(parent)
				FROM referral_chain rc 
				WHERE 
				broker_id=$1 AND
				lower(parent) NOT IN (
					SELECT DISTINCT lower(child)
					FROM referral_chain
					WHERE broker_id=$1
				);`
	rows, err := a.Db.Query(query, a.Settings.BrokerId)
	if err != nil {
		return errors.New("Error in DbCleanBrokerChain: " + err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		var addr string
		rows.Scan(&addr)
		if addr == brokerAddr {
			continue
		}
		msg := fmt.Sprintf("DbCleanBrokerChain: found obsolete broker %s in referral chain. Replacing with %s", addr, brokerAddr)
		slog.Info(msg)
		// replace in referral_code
		query = `UPDATE referral_code
					SET referrer_addr = CASE
										WHEN lower(referrer_addr) = $1 THEN $2
										ELSE lower(referrer_addr)
										END
					WHERE lower(referrer_addr) = $1 
					AND broker_id=$3;`
		_, err := a.Db.Exec(query, addr, brokerAddr, a.Settings.BrokerId)
		if err != nil {
			return errors.New("Failed to update data: " + err.Error())
		}
		// also replace in referral chain
		query = `UPDATE referral_chain
					SET parent= CASE
								WHEN lower(parent) = $1 THEN $2
								ELSE lower(parent)
								END
					WHERE lower(parent) = $1
					AND broker_id=$3;`
		_, err = a.Db.Exec(query, addr, brokerAddr, a.Settings.BrokerId)
		if err != nil {
			return errors.New("Failed to update data: " + err.Error())
		}
	}
	return nil
}
