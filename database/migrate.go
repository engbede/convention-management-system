package database

import (
	"os"
)

func Migrate() error {

	if os.Getenv("DATABASE_URL") != "" {
		return MigratePostgres()
	}

	return MigrateSQLite()

}
