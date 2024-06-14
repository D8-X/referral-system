package referral

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"referral-system/env"
	"referral-system/src/utils"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
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
			JOIN referral_settings rs
				ON rs.property='broker_addr'
				AND rs.broker_id=$2
			WHERE LOWER(trader_addr)=$1
				AND LOWER(rs.value) = LOWER(rafpt.broker_addr)`
	rows, err := a.Db.Query(query, traderAddr, a.Settings.BrokerId)
	if err != nil {
		slog.Error("Error for open pay" + err.Error())
		return utils.APIResponseOpenEarnings{}, errors.New("unable to query payment")
	}
	defer rows.Close()
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
	durUntilDue := time.Until(dueTimestamp)

	go func() {
		slog.Info("Waiting for " + durUntilDue.String() + " until next payment is due...")
		time.Sleep(durUntilDue)

		// Execute
		fmt.Println("Payment is now due, executing...")
		a.ManagePayments()
	}()
}

// dbGetPayBatch returns hasFinished and if yes the
// batch number
func (a *App) dbGetPayBatch() (bool, int64) {
	query := `SELECT value FROM referral_settings rs
	WHERE rs.property='batch_finished' AND broker_id=$1`

	var hasFinishedStr string
	err := a.Db.QueryRow(query, a.Settings.BrokerId).Scan(&hasFinishedStr)
	if err == sql.ErrNoRows {
		return true, 0
	}

	var ts string
	query = `SELECT value FROM referral_settings rs
		WHERE rs.property='batch_timestamp' AND broker_id=$1`
	err = a.Db.QueryRow(query, a.Settings.BrokerId).Scan(&ts)
	if err != nil {
		return true, 0
	}
	time, _ := strconv.Atoi(ts)
	return hasFinishedStr == "true", int64(time)
}

// determineScalingFactor sums all broker fees for the current broker
// per pool token and divides the token holdings of the broker by it, s.t.
// distributable_amount * scale = available amount
// This is needed if the balance is lower than the fees earned (e.g. due to
// an error with collecting historical payments or the broker moving out funds)
func (a *App) DetermineScalingFactor() (map[uint32]float64, error) {

	query := `SELECT agfpt.pool_id, 
					sum(agfpt.broker_fee_cc) as broker_fee_cc,
					mti.token_addr, mti.token_decimals
				FROM referral_aggr_fees_per_trader agfpt
				JOIN margin_token_info mti
					ON mti.pool_id = agfpt.pool_id
				JOIN referral_settings rs
					ON rs.property='broker_addr'
					AND rs.broker_id=$1
				WHERE LOWER(agfpt.broker_addr)=LOWER(rs.value)
				GROUP BY agfpt.pool_id, mti.token_addr, mti.token_decimals`
	rows, err := a.Db.Query(query, a.Settings.BrokerId)
	if err != nil {
		return nil, errors.New("determineScalingFactor:" + err.Error())
	}
	defer rows.Close()
	scale := make(map[uint32]float64)
	for rows.Next() {
		var pool uint32
		var broker_fee_ccStr string
		var tokenAddr string
		var decimals uint8
		rows.Scan(&pool, &broker_fee_ccStr, &tokenAddr, &decimals)
		tkn, err := a.CreateErc20Instance(tokenAddr)
		if err != nil {
			slog.Error("determineScalingFactor: could not create token instance")
			scale[pool] = 1
			continue
		}
		holdingsDecN, err := a.QueryTokenBalance(tkn, a.BrokerAddr)
		if err != nil {
			slog.Error("determineScalingFactor: could not query token balance")
			scale[pool] = 1
			continue
		}

		broker_fee_cc, _ := new(big.Int).SetString(broker_fee_ccStr, 10)
		feeDecN := utils.ABDKToDecN(broker_fee_cc, decimals)
		fmt.Printf("Token holding for pool %d of broker (dec %d): %s; tot fee=%s\n", pool, decimals, holdingsDecN.String(), feeDecN.String())
		var ratio float64 = 1
		if feeDecN.Cmp(holdingsDecN) == 1 {
			ratio = utils.Ratio(holdingsDecN, feeDecN)
			fmt.Printf("Adjusted payout ratio for pool %d to %.4f\n", pool, ratio)
		}
		scale[pool] = ratio
	}
	return scale, nil
}

// ManagePayments determins how much to pay and ultimately delegates
// payment execution to payexec
func (a *App) ManagePayments() {
	// Filter blockchain events to confirm payments
	slog.Info("Reading onchain payments ...")
	var err error = nil
	for trial := 0; trial < 5; trial++ {
		if trial > 0 {
			msg := fmt.Sprintf("Retrying, waiting for %d seconds ", 60*trial)
			slog.Info(msg)
			time.Sleep(time.Duration(60*trial) * time.Second)
		}
		// switch RPC
		a.CreateRpcClient()
		err = a.SavePayments()
		if err == nil {
			break
		}
		slog.Info("Reading onchain payments failed:" + err.Error())
	}
	if err != nil {
		slog.Info("Reading onchain payments failed: rescheduling payments")
		a.SchedulePayment()
		return
	}
	slog.Info("Reading onchain payments completed")
	// Create a token bucket with a limit of 5 tokens and a refill rate of 3 tokens per second
	a.PaymentExecutor.NewTokenBucket(5, 3)
	// determine batch timestamp
	var batchTs string
	err = nil
	hasFinished, batchTime := a.dbGetPayBatch()
	if !hasFinished {
		// continue payment execution
		batchTs = fmt.Sprintf("%d", batchTime)
		slog.Info("Continuing aborted payments for batch " + batchTs)
		err = a.processPayments(batchTs)
	} else if a.isPaymentDue(batchTime) {

		// register intend to start
		// payment batch in database
		currentTime := time.Now().Unix()
		batchTs = fmt.Sprintf("%d", currentTime)
		slog.Info("Payment is due, batch " + batchTs)
		err = a.DbSetPaymentExecFinished(batchTs, false)
		if err == nil {
			err = a.processPayments(batchTs)
		}
	} else {
		slog.Info("No payment due, scheduling next payment")
	}
	if err != nil {
		slog.Error("Error processing payments:" + err.Error())
	}
	// schedule next payments
	a.SchedulePayment()
}

func (a *App) isPaymentDue(batchTime int64) bool {
	ts := time.Unix(batchTime, 0)
	prevTime := utils.PrevPaymentSchedule(a.Settings.PayCronSchedule)
	slog.Info("Cron schedule:" + a.Settings.PayCronSchedule)
	slog.Info("Last payment due time: " + prevTime.Format("2006-01-02 15:04:05"))
	slog.Info("Last payment batch execution time: " + ts.Format("2006-01-02 15:04:05"))

	return prevTime.After(ts)
}

func (a *App) processPayments(batchTs string) error {
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
			  	ON mti.pool_id = agfpt.pool_id
			  JOIN referral_settings rs
			  	ON rs.property='broker_addr'
			  	AND rs.broker_id = $1
			  WHERE LOWER(agfpt.broker_addr)=LOWER(rs.value)`
	rows, err := a.Db.Query(query, a.Settings.BrokerId)
	if err != nil {
		slog.Error("Error for process pay" + err.Error())
		return err
	}
	defer rows.Close()
	// in case we have less balance than fee earnings,
	// fee redistribution must be scaled
	scale, err := a.DetermineScalingFactor()
	if err != nil {
		slog.Error("Error for process pay" + err.Error())
		return err
	}
	codePaths := make(map[string][]DbReferralChainOfChild)
	for rows.Next() {
		var el AggregatedFeesRow
		var fee string
		rows.Scan(&el.PoolId, &el.TraderAddr, &el.Code, &fee,
			&el.LastTradeConsidered, &el.TokenAddr, &el.TokenDecimals)
		el.BrokerFeeABDKCC = new(big.Int)
		el.BrokerFeeABDKCC.SetString(fee, 10)
		fmt.Println("fee=", el.BrokerFeeABDKCC)

		// determine referralchain for the code
		if _, exists := codePaths[el.Code]; !exists {
			chain, err := a.DbGetReferralChainForCode(el.Code)
			if err != nil {
				slog.Error("could not find referral chain for code " + el.Code + ": " + err.Error())
				continue
			}
			codePaths[el.Code] = chain
		}
		// process
		scalingFactor := scale[el.PoolId]
		err = a.payBatch(el, codePaths[el.Code], batchTs, scalingFactor)
		if err != nil {
			slog.Info("aborting payments...")
			break
		}
	}
	err = a.DbSetPaymentExecFinished(batchTs, true)
	if err != nil {
		slog.Error("could not set payment status to finished, but finished:" + err.Error())
	}
	slog.Info("Payment execution done, waiting before confirming payments...")
	// wait before we aim to confirm payments
	time.Sleep(2 * time.Minute)
	// Filter blockchain events to confirm payments
	slog.Info("Confirming payments")
	a.ConfirmPaymentTxs()
	return nil
}

func (a *App) payBatch(row AggregatedFeesRow, chain []DbReferralChainOfChild, batchTs string, scaling float64) error {
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
	payees[1] = a.Settings.BrokerPayoutAddr
	// rounding down typically leads to totalDecN<distributed
	if distributed.Cmp(totalDecN) < 0 {
		totalDecN = distributed
	}
	// encode message: batchTs.<code>.<poolId>.<encodingversion>
	msg := encodePaymentInfo(batchTs, row.Code, int(row.PoolId))
	// id = lastTradeConsideredTs in seconds
	id := row.LastTradeConsidered.Unix()
	// switch rpc
	err := a.CreateRpcClient()
	if err != nil {
		slog.Info("Could not switch rpc client, ignoring")
	}
	a.PaymentExecutor.SetClient(a.RpcClient)

	txHash, err := a.PaymentExecutor.TransactPayment(common.HexToAddress(row.TokenAddr), totalDecN, amounts, payees, id, msg, row.Code, a.RpcClient)
	if err != nil {
		slog.Error(err.Error())
		if strings.Contains(err.Error(), "insufficient funds") {
			return err
		} else {
			return nil
		}
	}
	_, err = waitForReceipt(a.RpcClient, txHash)
	if err != nil {
		slog.Info("Could not wait for receipt:" + err.Error())
	}

	a.dbWriteTx(row.TraderAddr, row.Code, amounts, payees, batchTs, row.PoolId, txHash.Hex())
	return nil
}

func waitForReceipt(client *ethclient.Client, txHash common.Hash) (*types.Receipt, error) {
	ctx := context.Background()
	for {
		receipt, err := client.TransactionReceipt(ctx, txHash)
		if err != nil {
			return nil, err
		}
		if receipt != nil {
			return receipt, nil
		}
		time.Sleep(2 * time.Second)
	}
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
	query := `SELECT LOWER(referrer_addr) as addr, trader_rebate_perc
		FROM referral_code WHERE code = $1 AND broker_id = $2`
	var refAddr string
	var traderCut float64
	err := a.Db.QueryRow(query, code, a.Settings.BrokerId).Scan(&refAddr, &traderCut)
	if err != nil {
		return []DbReferralChainOfChild{}, errors.New("DbGetReferralChainForCode:" + err.Error())
	}
	traderCut = traderCut / 100

	chain, _, err := a.DbGetReferralChainFromChild(refAddr, nil)
	if err != nil {
		return []DbReferralChainOfChild{}, errors.New("DbGetReferralChainForCode:" + err.Error())
	}
	var crumble float64
	if len(chain) == 0 {
		// the broker is the one who distributed the code
		crumble = 1
	} else {
		crumble = chain[len(chain)-1].ChildAvail
	}
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
