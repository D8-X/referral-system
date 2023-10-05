package api

import (
	"net/http"
	"referral-system/src/referral"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes registers all API routes for D8X-Backend application
func RegisterRoutes(router chi.Router, app *referral.App) {

	// Endpoint: /my-referrals?addr=0xabce...
	router.Get("/my-referrals", func(w http.ResponseWriter, r *http.Request) {
		onMyReferrals(w, r, app) // Pass fee here
	})

	// Endpoint: /open-pay?traderAddr=0xabce...
	router.Get("/open-pay", func(w http.ResponseWriter, r *http.Request) {
		onOpenPay(w, r, app) // Pass fee here
	})

	// Endpoint: /food-chain?code=ABCD
	router.Get("/food-chain", func(w http.ResponseWriter, r *http.Request) {
		onFoodChain(w, r, app) // Pass fee here
	})

	// Endpoint: /earnings
	router.Get("/earnings", func(w http.ResponseWriter, r *http.Request) {
		onEarnings(w, r, app) // Pass fee here
	})

	// Endpoint: /refer-cut
	router.Get("/refer-cut", func(w http.ResponseWriter, r *http.Request) {
		onReferCut(w, r, app) // Pass fee here
	})

	// Endpoint: /code-rebate?code=ABCD
	router.Get("/code-rebate", func(w http.ResponseWriter, r *http.Request) {
		onCodeRebate(w, r, app) // Pass fee here
	})

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
