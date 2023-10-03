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
	Settings        Settings
	Rpc             []string
	RpcClient       *ethclient.Client
	MultipayCtrct   *contracts.MultiPay
}

type Settings struct {
	PaymentMaxLookBackDays int    `json:"paymentMaxLookBackDays"`
	PayCronSchedule        string `json:"paymentScheduleMinHourDayofmonthWeekday"`
	MultiPayContractAddr   string `json:"multiPayContractAddr"`
	TokenX                 struct {
		Address  string `json:"address"`
		Decimals uint8  `json:"decimals"`
	} `json:"tokenX"`
	ReferrerCut [][]float64 `json:"referrerCutPercentForTokenXHolding"`
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
	s.TokenX.Address = strings.ToLower(s.TokenX.Address)
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

	// referral cut based on token holdings
	dec := a.Settings.TokenX.Decimals
	tkn := a.Settings.TokenX.Address

	query = `DELETE FROM referral_setting_cut;`
	_, err = a.Db.Exec(query)
	if err != nil {
		slog.Error(err.Error())
	}

	for k := 0; k < len(a.Settings.ReferrerCut); k++ {
		perc := a.Settings.ReferrerCut[k][0]
		holding := a.Settings.ReferrerCut[k][1]
		holdingDecN := toDecN(holding, dec)
		query = `
		INSERT INTO referral_setting_cut (cut_perc, holding_amount_dec_n, token_addr)
		VALUES ($1, $2, $3)`
		_, err = a.Db.Exec(query, perc, holdingDecN.String(), tkn)
		if err != nil {
			slog.Error("could not insert referral setting:" + err.Error())
			continue
		}
	}

	return nil
}

// toDecN converts a floating point number to a decimal-n,
// i.e., to num*10^decN
func toDecN(num float64, decN uint8) *big.Int {
	// Convert the floating-point number to a big.Float
	floatNumber := new(big.Float).SetFloat64(num)

	// Multiply the floatNumber by 10^n
	multiplier := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decN)), nil))
	result := new(big.Float).Mul(floatNumber, multiplier)

	// Convert the result to a big.Int
	intResult, _ := result.Int(nil)
	return intResult
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

// DbGetReferralChainOfChild returns the percentage of trader fees earned by an
// agency
func (a *App) DbGetReferralChainOfChild(child string) ([]DbReferralChainOfChild, error) {
	child = strings.ToLower(child)
	var row DbReferralChainOfChild
	var chain []DbReferralChainOfChild
	query := "WITH RECURSIVE child_to_root AS (" +
		"SELECT child, parent, pass_on, 1 AS lvl " +
		"FROM referral_chain " +
		"WHERE lower(child) = '" + child + "' " +
		"UNION ALL " +
		"SELECT c.child, c.parent, c.pass_on, cr.lvl + 1 " +
		"FROM referral_chain c " +
		"INNER JOIN child_to_root cr ON lower(cr.parent) = lower(c.child)" +
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

func (a *App) CutPercentage(addr string) {

}

// Is agency returns true if the address is either the broker,
// or a child in the referral chain (hence an agency)
func (a *App) IsAgency(addr string) bool {
	query := `SELECT child from referral_chain WHERE LOWER(child)=$1
		UNION SELECT value as child from referral_setting WHERE property="broker_addr" AND LOWER(value)=$1`
	var dbAddr string
	err := a.Db.QueryRow(query, addr).Scan(&dbAddr)
	return err != sql.ErrNoRows
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

// HasLoopOnChainAddition returns true if when adding the new child to the
// referral chain, there would be a loop
func (a *App) HasLoopOnChainAddition(parent string, newChild string) (bool, error) {
	chain, err := a.DbGetReferralChainOfChild(parent)
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

// SelectCode tries to select a given code. Signature must have been checked
// before. The error message returned (if any) is exposed to the API
func (a *App) SelectCode(csp utils.APICodeSelectionPayload) error {
	csp.TraderAddr = strings.ToLower(csp.TraderAddr)
	timeNow := time.Now().Unix()
	// code exists?
	query := `SELECT expiry
		FROM referral_code
		where code=$1`
	var ts time.Time
	err := a.Db.QueryRow(query, csp.Code).Scan(&ts)
	if err != sql.ErrNoRows && err != nil {
		slog.Info("Failed to search for code:" + err.Error())
		return errors.New("Failed")
	} else if err == sql.ErrNoRows {
		slog.Info("Code does not exist")
		return errors.New("Failed")
	}
	if ts != (time.Time{}) && ts.Before(time.Unix(timeNow, 0)) {
		slog.Info("Code " + csp.Code + " expired")
		return errors.New("Code expired")
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
		ORDER BY valid_to DESC
		LIMIT 1`
	err = a.Db.QueryRow(query, csp.TraderAddr).Scan(&latestCode.TraderAddr, &latestCode.Code, &latestCode.ValidFrom, &latestCode.ValidTo)
	if err != sql.ErrNoRows && err != nil {
		slog.Info("Failed to query latest code:" + err.Error())
		return errors.New("Failed")
	}

	if latestCode.Code != "" {
		if latestCode.Code == csp.Code {
			return errors.New("Code already selected")
		}
		// update valid to of old code
		query = `UPDATE referral_code_usage
		SET valid_to=to_timestamp($1)
		WHERE LOWER(trader_addr)=$2
			AND code=$3
			AND valid_to=$4`
		_, err := a.Db.Exec(query, timeNow, csp.TraderAddr, latestCode.Code, latestCode.ValidTo)
		if err != nil {
			slog.Error("Failed to insert data: " + err.Error())
			return errors.New("Failed updating existing code")
		}
	}
	// now insert new code
	query = `INSERT INTO referral_code_usage (trader_addr, valid_from, code) VALUES ($1, to_timestamp($2), $3)`
	_, err = a.Db.Exec(query, csp.TraderAddr, timeNow, csp.Code)
	if err != nil {
		slog.Error("Failed to insert data: " + err.Error())
		return errors.New("Failed inserting new code")
	}
	return nil
}

func (a *App) UpsertCode(csp utils.APICodePayload) error {
	var passOn float32 = float32(csp.PassOnPercTDF) / 100.0
	// check whether code exists
	query := `SELECT referrer_addr 
		FROM referral_code
		WHERE code=$1`
	var refAddr string
	err := a.Db.QueryRow(query, csp.Code).Scan(&refAddr)
	if err != sql.ErrNoRows && err != nil {
		slog.Info("Failed to query latest code:" + err.Error())
		return errors.New("Failed")
	} else if err == sql.ErrNoRows {
		// not found, we can insert
		query = `INSERT INTO referral_code (code, referrer_addr, trader_rebate_perc)
          VALUES ($1, $2, $3)`
		_, err := a.Db.Exec(query, csp.Code, csp.ReferrerAddr, passOn)
		if err != nil {
			slog.Error("Failed to insert code" + err.Error())
			return errors.New("Failed to insert code")
		}
		return nil
	}
	// found, we check whether the referral addr is correct
	if strings.ToLower(refAddr) != strings.ToLower(csp.ReferrerAddr) {
		return errors.New("Not code owner")
	}
	query = `UPDATE referral_code SET trader_rebate_perc = $1
			 WHERE code = $2`
	_, err = a.Db.Exec(query, passOn, csp.Code)
	if err != nil {
		return errors.New("Failed to insert data: " + err.Error())
	}
	return nil
}

func (a *App) Refer(rpl utils.APIReferPayload) error {
	var passOn float32 = float32(rpl.PassOnPercTDF) / 100.0
	rpl.ParentAddr = strings.ToLower(rpl.ParentAddr)
	// parent can only refer if they are the broker or a child
	if !a.IsAgency(rpl.ParentAddr) {
		return errors.New("Not an agency")
	}
	rpl.ReferToAddr = strings.ToLower(rpl.ReferToAddr)
	h, err := a.HasLoopOnChainAddition(rpl.ParentAddr, rpl.ReferToAddr)
	if err != nil {
		slog.Error("HasLoopOnChainAddition failed")
		return errors.New("Failed")
	}
	if h {
		return errors.New("Referral already in chain")
	}
	query := "SELECT child from referral_chain WHERE LOWER(child)=$1"
	var addr string
	err = a.Db.QueryRow(query, rpl.ReferToAddr).Scan(&addr)
	if err != sql.ErrNoRows {
		return errors.New("Refer to addr already in use")
	}
	// now safe to insert
	query = `INSERT INTO referral_chain (parent, child, pass_on)
          	 VALUES ($1, $2, $3)`
	_, err = a.Db.Exec(query, rpl.ParentAddr, rpl.ReferToAddr, passOn)
	if err != nil {
		slog.Error("Failed to insert referral" + err.Error())
		return errors.New("Failed to insert referral")
	}
	return nil
}

// DbUpdateTokenHoldings queries balances of TokenX from the blockchain
// and updates the holdings in the database
func (a *App) DbUpdateTokenHoldings() error {
	// select referrers that are no agency (not in referral chain)
	query := `SELECT distinct LOWER(rc.referrer_addr) as referrer_addr FROM referral_code rc
		WHERE expiry>NOW() AND LOWER(referrer_addr) NOT IN
		(SELECT LOWER(child) as referrer_addr FROM referral_chain)`
	rows, err := a.Db.Query(query)
	defer rows.Close()
	if err != nil {
		return err
	}

	tkn, err := a.CreateErc20Instance(a.Settings.TokenX.Address)
	if err != nil {
		return err
	}

	var currReferrerAddr string
	for rows.Next() {
		rows.Scan(&currReferrerAddr)
		slog.Info("Adding addr to list of pure referrers " + currReferrerAddr)

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
