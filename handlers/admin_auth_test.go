package handlers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"convention-management-system/database"
	"convention-management-system/repository"
	"convention-management-system/services"

	"golang.org/x/crypto/bcrypt"
)

func setupAdminAuthTestDB(t *testing.T) func() {
	t.Helper()
	origWd, _ := os.Getwd()
	if strings.HasSuffix(origWd, "handlers") {
		_ = os.Chdir("..")
	}

	origDB := database.DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "admin_auth_test.db")

	testDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	database.DB = testDB

	if err := database.MigrateSQLite(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Seed an administrator
	hashed, _ := bcrypt.GenerateFromPassword([]byte("StrongAdminPass123!"), bcrypt.DefaultCost)
	_, err = testDB.Exec(`
		INSERT INTO admins (id, username, password, email)
		VALUES (1, 'diocesan_admin', $1, 'admin@apa-diocese.org')
	`, string(hashed))
	if err != nil {
		t.Fatalf("failed inserting test admin: %v", err)
	}

	return func() {
		_ = os.Chdir(origWd)
		testDB.Close()
		database.DB = origDB
	}
}

func TestAdminLogin_Flow(t *testing.T) {
	cleanup := setupAdminAuthTestDB(t)
	defer cleanup()

	// 1. GET /login returns 200 OK
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rr := httptest.NewRecorder()
	AdminLogin(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 for GET /login, got %d", rr.Code)
	}

	// 2. POST with wrong password returns 401 Unauthorized
	form := url.Values{
		"username": {"diocesan_admin"},
		"password": {"WrongPassword!"},
	}
	req = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	AdminLogin(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for wrong password, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Invalid username or password") {
		t.Errorf("Expected error message in body, got: %s", rr.Body.String())
	}

	// 3. POST with correct password returns 303 SeeOther and sets admin_session cookie
	form = url.Values{
		"username": {"diocesan_admin"},
		"password": {"StrongAdminPass123!"},
	}
	req = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	AdminLogin(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303 for successful login, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/dashboard" {
		t.Errorf("Expected redirect to /dashboard, got %q", loc)
	}

	cookies := rr.Result().Cookies()
	var foundSession bool
	for _, c := range cookies {
		if c.Name == "admin_session" && c.Value == "1" {
			foundSession = true
			if !c.HttpOnly {
				t.Errorf("Expected admin_session cookie to be HttpOnly")
			}
			if c.SameSite != http.SameSiteLaxMode {
				t.Errorf("Expected SameSiteLaxMode on admin cookie, got %v", c.SameSite)
			}
		}
	}
	if !foundSession {
		t.Errorf("Expected admin_session cookie to be set")
	}

	// 4. POST with email instead of username succeeds
	form = url.Values{
		"username": {"admin@apa-diocese.org"},
		"password": {"StrongAdminPass123!"},
	}
	req = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	AdminLogin(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303 for login via email, got %d", rr.Code)
	}
}

func TestAdminPassword_ForgotAndResetFlow(t *testing.T) {
	cleanup := setupAdminAuthTestDB(t)
	defer cleanup()

	// Track dispatched email
	var sentTo, sentSubject, sentBody string
	origSender := services.EmailSenderFunc
	services.EmailSenderFunc = func(to, subject, body string) error {
		sentTo = to
		sentSubject = subject
		sentBody = body
		return nil
	}
	defer func() { services.EmailSenderFunc = origSender }()

	// 1. Request password reset for admin
	form := url.Values{
		"login": {"admin@apa-diocese.org"},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/forgot-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	AdminForgotPassword(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 for AdminForgotPassword, got %d", rr.Code)
	}
	if sentTo != "admin@apa-diocese.org" {
		t.Errorf("Expected email sent to admin@apa-diocese.org, got %q", sentTo)
	}
	if !strings.Contains(sentSubject, "Password Reset") {
		t.Errorf("Expected subject to mention Password Reset, got %q", sentSubject)
	}
	if !strings.Contains(sentBody, "/admin/reset-password?token=") {
		t.Fatalf("Expected reset link in email body, got: %s", sentBody)
	}

	// Extract token from reset link
	idx := strings.Index(sentBody, "token=")
	rawToken := sentBody[idx+len("token=") : idx+len("token=")+64]

	// 2. GET /admin/reset-password with valid token
	req = httptest.NewRequest(http.MethodGet, "/admin/reset-password?token="+rawToken, nil)
	rr = httptest.NewRecorder()
	AdminResetPassword(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 for valid token reset page, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), rawToken) {
		t.Errorf("Expected reset page to include token in hidden field")
	}

	// 3. POST /admin/reset-password to set new password
	resetForm := url.Values{
		"token":            {rawToken},
		"password":         {"NewSuperSecurePass456!"},
		"confirm_password": {"NewSuperSecurePass456!"},
	}
	req = httptest.NewRequest(http.MethodPost, "/admin/reset-password", strings.NewReader(resetForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	AdminResetPassword(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303 redirect on successful reset, got %d", rr.Code)
	}

	// 4. Verify login with NEW password succeeds
	loginForm := url.Values{
		"username": {"diocesan_admin"},
		"password": {"NewSuperSecurePass456!"},
	}
	req = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(loginForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	AdminLogin(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("Expected successful login with new password, got %d", rr.Code)
	}

	// 5. Verify token cannot be reused
	req = httptest.NewRequest(http.MethodPost, "/admin/reset-password", strings.NewReader(resetForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	AdminResetPassword(rr, req)

	if strings.Contains(rr.Body.String(), "already been used") || strings.Contains(rr.Body.String(), "invalid or has expired") {
		// Pass - token rejection verified
	} else {
		t.Errorf("Expected reused token to be rejected, got response: %s", rr.Body.String())
	}
}

func TestAdminChangePassword_Authenticated(t *testing.T) {
	cleanup := setupAdminAuthTestDB(t)
	defer cleanup()

	// 1. GET /admin/change-password
	req := httptest.NewRequest(http.MethodGet, "/admin/change-password", nil)
	req.AddCookie(&http.Cookie{
		Name:  "admin_session",
		Value: "1",
	})
	rr := httptest.NewRecorder()
	AdminChangePassword(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 for GET /admin/change-password, got %d", rr.Code)
	}

	// 2. POST with wrong current password fails
	form := url.Values{
		"current_password": {"WrongPassword!"},
		"new_password":     {"BrandNewPass789!"},
		"confirm_password": {"BrandNewPass789!"},
	}
	req = httptest.NewRequest(http.MethodPost, "/admin/change-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{
		Name:  "admin_session",
		Value: "1",
	})
	rr = httptest.NewRecorder()
	AdminChangePassword(rr, req)

	if !strings.Contains(rr.Body.String(), "Current password does not match") {
		t.Errorf("Expected error for wrong current password, got: %s", rr.Body.String())
	}

	// 3. POST with correct current password succeeds
	form = url.Values{
		"current_password": {"StrongAdminPass123!"},
		"new_password":     {"BrandNewPass789!"},
		"confirm_password": {"BrandNewPass789!"},
	}
	req = httptest.NewRequest(http.MethodPost, "/admin/change-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{
		Name:  "admin_session",
		Value: "1",
	})
	rr = httptest.NewRecorder()
	AdminChangePassword(rr, req)

	if !strings.Contains(rr.Body.String(), "updated successfully") {
		t.Errorf("Expected success message for password change, got: %s", rr.Body.String())
	}

	// 4. Verify admin can now login with BrandNewPass789!
	admin, err := repository.GetAdminByID(1)
	if err != nil {
		t.Fatalf("failed fetching admin: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte("BrandNewPass789!")); err != nil {
		t.Errorf("Admin password was not updated to BrandNewPass789!")
	}
}
