package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"referral-system/src/referral"
	"referral-system/src/utils"
	"strings"
)

func onSelectCode(w http.ResponseWriter, r *http.Request, app *referral.App) {
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
	if strings.ToLower(addr.String()) != strings.ToLower(req.TraderAddr) {
		errMsg := `code selection signature wrong`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	// all tests passed, we can execute
	req.Code = WashCode(req.Code)
	err = app.SelectCode(req)
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

func onRefer(w http.ResponseWriter, r *http.Request, app *referral.App) {
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
	if false && !isCurrentTimestamp(req.CreatedOn) {
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
	if strings.ToLower(addr.String()) != strings.ToLower(req.ParentAddr) {
		errMsg := `code selection signature wrong`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	// now hand over to db
	err = app.Refer(req)
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

func onUpsertCode(w http.ResponseWriter, r *http.Request, app *referral.App) {
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
			'Code' : CODE1,
			'ReferrerAddr' : '0xabc...' ,
			'AgencyAddr' : '0xcbc...',
			'CreatedOn' : 1696166434,
			'PassOnPercTDF' : 5000,
			'Signature' :  '0xa1ef...'
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
	if strings.ToLower(addr.String()) != strings.ToLower(req.ReferrerAddr) {
		errMsg := `code selection signature wrong`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	req.Code = WashCode(req.Code)
	// hand over to db process
	err = app.UpsertCode(req)
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
