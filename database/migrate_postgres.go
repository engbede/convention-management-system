package database

import "fmt"

func MigratePostgres() error {

	fmt.Println("Running PostgreSQL migration...")

	// ----------------------------
	// Conventions
	// ----------------------------
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

	// ----------------------------
	// Registrations
	// ----------------------------
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

_, err := DB.Exec(`
ALTER TABLE registrations
ADD COLUMN IF NOT EXISTS registration_number TEXT;

ALTER TABLE registrations
ADD COLUMN IF NOT EXISTS email TEXT;

ALTER TABLE registrations
ADD COLUMN IF NOT EXISTS relationship TEXT;

ALTER TABLE registrations
ADD COLUMN IF NOT EXISTS address TEXT;

ALTER TABLE registrations
ADD COLUMN IF NOT EXISTS qr_code TEXT;
`)

if err != nil {
    return err
}

	DB.Exec(`
ALTER TABLE registrations
ADD COLUMN IF NOT EXISTS qr_code TEXT;
`)

	fmt.Println("Creating registrations table...")

	// ----------------------------
	// Admins
	// ----------------------------
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

	// ----------------------------
	// Officials
	// ----------------------------
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

	fmt.Println("Creating officials table...")

	// ----------------------------
	// Notices
	// ----------------------------
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
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`

	if _, err := DB.Exec(noticesTable); err != nil {
		return err
	}

	fmt.Println("Creating notices table...")

	// ----------------------------
	// Finance
	// ----------------------------
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

	fmt.Println("Creating finance table...")

	return nil
}
