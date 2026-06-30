package database

func Init() error {
	if err := InitDB(); err != nil {
		return err
	}

	return Migrate()
}