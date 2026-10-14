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

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS conversations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL DEFAULT 'direct',
    title TEXT,
    category TEXT DEFAULT 'general',
    description TEXT DEFAULT '',
    avatar_url TEXT DEFAULT '',
    call_permission TEXT DEFAULT 'all',
    is_public INTEGER DEFAULT 1,
    created_by INTEGER NOT NULL,
    last_message_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(created_by) REFERENCES users(id) ON DELETE CASCADE
);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE INDEX IF NOT EXISTS idx_conversations_category ON conversations(category);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS conversation_members (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    role TEXT NOT NULL DEFAULT 'member',
    last_read_message_id INTEGER DEFAULT 0,
    last_read_at DATETIME,
    is_muted BOOLEAN DEFAULT 0,
    is_archived BOOLEAN DEFAULT 0,
    joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(conversation_id, user_id),
    FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE INDEX IF NOT EXISTS idx_conv_members_user
ON conversation_members(user_id);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE INDEX IF NOT EXISTS idx_conv_members_conv
ON conversation_members(conversation_id);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id INTEGER NOT NULL,
    sender_id INTEGER NOT NULL,
    message_type TEXT NOT NULL DEFAULT 'text',
    body TEXT NOT NULL,
    client_message_id TEXT,
    reply_to_message_id INTEGER,
    is_edited BOOLEAN DEFAULT 0,
    is_deleted BOOLEAN DEFAULT 0,
    attachment_url TEXT,
    attachment_name TEXT,
    attachment_size INTEGER DEFAULT 0,
    attachment_type TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
    FOREIGN KEY(sender_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(reply_to_message_id) REFERENCES messages(id) ON DELETE SET NULL
);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE INDEX IF NOT EXISTS idx_messages_conv_created
ON messages(conversation_id, created_at);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE INDEX IF NOT EXISTS idx_messages_client_id
ON messages(client_message_id);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS blocked_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    blocker_id INTEGER NOT NULL,
    blocked_id INTEGER NOT NULL,
    reason TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(blocker_id, blocked_id),
    FOREIGN KEY(blocker_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(blocked_id) REFERENCES users(id) ON DELETE CASCADE
);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE INDEX IF NOT EXISTS idx_blocked_users_blocker ON blocked_users(blocker_id);
CREATE INDEX IF NOT EXISTS idx_blocked_users_blocked ON blocked_users(blocked_id);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS chat_reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    reporter_id INTEGER NOT NULL,
    reported_user_id INTEGER NOT NULL,
    conversation_id INTEGER,
    message_id INTEGER,
    reason TEXT NOT NULL,
    details TEXT,
    status TEXT DEFAULT 'pending',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(reporter_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(reported_user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE SET NULL,
    FOREIGN KEY(message_id) REFERENCES messages(id) ON DELETE SET NULL
);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE INDEX IF NOT EXISTS idx_chat_reports_reporter ON chat_reports(reporter_id);
CREATE INDEX IF NOT EXISTS idx_chat_reports_status ON chat_reports(status);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS call_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id INTEGER NOT NULL,
    caller_id INTEGER NOT NULL,
    receiver_id INTEGER,
    call_type TEXT NOT NULL DEFAULT 'audio',
    status TEXT NOT NULL DEFAULT 'initiated',
    duration_seconds INTEGER DEFAULT 0,
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    ended_at DATETIME,
    FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
    FOREIGN KEY(caller_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(receiver_id) REFERENCES users(id) ON DELETE SET NULL
);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE INDEX IF NOT EXISTS idx_call_logs_conv ON call_logs(conversation_id);
CREATE INDEX IF NOT EXISTS idx_call_logs_caller ON call_logs(caller_id);
CREATE INDEX IF NOT EXISTS idx_call_logs_receiver ON call_logs(receiver_id);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS call_participants (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    call_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    left_at DATETIME,
    duration_seconds INTEGER DEFAULT 0,
    FOREIGN KEY(call_id) REFERENCES call_logs(id) ON DELETE CASCADE,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE INDEX IF NOT EXISTS idx_call_participants_call ON call_participants(call_id);
CREATE INDEX IF NOT EXISTS idx_call_participants_user ON call_participants(user_id);
`); err != nil {
		return err
	}

	if err := migrateUserProfileColumns(); err != nil {
		return err
	}

	if err := migrateChatMessageAttachmentColumns(); err != nil {
		return err
	}

	if err := migrateConversationGroupColumns(); err != nil {
		return err
	}

	if err := seedDefaultCommunityGroups(); err != nil {
		return err
	}

	return nil
}

func migrateConversationGroupColumns() error {
	queries := []string{
		`ALTER TABLE conversations ADD COLUMN category TEXT DEFAULT 'general'`,
		`ALTER TABLE conversations ADD COLUMN description TEXT DEFAULT ''`,
		`ALTER TABLE conversations ADD COLUMN avatar_url TEXT DEFAULT ''`,
		`ALTER TABLE conversations ADD COLUMN call_permission TEXT DEFAULT 'all'`,
		`ALTER TABLE conversations ADD COLUMN is_public INTEGER DEFAULT 1`,
	}

	for _, query := range queries {
		if err := addColumn(query); err != nil {
			return err
		}
	}

	return nil
}

func seedDefaultCommunityGroups() error {
	if DB == nil {
		return nil
	}

	var firstUserID int
	err := DB.QueryRow("SELECT id FROM users ORDER BY id ASC LIMIT 1").Scan(&firstUserID)
	if err != nil {
		// No users yet, skip seeding for now
		return nil
	}

	type defaultGroup struct {
		title       string
		category    string
		description string
	}

	groups := []defaultGroup{
		{
			title:       "Youth General Community",
			category:    "general",
			description: "Open fellowship space for all convention youth, announcements, and general discussions.",
		},
		{
			title:       "Bible Study Fellowship",
			category:    "bible_study",
			description: "Weekly scripture reflections, topical bible studies, and word meditations.",
		},
		{
			title:       "Youth Prayer Ministry",
			category:    "prayer",
			description: "Intercessory prayer requests, morning devotionals, and spiritual warfare declarations.",
		},
		{
			title:       "Evangelism & Outreach Team",
			category:    "evangelism",
			description: "Gospel tract distribution, youth outreach missions, and evangelism training.",
		},
	}

	for _, g := range groups {
		var exists int
		_ = DB.QueryRow("SELECT COUNT(*) FROM conversations WHERE type = 'group' AND title = ?", g.title).Scan(&exists)
		if exists == 0 {
			res, err := DB.Exec(`
				INSERT INTO conversations (type, title, category, description, call_permission, is_public, created_by, created_at, updated_at)
				VALUES ('group', ?, ?, ?, 'all', 1, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			`, g.title, g.category, g.description, firstUserID)
			if err == nil {
				if convID, err := res.LastInsertId(); err == nil && convID > 0 {
					_, _ = DB.Exec(`
						INSERT INTO conversation_members (conversation_id, user_id, role, joined_at)
						VALUES (?, ?, 'owner', CURRENT_TIMESTAMP)
					`, convID, firstUserID)
				}
			}
		}
	}
	return nil
}

func migrateChatMessageAttachmentColumns() error {
	queries := []string{
		`ALTER TABLE messages ADD COLUMN attachment_url TEXT`,
		`ALTER TABLE messages ADD COLUMN attachment_name TEXT`,
		`ALTER TABLE messages ADD COLUMN attachment_size INTEGER DEFAULT 0`,
		`ALTER TABLE messages ADD COLUMN attachment_type TEXT`,
	}

	for _, query := range queries {
		if err := addColumn(query); err != nil {
			return err
		}
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
