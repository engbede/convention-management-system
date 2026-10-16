package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"convention-management-system/repository"
)

func TestShowSignup(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/signup", nil)
	rr := httptest.NewRecorder()

	ShowSignup(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK for /signup, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Create Community Account") {
		t.Errorf("Expected signup page title in body")
	}
	if !strings.Contains(body, `action="/create-account"`) {
		t.Errorf("Expected form action to be /create-account")
	}
}

func TestSignup_Success(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	formData := url.Values{
		"full_name":        {"New Member"},
		"username":         {"new_member_99"},
		"email":            {"new_member@example.com"},
		"phone":            {"08099998888"},
		"password":         {"SuperSecret123!"},
		"confirm_password": {"SuperSecret123!"},
	}

	req := httptest.NewRequest(http.MethodPost, "/create-account", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	Signup(rr, req)

	// Should redirect to /community/login on success
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("Expected status 303 See Other, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if loc := rr.Header().Get("Location"); loc != "/community/login" {
		t.Errorf("Expected redirect to /community/login, got %q", loc)
	}

	// Verify user is in database
	user, err := repository.GetUserByUsername("new_member_99")
	if err != nil {
		t.Fatalf("Failed to fetch newly created user: %v", err)
	}

	if user.FullName != "New Member" {
		t.Errorf("Expected FullName 'New Member', got %q", user.FullName)
	}
	if user.Email != "new_member@example.com" {
		t.Errorf("Expected Email 'new_member@example.com', got %q", user.Email)
	}

	// Verify password was securely hashed with bcrypt and plaintext is not stored
	if user.PasswordHash == "SuperSecret123!" {
		t.Errorf("CRITICAL SECURITY FLAW: Password stored as plaintext!")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("SuperSecret123!")); err != nil {
		t.Errorf("Password hash does not verify against original password: %v", err)
	}
}

func TestSignup_MissingRequiredFields(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Missing username and password
	formData := url.Values{
		"full_name":        {"Incomplete User"},
		"username":         {""},
		"password":         {""},
		"confirm_password": {""},
	}

	req := httptest.NewRequest(http.MethodPost, "/create-account", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	Signup(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for missing fields, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Please complete all required fields") {
		t.Errorf("Expected error message for missing fields, got %q", rr.Body.String())
	}
}

func TestSignup_PasswordsDoNotMatch(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	formData := url.Values{
		"full_name":        {"Mismatch User"},
		"username":         {"mismatch_user"},
		"password":         {"PasswordOne123"},
		"confirm_password": {"PasswordTwo999"},
	}

	req := httptest.NewRequest(http.MethodPost, "/create-account", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	Signup(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for mismatched passwords, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Passwords do not match") {
		t.Errorf("Expected 'Passwords do not match' error, got %q", rr.Body.String())
	}
}

func TestSignup_DuplicateUsername(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// user1 already exists in setupTestDB
	formData := url.Values{
		"full_name":        {"Duplicate User One"},
		"username":         {"user1"}, // Existing username in handlers setupTestDB
		"password":         {"Pass123!"},
		"confirm_password": {"Pass123!"},
	}

	req := httptest.NewRequest(http.MethodPost, "/create-account", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	Signup(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for duplicate username, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Username is already taken") {
		t.Errorf("Expected user-friendly message, got: %q", rr.Body.String())
	}
	// Verify raw internal SQL error is NOT exposed
	if strings.Contains(rr.Body.String(), "SQL") || strings.Contains(rr.Body.String(), "syntax") {
		t.Errorf("Internal SQL error was leaked to response: %s", rr.Body.String())
	}
}

func TestSignup_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/create-account", nil)
	rr := httptest.NewRecorder()

	Signup(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("Expected redirect status 303, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/signup" {
		t.Errorf("Expected redirect to /signup, got %q", loc)
	}
}
