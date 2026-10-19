package database

import (
	"database/sql"
	"fmt"
	"strings"
)

func execMigration(query string) error {
	_, err := DB.Exec(query)
	if err != nil {
		preview := strings.TrimSpace(query)
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		return fmt.Errorf("sqlite migration failed executing [%s]: %w", preview, err)
	}
	return nil
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

	for _, colQ := range []string{
		`ALTER TABLE registrations ADD COLUMN registration_number TEXT`,
		`ALTER TABLE registrations ADD COLUMN email TEXT`,
		`ALTER TABLE registrations ADD COLUMN relationship TEXT`,
		`ALTER TABLE registrations ADD COLUMN address TEXT`,
		`ALTER TABLE registrations ADD COLUMN qr_code TEXT`,
		`ALTER TABLE registrations ADD COLUMN checked_in INTEGER DEFAULT 0`,
		`ALTER TABLE registrations ADD COLUMN bible_study_group INTEGER DEFAULT 0`,
	} {
		if err := addColumn(colQ); err != nil {
			return err
		}
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
    join_policy TEXT DEFAULT 'approval',
    disappearing_duration INTEGER DEFAULT 0,
    created_by INTEGER NOT NULL,
    last_message_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(created_by) REFERENCES users(id) ON DELETE CASCADE
);
`); err != nil {
		return err
	}

	for _, colQ := range []string{
		`ALTER TABLE conversations ADD COLUMN category TEXT DEFAULT 'general'`,
		`ALTER TABLE conversations ADD COLUMN description TEXT DEFAULT ''`,
		`ALTER TABLE conversations ADD COLUMN avatar_url TEXT DEFAULT ''`,
		`ALTER TABLE conversations ADD COLUMN call_permission TEXT DEFAULT 'all'`,
		`ALTER TABLE conversations ADD COLUMN is_public INTEGER DEFAULT 1`,
		`ALTER TABLE conversations ADD COLUMN join_policy TEXT DEFAULT 'approval'`,
		`ALTER TABLE conversations ADD COLUMN disappearing_duration INTEGER DEFAULT 0`,
		`ALTER TABLE conversations ADD COLUMN last_message_at DATETIME`,
		`ALTER TABLE conversations ADD COLUMN updated_at DATETIME`,
	} {
		if err := addColumn(colQ); err != nil {
			return err
		}
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
    muted_until DATETIME,
    is_pinned BOOLEAN DEFAULT 0,
    cleared_at DATETIME,
    is_archived BOOLEAN DEFAULT 0,
    joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(conversation_id, user_id),
    FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);
`); err != nil {
		return err
	}

	for _, colQ := range []string{
		`ALTER TABLE conversation_members ADD COLUMN muted_until DATETIME`,
		`ALTER TABLE conversation_members ADD COLUMN is_pinned BOOLEAN DEFAULT 0`,
		`ALTER TABLE conversation_members ADD COLUMN cleared_at DATETIME`,
	} {
		if err := addColumn(colQ); err != nil {
			return err
		}
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
    deleted_by INTEGER,
    deleted_at DATETIME,
    attachment_url TEXT,
    attachment_name TEXT,
    attachment_size INTEGER DEFAULT 0,
    attachment_type TEXT,
    expires_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
    FOREIGN KEY(sender_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(reply_to_message_id) REFERENCES messages(id) ON DELETE SET NULL,
    FOREIGN KEY(deleted_by) REFERENCES users(id) ON DELETE SET NULL
);
`); err != nil {
		return err
	}

	for _, colQ := range []string{
		`ALTER TABLE messages ADD COLUMN client_message_id TEXT`,
		`ALTER TABLE messages ADD COLUMN reply_to_message_id INTEGER`,
		`ALTER TABLE messages ADD COLUMN is_edited BOOLEAN DEFAULT 0`,
		`ALTER TABLE messages ADD COLUMN is_deleted BOOLEAN DEFAULT 0`,
		`ALTER TABLE messages ADD COLUMN deleted_by INTEGER`,
		`ALTER TABLE messages ADD COLUMN deleted_at DATETIME`,
		`ALTER TABLE messages ADD COLUMN attachment_url TEXT`,
		`ALTER TABLE messages ADD COLUMN attachment_name TEXT`,
		`ALTER TABLE messages ADD COLUMN attachment_size INTEGER DEFAULT 0`,
		`ALTER TABLE messages ADD COLUMN attachment_type TEXT`,
		`ALTER TABLE messages ADD COLUMN expires_at DATETIME`,
	} {
		if err := addColumn(colQ); err != nil {
			return err
		}
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
CREATE INDEX IF NOT EXISTS idx_messages_expires_at
ON messages(expires_at);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS message_deletions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    deleted_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(message_id, user_id),
    FOREIGN KEY(message_id) REFERENCES messages(id) ON DELETE CASCADE,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE INDEX IF NOT EXISTS idx_message_deletions_user
ON message_deletions(user_id, message_id);
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
    evidence_text TEXT DEFAULT '',
    evidence_url TEXT DEFAULT '',
    reviewed_by INTEGER,
    reviewed_at DATETIME,
    resolution_notes TEXT DEFAULT '',
    resolution_action TEXT DEFAULT 'none',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(reporter_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(reported_user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE SET NULL,
    FOREIGN KEY(message_id) REFERENCES messages(id) ON DELETE SET NULL,
    FOREIGN KEY(reviewed_by) REFERENCES users(id) ON DELETE SET NULL
);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE INDEX IF NOT EXISTS idx_chat_reports_reporter ON chat_reports(reporter_id);
CREATE INDEX IF NOT EXISTS idx_chat_reports_reported ON chat_reports(reported_user_id);
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
CREATE TABLE IF NOT EXISTS call_invitations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id INTEGER NOT NULL,
    token TEXT UNIQUE NOT NULL,
    call_type TEXT NOT NULL DEFAULT 'audio',
    created_by INTEGER NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
    FOREIGN KEY(created_by) REFERENCES users(id) ON DELETE CASCADE
);
`); err != nil {
		return err
	}

	if err := execMigration(`
CREATE INDEX IF NOT EXISTS idx_call_invitations_token ON call_invitations(token);
CREATE INDEX IF NOT EXISTS idx_call_invitations_conv ON call_invitations(conversation_id);
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

	if err := execMigration(`
CREATE INDEX IF NOT EXISTS idx_conversations_category
ON conversations(category);
`); err != nil {
		return err
	}

	if err := seedDefaultCommunityGroups(); err != nil {
		return err
	}

	if err := execMigration(`
CREATE TABLE IF NOT EXISTS password_resets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    email TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at DATETIME NOT NULL,
    used INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_password_resets_token_hash ON password_resets(token_hash);
CREATE INDEX IF NOT EXISTS idx_password_resets_user_id ON password_resets(user_id);
`); err != nil {
		return err
	}

	if err := migrateLiveProgrammesTables(); err != nil {
		return err
	}

	if err := migrateGroupGovernanceAndModeration(); err != nil {
		return err
	}

	if err := migrateChatReportsColumns(); err != nil {
		return err
	}

	return nil
}

func migrateChatReportsColumns() error {
	queries := []string{
		`ALTER TABLE chat_reports ADD COLUMN evidence_text TEXT DEFAULT ''`,
		`ALTER TABLE chat_reports ADD COLUMN evidence_url TEXT DEFAULT ''`,
		`ALTER TABLE chat_reports ADD COLUMN reviewed_by INTEGER`,
		`ALTER TABLE chat_reports ADD COLUMN reviewed_at DATETIME`,
		`ALTER TABLE chat_reports ADD COLUMN resolution_notes TEXT DEFAULT ''`,
		`ALTER TABLE chat_reports ADD COLUMN resolution_action TEXT DEFAULT 'none'`,
	}

	for _, query := range queries {
		if err := addColumn(query); err != nil {
			return err
		}
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
	if err == sql.ErrNoRows {
		// No users yet, skip seeding until initial user signs up
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed querying initial user for default groups: %w", err)
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
		if err := DB.QueryRow("SELECT COUNT(*) FROM conversations WHERE type = 'group' AND title = ?", g.title).Scan(&exists); err != nil {
			return fmt.Errorf("failed checking group existence for %q: %w", g.title, err)
		}
		if exists == 0 {
			res, err := DB.Exec(`
				INSERT INTO conversations (type, title, category, description, call_permission, is_public, created_by, created_at, updated_at)
				VALUES ('group', ?, ?, ?, 'all', 1, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			`, g.title, g.category, g.description, firstUserID)
			if err != nil {
				return fmt.Errorf("failed inserting community group %q: %w", g.title, err)
			}
			convID, err := res.LastInsertId()
			if err != nil {
				return fmt.Errorf("failed getting last insert id for group %q: %w", g.title, err)
			}
			if convID > 0 {
				if _, err := DB.Exec(`
					INSERT INTO conversation_members (conversation_id, user_id, role, joined_at)
					VALUES (?, ?, 'owner', CURRENT_TIMESTAMP)
				`, convID, firstUserID); err != nil {
					return fmt.Errorf("failed inserting owner member for group %q: %w", g.title, err)
				}
			}
		}
	}

	// Ensure all existing users are enrolled in the open "Youth General Community"
	if DB != nil {
		_, _ = DB.Exec(`
			INSERT INTO conversation_members (conversation_id, user_id, role, joined_at)
			SELECT c.id, u.id, 'member', CURRENT_TIMESTAMP
			FROM conversations c
			CROSS JOIN users u
			WHERE c.type = 'group' AND c.title = 'Youth General Community'
			  AND NOT EXISTS (
				  SELECT 1 FROM conversation_members cm
				  WHERE cm.conversation_id = c.id AND cm.user_id = u.id
			  );
		`)
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
		`ALTER TABLE users ADD COLUMN updated_at DATETIME`,
	}

	for _, query := range queries {

		if err := addColumn(query); err != nil {
			return err
		}
	}

	return nil
}

func migrateLiveProgrammesTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS programmes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			description TEXT DEFAULT '',
			category TEXT NOT NULL DEFAULT 'service',
			organizer_id INTEGER NOT NULL,
			schedule_start_time DATETIME NOT NULL,
			schedule_end_time DATETIME NOT NULL,
			timezone TEXT NOT NULL DEFAULT 'UTC',
			visibility TEXT NOT NULL DEFAULT 'public',
			youtube_video_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft',
			cancellation_reason TEXT DEFAULT '',
			recording_available INTEGER DEFAULT 0,
			recording_video_id TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(organizer_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_programmes_status ON programmes(status);`,
		`CREATE INDEX IF NOT EXISTS idx_programmes_visibility ON programmes(visibility);`,
		`CREATE INDEX IF NOT EXISTS idx_programmes_category ON programmes(category);`,
		`CREATE INDEX IF NOT EXISTS idx_programmes_start_time ON programmes(schedule_start_time);`,
		`CREATE INDEX IF NOT EXISTS idx_programmes_organizer ON programmes(organizer_id);`,

		`CREATE TABLE IF NOT EXISTS programme_moderators (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			programme_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			assigned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(programme_id) REFERENCES programmes(id) ON DELETE CASCADE,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(programme_id, user_id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_programme_moderators_prog ON programme_moderators(programme_id);`,
		`CREATE INDEX IF NOT EXISTS idx_programme_moderators_user ON programme_moderators(user_id);`,

		`CREATE TABLE IF NOT EXISTS programme_chat_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			programme_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			message TEXT NOT NULL,
			is_hidden INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(programme_id) REFERENCES programmes(id) ON DELETE CASCADE,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_programme_chat_prog_time ON programme_chat_messages(programme_id, created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_programme_chat_user ON programme_chat_messages(user_id);`,

		`CREATE TABLE IF NOT EXISTS programme_prayer_requests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			programme_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			request TEXT NOT NULL,
			is_private INTEGER DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(programme_id) REFERENCES programmes(id) ON DELETE CASCADE,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_programme_prayer_prog_time ON programme_prayer_requests(programme_id, created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_programme_prayer_user ON programme_prayer_requests(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_programme_prayer_private ON programme_prayer_requests(is_private);`,

		`CREATE TABLE IF NOT EXISTS programme_notifications_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			programme_id INTEGER NOT NULL,
			notification_type TEXT NOT NULL,
			sent_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(programme_id) REFERENCES programmes(id) ON DELETE CASCADE,
			UNIQUE(programme_id, notification_type)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_programme_notif_log ON programme_notifications_log(programme_id, notification_type);`,
	}

	for _, q := range queries {
		if err := execMigration(q); err != nil {
			return err
		}
	}
	return nil
}

func migrateGroupGovernanceAndModeration() error {
	// 1. Add join_policy to conversations
	if err := addColumn(`ALTER TABLE conversations ADD COLUMN join_policy TEXT DEFAULT 'approval'`); err != nil {
		return fmt.Errorf("failed adding join_policy column: %w", err)
	}

	// 2. Ensure existing group 2 ("Youth General Community") has open join policy, preserving other groups
	if DB != nil {
		if _, err := DB.Exec(`UPDATE conversations SET join_policy = 'open' WHERE id = 2 AND (join_policy IS NULL OR join_policy = '' OR join_policy = 'approval')`); err != nil {
			return fmt.Errorf("failed setting group 2 join policy: %w", err)
		}
	}

	// 3. Add moderation columns to messages
	if err := addColumn(`ALTER TABLE messages ADD COLUMN deleted_by INTEGER`); err != nil {
		return fmt.Errorf("failed adding deleted_by column: %w", err)
	}
	if err := addColumn(`ALTER TABLE messages ADD COLUMN deleted_at DATETIME`); err != nil {
		return fmt.Errorf("failed adding deleted_at column: %w", err)
	}

	// 4. Create group_membership_requests table and indexes
	queries := []string{
		`CREATE TABLE IF NOT EXISTS group_membership_requests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			request_message TEXT DEFAULT '',
			reviewed_by INTEGER,
			reviewed_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(reviewed_by) REFERENCES users(id) ON DELETE SET NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_group_req_conv_status ON group_membership_requests(conversation_id, status);`,
		`CREATE INDEX IF NOT EXISTS idx_group_req_user ON group_membership_requests(user_id);`,
	}

	for _, q := range queries {
		if err := execMigration(q); err != nil {
			return err
		}
	}

	// 5. Reconcile group administrators: ensure every group has at least one administrator (owner or admin)
	if DB != nil {
		if _, err := DB.Exec(`
			INSERT INTO conversation_members (conversation_id, user_id, role, joined_at)
			SELECT c.id, c.created_by, 'owner', CURRENT_TIMESTAMP
			FROM conversations c
			WHERE c.type = 'group'
			  AND NOT EXISTS (
				  SELECT 1 FROM conversation_members cm
				  WHERE cm.conversation_id = c.id AND cm.role IN ('owner', 'admin')
			  )
			  AND EXISTS (SELECT 1 FROM users u WHERE u.id = c.created_by)
		`); err != nil {
			return fmt.Errorf("failed reconciling group administrators: %w", err)
		}
	}

	// 6. Add email column to admins table for password recovery
	_ = addColumn(`ALTER TABLE admins ADD COLUMN email TEXT DEFAULT ''`)

	// 7. Community Prayer Requests and Intercession Tables
	communityPrayerQueries := []string{
		`CREATE TABLE IF NOT EXISTS community_prayer_requests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL,
			privacy_scope TEXT NOT NULL DEFAULT 'public',
			pray_count INTEGER DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_comm_prayer_user ON community_prayer_requests(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_comm_prayer_scope ON community_prayer_requests(privacy_scope);`,
		`CREATE INDEX IF NOT EXISTS idx_comm_prayer_status ON community_prayer_requests(status);`,
		`CREATE TABLE IF NOT EXISTS prayer_request_prayers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			prayer_request_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(prayer_request_id, user_id),
			FOREIGN KEY(prayer_request_id) REFERENCES community_prayer_requests(id) ON DELETE CASCADE,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_pr_prayers_req ON prayer_request_prayers(prayer_request_id);`,
		`CREATE INDEX IF NOT EXISTS idx_pr_prayers_user ON prayer_request_prayers(user_id);`,
	}

	for _, q := range communityPrayerQueries {
		if err := execMigration(q); err != nil {
			return err
		}
	}

	// 8. Terms and Conditions Acceptance Table
	termsQueries := []string{
		`CREATE TABLE IF NOT EXISTS terms_acceptances (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			terms_version TEXT NOT NULL,
			accepted_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			ip_address TEXT DEFAULT '',
			user_agent TEXT DEFAULT '',
			UNIQUE(user_id, terms_version),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_terms_acceptances_user ON terms_acceptances(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_terms_acceptances_user_ver ON terms_acceptances(user_id, terms_version);`,
	}

	for _, q := range termsQueries {
		if err := execMigration(q); err != nil {
			return err
		}
	}

	// 9. Web Push Subscriptions Table
	pushQueries := []string{
		`CREATE TABLE IF NOT EXISTS push_subscriptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			endpoint TEXT NOT NULL UNIQUE,
			p256dh TEXT NOT NULL,
			auth TEXT NOT NULL,
			user_agent TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_push_subs_user ON push_subscriptions(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_push_subs_endpoint ON push_subscriptions(endpoint);`,
	}

	for _, q := range pushQueries {
		if err := execMigration(q); err != nil {
			return err
		}
	}

	// Add expires_at column to call_logs if not present
	_ = addColumn(`ALTER TABLE call_logs ADD COLUMN expires_at DATETIME`)

	return nil
}


