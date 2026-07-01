package database

import "fmt"

func MigratePostgres() error {

	fmt.Println("Running PostgreSQL migration...")

	conventionsTable := `
	CREATE TABLE IF NOT EXISTS conventions (
		id BIGSERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		theme TEXT,
		venue TEXT,
		start_date DATE,
		end_date DATE,
		year INTEGER NOT NULL,
		active BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := DB.Exec(conventionsTable); err != nil {
		return err
	}

	fmt.Println("Creating conventions table...")

	registrationsTable := `
	CREATE TABLE IF NOT EXISTS registrations (
		id BIGSERIAL PRIMARY KEY,
		convention_id BIGINT REFERENCES conventions(id) ON DELETE CASCADE,

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

	if _, err := DB.Exec(registrationsTable); err != nil {
		return err
	}

	fmt.Println("Creating registrations table...")

	adminsTable := `
	CREATE TABLE IF NOT EXISTS admins (
		id BIGSERIAL PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL
	);
	`

	if _, err := DB.Exec(adminsTable); err != nil {
		return err
	}

	fmt.Println("Creating admins table...")

	return nil
}