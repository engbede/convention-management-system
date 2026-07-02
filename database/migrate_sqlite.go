package database

func MigrateSQLite() error {

	query := `
CREATE TABLE IF NOT EXISTS registrations (

	id INTEGER PRIMARY KEY AUTOINCREMENT,

	convention_id INTEGER,

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

	first_time_attendee INTEGER DEFAULT 0,

	checked_in INTEGER DEFAULT 0,

	bible_study_group INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS admins (

	id INTEGER PRIMARY KEY AUTOINCREMENT,

	username TEXT UNIQUE NOT NULL,

	password TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS conventions (

	id INTEGER PRIMARY KEY AUTOINCREMENT,

	name TEXT NOT NULL,

	theme TEXT,

	venue TEXT,

	start_date TEXT,

	end_date TEXT,

	year INTEGER,

	active INTEGER DEFAULT 0
);
`
	_, err := DB.Exec(query)
	if err != nil {
		return err
	}

	_, _ = DB.Exec(`
		ALTER TABLE registrations
		ADD COLUMN checked_in INTEGER DEFAULT 0
	`)

	_, _ = DB.Exec(`
    ALTER TABLE registrations
    ADD COLUMN convention_id INTEGER
	`)

	_, _ = DB.Exec(`
		ALTER TABLE registrations
		ADD COLUMN bible_study_group INTEGER DEFAULT 0
	`)

	return nil
}
