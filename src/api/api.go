package api

type APICodeSelectionPayload struct {
	Code       string `json:"code"`
	TraderAddr string `json:"traderAddr"`
	CreatedOn  uint32 `json:"createdOn"`
	Signature  string `json:"signature"`
}

type APICodePayload struct {
	Code          string `json:"code"`
	ReferrerAddr  string `json:"referrerAddr"`
	AgencyAddr    string `json:"agencyAddr"`
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
