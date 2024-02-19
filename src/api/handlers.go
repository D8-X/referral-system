package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"referral-system/env"
	"referral-system/src/referral"
	"referral-system/src/utils"
	"strconv"
	"strings"
)

// onSelectCode POST request; only code referral
func onSelectCode(w http.ResponseWriter, r *http.Request, app *referral.App) {
	if app.RS.GetType() == referral.SOCIAL_SYS_TYPE {
		http.Error(w, string(formatError("n/a for social system")), http.StatusBadRequest)
		return
	}
	// Read the JSON data from the request body
	var jsonData []byte
	if r.Body != nil {
		defer r.Body.Close()
		jsonData, _ = io.ReadAll(r.Body)
	}
	var req utils.APICodeSelectionPayload
	err := json.Unmarshal(jsonData, &req)
	if err != nil {
		errMsg := `Wrong argument types. Usage:
		{
		   'code': 'ABC'
		   'traderAddr':'0xaCFe...'
		   'createdOn': 1696166434
		   'signature':'0xABCE...'
	   }`
		errMsg = strings.ReplaceAll(errMsg, "\t", "")
		errMsg = strings.ReplaceAll(errMsg, "\n", "")
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}

	if !isValidEvmAddr(req.TraderAddr) {
		errMsg := `invalid address`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	if !isCurrentTimestamp(req.CreatedOn) {
		errMsg := `timestamp not current`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	addr, err := RecoverCodeSelectSigAddr(req)
	if err != nil {
		slog.Info("Recovering code selection failed:" + err.Error())
		errMsg := `code selection signature recovery failed`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	if strings.EqualFold(addr.String(), req.TraderAddr) {
		errMsg := `code selection signature wrong`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	// switch to codesystem
	codeSystem, ok := app.RS.(*referral.CodeSystem)
	if !ok {
		slog.Error("failed to assert as CodeSystem")
		http.Error(w, string(formatError("failed")), http.StatusBadRequest)
		return
	}

	// all tests passed, we can execute
	req.Code = WashCode(req.Code)
	err = codeSystem.SelectCode(req)
	if err != nil {
		errMsg := `code selection failed:` + err.Error()
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	// Set the Content-Type header to application/json
	w.Header().Set("Content-Type", "application/json")
	// Write the JSON response
	jsonResponse := `{"type":"select-code", "data":{"code": ` + req.Code + `}}`
	w.Write([]byte(jsonResponse))
}

// onRefer is the endpoint handler for POST /refer, only code referral
func onRefer(w http.ResponseWriter, r *http.Request, app *referral.App) {
	if app.RS.GetType() == referral.SOCIAL_SYS_TYPE {
		http.Error(w, string(formatError("n/a for social system")), http.StatusBadRequest)
		return
	}
	// Read the JSON data from the request body
	var jsonData []byte
	if r.Body != nil {
		defer r.Body.Close()
		jsonData, _ = io.ReadAll(r.Body)
	}
	var req utils.APIReferPayload
	err := json.Unmarshal(jsonData, &req)
	if err != nil {
		errMsg := `Wrong argument types. Usage:
		{
			'parentAddr': '0x..',
			'referToAddr': '0x..',
			'passOnPercTDF': 500,
			'createdOn': 1696166434,
			'signature': '0x...'
		}`
		errMsg = strings.ReplaceAll(errMsg, "\t", "")
		errMsg = strings.ReplaceAll(errMsg, "\n", "")
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	if !isValidEvmAddr(req.ParentAddr) || !isValidEvmAddr(req.ReferToAddr) {
		errMsg := `invalid address`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	if !isCurrentTimestamp(req.CreatedOn) {
		errMsg := `timestamp not current`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	if req.PassOnPercTDF >= 10000 {
		errMsg := `pass on percentage invalid`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	addr, err := RecoverReferralSigAddr(req)
	if err != nil {
		slog.Info("Recovering referral signature failed:" + err.Error())
		errMsg := `referral signature recovery failed`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	if strings.EqualFold(addr.String(), req.ParentAddr) {
		errMsg := `code selection signature wrong`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	// switch to codesystem
	codeSystem, ok := app.RS.(*referral.CodeSystem)
	if !ok {
		slog.Error("failed to assert as CodeSystem")
		http.Error(w, string(formatError("failed")), http.StatusBadRequest)
		return
	}

	// now hand over to db
	err = codeSystem.Refer(req)
	if err != nil {
		errMsg := `referral failed:` + err.Error()
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	// Set the Content-Type header to application/json
	w.Header().Set("Content-Type", "application/json")
	// Write the JSON response
	jsonResponse := `{"type":"referral-code", "data":{"referToAddr": "` + req.ReferToAddr + `"}}`
	w.Write([]byte(jsonResponse))

}

// onUpsertCode implements the POST request for /upsert-code, only code referral
func onUpsertCode(w http.ResponseWriter, r *http.Request, app *referral.App) {
	if app.RS.GetType() == referral.SOCIAL_SYS_TYPE {
		http.Error(w, string(formatError("n/a for social system")), http.StatusBadRequest)
		return
	}
	// Read the JSON data from the request body
	var jsonData []byte
	if r.Body != nil {
		defer r.Body.Close()
		jsonData, _ = io.ReadAll(r.Body)
	}
	var req utils.APICodePayload
	err := json.Unmarshal(jsonData, &req)
	if err != nil {
		errMsg := `Wrong argument types. Usage:
		{
			'code' : 'CODE1',
			'referrerAddr' : '0xabc...' ,
			'agencyAddr' : '0xcbc...',
			'createdOn' : 1696166434,
			'passOnPercTDF' : 5000,
			'signature' :  '0xa1ef...'
		}`
		errMsg = strings.ReplaceAll(errMsg, "\t", "")
		errMsg = strings.ReplaceAll(errMsg, "\n", "")
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	if !isValidEvmAddr(req.ReferrerAddr) {
		errMsg := `invalid address`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	if !isCurrentTimestamp(req.CreatedOn) {
		errMsg := `timestamp not current`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	if req.PassOnPercTDF >= 10000 {
		errMsg := `pass on percentage invalid`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	addr, err := RecoverCodeSigAddr(req)
	if err != nil {
		slog.Info("Recovering code selection failed:" + err.Error())
		errMsg := `code selection signature recovery failed`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	if strings.EqualFold(addr.String(), req.ReferrerAddr) {
		errMsg := `code selection signature wrong`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}

	req.Code = WashCode(req.Code)
	if req.Code == env.DEFAULT_CODE {
		// default code is reserved for traders without code
		errMsg := `code invalid`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}

	// switch to codesystem
	codeSystem, ok := app.RS.(*referral.CodeSystem)
	if !ok {
		slog.Error("failed to assert as CodeSystem")
		http.Error(w, string(formatError("failed")), http.StatusBadRequest)
		return
	}

	// hand over to db process
	err = codeSystem.UpsertCode(req)
	if err != nil {
		errMsg := `code upsert failed:` + err.Error()
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	// Set the Content-Type header to application/json
	w.Header().Set("Content-Type", "application/json")
	// Write the JSON response
	jsonResponse := `{"type":"upsert-code", "data":{"code": "` + req.Code + `"}}`
	w.Write([]byte(jsonResponse))

}

// onCodeRebate implements the endpoint /code-rebate?code=ABCD
// which for the code system is based on code a code
// and for the social system based on trader code-rebate?code=<twitter-number>
func onCodeRebate(w http.ResponseWriter, r *http.Request, app *referral.App) {
	// Read the JSON data from the request body
	code := r.URL.Query().Get("code")
	if code == "" {
		errMsg := "Missing 'code' parameter"
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}

	if app.RS.GetType() == referral.SOCIAL_SYS_TYPE {
		socialSysRebate(w, app, code)
	} else if app.RS.GetType() == referral.CODE_SYS_TYPE {
		codeSysRebate(w, app, code)
	}
}

// onSocialSysRebate returns the rebate associated with the given twitter-id
func socialSysRebate(w http.ResponseWriter, app *referral.App, code string) {
	// TODO
}

// onCodeSysRebate returns the trader rebate associated with the given
// referral code
func codeSysRebate(w http.ResponseWriter, app *referral.App, code string) {

	code = WashCode(code)
	// switch to codesystem
	codeSystem, ok := app.RS.(*referral.CodeSystem)
	if !ok {
		slog.Error("failed to assert as CodeSystem")
		http.Error(w, string(formatError("failed")), http.StatusBadRequest)
		return
	}

	rebate, err := codeSystem.CutPercentageCode(code)
	if err != nil {
		errMsg := err.Error()
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	// Write the JSON response
	stringValue := strconv.FormatFloat(rebate, 'f', -1, 64)
	jsonResponse := `{"type":"code-rebate", "data":{"rebate_percent": ` + stringValue + `}}`
	w.Write([]byte(jsonResponse))
}

// onReferCut implements the endpoint for /refer-cut, only code referral
func onReferCut(w http.ResponseWriter, r *http.Request, app *referral.App) {
	if app.RS.GetType() == referral.SOCIAL_SYS_TYPE {
		http.Error(w, string(formatError("n/a for social system")), http.StatusBadRequest)
		return
	}
	// Read the JSON data from the request body
	addr := r.URL.Query().Get("addr")
	if addr == "" || !isValidEvmAddr(addr) {
		errMsg := "Incorrect 'addr' parameter"
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	addr = strings.ToLower(addr)
	// optional:
	h := r.URL.Query().Get("holdings")
	holdings := new(big.Int).SetInt64(0)
	if h != "" {
		holdings.SetString(h, 10)
	}

	// switch to codesystem
	codeSystem, ok := app.RS.(*referral.CodeSystem)
	if !ok {
		slog.Error("failed to assert as CodeSystem")
		http.Error(w, string(formatError("failed")), http.StatusBadRequest)
		return
	}

	cut, isAgency, err := codeSystem.CutPercentageAgency(addr, holdings)
	if err != nil {
		errMsg := err.Error()
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	// Write the JSON response
	stringValue := strconv.FormatFloat(cut, 'f', -1, 64)
	jsonResponse := `{"type":"refer-cut", "data":{"isAgency":` +
		strconv.FormatBool(isAgency) + `, "passed_on_percent": ` + stringValue + `}}`
	w.Write([]byte(jsonResponse))
}

// OnMyCodeSelection implements the endpoint for my-code-selection,
// for the code system we return the code, for the social system the twitter id
func OnMyCodeSelection(w http.ResponseWriter, r *http.Request, app *referral.App) {
	addr := r.URL.Query().Get("traderAddr")
	if addr == "" || !isValidEvmAddr(addr) {
		errMsg := "Incorrect 'traderAddr' parameter"
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	addr = strings.ToLower(addr)

	if app.RS.GetType() == referral.SOCIAL_SYS_TYPE {
		idSelection(w, app, addr)
	} else if app.RS.GetType() == referral.CODE_SYS_TYPE {
		codeSelection(w, app, addr)
	}
}

// idSelection returns the twitter handle for the given trader address
func idSelection(w http.ResponseWriter, app *referral.App, addr string) {
	// TODO
}

// codeSelection returns the selected code for the given trader address
func codeSelection(w http.ResponseWriter, app *referral.App, addr string) {
	// switch to codesystem
	codeSystem, ok := app.RS.(*referral.CodeSystem)
	if !ok {
		slog.Error("failed to assert as CodeSystem")
		http.Error(w, string(formatError("failed")), http.StatusBadRequest)
		return
	}
	code, err := codeSystem.DbGetMyCodeSelection(addr)
	if err != nil {
		errMsg := err.Error()
		http.Error(w, string(formatError(errMsg)), http.StatusInternalServerError)
		return
	}

	// Set the Content-Type header to application/json
	w.Header().Set("Content-Type", "application/json")
	// Write the JSON response
	response := utils.APIResponse{Type: "my-code-selection", Data: code}
	// Marshal the struct into JSON
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		slog.Error("OnMyCodeSelection unable to marshal response" + err.Error())
		errMsg := "Unavailable"
		http.Error(w, string(formatError(errMsg)), http.StatusInternalServerError)
		return
	}
	w.Write(jsonResponse)
}

// onTokenInfo handles /token-info which is only available for the code based
// referral system
func onTokenInfo(w http.ResponseWriter, r *http.Request, app *referral.App) {
	if app.RS.GetType() == referral.SOCIAL_SYS_TYPE {
		http.Error(w, string(formatError("n/a for social system")), http.StatusBadRequest)
		return
	}
	// switch to codesystem
	codeSystem, ok := app.RS.(*referral.CodeSystem)
	if !ok {
		slog.Error("failed to assert as CodeSystem")
		http.Error(w, string(formatError("failed")), http.StatusBadRequest)
		return
	}
	info, err := codeSystem.DbGetTokenInfo()
	if err != nil {
		errMsg := err.Error()
		http.Error(w, string(formatError(errMsg)), http.StatusInternalServerError)
		return
	}
	// Set the Content-Type header to application/json
	w.Header().Set("Content-Type", "application/json")
	// Write the JSON response
	response := utils.APIResponse{Type: "token-info", Data: info}
	// Marshal the struct into JSON
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		slog.Error("onTokenInfo unable to marshal response" + err.Error())
		errMsg := "Unavailable"
		http.Error(w, string(formatError(errMsg)), http.StatusInternalServerError)
		return
	}
	w.Write(jsonResponse)
}

// onMyReferrals implements the endpoint /my-referrals which is available for
// both referral systems. In the code system we return the referred parties and
// codes, for the social system we return all twitter ids for which the given
// id is in the top3
func onMyReferrals(w http.ResponseWriter, r *http.Request, app *referral.App) {
	addr := r.URL.Query().Get("addr")
	if addr == "" || !isValidEvmAddr(addr) {
		errMsg := "Incorrect 'addr' parameter"
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	addr = strings.ToLower(addr)

	if app.RS.GetType() == referral.SOCIAL_SYS_TYPE {
		mySocialReferrals(w, app, addr)
	} else if app.RS.GetType() == referral.CODE_SYS_TYPE {
		myCodeReferrals(w, app, addr)
	}
}

func mySocialReferrals(w http.ResponseWriter, app *referral.App, addr string) {
	// todo
}

func myCodeReferrals(w http.ResponseWriter, app *referral.App, addr string) {
	// switch to codesystem
	codeSystem, ok := app.RS.(*referral.CodeSystem)
	if !ok {
		slog.Error("failed to assert as CodeSystem")
		http.Error(w, string(formatError("failed")), http.StatusBadRequest)
		return
	}
	ref, err := codeSystem.DbGetMyReferrals(addr)
	if err != nil {
		errMsg := err.Error()
		http.Error(w, string(formatError(errMsg)), http.StatusInternalServerError)
		return
	}

	// Set the Content-Type header to application/json
	w.Header().Set("Content-Type", "application/json")
	// Write the JSON response
	if ref == nil {
		ref = []utils.APIResponseMyReferrals{}
	}
	response := utils.APIResponse{Type: "my-referrals", Data: ref}
	// Marshal the struct into JSON
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		slog.Error("onMyReferrals unable to marshal response" + err.Error())
		errMsg := "Unavailable"
		http.Error(w, string(formatError(errMsg)), http.StatusInternalServerError)
		return
	}
	w.Write(jsonResponse)
}

// onNextPay handles the endpoint that sends the next payment date as
// timestamp and human-readable date; available for both referral systems
func onNextPay(w http.ResponseWriter, r *http.Request, app *referral.App) {
	nxt := utils.NextPaymentSchedule(app.RS.GetCronSchedule())
	// Set the Content-Type header to application/json
	w.Header().Set("Content-Type", "application/json")
	// Write the JSON response
	type Res struct {
		NextPaymentDueTs int    `json:"nextPaymentDueTs"`
		NextPaymentDue   string `json:"nextPaymentDue"`
	}
	info := Res{
		NextPaymentDueTs: int(nxt.Unix()),
		NextPaymentDue:   nxt.Format("2006-January-02 15:04:05"),
	}
	response := utils.APIResponse{Type: "next-pay", Data: info}
	// Marshal the struct into JSON
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		slog.Error("onNextPay unable to marshal response" + err.Error())
		errMsg := "Unavailable"
		http.Error(w, string(formatError(errMsg)), http.StatusInternalServerError)
		return
	}
	w.Write(jsonResponse)
}

// onExecutor returns payment executor and broker address, available for both
// referral systems
func onExecutor(w http.ResponseWriter, r *http.Request, app *referral.App) {
	excAddr := app.PaymentExecutor.GetExecutorAddrHex()
	brkrAddr := app.PaymentExecutor.GetBrokerAddr()
	type Res struct {
		ExecutorAddr string `json:"executorAddr"`
		BrokerAddr   string `json:"brokerAddr"`
	}
	info := Res{
		ExecutorAddr: excAddr,
		BrokerAddr:   brkrAddr.String(),
	}
	jsonResponse, err := json.Marshal(info)
	if err != nil {
		slog.Error("onExecutor unable to marshal response" + err.Error())
		errMsg := "Unavailable"
		http.Error(w, string(formatError(errMsg)), http.StatusInternalServerError)
		return
	}
	w.Write(jsonResponse)
}

// onReferralRanking returns the global ranking of the top n
func onReferralRanking(w http.ResponseWriter, r *http.Request, app *referral.App) {
	if app.RS.GetType() == referral.CODE_SYS_TYPE {
		http.Error(w, string(formatError("n/a for code system")), http.StatusBadRequest)
		return
	}
	// TODO

}

// onSocialVerify handles the POST endpoint /social-verify only available
// for the social referral system
func onSocialVerify(w http.ResponseWriter, r *http.Request, app *referral.App) {
	if app.RS.GetType() == referral.CODE_SYS_TYPE {
		http.Error(w, string(formatError("n/a for code system")), http.StatusBadRequest)
		return
	}
	// switch to social system
	socSystem, ok := app.RS.(*referral.SocialSystem)
	if !ok {
		slog.Error("failed to assert as social system")
		http.Error(w, string(formatError("failed")), http.StatusBadRequest)
		return
	}
	if socSystem.Xsdk == nil {
		errMsg := `Social referral system not setup`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	type VerifyRequest struct {
		AppPubKey string `json:"appPubKey"`
	}
	// Extract jsonIdToken from the request header
	jsonIdToken := r.Header.Get("Authorization")
	if jsonIdToken == "" {
		http.Error(w, "Authorization token not provided", http.StatusUnauthorized)
		return
	}
	jsonIdToken = strings.TrimPrefix(jsonIdToken, "Bearer ")
	// Read the JSON data from the request body
	var verifyRequest VerifyRequest
	defer r.Body.Close()
	jsonData, _ := io.ReadAll(r.Body)
	err := json.Unmarshal(jsonData, &verifyRequest)
	if err != nil {
		errMsg := `Wrong argument types. Usage: web3auth server-side-verification`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	err = socSystem.RegisterSocialUser(jsonIdToken, verifyRequest.AppPubKey)
	if err != nil {
		slog.Error("Authentication failed:" + err.Error())
		http.Error(w, string(formatError("Authentication failed")), http.StatusBadRequest)
		return
	}
	slog.Info("Logged-in new social user")
	w.Write([]byte("success"))
}

// onEarnings handles the endpoint /earnings which is enabled for both
// referral systems
func onEarnings(w http.ResponseWriter, r *http.Request, app *referral.App) {

	addr := r.URL.Query().Get("addr")
	if addr == "" || !isValidEvmAddr(addr) {
		errMsg := "Incorrect 'addr' parameter"
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	addr = strings.ToLower(addr)
	res, err := app.HistoricEarnings(addr)
	if err != nil {
		errMsg := err.Error()
		http.Error(w, string(formatError(errMsg)), http.StatusInternalServerError)
		return
	}
	response := utils.APIResponse{Type: "earnings", Data: res}
	// Marshal the struct into JSON
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		slog.Error("onEarnings unable to marshal response" + err.Error())
		errMsg := "Unavailable"
		http.Error(w, string(formatError(errMsg)), http.StatusInternalServerError)
		return
	}
	// Set the Content-Type header to application/json
	w.Header().Set("Content-Type", "application/json")
	// Write the JSON response
	w.Write(jsonResponse)
}

// onEarnings handles the endpoint /earnings which is enabled for both
// referral systems
func onOpenPay(w http.ResponseWriter, r *http.Request, app *referral.App) {
	addr := r.URL.Query().Get("traderAddr")
	if addr == "" || !isValidEvmAddr(addr) {
		errMsg := "Incorrect 'addr' parameter"
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	addr = strings.ToLower(addr)
	res, err := app.RS.OpenPay(app, addr)
	if err != nil {
		errMsg := err.Error()
		http.Error(w, string(formatError(errMsg)), http.StatusInternalServerError)
		return
	}

	// Set the Content-Type header to application/json
	w.Header().Set("Content-Type", "application/json")
	response := utils.APIResponse{Type: "open-pay", Data: res}
	// Marshal the struct into JSON
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		slog.Error("earnings unable to marshal response" + err.Error())
		errMsg := "Unavailable"
		http.Error(w, string(formatError(errMsg)), http.StatusInternalServerError)
		return
	}
	// Write the JSON response
	w.Write(jsonResponse)
}
