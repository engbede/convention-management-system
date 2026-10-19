package services

import (
	"database/sql"
	"path/filepath"
	"testing"

	"convention-management-system/database"
	_ "github.com/mattn/go-sqlite3"
)

func setupTermsServiceTestDB(t *testing.T) func() {
	t.Helper()
	origDB := database.DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_terms_svc.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test db: %v", err)
	}
	database.DB = db

	if err := database.MigrateSQLite(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO users (id, full_name, username, password_hash)
		VALUES (1, 'Member One', 'member1', 'hash1');
	`)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	return func() {
		db.Close()
		database.DB = origDB
	}
}

func TestTermsService_Workflow(t *testing.T) {
	cleanup := setupTermsServiceTestDB(t)
	defer cleanup()

	// Default version should be v1.0
	ver := GetActiveTermsVersion()
	if ver != DefaultTermsVersion {
		t.Fatalf("expected active version %s, got %s", DefaultTermsVersion, ver)
	}

	// User initially not accepted
	accepted, reqVer, latest, err := CheckTermsAcceptance(1)
	if err != nil {
		t.Fatalf("CheckTermsAcceptance failed: %v", err)
	}
	if accepted {
		t.Errorf("expected user not accepted")
	}
	if reqVer != "v1.0" {
		t.Errorf("expected reqVer v1.0, got %s", reqVer)
	}
	if latest != nil {
		t.Errorf("expected nil latest record")
	}

	// Accept terms v1.0
	err = AcceptTerms(1, "v1.0", "192.168.1.50:54321", "Mozilla/5.0")
	if err != nil {
		t.Fatalf("AcceptTerms failed: %v", err)
	}

	// Check terms accepted
	accepted, reqVer, latest, err = CheckTermsAcceptance(1)
	if err != nil {
		t.Fatalf("CheckTermsAcceptance failed: %v", err)
	}
	if !accepted {
		t.Errorf("expected accepted to be true")
	}
	if latest == nil || latest.IPAddress != "192.168.1.50" {
		t.Errorf("expected latest record with clean IP, got %+v", latest)
	}

	// Administrator bumps terms version to v2.0
	err = SetActiveTermsVersion("v2.0")
	if err != nil {
		t.Fatalf("SetActiveTermsVersion failed: %v", err)
	}

	// Now user 1 should NOT be accepted for v2.0
	accepted, reqVer, latest, err = CheckTermsAcceptance(1)
	if err != nil {
		t.Fatalf("CheckTermsAcceptance for v2.0 failed: %v", err)
	}
	if accepted {
		t.Errorf("expected user not to have accepted updated v2.0 terms")
	}
	if reqVer != "v2.0" {
		t.Errorf("expected reqVer v2.0, got %s", reqVer)
	}
	if latest == nil || latest.TermsVersion != "v1.0" {
		t.Errorf("expected user's previous accepted version to be v1.0, got %+v", latest)
	}

	// User accepts v2.0
	err = AcceptTerms(1, "v2.0", "10.0.0.1", "Mozilla/5.0 Updated")
	if err != nil {
		t.Fatalf("AcceptTerms v2.0 failed: %v", err)
	}

	accepted, reqVer, latest, err = CheckTermsAcceptance(1)
	if err != nil {
		t.Fatalf("CheckTermsAcceptance after v2.0 acceptance failed: %v", err)
	}
	if !accepted {
		t.Errorf("expected user to be accepted for v2.0")
	}
	if latest.TermsVersion != "v2.0" {
		t.Errorf("expected latest record terms version to be v2.0, got %s", latest.TermsVersion)
	}
}

