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
	"referral-system/src/utils"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

const CODE_SYS_TYPE = "CodeSystem"

type CodeSystem struct {
	Config     CodeSysConf
	Db         *sql.DB
	BrokerAddr string
}

type CodeSysConf struct {
	ChainId                int    `json:"chainId"`
	PaymentMaxLookBackDays int    `json:"paymentMaxLookBackDays"`
	PayCronSchedule        string `json:"paymentScheduleCron"`
	MultiPayContractAddr   string `json:"multiPayContractAddr"`
	TokenX                 struct {
		Address  string `json:"address"`
		Decimals uint8  `json:"decimals"`
	} `json:"tokenX"`
	ReferrerCut      [][]float64    `json:"referrerCutPercentForTokenXHolding"`
	BrokerPayoutAddr common.Address `json:"brokerPayoutAddr"`
}

func (c *CodeSystem) GetType() string {
	return CODE_SYS_TYPE
}

func (rs *CodeSystem) SetDb(db *sql.DB) {
	rs.Db = db
}

func (rs *CodeSystem) GetDb() *sql.DB {
	return rs.Db
}

func (rs *CodeSystem) SetBrokerAddr(addr string) {
	rs.BrokerAddr = addr
}
func (rs *CodeSystem) GetBrokerAddr() string {
	return rs.BrokerAddr
}

// loadCodeSysConf loads the configuration file from the file system
// and returns the CodeSysConf struct
func (rs *CodeSystem) LoadConfig(fileName string, chainId int) error {
	var settings []CodeSysConf
	data, err := os.ReadFile(fileName)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &settings)
	if err != nil {
		return err
	}
	// pick correct setting by chain id
	var setting CodeSysConf = CodeSysConf{}

	for k := 0; k < len(settings); k++ {
		if settings[k].ChainId == chainId {
			setting = settings[k]
			break
		}
	}
	if setting.ChainId != chainId {
		return errors.New("No setting found for chain id " + strconv.Itoa(chainId))
	}
	setting.TokenX.Address = strings.ToLower(setting.TokenX.Address)
	rs.Config = setting
	return nil
}

func (rs *CodeSystem) GetCronSchedule() string {
	return rs.Config.PayCronSchedule
}

func (rs *CodeSystem) GetMultiPayAddr() string {
	return rs.Config.MultiPayContractAddr
}

func (rs *CodeSystem) GetPaymentLookBackDays() int {
	return rs.Config.PaymentMaxLookBackDays
}

func (rs *CodeSystem) SettingsToDb() error {
	// referral cut based on token holdings
	dec := rs.Config.TokenX.Decimals
	tkn := rs.Config.TokenX.Address

	query := `DELETE FROM referral_setting_cut;`
	_, err := rs.Db.Exec(query)
	if err != nil {
		slog.Error(err.Error())
	}

	for k := 0; k < len(rs.Config.ReferrerCut); k++ {
		perc := rs.Config.ReferrerCut[k][0]
		holding := rs.Config.ReferrerCut[k][1]
		holdingDecN := utils.FloatToDecN(holding, dec)
		query = `
			INSERT INTO referral_setting_cut (cut_perc, holding_amount_dec_n, token_addr)
			VALUES ($1, $2, $3)`
		_, err = rs.Db.Exec(query, perc, holdingDecN.String(), tkn)
		if err != nil {
			slog.Error("could not insert referral setting:" + err.Error())
			continue
		}
	}
	return nil
}

// IsAgency returns true if the address is either the broker,
// or a child in the referral chain (hence an agency)
func (rs *CodeSystem) IsAgency(addr string) bool {
	query := `SELECT LOWER(child) from referral_chain WHERE LOWER(child)=$1
		UNION SELECT value as child from referral_settings WHERE property='broker_addr' AND LOWER(value)=$1`
	var dbAddr string
	err := rs.Db.QueryRow(query, addr).Scan(&dbAddr)
	return err != sql.ErrNoRows
}

// Cut percentage returns how much % (1% is represented as 1) of the broker fees
// trickle down to this agency or referrer address
func (rs *CodeSystem) CutPercentageAgency(addr string, holdingsDecN *big.Int) (float64, bool, error) {
	addr = strings.ToLower(addr)
	chain, isAg, err := rs.dbGetReferralChainFromChild(addr, holdingsDecN)
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

// HasLoopOnChainAddition returns true if when adding the new child to the
// referral chain, there would be a loop
func (rs *CodeSystem) HasLoopOnChainAddition(parent string, newChild string) (bool, error) {

	h := new(big.Int).SetInt64(0)
	chain, _, err := rs.dbGetReferralChainFromChild(parent, h)
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

// Refer handles new referral requests (checks and insert into db)
func (rs *CodeSystem) Refer(rpl utils.APIReferPayload) error {
	var passOn float32 = float32(rpl.PassOnPercTDF) / 100.0
	rpl.ParentAddr = strings.ToLower(rpl.ParentAddr)
	// parent can only refer if they are the broker or a child
	if !rs.IsAgency(rpl.ParentAddr) {
		return errors.New("not an agency")
	}
	rpl.ReferToAddr = strings.ToLower(rpl.ReferToAddr)
	h, err := rs.HasLoopOnChainAddition(rpl.ParentAddr, rpl.ReferToAddr)
	if err != nil {
		slog.Error("HasLoopOnChainAddition failed")
		return errors.New("failed")
	}
	if h {
		return errors.New("referral already in chain")
	}
	query := "SELECT child from referral_chain WHERE LOWER(child)=$1"
	var addr string
	err = rs.Db.QueryRow(query, rpl.ReferToAddr).Scan(&addr)
	if err != sql.ErrNoRows {
		return errors.New("refer to addr already in use")
	}
	// referral chain length
	chain, _, err := rs.dbGetReferralChainFromChild(rpl.ParentAddr, big.NewInt(0))
	if err == nil && len(chain) > env.MAX_REFERRAL_CHAIN_LEN {
		slog.Info("Max referral chain length reached for " + rpl.ParentAddr)
		return errors.New("reached maximum number of referrals")
	}
	// now safe to insert
	query = `INSERT INTO referral_chain (parent, child, pass_on)
          	 VALUES ($1, $2, $3)`
	_, err = rs.Db.Exec(query, rpl.ParentAddr, rpl.ReferToAddr, passOn)
	if err != nil {
		slog.Error("Failed to insert referral" + err.Error())
		return errors.New("failed to insert referral")
	}
	return nil
}

// CutPercentageCode calculates the percent (1% -> 1) rebate on broker trading fees,
// when selecting this code
// Code has to be "cleaned" outside this function
func (rs *CodeSystem) CutPercentageCode(code string) (float64, error) {
	query := `SELECT referrer_addr, trader_rebate_perc FROM
		referral_code WHERE code=$1`
	var refAddr string
	var traderCut float64
	err := rs.Db.QueryRow(query, code).Scan(&refAddr, &traderCut)
	if err != nil {
		//no log
		return 0, errors.New("could not identify code")
	}

	h := new(big.Int).SetInt64(0)
	passOnCut, _, err := rs.CutPercentageAgency(refAddr, h)
	if err != nil {
		slog.Error("Error for CutPercentageCode code " + code)
		return 0, errors.New("could not identify cut")
	}
	return passOnCut * traderCut / 100, nil
}

// dbGetReferralChainFromChild returns the percentage of trader
// fees earned by an agency.
// Holdings are relevant for pure referrers only. The fees for pure referrers
// are calculated conditionally to this number
func (rs *CodeSystem) dbGetReferralChainFromChild(child string, holdings *big.Int) ([]DbReferralChainOfChild, bool, error) {
	child = strings.ToLower(child)
	var chain []DbReferralChainOfChild
	isAg := rs.IsAgency(child)
	if isAg {
		var row DbReferralChainOfChild
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
		rows, err := rs.Db.Query(query)
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

	// referrer without agency, we calculate the rebate based
	// on token holdings
	query := `SELECT MAX(cut_perc) as max_cut
			FROM referral_setting_cut rsc
			LEFT join referral_token_holdings rth 
			on lower(rth.referrer_addr) = $1
			WHERE LOWER(rsc.token_addr)= $2
			AND rsc.holding_amount_dec_n<=coalesce($3, rth.holding_amount_dec_n)`
	var cut float64
	err := rs.Db.QueryRow(query, child, rs.Config.TokenX.Address, holdings.String()).Scan(&cut)
	if err != nil {
		slog.Error("Error for CutPercentageAgency address " + child)
		return []DbReferralChainOfChild{}, isAg, errors.New("could not get percentage")
	}
	cut = cut / 100
	var el = DbReferralChainOfChild{
		Parent:     rs.GetBrokerAddr(),
		Child:      child,
		PassOn:     cut,
		ParentPay:  1.0 - cut,
		ChildAvail: cut,
		Lvl:        1,
	}
	chain = append(chain, el)

	return chain, isAg, nil
}

// OpenPay determines how much the given trader gets paid back
// from his trading activity
func (rs *CodeSystem) OpenPay(rows *sql.Rows, app *App, traderAddr string) (utils.APIResponseOpenEarnings, error) {
	var payments []utils.OpenPay
	var codeChain DbReferralChainOfChild
	var res utils.APIResponseOpenEarnings
	for rows.Next() {
		var el AggrFeesOpenPay
		rows.Scan(&el.PoolId, &el.Code, &el.BrokerFeeCc,
			&el.LastTradeConsidered, &el.TokenName, &el.TokenDecimals)
		if el.Code == env.DEFAULT_CODE {
			// no code, hence no rebate
			var op = utils.OpenPay{
				PoolId:    el.PoolId,
				Amount:    0,
				TokenName: el.TokenName,
			}
			payments = append(payments, op)
			continue
		}
		if res.Code != el.Code {
			chain, err := rs.DbGetReferralChainForCode(el.Code)
			if err != nil {
				slog.Error("error in OpenPay" + err.Error())
				return utils.APIResponseOpenEarnings{}, errors.New("unable to query payment")
			}
			codeChain = chain[len(chain)-1]
			res.Code = el.Code
		}
		fee := new(big.Int)
		fee.SetString(el.BrokerFeeCc, 10)
		amount := utils.ABDKToFloat(fee)
		amount = amount * codeChain.ChildAvail
		var op = utils.OpenPay{
			PoolId:    el.PoolId,
			Amount:    amount,
			TokenName: el.TokenName,
		}
		payments = append(payments, op)
	}
	res.OpenPay = payments
	return res, nil
}

func (rs *CodeSystem) ProcessPayments(app *App, scale map[uint32]float64, batchTs string) error {
	// query snapshot of open pay view
	query := `SELECT agfpt.pool_id, agfpt.trader_addr, agfpt.code, 
		agfpt.broker_fee_cc, agfpt.last_trade_considered_ts,
		mti.token_addr, mti.token_decimals
	FROM referral_aggr_fees_per_trader agfpt
	JOIN margin_token_info mti
	ON mti.pool_id = agfpt.pool_id
	join referral_settings rs
	on LOWER(rs.value) = LOWER(agfpt.broker_addr)
	and rs.property='broker_addr'`
	rows, err := rs.GetDb().Query(query)
	if err != nil {
		slog.Error("Error for process pay" + err.Error())
		return err
	}
	defer rows.Close()

	codePaths := make(map[string][]DbReferralChainOfChild)
	for rows.Next() {
		var el AggregatedFeesRow
		var fee string
		rows.Scan(&el.PoolId, &el.TraderAddr, &el.Code, &fee,
			&el.LastTradeConsidered, &el.TokenAddr, &el.TokenDecimals)
		el.BrokerFeeABDKCC = new(big.Int)
		el.BrokerFeeABDKCC.SetString(fee, 10)
		fmt.Println("fee=", el.BrokerFeeABDKCC)
		if fee == "" {
			fmt.Println("no fee, continuing")
			continue
		}
		// determine referralchain for the code
		if _, exists := codePaths[el.Code]; !exists {
			chain, err := rs.DbGetReferralChainForCode(el.Code)
			if err != nil {
				slog.Error("Could not find referral chain for code " + el.Code + ": " + err.Error())
				continue
			}
			codePaths[el.Code] = chain
		}
		// process
		scalingFactor := scale[el.PoolId]
		err := rs.processCodePaymentRow(app, el, codePaths[el.Code], batchTs, scalingFactor)
		if err != nil {
			slog.Info("aborting payments...")
			break
		}
	}
	return nil
}

func (rs *CodeSystem) processCodePaymentRow(app *App, row AggregatedFeesRow, chain []DbReferralChainOfChild, batchTs string, scaling float64) error {
	totalDecN := utils.ABDKToDecN(row.BrokerFeeABDKCC, row.TokenDecimals)
	// scale
	if scaling < 1 {
		totalDecN = utils.DecNTimesFloat(totalDecN, scaling, 18)
		msg := fmt.Sprintf("Scaling payment amount by %.2f", scaling)
		slog.Info(msg)
	}

	payees := make([]common.Address, len(chain)+1)
	amounts := make([]*big.Int, len(chain)+1)
	// order: trader, broker, [agent1, agent2, ...], referrer
	// trader address must go first
	precision := 6
	payees[0] = common.HexToAddress(row.TraderAddr)
	amounts[0] = utils.DecNTimesFloat(totalDecN, chain[len(chain)-1].ChildAvail, precision)
	distributed := new(big.Int).Set(amounts[0])
	for k := 0; k < len(chain); k++ {
		el := chain[k]
		amount := utils.DecNTimesFloat(totalDecN, el.ParentPay, precision)
		amounts[k+1] = amount
		payees[k+1] = common.HexToAddress(el.Parent)
		distributed.Add(distributed, amount)
	}
	// parent amount goes to broker payout address
	payees[1] = rs.Config.BrokerPayoutAddr
	// rounding down typically leads to totalDecN<distributed
	if distributed.Cmp(totalDecN) < 0 {
		totalDecN = distributed
	}
	// encode message: batchTs.<code>.<poolId>.<encodingversion>
	msg := EncodePaymentInfo(batchTs, row.Code, int(row.PoolId))
	// id = lastTradeConsideredTs in seconds
	id := row.LastTradeConsidered.Unix()
	app.PaymentExecutor.SetClient(app.RpcClient)

	txHash, err := app.PaymentExecutor.TransactPayment(common.HexToAddress(row.TokenAddr), totalDecN, amounts, payees, id, msg, row.Code, app.RpcClient)
	if err != nil {
		slog.Error(err.Error())
		if strings.Contains(err.Error(), "insufficient funds") {
			return err
		} else {
			return nil
		}
	}
	app.DbWriteTx(row.TraderAddr, row.Code, amounts, payees, batchTs, row.PoolId, txHash)
	return nil
}

// DbGetReferralChainForCode gets the entire chain of referrals
// for a code, calculating what each participant earns (percent)
func (rs *CodeSystem) DbGetReferralChainForCode(code string) ([]DbReferralChainOfChild, error) {
	if code == env.DEFAULT_CODE {
		res := make([]DbReferralChainOfChild, 1)
		res[0] = DbReferralChainOfChild{
			Parent:     strings.ToLower(rs.Config.BrokerPayoutAddr.Hex()),
			Child:      "DEFAULT",
			PassOn:     0,
			ParentPay:  1,
			ChildAvail: 0,
			Lvl:        0,
		}
		return res, nil
	}
	query := `SELECT LOWER(referrer_addr), trader_rebate_perc
		FROM referral_code WHERE code = $1`
	var refAddr string
	var traderCut float64
	err := rs.GetDb().QueryRow(query, code).Scan(&refAddr, &traderCut)
	if err != nil {
		return []DbReferralChainOfChild{}, errors.New("DbGetReferralChainForCode:" + err.Error())
	}
	traderCut = traderCut / 100

	h := new(big.Int).SetInt64(0)
	chain, _, err := rs.dbGetReferralChainFromChild(refAddr, h)
	if err != nil {
		return []DbReferralChainOfChild{}, errors.New("DbGetReferralChainForCode:" + err.Error())
	}
	crumble := chain[len(chain)-1].ChildAvail
	codeUser := DbReferralChainOfChild{
		Parent:     refAddr,
		Child:      code,
		PassOn:     traderCut,
		ParentPay:  crumble * (1 - traderCut),
		ChildAvail: crumble * traderCut,
		Lvl:        0,
	}
	chain = append(chain, codeUser)
	return chain, nil
}

func (rs *CodeSystem) DbGetMyReferrals(addr string) ([]utils.APIResponseMyReferrals, error) {
	// as agency select downstream partners and pass-on
	query := `select child, pass_on as pass_on_perc
			   from referral_chain rc 
			  where lower(rc.parent) = $1
				union -- as referrer:
			  select code as child, trader_rebate_perc as pass_on_perc
				from referral_code 
				where lower(referrer_addr) = $1`
	rows, err := rs.Db.Query(query, addr)
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

func (rs *CodeSystem) DbGetMyCodeSelection(addr string) (string, error) {
	query := `select code
			  from referral_code_usage rcu 
			  where lower(rcu.trader_addr) = $1
			  and valid_to>NOW() and valid_from<NOW()`
	var code string
	err := rs.Db.QueryRow(query, addr).Scan(&code)
	if err == sql.ErrNoRows {
		return "", nil
	} else if err != nil {
		slog.Error("DbMyCodeSelection failed:" + err.Error())
		return "", errors.New("code retrieval failed")
	}
	return code, nil
}

// DbGetActiveReferrers returns a list of addresses that are
// (1) not an agency, (2) have a code which is used in the
// view referral_aggr_fees_per_trader. The func also returns the last
// token balance update time
func (rs *CodeSystem) DbGetActiveReferrers() ([]string, []time.Time, error) {
	query := `select distinct(lower(rc.referrer_addr)), rth.last_updated 
			  from referral_aggr_fees_per_trader rafpt 
			  join referral_code rc 
				on rc.code = rafpt.code
				and LOWER(rc.referrer_addr) not in (select LOWER(rc2.child) from referral_chain rc2)
			  left join referral_token_holdings rth 
				on LOWER(rth.referrer_addr) = LOWER(rc.referrer_addr)`
	rows, err := rs.Db.Query(query)
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

func (rs *CodeSystem) PreProcessPayments(rpc *ethclient.Client) error {
	return rs.dbUpdateTokenHoldings(rpc)
}

// DbUpdateTokenHoldings queries balances of TokenX from the blockchain
// and updates the holdings for active referrers in the database
func (rs *CodeSystem) dbUpdateTokenHoldings(rpc *ethclient.Client) error {
	// select referrers that are no agency (not in referral chain)
	refAddr, lastUpdate, err := rs.DbGetActiveReferrers()
	if err != nil {
		return err
	}
	tkn, err := CreateErc20Instance(rs.Config.TokenX.Address, rpc)
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

		holdings, err := QueryTokenBalance(tkn, currReferrerAddr)
		if err != nil {
			slog.Error("Error when trying to get token balance:" + err.Error())
			continue
		}
		query := `
		INSERT INTO referral_token_holdings (referrer_addr, holding_amount_dec_n, token_addr)
		VALUES ($1, $2, $3)
		ON CONFLICT (referrer_addr, token_addr) DO UPDATE SET holding_amount_dec_n = EXCLUDED.holding_amount_dec_n`
		_, err = rs.Db.Exec(query, currReferrerAddr, holdings.String(), rs.Config.TokenX.Address)
		if err != nil {
			slog.Error("Error when trying to upsert token balance:" + err.Error())
			continue
		}
	}

	return nil
}

func (rs *CodeSystem) DbGetTokenInfo() (utils.APIResponseTokenHoldings, error) {
	query := `select cut_perc, holding_amount_dec_n/power(10, $1) as holding, token_addr from referral_setting_cut rsc`
	rows, err := rs.Db.Query(query, rs.Config.TokenX.Decimals)
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

// SelectCode tries to select a given code for a trader. Future trades will
// be using this code.
// Signature must have been checked
// before. The error message returned (if any) is exposed to the API
func (rs *CodeSystem) SelectCode(csp utils.APICodeSelectionPayload) error {
	csp.TraderAddr = strings.ToLower(csp.TraderAddr)
	timeNow := time.Now().Unix()
	// code exists?
	query := `SELECT expiry
		FROM referral_code
		where code=$1`
	var ts time.Time
	err := rs.Db.QueryRow(query, csp.Code).Scan(&ts)
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
		ORDER BY valid_to DESC
		LIMIT 1`
	err = rs.Db.QueryRow(query, csp.TraderAddr).Scan(&latestCode.TraderAddr, &latestCode.Code, &latestCode.ValidFrom, &latestCode.ValidTo)
	if err != sql.ErrNoRows && err != nil {
		slog.Info("Failed to query latest code:" + err.Error())
		return errors.New("Failed")
	}

	if latestCode.Code != "" {
		if latestCode.Code == csp.Code {
			return errors.New("code already selected")
		}
		// update valid to of old code
		query = `UPDATE referral_code_usage
		SET valid_to=to_timestamp($1)
		WHERE LOWER(trader_addr)=$2
			AND code=$3
			AND valid_to=$4`
		_, err := rs.Db.Exec(query, timeNow, csp.TraderAddr, latestCode.Code, latestCode.ValidTo)
		if err != nil {
			slog.Error("Failed to insert data: " + err.Error())
			return errors.New("failed updating existing code")
		}
	}
	// now insert new code
	query = `INSERT INTO referral_code_usage (trader_addr, valid_from, code) VALUES ($1, to_timestamp($2), $3)`
	_, err = rs.Db.Exec(query, csp.TraderAddr, timeNow, csp.Code)
	if err != nil {
		slog.Error("Failed to insert data: " + err.Error())
		return errors.New("failed inserting new code")
	}
	return nil
}

// UpsertCode inserts new codes and updates the code rebate
func (rs *CodeSystem) UpsertCode(csp utils.APICodePayload) error {
	var passOn float32 = float32(csp.PassOnPercTDF) / 100.0
	// check whether code exists
	query := `SELECT referrer_addr 
		FROM referral_code
		WHERE code=$1`
	var refAddr string
	err := rs.Db.QueryRow(query, csp.Code).Scan(&refAddr)
	if err != sql.ErrNoRows && err != nil {
		slog.Info("Failed to query latest code:" + err.Error())
		return errors.New("Failed")
	} else if err == sql.ErrNoRows {
		// not found, we can insert
		query = `INSERT INTO referral_code (code, referrer_addr, trader_rebate_perc)
          VALUES ($1, $2, $3)`
		_, err := rs.Db.Exec(query, csp.Code, csp.ReferrerAddr, passOn)
		if err != nil {
			slog.Error("Failed to insert code" + err.Error())
			return errors.New("failed to insert code")
		}
		return nil
	}
	// found, we check whether the referral addr is correct
	if strings.EqualFold(refAddr, csp.ReferrerAddr) {
		return errors.New("not code owner")
	}
	query = `UPDATE referral_code SET trader_rebate_perc = $1
			 WHERE code = $2`
	_, err = rs.Db.Exec(query, passOn, csp.Code)
	if err != nil {
		return errors.New("Failed to insert data: " + err.Error())
	}
	return nil
}
