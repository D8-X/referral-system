package referral

import (
	"database/sql"
	"referral-system/src/utils"
)

type SocialSystem struct {
}

func (rs SocialSystem) OpenPay(app *App, traderAddr string) (utils.APIResponseOpenEarnings, error) {
	return utils.APIResponseOpenEarnings{}, nil
}

func (rs SocialSystem) ProcessPayments(app *App, rows *sql.Rows, scale map[uint32]float64, batchTs string) {
}

func (rs SocialSystem) processCodePaymentRow(app *App, row AggregatedFeesRow, chain []DbReferralChainOfChild, batchTs string, scaling float64) error {
	return nil
}
