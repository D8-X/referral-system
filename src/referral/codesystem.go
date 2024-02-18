package referral

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"referral-system/env"
	"referral-system/src/utils"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type CodeSystem struct{}

func (rs CodeSystem) OpenPay(app *App, traderAddr string) (utils.APIResponseOpenEarnings, error) {
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
			join referral_settings rs
			on LOWER(rs.value) = LOWER(rafpt.broker_addr)
			and rs.property='broker_addr'
			WHERE LOWER(trader_addr)=$1`
	rows, err := app.Db.Query(query, traderAddr)
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
			chain, err := rs.DbGetReferralChainForCode(app, el.Code)
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

func (rs CodeSystem) ProcessPayments(app *App, rows *sql.Rows, scale map[uint32]float64, batchTs string) {
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
			chain, err := rs.DbGetReferralChainForCode(app, el.Code)
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

}

func (rs CodeSystem) processCodePaymentRow(app *App, row AggregatedFeesRow, chain []DbReferralChainOfChild, batchTs string, scaling float64) error {
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
	payees[1] = app.Settings.BrokerPayoutAddr
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
func (a *CodeSystem) DbGetReferralChainForCode(app *App, code string) ([]DbReferralChainOfChild, error) {
	if code == env.DEFAULT_CODE {
		res := make([]DbReferralChainOfChild, 1)
		res[0] = DbReferralChainOfChild{
			Parent:     app.Settings.BrokerPayoutAddr.String(),
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
	err := app.Db.QueryRow(query, code).Scan(&refAddr, &traderCut)
	if err != nil {
		return []DbReferralChainOfChild{}, errors.New("DbGetReferralChainForCode:" + err.Error())
	}
	traderCut = traderCut / 100

	h := new(big.Int).SetInt64(0)
	chain, _, err := app.DbGetReferralChainFromChild(refAddr, h)
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
