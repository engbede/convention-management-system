package routes_test

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"convention-management-system/database"
	"convention-management-system/repository"
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
		{name: "Community Guidelines", path: "/community/guidelines"},
		{name: "Guidelines Alias", path: "/guidelines"},
		{name: "Privacy Policy", path: "/privacy"},
		{name: "Terms of Use", path: "/terms"},
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
		"/terms/status",
		"/terms/accept",
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

	// 3. Dedicated /live public viewing page serves 200 OK
	reqLive := httptest.NewRequest(http.MethodGet, "/live", nil)
	rrLive := httptest.NewRecorder()
	mux.ServeHTTP(rrLive, reqLive)
	if rrLive.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for dedicated /live page, got %d", rrLive.Code)
	}

	// 4. Aliases /live-programmes and /online-programmes
	aliases := []struct {
		path     string
		expected string
	}{
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

func TestRouteAccessibility_ServiceWorker(t *testing.T) {
	mux, cleanup := setupTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/sw.js", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK for /sw.js, got %d", rr.Code)
	}

	if ctype := rr.Header().Get("Content-Type"); !strings.Contains(ctype, "application/javascript") {
		t.Errorf("Expected Content-Type to contain application/javascript, got %s", ctype)
	}

	if allowed := rr.Header().Get("Service-Worker-Allowed"); allowed != "/" {
		t.Errorf("Expected Service-Worker-Allowed: /, got %q", allowed)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "notificationclick") || !strings.Contains(body, "Youth Community") {
		t.Errorf("Expected sw.js body to contain call notification logic, got: %s", body)
	}
}

func TestUploadsRoute_DurableMediaServing(t *testing.T) {
	mux, cleanup := setupTestEnv(t)
	defer cleanup()

	// 1. Create a dummy PNG image payload
	pngPayload := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}
	testKey := "posts/durable_test_image.png"

	// 2. Persist directly in PostgreSQL/SQLite durable media storage
	if err := repository.SaveMediaBlob(testKey, "image/png", pngPayload); err != nil {
		t.Fatalf("Failed to save media blob: %v", err)
	}

	// 3. Ensure disk file does not exist (simulating fresh container restart on Render)
	diskFile := filepath.Join("uploads", "posts", "durable_test_image.png")
	_ = os.Remove(diskFile)

	// 4. Send GET request to /uploads/posts/durable_test_image.png
	req := httptest.NewRequest(http.MethodGet, "/uploads/"+testKey, nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected HTTP 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	if ctype := rr.Header().Get("Content-Type"); !strings.Contains(ctype, "image/png") {
		t.Errorf("Expected Content-Type image/png, got %s", ctype)
	}

	if !bytes.Equal(rr.Body.Bytes(), pngPayload) {
		t.Errorf("Response body does not match expected image bytes")
	}

	// 5. Verify file was restored to disk for fast caching
	if _, err := os.Stat(diskFile); os.IsNotExist(err) {
		t.Errorf("Expected file to be reconstructed on disk at %s", diskFile)
	}
	_ = os.Remove(diskFile)
}

func TestUploadsRoute_PathPrefixNormalization(t *testing.T) {
	mux, cleanup := setupTestEnv(t)
	defer cleanup()

	pngPayload := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	testFilename := "normalized_test_pic.png"

	if err := repository.SaveMediaBlob("posts/"+testFilename, "image/png", pngPayload); err != nil {
		t.Fatalf("Failed to save media blob: %v", err)
	}

	// Request without "posts/" prefix: /uploads/normalized_test_pic.png
	req := httptest.NewRequest(http.MethodGet, "/uploads/"+testFilename, nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected HTTP 200 OK for un-prefixed path, got %d", rr.Code)
	}

	if !bytes.Equal(rr.Body.Bytes(), pngPayload) {
		t.Errorf("Expected body to match pngPayload")
	}
	_ = os.Remove(filepath.Join("uploads", "posts", testFilename))
}

func TestUploadsRoute_FallbackPlaceholders(t *testing.T) {
	mux, cleanup := setupTestEnv(t)
	defer cleanup()

	// 1. Missing post image should return 200 OK with SVG placeholder
	reqPost := httptest.NewRequest(http.MethodGet, "/uploads/posts/genuinely_missing_post.jpg", nil)
	rrPost := httptest.NewRecorder()
	mux.ServeHTTP(rrPost, reqPost)

	if rrPost.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for missing post fallback, got %d", rrPost.Code)
	}
	if !strings.Contains(rrPost.Header().Get("Content-Type"), "image/svg+xml") {
		t.Errorf("Expected SVG content type, got %s", rrPost.Header().Get("Content-Type"))
	}
	if !strings.Contains(rrPost.Body.String(), "<svg") {
		t.Errorf("Expected SVG body in fallback")
	}

	// 2. Missing user avatar should return 200 OK with default avatar SVG
	reqAvatar := httptest.NewRequest(http.MethodGet, "/uploads/profiles/genuinely_missing_avatar.jpg", nil)
	rrAvatar := httptest.NewRecorder()
	mux.ServeHTTP(rrAvatar, reqAvatar)

	if rrAvatar.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for missing profile photo fallback, got %d", rrAvatar.Code)
	}
	if !strings.Contains(rrAvatar.Header().Get("Content-Type"), "image/svg+xml") {
		t.Errorf("Expected SVG content type, got %s", rrAvatar.Header().Get("Content-Type"))
	}
	if !strings.Contains(rrAvatar.Body.String(), "<svg") {
		t.Errorf("Expected SVG body in default avatar fallback")
	}
}

func TestUploadsRoute_ProfilePhotoDurablePersistence(t *testing.T) {
	mux, cleanup := setupTestEnv(t)
	defer cleanup()

	jpegPayload := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}
	testAvatar := "profiles/persisted_avatar.jpg"

	if err := repository.SaveMediaBlob(testAvatar, "image/jpeg", jpegPayload); err != nil {
		t.Fatalf("Failed to save media blob: %v", err)
	}

	diskFile := filepath.Join("uploads", "profiles", "persisted_avatar.jpg")
	_ = os.Remove(diskFile)

	req := httptest.NewRequest(http.MethodGet, "/uploads/"+testAvatar, nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for profile photo, got %d", rr.Code)
	}

	if ctype := rr.Header().Get("Content-Type"); !strings.Contains(ctype, "image/jpeg") {
		t.Errorf("Expected image/jpeg, got %s", ctype)
	}

	if !bytes.Equal(rr.Body.Bytes(), jpegPayload) {
		t.Errorf("Returned bytes do not match original profile photo")
	}
	_ = os.Remove(diskFile)
}

func TestUploadsRoute_VideoStreamingByteRangeAndCORS(t *testing.T) {
	mux, cleanup := setupTestEnv(t)
	defer cleanup()

	_ = os.MkdirAll("uploads/posts", 0755)
	testVideoPath := filepath.Join("uploads", "posts", "stream_test.mp4")
	videoData := []byte("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz")
	if err := os.WriteFile(testVideoPath, videoData, 0644); err != nil {
		t.Fatalf("Failed to create test video file: %v", err)
	}
	defer os.Remove(testVideoPath)

	// 1. Test Byte-Range GET request (HTTP 206 Partial Content)
	req := httptest.NewRequest(http.MethodGet, "/uploads/posts/stream_test.mp4", nil)
	req.Header.Set("Range", "bytes=0-9")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusPartialContent {
		t.Fatalf("Expected 206 Partial Content for byte-range video request, got %d", rr.Code)
	}

	if ctype := rr.Header().Get("Content-Type"); !strings.Contains(ctype, "video/mp4") {
		t.Errorf("Expected video/mp4 Content-Type, got %q", ctype)
	}

	if crange := rr.Header().Get("Content-Range"); !strings.HasPrefix(crange, "bytes 0-9/") {
		t.Errorf("Expected Content-Range starting with 'bytes 0-9/', got %q", crange)
	}

	if acceptRanges := rr.Header().Get("Accept-Ranges"); acceptRanges != "bytes" {
		t.Errorf("Expected Accept-Ranges: bytes, got %q", acceptRanges)
	}

	if cors := rr.Header().Get("Access-Control-Allow-Origin"); cors != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin: *, got %q", cors)
	}

	if expose := rr.Header().Get("Access-Control-Expose-Headers"); !strings.Contains(expose, "Content-Range") {
		t.Errorf("Expected Content-Range in Access-Control-Expose-Headers, got %q", expose)
	}

	if !bytes.Equal(rr.Body.Bytes(), videoData[:10]) {
		t.Errorf("Expected first 10 bytes %q, got %q", videoData[:10], rr.Body.Bytes())
	}

	// 2. Test OPTIONS Preflight request (HTTP 204 No Content)
	reqOptions := httptest.NewRequest(http.MethodOptions, "/uploads/posts/stream_test.mp4", nil)
	rrOptions := httptest.NewRecorder()
	mux.ServeHTTP(rrOptions, reqOptions)

	if rrOptions.Code != http.StatusNoContent {
		t.Errorf("Expected 204 No Content for OPTIONS preflight, got %d", rrOptions.Code)
	}
	if rrOptions.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Expected CORS header on OPTIONS response")
	}
}

func TestUploadsRoute_MissingVideoReturns404NotSVG(t *testing.T) {
	mux, cleanup := setupTestEnv(t)
	defer cleanup()

	// Missing video must return 404 Not Found, NEVER 200 with SVG (which breaks HTML5 video players)
	missingVideos := []string{
		"/uploads/posts/genuinely_missing.mp4",
		"/uploads/posts/nonexistent_clip.webm",
		"/uploads/posts/another_service.mov",
	}

	for _, vpath := range missingVideos {
		req := httptest.NewRequest(http.MethodGet, vpath, nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("Expected 404 Not Found for missing video %s, got %d", vpath, rr.Code)
		}

		if strings.Contains(rr.Header().Get("Content-Type"), "image/svg+xml") {
			t.Errorf("CRITICAL BUG: Missing video %s returned SVG Content-Type!", vpath)
		}

		if strings.Contains(rr.Body.String(), "<svg") {
			t.Errorf("CRITICAL BUG: Missing video %s returned SVG body!", vpath)
		}
	}
}

func TestUploadsRoute_DurableVideoRestorationAndStreaming(t *testing.T) {
	mux, cleanup := setupTestEnv(t)
	defer cleanup()

	testBlobKey := "posts/durable_clip.mp4"
	videoData := []byte("HEADER-MP4-STREAM-TEST-DURABLE-BLOB-CONTENT-BYTE-RANGE-CHECK")

	if err := repository.SaveMediaBlob(testBlobKey, "video/mp4", videoData); err != nil {
		t.Fatalf("Failed to save durable video blob: %v", err)
	}

	// Ensure file is deleted from disk to simulate ephemeral container restart
	diskFile := filepath.Join("uploads", "posts", "durable_clip.mp4")
	_ = os.Remove(diskFile)

	// Make byte-range request to trigger restoration from DB + byte-range streaming
	req := httptest.NewRequest(http.MethodGet, "/uploads/"+testBlobKey, nil)
	req.Header.Set("Range", "bytes=0-9")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusPartialContent {
		t.Fatalf("Expected 206 Partial Content on durable video restoration, got %d", rr.Code)
	}

	if ctype := rr.Header().Get("Content-Type"); !strings.Contains(ctype, "video/mp4") {
		t.Errorf("Expected video/mp4 Content-Type, got %q", ctype)
	}

	if !bytes.Equal(rr.Body.Bytes(), videoData[:10]) {
		t.Errorf("Expected restored byte-range content %q, got %q", videoData[:10], rr.Body.Bytes())
	}

	// Verify file was restored to disk
	if _, err := os.Stat(diskFile); os.IsNotExist(err) {
		t.Errorf("Expected disk file to be recreated after durable DB fetch")
	}

	_ = os.Remove(diskFile)
	_ = repository.DeleteMediaBlob(testBlobKey)
}



