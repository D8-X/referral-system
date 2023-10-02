package api

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"referral-system/src/referral"

	"github.com/go-chi/chi/v5"
	"golang.org/x/exp/slog"
)

func StartApiServer(app *referral.App, host string, port string) error {
	router := chi.NewRouter()
	RegisterGlobalMiddleware(router)
	RegisterRoutes(router, app)

	addr := net.JoinHostPort(
		host,
		port,
	)
	slog.Info("starting api server host_port " + addr)
	err := http.ListenAndServe(
		addr,
		router,
	)
	return errors.New("api server is shutting down" + err.Error())
}

func formatError(errorMsg string) []byte {
	response := struct {
		Error string `json:"error"`
	}{
		Error: errorMsg,
	}
	// Marshal the struct into JSON
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return []byte(err.Error())
	}
	return jsonResponse
}
