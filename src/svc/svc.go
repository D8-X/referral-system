package svc

import (
	"database/sql"
	"fmt"
	"log/slog"
	"referral-system/env"
	"referral-system/src/utils"

	_ "github.com/lib/pq"
	"github.com/spf13/viper"
)

func Run() {
	err := loadEnv()
	if err != nil {
		slog.Error("Error:" + err.Error())
		return
	}
	db, err := connectDB()
	if err != nil {
		slog.Error("Error:" + err.Error())
		return
	}
	var structure []utils.DbReferralStruct
	var row utils.DbReferralStruct

	rows, err := db.Query("SELECT * FROM referral_struct")
	defer rows.Close()
	if err != nil {
		slog.Error(err.Error())
		return
	}
	for rows.Next() {
		rows.Scan(&row.Parent, &row.Child, &row.PassOn, &row.CreatedOn)
		structure = append(structure, row)
		fmt.Println(row)
	}
}

func connectDB() (*sql.DB, error) {
	connStr := viper.GetString(env.DATABASE_DSN_HISTORY)
	// Connect to database
	// From documentation: "The returned DB is safe for concurrent use by multiple goroutines and
	// maintains its own pool of idle connections. Thus, the Open function should be called just once.
	// It is rarely necessary to close a DB."
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func loadEnv() error {
	viper.SetConfigFile(".env")
	if err := viper.ReadInConfig(); err != nil {
		slog.Error("could not load .env file" + err.Error())
	}

	viper.AutomaticEnv()

	viper.SetDefault(env.DATABASE_DSN_HISTORY, "postgres://postgres:postgres@localhost:5432/referral")

	requiredEnvs := []string{
		env.DATABASE_DSN_HISTORY,
	}

	for _, e := range requiredEnvs {
		if !viper.IsSet(e) {
			return fmt.Errorf("required environment variable not set %s", e)
		}
	}

	return nil
}
