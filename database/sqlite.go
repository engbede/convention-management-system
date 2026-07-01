package database

import (
	"database/sql"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB() error {

	if err := os.MkdirAll("/var/data", 0755); err != nil {
	return err
}
	var err error

	DB, err = sql.Open(
	"sqlite3",
	"/var/data/convention.db",
)

	if err != nil {
		return err
	}

	return DB.Ping()
}
