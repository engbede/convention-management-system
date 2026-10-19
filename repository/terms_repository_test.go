package repository

import (
	"database/sql"
	"path/filepath"
	"testing"

	"convention-management-system/database"
	_ "github.com/mattn/go-sqlite3"
)

func setupTermsTestDB(t *testing.T) func() {
	t.Helper()
	origDB := database.DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_terms.db")

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
		VALUES (1, 'Terms Tester', 'termstester', 'hash123');
	`)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	return func() {
		db.Close()
		database.DB = origDB
	}
}

func TestTermsRepository_Flow(t *testing.T) {
	cleanup := setupTermsTestDB(t)
	defer cleanup()

	// 1. Initial check: user 1 has not accepted v1.0
	accepted, err := HasUserAcceptedTerms(1, "v1.0")
	if err != nil {
		t.Fatalf("HasUserAcceptedTerms failed: %v", err)
	}
	if accepted {
		t.Errorf("expected user not to have accepted v1.0 initially")
	}

	latest, err := GetLatestTermsAcceptance(1)
	if err != nil {
		t.Fatalf("GetLatestTermsAcceptance failed: %v", err)
	}
	if latest != nil {
		t.Errorf("expected latest to be nil initially, got %+v", latest)
	}

	// 2. Record acceptance for v1.0
	ip := "192.168.1.100"
	ua := "Mozilla/5.0 TestBrowser"
	err = RecordTermsAcceptance(1, "v1.0", ip, ua)
	if err != nil {
		t.Fatalf("RecordTermsAcceptance failed: %v", err)
	}

	// 3. Verify acceptance for v1.0
	accepted, err = HasUserAcceptedTerms(1, "v1.0")
	if err != nil {
		t.Fatalf("HasUserAcceptedTerms failed: %v", err)
	}
	if !accepted {
		t.Errorf("expected user to have accepted v1.0 after recording")
	}

	// 4. Verify user has not accepted unreleased v2.0
	acceptedV2, err := HasUserAcceptedTerms(1, "v2.0")
	if err != nil {
		t.Fatalf("HasUserAcceptedTerms v2.0 failed: %v", err)
	}
	if acceptedV2 {
		t.Errorf("expected user not to have accepted v2.0")
	}

	// 5. Check latest record
	latest, err = GetLatestTermsAcceptance(1)
	if err != nil {
		t.Fatalf("GetLatestTermsAcceptance failed: %v", err)
	}
	if latest == nil {
		t.Fatalf("expected non-nil latest acceptance")
	}
	if latest.TermsVersion != "v1.0" {
		t.Errorf("expected terms version v1.0, got %s", latest.TermsVersion)
	}
	if latest.IPAddress != ip {
		t.Errorf("expected ip %s, got %s", ip, latest.IPAddress)
	}
	if latest.UserAgent != ua {
		t.Errorf("expected user agent %s, got %s", ua, latest.UserAgent)
	}

	// 6. Idempotency / Update: Re-accept v1.0 from new IP
	newIP := "10.0.0.1"
	err = RecordTermsAcceptance(1, "v1.0", newIP, ua)
	if err != nil {
		t.Fatalf("Re-recording terms acceptance failed: %v", err)
	}

	latest, err = GetLatestTermsAcceptance(1)
	if err != nil {
		t.Fatalf("GetLatestTermsAcceptance after update failed: %v", err)
	}
	if latest.IPAddress != newIP {
		t.Errorf("expected updated ip %s, got %s", newIP, latest.IPAddress)
	}

	// 7. Accept v2.0 and verify history
	err = RecordTermsAcceptance(1, "v2.0", newIP, ua)
	if err != nil {
		t.Fatalf("RecordTermsAcceptance v2.0 failed: %v", err)
	}

	history, err := GetTermsAcceptanceHistory(1)
	if err != nil {
		t.Fatalf("GetTermsAcceptanceHistory failed: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history items, got %d", len(history))
	}
}

