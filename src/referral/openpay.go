package referral

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"referral-system/src/utils"
	"strconv"
	"time"
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

func (a *App) SchedulePayment() {
	// Define the timestamp when the task is due (replace with your timestamp)
	dueTimestamp := utils.NextPaymentSchedule(a.Settings.PayCronSchedule)
	durUntilDue := dueTimestamp.Sub(time.Now())

	go func() {
		slog.Info("Waiting for " + durUntilDue.String() + " until next payment is due...")
		time.Sleep(durUntilDue)

		// Execute
		fmt.Println("Payment is now due, executing...")
		err := a.ProcessAllPayments(true)
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
	slog.Info("Last payment due time: " + prevTime.Format("2006-01-02 15:04:05"))
	slog.Info("Last payment batch execution time: " + ts.Format("2006-01-02 15:04:05"))

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
				join referral_settings rs
				on LOWER(rs.value) = LOWER(agfpt.broker_addr)
				and rs.property='broker_addr'
				group by agfpt.pool_id, mti.token_addr, mti.token_decimals`
	rows, err := a.Db.Query(query)
	defer rows.Close()
	if err != nil {
		return nil, errors.New("determineScalingFactor:" + err.Error())
	}
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
		var ratio float64 = 1
		if feeDecN.Cmp(holdingsDecN) == 1 {
			ratio = utils.Ratio(holdingsDecN, feeDecN)
		}
		scale[pool] = ratio
	}
	return scale, nil
}

// ProcessAllPayments determins how much to pay and ultimately delegates
// payment execution to payexec
func (a *App) ProcessAllPayments(filterPayments bool) error {
	// Filter blockchain events to confirm payments
	if filterPayments {
		a.SavePayments()
		slog.Info("Historical payment filtering done")
	}
	// Create a token bucket with a limit of 5 tokens and a refill rate of 3 tokens per second
	a.PaymentExecutor.NewTokenBucket(5, 3)
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
				ON mti.pool_id = agfpt.pool_id
				join referral_settings rs
				on LOWER(rs.value) = LOWER(agfpt.broker_addr)
				and rs.property='broker_addr'`
	rows, err := a.Db.Query(query)
	defer rows.Close()
	if err != nil {
		slog.Error("Error for process pay" + err.Error())
		return err
	}
	// in case we have less balance than fee earnings,
	// fee redistribution must be scaled
	scale, err := a.DetermineScalingFactor()
	if err != nil {
		slog.Error("Error for process pay" + err.Error())
		return err
	}
	a.RS.ProcessPayments(a, rows, scale, batchTs)
	err = a.DbSetPaymentExecFinished(batchTs, true)
	if err != nil {
		slog.Error("Could not set payment status to finished, but finished:" + err.Error())
	}
	slog.Info("Payment execution done, waiting before confirming payments...")
	// wait before we aim to confirm payments
	time.Sleep(2 * time.Minute)
	// Filter blockchain events to confirm payments
	slog.Info("Confirming payments")
	a.ConfirmPaymentTxs()

	// schedule next payments
	a.SchedulePayment()
	return nil
}
