package utils

import (
	"time"

	"github.com/adhocore/gronx"
)

type APICodeSelectionPayload struct {
	Code       string `json:"code"`
	TraderAddr string `json:"traderAddr"`
	CreatedOn  uint32 `json:"createdOn"`
	Signature  string `json:"signature"`
}

type APICodePayload struct {
	Code          string `json:"code"`
	ReferrerAddr  string `json:"referrerAddr"`
	CreatedOn     uint32 `json:"createdOn"`
	PassOnPercTDF uint32 `json:"passOnPercTDF"`
	Signature     string `json:"signature"`
}

type APIReferPayload struct {
	ParentAddr    string `json:"parentAddr"`
	ReferToAddr   string `json:"referToAddr"`
	PassOnPercTDF uint32 `json:"passOnPercTDF"`
	CreatedOn     uint32 `json:"createdOn"`
	Signature     string `json:"signature"`
}

type APIResponseHistEarnings struct {
	PoolId    uint32  `json:"poolId"`
	Code      string  `json:"code"`
	Earnings  float64 `json:"earnings"`
	AsTrader  bool    `json:"asTrader"`
	TokenName string  `json:"tokenName"`
}

type OpenPay struct {
	PoolId    uint32  `json:"poolId"`
	Amount    float64 `json:"earnings"`
	TokenName string  `json:"tokenName"`
}
type APIResponseOpenEarnings struct {
	Code    string    `json:"code"`
	OpenPay []OpenPay `json:"openEarnings"`
}

type APIResponseMyReferrals struct {
	Referral   string  `json:"referral"`
	PassOnPerc float64 `json:"passOnPerc"`
}

type APIResponse struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func IsValidPaymentSchedule(expr string) bool {
	gron := gronx.New()
	return gron.IsValid(expr)
}

func NextPaymentSchedule(expr string) time.Time {
	time, _ := gronx.NextTick(expr, true)
	return time
}

func PrevPaymentSchedule(expr string) time.Time {
	time, _ := gronx.PrevTick(expr, true)
	return time
}
