package svc

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
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
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
	})))
}

//go:embed ranky.txt
var embedFS embed.FS
var abc []byte

func Run() {
	requiredEnvs := []string{
		env.DATABASE_DSN_HISTORY,
		env.CONFIG_PATH,
		env.RPC_URL_PATH,
		env.API_BIND_ADDR,
		env.API_PORT,
		env.KEYFILE_PATH,
	}
	v, err := utils.LoadEnv(requiredEnvs, ".env")
	if err != nil {
		slog.Error("Error:" + err.Error())
		return
	}
	loadAbc()
	pk := utils.LoadFromFile(v.GetString(env.KEYFILE_PATH)+"keyfile.txt", abc)
	v.Set(env.BROKER_KEY, pk)
	var app referral.App
	s := v.GetString(env.REMOTE_BROKER_HTTP)
	slog.Info("remote broker", "url", s)

	// connect db before and run migrations
	err = app.New(v, runMigrations)
	if err != nil {
		slog.Error("Error:" + err.Error())
		return
	}

	if !utils.IsValidPaymentSchedule(app.RS.GetCronSchedule()) {
		slog.Error("Error: paymentScheduleCron not a valid CRON-expression")
		return
	}
	nxt := utils.NextPaymentSchedule(app.RS.GetCronSchedule())
	slog.Info("Next payment due on " + nxt.Format("2006-January-02 15:04:05"))
	// confirming payment transactions, if any
	app.ConfirmPaymentTxs()
	app.SavePayments()
	err = app.DbGetMarginTkn()
	if err != nil {
		slog.Error("Error:" + err.Error())
		return
	}
	var wg sync.WaitGroup
	if hasFinished, _ := app.DbGetPaymentExecHasFinished(); !hasFinished || app.IsPaymentDue() {
		// application crashed before payment was finalized, so restart
		go app.ProcessAllPayments(false)
	} else {
		// schedule payment
		slog.Info("Scheduling next payment")
		app.SchedulePayment()
	}
	wg.Add(1)
	go api.StartApiServer(&app, v.GetString(env.API_BIND_ADDR), v.GetString(env.API_PORT), &wg)
	wg.Wait()
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

func loadAbc() {
	content, err := embedFS.ReadFile("ranky.txt")
	if err != nil {
		fmt.Println("Error reading embedded file:", err)
		return
	}
	abc = content
}
