package database

func MigrateSQLite() error {

	query := `
CREATE TABLE IF NOT EXISTS registrations (

	id INTEGER PRIMARY KEY AUTOINCREMENT,

	convention_id INTEGER,

	full_name TEXT NOT NULL,

	gender TEXT NOT NULL,

	age INTEGER,

	phone TEXT NOT NULL UNIQUE,

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
	_, _ = DB.Exec(`
CREATE TABLE IF NOT EXISTS officials (

    id INTEGER PRIMARY KEY AUTOINCREMENT,

    full_name TEXT NOT NULL,

    gender TEXT,

    phone TEXT,

    email TEXT,

    circuit TEXT,

    local_church TEXT,

    position TEXT,

    department TEXT,

    status TEXT,

    photo TEXT,

    created_at DATETIME DEFAULT CURRENT_TIMESTAMP

);
`)

	_, _ = DB.Exec(`
CREATE TABLE IF NOT EXISTS finance (

    id INTEGER PRIMARY KEY AUTOINCREMENT,

    type TEXT NOT NULL,

    category TEXT NOT NULL,

    description TEXT,

    amount REAL NOT NULL,

    recorded_by TEXT,

    date TEXT NOT NULL

);
`)
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

	_, _ = DB.Exec(`
    ALTER TABLE registrations
    ADD COLUMN registration_number TEXT
`)

	_, _ = DB.Exec(`
    ALTER TABLE registrations
    ADD COLUMN qr_code TEXT
`)
	return nil
}
