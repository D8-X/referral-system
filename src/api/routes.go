package api

import (
	"net/http"
	"referral-system/src/referral"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes registers all API routes for D8X-Backend application
func RegisterRoutes(router chi.Router, app *referral.App) {
	/* Endpoint: /broker-address?id={id}
	router.Get("/broker-address", func(w http.ResponseWriter, r *http.Request) {
		GetBrokerAddress(w, r, a.Pen) // Pass fee here
	})

	// Endpoint: /broker-fee?perpetualId={perpetualId}
	router.Get("/broker-fee", func(w http.ResponseWriter, r *http.Request) {
		GetBrokerFee(w, r, a.BrokerFeeTbps) // Pass fee here
	})
	*/
	router.Post("/select-code", func(w http.ResponseWriter, r *http.Request) {
		onSelectCode(w, r, app)
	})

	router.Post("/refer", func(w http.ResponseWriter, r *http.Request) {
		onRefer(w, r, app)
	})

	router.Post("/upsert-code", func(w http.ResponseWriter, r *http.Request) {
		onUpsertCode(w, r, app)
	})
}
