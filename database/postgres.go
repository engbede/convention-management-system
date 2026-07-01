package database

import (
	"database/sql"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func InitPostgres() error {

	databaseURL := os.Getenv("DATABASE_URL")

	db, err := sql.Open(
		"pgx",
		databaseURL,
	)
	if err != nil {
		return err
	}

	if err = db.Ping(); err != nil {
		return err
	}

	DB = db

	return nil
}