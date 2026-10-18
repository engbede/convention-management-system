package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"convention-management-system/database"
	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/services"
	"convention-management-system/sessions"
)

func setupSidebarTestDB(t *testing.T) func() {
	t.Helper()
	origWd, _ := os.Getwd()
	if strings.HasSuffix(origWd, "handlers") {
		_ = os.Chdir("..")
	}

	origDB := database.DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "sidebar_test.db")

	testDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	database.DB = testDB

	if err := database.MigrateSQLite(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Seed test users
	_, err = testDB.Exec(`
		INSERT INTO users (id, full_name, username, email, password_hash)
		VALUES 
			(1, 'Sister Mary', 'mary_grace', 'mary@example.com', 'hash'),
			(2, 'Brother John', 'john_faith', 'john@example.com', 'hash');
	`)
	if err != nil {
		t.Fatalf("failed inserting test users: %v", err)
	}

	return func() {
		_ = os.Chdir(origWd)
		testDB.Close()
		database.DB = origDB
	}
}

func TestCommunityPrayerRequests_ScopesAndIntercession(t *testing.T) {
	cleanup := setupSidebarTestDB(t)
	defer cleanup()

	// 1. Create prayer requests with different scopes
	req1 := &models.CommunityPrayerRequest{
		UserID:       1,
		Title:        "Healing for Grandma",
		Description:  "Please join us in prayer for quick recovery and divine health.",
		PrivacyScope: "public",
	}
	if err := repository.CreateCommunityPrayerRequest(req1); err != nil {
		t.Fatalf("failed creating public request: %v", err)
	}

	req2 := &models.CommunityPrayerRequest{
		UserID:       1,
		Title:        "Personal Guidance",
		Description:  "Seeking God's direction in my upcoming career choices.",
		PrivacyScope: "fellowship",
	}
	if err := repository.CreateCommunityPrayerRequest(req2); err != nil {
		t.Fatalf("failed creating fellowship request: %v", err)
	}

	req3 := &models.CommunityPrayerRequest{
		UserID:       1,
		Title:        "Private Journal Entry",
		Description:  "Personal reflections and confession.",
		PrivacyScope: "private",
	}
	if err := repository.CreateCommunityPrayerRequest(req3); err != nil {
		t.Fatalf("failed creating private request: %v", err)
	}

	// 2. Fetch requests as Brother John (user 2)
	// Should see public (req1) and fellowship (req2), but NOT private (req3)
	requestsUser2, err := repository.GetCommunityPrayerRequests(2, false, 10)
	if err != nil {
		t.Fatalf("failed getting requests for user 2: %v", err)
	}
	if len(requestsUser2) != 2 {
		t.Fatalf("expected 2 visible requests for user 2, got %d", len(requestsUser2))
	}

	// 3. Fetch requests as Sister Mary (user 1 - owner)
	// Should see all 3 because she owns req3
	requestsUser1, err := repository.GetCommunityPrayerRequests(1, false, 10)
	if err != nil {
		t.Fatalf("failed getting requests for user 1: %v", err)
	}
	if len(requestsUser1) != 3 {
		t.Fatalf("expected 3 visible requests for owner (user 1), got %d", len(requestsUser1))
	}

	// 4. Test TogglePrayerRequestPray
	// User 2 prays for req1
	hasPrayed, newCount, err := repository.TogglePrayerRequestPray(req1.ID, 2)
	if err != nil {
		t.Fatalf("failed to toggle pray: %v", err)
	}
	if !hasPrayed || newCount != 1 {
		t.Errorf("expected hasPrayed=true, newCount=1; got hasPrayed=%v, newCount=%d", hasPrayed, newCount)
	}

	// User 2 toggles pray again -> should unpray
	hasPrayed, newCount, err = repository.TogglePrayerRequestPray(req1.ID, 2)
	if err != nil {
		t.Fatalf("failed to toggle unpray: %v", err)
	}
	if hasPrayed || newCount != 0 {
		t.Errorf("expected hasPrayed=false, newCount=0; got hasPrayed=%v, newCount=%d", hasPrayed, newCount)
	}
}

func TestCommunitySidebarService_Aggregation(t *testing.T) {
	cleanup := setupSidebarTestDB(t)
	defer cleanup()

	// Seed a notice and programme
	_, err := database.DB.Exec(`
		INSERT INTO notices (title, message, audience, priority, start_date, end_date, created_by, created_at)
		VALUES ('Convention Opening', 'Welcome all delegates to Apa 2026', 'all', 'high', '2026-10-01', '2026-10-05', 'Admin', CURRENT_TIMESTAMP);
	`)
	if err != nil {
		t.Fatalf("failed inserting test notice: %v", err)
	}

	_, err = database.DB.Exec(`
		INSERT INTO programmes (title, description, category, organizer_id, schedule_start_time, schedule_end_time, timezone, visibility, status, created_at, updated_at)
		VALUES ('Youth Revival Session', 'Spiritual awakening', 'Worship', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'Africa/Lagos', 'public', 'scheduled', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
	`)
	if err != nil {
		t.Fatalf("failed inserting test programme: %v", err)
	}

	sidebarData, err := services.GetCommunitySidebarData(1, false)
	if err != nil {
		t.Fatalf("failed getting sidebar data: %v", err)
	}

	if sidebarData.Stats == nil {
		t.Fatal("expected sidebar stats to be non-nil")
	}
	if sidebarData.Stats.MembersCount != 2 {
		t.Errorf("expected 2 members, got %d", sidebarData.Stats.MembersCount)
	}
	if sidebarData.BibleStudy == nil || sidebarData.BibleStudy.Theme == "" {
		t.Error("expected weekly bible study theme to be populated")
	}
	if len(sidebarData.Announcements) == 0 {
		t.Error("expected announcements to be populated")
	}
	if len(sidebarData.UpcomingEvents) == 0 {
		t.Error("expected upcoming events to be populated")
	}
}

func TestSubmitCommunityPrayerRequest_Handler(t *testing.T) {
	cleanup := setupSidebarTestDB(t)
	defer cleanup()

	// 1. Submit valid prayer request with JSON accept
	form := url.Values{
		"title":         {"Exam Success"},
		"description":   {"Praying for grace and memory in final semester exams."},
		"privacy_scope": {"fellowship"},
	}
	req := httptest.NewRequest(http.MethodPost, "/community/prayer/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Set session
	session, _ := sessions.Store.Get(req, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(req, httptest.NewRecorder())

	rr := httptest.NewRecorder()
	SubmitCommunityPrayerRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK for JSON prayer submission, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed decoding json response: %v", err)
	}
	if resp["success"] != true {
		t.Errorf("expected success=true, got %v", resp)
	}

	// 2. Toggle prayer via TogglePrayerIntercessionAPI
	reqID := int(resp["id"].(float64))
	toggleForm := url.Values{
		"request_id": {strconv.Itoa(reqID)},
	}
	toggleReq := httptest.NewRequest(http.MethodPost, "/community/prayer/pray", strings.NewReader(toggleForm.Encode()))
	toggleReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	session2, _ := sessions.Store.Get(toggleReq, "youth-community")
	session2.Values["user_id"] = 2
	_ = session2.Save(toggleReq, httptest.NewRecorder())

	toggleRR := httptest.NewRecorder()
	TogglePrayerIntercessionAPI(toggleRR, toggleReq)

	if toggleRR.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK for prayer toggle, got %d", toggleRR.Code)
	}

	var toggleResp map[string]any
	_ = json.Unmarshal(toggleRR.Body.Bytes(), &toggleResp)
	if toggleResp["has_prayed"] != true || toggleResp["pray_count"].(float64) != 1 {
		t.Errorf("expected has_prayed=true and count=1, got %v", toggleResp)
	}
}

func TestCommunityFeed_WithSidebarRender(t *testing.T) {
	cleanup := setupSidebarTestDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/community", nil)
	session, _ := sessions.Store.Get(req, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(req, httptest.NewRecorder())

	rr := httptest.NewRecorder()
	Community(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK for /community, got %d", rr.Code)
	}

	body := rr.Body.String()
	// Assert dynamic sections are rendered
	if !strings.Contains(body, "Community Statistics") {
		t.Errorf("expected Community Statistics in body")
	}
	if !strings.Contains(body, "Upcoming Events") {
		t.Errorf("expected Upcoming Events in body")
	}
	if !strings.Contains(body, "Weekly Bible Study") {
		t.Errorf("expected Weekly Bible Study in body")
	}
	if !strings.Contains(body, "Prayer Requests") {
		t.Errorf("expected Prayer Requests in body")
	}
	if !strings.Contains(body, "submitPrayerModal") {
		t.Errorf("expected submitPrayerModal in body")
	}
}
