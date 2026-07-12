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
	DB.Exec(`
ALTER TABLE registrations
ADD COLUMN IF NOT EXISTS registration_number TEXT;
`)

	DB.Exec(`
ALTER TABLE registrations
ADD COLUMN IF NOT EXISTS qr_code TEXT;
`)
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

	fmt.Println("Creating officials table...")

	fmt.Println("Creating notices table...")

	noticesTable := `
CREATE TABLE IF NOT EXISTS notices (

	id BIGSERIAL PRIMARY KEY,

	title TEXT NOT NULL,

	message TEXT NOT NULL,

	audience TEXT NOT NULL,

	priority TEXT DEFAULT 'Normal',

	pinned BOOLEAN DEFAULT FALSE,

	start_date TEXT,

	end_date TEXT,

	created_by TEXT,

	created_at TIMESTAMP DEFAULT NOW()

);
`

	if _, err := DB.Exec(noticesTable); err != nil {
		return err
	}

	officialsTable := `
CREATE TABLE IF NOT EXISTS officials (

	id BIGSERIAL PRIMARY KEY,

	full_name TEXT NOT NULL,

	gender TEXT NOT NULL,

	phone TEXT UNIQUE NOT NULL,

	email TEXT UNIQUE,

	circuit TEXT NOT NULL,

	local_church TEXT NOT NULL,

	position TEXT NOT NULL,

	department TEXT NOT NULL,

	status TEXT DEFAULT 'Active',

	photo TEXT,

	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`
	if _, err := DB.Exec(officialsTable); err != nil {
		return err
	}

	financeTable := `
CREATE TABLE IF NOT EXISTS finance (

    id BIGSERIAL PRIMARY KEY,

    type TEXT NOT NULL,

    category TEXT NOT NULL,

    description TEXT,

    amount NUMERIC(12,2) NOT NULL,

    recorded_by TEXT,

    date DATE NOT NULL

);
`
	if _, err := DB.Exec(financeTable); err != nil {
		return err
	}
	return nil
}
