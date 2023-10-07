package referral

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"referral-system/env"
	"referral-system/src/utils"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type AggregatedFeesRow struct {
	PoolId              uint32
	TraderAddr          string
	Code                string
	BrokerFeeABDKCC     *big.Int
	LastTradeConsidered time.Time
	TokenAddr           string
	TokenDecimals       uint8
}

func (a *App) OpenPay(traderAddr string) (utils.APIResponseOpenEarnings, error) {
	type AggrFees struct {
		PoolId              uint32
		Code                string
		BrokerFeeCc         string
		LastTradeConsidered time.Time
		TokenName           string
		TokenDecimals       uint8
	}
	// get aggregated fees per pool and associated margin token info
	// for the given trader
	query := `SELECT 
				mti.pool_id, rafpt.code, rafpt.broker_fee_cc, 
				rafpt.last_trade_considered_ts, 
				mti.token_name, mti.token_decimals
			  FROM referral_aggr_fees_per_trader rafpt
			  JOIN margin_token_info mti
			  ON mti.pool_id = rafpt.pool_id
			  WHERE LOWER(trader_addr)=$1`
	rows, err := a.Db.Query(query, traderAddr)
	defer rows.Close()
	if err != nil {
		slog.Error("Error for open pay" + err.Error())
		return utils.APIResponseOpenEarnings{}, errors.New("unable to query payment")
	}
	var payments []utils.OpenPay
	var codeChain DbReferralChainOfChild
	var res utils.APIResponseOpenEarnings
	for rows.Next() {
		var el AggrFees
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
			chain, err := a.DbGetReferralChainForCode(el.Code)
			if err != nil {
				slog.Error("Error in OpenPay" + err.Error())
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

func (a *App) SchedulePayment() {
	// Define the timestamp when the task is due (replace with your timestamp)
	dueTimestamp := utils.NextPaymentSchedule(a.Settings.PayCronSchedule)
	durUntilDue := dueTimestamp.Sub(time.Now())

	go func() {
		slog.Info("Waiting for " + durUntilDue.String() + " until next payment is due...")
		time.Sleep(durUntilDue)

		// Execute
		fmt.Println("Payment is now due, executing...")
		err := a.ProcessAllPayments()
		if err != nil {
			slog.Error("Error when processing payments:" + err.Error())
		}
	}()
}

func (a *App) IsPaymentDue() bool {
	hasFinished, batchTime := a.dbGetPayBatch()
	if !hasFinished {
		return false
	}
	ts := time.Unix(batchTime, 0)
	prevTime := utils.PrevPaymentSchedule(a.Settings.PayCronSchedule)
	slog.Info("Last payment due  time: " + prevTime.Format("2006-01-02 15:04:05"))
	slog.Info("Last payment exec time: " + ts.Format("2006-01-02 15:04:05"))

	return prevTime.After(ts)
}

// dbGetPayBatch returns hasFinished and if yes the
// batch number
func (a *App) dbGetPayBatch() (bool, int64) {
	query := `SELECT value FROM referral_settings rs
	WHERE rs.property='batch_finished'`

	var hasFinishedStr string
	err := a.Db.QueryRow(query).Scan(&hasFinishedStr)
	if err == sql.ErrNoRows {
		return true, 0
	}

	var ts string
	query = `SELECT value FROM referral_settings rs
		WHERE rs.property='batch_timestamp'`
	err = a.Db.QueryRow(query).Scan(&ts)
	if err != nil {
		return true, 0
	}
	time, _ := strconv.Atoi(ts)
	return hasFinishedStr == "true", int64(time)
}

// ProcessAllPayments determins how much to pay and ultimately delegates
// payment execution to payexec
func (a *App) ProcessAllPayments() error {
	// determine batch timestamp
	var batchTs string

	hasFinished, batchTime := a.dbGetPayBatch()
	if !hasFinished {
		// continue payment execution
		batchTs = fmt.Sprintf("%d", batchTime)
	} else {
		// register intend to start
		// payment batch in database
		currentTime := time.Now().Unix()
		batchTs = fmt.Sprintf("%d", currentTime)
		err := a.DbSetPaymentExecFinished(batchTs, false)
		if err != nil {
			return err
		}
	}

	// update token holdings
	err := a.DbUpdateTokenHoldings()
	if err != nil {
		return errors.New("ProcessAllPayments: Failed to update token holdings " + err.Error())
	}
	// query snapshot of open pay view
	query := `SELECT agfpt.pool_id, agfpt.trader_addr, agfpt.code, 
				agfpt.broker_fee_cc, agfpt.last_trade_considered_ts,
				mti.token_addr, mti.token_decimals
			  FROM referral_aggr_fees_per_trader agfpt
			  JOIN margin_token_info mti
			  ON mti.pool_id = agfpt.pool_id`
	rows, err := a.Db.Query(query)
	defer rows.Close()
	if err != nil {
		slog.Error("Error for process pay" + err.Error())
		return err
	}
	var aggrFeesPerTrader []AggregatedFeesRow
	codePaths := make(map[string][]DbReferralChainOfChild)
	for rows.Next() {
		var el AggregatedFeesRow
		var fee string
		rows.Scan(&el.PoolId, &el.TraderAddr, &el.Code, &fee,
			&el.LastTradeConsidered, &el.TokenAddr, &el.TokenDecimals)
		el.BrokerFeeABDKCC = new(big.Int)
		el.BrokerFeeABDKCC.SetString(fee, 10)
		aggrFeesPerTrader = append(aggrFeesPerTrader, el)

		// determine referralchain for the code
		if _, exists := codePaths[el.Code]; !exists {
			chain, err := a.DbGetReferralChainForCode(el.Code)
			if err != nil {
				slog.Error("Could not find referral chain for code " + el.Code + ": " + err.Error())
				continue
			}
			codePaths[el.Code] = chain
		}
		// process
		a.processPayment(el, codePaths[el.Code], batchTs)
	}
	err = a.DbSetPaymentExecFinished(batchTs, true)
	if err != nil {
		slog.Error("Could not set payment status to finished, but finished:" + err.Error())
	}
	// schedule next payments
	a.SchedulePayment()
	return nil
}

func (a *App) processPayment(row AggregatedFeesRow, chain []DbReferralChainOfChild, batchTs string) {
	totalDecN := utils.ABDKToDecN(row.BrokerFeeABDKCC, row.TokenDecimals)
	payees := make([]common.Address, len(chain)+1)
	amounts := make([]*big.Int, len(chain)+1)
	// order: trader, broker, [agent1, agent2, ...], referrer
	// trader address must go first
	payees[0] = common.HexToAddress(row.TraderAddr)
	amounts[0] = utils.DecNTimesFloat(totalDecN, chain[len(chain)-1].ChildAvail)
	distributed := new(big.Int).Set(amounts[0])
	// we start at 1 (after broker), to set the broker amount to the
	// remainder (to avoid floating point rounding issues)
	for k := 1; k < len(chain); k++ {
		el := chain[k]
		amount := utils.DecNTimesFloat(totalDecN, el.ParentPay)
		amounts[k+1] = amount
		payees[k+1] = common.HexToAddress(el.Parent)
		distributed.Add(distributed, amount)
	}
	// parent amount goes to broker payout address
	payees[1] = a.Settings.BrokerPayoutAddr
	// amount for parent is set to remainder so that we ensure
	// totalDecN = sum of distributed amounts
	amounts[1] = new(big.Int).Sub(totalDecN, distributed)

	// encode message: batchTs.<code>.<poolId>
	msg := batchTs + "." + row.Code + "." + strconv.Itoa(int(row.PoolId))
	// id = lastTradeConsideredTs in seconds
	id := row.LastTradeConsidered.Unix()
	a.PaymentExecutor.TransactPayment(common.HexToAddress(row.TokenAddr), amounts, payees, id, msg)
}

// DbGetReferralChainForCode gets the entire chain of referrals
// for a code, calculating what each participant earns (percent)
func (a *App) DbGetReferralChainForCode(code string) ([]DbReferralChainOfChild, error) {
	if code == env.DEFAULT_CODE {
		res := make([]DbReferralChainOfChild, 1)
		res[0] = DbReferralChainOfChild{
			Parent:     a.Settings.BrokerPayoutAddr.String(),
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
	err := a.Db.QueryRow(query, code).Scan(&refAddr, &traderCut)
	if err != nil {
		return []DbReferralChainOfChild{}, errors.New("DbGetReferralChainForCode:" + err.Error())
	}
	traderCut = traderCut / 100
	chain, _, err := a.DbGetReferralChainFromChild(refAddr)
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
