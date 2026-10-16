package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"convention-management-system/database"
	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/services"
)

func TestShowForgotPassword(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/forgot-password", nil)
	rr := httptest.NewRecorder()

	ForgotPasswordHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK for GET /forgot-password, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Forgot Your Password?") {
		t.Errorf("Expected 'Forgot Your Password?' heading in body")
	}
	if !strings.Contains(body, "Send Reset Link") {
		t.Errorf("Expected 'Send Reset Link' button in body")
	}
	if !strings.Contains(body, `href="/community/login"`) {
		t.Errorf("Expected link back to /community/login in body")
	}
}

func TestForgotPassword_InvalidEmail(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	formData := url.Values{
		"email": {"notanemail"},
	}
	req := httptest.NewRequest(http.MethodPost, "/forgot-password", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	ForgotPasswordHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Please enter a valid email address.") {
		t.Errorf("Expected invalid email error message in body, got: %s", rr.Body.String())
	}
}

func TestForgotPassword_RegisteredEmail_And_UnregisteredEmail(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Seed user with email
	_, err := database.DB.Exec(`
		INSERT INTO users (id, full_name, username, email, password_hash)
		VALUES (10, 'Reset User', 'resetuser', 'member@example.com', '$2a$10$hashplaceholder');
	`)
	if err != nil {
		t.Fatalf("Failed to seed user: %v", err)
	}

	var sentLink string
	origSender := services.EmailSenderFunc
	services.EmailSenderFunc = func(to, subject, body string) error {
		sentLink = body
		return nil
	}
	defer func() { services.EmailSenderFunc = origSender }()

	// 1. Registered Email
	formData := url.Values{
		"email": {"member@example.com"},
	}
	req := httptest.NewRequest(http.MethodPost, "/forgot-password", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	ForgotPasswordHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "If an account with that email exists, we have sent a password reset link.") {
		t.Errorf("Expected generic success message in body")
	}
	if sentLink == "" {
		t.Errorf("Expected email to be sent for registered user")
	}

	// 2. Unregistered Email (Must return the exact same generic success message to prevent user enumeration)
	sentLink = ""
	formDataUnreg := url.Values{
		"email": {"unregistered@example.com"},
	}
	reqUnreg := httptest.NewRequest(http.MethodPost, "/forgot-password", strings.NewReader(formDataUnreg.Encode()))
	reqUnreg.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrUnreg := httptest.NewRecorder()

	ForgotPasswordHandler(rrUnreg, reqUnreg)

	if rrUnreg.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK, got %d", rrUnreg.Code)
	}
	if !strings.Contains(rrUnreg.Body.String(), "If an account with that email exists, we have sent a password reset link.") {
		t.Errorf("Expected identical generic success message for unregistered user")
	}
	if sentLink != "" {
		t.Errorf("Expected no email for unregistered user")
	}
}

func TestShowResetPassword_ValidationStates(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Missing token
	reqNoTok := httptest.NewRequest(http.MethodGet, "/reset-password", nil)
	rrNoTok := httptest.NewRecorder()
	ResetPasswordHandler(rrNoTok, reqNoTok)
	if !strings.Contains(rrNoTok.Body.String(), "Password reset token is missing") {
		t.Errorf("Expected missing token error message")
	}

	// 2. Invalid token
	reqInvalid := httptest.NewRequest(http.MethodGet, "/reset-password?token=invalid_dummy_token", nil)
	rrInvalid := httptest.NewRecorder()
	ResetPasswordHandler(rrInvalid, reqInvalid)
	if !strings.Contains(rrInvalid.Body.String(), "This password reset link is invalid or has expired") {
		t.Errorf("Expected invalid token error message")
	}

	// 3. Valid token
	rawTok, _ := services.GenerateSecureToken()
	tokHash := services.HashToken(rawTok)
	_ = repository.CreatePasswordReset(&models.PasswordReset{
		UserID:    1,
		Email:     "user1@example.com",
		TokenHash: tokHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Used:      false,
	})

	reqValid := httptest.NewRequest(http.MethodGet, "/reset-password?token="+rawTok, nil)
	rrValid := httptest.NewRecorder()
	ResetPasswordHandler(rrValid, reqValid)

	if rrValid.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for valid token, got %d", rrValid.Code)
	}
	bodyValid := rrValid.Body.String()
	if !strings.Contains(bodyValid, `name="token" value="`+rawTok+`"`) {
		t.Errorf("Expected form to contain hidden token")
	}
	if !strings.Contains(bodyValid, `id="toggleNewPassword"`) {
		t.Errorf("Expected Show/Hide toggle for new password")
	}
	if !strings.Contains(bodyValid, `id="toggleConfirmNewPassword"`) {
		t.Errorf("Expected independent Show/Hide toggle for confirm new password")
	}
}

func TestSubmitResetPassword_FullWorkflow(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Create user with old password
	oldHash, _ := bcrypt.GenerateFromPassword([]byte("OldSecretPassword123!"), bcrypt.DefaultCost)
	_, err := database.DB.Exec(`
		INSERT INTO users (id, full_name, username, email, password_hash)
		VALUES (20, 'Reset Target', 'resettarget', 'target@example.com', ?);
	`, string(oldHash))
	if err != nil {
		t.Fatalf("Failed to seed user: %v", err)
	}

	// Create valid reset token
	rawTok, _ := services.GenerateSecureToken()
	tokHash := services.HashToken(rawTok)
	_ = repository.CreatePasswordReset(&models.PasswordReset{
		UserID:    20,
		Email:     "target@example.com",
		TokenHash: tokHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Used:      false,
	})

	// 1. Password confirmation mismatch
	mismatchForm := url.Values{
		"token":            {rawTok},
		"password":         {"NewPassword123!"},
		"confirm_password": {"DifferentPassword!"},
	}
	reqMismatch := httptest.NewRequest(http.MethodPost, "/reset-password", strings.NewReader(mismatchForm.Encode()))
	reqMismatch.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrMismatch := httptest.NewRecorder()
	ResetPasswordHandler(rrMismatch, reqMismatch)
	if !strings.Contains(rrMismatch.Body.String(), "Passwords do not match.") {
		t.Errorf("Expected password mismatch error, got: %s", rrMismatch.Body.String())
	}

	// 2. Short password
	shortForm := url.Values{
		"token":            {rawTok},
		"password":         {"123"},
		"confirm_password": {"123"},
	}
	reqShort := httptest.NewRequest(http.MethodPost, "/reset-password", strings.NewReader(shortForm.Encode()))
	reqShort.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrShort := httptest.NewRecorder()
	ResetPasswordHandler(rrShort, reqShort)
	if !strings.Contains(rrShort.Body.String(), "Password must be at least 6 characters long.") {
		t.Errorf("Expected short password error, got: %s", rrShort.Body.String())
	}

	// 3. Successful password reset
	validForm := url.Values{
		"token":            {rawTok},
		"password":         {"BrandNewStrongPassword123!"},
		"confirm_password": {"BrandNewStrongPassword123!"},
	}
	reqValid := httptest.NewRequest(http.MethodPost, "/reset-password", strings.NewReader(validForm.Encode()))
	reqValid.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrValid := httptest.NewRecorder()
	ResetPasswordHandler(rrValid, reqValid)

	if !strings.Contains(rrValid.Body.String(), "Your password has been reset successfully!") {
		t.Fatalf("Expected success message after reset, got: %s", rrValid.Body.String())
	}

	// 4. Verify login with NEW password succeeds
	loginFormNew := url.Values{
		"username": {"resettarget"},
		"password": {"BrandNewStrongPassword123!"},
	}
	reqLoginNew := httptest.NewRequest(http.MethodPost, "/community/login", strings.NewReader(loginFormNew.Encode()))
	reqLoginNew.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrLoginNew := httptest.NewRecorder()
	CommunityLogin(rrLoginNew, reqLoginNew)
	if rrLoginNew.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303 for login with new password, got %d", rrLoginNew.Code)
	}

	// 5. Verify login with OLD password fails (401 Unauthorized)
	loginFormOld := url.Values{
		"username": {"resettarget"},
		"password": {"OldSecretPassword123!"},
	}
	reqLoginOld := httptest.NewRequest(http.MethodPost, "/community/login", strings.NewReader(loginFormOld.Encode()))
	reqLoginOld.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrLoginOld := httptest.NewRecorder()
	CommunityLogin(rrLoginOld, reqLoginOld)
	if rrLoginOld.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized for login with old password, got %d", rrLoginOld.Code)
	}

	// 6. Token reuse: Submitting again with the same token must fail
	reqReuse := httptest.NewRequest(http.MethodPost, "/reset-password", strings.NewReader(validForm.Encode()))
	reqReuse.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrReuse := httptest.NewRecorder()
	ResetPasswordHandler(rrReuse, reqReuse)
	if !strings.Contains(rrReuse.Body.String(), "This password reset link is invalid or has expired") {
		t.Errorf("Expected token reuse to be rejected, got: %s", rrReuse.Body.String())
	}
}

func TestSubmitResetPassword_ExpiredToken(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	rawTok, _ := services.GenerateSecureToken()
	tokHash := services.HashToken(rawTok)
	// Expired 1 hour ago
	_ = repository.CreatePasswordReset(&models.PasswordReset{
		UserID:    1,
		Email:     "user1@example.com",
		TokenHash: tokHash,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		Used:      false,
	})

	form := url.Values{
		"token":            {rawTok},
		"password":         {"NewPassword123!"},
		"confirm_password": {"NewPassword123!"},
	}
	req := httptest.NewRequest(http.MethodPost, "/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	ResetPasswordHandler(rr, req)

	if !strings.Contains(rr.Body.String(), "This password reset link is invalid or has expired") {
		t.Errorf("Expected expired token to be rejected, got: %s", rr.Body.String())
	}
}
