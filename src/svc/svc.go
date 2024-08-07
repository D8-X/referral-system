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

	"github.com/spf13/viper"
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
	v, err := loadEnv()
	if err != nil {
		slog.Error("Error:" + err.Error())
		return
	}
	pk := utils.LoadFromFile(v.GetString(env.KEYFILE_PATH)+"keyfile.txt", abc)
	v.Set(env.BROKER_KEY, pk)
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
		slog.Error(err.Error())
		return
	}
	err = replaceEmptyBrokerName(app.Db, app.Settings.BrokerId)
	if err != nil {
		slog.Error(err.Error())
		return
	}
	// settings to database
	slog.Info("Writing settings to DB")
	err = app.SettingsToDB()
	if err != nil {
		slog.Error(err.Error())
		return
	}

	if !utils.IsValidPaymentSchedule(app.Settings.PayCronSchedule) {
		slog.Error("Error: paymentScheduleCron not a valid CRON-expression")
		return
	}
	fmt.Println("Payment schedule " + app.Settings.PayCronSchedule)
	err = app.DbGetMarginTkn()
	if err != nil {
		slog.Error("Error:" + err.Error())
		return
	}
	var wg sync.WaitGroup

	// execute payments if needed and schedule next payment
	go app.ManagePayments()

	wg.Add(1)
	go api.StartApiServer(&app, v.GetString(env.API_BIND_ADDR), v.GetString(env.API_PORT), &wg)
	wg.Wait()
}

func loadEnv() (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	if err := v.ReadInConfig(); err != nil {
		slog.Info("could not load .env file" + err.Error() + " using automatic envs")
	}
	loadAbc()
	v.AutomaticEnv()

	v.SetDefault(env.DATABASE_DSN_HISTORY, "postgres://postgres:postgres@localhost:5432/referral")

	requiredEnvs := []string{
		env.DATABASE_DSN_HISTORY,
		env.CONFIG_PATH,
		env.RPC_URL_PATH,
		env.API_BIND_ADDR,
		env.API_PORT,
		env.KEYFILE_PATH,
	}

	for _, e := range requiredEnvs {
		if !v.IsSet(e) {
			return nil, fmt.Errorf("required environment variable not set %s", e)
		}
	}

	return v, nil
}

// replaceEmptyBrokerName goes through all relevant tables and replaces
// an empty brokerId "" with the broker name configured in
// settings
func replaceEmptyBrokerName(dbInstance *sql.DB, brokerId string) error {
	// Update statements
	updateQueries := []string{
		`UPDATE "referral_chain" SET "broker_id" = $1 WHERE "broker_id" = ''`,
		`UPDATE "referral_code" SET "broker_id" = $1 WHERE "broker_id" = ''`,
		`UPDATE "referral_code_usage" SET "broker_id" = $1 WHERE "broker_id" = ''`,
		`UPDATE "referral_settings" SET "broker_id" = $1 WHERE "broker_id" = ''`,
		`UPDATE "referral_setting_cut" SET "broker_id" = $1 WHERE "broker_id" = ''`,
	}
	// Execute the update statements
	var rows int64
	for _, query := range updateQueries {
		result, err := dbInstance.Exec(query, brokerId)
		if err != nil {
			return fmt.Errorf("error executing query %s: %v", query, err)
		}
		rowsAffected, _ := result.RowsAffected()
		rows += rowsAffected
	}
	if rows > 0 {
		fmt.Printf("updated %d occurrences of empty broker with %s", rows, brokerId)
	}
	return nil
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
