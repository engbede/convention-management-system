package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

var expectedTables = []string{
	"registrations",
	"admins",
	"conventions",
	"officials",
	"finance",
	"settings",
	"notices",
	"inquiries",
	"inquiry_replies",
	"documents",
	"document_files",
	"users",
	"posts",
	"comments",
	"reactions",
	"follows",
	"notifications",
	"friend_requests",
	"friends",
	"conversations",
	"conversation_members",
	"messages",
	"blocked_users",
	"chat_reports",
	"call_logs",
	"call_participants",
	"password_resets",
	"programmes",
	"programme_moderators",
	"programme_chat_messages",
	"programme_prayer_requests",
	"programme_notifications_log",
	"group_membership_requests",
}

// setupTestDB creates an isolated SQLite database file for testing and restores original DB when done.
func setupTestDB(t *testing.T, name string) (*sql.DB, func()) {
	t.Helper()

	origDB := DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, name+".db")

	testDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open test database %s: %v", name, err)
	}

	DB = testDB

	cleanup := func() {
		testDB.Close()
		DB = origDB
	}

	return testDB, cleanup
}

func getTableList(t *testing.T, db *sql.DB) map[string]bool {
	t.Helper()
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		t.Fatalf("failed querying tables: %v", err)
	}
	defer rows.Close()

	tables := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("failed scanning table name: %v", err)
		}
		tables[name] = true
	}
	return tables
}

func hasColumn(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		t.Fatalf("failed querying table_info for %s: %v", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notnull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("failed scanning column info: %v", err)
		}
		if name == column {
			return true
		}
	}
	return false
}

// 1. Fresh Database Migration Test
func TestFreshDatabaseMigration(t *testing.T) {
	db, cleanup := setupTestDB(t, "test_fresh_migration")
	defer cleanup()

	if err := MigrateSQLite(); err != nil {
		t.Fatalf("fresh MigrateSQLite() failed: %v", err)
	}

	tables := getTableList(t, db)
	for _, expected := range expectedTables {
		if !tables[expected] {
			t.Errorf("expected table %q was not created by MigrateSQLite()", expected)
		}
	}

	// Verify required columns on users
	expectedUserCols := []string{
		"favorite_bible_verse", "life_verse", "calling", "spiritual_gifts",
		"salvation_testimony", "water_baptized", "holy_spirit_baptized",
		"favorite_preacher", "favorite_christian_book", "favorite_worship_song",
		"favorite_gospel_artist", "skills", "languages", "hobbies",
		"mission", "vision", "favorite_quote",
	}
	for _, col := range expectedUserCols {
		if !hasColumn(t, db, "users", col) {
			t.Errorf("expected column users.%s was not found", col)
		}
	}

	// Verify required columns on conversations
	if !hasColumn(t, db, "conversations", "join_policy") {
		t.Errorf("expected column conversations.join_policy was not found")
	}

	// Verify required columns on messages
	if !hasColumn(t, db, "messages", "deleted_by") {
		t.Errorf("expected column messages.deleted_by was not found")
	}
	if !hasColumn(t, db, "messages", "deleted_at") {
		t.Errorf("expected column messages.deleted_at was not found")
	}
}

// 2. Migration Idempotency Test (running 3x consecutively)
func TestMigrationIdempotency(t *testing.T) {
	db, cleanup := setupTestDB(t, "test_migration_idempotency")
	defer cleanup()

	for run := 1; run <= 3; run++ {
		if err := MigrateSQLite(); err != nil {
			t.Fatalf("MigrateSQLite() failed on run #%d: %v", run, err)
		}
	}

	tables := getTableList(t, db)
	for _, expected := range expectedTables {
		if !tables[expected] {
			t.Errorf("table %q missing after 3 consecutive migration runs", expected)
		}
	}
}

// 3. Schema Upgrade Test against pre-existing database with missing columns
func TestSchemaUpgrade_OlderDatabase(t *testing.T) {
	db, cleanup := setupTestDB(t, "test_schema_upgrade")
	defer cleanup()

	// Create legacy tables with missing columns
	legacyDDL := `
	CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		full_name TEXT NOT NULL,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE conversations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL DEFAULT 'direct',
		title TEXT,
		created_by INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		conversation_id INTEGER NOT NULL,
		sender_id INTEGER NOT NULL,
		body TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE registrations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		full_name TEXT NOT NULL,
		gender TEXT NOT NULL,
		phone TEXT UNIQUE NOT NULL
	);
	`
	if _, err := db.Exec(legacyDDL); err != nil {
		t.Fatalf("failed creating legacy tables: %v", err)
	}

	// Insert legacy data
	_, err := db.Exec(`
		INSERT INTO users (id, full_name, username, password_hash)
		VALUES (1, 'Legacy User', 'legacy_user', 'hash123');

		INSERT INTO conversations (id, type, title, created_by)
		VALUES (1, 'group', 'Legacy Group', 1);

		INSERT INTO messages (id, conversation_id, sender_id, body)
		VALUES (1, 1, 1, 'Legacy message');

		INSERT INTO registrations (id, full_name, gender, phone)
		VALUES (1, 'Legacy Registrant', 'Male', '1234567890');
	`)
	if err != nil {
		t.Fatalf("failed inserting legacy data: %v", err)
	}

	// Run migration to upgrade
	if err := MigrateSQLite(); err != nil {
		t.Fatalf("MigrateSQLite() failed upgrading older schema: %v", err)
	}

	// Verify legacy data was preserved
	var userName, userVerse string
	var waterBaptized int
	err = db.QueryRow("SELECT full_name, COALESCE(favorite_bible_verse, ''), water_baptized FROM users WHERE id = 1").
		Scan(&userName, &userVerse, &waterBaptized)
	if err != nil {
		t.Fatalf("failed querying upgraded user: %v", err)
	}
	if userName != "Legacy User" {
		t.Errorf("expected user full_name 'Legacy User', got %q", userName)
	}
	if userVerse != "" {
		t.Errorf("expected empty userVerse default, got %q", userVerse)
	}

	var groupTitle, joinPolicy string
	err = db.QueryRow("SELECT title, COALESCE(join_policy, '') FROM conversations WHERE id = 1").
		Scan(&groupTitle, &joinPolicy)
	if err != nil {
		t.Fatalf("failed querying upgraded conversation: %v", err)
	}
	if groupTitle != "Legacy Group" {
		t.Errorf("expected group title 'Legacy Group', got %q", groupTitle)
	}
	if joinPolicy != "approval" {
		t.Errorf("expected join_policy default 'approval', got %q", joinPolicy)
	}

	var msgBody string
	var deletedBy sql.NullInt64
	err = db.QueryRow("SELECT body, deleted_by FROM messages WHERE id = 1").Scan(&msgBody, &deletedBy)
	if err != nil {
		t.Fatalf("failed querying upgraded message: %v", err)
	}
	if msgBody != "Legacy message" {
		t.Errorf("expected message body 'Legacy message', got %q", msgBody)
	}
	if deletedBy.Valid {
		t.Errorf("expected deleted_by to be NULL, got %d", deletedBy.Int64)
	}

	var regName, qrCode sql.NullString
	err = db.QueryRow("SELECT full_name, qr_code FROM registrations WHERE id = 1").Scan(&regName, &qrCode)
	if err != nil {
		t.Fatalf("failed querying upgraded registration: %v", err)
	}
	if regName.String != "Legacy Registrant" {
		t.Errorf("expected registration full_name 'Legacy Registrant', got %q", regName.String)
	}
}

// 4. Seed Idempotency Test
func TestSeedIdempotency(t *testing.T) {
	db, cleanup := setupTestDB(t, "test_seed_idempotency")
	defer cleanup()

	if err := MigrateSQLite(); err != nil {
		t.Fatalf("MigrateSQLite() failed: %v", err)
	}

	// Run seeds 3 times
	for i := 1; i <= 3; i++ {
		if err := SeedAdmin(); err != nil {
			t.Fatalf("SeedAdmin() failed on run #%d: %v", i, err)
		}
		if err := SeedConvention(); err != nil {
			t.Fatalf("SeedConvention() failed on run #%d: %v", i, err)
		}
	}

	var adminCount, conventionCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM admins").Scan(&adminCount); err != nil {
		t.Fatalf("failed counting admins: %v", err)
	}
	if adminCount != 1 {
		t.Errorf("expected exactly 1 admin after repeated seeding, got %d", adminCount)
	}

	if err := db.QueryRow("SELECT COUNT(*) FROM conventions").Scan(&conventionCount); err != nil {
		t.Fatalf("failed counting conventions: %v", err)
	}
	if conventionCount != 1 {
		t.Errorf("expected exactly 1 convention after repeated seeding, got %d", conventionCount)
	}
}

// 5. Default Group Seeding and Admin Reconciliation Test
func TestDefaultGroupSeedingAndAdminReconciliation(t *testing.T) {
	db, cleanup := setupTestDB(t, "test_group_seeding")
	defer cleanup()

	if err := MigrateSQLite(); err != nil {
		t.Fatalf("MigrateSQLite() failed: %v", err)
	}

	// Insert a test user
	_, err := db.Exec(`
		INSERT INTO users (id, full_name, username, password_hash)
		VALUES (1, 'Admin User', 'admin_user', 'hash123')
	`)
	if err != nil {
		t.Fatalf("failed inserting test user: %v", err)
	}

	// Run group seed
	if err := seedDefaultCommunityGroups(); err != nil {
		t.Fatalf("seedDefaultCommunityGroups() failed: %v", err)
	}

	var groupCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM conversations WHERE type = 'group'").Scan(&groupCount); err != nil {
		t.Fatalf("failed counting groups: %v", err)
	}
	if groupCount != 4 {
		t.Errorf("expected 4 default community groups, got %d", groupCount)
	}

	// Run again to test idempotency
	if err := seedDefaultCommunityGroups(); err != nil {
		t.Fatalf("second seedDefaultCommunityGroups() failed: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM conversations WHERE type = 'group'").Scan(&groupCount); err != nil {
		t.Fatalf("failed counting groups on second run: %v", err)
	}
	if groupCount != 4 {
		t.Errorf("expected group count to remain 4 after re-seed, got %d", groupCount)
	}

	// Run group governance migration which reconciles administrators and sets Group 2 to open
	if err := migrateGroupGovernanceAndModeration(); err != nil {
		t.Fatalf("migrateGroupGovernanceAndModeration() failed: %v", err)
	}

	var group2Policy string
	if err := db.QueryRow("SELECT join_policy FROM conversations WHERE id = 2").Scan(&group2Policy); err != nil {
		t.Fatalf("failed checking group 2 join policy: %v", err)
	}
	if group2Policy != "open" {
		t.Errorf("expected group 2 join_policy to be 'open', got %q", group2Policy)
	}

	// Verify all 4 groups have owner members
	var ownerCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM conversation_members WHERE role = 'owner'").Scan(&ownerCount); err != nil {
		t.Fatalf("failed checking owner members: %v", err)
	}
	if ownerCount != 4 {
		t.Errorf("expected 4 owners in conversation_members, got %d", ownerCount)
	}
}

// 6. Error Context Propagation Test
func TestErrorPropagation(t *testing.T) {
	_, cleanup := setupTestDB(t, "test_error_propagation")
	defer cleanup()

	err := execMigration("THIS IS DELIBERATELY INVALID SQL QUERY;")
	if err == nil {
		t.Fatal("expected error from invalid SQL query, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "sqlite migration failed executing") {
		t.Errorf("expected error message to contain migration context wrapper, got: %s", errMsg)
	}
}

// 7. PostgreSQL Parity & DDL Syntax Validation Test
func TestPostgresDDLSyntaxValidation(t *testing.T) {
	// Verify that migrate_postgres.go creates all expected tables
	content, err := os.ReadFile("migrate_postgres.go")
	if err != nil {
		t.Fatalf("failed reading migrate_postgres.go: %v", err)
	}
	pgCode := string(content)

	for _, table := range expectedTables {
		pattern := fmt.Sprintf("TABLE IF NOT EXISTS %s", table)
		if !strings.Contains(pgCode, pattern) {
			t.Errorf("PostgreSQL migration is missing table definition for %q (pattern %q)", table, pattern)
		}
	}

	// Verify critical missing tables are present
	if !strings.Contains(pgCode, "inquiry_replies") {
		t.Errorf("PostgreSQL migration is missing inquiry_replies")
	}
	if !strings.Contains(pgCode, "password_resets") {
		t.Errorf("PostgreSQL migration is missing password_resets")
	}

	// Verify critical user profile columns are present in PostgreSQL migration
	expectedCols := []string{
		"favorite_bible_verse", "life_verse", "calling", "spiritual_gifts",
		"salvation_testimony", "water_baptized", "holy_spirit_baptized",
		"favorite_preacher", "favorite_christian_book", "favorite_worship_song",
		"favorite_gospel_artist", "skills", "languages", "hobbies",
		"mission", "vision", "favorite_quote",
	}
	for _, col := range expectedCols {
		if !strings.Contains(pgCode, col) {
			t.Errorf("PostgreSQL migration is missing column %q for users", col)
		}
	}

	// Verify invalid SQLite syntax is NOT in PostgreSQL migration
	if strings.Contains(pgCode, "AUTOINCREMENT") {
		t.Errorf("PostgreSQL migration contains invalid SQLite keyword AUTOINCREMENT")
	}

	// Verify group seeding exists for PostgreSQL
	if !strings.Contains(pgCode, "seedDefaultCommunityGroupsPostgres") {
		t.Errorf("PostgreSQL migration is missing seedDefaultCommunityGroupsPostgres")
	}
}

// 8. Live PostgreSQL Integration Test (conditional)
func TestLivePostgresMigration_Conditional(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("Skipping live PostgreSQL integration test: DATABASE_URL not set in environment")
	}

	origDB := DB
	defer func() { DB = origDB }()

	if err := InitPostgres(); err != nil {
		t.Fatalf("failed connecting to live PostgreSQL: %v", err)
	}

	if err := MigratePostgres(); err != nil {
		t.Fatalf("MigratePostgres() failed on live database: %v", err)
	}

	// Run second time to verify idempotency on PostgreSQL
	if err := MigratePostgres(); err != nil {
		t.Fatalf("MigratePostgres() idempotency run failed on live database: %v", err)
	}
}
