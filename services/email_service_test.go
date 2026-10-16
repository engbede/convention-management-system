package services_test

import (
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"convention-management-system/database"
	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/services"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) func() {
	t.Helper()
	testDBPath := "test_services.db"
	_ = os.Remove(testDBPath)

	db, err := sql.Open("sqlite3", testDBPath)
	if err != nil {
		t.Fatalf("Failed to open test db: %v", err)
	}
	database.DB = db

	if err := database.MigrateSQLite(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO users (id, full_name, username, email, password_hash)
		VALUES (1, 'Alice Test', 'alice', 'alice@example.com', '$2a$10$oldhashplaceholder');
	`)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	return func() {
		db.Close()
		_ = os.Remove(testDBPath)
	}
}

func TestGenerateSecureToken(t *testing.T) {
	tok1, err := services.GenerateSecureToken()
	if err != nil {
		t.Fatalf("GenerateSecureToken failed: %v", err)
	}
	tok2, err := services.GenerateSecureToken()
	if err != nil {
		t.Fatalf("GenerateSecureToken failed: %v", err)
	}

	if len(tok1) != 64 {
		t.Errorf("Expected 64 hex chars, got %d", len(tok1))
	}
	if tok1 == tok2 {
		t.Errorf("Expected distinct tokens, got identical tokens")
	}
}

func TestHashToken(t *testing.T) {
	tok := "abcdef1234567890abcdef1234567890"
	h1 := services.HashToken(tok)
	h2 := services.HashToken(tok)
	if h1 != h2 {
		t.Errorf("HashToken should be deterministic")
	}
	if len(h1) != 64 {
		t.Errorf("Expected 64 hex chars for SHA-256, got %d", len(h1))
	}
}

func TestRequestPasswordReset_RegisteredUser(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	var sentTo, sentSubject, sentBody string
	origSender := services.EmailSenderFunc
	services.EmailSenderFunc = func(to, subject, body string) error {
		sentTo = to
		sentSubject = subject
		sentBody = body
		return nil
	}
	defer func() { services.EmailSenderFunc = origSender }()

	err := services.RequestPasswordReset("http://localhost:8085", "alice@example.com")
	if err != nil {
		t.Fatalf("RequestPasswordReset failed: %v", err)
	}

	if sentTo != "alice@example.com" {
		t.Errorf("Expected email to alice@example.com, got %q", sentTo)
	}
	if !strings.Contains(sentSubject, "Reset Your Youth Community Password") {
		t.Errorf("Expected password reset subject, got %q", sentSubject)
	}
	if !strings.Contains(sentBody, "/reset-password?token=") {
		t.Errorf("Expected reset link in email body, got %q", sentBody)
	}
}

func TestRequestPasswordReset_UnregisteredUserDoesNotDisclose(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	var sent bool
	origSender := services.EmailSenderFunc
	services.EmailSenderFunc = func(to, subject, body string) error {
		sent = true
		return nil
	}
	defer func() { services.EmailSenderFunc = origSender }()

	err := services.RequestPasswordReset("http://localhost:8085", "nobody@example.com")
	if err != nil {
		t.Fatalf("Expected nil error for unregistered user, got %v", err)
	}
	if sent {
		t.Errorf("Expected no email to be sent for unregistered user")
	}
}

func TestRequestPasswordReset_RateLimiting(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	origSender := services.EmailSenderFunc
	services.EmailSenderFunc = func(to, subject, body string) error {
		return nil
	}
	defer func() { services.EmailSenderFunc = origSender }()

	// Seed 5 recent resets for alice@example.com
	for i := 0; i < 5; i++ {
		_ = repository.CreatePasswordReset(&models.PasswordReset{
			UserID:    1,
			Email:     "alice@example.com",
			TokenHash: services.HashToken(string(rune('a' + i))),
			ExpiresAt: time.Now().Add(1 * time.Hour),
			Used:      false,
			CreatedAt: time.Now(),
		})
	}

	err := services.RequestPasswordReset("http://localhost:8085", "alice@example.com")
	if err == nil {
		t.Fatalf("Expected rate limit error after 5 attempts, got nil")
	}
	if !strings.Contains(err.Error(), "too many reset requests") {
		t.Errorf("Expected rate limit message, got %v", err)
	}
}
