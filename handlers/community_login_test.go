package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"convention-management-system/database"
	"convention-management-system/models"
	"convention-management-system/repository"
)

func TestCommunityLogin_RenderPage(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/community/login", nil)
	rr := httptest.NewRecorder()

	CommunityLogin(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK for GET /community/login, got %d", rr.Code)
	}

	body := rr.Body.String()

	// Verify Methodist Youth Fellowship welcome & branding
	expectedStrings := []string{
		"WELCOME TO THE METHODIST CHURCH NIGERIA, DIOCESE OF APA — YOUTH FELLOWSHIP COMMUNITY!",
		"Grace and peace to you in the name of our Lord and Saviour, Jesus Christ!",
		"1 Timothy 4:12 (KJV)",
		"Faith:",
		"Fellowship:",
		"Unity:",
		"Spiritual Growth:",
		"Service:",
		"One Faith. One Fellowship. One Purpose",
		"Community Guidelines",
		"/community/guidelines",
		"Privacy Policy",
		"/privacy",
		"Terms of Use",
		"/terms",
		"Welcome Back!",
		"loginUsername",
		"loginPassword",
		"toggleLoginPassword",
		"rememberMe",
		"Forgot Password?",
		"Sign In",
		"/signup",
		"Create Account",
	}

	for _, s := range expectedStrings {
		if !strings.Contains(body, s) {
			t.Errorf("Expected body to contain %q, but was not found", s)
		}
	}
}

func TestCommunityLogin_AuthenticationFlows(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Create an active test user
	activePass := "MySecretPassword123!"
	activeHash, err := bcrypt.GenerateFromPassword([]byte(activePass), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	activeUser := &models.User{
		FullName:     "Brother David",
		Username:     "bro_david",
		Email:        "david@example.com",
		PasswordHash: string(activeHash),
		Role:         "youth",
		IsActive:     true,
	}
	err = repository.CreateUser(activeUser)
	if err != nil {
		t.Fatalf("Failed to create active test user: %v", err)
	}

	// Create an inactive test user
	inactivePass := "InactivePass123!"
	inactiveHash, err := bcrypt.GenerateFromPassword([]byte(inactivePass), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	inactiveUser := &models.User{
		FullName:     "Suspended Member",
		Username:     "suspended_user",
		Email:        "suspended@example.com",
		PasswordHash: string(inactiveHash),
		Role:         "youth",
		IsActive:     false,
	}
	err = repository.CreateUser(inactiveUser)
	if err != nil {
		t.Fatalf("Failed to create inactive test user: %v", err)
	}
	// Explicitly ensure is_active is false in database
	_, _ = database.DB.Exec("UPDATE users SET is_active = FALSE WHERE id = ?", inactiveUser.ID)

	t.Run("Empty Credentials", func(t *testing.T) {
		form := url.Values{"username": {""}, "password": {""}}
		req := httptest.NewRequest(http.MethodPost, "/community/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		CommunityLogin(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 for empty credentials, got %d", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "Please enter both") {
			t.Errorf("Expected error message for empty credentials in response body")
		}
	})

	t.Run("Non-existent User", func(t *testing.T) {
		form := url.Values{"username": {"nonexistent_user"}, "password": {"SomePass123!"}}
		req := httptest.NewRequest(http.MethodPost, "/community/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		CommunityLogin(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 for non-existent user, got %d", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "Invalid username or password") {
			t.Errorf("Expected invalid credentials message in response body")
		}
	})

	t.Run("Valid Username and Password", func(t *testing.T) {
		form := url.Values{
			"username":    {"bro_david"},
			"password":    {activePass},
			"remember_me": {"on"},
		}
		req := httptest.NewRequest(http.MethodPost, "/community/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		CommunityLogin(rr, req)
		if rr.Code != http.StatusSeeOther {
			t.Fatalf("Expected 303 SeeOther for valid login, got %d", rr.Code)
		}
		loc := rr.Header().Get("Location")
		if loc != "/community" {
			t.Errorf("Expected redirect to /community, got %s", loc)
		}
	})

	t.Run("Valid Email and Password", func(t *testing.T) {
		form := url.Values{
			"username": {"david@example.com"},
			"password": {activePass},
		}
		req := httptest.NewRequest(http.MethodPost, "/community/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		CommunityLogin(rr, req)
		if rr.Code != http.StatusSeeOther {
			t.Fatalf("Expected 303 SeeOther for login via email, got %d", rr.Code)
		}
		loc := rr.Header().Get("Location")
		if loc != "/community" {
			t.Errorf("Expected redirect to /community, got %s", loc)
		}
	})

	t.Run("Invalid Password", func(t *testing.T) {
		form := url.Values{
			"username": {"bro_david"},
			"password": {"WrongPassword999!"},
		}
		req := httptest.NewRequest(http.MethodPost, "/community/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		CommunityLogin(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 for wrong password, got %d", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "Invalid username or password") {
			t.Errorf("Expected invalid credentials error message")
		}
	})

	t.Run("Inactive User Account", func(t *testing.T) {
		form := url.Values{
			"username": {"suspended_user"},
			"password": {inactivePass},
		}
		req := httptest.NewRequest(http.MethodPost, "/community/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		CommunityLogin(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden for inactive account, got %d", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "inactive") && !strings.Contains(rr.Body.String(), "suspended") {
			t.Errorf("Expected inactive account error message, got: %s", rr.Body.String())
		}
	})
}
