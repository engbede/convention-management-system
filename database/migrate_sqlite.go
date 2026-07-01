package database

func MigrateSQLite() error {

	query := `
	CREATE TABLE IF NOT EXISTS registrations (

		id INTEGER PRIMARY KEY AUTOINCREMENT,

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
	`

	_, err := DB.Exec(query)
	if err != nil {
		return err
	}

	// Add checked_in column for older databases.
	_, _ = DB.Exec(`
		ALTER TABLE registrations
		ADD COLUMN checked_in INTEGER DEFAULT 0
	`)

	// Add bible_study_group column for older databases.
	_, _ = DB.Exec(`
		ALTER TABLE registrations
		ADD COLUMN bible_study_group INTEGER DEFAULT 0
	`)

	return nil
}