package referral

import (
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
			var op = utils.OpenPay{
				PoolId:    el.PoolId,
				Amount:    0,
				TokenName: el.TokenName,
			}
			payments = append(payments, op)
			continue
		}
		if codeChain.Child == "" {
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
		amount = amount * codeChain.ChildPay
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

// ProcessAllPayments determins how much to pay and ultimately delegates
// payment execution to payexec
func (a *App) ProcessAllPayments() error {
	// update token holdings
	err := a.DbUpdateTokenHoldings()
	if err != nil {
		return errors.New("ProcessAllPayments: Failed to update token holdings " + err.Error())
	}
	// query snapshot of open pay view
	currentTime := time.Now().Unix()
	batchTs := fmt.Sprintf("%d", currentTime)
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
	return nil
}

func (a *App) processPayment(row AggregatedFeesRow, chain []DbReferralChainOfChild, batchTs string) {
	totalDecN := utils.ABDKToDecN(row.BrokerFeeABDKCC, row.TokenDecimals)
	payees := make([]common.Address, len(chain))
	amounts := make([]*big.Int, len(chain))
	// trader address must go first
	payees[0] = common.HexToAddress(row.TraderAddr)
	distributed := new(big.Int).SetInt64(0)
	for k := len(chain) - 1; k > 0; k-- {
		el := chain[k]
		amount := utils.DecNTimesFloat(totalDecN, el.ChildPay)
		idx := len(chain) - 1 - k
		amounts[idx] = amount
		if idx > 0 {
			payees[idx] = common.HexToAddress(el.Child)
		}
		distributed.Add(distributed, amount)
	}
	// parent
	amount := new(big.Int)
	amount.Sub(totalDecN, distributed)
	payees[len(payees)-1] = a.Settings.BrokerPayoutAddr
	amounts[len(payees)-1] = amount
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
			Parent:   a.Settings.BrokerPayoutAddr.String(),
			Child:    "DEFAULT",
			PassOn:   0,
			Lvl:      0,
			ChildPay: 0,
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
	chain, err := a.DbGetReferralChainFromChild(refAddr)
	if err != nil {
		return []DbReferralChainOfChild{}, errors.New("DbGetReferralChainForCode:" + err.Error())
	}
	crumble := chain[len(chain)-1].ChildPay
	codeUser := DbReferralChainOfChild{
		Parent:   refAddr,
		Child:    code,
		PassOn:   float32(traderCut),
		Lvl:      0,
		ChildPay: crumble * traderCut / 100,
	}
	chain = append(chain, codeUser)
	return chain, nil
}
