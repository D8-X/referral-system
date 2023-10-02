package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

func RegisterGlobalMiddleware(r chi.Router) {
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
	}))
}
