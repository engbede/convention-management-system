package routes_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"convention-management-system/database"
	"convention-management-system/routes"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestEnv(t *testing.T) (*http.ServeMux, func()) {
	t.Helper()
	origWd, _ := os.Getwd()
	if strings.HasSuffix(origWd, "routes") {
		_ = os.Chdir("..")
	}

	testDBPath := "test_routes.db"
	_ = os.Remove(testDBPath)

	db, err := sql.Open("sqlite3", testDBPath)
	if err != nil {
		t.Fatalf("Failed to open test db: %v", err)
	}
	database.DB = db

	if err := database.MigrateSQLite(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	mux := http.NewServeMux()
	routes.RegisterRoutes(mux)

	cleanup := func() {
		db.Close()
		_ = os.Remove(testDBPath)
		_ = os.Chdir(origWd)
	}

	return mux, cleanup
}

func TestRouteAccessibility_PublicRoutes(t *testing.T) {
	mux, cleanup := setupTestEnv(t)
	defer cleanup()

	// Seed an active convention so convention pages render normally
	_, _ = database.DB.Exec(`
		INSERT INTO conventions (id, name, theme, venue, start_date, end_date, year, active)
		VALUES (1, 'Convention 2026', 'Faith', 'Cathedral', '2026-11-20', '2026-11-24', 2026, 1);
	`)

	publicRoutes := []struct {
		name string
		path string
	}{
		{name: "Home", path: "/"},
		{name: "Convention Form", path: "/register"},
		{name: "Admin Login", path: "/login"},
		{name: "Community Login", path: "/community/login"},
		{name: "Community Signup", path: "/signup"},
		{name: "Public Announcements", path: "/announcements"},
		{name: "Contact", path: "/contact"},
		{name: "Forgot Password", path: "/forgot-password"},
		{name: "Reset Password", path: "/reset-password"},
		{name: "Live Programmes", path: "/programmes"},
	}

	for _, tc := range publicRoutes {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Expected status 200 OK for public route %s, got %d", tc.path, rr.Code)
			}
		})
	}
}

func TestRouteAccessibility_AdminProtection(t *testing.T) {
	mux, cleanup := setupTestEnv(t)
	defer cleanup()

	adminRoutes := []string{
		"/dashboard",
		"/registrations",
		"/reports",
		"/settings",
		"/backup/delete",
	}

	for _, path := range adminRoutes {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusSeeOther && rr.Code != http.StatusFound {
				t.Errorf("Expected redirect status (302/303) for admin route %s without session, got %d", path, rr.Code)
			}
			if loc := rr.Header().Get("Location"); loc != "/login" {
				t.Errorf("Expected redirect to /login for admin route %s, got %q", path, loc)
			}
		})
	}
}

func TestRouteAccessibility_CommunityProtection(t *testing.T) {
	mux, cleanup := setupTestEnv(t)
	defer cleanup()

	communityProtectedRoutes := []string{
		"/community",
		"/messages",
		"/notifications",
		"/profile",
		"/groups",
		"/prayer",
		"/bible-study",
		"/saved",
	}

	for _, path := range communityProtectedRoutes {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusSeeOther && rr.Code != http.StatusFound {
				t.Errorf("Expected redirect status (302/303) for community route %s without session, got %d", path, rr.Code)
			}
			if loc := rr.Header().Get("Location"); loc != "/community/login" {
				t.Errorf("Expected redirect to /community/login for route %s, got %q", path, loc)
			}
		})
	}
}

func TestRouteAccessibility_PublicConvenienceRedirects(t *testing.T) {
	mux, cleanup := setupTestEnv(t)
	defer cleanup()

	convenienceRedirects := []struct {
		path     string
		expected string
	}{
		{path: "/events", expected: "/#conventions"},
		{path: "/publications", expected: "/announcements"},
	}

	for _, tc := range convenienceRedirects {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusSeeOther {
				t.Errorf("Expected status 303 See Other for %s, got %d", tc.path, rr.Code)
			}
			if loc := rr.Header().Get("Location"); loc != tc.expected {
				t.Errorf("Expected redirect for %s to %q, got %q", tc.path, tc.expected, loc)
			}
		})
	}
}

func TestLiveProgrammesRoutes(t *testing.T) {
	mux, cleanup := setupTestEnv(t)
	defer cleanup()

	// 1. Direct /programmes returns 200
	req := httptest.NewRequest(http.MethodGet, "/programmes", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 for /programmes, got %d", rr.Code)
	}

	// 2. Trailing slash /programmes/ redirects to /programmes
	reqSlash := httptest.NewRequest(http.MethodGet, "/programmes/", nil)
	rrSlash := httptest.NewRecorder()
	mux.ServeHTTP(rrSlash, reqSlash)
	if rrSlash.Code != http.StatusMovedPermanently && rrSlash.Code != http.StatusOK {
		t.Errorf("Expected status 301 or 200 for /programmes/, got %d", rrSlash.Code)
	}

	// 3. Aliases /live and /live-programmes
	aliases := []struct {
		path     string
		expected string
	}{
		{"/live", "/programmes?tab=live"},
		{"/live-programmes", "/programmes"},
		{"/online-programmes", "/programmes"},
	}
	for _, a := range aliases {
		reqA := httptest.NewRequest(http.MethodGet, a.path, nil)
		rrA := httptest.NewRecorder()
		mux.ServeHTTP(rrA, reqA)
		if rrA.Code != http.StatusSeeOther {
			t.Errorf("Expected 303 for %s, got %d", a.path, rrA.Code)
		}
		if loc := rrA.Header().Get("Location"); loc != a.expected {
			t.Errorf("Expected redirect to %s, got %s", a.expected, loc)
		}
	}
}

func TestRouteAccessibility_UnknownPathReturns404(t *testing.T) {
	mux, cleanup := setupTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/unregistered/mystery/route/123", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 Not Found for unregistered route, got %d", rr.Code)
	}
}
