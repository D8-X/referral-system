package api

type APIReferralCodeSelectionPayload struct {
	Code       string `json:"code"`
	TraderAddr string `json:"traderAddr"`
	CreatedOn  uint32 `json:"createdOn"`
	Signature  string `json:"signature"`
}

type APIAgencyReferralPayload struct {
	ReferToAddr   string  `json:"referToAddr"`
	PassOnPercent float64 `json:"passOnPercent"`
	CreatedOn     uint32  `json:"createdOn"`
	Signature     string  `json:"signature"`
}
