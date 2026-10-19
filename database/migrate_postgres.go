package database

import (
	"database/sql"
	"fmt"
)

func execPostgresMigration(name string, query string) error {
	if _, err := DB.Exec(query); err != nil {
		return fmt.Errorf("postgres migration %q failed: %w", name, err)
	}
	return nil
}

func MigratePostgres() error {
	fmt.Println("Running PostgreSQL migration...")

	// ----------------------------
	// Conventions
	// ----------------------------
	if err := execPostgresMigration("conventions", `
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
`); err != nil {
		return err
	}

	// ----------------------------
	// Registrations
	// ----------------------------
	if err := execPostgresMigration("registrations", `
CREATE TABLE IF NOT EXISTS registrations (
	id BIGSERIAL PRIMARY KEY,
	convention_id BIGINT REFERENCES conventions(id) ON DELETE CASCADE,
	full_name TEXT NOT NULL,
	gender TEXT NOT NULL,
	age INTEGER,
	phone TEXT UNIQUE NOT NULL,
	circuit TEXT,
	local_church TEXT,
	membership TEXT,
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

ALTER TABLE registrations ADD COLUMN IF NOT EXISTS registration_number TEXT;
ALTER TABLE registrations ADD COLUMN IF NOT EXISTS email TEXT;
ALTER TABLE registrations ADD COLUMN IF NOT EXISTS relationship TEXT;
ALTER TABLE registrations ADD COLUMN IF NOT EXISTS address TEXT;
ALTER TABLE registrations ADD COLUMN IF NOT EXISTS qr_code TEXT;

ALTER TABLE registrations ALTER COLUMN circuit DROP NOT NULL;
ALTER TABLE registrations ALTER COLUMN local_church DROP NOT NULL;
ALTER TABLE registrations ALTER COLUMN membership DROP NOT NULL;
`); err != nil {
		return err
	}

	// ----------------------------
	// Admins
	// ----------------------------
	if err := execPostgresMigration("admins", `
CREATE TABLE IF NOT EXISTS admins (
	id BIGSERIAL PRIMARY KEY,
	username TEXT UNIQUE NOT NULL,
	password TEXT NOT NULL
);
`); err != nil {
		return err
	}

	// ----------------------------
	// Officials
	// ----------------------------
	if err := execPostgresMigration("officials", `
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
`); err != nil {
		return err
	}

	// ----------------------------
	// Notices
	// ----------------------------
	if err := execPostgresMigration("notices", `
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
`); err != nil {
		return err
	}

	// ----------------------------
	// Finance
	// ----------------------------
	if err := execPostgresMigration("finance", `
CREATE TABLE IF NOT EXISTS finance (
	id BIGSERIAL PRIMARY KEY,
	type TEXT NOT NULL,
	category TEXT NOT NULL,
	description TEXT,
	amount NUMERIC(12,2) NOT NULL,
	recorded_by TEXT,
	date DATE NOT NULL
);
`); err != nil {
		return err
	}

	// ----------------------------
	// Inquiries
	// ----------------------------
	if err := execPostgresMigration("inquiries", `
CREATE TABLE IF NOT EXISTS inquiries (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	phone TEXT NOT NULL,
	email TEXT,
	subject TEXT NOT NULL,
	message TEXT NOT NULL,
	status TEXT DEFAULT 'Pending',
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`); err != nil {
		return err
	}

	// ----------------------------
	// Inquiry Replies (Equivalence with SQLite)
	// ----------------------------
	if err := execPostgresMigration("inquiry_replies", `
CREATE TABLE IF NOT EXISTS inquiry_replies (
	id BIGSERIAL PRIMARY KEY,
	inquiry_id BIGINT NOT NULL REFERENCES inquiries(id) ON DELETE CASCADE,
	admin_name TEXT NOT NULL,
	message TEXT NOT NULL,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_pg_inquiry_replies_inquiry_id ON inquiry_replies(inquiry_id);
`); err != nil {
		return err
	}

	// ----------------------------
	// Documents & Document Files
	// ----------------------------
	if err := execPostgresMigration("documents", `
CREATE TABLE IF NOT EXISTS documents (
	id BIGSERIAL PRIMARY KEY,
	title TEXT NOT NULL,
	category TEXT NOT NULL,
	convention TEXT,
	year INTEGER,
	description TEXT,
	content TEXT,
	created_by TEXT,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS document_files (
	id BIGSERIAL PRIMARY KEY,
	document_id BIGINT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
	file_name TEXT NOT NULL,
	file_path TEXT NOT NULL,
	file_type TEXT,
	uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`); err != nil {
		return err
	}

	// ----------------------------
	// Settings & Users
	// ----------------------------
	if err := execPostgresMigration("settings_and_users", `
CREATE TABLE IF NOT EXISTS settings (
	id BIGSERIAL PRIMARY KEY,
	key TEXT UNIQUE,
	value TEXT
);

CREATE TABLE IF NOT EXISTS users (
	id BIGSERIAL PRIMARY KEY,
	full_name TEXT NOT NULL,
	username TEXT UNIQUE NOT NULL,
	email TEXT UNIQUE,
	phone TEXT UNIQUE,
	password_hash TEXT NOT NULL,
	role TEXT DEFAULT 'member',
	profile_photo TEXT,
	cover_photo TEXT,
	bio TEXT,
	gender TEXT,
	date_of_birth DATE,
	birth_date TEXT,
	occupation TEXT,
	circuit TEXT,
	local_church TEXT,
	church_name TEXT,
	department TEXT,
	position TEXT,
	country TEXT,
	state TEXT,
	city TEXT,
	location TEXT,
	website TEXT,
	verified BOOLEAN DEFAULT FALSE,
	is_verified BOOLEAN DEFAULT FALSE,
	is_private BOOLEAN DEFAULT FALSE,
	is_active BOOLEAN DEFAULT TRUE,
	followers INTEGER DEFAULT 0,
	following INTEGER DEFAULT 0,
	last_seen TIMESTAMP,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`); err != nil {
		return err
	}

	// ----------------------------
	// Extended User Profile Columns (Parity with SQLite & upgrade safe)
	// ----------------------------
	if err := execPostgresMigration("users_extended_profile", `
ALTER TABLE users ADD COLUMN IF NOT EXISTS favorite_bible_verse TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS life_verse TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS calling TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS spiritual_gifts TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS salvation_testimony TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS water_baptized INTEGER DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS holy_spirit_baptized INTEGER DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS favorite_preacher TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS favorite_christian_book TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS favorite_worship_song TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS favorite_gospel_artist TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS skills TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS languages TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS hobbies TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS mission TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS vision TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS favorite_quote TEXT;

ALTER TABLE users ADD COLUMN IF NOT EXISTS occupation TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS country TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS state TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS church_name TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS circuit TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS local_church TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS department TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS position TEXT;
`); err != nil {
		return err
	}

	// ----------------------------
	// Posts, Comments, Reactions, Follows
	// ----------------------------
	if err := execPostgresMigration("social_core", `
CREATE TABLE IF NOT EXISTS posts (
	id BIGSERIAL PRIMARY KEY,
	user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	content TEXT NOT NULL,
	image TEXT,
	video TEXT,
	visibility TEXT DEFAULT 'public',
	likes INTEGER DEFAULT 0,
	comments INTEGER DEFAULT 0,
	shares INTEGER DEFAULT 0,
	is_edited BOOLEAN DEFAULT FALSE,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS comments (
	id BIGSERIAL PRIMARY KEY,
	post_id BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
	user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	parent_id BIGINT REFERENCES comments(id) ON DELETE CASCADE,
	content TEXT NOT NULL,
	is_edited BOOLEAN DEFAULT FALSE,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS reactions (
	id BIGSERIAL PRIMARY KEY,
	post_id BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
	user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	reaction TEXT NOT NULL,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(post_id, user_id)
);

CREATE TABLE IF NOT EXISTS follows (
	id BIGSERIAL PRIMARY KEY,
	follower_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	following_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(follower_id, following_id)
);

CREATE TABLE IF NOT EXISTS notifications (
	id BIGSERIAL PRIMARY KEY,
	sender_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	receiver_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	post_id BIGINT REFERENCES posts(id) ON DELETE CASCADE,
	comment_id BIGINT REFERENCES comments(id) ON DELETE CASCADE,
	type TEXT NOT NULL,
	message TEXT,
	is_read BOOLEAN DEFAULT FALSE,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS friend_requests (
	id BIGSERIAL PRIMARY KEY,
	sender_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	receiver_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	status TEXT NOT NULL DEFAULT 'pending',
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(sender_id, receiver_id)
);

CREATE TABLE IF NOT EXISTS friends (
	id BIGSERIAL PRIMARY KEY,
	user1_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	user2_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(user1_id, user2_id)
);
`); err != nil {
		return err
	}

	// ----------------------------
	// Messaging & Conversations
	// ----------------------------
	if err := execPostgresMigration("messaging", `
CREATE TABLE IF NOT EXISTS conversations (
	id BIGSERIAL PRIMARY KEY,
	type TEXT NOT NULL DEFAULT 'direct',
	title TEXT,
	category TEXT DEFAULT 'general',
	description TEXT DEFAULT '',
	avatar_url TEXT DEFAULT '',
	call_permission TEXT DEFAULT 'all',
	is_public BOOLEAN DEFAULT TRUE,
	join_policy VARCHAR(50) DEFAULT 'approval',
	disappearing_duration INTEGER DEFAULT 0,
	created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	last_message_at TIMESTAMP,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE conversations ADD COLUMN IF NOT EXISTS category TEXT DEFAULT 'general';
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS description TEXT DEFAULT '';
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS avatar_url TEXT DEFAULT '';
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS call_permission TEXT DEFAULT 'all';
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS is_public BOOLEAN DEFAULT TRUE;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS join_policy VARCHAR(50) DEFAULT 'approval';
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS disappearing_duration INTEGER DEFAULT 0;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS last_message_at TIMESTAMP;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_pg_conversations_category ON conversations(category);

CREATE TABLE IF NOT EXISTS conversation_members (
	id BIGSERIAL PRIMARY KEY,
	conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
	user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	role TEXT NOT NULL DEFAULT 'member',
	last_read_message_id BIGINT DEFAULT 0,
	last_read_at TIMESTAMP,
	is_muted BOOLEAN DEFAULT FALSE,
	muted_until TIMESTAMP,
	is_pinned BOOLEAN DEFAULT FALSE,
	cleared_at TIMESTAMP,
	is_archived BOOLEAN DEFAULT FALSE,
	joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(conversation_id, user_id)
);

ALTER TABLE conversation_members ADD COLUMN IF NOT EXISTS muted_until TIMESTAMP;
ALTER TABLE conversation_members ADD COLUMN IF NOT EXISTS is_pinned BOOLEAN DEFAULT FALSE;
ALTER TABLE conversation_members ADD COLUMN IF NOT EXISTS cleared_at TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_pg_conv_members_user ON conversation_members(user_id);
CREATE INDEX IF NOT EXISTS idx_pg_conv_members_conv ON conversation_members(conversation_id);

CREATE TABLE IF NOT EXISTS messages (
	id BIGSERIAL PRIMARY KEY,
	conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
	sender_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	message_type TEXT NOT NULL DEFAULT 'text',
	body TEXT NOT NULL,
	client_message_id TEXT,
	reply_to_message_id BIGINT REFERENCES messages(id) ON DELETE SET NULL,
	is_edited BOOLEAN DEFAULT FALSE,
	is_deleted BOOLEAN DEFAULT FALSE,
	attachment_url TEXT,
	attachment_name TEXT,
	attachment_size BIGINT DEFAULT 0,
	attachment_type TEXT,
	expires_at TIMESTAMP,
	deleted_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
	deleted_at TIMESTAMP,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE messages ADD COLUMN IF NOT EXISTS attachment_url TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS attachment_name TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS attachment_size BIGINT DEFAULT 0;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS attachment_type TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS expires_at TIMESTAMP;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS deleted_by BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_pg_messages_conv_created ON messages(conversation_id, created_at);
CREATE INDEX IF NOT EXISTS idx_pg_messages_client_id ON messages(client_message_id);
CREATE INDEX IF NOT EXISTS idx_pg_messages_expires_at ON messages(expires_at);

CREATE TABLE IF NOT EXISTS message_deletions (
	id BIGSERIAL PRIMARY KEY,
	message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
	user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	deleted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(message_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_pg_message_deletions_user ON message_deletions(user_id, message_id);
`); err != nil {
		return err
	}

	// ----------------------------
	// Moderation & Calling
	// ----------------------------
	if err := execPostgresMigration("moderation_and_calls", `
CREATE TABLE IF NOT EXISTS blocked_users (
	id BIGSERIAL PRIMARY KEY,
	blocker_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	blocked_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	reason TEXT,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(blocker_id, blocked_id)
);

CREATE INDEX IF NOT EXISTS idx_pg_blocked_users_blocker ON blocked_users(blocker_id);
CREATE INDEX IF NOT EXISTS idx_pg_blocked_users_blocked ON blocked_users(blocked_id);

CREATE TABLE IF NOT EXISTS chat_reports (
	id BIGSERIAL PRIMARY KEY,
	reporter_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	reported_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	conversation_id BIGINT REFERENCES conversations(id) ON DELETE SET NULL,
	message_id BIGINT REFERENCES messages(id) ON DELETE SET NULL,
	reason TEXT NOT NULL,
	details TEXT,
	status TEXT DEFAULT 'pending',
	evidence_text TEXT DEFAULT '',
	evidence_url TEXT DEFAULT '',
	reviewed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
	reviewed_at TIMESTAMP,
	resolution_notes TEXT DEFAULT '',
	resolution_action TEXT DEFAULT 'none',
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pg_chat_reports_reporter ON chat_reports(reporter_id);
CREATE INDEX IF NOT EXISTS idx_pg_chat_reports_reported ON chat_reports(reported_user_id);
CREATE INDEX IF NOT EXISTS idx_pg_chat_reports_status ON chat_reports(status);

ALTER TABLE chat_reports ADD COLUMN IF NOT EXISTS evidence_text TEXT DEFAULT '';
ALTER TABLE chat_reports ADD COLUMN IF NOT EXISTS evidence_url TEXT DEFAULT '';
ALTER TABLE chat_reports ADD COLUMN IF NOT EXISTS reviewed_by BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE chat_reports ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMP;
ALTER TABLE chat_reports ADD COLUMN IF NOT EXISTS resolution_notes TEXT DEFAULT '';
ALTER TABLE chat_reports ADD COLUMN IF NOT EXISTS resolution_action TEXT DEFAULT 'none';

CREATE TABLE IF NOT EXISTS call_logs (
	id BIGSERIAL PRIMARY KEY,
	conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
	caller_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	receiver_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
	call_type TEXT NOT NULL DEFAULT 'audio',
	status TEXT NOT NULL DEFAULT 'initiated',
	duration_seconds INTEGER DEFAULT 0,
	started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	ended_at TIMESTAMP
);

ALTER TABLE call_logs ALTER COLUMN receiver_id DROP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_pg_call_logs_conv ON call_logs(conversation_id);
CREATE INDEX IF NOT EXISTS idx_pg_call_logs_caller ON call_logs(caller_id);
CREATE INDEX IF NOT EXISTS idx_pg_call_logs_receiver ON call_logs(receiver_id);

CREATE TABLE IF NOT EXISTS call_participants (
	id BIGSERIAL PRIMARY KEY,
	call_id BIGINT NOT NULL REFERENCES call_logs(id) ON DELETE CASCADE,
	user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	left_at TIMESTAMP,
	duration_seconds INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_pg_call_participants_call ON call_participants(call_id);
CREATE INDEX IF NOT EXISTS idx_pg_call_participants_user ON call_participants(user_id);
`); err != nil {
		return err
	}

	// ----------------------------
	// Call Invitations
	// ----------------------------
	if err := execPostgresMigration("call_invitations", `
CREATE TABLE IF NOT EXISTS call_invitations (
	id BIGSERIAL PRIMARY KEY,
	conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
	token TEXT UNIQUE NOT NULL,
	call_type TEXT NOT NULL DEFAULT 'audio',
	created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at TIMESTAMP NOT NULL,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pg_call_invitations_token ON call_invitations(token);
CREATE INDEX IF NOT EXISTS idx_pg_call_invitations_conv ON call_invitations(conversation_id);
`); err != nil {
		return err
	}

	// ----------------------------
	// Password Resets (Equivalence with SQLite)
	// ----------------------------
	if err := execPostgresMigration("password_resets", `
CREATE TABLE IF NOT EXISTS password_resets (
	id BIGSERIAL PRIMARY KEY,
	user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	email TEXT NOT NULL,
	token_hash TEXT NOT NULL UNIQUE,
	expires_at TIMESTAMP NOT NULL,
	used INTEGER DEFAULT 0,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pg_password_resets_token_hash ON password_resets(token_hash);
CREATE INDEX IF NOT EXISTS idx_pg_password_resets_user_id ON password_resets(user_id);
`); err != nil {
		return err
	}

	// ----------------------------
	// Live Programmes
	// ----------------------------
	if err := execPostgresMigration("live_programmes", `
CREATE TABLE IF NOT EXISTS programmes (
	id BIGSERIAL PRIMARY KEY,
	title TEXT NOT NULL,
	description TEXT DEFAULT '',
	category TEXT NOT NULL DEFAULT 'service',
	organizer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	schedule_start_time TIMESTAMP NOT NULL,
	schedule_end_time TIMESTAMP NOT NULL,
	timezone TEXT NOT NULL DEFAULT 'UTC',
	visibility TEXT NOT NULL DEFAULT 'public',
	youtube_video_id TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'draft',
	cancellation_reason TEXT DEFAULT '',
	recording_available BOOLEAN DEFAULT FALSE,
	recording_video_id TEXT DEFAULT '',
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pg_programmes_status ON programmes(status);
CREATE INDEX IF NOT EXISTS idx_pg_programmes_visibility ON programmes(visibility);
CREATE INDEX IF NOT EXISTS idx_pg_programmes_category ON programmes(category);
CREATE INDEX IF NOT EXISTS idx_pg_programmes_start_time ON programmes(schedule_start_time);
CREATE INDEX IF NOT EXISTS idx_pg_programmes_organizer ON programmes(organizer_id);

CREATE TABLE IF NOT EXISTS programme_moderators (
	id BIGSERIAL PRIMARY KEY,
	programme_id BIGINT NOT NULL REFERENCES programmes(id) ON DELETE CASCADE,
	user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(programme_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_pg_programme_moderators_prog ON programme_moderators(programme_id);
CREATE INDEX IF NOT EXISTS idx_pg_programme_moderators_user ON programme_moderators(user_id);

CREATE TABLE IF NOT EXISTS programme_chat_messages (
	id BIGSERIAL PRIMARY KEY,
	programme_id BIGINT NOT NULL REFERENCES programmes(id) ON DELETE CASCADE,
	user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	message TEXT NOT NULL,
	is_hidden BOOLEAN DEFAULT FALSE,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pg_programme_chat_prog_time ON programme_chat_messages(programme_id, created_at);
CREATE INDEX IF NOT EXISTS idx_pg_programme_chat_user ON programme_chat_messages(user_id);

CREATE TABLE IF NOT EXISTS programme_prayer_requests (
	id BIGSERIAL PRIMARY KEY,
	programme_id BIGINT NOT NULL REFERENCES programmes(id) ON DELETE CASCADE,
	user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	request TEXT NOT NULL,
	is_private BOOLEAN DEFAULT FALSE,
	status TEXT NOT NULL DEFAULT 'pending',
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pg_programme_prayer_prog_time ON programme_prayer_requests(programme_id, created_at);
CREATE INDEX IF NOT EXISTS idx_pg_programme_prayer_user ON programme_prayer_requests(user_id);
CREATE INDEX IF NOT EXISTS idx_pg_programme_prayer_private ON programme_prayer_requests(is_private);

CREATE TABLE IF NOT EXISTS programme_notifications_log (
	id BIGSERIAL PRIMARY KEY,
	programme_id BIGINT NOT NULL REFERENCES programmes(id) ON DELETE CASCADE,
	notification_type TEXT NOT NULL,
	sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(programme_id, notification_type)
);

CREATE INDEX IF NOT EXISTS idx_pg_programme_notif_log ON programme_notifications_log(programme_id, notification_type);
`); err != nil {
		return err
	}

	// ----------------------------
	// Group Governance & Membership Requests
	// ----------------------------
	if err := execPostgresMigration("group_governance", `
CREATE TABLE IF NOT EXISTS group_membership_requests (
	id BIGSERIAL PRIMARY KEY,
	conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
	user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	status VARCHAR(50) NOT NULL DEFAULT 'pending',
	request_message TEXT DEFAULT '',
	reviewed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
	reviewed_at TIMESTAMP,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pg_group_req_conv_status ON group_membership_requests(conversation_id, status);
CREATE INDEX IF NOT EXISTS idx_pg_group_req_user ON group_membership_requests(user_id);

-- Ensure group 2 ("Youth General Community") has open join policy
UPDATE conversations SET join_policy = 'open' WHERE id = 2 AND (join_policy IS NULL OR join_policy = '' OR join_policy = 'approval');

-- Reconcile group administrators in PostgreSQL
INSERT INTO conversation_members (conversation_id, user_id, role, joined_at)
SELECT c.id, c.created_by, 'owner', CURRENT_TIMESTAMP
FROM conversations c
WHERE c.type = 'group'
  AND NOT EXISTS (
	  SELECT 1 FROM conversation_members cm
	  WHERE cm.conversation_id = c.id AND cm.role IN ('owner', 'admin')
  )
  AND EXISTS (SELECT 1 FROM users u WHERE u.id = c.created_by);
`); err != nil {
		return err
	}

	// ----------------------------
	// Default Community Groups Seeding
	// ----------------------------
	if err := seedDefaultCommunityGroupsPostgres(); err != nil {
		return err
	}

	// ----------------------------
	// Admin Password Recovery & Community Prayer Requests
	// ----------------------------
	if err := execPostgresMigration("community_prayer_and_admin_recovery", `
ALTER TABLE admins ADD COLUMN IF NOT EXISTS email TEXT DEFAULT '';

CREATE TABLE IF NOT EXISTS community_prayer_requests (
	id BIGSERIAL PRIMARY KEY,
	user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	title TEXT NOT NULL,
	description TEXT NOT NULL,
	privacy_scope TEXT NOT NULL DEFAULT 'public',
	pray_count INTEGER DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'active',
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pg_comm_prayer_user ON community_prayer_requests(user_id);
CREATE INDEX IF NOT EXISTS idx_pg_comm_prayer_scope ON community_prayer_requests(privacy_scope);
CREATE INDEX IF NOT EXISTS idx_pg_comm_prayer_status ON community_prayer_requests(status);

CREATE TABLE IF NOT EXISTS prayer_request_prayers (
	id BIGSERIAL PRIMARY KEY,
	prayer_request_id BIGINT NOT NULL REFERENCES community_prayer_requests(id) ON DELETE CASCADE,
	user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(prayer_request_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_pg_pr_prayers_req ON prayer_request_prayers(prayer_request_id);
CREATE INDEX IF NOT EXISTS idx_pg_pr_prayers_user ON prayer_request_prayers(user_id);

CREATE TABLE IF NOT EXISTS terms_acceptances (
	id BIGSERIAL PRIMARY KEY,
	user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	terms_version VARCHAR(32) NOT NULL,
	accepted_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	ip_address VARCHAR(64) DEFAULT '',
	user_agent TEXT DEFAULT '',
	UNIQUE(user_id, terms_version)
);

CREATE INDEX IF NOT EXISTS idx_pg_terms_acceptances_user ON terms_acceptances(user_id);
CREATE INDEX IF NOT EXISTS idx_pg_terms_acceptances_user_ver ON terms_acceptances(user_id, terms_version);

CREATE TABLE IF NOT EXISTS push_subscriptions (
	id BIGSERIAL PRIMARY KEY,
	user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	endpoint TEXT NOT NULL UNIQUE,
	p256dh TEXT NOT NULL,
	auth TEXT NOT NULL,
	user_agent TEXT DEFAULT '',
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pg_push_subs_user ON push_subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_pg_push_subs_endpoint ON push_subscriptions(endpoint);

ALTER TABLE call_logs ADD COLUMN IF NOT EXISTS expires_at TIMESTAMP WITH TIME ZONE;
`); err != nil {
		return err
	}

	return nil
}

func seedDefaultCommunityGroupsPostgres() error {
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
		return fmt.Errorf("failed querying initial user for postgres default groups: %w", err)
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
		if err := DB.QueryRow("SELECT COUNT(*) FROM conversations WHERE type = 'group' AND title = $1", g.title).Scan(&exists); err != nil {
			return fmt.Errorf("failed checking group existence in postgres for %q: %w", g.title, err)
		}
		if exists == 0 {
			var convID int64
			err := DB.QueryRow(`
				INSERT INTO conversations (type, title, category, description, call_permission, is_public, created_by, created_at, updated_at)
				VALUES ('group', $1, $2, $3, 'all', TRUE, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
				RETURNING id
			`, g.title, g.category, g.description, firstUserID).Scan(&convID)
			if err != nil {
				return fmt.Errorf("failed inserting community group %q in postgres: %w", g.title, err)
			}
			if convID > 0 {
				if _, err := DB.Exec(`
					INSERT INTO conversation_members (conversation_id, user_id, role, joined_at)
					VALUES ($1, $2, 'owner', CURRENT_TIMESTAMP)
				`, convID, firstUserID); err != nil {
					return fmt.Errorf("failed inserting owner member for group %q in postgres: %w", g.title, err)
				}
			}
		}
	}

	// Ensure all existing users are enrolled in the open "Youth General Community"
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

	return nil
}
