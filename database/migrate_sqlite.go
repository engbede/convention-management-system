package database

func execMigration(query string) error {
	_, err := DB.Exec(query)
	return err
}

func MigrateSQLite() error {

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS registrations (

	id INTEGER PRIMARY KEY AUTOINCREMENT,

	convention_id INTEGER,

	full_name TEXT NOT NULL,

	gender TEXT NOT NULL,

	age INTEGER,

	phone TEXT UNIQUE NOT NULL,

	email TEXT,

	circuit TEXT,

	local_church TEXT,

	membership TEXT,

	position TEXT,

	marital_status TEXT,

	occupation TEXT,

	emergency_contact_name TEXT,

	emergency_contact_phone TEXT,

	relationship TEXT,

	address TEXT,

	arrival_date TEXT,

	first_time_attendee INTEGER DEFAULT 0,

	checked_in INTEGER DEFAULT 0,

	bible_study_group INTEGER DEFAULT 0,

	registration_number TEXT,

	qr_code TEXT

);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS admins (

	id INTEGER PRIMARY KEY AUTOINCREMENT,

	username TEXT UNIQUE NOT NULL,

	password TEXT NOT NULL

);
`); err != nil {
		return err
	}

	if err := execMigration(`
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
`); err != nil {
		return err
	}

	if err := execMigration(`
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
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS finance (

	id INTEGER PRIMARY KEY AUTOINCREMENT,

	type TEXT NOT NULL,

	category TEXT NOT NULL,

	description TEXT,

	amount REAL NOT NULL,

	recorded_by TEXT,

	date TEXT NOT NULL

);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS settings (

	id INTEGER PRIMARY KEY AUTOINCREMENT,

	key TEXT UNIQUE,

	value TEXT

);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS notices (

	id INTEGER PRIMARY KEY AUTOINCREMENT,

	title TEXT NOT NULL,

	message TEXT NOT NULL,

	audience TEXT NOT NULL,

	priority TEXT NOT NULL,

	pinned INTEGER DEFAULT 0,

	start_date TEXT,

	end_date TEXT,

	created_by TEXT,

	created_at DATETIME DEFAULT CURRENT_TIMESTAMP

);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS inquiries (

	id INTEGER PRIMARY KEY AUTOINCREMENT,

	name TEXT NOT NULL,

	phone TEXT,

	email TEXT,

	subject TEXT,

	message TEXT NOT NULL,

	status TEXT DEFAULT 'Pending',

	created_at DATETIME DEFAULT CURRENT_TIMESTAMP

);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS inquiry_replies (

	id INTEGER PRIMARY KEY AUTOINCREMENT,

	inquiry_id INTEGER NOT NULL,

	admin_name TEXT NOT NULL,

	message TEXT NOT NULL,

	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

	FOREIGN KEY(inquiry_id)
	REFERENCES inquiries(id)
	ON DELETE CASCADE

);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS documents (

	id INTEGER PRIMARY KEY AUTOINCREMENT,

	title TEXT NOT NULL,

	category TEXT NOT NULL,

	convention TEXT,

	year INTEGER,

	description TEXT,

	content TEXT,

	created_by TEXT,

	created_at DATETIME DEFAULT CURRENT_TIMESTAMP

);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS document_files (

	id INTEGER PRIMARY KEY AUTOINCREMENT,

	document_id INTEGER NOT NULL,

	file_name TEXT NOT NULL,

	file_path TEXT NOT NULL,

	file_type TEXT,

	uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP,

	FOREIGN KEY(document_id)
	REFERENCES documents(id)
	ON DELETE CASCADE

);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS users (

	id INTEGER PRIMARY KEY AUTOINCREMENT,

	full_name TEXT NOT NULL,

	username TEXT UNIQUE NOT NULL,

	email TEXT UNIQUE,

	phone TEXT UNIQUE,

	password_hash TEXT NOT NULL,

	role TEXT DEFAULT 'attendee',

	bio TEXT,

	gender TEXT,

	birth_date TEXT,

	location TEXT,

	website TEXT,

	profile_photo TEXT,

	cover_photo TEXT,

	is_verified INTEGER DEFAULT 0,

	is_active INTEGER DEFAULT 1,

	followers INTEGER DEFAULT 0,

	following INTEGER DEFAULT 0,

	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP

);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS posts (

	id INTEGER PRIMARY KEY AUTOINCREMENT,

	user_id INTEGER NOT NULL,

	content TEXT NOT NULL,

	image TEXT,

	video TEXT,

	visibility TEXT DEFAULT 'public',

	likes INTEGER DEFAULT 0,

	comments INTEGER DEFAULT 0,

	shares INTEGER DEFAULT 0,

	is_edited INTEGER DEFAULT 0,

	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,

	FOREIGN KEY(user_id)
	REFERENCES users(id)
	ON DELETE CASCADE

);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS comments (

	id INTEGER PRIMARY KEY AUTOINCREMENT,

	post_id INTEGER NOT NULL,

	user_id INTEGER NOT NULL,

	content TEXT NOT NULL,

	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

	FOREIGN KEY(post_id)
	REFERENCES posts(id)
	ON DELETE CASCADE,

	FOREIGN KEY(user_id)
	REFERENCES users(id)
	ON DELETE CASCADE

);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS reactions (

	id INTEGER PRIMARY KEY AUTOINCREMENT,

	post_id INTEGER NOT NULL,

	user_id INTEGER NOT NULL,

	reaction TEXT NOT NULL,

	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

	FOREIGN KEY(post_id)
	REFERENCES posts(id)
	ON DELETE CASCADE,

	FOREIGN KEY(user_id)
	REFERENCES users(id)
	ON DELETE CASCADE

);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS follows (

	id INTEGER PRIMARY KEY AUTOINCREMENT,

	follower_id INTEGER NOT NULL,

	following_id INTEGER NOT NULL,

	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

	FOREIGN KEY(follower_id)
	REFERENCES users(id)
	ON DELETE CASCADE,

	FOREIGN KEY(following_id)
	REFERENCES users(id)
	ON DELETE CASCADE

);
`); err != nil {
		return err
	}

	return nil
}
