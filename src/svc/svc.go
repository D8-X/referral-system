package svc

import (
	"fmt"
	"log"
	"log/slog"
	"referral-system/env"
	"referral-system/src/api"
	"referral-system/src/referral"

	_ "github.com/lib/pq"
	"github.com/spf13/viper"
)

func Run() {
	v, err := loadEnv()
	if err != nil {
		slog.Error("Error:" + err.Error())
		return
	}
	if err != nil {
		// Handle the error
		log.Fatal(err)
	}

	var app referral.App
	s := v.GetString(env.REMOTE_BROKER_HTTP)
	fmt.Print(s)

	err = app.New(v)
	if err != nil {
		slog.Error("Error:" + err.Error())
		return
	}
	app.SavePayments()
	err = app.DbGetMarginTkn()
	if err != nil {
		slog.Error("Error:" + err.Error())
		return
	}
	// update token holdings
	app.DbUpdateTokenHoldings()
	app.ProcessAllPayments()
	api.StartApiServer(&app, v.GetString(env.API_BIND_ADDR), v.GetString(env.API_PORT))
	//app.DbGetReferralChainOfChild("0x9d5aaB428e98678d0E645ea4AeBd25f744341a05")
	//https://github.com/gitploy-io/cronexpr
}

func loadEnv() (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	if err := v.ReadInConfig(); err != nil {
		slog.Error("could not load .env file" + err.Error())
	}

	v.AutomaticEnv()

	v.SetDefault(env.DATABASE_DSN_HISTORY, "postgres://postgres:postgres@localhost:5432/referral")

	requiredEnvs := []string{
		env.DATABASE_DSN_HISTORY,
		env.CONFIG_PATH,
		env.BROKER_KEY,
		env.API_BIND_ADDR,
		env.API_PORT,
	}

	for _, e := range requiredEnvs {
		if !v.IsSet(e) {
			return nil, fmt.Errorf("required environment variable not set %s", e)
		}
	}

	return v, nil
}
