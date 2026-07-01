package database

import "fmt"

func MigratePostgres() error {

	fmt.Println("Running PostgreSQL migration...")

	registrationsTable := `
	CREATE TABLE IF NOT EXISTS registrations (
		id BIGSERIAL PRIMARY KEY,
		full_name TEXT NOT NULL,
		gender TEXT NOT NULL,
		age INTEGER,
		phone TEXT UNIQUE NOT NULL,
		circuit TEXT NOT NULL,
		local_church TEXT NOT NULL,
		membership TEXT NOT NULL,
		position TEXT,
		marital_status TEXT,
		occupation TEXT,
		emergency_contact_name TEXT,
		emergency_contact_phone TEXT,
		arrival_date TEXT,
		first_time_attendee BOOLEAN DEFAULT FALSE,
		checked_in BOOLEAN DEFAULT FALSE,
		bible_study_group INTEGER DEFAULT 0
	);
	`

	fmt.Println("Creating registrations table...")

	if _, err := DB.Exec(registrationsTable); err != nil {
		return fmt.Errorf("registrations table failed: %w", err)
	}

	adminsTable := `
	CREATE TABLE IF NOT EXISTS admins (
		id BIGSERIAL PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL
	);
	`

	fmt.Println("Creating admins table...")

	if _, err := DB.Exec(adminsTable); err != nil {
		return fmt.Errorf("admins table failed: %w", err)
	}

	fmt.Println("Migration completed.")

	return nil
}