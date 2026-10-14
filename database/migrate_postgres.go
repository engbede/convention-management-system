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

	// ----------------------------
	// Inquiries
	// ----------------------------
	inquiriesTable := `
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
`

	if _, err := DB.Exec(inquiriesTable); err != nil {
		return err
	}

	fmt.Println("Creating inquiries table...")

	// ----------------------------
	// Documents
	// ----------------------------
	documentsTable := `
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
`

	if _, err := DB.Exec(documentsTable); err != nil {
		return err
	}

	fmt.Println("Creating documents table...")

	// ----------------------------
	// Document Files
	// ----------------------------
	documentFilesTable := `
CREATE TABLE IF NOT EXISTS document_files (
	id BIGSERIAL PRIMARY KEY,

	document_id BIGINT NOT NULL
		REFERENCES documents(id)
		ON DELETE CASCADE,

	file_name TEXT NOT NULL,

	file_path TEXT NOT NULL,

	file_type TEXT,

	uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`

	if _, err := DB.Exec(documentFilesTable); err != nil {
		return err
	}

	fmt.Println("Creating document_files table...")

	_, err = DB.Exec(`
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
    role TEXT DEFAULT 'attendee',
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

CREATE TABLE IF NOT EXISTS posts (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    content TEXT NOT NULL,
    image TEXT,
    video TEXT,
    visibility TEXT DEFAULT 'public',
    likes INTEGER DEFAULT 0,
    comments INTEGER DEFAULT 0,
    shares INTEGER DEFAULT 0,
    is_edited BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id)
        REFERENCES users(id)
        ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS comments (
    id BIGSERIAL PRIMARY KEY,
    post_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    parent_id BIGINT,
    content TEXT NOT NULL,
    is_edited BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(post_id) REFERENCES posts(id) ON DELETE CASCADE,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(parent_id) REFERENCES comments(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS reactions (
    id BIGSERIAL PRIMARY KEY,
    post_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    reaction TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(post_id, user_id),
    FOREIGN KEY(post_id) REFERENCES posts(id) ON DELETE CASCADE,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS follows (
    id BIGSERIAL PRIMARY KEY,
    follower_id BIGINT NOT NULL,
    following_id BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(follower_id, following_id),
    FOREIGN KEY(follower_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(following_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS notifications (
    id BIGSERIAL PRIMARY KEY,
    sender_id BIGINT NOT NULL,
    receiver_id BIGINT NOT NULL,
    post_id BIGINT,
    comment_id BIGINT,
    type TEXT NOT NULL,
    message TEXT,
    is_read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(sender_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(receiver_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(post_id) REFERENCES posts(id) ON DELETE CASCADE,
    FOREIGN KEY(comment_id) REFERENCES comments(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS friend_requests (
    id BIGSERIAL PRIMARY KEY,
    sender_id BIGINT NOT NULL,
    receiver_id BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(sender_id, receiver_id),
    FOREIGN KEY(sender_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(receiver_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS friends (
    id BIGSERIAL PRIMARY KEY,
    user1_id BIGINT NOT NULL,
    user2_id BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user1_id, user2_id),
    FOREIGN KEY(user1_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(user2_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS conversations (
    id BIGSERIAL PRIMARY KEY,
    type TEXT NOT NULL DEFAULT 'direct',
    title TEXT,
    category TEXT DEFAULT 'general',
    description TEXT DEFAULT '',
    avatar_url TEXT DEFAULT '',
    call_permission TEXT DEFAULT 'all',
    is_public BOOLEAN DEFAULT TRUE,
    created_by BIGINT NOT NULL,
    last_message_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(created_by) REFERENCES users(id) ON DELETE CASCADE
);

ALTER TABLE conversations ADD COLUMN IF NOT EXISTS category TEXT DEFAULT 'general';
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS description TEXT DEFAULT '';
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS avatar_url TEXT DEFAULT '';
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS call_permission TEXT DEFAULT 'all';
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS is_public BOOLEAN DEFAULT TRUE;

CREATE INDEX IF NOT EXISTS idx_pg_conversations_category ON conversations(category);

CREATE TABLE IF NOT EXISTS conversation_members (
    id BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member',
    last_read_message_id BIGINT DEFAULT 0,
    last_read_at TIMESTAMP,
    is_muted BOOLEAN DEFAULT FALSE,
    is_archived BOOLEAN DEFAULT FALSE,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(conversation_id, user_id),
    FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS messages (
    id BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT NOT NULL,
    sender_id BIGINT NOT NULL,
    message_type TEXT NOT NULL DEFAULT 'text',
    body TEXT NOT NULL,
    client_message_id TEXT,
    reply_to_message_id BIGINT,
    is_edited BOOLEAN DEFAULT FALSE,
    is_deleted BOOLEAN DEFAULT FALSE,
    attachment_url TEXT,
    attachment_name TEXT,
    attachment_size BIGINT DEFAULT 0,
    attachment_type TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
    FOREIGN KEY(sender_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(reply_to_message_id) REFERENCES messages(id) ON DELETE SET NULL
);

ALTER TABLE messages ADD COLUMN IF NOT EXISTS attachment_url TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS attachment_name TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS attachment_size BIGINT DEFAULT 0;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS attachment_type TEXT;

CREATE INDEX IF NOT EXISTS idx_pg_conv_members_user ON conversation_members(user_id);
CREATE INDEX IF NOT EXISTS idx_pg_conv_members_conv ON conversation_members(conversation_id);
CREATE INDEX IF NOT EXISTS idx_pg_messages_conv_created ON messages(conversation_id, created_at);
CREATE INDEX IF NOT EXISTS idx_pg_messages_client_id ON messages(client_message_id);

CREATE TABLE IF NOT EXISTS blocked_users (
    id BIGSERIAL PRIMARY KEY,
    blocker_id BIGINT NOT NULL,
    blocked_id BIGINT NOT NULL,
    reason TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(blocker_id, blocked_id),
    FOREIGN KEY(blocker_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(blocked_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_pg_blocked_users_blocker ON blocked_users(blocker_id);
CREATE INDEX IF NOT EXISTS idx_pg_blocked_users_blocked ON blocked_users(blocked_id);

CREATE TABLE IF NOT EXISTS chat_reports (
    id BIGSERIAL PRIMARY KEY,
    reporter_id BIGINT NOT NULL,
    reported_user_id BIGINT NOT NULL,
    conversation_id BIGINT,
    message_id BIGINT,
    reason TEXT NOT NULL,
    details TEXT,
    status TEXT DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(reporter_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(reported_user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE SET NULL,
    FOREIGN KEY(message_id) REFERENCES messages(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_pg_chat_reports_reporter ON chat_reports(reporter_id);
CREATE INDEX IF NOT EXISTS idx_pg_chat_reports_status ON chat_reports(status);

CREATE TABLE IF NOT EXISTS call_logs (
    id BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT NOT NULL,
    caller_id BIGINT NOT NULL,
    receiver_id BIGINT,
    call_type TEXT NOT NULL DEFAULT 'audio',
    status TEXT NOT NULL DEFAULT 'initiated',
    duration_seconds INTEGER DEFAULT 0,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP,
    FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
    FOREIGN KEY(caller_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY(receiver_id) REFERENCES users(id) ON DELETE SET NULL
);

ALTER TABLE call_logs ALTER COLUMN receiver_id DROP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_pg_call_logs_conv ON call_logs(conversation_id);
CREATE INDEX IF NOT EXISTS idx_pg_call_logs_caller ON call_logs(caller_id);
CREATE INDEX IF NOT EXISTS idx_pg_call_logs_receiver ON call_logs(receiver_id);

CREATE TABLE IF NOT EXISTS call_participants (
    id BIGSERIAL PRIMARY KEY,
    call_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    left_at TIMESTAMP,
    duration_seconds INTEGER DEFAULT 0,
    FOREIGN KEY(call_id) REFERENCES call_logs(id) ON DELETE CASCADE,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_pg_call_participants_call ON call_participants(call_id);
CREATE INDEX IF NOT EXISTS idx_pg_call_participants_user ON call_participants(user_id);
`)

	if err != nil {
		return err
	}

	return nil
}
