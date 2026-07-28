package database

import (
	"strings"
)

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

	-- Identity
	full_name TEXT NOT NULL,
	username TEXT UNIQUE NOT NULL,

	-- Contact
	email TEXT UNIQUE,
	phone TEXT,

	-- Authentication
	password_hash TEXT NOT NULL,
	role TEXT DEFAULT 'member',

	-- Basic Profile
	bio TEXT,
	gender TEXT,
	birth_date TEXT,
	occupation TEXT,

	-- Location
	country TEXT,
	state TEXT,
	location TEXT,
	website TEXT,

	-- Media
	profile_photo TEXT,
	cover_photo TEXT,

	-- Church Information
	church_name TEXT,
	circuit TEXT,
	local_church TEXT,
	department TEXT,
	position TEXT,

	-- Faith Journey
	favorite_bible_verse TEXT,
	life_verse TEXT,
	calling TEXT,
	spiritual_gifts TEXT,
	salvation_testimony TEXT,

	water_baptized INTEGER DEFAULT 0,
	holy_spirit_baptized INTEGER DEFAULT 0,

	-- Interests
	favorite_preacher TEXT,
	favorite_christian_book TEXT,
	favorite_worship_song TEXT,
	favorite_gospel_artist TEXT,

	skills TEXT,
	languages TEXT,
	hobbies TEXT,

	-- Personal Mission
	mission TEXT,
	vision TEXT,
	favorite_quote TEXT,

	-- Account Status
	is_verified INTEGER DEFAULT 0,
	is_active INTEGER DEFAULT 1,

	-- Social Statistics
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

    parent_id INTEGER,

    content TEXT NOT NULL,

    is_edited BOOLEAN DEFAULT 0,

    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY(post_id)
        REFERENCES posts(id)
        ON DELETE CASCADE,

    FOREIGN KEY(user_id)
        REFERENCES users(id)
        ON DELETE CASCADE,

    FOREIGN KEY(parent_id)
        REFERENCES comments(id)
        ON DELETE CASCADE

);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE INDEX IF NOT EXISTS idx_comments_post
ON comments(post_id);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE INDEX IF NOT EXISTS idx_comments_user
ON comments(user_id);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE INDEX IF NOT EXISTS idx_comments_parent
ON comments(parent_id);
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

    UNIQUE(post_id, user_id),

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
	// Upgrade old posts table

	DB.Exec(`
ALTER TABLE posts
ADD COLUMN video TEXT
`)

	DB.Exec(`
ALTER TABLE posts
ADD COLUMN visibility TEXT DEFAULT 'public'
`)

	DB.Exec(`
ALTER TABLE posts
ADD COLUMN shares INTEGER DEFAULT 0
`)

	DB.Exec(`
ALTER TABLE posts
ADD COLUMN is_edited INTEGER DEFAULT 0
`)

	DB.Exec(`
ALTER TABLE posts
ADD COLUMN updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
`)

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS follows (

    id INTEGER PRIMARY KEY AUTOINCREMENT,

    follower_id INTEGER NOT NULL,

    following_id INTEGER NOT NULL,

    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(follower_id, following_id),

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

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS notifications (

    id INTEGER PRIMARY KEY AUTOINCREMENT,

    sender_id INTEGER NOT NULL,

    receiver_id INTEGER NOT NULL,

    post_id INTEGER,

    comment_id INTEGER,

    type TEXT NOT NULL,

    message TEXT,

    is_read INTEGER DEFAULT 0,

    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY(sender_id)
        REFERENCES users(id)
        ON DELETE CASCADE,

    FOREIGN KEY(receiver_id)
        REFERENCES users(id)
        ON DELETE CASCADE,

    FOREIGN KEY(post_id)
        REFERENCES posts(id)
        ON DELETE CASCADE,

    FOREIGN KEY(comment_id)
        REFERENCES comments(id)
        ON DELETE CASCADE

);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS notifications (

    id SERIAL PRIMARY KEY,

    sender_id INTEGER NOT NULL,

    receiver_id INTEGER NOT NULL,

    post_id INTEGER,

    comment_id INTEGER,

    type TEXT NOT NULL,

    message TEXT,

    is_read BOOLEAN DEFAULT FALSE,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY(sender_id)
        REFERENCES users(id)
        ON DELETE CASCADE,

    FOREIGN KEY(receiver_id)
        REFERENCES users(id)
        ON DELETE CASCADE,

    FOREIGN KEY(post_id)
        REFERENCES posts(id)
        ON DELETE CASCADE,

    FOREIGN KEY(comment_id)
        REFERENCES comments(id)
        ON DELETE CASCADE

);
`); err != nil {
		return err
	}

	if err := execMigration(`
	CREATE TABLE IF NOT EXISTS friend_requests (

    id INTEGER PRIMARY KEY AUTOINCREMENT,

    sender_id INTEGER NOT NULL,

    receiver_id INTEGER NOT NULL,

    status TEXT NOT NULL DEFAULT 'pending',

    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY(sender_id)
        REFERENCES users(id)
        ON DELETE CASCADE,

    FOREIGN KEY(receiver_id)
        REFERENCES users(id)
        ON DELETE CASCADE,

    UNIQUE(sender_id, receiver_id)

);
`); err != nil {
		return err
	}
	if err := execMigration(`
CREATE INDEX IF NOT EXISTS idx_friend_requests_sender
ON friend_requests(sender_id);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE INDEX IF NOT EXISTS idx_friend_requests_receiver
ON friend_requests(receiver_id);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS friends (

    id INTEGER PRIMARY KEY AUTOINCREMENT,

    user1_id INTEGER NOT NULL,

    user2_id INTEGER NOT NULL,

    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY(user1_id)
        REFERENCES users(id)
        ON DELETE CASCADE,

    FOREIGN KEY(user2_id)
        REFERENCES users(id)
        ON DELETE CASCADE,

    UNIQUE(user1_id, user2_id)
);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE INDEX IF NOT EXISTS idx_friends_user1
ON friends(user1_id);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE INDEX IF NOT EXISTS idx_friends_user2
ON friends(user2_id);
`); err != nil {
		return err
	}

	if err := migrateUserProfileColumns(); err != nil {
		return err
	}
	return nil
}

func addColumn(query string) error {

	err := execMigration(query)
	if err != nil {

		if strings.Contains(err.Error(), "duplicate column name") {
			return nil
		}

		return err
	}

	return nil
}

func migrateUserProfileColumns() error {

	queries := []string{

		`ALTER TABLE users ADD COLUMN occupation TEXT`,
		`ALTER TABLE users ADD COLUMN country TEXT`,
		`ALTER TABLE users ADD COLUMN state TEXT`,

		`ALTER TABLE users ADD COLUMN church_name TEXT`,
		`ALTER TABLE users ADD COLUMN circuit TEXT`,
		`ALTER TABLE users ADD COLUMN local_church TEXT`,
		`ALTER TABLE users ADD COLUMN department TEXT`,
		`ALTER TABLE users ADD COLUMN position TEXT`,

		`ALTER TABLE users ADD COLUMN favorite_bible_verse TEXT`,
		`ALTER TABLE users ADD COLUMN life_verse TEXT`,
		`ALTER TABLE users ADD COLUMN calling TEXT`,
		`ALTER TABLE users ADD COLUMN spiritual_gifts TEXT`,
		`ALTER TABLE users ADD COLUMN salvation_testimony TEXT`,

		`ALTER TABLE users ADD COLUMN water_baptized INTEGER DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN holy_spirit_baptized INTEGER DEFAULT 0`,

		`ALTER TABLE users ADD COLUMN favorite_preacher TEXT`,
		`ALTER TABLE users ADD COLUMN favorite_christian_book TEXT`,
		`ALTER TABLE users ADD COLUMN favorite_worship_song TEXT`,
		`ALTER TABLE users ADD COLUMN favorite_gospel_artist TEXT`,

		`ALTER TABLE users ADD COLUMN skills TEXT`,
		`ALTER TABLE users ADD COLUMN languages TEXT`,
		`ALTER TABLE users ADD COLUMN hobbies TEXT`,

		`ALTER TABLE users ADD COLUMN mission TEXT`,
		`ALTER TABLE users ADD COLUMN vision TEXT`,
		`ALTER TABLE users ADD COLUMN favorite_quote TEXT`,
	}

	for _, query := range queries {

		if err := addColumn(query); err != nil {
			return err
		}
	}

	return nil
}
