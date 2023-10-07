package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

func RegisterGlobalMiddleware(r chi.Router) {
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"DNT", "User-Agent", "X-Requested-With", "If-Modified-Since", "Cache-Control", "Content-Type", "Range"},
	}))
}
