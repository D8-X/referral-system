package referral

import (
	"errors"
	"log/slog"
	"math/big"
	"referral-system/src/utils"
	"time"
)

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
		if el.Code == "DEFAULT" {
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

// DbGetReferralChainForCode gets the entire chain of referrals
// for a code, calculating what each participant earns (percent)
func (a *App) DbGetReferralChainForCode(code string) ([]DbReferralChainOfChild, error) {
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
