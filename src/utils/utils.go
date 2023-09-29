package utils

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"referral-system/env"
	"strconv"
	"time"

	"github.com/spf13/viper"
)

type App struct {
	Db              *sql.DB
	MarginTokenInfo []DbMarginTokenInfo
	Settings        Settings
	Rpc             []string
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

func (a *App) New(viper *viper.Viper) error {
	// connect db
	connStr := viper.GetString(env.DATABASE_DSN_HISTORY)
	err := a.ConnectDB(connStr)
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
	_, err := a.Db.Exec(query, "paymentMaxLookBackDays", a.Settings.PaymentMaxLookBackDays)

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
