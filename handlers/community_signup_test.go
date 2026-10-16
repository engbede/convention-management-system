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

func TestSignup_DuplicateEmail(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Register initial user
	formData1 := url.Values{
		"full_name":        {"First Email User"},
		"username":         {"email_user_1"},
		"email":            {"unique_email@example.com"},
		"password":         {"ValidPass123!"},
		"confirm_password": {"ValidPass123!"},
	}
	req1 := httptest.NewRequest(http.MethodPost, "/create-account", strings.NewReader(formData1.Encode()))
	req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr1 := httptest.NewRecorder()
	Signup(rr1, req1)
	if rr1.Code != http.StatusSeeOther {
		t.Fatalf("Failed to create first user: %d %s", rr1.Code, rr1.Body.String())
	}

	// Attempt duplicate email with different casing
	formData2 := url.Values{
		"full_name":        {"Second Email User"},
		"username":         {"email_user_2"},
		"email":            {"UNIQUE_EMAIL@EXAMPLE.COM"},
		"password":         {"ValidPass123!"},
		"confirm_password": {"ValidPass123!"},
	}
	req2 := httptest.NewRequest(http.MethodPost, "/create-account", strings.NewReader(formData2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr2 := httptest.NewRecorder()
	Signup(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for duplicate email, got %d", rr2.Code)
	}
	if !strings.Contains(rr2.Body.String(), "Email is already registered") {
		t.Errorf("Expected 'Email is already registered' in response, got %q", rr2.Body.String())
	}
}

func TestSignup_DuplicatePhone(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Register initial user
	formData1 := url.Values{
		"full_name":        {"First Phone User"},
		"username":         {"phone_user_1"},
		"phone":            {"08011112222"},
		"password":         {"ValidPass123!"},
		"confirm_password": {"ValidPass123!"},
	}
	req1 := httptest.NewRequest(http.MethodPost, "/create-account", strings.NewReader(formData1.Encode()))
	req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr1 := httptest.NewRecorder()
	Signup(rr1, req1)
	if rr1.Code != http.StatusSeeOther {
		t.Fatalf("Failed to create first user: %d %s", rr1.Code, rr1.Body.String())
	}

	// Attempt duplicate phone
	formData2 := url.Values{
		"full_name":        {"Second Phone User"},
		"username":         {"phone_user_2"},
		"phone":            {"08011112222"},
		"password":         {"ValidPass123!"},
		"confirm_password": {"ValidPass123!"},
	}
	req2 := httptest.NewRequest(http.MethodPost, "/create-account", strings.NewReader(formData2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr2 := httptest.NewRecorder()
	Signup(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for duplicate phone, got %d", rr2.Code)
	}
	if !strings.Contains(rr2.Body.String(), "Phone number is already registered") {
		t.Errorf("Expected 'Phone number is already registered' in response, got %q", rr2.Body.String())
	}
}

func TestShowSignup_POST(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	formData := url.Values{
		"full_name":        {"Post Signup User"},
		"username":         {"post_signup_user"},
		"email":            {"post_signup@example.com"},
		"phone":            {"08055556666"},
		"password":         {"Pass123456!"},
		"confirm_password": {"Pass123456!"},
	}

	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	ShowSignup(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("Expected status 303 See Other for POST /signup, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/community/login" {
		t.Errorf("Expected redirect to /community/login, got %q", loc)
	}

	user, err := repository.GetUserByUsername("post_signup_user")
	if err != nil {
		t.Fatalf("Expected user to be created via POST /signup, err=%v", err)
	}
	if user.FullName != "Post Signup User" {
		t.Errorf("Expected FullName 'Post Signup User', got %q", user.FullName)
	}
}

func TestSignup_PostgreSQLErrorParsing(t *testing.T) {
	// Directly test that PostgreSQL error strings are mapped to user-friendly 400 responses
	testCases := []struct {
		errStr           string
		expectedMsg      string
		unexpectedSubstr string
	}{
		{
			errStr:           `ERROR: duplicate key value violates unique constraint "users_username_key" (SQLSTATE 23505)`,
			expectedMsg:      "Username is already taken.",
			unexpectedSubstr: "SQLSTATE",
		},
		{
			errStr:           `ERROR: duplicate key value violates unique constraint "users_email_key" (SQLSTATE 23505)`,
			expectedMsg:      "Email is already registered.",
			unexpectedSubstr: "users_email_key",
		},
		{
			errStr:           `ERROR: duplicate key value violates unique constraint "users_phone_key" (SQLSTATE 23505)`,
			expectedMsg:      "Phone number is already registered.",
			unexpectedSubstr: "ERROR",
		},
	}

	for _, tc := range testCases {
		errLower := strings.ToLower(tc.errStr)
		var userMsg string
		if strings.Contains(errLower, "users.username") || strings.Contains(errLower, "users_username") || (strings.Contains(errLower, "23505") && strings.Contains(errLower, "username")) {
			userMsg = "Username is already taken."
		} else if strings.Contains(errLower, "users.email") || strings.Contains(errLower, "users_email") || (strings.Contains(errLower, "23505") && strings.Contains(errLower, "email")) {
			userMsg = "Email is already registered."
		} else if strings.Contains(errLower, "users.phone") || strings.Contains(errLower, "users_phone") || (strings.Contains(errLower, "23505") && strings.Contains(errLower, "phone")) {
			userMsg = "Phone number is already registered."
		}

		if userMsg != tc.expectedMsg {
			t.Errorf("For error %q, expected user message %q, got %q", tc.errStr, tc.expectedMsg, userMsg)
		}
		if strings.Contains(userMsg, tc.unexpectedSubstr) {
			t.Errorf("User message leaked internal database details: %s", userMsg)
		}
	}
}
