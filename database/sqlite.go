package database

import (
	"database/sql"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB() error {

	// Create the data directory if it doesn't exist
	if err := os.MkdirAll("data", 0755); err != nil {
		return err
	}

	var err error

	DB, err = sql.Open(
		"sqlite3",
		"data/convention.db",
	)

	if err != nil {
		return err
	}

	// Verify the connection
	return DB.Ping()
}
