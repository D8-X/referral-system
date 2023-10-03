package utils

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
