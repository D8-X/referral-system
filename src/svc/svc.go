package svc

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"referral-system/env"
	"referral-system/src/api"
	"referral-system/src/db"
	"referral-system/src/referral"
	"referral-system/src/utils"
	"sync"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

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
	slog.Info("remote broker", "url", s)

	// connect db before running migrations
	if err := app.ConnectDB(v.GetString(env.DATABASE_DSN_HISTORY)); err != nil {
		slog.Error("connecting to db", "error", err)
		return
	}

	// Run migrations on startup. If migrations fail - exit.
	if err := runMigrations(v.GetString(env.DATABASE_DSN_HISTORY), app.Db); err != nil {
		slog.Error("running migrations", "error", err)
		os.Exit(1)
		return
	} else {
		slog.Info("migrations run completed")
	}

	err = app.New(v)
	if err != nil {
		slog.Error("Error:" + err.Error())
		return
	}
	if !utils.IsValidPaymentSchedule(app.Settings.PayCronSchedule) {
		slog.Error("Error: paymentScheduleCron not a valid CRON-expression")
		return
	}
	nxt := utils.NextPaymentSchedule(app.Settings.PayCronSchedule)
	slog.Info("Next payment due on " + nxt.Format("2006-January-02 15:04:05"))
	app.SavePayments()
	app.PurgeUnconfirmedPayments()
	err = app.DbGetMarginTkn()
	if err != nil {
		slog.Error("Error:" + err.Error())
		return
	}
	var wg sync.WaitGroup
	if hasFinished, _ := app.DbGetPaymentExecHasFinished(); !hasFinished || app.IsPaymentDue() {
		// application crashed before payment was finalized, so restart
		go app.ProcessAllPayments()
	}

	go api.StartApiServer(&app, v.GetString(env.API_BIND_ADDR), v.GetString(env.API_PORT), &wg)
	wg.Wait()
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
		env.RPC_URL_PATH,
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

func runMigrations(postgresDSN string, dbInstance *sql.DB) error {
	// HACK: we want to run migrations only when history tables are present.
	// Otherwise referral migrations will fail and loop on being marked as dirty
	// on service restarts.
	res, err := dbInstance.Query("select exists (select * from pg_tables where tablename= 'trades_history')")
	if err != nil {
		return fmt.Errorf("querying history tables existence: %w", err)
	}
	historyTablesExist := false
	res.Next()
	if err := res.Scan(&historyTablesExist); err != nil {
		return fmt.Errorf("scanning history tables existence result: %w", err)
	}

	if !historyTablesExist {
		return errors.New("history tables do not exist, skipping migrations")
	}

	slog.Info("history tables exist, running migrations...")

	source, err := iofs.New(db.MigrationsFS, "migrations")
	if err != nil {
		return err
	}
	m, err := migrate.NewWithSourceInstance(
		"MigrationsFS",
		source,
		postgresDSN,
	)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil {
		// Only return error if it's not "no change" error
		if err.Error() != "no change" {
			return err
		}
	}

	e1, e2 := m.Close()
	if e1 != nil {
		return e1
	}
	if e2 != nil {
		return e2
	}
	return nil
}
