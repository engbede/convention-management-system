package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"convention-management-system/models"
	"convention-management-system/repository"
)

func TestListProgrammesHandler_RendersPage(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/programmes", nil)
	rr := httptest.NewRecorder()

	ListProgrammes(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK for ListProgrammes, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Live Streaming &amp; Online Programmes") && !strings.Contains(body, "Live Streaming & Online Programmes") {
		t.Errorf("Expected page title in rendered HTML")
	}
	if !strings.Contains(body, "Live Now") {
		t.Errorf("Expected Live Now tab in rendered HTML")
	}
}

func TestViewProgrammeHandler_Public(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	p := &models.Programme{
		Title:             "Sunday Worship Service",
		Category:          models.ProgrammeCategoryService,
		OrganizerID:       1,
		ScheduleStartTime: time.Now(),
		ScheduleEndTime:   time.Now().Add(2 * time.Hour),
		Timezone:          "UTC",
		Visibility:        models.ProgrammeVisibilityPublic,
		Status:            models.ProgrammeStatusLive,
		YouTubeVideoID:    "dQw4w9WgXcQ",
	}
	id, err := repository.CreateProgramme(p)
	if err != nil {
		t.Fatalf("Failed to create test programme: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/programmes/view?id=%d", id), nil)
	rr := httptest.NewRecorder()

	ViewProgramme(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK for ViewProgramme, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Sunday Worship Service") {
		t.Errorf("Expected programme title in rendered page")
	}
	if !strings.Contains(body, "youtube-nocookie.com/embed/dQw4w9WgXcQ") {
		t.Errorf("Expected YouTube embed URL in player")
	}
}

func TestViewProgrammeHandler_MembersOnlyRedirectsAnonymous(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	p := &models.Programme{
		Title:             "Private Leadership Meeting",
		Category:          models.ProgrammeCategorySpecial,
		OrganizerID:       1,
		ScheduleStartTime: time.Now(),
		ScheduleEndTime:   time.Now().Add(2 * time.Hour),
		Timezone:          "UTC",
		Visibility:        models.ProgrammeVisibilityMembersOnly,
		Status:            models.ProgrammeStatusScheduled,
	}
	id, err := repository.CreateProgramme(p)
	if err != nil {
		t.Fatalf("Failed to create test programme: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/programmes/view?id=%d", id), nil)
	rr := httptest.NewRecorder()

	ViewProgramme(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("Expected status 303 See Other for anonymous member-only view, got %d", rr.Code)
	}
	if !strings.Contains(rr.Header().Get("Location"), "/community/login") {
		t.Errorf("Expected redirect to login page, got: %s", rr.Header().Get("Location"))
	}
}

func TestSendProgrammeChatMessageUnauthorized(t *testing.T) {
	form := url.Values{}
	form.Set("programme_id", "1")
	form.Set("message", "Praise God!")

	req := httptest.NewRequest(http.MethodPost, "/programmes/chat/send", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	SendProgrammeChatMessage(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized, got: %d", resp.StatusCode)
	}

	var res map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if res["success"] == true {
		t.Errorf("expected success to be false for unauthenticated chat attempt")
	}
}

func TestSubmitPrayerRequestUnauthorized(t *testing.T) {
	form := url.Values{}
	form.Set("programme_id", "1")
	form.Set("request", "Please pray for youth exam success")
	form.Set("is_private", "1")

	req := httptest.NewRequest(http.MethodPost, "/programmes/prayer/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	SubmitPrayerRequest(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized, got: %d", resp.StatusCode)
	}

	var res map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if res["success"] == true {
		t.Errorf("expected success to be false for unauthenticated prayer request")
	}
}

func TestTransitionProgrammeStatusUnauthorized(t *testing.T) {
	form := url.Values{}
	form.Set("id", "1")
	form.Set("status", "live")

	req := httptest.NewRequest(http.MethodPost, "/programmes/status", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	TransitionProgrammeStatusHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized, got: %d", resp.StatusCode)
	}
}

func TestPrayerRequestPrivacyEnforcement(t *testing.T) {
	reqs, err := repository.GetProgrammePrayerRequests(1, 9999, false)
	if err != nil {
		t.Logf("GetProgrammePrayerRequests returned: %v (expected if DB empty)", err)
	}

	for _, pr := range reqs {
		if pr.IsPrivate && pr.UserID != 9999 {
			t.Errorf("Privacy leak! User 9999 received private prayer request from user %d", pr.UserID)
		}
	}
}

func TestProgrammeStateTransitions(t *testing.T) {
	p := &models.Programme{
		ID:     1,
		Status: models.ProgrammeStatusDraft,
	}

	if p.IsLive() {
		t.Errorf("expected draft programme to not be live")
	}

	p.Status = models.ProgrammeStatusLive
	if !p.IsLive() {
		t.Errorf("expected live programme to return IsLive() = true")
	}

	p.Status = models.ProgrammeStatusEnded
	if !p.IsEnded() {
		t.Errorf("expected ended programme to return IsEnded() = true")
	}

	p.Status = models.ProgrammeStatusCancelled
	if !p.IsCancelled() {
		t.Errorf("expected cancelled programme to return IsCancelled() = true")
	}
}

func TestPublicLiveViewing_AccessibleWithoutAuth(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Empty state when no live programme
	req := httptest.NewRequest(http.MethodGet, "/live", nil)
	rr := httptest.NewRecorder()

	PublicLiveViewing(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK for public live viewing, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "No live programme is broadcasting at the moment. Please check back later.") {
		t.Errorf("expected friendly empty state message in rendered HTML")
	}

	// 2. Active live programme state
	p := &models.Programme{
		Title:             "Convention Revival Service",
		Description:       "Evening praise, prayer and revival word",
		Category:          models.ProgrammeCategoryService,
		OrganizerID:       1,
		ScheduleStartTime: time.Now(),
		ScheduleEndTime:   time.Now().Add(2 * time.Hour),
		Timezone:          "UTC",
		Visibility:        models.ProgrammeVisibilityPublic,
		Status:            models.ProgrammeStatusLive,
		YouTubeVideoID:    "liveStream01",
	}
	_, err := repository.CreateProgramme(p)
	if err != nil {
		t.Fatalf("failed to create live test programme: %v", err)
	}

	reqLive := httptest.NewRequest(http.MethodGet, "/live", nil)
	rrLive := httptest.NewRecorder()

	PublicLiveViewing(rrLive, reqLive)

	if rrLive.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK with live programme, got %d", rrLive.Code)
	}

	bodyLive := rrLive.Body.String()
	if !strings.Contains(bodyLive, "Convention Revival Service") {
		t.Errorf("expected live programme title in rendered HTML")
	}
	if !strings.Contains(bodyLive, "liveStream01") {
		t.Errorf("expected YouTube embed ID in rendered HTML")
	}
}

func TestProgrammeManagement_AdminProtection(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Unauthenticated request to /programmes/new should redirect to login
	reqNew := httptest.NewRequest(http.MethodGet, "/programmes/new", nil)
	rrNew := httptest.NewRecorder()
	NewProgramme(rrNew, reqNew)
	if rrNew.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect to admin login for NewProgramme, got %d", rrNew.Code)
	}

	// 2. Non-admin POST to /programmes/create should be rejected with 403 Forbidden
	form := url.Values{}
	form.Set("title", "Unauthorized Programme")
	reqCreate := httptest.NewRequest(http.MethodPost, "/programmes/create", strings.NewReader(form.Encode()))
	reqCreate.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrCreate := httptest.NewRecorder()
	CreateProgramme(rrCreate, reqCreate)
	if rrCreate.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for non-admin CreateProgramme, got %d", rrCreate.Code)
	}

	// 3. Non-admin request to /programmes/edit should redirect to login
	reqEdit := httptest.NewRequest(http.MethodGet, "/programmes/edit?id=1", nil)
	rrEdit := httptest.NewRecorder()
	EditProgramme(rrEdit, reqEdit)
	if rrEdit.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect to admin login for EditProgramme, got %d", rrEdit.Code)
	}
}
