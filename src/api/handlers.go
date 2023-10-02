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
	req.Code = WashCode(req.Code)
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
	addrStr := strings.ToLower(addr.String())
	addrTarget := strings.ToLower(req.TraderAddr)
	if addrStr != addrTarget {
		errMsg := `code selection signature wrong`
		http.Error(w, string(formatError(errMsg)), http.StatusBadRequest)
		return
	}
	// all tests passed, we can execute
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

func onRefer(w http.ResponseWriter, r *http.Request) {

}

func onUpsertCode(w http.ResponseWriter, r *http.Request) {
}
