package main

import (
	"log/slog"
	"referral-system/src/svc"
)

// Injected via -ldflags -X
var VERSION = "referral-system-development"

func main() {
	slog.Info("starting service",
		slog.String("name", "referral-system"),
		slog.String("version", VERSION),
	)
	svc.Run()
}
