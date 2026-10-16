package database

import "os"

// Init initializes the database connection pool without executing migrations.
// Callers (such as main) explicitly invoke database.Migrate().
func Init() error {

	if os.Getenv("DATABASE_URL") != "" {

		if err := InitPostgres(); err != nil {
			return err
		}

	} else {

		if err := InitDB(); err != nil {
			return err
		}

	}

	return nil
}
