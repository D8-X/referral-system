package referral

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"referral-system/env"
	"referral-system/src/contracts"
	"strconv"
	"time"

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
}

type Settings struct {
	PaymentMaxLookBackDays int    `json:"paymentMaxLookBackDays"`
	PayCronSchedule        string `json:"paymentScheduleMinHourDayofmonthWeekday"`
	MultiPayContractAddr   string `json:"multiPayContractAddr"`
}

type Rpc struct {
	ChainId int      `json:"chainId"`
	Rpc     []string `json:"HTTP"`
}

type DbReferralChain struct {
	Parent    string
	Child     string
	PassOn    float32
	CreatedOn time.Time
}

type DbReferralChainOfChild struct {
	Parent   string
	Child    string
	PassOn   float32
	Lvl      uint8
	ChildPay float64 // fraction of total payment to child; not in DB
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
	PoolId       int
	BatchTs      time.Time
	PaidAmountCC string
	TxHash       string
	BlockTs      time.Time
	TxConfirmed  bool
}

func (a *App) New(viper *viper.Viper) error {

	// decide whether we have a local broker or a remote broker
	if viper.GetString(env.REMOTE_BROKER_HTTP) == "" {
		a.PaymentExecutor = &LocalPayExec{}
	} else {
		a.PaymentExecutor = &RemotePayExec{}
	}
	err := a.PaymentExecutor.Init(viper)
	if err != nil {
		return err
	}

	// connect db
	connStr := viper.GetString(env.DATABASE_DSN_HISTORY)
	err = a.ConnectDB(connStr)
	if err != nil {
		return err
	}
	// load settings
	s, err := loadConfig(viper)
	if err != nil {
		return err
	}
	a.Settings = s
	err = a.SettingsToDB()
	if err != nil {
		return err
	}

	rpcs, err := loadRPCConfig(viper)
	if err != nil {
		return err
	}
	a.Rpc = rpcs

	err = a.CreateRpcClient()
	if err != nil {
		return err
	}
	err = a.CreateMultipayInstance()
	if err != nil {
		return err
	}
	return nil
}

func loadConfig(v *viper.Viper) (Settings, error) {
	fileName := v.GetString(env.CONFIG_PATH)
	var s Settings
	data, err := os.ReadFile(fileName)
	if err != nil {
		return Settings{}, err
	}
	err = json.Unmarshal(data, &s)
	if err != nil {
		return Settings{}, err
	}
	return s, nil
}

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

func (a *App) SettingsToDB() error {
	query := `
	INSERT INTO referral_settings (property, value)
	VALUES ($1, $2)
	ON CONFLICT (property) DO UPDATE SET value = EXCLUDED.value`
	_, err := a.Db.Exec(query, "payment_max_lookback_days", a.Settings.PaymentMaxLookBackDays)

	if err != nil {
		return err
	}

	// broker address
	addr := a.PaymentExecutor.GetBrokerAddr().String()
	query = `
	INSERT INTO referral_settings (property, value)
	VALUES ($1, $2)
	ON CONFLICT (property) DO UPDATE SET value = EXCLUDED.value`
	_, err = a.Db.Exec(query, "broker_addr", addr)
	if err != nil {
		return err
	}
	return nil
}

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

func (a *App) DbGetReferralChainOfChild(child string) ([]DbReferralChainOfChild, error) {
	var row DbReferralChainOfChild
	var chain []DbReferralChainOfChild
	query := "WITH RECURSIVE child_to_root AS (" +
		"SELECT child, parent, pass_on, 1 AS lvl " +
		"FROM referral_chain " +
		"WHERE child = '" + child + "' " +
		"UNION ALL " +
		"SELECT c.child, c.parent, c.pass_on, cr.lvl + 1 " +
		"FROM referral_chain c " +
		"INNER JOIN child_to_root cr ON cr.parent = c.child " +
		") " +
		"SELECT parent, child, pass_on, lvl " +
		"FROM child_to_root " +
		"ORDER BY -lvl;"
	rows, err := a.Db.Query(query)
	defer rows.Close()
	if err != nil {
		return []DbReferralChainOfChild{}, err
	}
	var currentPassOn float64 = 1.0
	for rows.Next() {
		rows.Scan(&row.Parent, &row.Child, &row.PassOn, &row.Lvl)
		currentPassOn = currentPassOn * float64(row.PassOn) / 100.0
		row.ChildPay = currentPassOn
		chain = append(chain, row)
		fmt.Println(row)
	}
	return chain, nil
}

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

// SavePayments gets the payment events from on-chain and
// updates or inserts database entries
func (a *App) SavePayments() error {
	payments, err := FilterPayments(a.MultipayCtrct, a.RpcClient)
	if err != nil {
		return err
	}
	for _, p := range payments {
		// key = trader_addr, payee_addr, pool_id, batch_timestamp
		traderAddr := p.PayeeAddr[0].String()
		for k, payee := range p.PayeeAddr {
			if p.AmountDecN[k].BitLen() == 0 {
				continue
			}
			a.writeDbPayment(traderAddr, payee.String(), p, k)
		}

	}
	return nil
}

// writeDbPayment writes data from multiplay contract into the database
// if there is already an entry for a given record which is not confirmed, it sets the confirmed flag to true
// db keys are trader_addr, payee_addr, pool_id and batch_ts
func (a *App) writeDbPayment(traderAddr string, payeeAddr string, p PaymentLog, payIdx int) error {
	if a.Db == nil {
		return errors.New("Db not initialized")
	}
	utcBatchTime := time.Unix(int64(p.BatchTimestamp), 0)
	utcBlockTime := time.Unix(int64(p.BlockTs), 0)
	query := "SELECT tx_confirmed FROM referral_payment " +
		"WHERE trader_addr = $1 AND payee_addr = $2 AND batch_ts = $3"
	var isConfirmed bool
	err := a.Db.QueryRow(query, traderAddr, payeeAddr, utcBatchTime).Scan(&isConfirmed)
	if err == sql.ErrNoRows {
		// insert
		query = `INSERT INTO referral_payment (trader_addr, payee_addr, code, pool_id, batch_ts, paid_amount_cc, tx_hash, block_nr, block_ts, tx_confirmed)
          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
		_, err := a.Db.Exec(query, traderAddr, payeeAddr, p.Code, p.PoolId, utcBatchTime, p.AmountDecN[payIdx].String(), p.TxHash, p.BlockNumber, utcBlockTime, true)
		if err != nil {
			return errors.New("Failed to insert data: " + err.Error())
		}
	} else if err != nil {
		return err
	} else if isConfirmed == false {
		// set tx confirmed to true
		query = `UPDATE referral_payment SET tx_confirmed = true
				WHERE trader_addr = $1 AND payee_addr = $2 AND batch_ts = $3`
		_, err := a.Db.Exec(query, traderAddr, payeeAddr, utcBatchTime)
		if err != nil {
			return errors.New("Failed to insert data: " + err.Error())
		}
	}
	return nil
}
