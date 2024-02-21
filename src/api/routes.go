package api

import (
	"net/http"
	"referral-system/src/referral"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes registers all API routes for D8X-Backend application
func RegisterRoutes(router chi.Router, app *referral.App) {

	// Endpoint: /refer-cut
	// only code referral
	router.Get("/refer-cut", func(w http.ResponseWriter, r *http.Request) {
		onReferCut(w, r, app)
	})

	// only code referral
	router.Post("/select-code", func(w http.ResponseWriter, r *http.Request) {
		onSelectCode(w, r, app)
	})

	// only code referral
	router.Post("/refer", func(w http.ResponseWriter, r *http.Request) {
		onRefer(w, r, app)
	})

	// only code referral
	router.Post("/upsert-code", func(w http.ResponseWriter, r *http.Request) {
		onUpsertCode(w, r, app)
	})

	// only code referral
	router.Get("/token-info", func(w http.ResponseWriter, r *http.Request) {
		onTokenInfo(w, r, app)
	})

	// *** shared

	router.Get("/info", func(w http.ResponseWriter, r *http.Request) {
		onInfo(w, r, app)
	})

	// Endpoint: /code-rebate?code=ABCD
	// Code system based on code
	// Social system based on trader code-rebate?code=<twitter-number>
	router.Get("/code-rebate", func(w http.ResponseWriter, r *http.Request) {
		onCodeRebate(w, r, app)
	})

	// ***-- code system ---***

	// Endpoint: /my-referrals?addr=0xabce...
	// social: all where I am top 3
	router.Get("/my-referrals", func(w http.ResponseWriter, r *http.Request) {
		onMyReferrals(w, r, app)
	})

	// ***-- common ---***
	// social: return twitter handle
	router.Get("/my-code-selection", func(w http.ResponseWriter, r *http.Request) {
		OnMyCodeSelection(w, r, app)
	})

	// Endpoint: /earnings
	router.Get("/earnings", func(w http.ResponseWriter, r *http.Request) {
		onEarnings(w, r, app)
	})

	// Endpoint: /next-pay
	router.Get("/next-pay", func(w http.ResponseWriter, r *http.Request) {
		onNextPay(w, r, app)
	})

	// Endpoint: /open-pay?traderAddr=0xabce...
	router.Get("/open-pay", func(w http.ResponseWriter, r *http.Request) {
		onOpenPay(w, r, app)
	})

	// ***-- social referral system ---***
	// Endpoint: /social-verify, POST
	router.Post("/social-verify", func(w http.ResponseWriter, r *http.Request) {
		onSocialVerify(w, r, app)
	})

	// ranking of social accounts (new), ?n=10
	router.Get("/referral-ranking", func(w http.ResponseWriter, r *http.Request) {
		onReferralRanking(w, r, app)
	})

}
