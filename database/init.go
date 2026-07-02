package database

import "os"

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

	return Migrate()
}
