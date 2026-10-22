package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"convention-management-system/database"
	"convention-management-system/middleware"
	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/services"
	"convention-management-system/sessions"
	"convention-management-system/utils"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) func() {
	t.Helper()
	origWd, _ := os.Getwd()
	if strings.HasSuffix(origWd, "handlers") {
		_ = os.Chdir("..")
	}

	testDBPath := "test_handlers.db"
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
		INSERT INTO users (id, full_name, username, password_hash)
		VALUES 
			(1, 'User One', 'user1', 'hash1'),
			(2, 'User Two', 'user2', 'hash2'),
			(3, 'User Three', 'user3', 'hash3');

		INSERT INTO terms_acceptances (user_id, terms_version, accepted_at, ip_address, user_agent)
		VALUES 
			(1, 'v1.0', CURRENT_TIMESTAMP, '127.0.0.1', 'TestSuite'),
			(2, 'v1.0', CURRENT_TIMESTAMP, '127.0.0.1', 'TestSuite'),
			(3, 'v1.0', CURRENT_TIMESTAMP, '127.0.0.1', 'TestSuite');
	`)
	if err != nil {
		t.Fatalf("Failed to insert test users: %v", err)
	}

	utils.SetMediaStorageHooks(repository.SaveMediaBlob, repository.DeleteMediaBlob)

	return func() {
		db.Close()
		_ = os.Remove(testDBPath)
		_ = os.Chdir(origWd)
	}
}

func TestChatInboxUnauthenticated(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/messages", nil)
	rr := httptest.NewRecorder()

	handler := middleware.RequireLogin(ChatInbox)
	handler(rr, req)

	// Should redirect to /community/login
	if rr.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303 See Other, got %d", rr.Code)
	}

	loc := rr.Header().Get("Location")
	if loc != "/community/login" {
		t.Errorf("Expected redirect to /community/login, got %q", loc)
	}
}

func TestChatInboxAuthenticated(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	rr := httptest.NewRecorder()

	// Attach authenticated session
	initReq := httptest.NewRequest(http.MethodGet, "/messages", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)

	// Create fresh request with the session cookie attached
	freshReq := httptest.NewRequest(http.MethodGet, "/messages", nil)
	for _, cookie := range saveRec.Result().Cookies() {
		freshReq.AddCookie(cookie)
	}

	handler := middleware.RequireLogin(ChatInbox)
	handler(rr, freshReq)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK for authenticated inbox, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Youth Chat") {
		t.Errorf("Expected response body to contain 'Youth Chat', got body length %d", len(body))
	}
}

func TestChatInbox_ChronologicalGroupingAndFormatting(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Create conversations
	_, err := database.DB.Exec(`
		INSERT INTO conversations (id, type, created_by, created_at) VALUES 
			(1, 'direct', 1, datetime('now')),
			(2, 'direct', 1, datetime('now', '-1 day'));
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES 
			(1, 1, 'member'), (1, 2, 'member'),
			(2, 1, 'member'), (2, 3, 'member');
		INSERT INTO messages (conversation_id, sender_id, message_type, body, created_at) VALUES
			(1, 1, 'text', 'Today message', datetime('now')),
			(2, 3, 'text', 'Yesterday message', datetime('now', '-1 day'));
	`)
	if err != nil {
		t.Fatalf("Failed to seed conversations: %v", err)
	}

	initReq := httptest.NewRequest(http.MethodGet, "/messages?c=1", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)

	freshReq := httptest.NewRequest(http.MethodGet, "/messages?c=1", nil)
	for _, cookie := range saveRec.Result().Cookies() {
		freshReq.AddCookie(cookie)
	}

	rr := httptest.NewRecorder()
	handler := middleware.RequireLogin(ChatInbox)
	handler(rr, freshReq)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK for inbox, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "conversation-group") {
		t.Errorf("Expected response body to contain 'conversation-group'")
	}
	if !strings.Contains(body, "conversation-group-heading") {
		t.Errorf("Expected response body to contain 'conversation-group-heading'")
	}
	if !strings.Contains(body, "Today") {
		t.Errorf("Expected response body to contain 'Today' heading")
	}
	if !strings.Contains(body, "Yesterday") {
		t.Errorf("Expected response body to contain 'Yesterday' heading")
	}
	if !strings.Contains(body, "message-date-separator") {
		t.Errorf("Expected response body to contain 'message-date-separator'")
	}
}

func TestToggleMuteAndArchiveAPI(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Create a conversation
	_, err := database.DB.Exec(`
		INSERT INTO conversations (id, type, created_by) VALUES (1, 'direct', 1);
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (1, 1, 'member');
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (1, 2, 'member');
	`)
	if err != nil {
		t.Fatalf("Failed to seed conversation: %v", err)
	}

	// 2. Prepare session for user 1
	initReq := httptest.NewRequest(http.MethodPost, "/messages/mute", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)

	// 3. Test Mute API
	muteReq := httptest.NewRequest(http.MethodPost, "/messages/mute?conversation_id=1", nil)
	muteReq.Header.Set("Accept", "application/json")
	for _, cookie := range saveRec.Result().Cookies() {
		muteReq.AddCookie(cookie)
	}
	muteRR := httptest.NewRecorder()
	ToggleMuteAPI(muteRR, muteReq)

	if muteRR.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK for ToggleMuteAPI, got %d (body: %s)", muteRR.Code, muteRR.Body.String())
	}
	if !strings.Contains(muteRR.Body.String(), `"is_muted":true`) {
		t.Errorf("Expected response to indicate is_muted: true, got %s", muteRR.Body.String())
	}

	// 4. Test Archive API
	archReq := httptest.NewRequest(http.MethodPost, "/messages/archive?conversation_id=1", nil)
	archReq.Header.Set("Accept", "application/json")
	for _, cookie := range saveRec.Result().Cookies() {
		archReq.AddCookie(cookie)
	}
	archRR := httptest.NewRecorder()
	ToggleArchiveAPI(archRR, archReq)

	if archRR.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK for ToggleArchiveAPI, got %d (body: %s)", archRR.Code, archRR.Body.String())
	}
	if !strings.Contains(archRR.Body.String(), `"is_archived":true`) {
		t.Errorf("Expected response to indicate is_archived: true, got %s", archRR.Body.String())
	}
}

func TestGetMessagesPaginationAPI(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Create a conversation with multiple messages
	_, err := database.DB.Exec(`
		INSERT INTO conversations (id, type, created_by) VALUES (1, 'direct', 1);
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (1, 1, 'member');
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (1, 2, 'member');
		INSERT INTO messages (id, conversation_id, sender_id, message_type, body, created_at, updated_at)
		VALUES 
			(1, 1, 1, 'text', 'Message 1', '2026-01-01 10:00:00', '2026-01-01 10:00:00'),
			(2, 1, 2, 'text', 'Message 2', '2026-01-01 10:01:00', '2026-01-01 10:01:00'),
			(3, 1, 1, 'text', 'Message 3', '2026-01-01 10:02:00', '2026-01-01 10:02:00');
	`)
	if err != nil {
		t.Fatalf("Failed to seed messages: %v", err)
	}

	// Prepare session for user 1
	initReq := httptest.NewRequest(http.MethodGet, "/messages/history", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)

	// Fetch page 1 (limit 2, offset 0)
	page1Req := httptest.NewRequest(http.MethodGet, "/messages/history?conversation_id=1&limit=2&offset=0", nil)
	for _, cookie := range saveRec.Result().Cookies() {
		page1Req.AddCookie(cookie)
	}
	page1RR := httptest.NewRecorder()
	GetMessagesAPI(page1RR, page1Req)

	if page1RR.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", page1RR.Code)
	}
	if !strings.Contains(page1RR.Body.String(), `"has_older":true`) {
		t.Errorf("Expected has_older: true for page 1, got %s", page1RR.Body.String())
	}
}

func TestChatWebSocketHandlerUnauthorized(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Unauthenticated request should be rejected with 401
	req := httptest.NewRequest(http.MethodGet, "/ws/chat", nil)
	rr := httptest.NewRecorder()

	ChatWebSocketHandler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized for unauthenticated WebSocket request, got %d", rr.Code)
	}
}

func TestSendMessageWithAttachmentAPI(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Create conversation between user 1 and user 2
	_, err := database.DB.Exec(`
		INSERT INTO conversations (id, type, created_by) VALUES (1, 'direct', 1);
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (1, 1, 'member');
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (1, 2, 'member');
	`)
	if err != nil {
		t.Fatalf("Failed to seed conversation: %v", err)
	}

	// Prepare session for user 1
	initReq := httptest.NewRequest(http.MethodPost, "/messages/upload", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)

	// 1. Valid PNG upload
	bodyBuf := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuf)
	_ = writer.WriteField("conversation_id", "1")
	_ = writer.WriteField("body", "Look at this picture")
	part, _ := writer.CreateFormFile("attachment", "photo.png")
	part.Write([]byte("\x89PNG\r\n\x1a\nfake-png-content"))
	writer.Close()

	uploadReq := httptest.NewRequest(http.MethodPost, "/messages/upload", bodyBuf)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadReq.Header.Set("Accept", "application/json")
	for _, cookie := range saveRec.Result().Cookies() {
		uploadReq.AddCookie(cookie)
	}

	uploadRR := httptest.NewRecorder()
	UploadAttachmentAPI(uploadRR, uploadReq)

	if uploadRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for valid attachment, got %d (body: %s)", uploadRR.Code, uploadRR.Body.String())
	}
	if !strings.Contains(uploadRR.Body.String(), `"AttachmentURL"`) || !strings.Contains(uploadRR.Body.String(), `"image"`) {
		t.Errorf("Expected response to contain AttachmentURL and image message type, got %s", uploadRR.Body.String())
	}

	// 2. Reject disallowed file extension (.exe)
	badBuf := &bytes.Buffer{}
	badWriter := multipart.NewWriter(badBuf)
	_ = badWriter.WriteField("conversation_id", "1")
	badPart, _ := badWriter.CreateFormFile("attachment", "danger.exe")
	badPart.Write([]byte("MZfake-exe"))
	badWriter.Close()

	badReq := httptest.NewRequest(http.MethodPost, "/messages/upload", badBuf)
	badReq.Header.Set("Content-Type", badWriter.FormDataContentType())
	badReq.Header.Set("Accept", "application/json")
	for _, cookie := range saveRec.Result().Cookies() {
		badReq.AddCookie(cookie)
	}

	badRR := httptest.NewRecorder()
	UploadAttachmentAPI(badRR, badReq)

	if badRR.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for disallowed file type, got %d", badRR.Code)
	}
}

func TestSendVoiceNoteAttachmentAPI_WithoutBody(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Create conversation between user 1 and user 2
	_, err := database.DB.Exec(`
		INSERT INTO conversations (id, type, created_by) VALUES (1, 'direct', 1);
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (1, 1, 'member');
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (1, 2, 'member');
	`)
	if err != nil {
		t.Fatalf("Failed to seed conversation: %v", err)
	}

	// Prepare session for user 1
	initReq := httptest.NewRequest(http.MethodPost, "/messages/upload", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)

	// Send WebM voice note with EMPTY body
	bodyBuf := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuf)
	_ = writer.WriteField("conversation_id", "1")
	_ = writer.WriteField("body", "") // Voice note recorded with no caption
	part, err := writer.CreateFormFile("attachment", "voice-note-test.webm")
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	part.Write([]byte("fake-webm-audio-data-stream-12345"))
	writer.Close()

	uploadReq := httptest.NewRequest(http.MethodPost, "/messages/upload", bodyBuf)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadReq.Header.Set("Accept", "application/json")
	for _, cookie := range saveRec.Result().Cookies() {
		uploadReq.AddCookie(cookie)
	}

	uploadRR := httptest.NewRecorder()
	UploadAttachmentAPI(uploadRR, uploadReq)

	if uploadRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for voice note attachment with empty body, got %d (body: %s)", uploadRR.Code, uploadRR.Body.String())
	}

	var resp struct {
		Success bool            `json:"success"`
		Message *models.Message `json:"message"`
	}
	if err := json.Unmarshal(uploadRR.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if !resp.Success || resp.Message == nil {
		t.Fatalf("Expected successful message in response, got %+v", resp)
	}
	if resp.Message.MessageType != "voice" {
		t.Errorf("Expected message_type 'voice', got %q", resp.Message.MessageType)
	}
	if resp.Message.AttachmentType != "audio/webm" {
		t.Errorf("Expected attachment_type 'audio/webm', got %q", resp.Message.AttachmentType)
	}
	if resp.Message.Body != "" {
		t.Errorf("Expected empty body, got %q", resp.Message.Body)
	}
	if !strings.HasPrefix(resp.Message.AttachmentURL, "/uploads/chat/") {
		t.Errorf("Expected attachment URL starting with '/uploads/chat/', got %q", resp.Message.AttachmentURL)
	}
}

func TestBlockAndUnblockUserAPI(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Prepare session for user 1
	initReq := httptest.NewRequest(http.MethodPost, "/messages/block", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)

	// 1. Block user 2
	blockReq := httptest.NewRequest(http.MethodPost, "/messages/block?user_id=2&reason=spam", nil)
	blockReq.Header.Set("Accept", "application/json")
	for _, cookie := range saveRec.Result().Cookies() {
		blockReq.AddCookie(cookie)
	}
	blockRR := httptest.NewRecorder()
	BlockUserAPI(blockRR, blockReq)

	if blockRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for BlockUserAPI, got %d", blockRR.Code)
	}
	if !strings.Contains(blockRR.Body.String(), `"success":true`) {
		t.Errorf("Expected success: true, got %s", blockRR.Body.String())
	}

	// 2. Unblock user 2
	unblockReq := httptest.NewRequest(http.MethodPost, "/messages/unblock?user_id=2", nil)
	unblockReq.Header.Set("Accept", "application/json")
	for _, cookie := range saveRec.Result().Cookies() {
		unblockReq.AddCookie(cookie)
	}
	unblockRR := httptest.NewRecorder()
	UnblockUserAPI(unblockRR, unblockReq)

	if unblockRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for UnblockUserAPI, got %d", unblockRR.Code)
	}
	if !strings.Contains(unblockRR.Body.String(), `"success":true`) {
		t.Errorf("Expected success: true, got %s", unblockRR.Body.String())
	}
}

func TestReportAbuseAPI(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Prepare session for user 1
	initReq := httptest.NewRequest(http.MethodPost, "/messages/report", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)

	// Submit report
	reportReq := httptest.NewRequest(http.MethodPost, "/messages/report?reported_user_id=2&reason=Harassment&details=Abusive+language", nil)
	reportReq.Header.Set("Accept", "application/json")
	for _, cookie := range saveRec.Result().Cookies() {
		reportReq.AddCookie(cookie)
	}
	reportRR := httptest.NewRecorder()
	ReportAbuseAPI(reportRR, reportReq)

	if reportRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for ReportAbuseAPI, got %d (body: %s)", reportRR.Code, reportRR.Body.String())
	}
	if !strings.Contains(reportRR.Body.String(), `"success":true`) {
		t.Errorf("Expected success: true, got %s", reportRR.Body.String())
	}
}

func TestSearchMessagesAPI(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Seed conversation and messages
	_, err := database.DB.Exec(`
		INSERT INTO conversations (id, type, created_by) VALUES (1, 'direct', 1);
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (1, 1, 'member');
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (1, 2, 'member');
		INSERT INTO messages (id, conversation_id, sender_id, message_type, body, created_at, updated_at)
		VALUES (1, 1, 1, 'text', 'Praise the Lord everyone', '2026-01-01 10:00:00', '2026-01-01 10:00:00');
	`)
	if err != nil {
		t.Fatalf("Failed to seed: %v", err)
	}

	// Prepare session for user 1
	initReq := httptest.NewRequest(http.MethodGet, "/messages/search", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)

	// Search
	searchReq := httptest.NewRequest(http.MethodGet, "/messages/search?q=Praise", nil)
	for _, cookie := range saveRec.Result().Cookies() {
		searchReq.AddCookie(cookie)
	}
	searchRR := httptest.NewRecorder()
	SearchMessagesAPI(searchRR, searchReq)

	if searchRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for SearchMessagesAPI, got %d", searchRR.Code)
	}
	if !strings.Contains(searchRR.Body.String(), "Praise the Lord") {
		t.Errorf("Expected search response to contain message body, got %s", searchRR.Body.String())
	}
}

func TestICEServersAPI(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Unauthenticated
	req := httptest.NewRequest(http.MethodGet, "/messages/call/ice-servers", nil)
	rr := httptest.NewRecorder()
	ICEServersAPI(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", rr.Code)
	}

	// 2. Authenticated
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(req, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(req, saveRec)

	authReq := httptest.NewRequest(http.MethodGet, "/messages/call/ice-servers", nil)
	for _, cookie := range saveRec.Result().Cookies() {
		authReq.AddCookie(cookie)
	}
	authRR := httptest.NewRecorder()
	ICEServersAPI(authRR, authReq)

	if authRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", authRR.Code)
	}
	if !strings.Contains(authRR.Body.String(), "stun:") {
		t.Errorf("Expected response to contain stun servers, got %s", authRR.Body.String())
	}
}

func TestLogCallAPI(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Seed conversation and call log
	_, err := database.DB.Exec(`
		INSERT INTO conversations (id, type, created_by) VALUES (1, 'direct', 1);
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (1, 1, 'member');
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (1, 2, 'member');
		INSERT INTO call_logs (id, conversation_id, caller_id, receiver_id, call_type, status)
		VALUES (1, 1, 1, 2, 'audio', 'initiated');
	`)
	if err != nil {
		t.Fatalf("Failed to seed: %v", err)
	}

	// Session for user 1
	initReq := httptest.NewRequest(http.MethodPost, "/messages/call/log", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)

	// Call update via JSON
	jsonBody := `{"call_id": 1, "status": "completed", "duration_seconds": 120}`
	req := httptest.NewRequest(http.MethodPost, "/messages/call/log", strings.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range saveRec.Result().Cookies() {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()
	LogCallAPI(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"success":true`) {
		t.Errorf("Expected success: true, got %s", rr.Body.String())
	}

	// Verify DB state
	logs, err := repository.GetConversationCallLogs(1, 10)
	if err != nil || len(logs) == 0 {
		t.Fatalf("Failed to retrieve call logs: %v", err)
	}
	if logs[0].Status != "completed" || logs[0].DurationSeconds != 120 {
		t.Errorf("Expected status 'completed' and duration 120, got status=%s, duration=%d", logs[0].Status, logs[0].DurationSeconds)
	}

	// Update to busy status
	busyJSON := `{"call_id": 1, "status": "busy", "duration_seconds": 0}`
	busyReq, busyRR := makeAuthReq(http.MethodPost, "/messages/call/log", busyJSON, "application/json", 1)
	LogCallAPI(busyRR, busyReq)
	if busyRR.Code != http.StatusOK {
		t.Errorf("Expected 200 OK on busy log update, got %d", busyRR.Code)
	}

	// Invalid call ID -> 400 Bad Request
	badReq, badRR := makeAuthReq(http.MethodPost, "/messages/call/log", `{"call_id": 0}`, "application/json", 1)
	LogCallAPI(badRR, badReq)
	if badRR.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for call_id 0, got %d", badRR.Code)
	}

	// Unauthorized request -> 401 Unauthorized
	unauthReq := httptest.NewRequest(http.MethodPost, "/messages/call/log", strings.NewReader(`{"call_id": 1}`))
	unauthRR := httptest.NewRecorder()
	LogCallAPI(unauthRR, unauthReq)
	if unauthRR.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized without session, got %d", unauthRR.Code)
	}
}

// Helper to create authenticated session request
func makeAuthReq(method, url string, body string, contentType string, userID int) (*http.Request, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")

	initReq := httptest.NewRequest(method, url, nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = userID
	_ = session.Save(initReq, saveRec)

	for _, cookie := range saveRec.Result().Cookies() {
		req.AddCookie(cookie)
	}
	return req, httptest.NewRecorder()
}

func TestGroupAPIs(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. CreateGroupAPI
	createJSON := `{"title": "Youth Prayer Fellowship", "category": "prayer", "description": "Intercessory prayer", "call_permission": "all", "is_public": true, "member_ids": [2]}`
	createReq, createRR := makeAuthReq(http.MethodPost, "/messages/group/create", createJSON, "application/json", 1)
	CreateGroupAPI(createRR, createReq)

	if createRR.Code != http.StatusOK {
		t.Fatalf("CreateGroupAPI failed: %d (body: %s)", createRR.Code, createRR.Body.String())
	}
	var createResp struct {
		Success        bool `json:"success"`
		ConversationID int  `json:"conversation_id"`
	}
	if err := json.Unmarshal(createRR.Body.Bytes(), &createResp); err != nil || !createResp.Success || createResp.ConversationID <= 0 {
		t.Fatalf("Unexpected CreateGroupAPI response: %s", createRR.Body.String())
	}
	groupID := createResp.ConversationID

	// 2. AddGroupMembersAPI (Owner 1 adds user 3)
	addJSON := fmt.Sprintf(`{"group_id": %d, "user_ids": [3]}`, groupID)
	addReq, addRR := makeAuthReq(http.MethodPost, "/messages/group/members/add", addJSON, "application/json", 1)
	AddGroupMembersAPI(addRR, addReq)

	if addRR.Code != http.StatusOK {
		t.Fatalf("AddGroupMembersAPI failed: %d (body: %s)", addRR.Code, addRR.Body.String())
	}

	// 3. UpdateGroupMemberRoleAPI (Owner 1 promotes user 2 to admin)
	roleJSON := fmt.Sprintf(`{"group_id": %d, "user_id": 2, "role": "admin"}`, groupID)
	roleReq, roleRR := makeAuthReq(http.MethodPost, "/messages/group/members/role", roleJSON, "application/json", 1)
	UpdateGroupMemberRoleAPI(roleRR, roleReq)

	if roleRR.Code != http.StatusOK {
		t.Fatalf("UpdateGroupMemberRoleAPI failed: %d (body: %s)", roleRR.Code, roleRR.Body.String())
	}

	// 4. UpdateGroupSettingsAPI (Owner updates group and sets join_policy to open)
	updateJSON := fmt.Sprintf(`{"group_id": %d, "title": "Youth Prayer Ministry", "category": "prayer", "description": "Updated description", "call_permission": "admin_only", "join_policy": "open"}`, groupID)
	updateReq, updateRR := makeAuthReq(http.MethodPost, "/messages/group/update", updateJSON, "application/json", 1)
	UpdateGroupSettingsAPI(updateRR, updateReq)

	if updateRR.Code != http.StatusOK {
		t.Fatalf("UpdateGroupSettingsAPI failed: %d (body: %s)", updateRR.Code, updateRR.Body.String())
	}

	// 5. RemoveGroupMemberAPI (Admin 2 removes user 3)
	removeJSON := fmt.Sprintf(`{"group_id": %d, "user_id": 3}`, groupID)
	removeReq, removeRR := makeAuthReq(http.MethodPost, "/messages/group/members/remove", removeJSON, "application/json", 2)
	RemoveGroupMemberAPI(removeRR, removeReq)

	if removeRR.Code != http.StatusOK {
		t.Fatalf("RemoveGroupMemberAPI failed: %d (body: %s)", removeRR.Code, removeRR.Body.String())
	}

	// 6. LeaveGroupAPI (User 2 leaves group)
	leaveJSON := fmt.Sprintf(`{"group_id": %d}`, groupID)
	leaveReq, leaveRR := makeAuthReq(http.MethodPost, "/messages/group/leave", leaveJSON, "application/json", 2)
	LeaveGroupAPI(leaveRR, leaveReq)

	if leaveRR.Code != http.StatusOK {
		t.Fatalf("LeaveGroupAPI failed: %d (body: %s)", leaveRR.Code, leaveRR.Body.String())
	}

	// 7. JoinGroupAPI (User 2 rejoins group)
	joinJSON := fmt.Sprintf(`{"group_id": %d}`, groupID)
	joinReq, joinRR := makeAuthReq(http.MethodPost, "/messages/group/join", joinJSON, "application/json", 2)
	JoinGroupAPI(joinRR, joinReq)

	if joinRR.Code != http.StatusOK {
		t.Fatalf("JoinGroupAPI failed: %d (body: %s)", joinRR.Code, joinRR.Body.String())
	}

	// 8. DiscoverGroupsAPI (User 3 discovers groups in category 'prayer')
	discReq, discRR := makeAuthReq(http.MethodGet, "/messages/groups/discover?category=prayer", "", "", 3)
	DiscoverGroupsAPI(discRR, discReq)

	if discRR.Code != http.StatusOK {
		t.Fatalf("DiscoverGroupsAPI failed: %d (body: %s)", discRR.Code, discRR.Body.String())
	}
	if !strings.Contains(discRR.Body.String(), "Youth Prayer Ministry") {
		t.Errorf("Expected DiscoverGroupsAPI to return 'Youth Prayer Ministry', got %s", discRR.Body.String())
	}
}

func TestGroupCallAPIs(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Create a group with User 1 as owner and User 2 and User 3 as members
	createJSON := `{"title": "Youth Fellowship Meeting", "category": "general", "description": "Weekly gathering", "call_permission": "all", "is_public": true, "member_ids": [2, 3]}`
	createReq, createRR := makeAuthReq(http.MethodPost, "/messages/groups/create", createJSON, "application/json", 1)
	CreateGroupAPI(createRR, createReq)

	if createRR.Code != http.StatusOK {
		t.Fatalf("CreateGroupAPI failed: %d (body: %s)", createRR.Code, createRR.Body.String())
	}

	var createResp struct {
		Success        bool `json:"success"`
		ConversationID int  `json:"conversation_id"`
	}
	_ = json.Unmarshal(createRR.Body.Bytes(), &createResp)
	groupID := createResp.ConversationID
	if groupID == 0 {
		t.Fatalf("Failed to parse group ID from create response: %s", createRR.Body.String())
	}

	// 2. StartGroupCallAPI: User 1 starts a video call
	startJSON := fmt.Sprintf(`{"conversation_id": %d, "call_type": "video"}`, groupID)
	startReq, startRR := makeAuthReq(http.MethodPost, "/messages/call/group/start", startJSON, "application/json", 1)
	StartGroupCallAPI(startRR, startReq)

	if startRR.Code != http.StatusOK {
		t.Fatalf("StartGroupCallAPI failed: %d (body: %s)", startRR.Code, startRR.Body.String())
	}
	if !strings.Contains(startRR.Body.String(), `"success":true`) {
		t.Errorf("Expected success:true in StartGroupCallAPI, got: %s", startRR.Body.String())
	}

	// 3. GetActiveGroupCallAPI: User 2 checks active call
	activeReq, activeRR := makeAuthReq(http.MethodGet, fmt.Sprintf("/messages/call/group/active?conversation_id=%d", groupID), "", "", 2)
	GetActiveGroupCallAPI(activeRR, activeReq)

	if activeRR.Code != http.StatusOK {
		t.Fatalf("GetActiveGroupCallAPI failed: %d (body: %s)", activeRR.Code, activeRR.Body.String())
	}
	if !strings.Contains(activeRR.Body.String(), `"active":true`) {
		t.Errorf("Expected active:true in GetActiveGroupCallAPI, got: %s", activeRR.Body.String())
	}

	// 4. JoinGroupCallAPI: User 2 joins call
	joinJSON := fmt.Sprintf(`{"conversation_id": %d}`, groupID)
	joinReq, joinRR := makeAuthReq(http.MethodPost, "/messages/call/group/join", joinJSON, "application/json", 2)
	JoinGroupCallAPI(joinRR, joinReq)

	if joinRR.Code != http.StatusOK {
		t.Fatalf("JoinGroupCallAPI failed for User 2: %d (body: %s)", joinRR.Code, joinRR.Body.String())
	}
	if !strings.Contains(joinRR.Body.String(), `"success":true`) {
		t.Errorf("Expected success:true in JoinGroupCallAPI, got: %s", joinRR.Body.String())
	}

	// 5. JoinGroupCallAPI: User 3 joins call
	joinReq3, joinRR3 := makeAuthReq(http.MethodPost, "/messages/call/group/join", joinJSON, "application/json", 3)
	JoinGroupCallAPI(joinRR3, joinReq3)

	if joinRR3.Code != http.StatusOK {
		t.Fatalf("JoinGroupCallAPI failed for User 3: %d (body: %s)", joinRR3.Code, joinRR3.Body.String())
	}

	// 5b. MuteGroupCallParticipantAPI:
	// User 3 (regular member) attempts to mute User 1 (owner) -> 403 Forbidden
	muteInvalidJSON := fmt.Sprintf(`{"conversation_id": %d, "user_id": 1}`, groupID)
	muteReq3, muteRR3 := makeAuthReq(http.MethodPost, "/messages/call/group/mute", muteInvalidJSON, "application/json", 3)
	MuteGroupCallParticipantAPI(muteRR3, muteReq3)
	if muteRR3.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden when member tries to mute owner, got %d", muteRR3.Code)
	}

	// User 1 (owner) remotely mutes User 3 (member) -> 200 OK
	muteValidJSON := fmt.Sprintf(`{"conversation_id": %d, "user_id": 3}`, groupID)
	muteReq1, muteRR1 := makeAuthReq(http.MethodPost, "/messages/call/group/mute", muteValidJSON, "application/json", 1)
	MuteGroupCallParticipantAPI(muteRR1, muteReq1)
	if muteRR1.Code != http.StatusOK {
		t.Fatalf("MuteGroupCallParticipantAPI failed for owner: %d (body: %s)", muteRR1.Code, muteRR1.Body.String())
	}
	if !strings.Contains(muteRR1.Body.String(), `"success":true`) {
		t.Errorf("Expected success:true in MuteGroupCallParticipantAPI, got: %s", muteRR1.Body.String())
	}

	// Mute with invalid ID -> 400 Bad Request
	muteBadReq, muteBadRR := makeAuthReq(http.MethodPost, "/messages/call/group/mute", `{"conversation_id": 0, "user_id": 0}`, "application/json", 1)
	MuteGroupCallParticipantAPI(muteBadRR, muteBadReq)
	if muteBadRR.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for invalid mute parameters, got %d", muteBadRR.Code)
	}

	// 6. KickGroupCallParticipantAPI: User 1 (owner) kicks User 3
	kickJSON := fmt.Sprintf(`{"conversation_id": %d, "user_id": 3}`, groupID)
	kickReq, kickRR := makeAuthReq(http.MethodPost, "/messages/call/group/kick", kickJSON, "application/json", 1)
	KickGroupCallParticipantAPI(kickRR, kickReq)

	if kickRR.Code != http.StatusOK {
		t.Fatalf("KickGroupCallParticipantAPI failed: %d (body: %s)", kickRR.Code, kickRR.Body.String())
	}
	if !strings.Contains(kickRR.Body.String(), `"success":true`) {
		t.Errorf("Expected success:true in KickGroupCallParticipantAPI, got: %s", kickRR.Body.String())
	}

	// 7. LeaveGroupCallAPI: User 2 leaves call
	leaveReq, leaveRR := makeAuthReq(http.MethodPost, "/messages/call/group/leave", joinJSON, "application/json", 2)
	LeaveGroupCallAPI(leaveRR, leaveReq)

	if leaveRR.Code != http.StatusOK {
		t.Fatalf("LeaveGroupCallAPI failed: %d (body: %s)", leaveRR.Code, leaveRR.Body.String())
	}
	if !strings.Contains(leaveRR.Body.String(), `"success":true`) {
		t.Errorf("Expected success:true in LeaveGroupCallAPI, got: %s", leaveRR.Body.String())
	}

	// 8. EndGroupCallAPI: User 1 ends call for everyone
	endReq, endRR := makeAuthReq(http.MethodPost, "/messages/call/group/end", joinJSON, "application/json", 1)
	EndGroupCallAPI(endRR, endReq)

	if endRR.Code != http.StatusOK {
		t.Fatalf("EndGroupCallAPI failed: %d (body: %s)", endRR.Code, endRR.Body.String())
	}
	if !strings.Contains(endRR.Body.String(), `"success":true`) {
		t.Errorf("Expected success:true in EndGroupCallAPI, got: %s", endRR.Body.String())
	}

	// 9. Verify call is no longer active
	activeReq2, activeRR2 := makeAuthReq(http.MethodGet, fmt.Sprintf("/messages/call/group/active?conversation_id=%d", groupID), "", "", 1)
	GetActiveGroupCallAPI(activeRR2, activeReq2)

	if activeRR2.Code != http.StatusOK {
		t.Fatalf("GetActiveGroupCallAPI failed: %d", activeRR2.Code)
	}
	if !strings.Contains(activeRR2.Body.String(), `"active":false`) {
		t.Errorf("Expected active:false after call ended, got: %s", activeRR2.Body.String())
	}
}

func TestSendMessageAPIWorkflow(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Seed conversations: conv 1 (direct: user 1 & user 2), conv 2 (group: user 1 owner, user 2 member)
	_, err := database.DB.Exec(`
		INSERT INTO conversations (id, type, created_by) VALUES (1, 'direct', 1);
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (1, 1, 'member');
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (1, 2, 'member');

		INSERT INTO conversations (id, type, title, created_by) VALUES (2, 'group', 'Youth General Meeting', 1);
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (2, 1, 'owner');
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (2, 2, 'member');
	`)
	if err != nil {
		t.Fatalf("Failed to seed conversations: %v", err)
	}

	// 1. Unauthenticated request should return 401
	unauthReq := httptest.NewRequest(http.MethodPost, "/messages/send", strings.NewReader(`{"conversation_id": 1, "body": "hello"}`))
	unauthReq.Header.Set("Content-Type", "application/json")
	unauthRR := httptest.NewRecorder()
	SendMessageAPI(unauthRR, unauthReq)
	if unauthRR.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for unauthenticated SendMessageAPI, got %d", unauthRR.Code)
	}

	// 2. JSON request to direct conversation (conv 1) from User 1
	jsonPayload := `{"conversation_id": 1, "body": "Hello direct friend", "client_message_id": "client-direct-1"}`
	req1, rr1 := makeAuthReq(http.MethodPost, "/messages/send", jsonPayload, "application/json", 1)
	SendMessageAPI(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for valid JSON SendMessageAPI, got %d (body: %s)", rr1.Code, rr1.Body.String())
	}
	var resp1 struct {
		Success bool `json:"success"`
		Message struct {
			ID              int    `json:"ID"`
			ConversationID  int    `json:"ConversationID"`
			SenderID        int    `json:"SenderID"`
			Body            string `json:"Body"`
			ClientMessageID string `json:"ClientMessageID"`
		} `json:"message"`
	}
	if err := json.Unmarshal(rr1.Body.Bytes(), &resp1); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}
	if !resp1.Success {
		t.Errorf("Expected success: true, got %v", resp1.Success)
	}
	if resp1.Message.ID <= 0 || resp1.Message.ConversationID != 1 || resp1.Message.SenderID != 1 || resp1.Message.Body != "Hello direct friend" || resp1.Message.ClientMessageID != "client-direct-1" {
		t.Errorf("Unexpected message in response: %+v", resp1.Message)
	}

	// 3. JSON request to group conversation (conv 2: Youth General Meeting) from User 1
	groupPayload := `{"conversation_id": 2, "body": "Hello everyone in youth group", "client_message_id": "client-group-1"}`
	req2, rr2 := makeAuthReq(http.MethodPost, "/messages/send", groupPayload, "application/json", 1)
	SendMessageAPI(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for group message SendMessageAPI, got %d (body: %s)", rr2.Code, rr2.Body.String())
	}
	var resp2 struct {
		Success bool `json:"success"`
		Message struct {
			ID              int    `json:"ID"`
			ConversationID  int    `json:"ConversationID"`
			SenderID        int    `json:"SenderID"`
			Body            string `json:"Body"`
			ClientMessageID string `json:"ClientMessageID"`
		} `json:"message"`
	}
	if err := json.Unmarshal(rr2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("Failed to decode group JSON response: %v", err)
	}
	if !resp2.Success {
		t.Errorf("Expected success: true for group message, got %v", resp2.Success)
	}
	if resp2.Message.ConversationID != 2 || resp2.Message.Body != "Hello everyone in youth group" {
		t.Errorf("Unexpected group message response: %+v", resp2.Message)
	}

	// 4. User 3 (not a member of conv 1 or conv 2) attempts to send a message -> should return 403 Forbidden
	req3, rr3 := makeAuthReq(http.MethodPost, "/messages/send", jsonPayload, "application/json", 3)
	SendMessageAPI(rr3, req3)
	if rr3.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for non-member SendMessageAPI, got %d (body: %s)", rr3.Code, rr3.Body.String())
	}

	// 5. Empty body -> should return 400 Bad Request
	emptyPayload := `{"conversation_id": 1, "body": "   "}`
	req4, rr4 := makeAuthReq(http.MethodPost, "/messages/send", emptyPayload, "application/json", 1)
	SendMessageAPI(rr4, req4)
	if rr4.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for empty body, got %d", rr4.Code)
	}

	// 6. Standard browser form POST (Accept: text/html) -> should redirect (303 See Other) to /messages?c=1
	formBody := "conversation_id=1&body=Standard+Form+Post+Message&client_message_id=form-msg-1"
	formReq := httptest.NewRequest(http.MethodPost, "/messages/send", strings.NewReader(formBody))
	formReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	formReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	initReq := httptest.NewRequest(http.MethodPost, "/messages/send", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		formReq.AddCookie(cookie)
	}

	formRR := httptest.NewRecorder()
	SendMessageAPI(formRR, formReq)

	if formRR.Code != http.StatusSeeOther {
		t.Errorf("Expected 303 See Other for standard HTML form submission, got %d (body: %s)", formRR.Code, formRR.Body.String())
	}
	if loc := formRR.Header().Get("Location"); loc != "/messages?c=1" {
		t.Errorf("Expected redirect to /messages?c=1, got %q", loc)
	}
}

func TestAddGroupMembersAPI_Comprehensive(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Setup a group owned by user 1 with member user 2
	createJSON := `{"title": "Youth Bible Study", "category": "bible_study", "is_public": true, "member_ids": [2]}`
	createReq, createRR := makeAuthReq(http.MethodPost, "/messages/group/create", createJSON, "application/json", 1)
	CreateGroupAPI(createRR, createReq)
	if createRR.Code != http.StatusOK {
		t.Fatalf("CreateGroupAPI failed: %d", createRR.Code)
	}
	var createResp struct {
		ConversationID int `json:"conversation_id"`
	}
	_ = json.Unmarshal(createRR.Body.Bytes(), &createResp)
	groupID := createResp.ConversationID

	// 2. Unauthenticated request to AddGroupMembersAPI
	unauthReq := httptest.NewRequest(http.MethodPost, "/messages/group/members/add", strings.NewReader(`{"group_id": 1, "user_ids": [3]}`))
	unauthReq.Header.Set("Content-Type", "application/json")
	unauthRR := httptest.NewRecorder()
	AddGroupMembersAPI(unauthRR, unauthReq)
	if unauthRR.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for unauthenticated request, got %d", unauthRR.Code)
	}

	// 3. User 3 (not in group) attempts to add user 1 -> should fail with 403 Forbidden
	nonMemberJSON := fmt.Sprintf(`{"group_id": %d, "user_ids": [1]}`, groupID)
	nonMemberReq, nonMemberRR := makeAuthReq(http.MethodPost, "/messages/group/members/add", nonMemberJSON, "application/json", 3)
	AddGroupMembersAPI(nonMemberRR, nonMemberReq)
	if nonMemberRR.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for non-admin, got %d (body: %s)", nonMemberRR.Code, nonMemberRR.Body.String())
	}

	// 4. Invalid parameters (empty user_ids) with AJAX headers -> 400 Bad Request with JSON error
	invalidJSON := fmt.Sprintf(`{"group_id": %d, "user_ids": []}`, groupID)
	invReq, invRR := makeAuthReq(http.MethodPost, "/messages/group/members/add", invalidJSON, "application/json", 1)
	AddGroupMembersAPI(invRR, invReq)
	if invRR.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for empty user_ids, got %d", invRR.Code)
	}
	var errResp map[string]any
	if err := json.Unmarshal(invRR.Body.Bytes(), &errResp); err != nil || errResp["success"] != false {
		t.Errorf("Expected JSON error response for invalid parameters, got %s", invRR.Body.String())
	}

	// 5. Successful AJAX JSON submission (Owner user 1 adds user 3)
	addJSON := fmt.Sprintf(`{"group_id": %d, "user_ids": [3]}`, groupID)
	addReq, addRR := makeAuthReq(http.MethodPost, "/messages/group/members/add", addJSON, "application/json", 1)
	AddGroupMembersAPI(addRR, addReq)
	if addRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for AJAX add members, got %d (body: %s)", addRR.Code, addRR.Body.String())
	}
	var successResp map[string]any
	if err := json.Unmarshal(addRR.Body.Bytes(), &successResp); err != nil || successResp["success"] != true {
		t.Errorf("Expected success JSON response, got %s", addRR.Body.String())
	}

	// 6. Multiple checkboxes form submission with AJAX (X-Requested-With)
	_, _ = database.DB.Exec("INSERT INTO users (id, full_name, username, password_hash) VALUES (4, 'User Four', 'user4', 'hash4')")
	formData := fmt.Sprintf("group_id=%d&user_ids=4", groupID)
	formAjaxReq := httptest.NewRequest(http.MethodPost, "/messages/group/members/add", strings.NewReader(formData))
	formAjaxReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	formAjaxReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	initReq := httptest.NewRequest(http.MethodPost, "/messages/group/members/add", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		formAjaxReq.AddCookie(cookie)
	}
	formAjaxRR := httptest.NewRecorder()
	AddGroupMembersAPI(formAjaxRR, formAjaxReq)
	if formAjaxRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for form AJAX add members, got %d (body: %s)", formAjaxRR.Code, formAjaxRR.Body.String())
	}
	var formSuccessResp map[string]any
	if err := json.Unmarshal(formAjaxRR.Body.Bytes(), &formSuccessResp); err != nil || formSuccessResp["success"] != true {
		t.Errorf("Expected success JSON for form AJAX, got %s", formAjaxRR.Body.String())
	}

	// 7. Standard browser form POST (Accept: text/html, no AJAX) -> redirect 303 See Other back to /messages?c=%d
	_, _ = database.DB.Exec("INSERT INTO users (id, full_name, username, password_hash) VALUES (5, 'User Five', 'user5', 'hash5')")
	normFormData := fmt.Sprintf("group_id=%d&user_ids=5", groupID)
	normReq := httptest.NewRequest(http.MethodPost, "/messages/group/members/add", strings.NewReader(normFormData))
	normReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	normReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	for _, cookie := range saveRec.Result().Cookies() {
		normReq.AddCookie(cookie)
	}
	normRR := httptest.NewRecorder()
	AddGroupMembersAPI(normRR, normReq)
	if normRR.Code != http.StatusSeeOther {
		t.Errorf("Expected 303 See Other for standard form POST, got %d (body: %s)", normRR.Code, normRR.Body.String())
	}
	expectedLoc := fmt.Sprintf("/messages?c=%d", groupID)
	if loc := normRR.Header().Get("Location"); loc != expectedLoc {
		t.Errorf("Expected redirect to %s, got %s", expectedLoc, loc)
	}
}

func TestAddGroupMembersAPI_ManualMember(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Setup a group owned by user 1 with member user 2
	createJSON := `{"title": "Youth Manual Group", "category": "fellowship", "is_public": true, "member_ids": [2]}`
	createReq, createRR := makeAuthReq(http.MethodPost, "/messages/group/create", createJSON, "application/json", 1)
	CreateGroupAPI(createRR, createReq)
	if createRR.Code != http.StatusOK {
		t.Fatalf("CreateGroupAPI failed: %d", createRR.Code)
	}
	var createResp struct {
		Success        bool `json:"success"`
		ConversationID int  `json:"conversation_id"`
	}
	_ = json.Unmarshal(createRR.Body.Bytes(), &createResp)
	groupID := createResp.ConversationID

	// 2. Add member manually by username with '@' prefix (Owner user 1 adds @user3)
	manualJSON := fmt.Sprintf(`{"group_id": %d, "manual_member": "@user3"}`, groupID)
	addReq, addRR := makeAuthReq(http.MethodPost, "/messages/group/members/add", manualJSON, "application/json", 1)
	AddGroupMembersAPI(addRR, addReq)
	if addRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for manual member add by username, got %d (body: %s)", addRR.Code, addRR.Body.String())
	}
	var successResp struct {
		Success    bool             `json:"success"`
		AddedUsers []map[string]any `json:"added_users"`
	}
	if err := json.Unmarshal(addRR.Body.Bytes(), &successResp); err != nil || !successResp.Success {
		t.Fatalf("Unexpected JSON response for manual member addition: %s", addRR.Body.String())
	}
	if len(successResp.AddedUsers) == 0 {
		t.Fatalf("Expected AddedUsers to contain user3, got empty")
	}

	// 3. Attempting to add an already-existing member manually should return a 400 error
	alreadyMemberJSON := fmt.Sprintf(`{"group_id": %d, "manual_member": "user2"}`, groupID)
	alreadyReq, alreadyRR := makeAuthReq(http.MethodPost, "/messages/group/members/add", alreadyMemberJSON, "application/json", 1)
	AddGroupMembersAPI(alreadyRR, alreadyReq)
	if alreadyRR.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request when adding existing member manually, got %d", alreadyRR.Code)
	}

	// 4. Attempting to add a nonexistent user manually should return a 400 error
	notFoundJSON := fmt.Sprintf(`{"group_id": %d, "manual_member": "unknown_user_999"}`, groupID)
	notFoundReq, notFoundRR := makeAuthReq(http.MethodPost, "/messages/group/members/add", notFoundJSON, "application/json", 1)
	AddGroupMembersAPI(notFoundRR, notFoundReq)
	if notFoundRR.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for nonexistent user, got %d", notFoundRR.Code)
	}
}

func TestUpdateGroupSettingsAPI_Comprehensive(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Setup a group owned by user 1 with member user 2
	createJSON := `{"title": "Original Fellowship", "category": "general", "is_public": true, "member_ids": [2]}`
	createReq, createRR := makeAuthReq(http.MethodPost, "/messages/group/create", createJSON, "application/json", 1)
	CreateGroupAPI(createRR, createReq)
	if createRR.Code != http.StatusOK {
		t.Fatalf("CreateGroupAPI failed: %d", createRR.Code)
	}
	var createResp struct {
		ConversationID int `json:"conversation_id"`
	}
	_ = json.Unmarshal(createRR.Body.Bytes(), &createResp)
	groupID := createResp.ConversationID

	// 2. Unauthenticated request -> 401 Unauthorized
	unauthReq := httptest.NewRequest(http.MethodPost, "/messages/group/update", strings.NewReader(`{"group_id": 1, "title": "New"}`))
	unauthReq.Header.Set("Content-Type", "application/json")
	unauthRR := httptest.NewRecorder()
	UpdateGroupSettingsAPI(unauthRR, unauthReq)
	if unauthRR.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for unauthenticated update, got %d", unauthRR.Code)
	}

	// 3. Normal member (user 2) attempts to update settings -> 403 Forbidden
	memberJSON := fmt.Sprintf(`{"group_id": %d, "title": "Hacked Title", "category": "prayer"}`, groupID)
	memberReq, memberRR := makeAuthReq(http.MethodPost, "/messages/group/update", memberJSON, "application/json", 2)
	UpdateGroupSettingsAPI(memberRR, memberReq)
	if memberRR.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for non-admin update, got %d (body: %s)", memberRR.Code, memberRR.Body.String())
	}

	// 4. Invalid parameters (empty or short title) with AJAX -> 400 Bad Request
	invJSON := fmt.Sprintf(`{"group_id": %d, "title": "A"}`, groupID)
	invReq, invRR := makeAuthReq(http.MethodPost, "/messages/group/update", invJSON, "application/json", 1)
	UpdateGroupSettingsAPI(invRR, invReq)
	if invRR.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for short title, got %d", invRR.Code)
	}
	var errResp map[string]any
	if err := json.Unmarshal(invRR.Body.Bytes(), &errResp); err != nil || errResp["success"] != false {
		t.Errorf("Expected JSON error response for invalid title, got %s", invRR.Body.String())
	}

	// 5. Successful AJAX JSON update (Owner user 1 updates settings)
	updateJSON := fmt.Sprintf(`{"group_id": %d, "title": "Youth Bible Study Fellowship", "category": "bible_study", "description": "Weekly group study", "call_permission": "admin_only"}`, groupID)
	updateReq, updateRR := makeAuthReq(http.MethodPost, "/messages/group/update", updateJSON, "application/json", 1)
	UpdateGroupSettingsAPI(updateRR, updateReq)
	if updateRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for AJAX group update, got %d (body: %s)", updateRR.Code, updateRR.Body.String())
	}
	var successResp map[string]any
	if err := json.Unmarshal(updateRR.Body.Bytes(), &successResp); err != nil || successResp["success"] != true {
		t.Fatalf("Expected success JSON response, got %s", updateRR.Body.String())
	}
	if successResp["title"] != "Youth Bible Study Fellowship" || successResp["category"] != "bible_study" {
		t.Errorf("Unexpected returned fields in success response: %v", successResp)
	}

	// 6. Standard browser form POST (Accept: text/html, non-AJAX) -> redirect 303 See Other back to /messages?c=%d
	formBody := fmt.Sprintf("group_id=%d&title=Updated+Via+Form&category=prayer&description=Desc&call_permission=all", groupID)
	normReq := httptest.NewRequest(http.MethodPost, "/messages/group/update", strings.NewReader(formBody))
	normReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	normReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	initReq := httptest.NewRequest(http.MethodPost, "/messages/group/update", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		normReq.AddCookie(cookie)
	}
	normRR := httptest.NewRecorder()
	UpdateGroupSettingsAPI(normRR, normReq)
	if normRR.Code != http.StatusSeeOther {
		t.Errorf("Expected 303 See Other for standard form POST, got %d (body: %s)", normRR.Code, normRR.Body.String())
	}
	expectedLoc := fmt.Sprintf("/messages?c=%d", groupID)
	if loc := normRR.Header().Get("Location"); loc != expectedLoc {
		t.Errorf("Expected redirect to %s, got %s", expectedLoc, loc)
	}
}

func TestChatTemplate_RenderWithAttachments(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Create a group conversation
	res, err := database.DB.Exec(`
		INSERT INTO conversations (type, title, category, description, call_permission, is_public, created_by)
		VALUES ('group', 'Bible Study Fellowship', 'bible_study', 'Study group', 'all', 1, 1)
	`)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}
	convID, _ := res.LastInsertId()

	_, err = database.DB.Exec(`
		INSERT INTO conversation_members (conversation_id, user_id, role)
		VALUES (?, 1, 'owner'), (?, 2, 'member')
	`, convID, convID)
	if err != nil {
		t.Fatalf("Failed to add members: %v", err)
	}

	// Insert messages with various attachment types
	_, err = database.DB.Exec(`
		INSERT INTO messages (conversation_id, sender_id, message_type, body, attachment_url, attachment_name, attachment_size, attachment_type)
		VALUES
			(?, 1, 'image', 'Here is the event banner', '/uploads/chat/banner.png', 'banner.png', 1572864, 'image/png'),
			(?, 2, 'voice', '', '/uploads/chat/voice_note.webm', 'voice_note.webm', 24576, 'audio/webm'),
			(?, 1, 'file', 'Review this study guide', '/uploads/chat/study_guide.pdf', 'study_guide.pdf', 3145728, 'application/pdf'),
			(?, 2, 'file', 'Meeting notes', '/uploads/chat/notes.docx', 'notes.docx', 512000, 'application/vnd.openxmlformats-officedocument.wordprocessingml.document')
	`, convID, convID, convID, convID)
	if err != nil {
		t.Fatalf("Failed to insert messages: %v", err)
	}

	// Request /messages?c={convID} as User 1 (Owner)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/messages?c=%d", convID), nil)
	initReq := httptest.NewRequest(http.MethodGet, "/messages", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		req.AddCookie(cookie)
	}

	rr := httptest.NewRecorder()
	ChatInbox(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from ChatInbox, got %d", rr.Code)
	}

	body := rr.Body.String()

	// 1. Check spacious message input
	if !strings.Contains(body, "chat-composer-input") {
		t.Errorf("Expected chat-composer-input class in rendered HTML")
	}
	if !strings.Contains(body, "Type a message or attach a file...") {
		t.Errorf("Expected enlarged message placeholder in rendered HTML")
	}

	// 2. Check three-dots dropdown action links
	expectedDropdownActions := []string{
		"View Group Details",
		"Search Messages",
		"Shared Media & Files",
		"Add Members",
		"Edit Group Settings",
		"Start Audio Call",
		"Start Video Call",
		"Mute Notifications",
		"Archive Conversation",
		"Leave Group",
	}
	for _, action := range expectedDropdownActions {
		if !strings.Contains(body, action) {
			t.Errorf("Expected dropdown action link %q in rendered HTML", action)
		}
	}

	// 3. Check attachment cards and human-readable sizes
	if !strings.Contains(body, "1.5 MB") {
		t.Errorf("Expected 1.5 MB formatted size for image attachment")
	}
	if !strings.Contains(body, "24.0 KB") {
		t.Errorf("Expected 24.0 KB formatted size for voice note")
	}
	if !strings.Contains(body, "3.0 MB") {
		t.Errorf("Expected 3.0 MB formatted size for PDF attachment")
	}
	if !strings.Contains(body, "500.0 KB") {
		t.Errorf("Expected 500.0 KB formatted size for docx attachment")
	}
	if !strings.Contains(body, "bi-file-earmark-pdf-fill") {
		t.Errorf("Expected PDF icon class in rendered HTML")
	}
	if !strings.Contains(body, "bi-file-earmark-word-fill") {
		t.Errorf("Expected Word icon class in rendered HTML")
	}
}

func TestChatTemplate_RenderDirectChatWithDropdownActions(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Create direct conversation between user 1 and user 2
	res, err := database.DB.Exec(`
		INSERT INTO conversations (type, title, category, call_permission, is_public, created_by)
		VALUES ('direct', '', 'general', 'all', 0, 1)
	`)
	if err != nil {
		t.Fatalf("Failed to create direct conv: %v", err)
	}
	convID, _ := res.LastInsertId()

	_, err = database.DB.Exec(`
		INSERT INTO conversation_members (conversation_id, user_id, role)
		VALUES (?, 1, 'member'), (?, 2, 'member')
	`, convID, convID)
	if err != nil {
		t.Fatalf("Failed to add members: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/messages?c=%d", convID), nil)
	initReq := httptest.NewRequest(http.MethodGet, "/messages", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		req.AddCookie(cookie)
	}

	rr := httptest.NewRecorder()
	ChatInbox(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from ChatInbox for direct chat, got %d", rr.Code)
	}

	body := rr.Body.String()
	expectedActions := []string{
		"View Contact Details",
		"View Full Profile",
		"Search Messages",
		"Shared Media & Files",
		"Voice Call",
		"Video Call",
		"Mute Notifications",
		"Archive Conversation",
		"Block Contact",
		"Report Abuse",
	}
	for _, action := range expectedActions {
		if !strings.Contains(body, action) {
			t.Errorf("Expected direct chat dropdown action %q in rendered HTML", action)
		}
	}
}

func TestMembershipRequestsAndModerationAPIs(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Create a group (User 1 is owner, approval required by default)
	createJSON := `{"title": "Leadership Group", "category": "general", "description": "Leaders only", "call_permission": "admin_only", "is_public": true}`
	createReq, createRR := makeAuthReq(http.MethodPost, "/messages/groups/create", createJSON, "application/json", 1)
	CreateGroupAPI(createRR, createReq)
	if createRR.Code != http.StatusOK {
		t.Fatalf("CreateGroupAPI failed: %d", createRR.Code)
	}

	var createResp struct {
		ConversationID int `json:"conversation_id"`
	}
	_ = json.Unmarshal(createRR.Body.Bytes(), &createResp)
	groupID := createResp.ConversationID

	// 2. User 2 submits membership request
	reqJSON := fmt.Sprintf(`{"group_id": %d, "message": "I would like to join"}`, groupID)
	reqReq, reqRR := makeAuthReq(http.MethodPost, "/messages/groups/requests/submit", reqJSON, "application/json", 2)
	SubmitGroupMembershipRequestAPI(reqRR, reqReq)
	if reqRR.Code != http.StatusOK {
		t.Fatalf("SubmitGroupMembershipRequestAPI failed: %d (body: %s)", reqRR.Code, reqRR.Body.String())
	}

	// 3. User 1 (owner) views pending requests
	getReq, getRR := makeAuthReq(http.MethodGet, fmt.Sprintf("/messages/groups/requests?group_id=%d", groupID), "", "", 1)
	GetGroupMembershipRequestsAPI(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("GetGroupMembershipRequestsAPI failed: %d (body: %s)", getRR.Code, getRR.Body.String())
	}

	var getResp struct {
		Success  bool `json:"success"`
		Requests []struct {
			ID int `json:"id"`
		} `json:"requests"`
	}
	_ = json.Unmarshal(getRR.Body.Bytes(), &getResp)
	if len(getResp.Requests) == 0 {
		t.Fatalf("Expected at least 1 pending request, got 0")
	}
	requestID := getResp.Requests[0].ID

	// 4. User 1 approves User 2's request
	revJSON := fmt.Sprintf(`{"request_id": %d, "action": "approve"}`, requestID)
	revReq, revRR := makeAuthReq(http.MethodPost, "/messages/groups/requests/review", revJSON, "application/json", 1)
	ReviewGroupMembershipRequestAPI(revRR, revReq)
	if revRR.Code != http.StatusOK {
		t.Fatalf("ReviewGroupMembershipRequestAPI approve failed: %d (body: %s)", revRR.Code, revRR.Body.String())
	}

	// 5. User 3 submits and cancels request
	req3JSON := fmt.Sprintf(`{"group_id": %d, "message": "User 3 request"}`, groupID)
	req3Req, req3RR := makeAuthReq(http.MethodPost, "/messages/groups/requests/submit", req3JSON, "application/json", 3)
	SubmitGroupMembershipRequestAPI(req3RR, req3Req)
	if req3RR.Code != http.StatusOK {
		t.Fatalf("SubmitGroupMembershipRequestAPI user 3 failed: %d", req3RR.Code)
	}

	cancelJSON := fmt.Sprintf(`{"group_id": %d}`, groupID)
	cancelReq, cancelRR := makeAuthReq(http.MethodPost, "/messages/groups/requests/cancel", cancelJSON, "application/json", 3)
	CancelGroupMembershipRequestAPI(cancelRR, cancelReq)
	if cancelRR.Code != http.StatusOK {
		t.Fatalf("CancelGroupMembershipRequestAPI failed: %d (body: %s)", cancelRR.Code, cancelRR.Body.String())
	}

	// 6. User 2 (now member) sends a message
	sendJSON := fmt.Sprintf(`{"conversation_id": %d, "body": "Hello leaders!"}`, groupID)
	sendReq, sendRR := makeAuthReq(http.MethodPost, "/messages/send", sendJSON, "application/json", 2)
	SendMessageAPI(sendRR, sendReq)
	if sendRR.Code != http.StatusOK {
		t.Fatalf("SendMessageAPI failed: %d (body: %s)", sendRR.Code, sendRR.Body.String())
	}

	var sendResp struct {
		Message struct {
			ID int `json:"id"`
		} `json:"message"`
	}
	_ = json.Unmarshal(sendRR.Body.Bytes(), &sendResp)
	msgID := sendResp.Message.ID

	// Outsider User 3 cannot delete User 2's message
	del3JSON := fmt.Sprintf(`{"message_id": %d}`, msgID)
	del3Req, del3RR := makeAuthReq(http.MethodPost, "/messages/message/delete", del3JSON, "application/json", 3)
	DeleteMessageAPI(del3RR, del3Req)
	if del3RR.Code != http.StatusForbidden && del3RR.Code != http.StatusBadRequest {
		t.Errorf("Expected 403 or 400 for unauthorized deletion, got %d", del3RR.Code)
	}

	// User 1 (group owner) moderates/deletes User 2's message
	del1JSON := fmt.Sprintf(`{"message_id": %d}`, msgID)
	del1Req, del1RR := makeAuthReq(http.MethodPost, "/messages/message/delete", del1JSON, "application/json", 1)
	DeleteMessageAPI(del1RR, del1Req)
	if del1RR.Code != http.StatusOK {
		t.Fatalf("DeleteMessageAPI by owner failed: %d (body: %s)", del1RR.Code, del1RR.Body.String())
	}
}

func TestConvertDirectToGroupAPI(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Insert additional test users
	_, _ = database.DB.Exec(`
		INSERT INTO users (id, full_name, username, password_hash)
		VALUES 
			(4, 'Fourth Member', 'fourth_member', 'hash4'),
			(5, 'Fifth Member', 'fifth_member', 'hash5');
	`)

	// 1. Create a direct chat between user 1 and user 2
	conv, err := services.StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("Failed to create direct conversation: %v", err)
	}

	// 2. Unauthenticated request must return 401
	unauthReq := httptest.NewRequest(http.MethodPost, "/messages/direct/add-members", strings.NewReader(`{"conversation_id":1,"user_ids":[3]}`))
	unauthReq.Header.Set("Content-Type", "application/json")
	unauthReq.Header.Set("Accept", "application/json")
	unauthRR := httptest.NewRecorder()
	ConvertDirectToGroupAPI(unauthRR, unauthReq)
	if unauthRR.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for unauthenticated request, got %d", unauthRR.Code)
	}

	// 3. Outsider User 3 trying to convert User 1 & 2's direct chat must be rejected (403/400)
	outsiderJSON := fmt.Sprintf(`{"conversation_id": %d, "user_ids": [4]}`, conv.ID)
	outReq, outRR := makeAuthReq(http.MethodPost, "/messages/direct/add-members", outsiderJSON, "application/json", 3)
	ConvertDirectToGroupAPI(outRR, outReq)
	if outRR.Code != http.StatusForbidden && outRR.Code != http.StatusBadRequest {
		t.Errorf("Expected 403 or 400 when non-participant tries to convert conversation, got %d", outRR.Code)
	}

	// 4. Invalid input (no users selected) must fail
	emptyJSON := fmt.Sprintf(`{"conversation_id": %d, "user_ids": []}`, conv.ID)
	emptyReq, emptyRR := makeAuthReq(http.MethodPost, "/messages/direct/add-members", emptyJSON, "application/json", 1)
	ConvertDirectToGroupAPI(emptyRR, emptyReq)
	if emptyRR.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for empty user selection, got %d", emptyRR.Code)
	}

	// 5. Valid conversion: User 1 adds User 3 with a custom group title
	validJSON := fmt.Sprintf(`{"conversation_id": %d, "user_ids": [3], "group_title": "Trinity Fellowship"}`, conv.ID)
	valReq, valRR := makeAuthReq(http.MethodPost, "/messages/direct/add-members", validJSON, "application/json", 1)
	ConvertDirectToGroupAPI(valRR, valReq)
	if valRR.Code != http.StatusOK {
		t.Fatalf("ConvertDirectToGroupAPI failed: %d (body: %s)", valRR.Code, valRR.Body.String())
	}

	var valResp struct {
		Success        bool   `json:"success"`
		ConversationID int    `json:"conversation_id"`
		RedirectURL    string `json:"redirect_url"`
	}
	if err := json.Unmarshal(valRR.Body.Bytes(), &valResp); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}
	if !valResp.Success || valResp.ConversationID != conv.ID {
		t.Errorf("Expected success true and matching conv ID %d, got %+v", conv.ID, valResp)
	}

	// 6. Verify converted state in repository
	updatedConv, err := repository.GetConversationByID(conv.ID)
	if err != nil {
		t.Fatalf("Failed to fetch updated conversation: %v", err)
	}
	if updatedConv.Type != "group" {
		t.Errorf("Expected type 'group', got %s", updatedConv.Type)
	}
	if updatedConv.Title != "Trinity Fellowship" {
		t.Errorf("Expected title 'Trinity Fellowship', got %s", updatedConv.Title)
	}

	// 7. Verify member roles in repository
	m1, _ := repository.GetConversationMember(conv.ID, 1)
	if m1 == nil || m1.Role != "owner" {
		t.Errorf("Expected User 1 to have role 'owner', got %+v", m1)
	}
	m3, _ := repository.GetConversationMember(conv.ID, 3)
	if m3 == nil || m3.Role != "member" {
		t.Errorf("Expected User 3 to have role 'member', got %+v", m3)
	}

	// 8. Test adding another member via form-urlencoded with manual username
	formBody := fmt.Sprintf("conversation_id=%d&manual_member=@fourth_member", conv.ID)
	formReq, formRR := makeAuthReq(http.MethodPost, "/messages/direct/add-members", formBody, "application/x-www-form-urlencoded", 1)
	ConvertDirectToGroupAPI(formRR, formReq)
	if formRR.Code != http.StatusOK {
		t.Fatalf("Form-based manual member addition failed: %d (body: %s)", formRR.Code, formRR.Body.String())
	}

	m4, _ := repository.GetConversationMember(conv.ID, 4)
	if m4 == nil || m4.Role != "member" {
		t.Errorf("Expected User 4 to have been added as 'member', got %+v", m4)
	}
}

func TestCallInvitationHandlersAndTemplateRendering(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Create a direct conversation
	conv, err := services.StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	// 2. GenerateCallLinkAPI (POST /messages/call/link/generate)
	genBody := fmt.Sprintf(`{"conversation_id": %d, "call_type": "audio"}`, conv.ID)
	genReq, genRR := makeAuthReq(http.MethodPost, "/messages/call/link/generate", genBody, "application/json", 1)
	GenerateCallLinkAPI(genRR, genReq)

	if genRR.Code != http.StatusOK {
		t.Fatalf("GenerateCallLinkAPI failed with status %d: %s", genRR.Code, genRR.Body.String())
	}

	var genResp struct {
		Success   bool   `json:"success"`
		Token     string `json:"token"`
		CallType  string `json:"call_type"`
		InviteURL string `json:"invite_url"`
		Path      string `json:"path"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal(genRR.Body.Bytes(), &genResp); err != nil {
		t.Fatalf("Failed to parse GenerateCallLinkAPI response: %v", err)
	}
	if !genResp.Success || genResp.Token == "" {
		t.Fatalf("Expected successful generation with non-empty token, got %+v", genResp)
	}
	if genResp.CallType != "audio" {
		t.Errorf("Expected call_type 'audio', got %q", genResp.CallType)
	}
	if !strings.Contains(genResp.InviteURL, "/messages/call/join?token="+genResp.Token) {
		t.Errorf("Expected InviteURL to contain join link, got %q", genResp.InviteURL)
	}

	// 3. JoinCallLinkHandler (GET /messages/call/join?token=...)
	// Valid join by participant (User 2)
	joinReq, joinRR := makeAuthReq(http.MethodGet, "/messages/call/join?token="+genResp.Token, "", "", 2)
	JoinCallLinkHandler(joinRR, joinReq)

	if joinRR.Code != http.StatusSeeOther {
		t.Fatalf("Expected 303 See Other redirect on valid join, got %d", joinRR.Code)
	}
	expectedRedirect := fmt.Sprintf("/messages?c=%d&autocall=audio", conv.ID)
	if joinRR.Header().Get("Location") != expectedRedirect {
		t.Errorf("Expected redirect Location %q, got %q", expectedRedirect, joinRR.Header().Get("Location"))
	}

	// Unauthorized join by outsider (User 3)
	outReq, outRR := makeAuthReq(http.MethodGet, "/messages/call/join?token="+genResp.Token, "", "", 3)
	JoinCallLinkHandler(outRR, outReq)
	if outRR.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for non-member call join, got %d", outRR.Code)
	}

	// Join with missing token
	noTokenReq, noTokenRR := makeAuthReq(http.MethodGet, "/messages/call/join", "", "", 2)
	JoinCallLinkHandler(noTokenRR, noTokenReq)
	if noTokenRR.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for missing token, got %d", noTokenRR.Code)
	}

	// 4. Verify template rendering of Call Dropdown, Modals, and Guidance
	inboxReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/messages?c=%d", conv.ID), nil)
	initReq := httptest.NewRequest(http.MethodGet, "/messages", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		inboxReq.AddCookie(cookie)
	}

	inboxRR := httptest.NewRecorder()
	ChatInbox(inboxRR, inboxReq)

	if inboxRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from ChatInbox, got %d", inboxRR.Code)
	}

	html := inboxRR.Body.String()
	requiredElements := []string{
		`id="chatCallDropdownBtn"`,
		`id="startAudioCallBtn"`,
		`id="startVideoCallBtn"`,
		`id="sendCallLinkModal"`,
		`id="callInvitationLinkInput"`,
		`id="copyCallInvitationLinkBtn"`,
		`id="sendCallLinkToChatBtn"`,
		`id="mediaPermissionModal"`,
		`id="mediaPermissionRetryBtn"`,
		`Android Chrome / Mobile Steps`,
		`Lock / Page Info icon`,
		`Microphone Access Blocked`,
	}

	for _, elem := range requiredElements {
		if !strings.Contains(html, elem) {
			t.Errorf("Expected HTML to contain %q, but was missing", elem)
		}
	}
}

func TestSearchMessagesAPI_ScopingAndPermissions(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Unauthenticated request should be 401
	unauthReq := httptest.NewRequest(http.MethodGet, "/messages/search?q=test", nil)
	unauthRR := httptest.NewRecorder()
	SearchMessagesAPI(unauthRR, unauthReq)
	if unauthRR.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for unauthenticated search, got %d", unauthRR.Code)
	}

	// Create conversation for user 1 & user 2
	conv, err := services.StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	// Seed message
	_, err = database.DB.Exec(`
		INSERT INTO messages (id, conversation_id, sender_id, message_type, body, created_at, updated_at, is_deleted)
		VALUES (501, ?, 1, 'text', 'Searchable fellowship topic', datetime('now'), datetime('now'), 0);
	`, conv.ID)
	if err != nil {
		t.Fatalf("Failed to seed message: %v", err)
	}

	// Helper to create authenticated session
	makeAuthReq := func(userID int, urlStr string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, urlStr, nil)
		rec := httptest.NewRecorder()
		session, _ := sessions.Store.Get(req, "youth-community")
		session.Values["user_id"] = userID
		_ = session.Save(req, rec)
		for _, cookie := range rec.Result().Cookies() {
			req.AddCookie(cookie)
		}
		return req
	}

	// 2. Invalid conversation_id parameter -> 400
	invReq := makeAuthReq(1, "/messages/search?q=test&conversation_id=invalid")
	invRR := httptest.NewRecorder()
	SearchMessagesAPI(invRR, invReq)
	if invRR.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for non-integer conversation_id, got %d", invRR.Code)
	}

	// 3. User 3 is not a member of conversation -> 403
	forbiddenReq := makeAuthReq(3, fmt.Sprintf("/messages/search?q=fellowship&conversation_id=%d", conv.ID))
	forbiddenRR := httptest.NewRecorder()
	SearchMessagesAPI(forbiddenRR, forbiddenReq)
	if forbiddenRR.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for non-participant search, got %d", forbiddenRR.Code)
	}

	// 4. Authorized user 1 searches scoped conversation -> 200 with metadata
	validReq := makeAuthReq(1, fmt.Sprintf("/messages/search?q=fellowship&conversation_id=%d", conv.ID))
	validRR := httptest.NewRecorder()
	SearchMessagesAPI(validRR, validReq)
	if validRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for scoped search, got %d", validRR.Code)
	}

	var res struct {
		Count          int              `json:"count"`
		Messages       []models.Message `json:"messages"`
		ConversationID int              `json:"conversation_id"`
		Limit          int              `json:"limit"`
		Offset         int              `json:"offset"`
	}
	if err := json.Unmarshal(validRR.Body.Bytes(), &res); err != nil {
		t.Fatalf("Failed to decode search response JSON: %v", err)
	}
	if res.Count != 1 || len(res.Messages) != 1 {
		t.Errorf("Expected 1 result, got count=%d, messages=%d", res.Count, len(res.Messages))
	}
	if res.ConversationID != conv.ID {
		t.Errorf("Expected response conversation_id=%d, got %d", conv.ID, res.ConversationID)
	}

	// 5. Global search (no conversation_id param) -> 200
	globalReq := makeAuthReq(1, "/messages/search?q=fellowship")
	globalRR := httptest.NewRecorder()
	SearchMessagesAPI(globalRR, globalReq)
	if globalRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for global search, got %d", globalRR.Code)
	}
}

func TestGetSharedMediaAPI(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Unauthenticated request -> 401
	unauthReq := httptest.NewRequest(http.MethodGet, "/messages/media?conversation_id=1", nil)
	unauthRR := httptest.NewRecorder()
	GetSharedMediaAPI(unauthRR, unauthReq)
	if unauthRR.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for unauthenticated media request, got %d", unauthRR.Code)
	}

	// Create conversation for user 1 & user 2
	conv, err := services.StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	// Seed media items
	_, err = database.DB.Exec(`
		INSERT INTO messages (id, conversation_id, sender_id, message_type, body, attachment_url, attachment_name, attachment_type, attachment_size, created_at, updated_at)
		VALUES 
			(601, ?, 1, 'image', 'Photo', '/uploads/chat/p1.jpg', 'p1.jpg', 'image/jpeg', 1024, datetime('now'), datetime('now')),
			(602, ?, 2, 'video', 'Video', '/uploads/chat/v1.mp4', 'v1.mp4', 'video/mp4', 2048, datetime('now'), datetime('now'));
	`, conv.ID, conv.ID)
	if err != nil {
		t.Fatalf("Failed to seed media messages: %v", err)
	}

	makeAuthReq := func(userID int, urlStr string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, urlStr, nil)
		rec := httptest.NewRecorder()
		session, _ := sessions.Store.Get(req, "youth-community")
		session.Values["user_id"] = userID
		_ = session.Save(req, rec)
		for _, cookie := range rec.Result().Cookies() {
			req.AddCookie(cookie)
		}
		return req
	}

	// 2. Missing conversation_id parameter -> 400
	missingReq := makeAuthReq(1, "/messages/media")
	missingRR := httptest.NewRecorder()
	GetSharedMediaAPI(missingRR, missingReq)
	if missingRR.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for missing conversation_id, got %d", missingRR.Code)
	}

	// 3. User 3 not a participant -> 403
	forbiddenReq := makeAuthReq(3, fmt.Sprintf("/messages/media?conversation_id=%d", conv.ID))
	forbiddenRR := httptest.NewRecorder()
	GetSharedMediaAPI(forbiddenRR, forbiddenReq)
	if forbiddenRR.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for outsider media request, got %d", forbiddenRR.Code)
	}

	// 4. Authorized user 1 requests photo category -> 200
	photoReq := makeAuthReq(1, fmt.Sprintf("/messages/media?conversation_id=%d&category=photo", conv.ID))
	photoRR := httptest.NewRecorder()
	GetSharedMediaAPI(photoRR, photoReq)
	if photoRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for photo media request, got %d", photoRR.Code)
	}

	var photoRes struct {
		Category string           `json:"category"`
		Count    int              `json:"count"`
		Media    []models.Message `json:"media"`
	}
	if err := json.Unmarshal(photoRR.Body.Bytes(), &photoRes); err != nil {
		t.Fatalf("Failed to decode media JSON: %v", err)
	}
	if photoRes.Count != 1 || len(photoRes.Media) != 1 || photoRes.Media[0].AttachmentName != "p1.jpg" {
		t.Errorf("Expected 1 photo p1.jpg, got %+v", photoRes)
	}

	// 5. Authorized user 1 requests video category -> 200
	videoReq := makeAuthReq(1, fmt.Sprintf("/messages/media?conversation_id=%d&category=video", conv.ID))
	videoRR := httptest.NewRecorder()
	GetSharedMediaAPI(videoRR, videoReq)
	if videoRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for video media request, got %d", videoRR.Code)
	}

	var videoRes struct {
		Category string           `json:"category"`
		Count    int              `json:"count"`
		Media    []models.Message `json:"media"`
	}
	if err := json.Unmarshal(videoRR.Body.Bytes(), &videoRes); err != nil {
		t.Fatalf("Failed to decode video JSON: %v", err)
	}
	if videoRes.Count != 1 || len(videoRes.Media) != 1 || videoRes.Media[0].AttachmentName != "v1.mp4" {
		t.Errorf("Expected 1 video v1.mp4, got %+v", videoRes)
	}
}

func TestTemplateSharedMediaAndSearchElements(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := services.StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	// Seed media
	_, err = database.DB.Exec(`
		INSERT INTO messages (id, conversation_id, sender_id, message_type, body, attachment_url, attachment_name, attachment_type, attachment_size, created_at, updated_at)
		VALUES 
			(701, ?, 1, 'image', 'Photo', '/uploads/chat/church.jpg', 'church.jpg', 'image/jpeg', 1024, datetime('now'), datetime('now')),
			(702, ?, 2, 'video', 'Video', '/uploads/chat/choir.mp4', 'choir.mp4', 'video/mp4', 2048, datetime('now'), datetime('now')),
			(703, ?, 1, 'file', 'Docs', '/uploads/chat/program.pdf', 'program.pdf', 'application/pdf', 4096, datetime('now'), datetime('now')),
			(704, ?, 2, 'voice', 'Voice', '/uploads/chat/amen.webm', 'amen.webm', 'audio/webm', 512, datetime('now'), datetime('now'));
	`, conv.ID, conv.ID, conv.ID, conv.ID)
	if err != nil {
		t.Fatalf("Failed to seed media messages: %v", err)
	}

	// Request chat inbox
	inboxReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/messages?c=%d", conv.ID), nil)
	initReq := httptest.NewRequest(http.MethodGet, "/messages", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		inboxReq.AddCookie(cookie)
	}

	inboxRR := httptest.NewRecorder()
	ChatInbox(inboxRR, inboxReq)

	if inboxRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from ChatInbox, got %d", inboxRR.Code)
	}

	html := inboxRR.Body.String()
	requiredStage2Elements := []string{
		`id="searchScopeGroup"`,
		`id="searchScopeCurrent"`,
		`id="searchScopeAll"`,
		`id="mediaLightboxModal"`,
		`id="mediaLightboxImg"`,
		`id="mediaLightboxDownloadBtn"`,
		`id="sharedMediaTabs"`,
		`id="media-photos-tab"`,
		`id="media-videos-tab"`,
		`id="media-docs-tab"`,
		`id="media-voice-tab"`,
		`openMediaLightbox(`,
		`title="Search Messages"`,
		`chat-attachment-video`,
	}

	for _, elem := range requiredStage2Elements {
		if !strings.Contains(html, elem) {
			t.Errorf("Expected HTML to contain %q, but was missing", elem)
		}
	}
}

func TestReportAbuseAPI_WorkflowAndDuplicateConflict(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Submit valid report via JSON
	reportJSON := `{"reported_user_id": 2, "reason": "Harassment or Bullying", "details": "Unsolicited aggressive messages"}`
	req, rr := makeAuthReq(http.MethodPost, "/messages/report", reportJSON, "application/json", 1)
	ReportAbuseAPI(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from ReportAbuseAPI, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"success":true`) {
		t.Errorf("Expected success:true in response: %s", rr.Body.String())
	}

	// 2. Submit duplicate report while first is pending -> must return 409 Conflict
	dupReq, dupRR := makeAuthReq(http.MethodPost, "/messages/report", reportJSON, "application/json", 1)
	ReportAbuseAPI(dupRR, dupReq)

	if dupRR.Code != http.StatusConflict {
		t.Errorf("Expected 409 Conflict for duplicate pending report, got %d (body: %s)", dupRR.Code, dupRR.Body.String())
	}

	// 3. Submit invalid reported user ID -> 400 Bad Request
	badJSON := `{"reported_user_id": 0, "reason": "Spam"}`
	badReq, badRR := makeAuthReq(http.MethodPost, "/messages/report", badJSON, "application/json", 1)
	ReportAbuseAPI(badRR, badReq)

	if badRR.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for invalid reported user ID, got %d", badRR.Code)
	}
}

func TestGetBlockedUsersAPI(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Block user 2
	_ = services.BlockUser(1, 2, "Spamming")

	req, rr := makeAuthReq(http.MethodGet, "/messages/blocked", "", "", 1)
	GetBlockedUsersAPI(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from GetBlockedUsersAPI, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success      bool          `json:"success"`
		BlockedUsers []models.User `json:"blocked_users"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if !resp.Success || len(resp.BlockedUsers) != 1 || resp.BlockedUsers[0].ID != 2 {
		t.Errorf("Expected blocked_users containing User 2, got: %+v", resp.BlockedUsers)
	}
}

func TestModerationReportsAPIs(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Set user 1 as member, user 2 as admin
	_, _ = database.DB.Exec("UPDATE users SET role = 'member' WHERE id = 1")
	_, _ = database.DB.Exec("UPDATE users SET role = 'admin' WHERE id = 2")

	// Submit a report
	report, err := services.ReportAbuse(1, 3, nil, nil, "Threats or Violence", "Severe threat")
	if err != nil {
		t.Fatalf("ReportAbuse failed: %v", err)
	}

	// 1. Regular user calling GetModerationReportsAPI -> 403 Forbidden
	req1, rr1 := makeAuthReq(http.MethodGet, "/messages/moderation/reports", "", "", 1)
	GetModerationReportsAPI(rr1, req1)
	if rr1.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for non-admin, got %d", rr1.Code)
	}

	// 2. Admin calling GetModerationReportsAPI -> 200 OK
	req2, rr2 := makeAuthReq(http.MethodGet, "/messages/moderation/reports?status=pending", "", "", 2)
	GetModerationReportsAPI(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for admin, got %d (body: %s)", rr2.Code, rr2.Body.String())
	}
	if !strings.Contains(rr2.Body.String(), `"success":true`) {
		t.Errorf("Expected success:true in response: %s", rr2.Body.String())
	}

	// 3. Regular user calling ResolveModerationReportAPI -> 403 Forbidden
	resolveJSON := fmt.Sprintf(`{"report_id": %d, "status": "resolved", "action": "dismiss", "notes": "Done"}`, report.ID)
	req3, rr3 := makeAuthReq(http.MethodPost, "/messages/moderation/reports/resolve", resolveJSON, "application/json", 1)
	ResolveModerationReportAPI(rr3, req3)
	if rr3.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for non-admin resolving report, got %d", rr3.Code)
	}

	// 4. Admin calling ResolveModerationReportAPI -> 200 OK
	req4, rr4 := makeAuthReq(http.MethodPost, "/messages/moderation/reports/resolve", resolveJSON, "application/json", 2)
	ResolveModerationReportAPI(rr4, req4)
	if rr4.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for admin resolving report, got %d (body: %s)", rr4.Code, rr4.Body.String())
	}
}

func TestTemplateStage3ReportAndBlockElements(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Make user 1 an admin
	_, _ = database.DB.Exec("UPDATE users SET role = 'admin' WHERE id = 1")

	conv, err := services.StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	// Add an incoming message from User 2
	_, _ = services.SendMessage(conv.ID, 2, "Incoming fellowship message", "cmsg_in_1", nil)

	// Render ChatInbox
	inboxReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/messages?c=%d", conv.ID), nil)
	initReq := httptest.NewRequest(http.MethodGet, "/messages", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		inboxReq.AddCookie(cookie)
	}

	inboxRR := httptest.NewRecorder()
	ChatInbox(inboxRR, inboxReq)

	if inboxRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from ChatInbox, got %d", inboxRR.Code)
	}

	html := inboxRR.Body.String()
	requiredStage3Elements := []string{
		`id="reportAbuseModal"`,
		`id="blockedUsersModal"`,
		`id="moderationReportsModal"`,
		`id="reportEvidencePreview"`,
		`report-msg-btn`,
		`id="reportAbuseForm"`,
		`id="blockedUsersList"`,
		`id="modReportsList"`,
	}

	for _, elem := range requiredStage3Elements {
		if !strings.Contains(html, elem) {
			t.Errorf("Expected HTML to contain %q, but was missing", elem)
		}
	}
}

func TestToggleMuteDurationAPIs(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := services.StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	// 1. Unauthorized request
	unauthReq := httptest.NewRequest(http.MethodPost, "/messages/mute", strings.NewReader(`{"conversation_id": 1}`))
	unauthRR := httptest.NewRecorder()
	ToggleMuteAPI(unauthRR, unauthReq)
	if unauthRR.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for unauth mute, got %d", unauthRR.Code)
	}

	// 2. Invalid conversation ID
	reqBadID, rrBadID := makeAuthReq(http.MethodPost, "/messages/mute", `{"conversation_id": 0}`, "application/json", 1)
	ToggleMuteAPI(rrBadID, reqBadID)
	if rrBadID.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for invalid conv ID, got %d", rrBadID.Code)
	}

	// 3. Outsider member -> 403 Forbidden
	reqOutsider, rrOutsider := makeAuthReq(http.MethodPost, "/messages/mute", fmt.Sprintf(`{"conversation_id": %d, "duration": "1h"}`, conv.ID), "application/json", 3)
	ToggleMuteAPI(rrOutsider, reqOutsider)
	if rrOutsider.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for outsider muting conv, got %d", rrOutsider.Code)
	}

	// 4. Mute option 1h via JSON
	req1h, rr1h := makeAuthReq(http.MethodPost, "/messages/mute", fmt.Sprintf(`{"conversation_id": %d, "duration": "1h"}`, conv.ID), "application/json", 1)
	ToggleMuteAPI(rr1h, req1h)
	if rr1h.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for mute 1h, got %d (body: %s)", rr1h.Code, rr1h.Body.String())
	}
	if !strings.Contains(rr1h.Body.String(), `"is_muted":true`) {
		t.Errorf("Expected is_muted:true in response, got %s", rr1h.Body.String())
	}

	// 5. Mute option 8h
	req8h, rr8h := makeAuthReq(http.MethodPost, "/messages/mute", fmt.Sprintf(`{"conversation_id": %d, "duration": "8h"}`, conv.ID), "application/json", 1)
	ToggleMuteAPI(rr8h, req8h)
	if rr8h.Code != http.StatusOK || !strings.Contains(rr8h.Body.String(), `"is_muted":true`) {
		t.Errorf("Expected 200 OK with is_muted:true for 8h, got %d: %s", rr8h.Code, rr8h.Body.String())
	}

	// 6. Mute option 1w via Form POST
	formBody := fmt.Sprintf("conversation_id=%d&duration=1w", conv.ID)
	req1w, rr1w := makeAuthReq(http.MethodPost, "/messages/mute", formBody, "application/x-www-form-urlencoded", 1)
	ToggleMuteAPI(rr1w, req1w)
	if rr1w.Code != http.StatusOK || !strings.Contains(rr1w.Body.String(), `"is_muted":true`) {
		t.Errorf("Expected 200 OK for 1w mute, got %d: %s", rr1w.Code, rr1w.Body.String())
	}

	// 7. Indefinite mute
	reqIndef, rrIndef := makeAuthReq(http.MethodPost, "/messages/mute", fmt.Sprintf(`{"conversation_id": %d, "duration": "indefinite"}`, conv.ID), "application/json", 1)
	ToggleMuteAPI(rrIndef, reqIndef)
	if rrIndef.Code != http.StatusOK || !strings.Contains(rrIndef.Body.String(), `"is_muted":true`) {
		t.Errorf("Expected 200 OK for indefinite mute, got %d: %s", rrIndef.Code, rrIndef.Body.String())
	}

	// 8. Unmute
	reqUnmute, rrUnmute := makeAuthReq(http.MethodPost, "/messages/mute", fmt.Sprintf(`{"conversation_id": %d, "duration": "unmute"}`, conv.ID), "application/json", 1)
	ToggleMuteAPI(rrUnmute, reqUnmute)
	if rrUnmute.Code != http.StatusOK || !strings.Contains(rrUnmute.Body.String(), `"is_muted":false`) {
		t.Errorf("Expected 200 OK with is_muted:false for unmute, got %d: %s", rrUnmute.Code, rrUnmute.Body.String())
	}
}

func TestSetDisappearingMessagesAPIs(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Direct conversation
	conv, err := services.StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	// Unauthenticated call
	unauthReq := httptest.NewRequest(http.MethodPost, "/messages/disappearing", nil)
	unauthRR := httptest.NewRecorder()
	SetDisappearingMessagesAPI(unauthRR, unauthReq)
	if unauthRR.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", unauthRR.Code)
	}

	// Invalid conv ID
	reqBadConv, rrBadConv := makeAuthReq(http.MethodPost, "/messages/disappearing", `{"conversation_id": 0}`, "application/json", 1)
	SetDisappearingMessagesAPI(rrBadConv, reqBadConv)
	if rrBadConv.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", rrBadConv.Code)
	}

	// Invalid duration
	reqBadDur, rrBadDur := makeAuthReq(http.MethodPost, "/messages/disappearing", fmt.Sprintf(`{"conversation_id": %d, "duration": 1234}`, conv.ID), "application/json", 1)
	SetDisappearingMessagesAPI(rrBadDur, reqBadDur)
	if rrBadDur.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for invalid duration, got %d", rrBadDur.Code)
	}

	// Valid 24h timer (86400)
	req24h, rr24h := makeAuthReq(http.MethodPost, "/messages/disappearing", fmt.Sprintf(`{"conversation_id": %d, "duration": 86400}`, conv.ID), "application/json", 1)
	SetDisappearingMessagesAPI(rr24h, req24h)
	if rr24h.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for 24h timer, got %d: %s", rr24h.Code, rr24h.Body.String())
	}
	if !strings.Contains(rr24h.Body.String(), `"disappearing_duration":86400`) {
		t.Errorf("Expected disappearing_duration:86400 in response, got %s", rr24h.Body.String())
	}

	// Valid 7d timer string alias ("7d")
	req7d, rr7d := makeAuthReq(http.MethodPost, "/messages/disappearing", fmt.Sprintf(`{"conversation_id": %d, "duration": "7d"}`, conv.ID), "application/json", 2)
	SetDisappearingMessagesAPI(rr7d, req7d)
	if rr7d.Code != http.StatusOK || !strings.Contains(rr7d.Body.String(), `"disappearing_duration":604800`) {
		t.Errorf("Expected 200 OK with 604800 for 7d alias, got %d: %s", rr7d.Code, rr7d.Body.String())
	}

	// 2. Group conversation permissions
	group, err := services.CreateGroup(1, "Youth Bible Group", "bible_study", "Study circle", "all", true, []int{2})
	if err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	err = services.AddGroupMembers(1, group.ID, []int{3})
	if err != nil {
		t.Fatalf("AddGroupMembers failed: %v", err)
	}

	// User 3 (regular member) -> 403 Forbidden
	reqMem, rrMem := makeAuthReq(http.MethodPost, "/messages/disappearing", fmt.Sprintf(`{"conversation_id": %d, "duration": 86400}`, group.ID), "application/json", 3)
	SetDisappearingMessagesAPI(rrMem, reqMem)
	if rrMem.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for regular member setting group timer, got %d: %s", rrMem.Code, rrMem.Body.String())
	}

	// User 1 (group owner) -> 200 OK
	reqOwner, rrOwner := makeAuthReq(http.MethodPost, "/messages/disappearing", fmt.Sprintf(`{"conversation_id": %d, "duration": 86400}`, group.ID), "application/json", 1)
	SetDisappearingMessagesAPI(rrOwner, reqOwner)
	if rrOwner.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for group owner setting timer, got %d: %s", rrOwner.Code, rrOwner.Body.String())
	}

	// Promote User 2 to admin -> 200 OK
	_ = services.UpdateGroupMemberRole(1, group.ID, 2, "admin")
	reqAdmin, rrAdmin := makeAuthReq(http.MethodPost, "/messages/disappearing", fmt.Sprintf(`{"conversation_id": %d, "duration": 0}`, group.ID), "application/json", 2)
	SetDisappearingMessagesAPI(rrAdmin, reqAdmin)
	if rrAdmin.Code != http.StatusOK || !strings.Contains(rrAdmin.Body.String(), `"disappearing_duration":0`) {
		t.Errorf("Expected 200 OK for group admin turning off timer, got %d: %s", rrAdmin.Code, rrAdmin.Body.String())
	}
}

func TestTemplateStage4MuteAndDisappearingElements(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := services.StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	// Enable disappearing messages and mute for user 1
	_, _ = services.SetDisappearingMessages(1, conv.ID, 86400)
	_, _, _ = services.SetConversationMute(conv.ID, 1, "8h")

	// Render ChatInbox for user 1
	inboxReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/messages?c=%d", conv.ID), nil)
	initReq := httptest.NewRequest(http.MethodGet, "/messages", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		inboxReq.AddCookie(cookie)
	}

	inboxRR := httptest.NewRecorder()
	ChatInbox(inboxRR, inboxReq)

	if inboxRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from ChatInbox, got %d", inboxRR.Code)
	}

	htmlDirect := inboxRR.Body.String()
	requiredDirectElements := []string{
		`id="muteConversationModal"`,
		`id="disappearingMessagesModal"`,
		`id="muteConversationForm"`,
		`id="disappearingMessagesForm"`,
		`id="headerMuteContainer"`,
		`id="headerDisappearingBadge"`,
		`id="muteOpt1h"`,
		`id="dispOpt24h"`,
	}

	for _, elem := range requiredDirectElements {
		if !strings.Contains(htmlDirect, elem) {
			t.Errorf("Expected direct HTML to contain %q, but was missing", elem)
		}
	}

	// 2. Test Group Conversation Rendering
	group, err := services.CreateGroup(1, "Stage 4 Group", "bible_study", "Fellowship study", "all", true, []int{2})
	if err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	_, _ = services.SetDisappearingMessages(1, group.ID, 604800)
	_, _, _ = services.SetConversationMute(group.ID, 1, "1w")

	groupReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/messages?c=%d", group.ID), nil)
	for _, cookie := range saveRec.Result().Cookies() {
		groupReq.AddCookie(cookie)
	}
	groupRR := httptest.NewRecorder()
	ChatInbox(groupRR, groupReq)
	if groupRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from ChatInbox for group, got %d", groupRR.Code)
	}

	htmlGroup := groupRR.Body.String()
	requiredGroupElements := []string{
		`id="groupHeaderMuteContainer"`,
		`id="groupHeaderDisappearingBadge"`,
		`id="groupHeaderDisappearingLabel"`,
		`id="muteConversationModal"`,
		`id="disappearingMessagesModal"`,
	}
	for _, elem := range requiredGroupElements {
		if !strings.Contains(htmlGroup, elem) {
			t.Errorf("Expected group HTML to contain %q, but was missing", elem)
		}
	}
}

func TestClearChatAPI(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := services.StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	makeAuthReq := func(userID int, bodyStr string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/messages/clear", strings.NewReader(bodyStr))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		session, _ := sessions.Store.Get(req, "youth-community")
		session.Values["user_id"] = userID
		_ = session.Save(req, rec)
		for _, cookie := range rec.Result().Cookies() {
			req.AddCookie(cookie)
		}
		return req
	}

	// 1. Unauthenticated request -> 401
	unauthReq := httptest.NewRequest(http.MethodPost, "/messages/clear", strings.NewReader(`{"conversation_id": 1}`))
	unauthReq.Header.Set("Content-Type", "application/json")
	unauthRR := httptest.NewRecorder()
	ClearChatAPI(unauthRR, unauthReq)
	if unauthRR.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for unauthenticated request, got %d", unauthRR.Code)
	}

	// 2. Invalid body -> 400
	badReq := makeAuthReq(1, `{"conversation_id": "invalid"}`)
	badRR := httptest.NewRecorder()
	ClearChatAPI(badRR, badReq)
	if badRR.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for bad JSON, got %d", badRR.Code)
	}

	// 3. User 3 (non-member) -> 403
	forbiddenReq := makeAuthReq(3, fmt.Sprintf(`{"conversation_id": %d}`, conv.ID))
	forbiddenRR := httptest.NewRecorder()
	ClearChatAPI(forbiddenRR, forbiddenReq)
	if forbiddenRR.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for non-member clear, got %d", forbiddenRR.Code)
	}

	// 4. User 1 (authorized member) -> 200
	successReq := makeAuthReq(1, fmt.Sprintf(`{"conversation_id": %d}`, conv.ID))
	successRR := httptest.NewRecorder()
	ClearChatAPI(successRR, successReq)
	if successRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for valid clear chat, got %d: %s", successRR.Code, successRR.Body.String())
	}
	var res map[string]interface{}
	_ = json.Unmarshal(successRR.Body.Bytes(), &res)
	if res["success"] != true {
		t.Errorf("Expected success: true, got %v", res)
	}
}

func TestTogglePinAPI(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := services.StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	makeAuthReq := func(userID int, bodyStr string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/messages/pin", strings.NewReader(bodyStr))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		session, _ := sessions.Store.Get(req, "youth-community")
		session.Values["user_id"] = userID
		_ = session.Save(req, rec)
		for _, cookie := range rec.Result().Cookies() {
			req.AddCookie(cookie)
		}
		return req
	}

	// 1. Unauthenticated -> 401
	unauthReq := httptest.NewRequest(http.MethodPost, "/messages/pin", strings.NewReader(`{"conversation_id": 1}`))
	unauthRR := httptest.NewRecorder()
	TogglePinAPI(unauthRR, unauthReq)
	if unauthRR.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for unauthenticated pin toggle, got %d", unauthRR.Code)
	}

	// 2. User 3 non-member -> 403
	forbiddenReq := makeAuthReq(3, fmt.Sprintf(`{"conversation_id": %d}`, conv.ID))
	forbiddenRR := httptest.NewRecorder()
	TogglePinAPI(forbiddenRR, forbiddenReq)
	if forbiddenRR.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for non-member pin toggle, got %d", forbiddenRR.Code)
	}

	// 3. User 1 pins conversation -> is_pinned: true
	pinReq := makeAuthReq(1, fmt.Sprintf(`{"conversation_id": %d}`, conv.ID))
	pinRR := httptest.NewRecorder()
	TogglePinAPI(pinRR, pinReq)
	if pinRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for pin, got %d: %s", pinRR.Code, pinRR.Body.String())
	}
	var res map[string]interface{}
	_ = json.Unmarshal(pinRR.Body.Bytes(), &res)
	if res["is_pinned"] != true {
		t.Errorf("Expected is_pinned: true, got %v", res["is_pinned"])
	}

	// 4. User 1 unpins conversation -> is_pinned: false
	unpinReq := makeAuthReq(1, fmt.Sprintf(`{"conversation_id": %d}`, conv.ID))
	unpinRR := httptest.NewRecorder()
	TogglePinAPI(unpinRR, unpinReq)
	if unpinRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for unpin, got %d", unpinRR.Code)
	}
	var res2 map[string]interface{}
	_ = json.Unmarshal(unpinRR.Body.Bytes(), &res2)
	if res2["is_pinned"] != false {
		t.Errorf("Expected is_pinned: false, got %v", res2["is_pinned"])
	}
}

func TestExportChatAPI(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := services.StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}
	_, _ = services.SendMessage(conv.ID, 1, "Hello from export test", "", nil)

	makeAuthReq := func(userID int, urlStr string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, urlStr, nil)
		rec := httptest.NewRecorder()
		session, _ := sessions.Store.Get(req, "youth-community")
		session.Values["user_id"] = userID
		_ = session.Save(req, rec)
		for _, cookie := range rec.Result().Cookies() {
			req.AddCookie(cookie)
		}
		return req
	}

	// 1. Unauthenticated -> 401
	unauthReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/messages/export?conversation_id=%d", conv.ID), nil)
	unauthRR := httptest.NewRecorder()
	ExportChatAPI(unauthRR, unauthReq)
	if unauthRR.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for unauthenticated export, got %d", unauthRR.Code)
	}

	// 2. User 3 non-member -> 403
	forbiddenReq := makeAuthReq(3, fmt.Sprintf("/messages/export?conversation_id=%d", conv.ID))
	forbiddenRR := httptest.NewRecorder()
	ExportChatAPI(forbiddenRR, forbiddenReq)
	if forbiddenRR.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for non-member export, got %d", forbiddenRR.Code)
	}

	// 3. User 1 export TXT -> 200
	txtReq := makeAuthReq(1, fmt.Sprintf("/messages/export?conversation_id=%d&format=txt", conv.ID))
	txtRR := httptest.NewRecorder()
	ExportChatAPI(txtRR, txtReq)
	if txtRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for TXT export, got %d: %s", txtRR.Code, txtRR.Body.String())
	}
	disp := txtRR.Header().Get("Content-Disposition")
	if !strings.Contains(disp, "attachment") || !strings.Contains(disp, ".txt") {
		t.Errorf("Expected Content-Disposition attachment with .txt, got %s", disp)
	}
	if !strings.Contains(txtRR.Body.String(), "Hello from export test") {
		t.Errorf("Expected export to include message content, got %s", txtRR.Body.String())
	}

	// 4. User 1 export JSON -> 200
	jsonReq := makeAuthReq(1, fmt.Sprintf("/messages/export?conversation_id=%d&format=json", conv.ID))
	jsonRR := httptest.NewRecorder()
	ExportChatAPI(jsonRR, jsonReq)
	if jsonRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for JSON export, got %d: %s", jsonRR.Code, jsonRR.Body.String())
	}
	dispJSON := jsonRR.Header().Get("Content-Disposition")
	if !strings.Contains(dispJSON, "attachment") || !strings.Contains(dispJSON, ".json") {
		t.Errorf("Expected Content-Disposition attachment with .json, got %s", dispJSON)
	}
	var exportData map[string]interface{}
	err = json.Unmarshal(jsonRR.Body.Bytes(), &exportData)
	if err != nil {
		t.Fatalf("Expected valid JSON export payload, got err: %v", err)
	}
	if exportData["messages"] == nil {
		t.Errorf("Expected messages key in JSON export payload")
	}
}

func TestGetConversationMembersAPI(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := services.StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	makeAuthReq := func(userID int, urlStr string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, urlStr, nil)
		rec := httptest.NewRecorder()
		session, _ := sessions.Store.Get(req, "youth-community")
		session.Values["user_id"] = userID
		_ = session.Save(req, rec)
		for _, cookie := range rec.Result().Cookies() {
			req.AddCookie(cookie)
		}
		return req
	}

	// 1. Unauthenticated -> 401
	unauthReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/messages/members?conversation_id=%d", conv.ID), nil)
	unauthRR := httptest.NewRecorder()
	GetConversationMembersAPI(unauthRR, unauthReq)
	if unauthRR.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for unauthenticated members call, got %d", unauthRR.Code)
	}

	// 2. User 3 non-member -> 403
	forbiddenReq := makeAuthReq(3, fmt.Sprintf("/messages/members?conversation_id=%d", conv.ID))
	forbiddenRR := httptest.NewRecorder()
	GetConversationMembersAPI(forbiddenRR, forbiddenReq)
	if forbiddenRR.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for non-member call, got %d", forbiddenRR.Code)
	}

	// 3. User 1 member -> 200 with members array
	successReq := makeAuthReq(1, fmt.Sprintf("/messages/members?conversation_id=%d", conv.ID))
	successRR := httptest.NewRecorder()
	GetConversationMembersAPI(successRR, successReq)
	if successRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", successRR.Code, successRR.Body.String())
	}
	var res map[string]interface{}
	_ = json.Unmarshal(successRR.Body.Bytes(), &res)
	members, ok := res["members"].([]interface{})
	if !ok || len(members) != 2 {
		t.Errorf("Expected 2 members in response, got %v", res["members"])
	}
}

func TestDeleteMessageAPI_Modes(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := services.StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	msg, err := services.SendMessage(conv.ID, 1, "Sensitive note to be deleted", "", nil)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	makeAuthReq := func(userID int, bodyStr string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/messages/message/delete", strings.NewReader(bodyStr))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		session, _ := sessions.Store.Get(req, "youth-community")
		session.Values["user_id"] = userID
		_ = session.Save(req, rec)
		for _, cookie := range rec.Result().Cookies() {
			req.AddCookie(cookie)
		}
		return req
	}

	// 1. Mode "for_me": User 2 deletes message for me -> 200
	delForMeReq := makeAuthReq(2, fmt.Sprintf(`{"message_id": %d, "mode": "for_me"}`, msg.ID))
	delForMeRR := httptest.NewRecorder()
	DeleteMessageAPI(delForMeRR, delForMeReq)
	if delForMeRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for delete for me, got %d: %s", delForMeRR.Code, delForMeRR.Body.String())
	}
	var resForMe map[string]interface{}
	_ = json.Unmarshal(delForMeRR.Body.Bytes(), &resForMe)
	if resForMe["mode"] != "for_me" {
		t.Errorf("Expected mode 'for_me', got %v", resForMe["mode"])
	}

	// 2. Mode "for_everyone": User 1 deletes own message for everyone -> 200
	delForEveryoneReq := makeAuthReq(1, fmt.Sprintf(`{"message_id": %d, "mode": "for_everyone"}`, msg.ID))
	delForEveryoneRR := httptest.NewRecorder()
	DeleteMessageAPI(delForEveryoneRR, delForEveryoneReq)
	if delForEveryoneRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for delete for everyone, got %d: %s", delForEveryoneRR.Code, delForEveryoneRR.Body.String())
	}
	var resForEveryone map[string]interface{}
	_ = json.Unmarshal(delForEveryoneRR.Body.Bytes(), &resForEveryone)
	if resForEveryone["mode"] != "for_everyone" {
		t.Errorf("Expected mode 'for_everyone', got %v", resForEveryone["mode"])
	}
}

func TestStage6_UIRendering(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := services.StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	// Send an initial message and a reply message
	msg1, err := services.SendMessage(conv.ID, 1, "First test message", "", nil)
	if err != nil {
		t.Fatalf("SendMessage 1 failed: %v", err)
	}
	_, err = services.SendMessage(conv.ID, 2, "Second reply message", "", &msg1.ID)
	if err != nil {
		t.Fatalf("SendMessage 2 failed: %v", err)
	}

	// Toggle pin so pinned UI elements render
	_, _ = services.TogglePinConversation(conv.ID, 1)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/messages?c=%d", conv.ID), nil)
	rec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(req, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(req, rec)
	for _, cookie := range rec.Result().Cookies() {
		req.AddCookie(cookie)
	}

	inboxRR := httptest.NewRecorder()
	ChatInbox(inboxRR, req)
	if inboxRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from ChatInbox, got %d", inboxRR.Code)
	}

	html := inboxRR.Body.String()
	requiredElements := []string{
		`id="headerPinnedIcon"`,
		`id="clearChatModal"`,
		`id="deleteMessageModal"`,
		`id="composerReplyPreview"`,
		`id="mentionDropdown"`,
		`reply-msg-btn`,
		`copy-msg-btn`,
		`delete-for-me-btn`,
		`delete-for-everyone-btn`,
		`chat-quoted-reply`,
		`scrollToQuotedMessage`,
		`Clear Chat History`,
		`Export Chat (.txt)`,
		`Export Chat (.json)`,
	}

	for _, elem := range requiredElements {
		if !strings.Contains(html, elem) {
			t.Errorf("Expected UI to contain %q, but was missing", elem)
		}
	}
}

func TestForwardMessagesAPI_Workflow(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Unauthenticated request should be rejected
	unauthReq := httptest.NewRequest(http.MethodPost, "/messages/forward", strings.NewReader(`{"message_ids":[1],"conversation_ids":[2]}`))
	unauthReq.Header.Set("Content-Type", "application/json")
	unauthRR := httptest.NewRecorder()
	ForwardMessagesAPI(unauthRR, unauthReq)
	if unauthRR.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized, got %d", unauthRR.Code)
	}

	// 2. Setup Conversations and Messages
	convA, err := services.StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation convA failed: %v", err)
	}
	convB, err := services.StartDirectConversation(1, 3)
	if err != nil {
		t.Fatalf("StartDirectConversation convB failed: %v", err)
	}

	msgA, err := services.SendMessage(convA.ID, 2, "Youth Fellowship Prayer Points", "client-msg-1", nil)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// 3. User 1 forwards msgA to convB
	payload := fmt.Sprintf(`{"message_ids":[%d],"conversation_ids":[%d]}`, msgA.ID, convB.ID)
	req := httptest.NewRequest(http.MethodPost, "/messages/forward", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(req, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(req, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		req.AddCookie(cookie)
	}

	rr := httptest.NewRecorder()
	ForwardMessagesAPI(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from ForwardMessagesAPI, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success        bool              `json:"success"`
		ForwardedCount int               `json:"forwarded_count"`
		Messages       []*models.Message `json:"messages"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}
	if !resp.Success || resp.ForwardedCount != 1 || len(resp.Messages) != 1 {
		t.Fatalf("Expected 1 forwarded message, got count=%d, len=%d", resp.ForwardedCount, len(resp.Messages))
	}
	if !resp.Messages[0].IsForwarded {
		t.Errorf("Expected IsForwarded to be true")
	}
	if resp.Messages[0].ForwardedFrom != "User Two" {
		t.Errorf("Expected ForwardedFrom 'User Two', got %q", resp.Messages[0].ForwardedFrom)
	}
}

func TestChatInbox_AndroidNavigationAndEligibleFriends(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Visit /messages without active conversation (?c= not provided)
	initReq := httptest.NewRequest(http.MethodGet, "/messages", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)

	inboxReq := httptest.NewRequest(http.MethodGet, "/messages", nil)
	for _, cookie := range saveRec.Result().Cookies() {
		inboxReq.AddCookie(cookie)
	}

	inboxRR := httptest.NewRecorder()
	ChatInbox(inboxRR, inboxReq)

	if inboxRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from ChatInbox, got %d", inboxRR.Code)
	}

	html := inboxRR.Body.String()

	// Verify conversation sidebar is visible without mobile d-none
	if !strings.Contains(html, `id="conversationSidebar"`) {
		t.Errorf("Expected #conversationSidebar to be present in HTML")
	}
	// Verify chatMainArea is hidden on mobile when no conversation is active
	if !strings.Contains(html, `id="chatMainArea" data-conv-id=""`) {
		t.Errorf("Expected chatMainArea to have empty conv-id when no conversation is active")
	}

	// Verify friends filter pill is present
	if !strings.Contains(html, `data-filter="friends"`) {
		t.Errorf("Expected Friends filter pill to be present")
	}

	// Verify eligible friends are rendered in the sidebar list
	if !strings.Contains(html, `conversation-group-friends`) {
		t.Errorf("Expected .conversation-group-friends to be rendered in sidebar")
	}
	if !strings.Contains(html, `/messages/start?user_id=2`) {
		t.Errorf("Expected link to start chat with User 2")
	}

	// 2. Click on eligible friend to start direct chat
	startReq := httptest.NewRequest(http.MethodGet, "/messages/start?user_id=2", nil)
	for _, cookie := range saveRec.Result().Cookies() {
		startReq.AddCookie(cookie)
	}
	startRR := httptest.NewRecorder()
	StartDirectChat(startRR, startReq)

	if startRR.Code != http.StatusSeeOther {
		t.Fatalf("Expected 303 See Other redirect from StartDirectChat, got %d", startRR.Code)
	}
	loc := startRR.Header().Get("Location")
	if !strings.HasPrefix(loc, "/messages?c=") {
		t.Fatalf("Expected redirect to /messages?c=..., got %q", loc)
	}

	// 3. Follow redirect to active conversation
	activeReq := httptest.NewRequest(http.MethodGet, loc, nil)
	for _, cookie := range saveRec.Result().Cookies() {
		activeReq.AddCookie(cookie)
	}
	activeRR := httptest.NewRecorder()
	ChatInbox(activeRR, activeReq)

	if activeRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from active conversation, got %d", activeRR.Code)
	}

	activeHTML := activeRR.Body.String()
	// Verify Back to conversations button with href="/messages" is rendered for mobile
	if !strings.Contains(activeHTML, `id="chatBackToInboxBtn"`) || !strings.Contains(activeHTML, `href="/messages"`) {
		t.Errorf("Expected #chatBackToInboxBtn with href='/messages' in active conversation")
	}
	// Verify composer form is active
	if !strings.Contains(activeHTML, `id="chatMessageForm"`) {
		t.Errorf("Expected #chatMessageForm in active conversation")
	}
}

func TestSendMessageAPI_ReplyToMessageID(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := services.StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	// User 1 sends initial message
	msg1, err := services.SendMessage(conv.ID, 1, "Original message from User One", "client-1", nil)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// User 2 sends reply via JSON SendMessageAPI with reply_to_message_id
	payload := fmt.Sprintf(`{
		"conversation_id": %d,
		"body": "Replying to User One",
		"client_message_id": "client-reply-1",
		"reply_to_message_id": %d
	}`, conv.ID, msg1.ID)

	req := httptest.NewRequest(http.MethodPost, "/messages/send", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(req, "youth-community")
	session.Values["user_id"] = 2
	_ = session.Save(req, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		req.AddCookie(cookie)
	}

	rr := httptest.NewRecorder()
	SendMessageAPI(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from SendMessageAPI, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success bool            `json:"success"`
		Message *models.Message `json:"message"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if !resp.Success || resp.Message == nil {
		t.Fatalf("Expected success true and valid message in response")
	}
	if resp.Message.ReplyToMessageID == nil || *resp.Message.ReplyToMessageID != msg1.ID {
		t.Fatalf("Expected ReplyToMessageID %d, got %v", msg1.ID, resp.Message.ReplyToMessageID)
	}
	if resp.Message.ReplyToMessage == nil {
		t.Fatalf("Expected ReplyToMessage to be hydrated, got nil")
	}
	if resp.Message.ReplyToMessage.Body != msg1.Body {
		t.Errorf("Expected quoted body %q, got %q", msg1.Body, resp.Message.ReplyToMessage.Body)
	}

	// Verify conversation retrieval hydrates quoted reply
	msgs, err := services.GetConversationMessages(conv.ID, 1, 10, 0)
	if err != nil {
		t.Fatalf("GetConversationMessages failed: %v", err)
	}
	var replyMsg *models.Message
	for i := range msgs {
		if msgs[i].ID == resp.Message.ID {
			replyMsg = &msgs[i]
			break
		}
	}
	if replyMsg == nil || replyMsg.ReplyToMessage == nil {
		t.Fatalf("Expected retrieved reply message to have hydrated ReplyToMessage")
	}
	if replyMsg.ReplyToMessage.Body != msg1.Body {
		t.Errorf("Expected quoted body in retrieved message %q, got %q", msg1.Body, replyMsg.ReplyToMessage.Body)
	}
}

func TestDeleteMessageAPI_ForMeAndForEveryone(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := services.StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	msg, err := services.SendMessage(conv.ID, 1, "Sensitive message to test deletion", "client-del-1", nil)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// 1. User 2 deletes message "for_me"
	delMePayload := fmt.Sprintf(`{"message_id": %d, "mode": "for_me"}`, msg.ID)
	reqMe := httptest.NewRequest(http.MethodPost, "/messages/message/delete", strings.NewReader(delMePayload))
	reqMe.Header.Set("Content-Type", "application/json")
	saveRecMe := httptest.NewRecorder()
	sessionMe, _ := sessions.Store.Get(reqMe, "youth-community")
	sessionMe.Values["user_id"] = 2
	_ = sessionMe.Save(reqMe, saveRecMe)
	for _, cookie := range saveRecMe.Result().Cookies() {
		reqMe.AddCookie(cookie)
	}

	rrMe := httptest.NewRecorder()
	DeleteMessageAPI(rrMe, reqMe)

	if rrMe.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from DeleteMessageAPI (for_me), got %d", rrMe.Code)
	}

	// User 2 cannot see the message anymore
	msgsUser2, err := services.GetConversationMessages(conv.ID, 2, 10, 0)
	if err != nil {
		t.Fatalf("GetConversationMessages user 2 failed: %v", err)
	}
	for _, m := range msgsUser2 {
		if m.ID == msg.ID {
			t.Errorf("Message %d should be hidden for User 2 after delete-for-me", msg.ID)
		}
	}

	// User 1 CAN still see the message
	msgsUser1, err := services.GetConversationMessages(conv.ID, 1, 10, 0)
	if err != nil {
		t.Fatalf("GetConversationMessages user 1 failed: %v", err)
	}
	found := false
	for _, m := range msgsUser1 {
		if m.ID == msg.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Message %d should still be visible for User 1", msg.ID)
	}

	// 2. User 2 (non-author, non-admin) attempts delete "for_everyone" -> Should fail 403 Forbidden
	delAllFailPayload := fmt.Sprintf(`{"message_id": %d, "mode": "for_everyone"}`, msg.ID)
	reqFail := httptest.NewRequest(http.MethodPost, "/messages/message/delete", strings.NewReader(delAllFailPayload))
	reqFail.Header.Set("Content-Type", "application/json")
	for _, cookie := range saveRecMe.Result().Cookies() {
		reqFail.AddCookie(cookie)
	}
	rrFail := httptest.NewRecorder()
	DeleteMessageAPI(rrFail, reqFail)
	if rrFail.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for non-author deleting for everyone, got %d", rrFail.Code)
	}

	// 3. User 1 (author) deletes "for_everyone" -> Should succeed
	delAllPayload := fmt.Sprintf(`{"message_id": %d, "mode": "for_everyone"}`, msg.ID)
	reqAuthor := httptest.NewRequest(http.MethodPost, "/messages/message/delete", strings.NewReader(delAllPayload))
	reqAuthor.Header.Set("Content-Type", "application/json")
	saveRecAuthor := httptest.NewRecorder()
	sessionAuthor, _ := sessions.Store.Get(reqAuthor, "youth-community")
	sessionAuthor.Values["user_id"] = 1
	_ = sessionAuthor.Save(reqAuthor, saveRecAuthor)
	for _, cookie := range saveRecAuthor.Result().Cookies() {
		reqAuthor.AddCookie(cookie)
	}
	rrAuthor := httptest.NewRecorder()
	DeleteMessageAPI(rrAuthor, reqAuthor)
	if rrAuthor.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from author deleting for everyone, got %d", rrAuthor.Code)
	}

	// Message should now be marked is_deleted
	deletedMsg, err := repository.GetMessageByID(msg.ID)
	if err != nil {
		t.Fatalf("GetMessageByID failed: %v", err)
	}
	if !deletedMsg.IsDeleted {
		t.Errorf("Expected message IsDeleted to be true")
	}
}

func TestReportAbuseAPI_MessageAndUser(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := services.StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	msg, err := services.SendMessage(conv.ID, 2, "Reportable content here", "client-rep-1", nil)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// 1. User 1 reports User 2's message
	payload := fmt.Sprintf(`{
		"reported_user_id": 2,
		"conversation_id": %d,
		"message_id": %d,
		"reason": "harassment",
		"details": "Unsolicited inappropriate content"
	}`, conv.ID, msg.ID)

	req := httptest.NewRequest(http.MethodPost, "/messages/report", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(req, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(req, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		req.AddCookie(cookie)
	}

	rr := httptest.NewRecorder()
	ReportAbuseAPI(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from ReportAbuseAPI, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success bool               `json:"success"`
		Report  *models.ChatReport `json:"report"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}
	if !resp.Success || resp.Report == nil {
		t.Fatalf("Expected report object in response")
	}
	if resp.Report.ReportedUserID != 2 || resp.Report.ReporterID != 1 {
		t.Errorf("Unexpected report fields: %+v", resp.Report)
	}

	// 2. Duplicate submission for the same message returns 409 Conflict
	rrDup := httptest.NewRecorder()
	reqDup := httptest.NewRequest(http.MethodPost, "/messages/report", strings.NewReader(payload))
	reqDup.Header.Set("Content-Type", "application/json")
	reqDup.Header.Set("Accept", "application/json")
	for _, cookie := range saveRec.Result().Cookies() {
		reqDup.AddCookie(cookie)
	}
	ReportAbuseAPI(rrDup, reqDup)
	if rrDup.Code != http.StatusConflict {
		t.Errorf("Expected 409 Conflict for duplicate report, got %d", rrDup.Code)
	}
}

func TestICEServersAPI_STUNConfiguration(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/messages/call/ice-servers", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(req, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(req, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		req.AddCookie(cookie)
	}

	rr := httptest.NewRecorder()
	ICEServersAPI(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from ICEServersAPI, got %d", rr.Code)
	}

	var resp struct {
		IceServers []map[string]any `json:"iceServers"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}
	if len(resp.IceServers) == 0 {
		t.Fatalf("Expected at least one ICE server entry")
	}
}

func TestCallInitiationAndStateTransitions(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Seed users and conversation
	_, err := database.DB.Exec(`
		INSERT INTO conversations (id, type, created_by) VALUES (10, 'direct', 1);
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (10, 1, 'member');
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (10, 2, 'member');
	`)
	if err != nil {
		t.Fatalf("Failed to seed conversation: %v", err)
	}

	// 1. Create initiated call log
	expiresAt := time.Now().Add(45 * time.Second)
	callLog := &models.CallLog{
		ConversationID: 10,
		CallerID:       1,
		ReceiverID:     2,
		CallType:       "audio",
		Status:         "initiated",
		ExpiresAt:      &expiresAt,
	}
	if err := repository.CreateCallLog(callLog); err != nil {
		t.Fatalf("CreateCallLog failed: %v", err)
	}
	if callLog.ID <= 0 {
		t.Fatalf("Expected positive CallID, got %d", callLog.ID)
	}

	// 2. Verify GetLatestInitiatedCall resolves this active call
	latest, err := repository.GetLatestInitiatedCall(1, 2)
	if err != nil || latest == nil {
		t.Fatalf("GetLatestInitiatedCall failed: %v", err)
	}
	if latest.ID != callLog.ID {
		t.Errorf("Expected call ID %d, got %d", callLog.ID, latest.ID)
	}
	if latest.Status != "initiated" {
		t.Errorf("Expected status 'initiated', got %q", latest.Status)
	}

	// 3. Transition to connected
	if err := repository.UpdateCallLogStatus(callLog.ID, "connected"); err != nil {
		t.Fatalf("UpdateCallLogStatus failed: %v", err)
	}

	// 4. Update call duration and complete via LogCallAPI
	logReq := httptest.NewRequest(http.MethodPost, "/messages/call/log", strings.NewReader(fmt.Sprintf(`{"call_id": %d, "status": "completed", "duration_seconds": 125}`, callLog.ID)))
	logReq.Header.Set("Content-Type", "application/json")
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(logReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(logReq, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		logReq.AddCookie(cookie)
	}

	logRR := httptest.NewRecorder()
	LogCallAPI(logRR, logReq)
	if logRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from LogCallAPI, got %d (body: %s)", logRR.Code, logRR.Body.String())
	}

	// 5. Verify call is completed and has duration 125s
	cl, err := repository.GetCallLogByID(callLog.ID)
	if err != nil {
		t.Fatalf("GetCallLogByID failed: %v", err)
	}
	if cl.Status != "completed" {
		t.Errorf("Expected status 'completed', got %q", cl.Status)
	}
	if cl.DurationSeconds != 125 {
		t.Errorf("Expected duration 125, got %d", cl.DurationSeconds)
	}

	// 6. After completion, GetLatestInitiatedCall should not return the completed call
	completedLatest, err := repository.GetLatestInitiatedCall(1, 2)
	if err == nil && completedLatest != nil {
		t.Errorf("Expected nil from GetLatestInitiatedCall for completed call, got %+v", completedLatest)
	}
}

func TestGenerateCallLinkAndJoinFlow(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Seed users and conversation
	_, err := database.DB.Exec(`
		INSERT INTO conversations (id, type, created_by) VALUES (20, 'direct', 1);
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (20, 1, 'member');
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (20, 2, 'member');
	`)
	if err != nil {
		t.Fatalf("Failed to seed conversation: %v", err)
	}

	// User 1 generates video call link
	genReq := httptest.NewRequest(http.MethodPost, "/messages/call/link/generate", strings.NewReader(`{"conversation_id": 20, "call_type": "video"}`))
	genReq.Header.Set("Content-Type", "application/json")
	saveRec1 := httptest.NewRecorder()
	session1, _ := sessions.Store.Get(genReq, "youth-community")
	session1.Values["user_id"] = 1
	_ = session1.Save(genReq, saveRec1)
	for _, cookie := range saveRec1.Result().Cookies() {
		genReq.AddCookie(cookie)
	}

	genRR := httptest.NewRecorder()
	GenerateCallLinkAPI(genRR, genReq)
	if genRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from GenerateCallLinkAPI, got %d (body: %s)", genRR.Code, genRR.Body.String())
	}

	var genResp struct {
		Success   bool   `json:"success"`
		Token     string `json:"token"`
		CallType  string `json:"call_type"`
		InviteURL string `json:"invite_url"`
		Path      string `json:"path"`
	}
	if err := json.Unmarshal(genRR.Body.Bytes(), &genResp); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}
	if !genResp.Success || genResp.Token == "" {
		t.Fatalf("Expected successful token generation, got %+v", genResp)
	}
	if genResp.CallType != "video" {
		t.Errorf("Expected call_type 'video', got %q", genResp.CallType)
	}

	// Non-member (user 3) attempts to join
	joinReq3 := httptest.NewRequest(http.MethodGet, "/messages/call/join?token="+genResp.Token, nil)
	saveRec3 := httptest.NewRecorder()
	session3, _ := sessions.Store.Get(joinReq3, "youth-community")
	session3.Values["user_id"] = 3
	_ = session3.Save(joinReq3, saveRec3)
	for _, cookie := range saveRec3.Result().Cookies() {
		joinReq3.AddCookie(cookie)
	}
	joinRR3 := httptest.NewRecorder()
	JoinCallLinkHandler(joinRR3, joinReq3)
	if joinRR3.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for non-member, got %d", joinRR3.Code)
	}

	// Authorized member (user 2) joins
	joinReq2 := httptest.NewRequest(http.MethodGet, "/messages/call/join?token="+genResp.Token, nil)
	saveRec2 := httptest.NewRecorder()
	session2, _ := sessions.Store.Get(joinReq2, "youth-community")
	session2.Values["user_id"] = 2
	_ = session2.Save(joinReq2, saveRec2)
	for _, cookie := range saveRec2.Result().Cookies() {
		joinReq2.AddCookie(cookie)
	}
	joinRR2 := httptest.NewRecorder()
	JoinCallLinkHandler(joinRR2, joinReq2)
	if joinRR2.Code != http.StatusSeeOther {
		t.Fatalf("Expected 303 Redirect for authorized join, got %d", joinRR2.Code)
	}
	redirectLoc := joinRR2.Header().Get("Location")
	if !strings.Contains(redirectLoc, "c=20") || !strings.Contains(redirectLoc, "autocall=video") {
		t.Errorf("Expected redirect to contain c=20 and autocall=video, got %q", redirectLoc)
	}
}

func TestSendMessageAPI_ContentParameterFallback(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, err := database.DB.Exec(`
		INSERT INTO conversations (id, type, created_by) VALUES (30, 'direct', 1);
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (30, 1, 'member');
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (30, 2, 'member');
	`)
	if err != nil {
		t.Fatalf("Failed to seed conversation: %v", err)
	}

	// Test multipart Form with 'content' field (used by sendCallLinkToChatBtn)
	bodyBuf := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuf)
	_ = writer.WriteField("conversation_id", "30")
	_ = writer.WriteField("content", "📹 Join our video call: https://example.com/messages/call/join?token=test1234")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/messages/send", bodyBuf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(req, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(req, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		req.AddCookie(cookie)
	}

	rr := httptest.NewRecorder()
	SendMessageAPI(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK when using 'content' form field, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success bool            `json:"success"`
		Message *models.Message `json:"message"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}
	if !resp.Success || resp.Message == nil {
		t.Fatalf("Expected successful message creation, got %+v", resp)
	}
	if !strings.Contains(resp.Message.Body, "Join our video call") {
		t.Errorf("Expected message body to contain call link invitation, got %q", resp.Message.Body)
	}
}

func TestChatTemplate_AllCallingAndVoiceControlsRendered(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Seed users and direct conversation
	_, err := database.DB.Exec(`
		INSERT INTO conversations (id, type, created_by) VALUES (40, 'direct', 1);
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (40, 1, 'member');
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (40, 2, 'member');
	`)
	if err != nil {
		t.Fatalf("Failed to seed conversation: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/messages?c=40", nil)
	initReq := httptest.NewRequest(http.MethodGet, "/messages", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		req.AddCookie(cookie)
	}

	rr := httptest.NewRecorder()
	ChatInbox(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from ChatInbox, got %d", rr.Code)
	}

	html := rr.Body.String()

	// Verify Audio & Video Call controls, Voice notes, and Call link elements
	expectedIDs := []string{
		`id="startAudioCallBtn"`,
		`id="startVideoCallBtn"`,
		`id="chatCallDropdownBtn"`,
		`id="sendDirectCallLinkBtn"`,
		`id="voiceRecordBtn"`,
		`id="voiceRecordingBar"`,
		`id="voicePreviewAudio"`,
		`id="sendCallLinkModal"`,
		`id="copyCallInvitationLinkBtn"`,
		`id="shareCallInvitationLinkBtn"`,
		`id="sendCallLinkToChatBtn"`,
		`id="incomingCallModal"`,
		`id="activeCallModal"`,
		`id="mediaPermissionModal"`,
		`id="remoteVideo"`,
		`id="localVideo"`,
	}

	for _, expectedID := range expectedIDs {
		if !strings.Contains(html, expectedID) {
			t.Errorf("Expected chat.html to render %s, but was not found in rendered HTML", expectedID)
		}
	}
}

func TestDirectVideoCall_OfferAnswerAndSignalingFlow(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Seed users and direct conversation
	_, err := database.DB.Exec(`
		INSERT INTO conversations (id, type, created_by) VALUES (50, 'direct', 1);
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (50, 1, 'member');
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (50, 2, 'member');
	`)
	if err != nil {
		t.Fatalf("Failed to seed conversation: %v", err)
	}

	// 1. Caller initiates video call
	expiresAt := time.Now().Add(45 * time.Second)
	callLog := &models.CallLog{
		ConversationID: 50,
		CallerID:       1,
		ReceiverID:     2,
		CallType:       "video",
		Status:         "initiated",
		ExpiresAt:      &expiresAt,
	}
	if err := repository.CreateCallLog(callLog); err != nil {
		t.Fatalf("CreateCallLog failed for video call: %v", err)
	}
	if callLog.ID <= 0 {
		t.Fatalf("Expected positive CallID, got %d", callLog.ID)
	}
	if callLog.CallType != "video" {
		t.Errorf("Expected CallType 'video', got %q", callLog.CallType)
	}

	// 2. Callee accepts and status moves to connected
	if err := repository.UpdateCallLogStatus(callLog.ID, "connected"); err != nil {
		t.Fatalf("Failed updating call status to connected: %v", err)
	}

	cl, err := repository.GetCallLogByID(callLog.ID)
	if err != nil || cl == nil {
		t.Fatalf("Failed retrieving call log: %v", err)
	}
	if cl.Status != "connected" {
		t.Errorf("Expected status 'connected', got %q", cl.Status)
	}

	// 3. Complete call via LogCallAPI
	logReq := httptest.NewRequest(http.MethodPost, "/messages/call/log", strings.NewReader(fmt.Sprintf(`{"call_id": %d, "status": "completed", "duration_seconds": 180}`, callLog.ID)))
	logReq.Header.Set("Content-Type", "application/json")
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(logReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(logReq, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		logReq.AddCookie(cookie)
	}

	logRR := httptest.NewRecorder()
	LogCallAPI(logRR, logReq)
	if logRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from LogCallAPI, got %d (body: %s)", logRR.Code, logRR.Body.String())
	}

	finalLog, err := repository.GetCallLogByID(callLog.ID)
	if err != nil || finalLog == nil {
		t.Fatalf("Failed retrieving final call log: %v", err)
	}
	if finalLog.Status != "completed" {
		t.Errorf("Expected completed status, got %q", finalLog.Status)
	}
	if finalLog.DurationSeconds != 180 {
		t.Errorf("Expected duration 180, got %d", finalLog.DurationSeconds)
	}
}

func TestGroupVideoCall_MeshSignalingAndRoster(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Seed community group conversation
	_, err := database.DB.Exec(`
		INSERT INTO conversations (id, type, title, created_by) VALUES (60, 'group', 'Video Fellowship', 1);
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (60, 1, 'owner');
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (60, 2, 'member');
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (60, 3, 'member');
	`)
	if err != nil {
		t.Fatalf("Failed to seed group conversation: %v", err)
	}

	// 1. User 1 initiates group video call
	room, err := services.GlobalGroupCallManager.StartGroupCall(60, 1, "video")
	if err != nil {
		t.Fatalf("StartGroupCall failed: %v", err)
	}
	if room.CallType != "video" {
		t.Errorf("Expected room call type 'video', got %q", room.CallType)
	}
	if len(room.Participants) != 1 {
		t.Errorf("Expected 1 initial participant, got %d", len(room.Participants))
	}

	// 2. User 2 and User 3 join
	_, p2, err := services.GlobalGroupCallManager.JoinGroupCall(60, 2)
	if err != nil {
		t.Fatalf("User 2 join failed: %v", err)
	}
	if p2.IsCamOff {
		t.Errorf("Expected User 2 camera to be on for video call, got IsCamOff=true")
	}

	_, p3, err := services.GlobalGroupCallManager.JoinGroupCall(60, 3)
	if err != nil {
		t.Fatalf("User 3 join failed: %v", err)
	}
	if p3.UserID != 3 {
		t.Errorf("Expected User 3 info, got ID %d", p3.UserID)
	}

	// 3. Verify AreParticipantsInCall for mesh pairs
	callID, inCall := services.GlobalGroupCallManager.AreParticipantsInCall(60, 1, 2)
	if !inCall || callID <= 0 {
		t.Errorf("Expected User 1 and User 2 to be in active group call, got inCall=%v callID=%d", inCall, callID)
	}
	_, inCall2 := services.GlobalGroupCallManager.AreParticipantsInCall(60, 1, 3)
	if !inCall2 {
		t.Errorf("Expected User 1 and User 3 to be in active group call")
	}
	_, inCallNonMember := services.GlobalGroupCallManager.AreParticipantsInCall(60, 1, 999)
	if inCallNonMember {
		t.Errorf("Expected non-participant 999 to return false for AreParticipantsInCall")
	}

	// 4. Update media state (toggle camera and mute)
	updatedParticipant, err := services.GlobalGroupCallManager.UpdateParticipantMediaState(60, 2, true, true, false)
	if err != nil {
		t.Fatalf("UpdateParticipantMediaState failed: %v", err)
	}
	if !updatedParticipant.IsMuted || !updatedParticipant.IsCamOff {
		t.Errorf("Expected participant 2 to be muted and camera off")
	}

	// 5. End group call by owner
	endedRoom, err := services.GlobalGroupCallManager.EndGroupCall(60, 1)
	if err != nil {
		t.Fatalf("EndGroupCall failed: %v", err)
	}
	if endedRoom == nil {
		t.Fatalf("Expected ended room return")
	}

	_, stillActive := services.GlobalGroupCallManager.GetActiveGroupCall(60)
	if stillActive {
		t.Errorf("Expected group call room to be removed from active manager")
	}
}

func TestChatTemplate_GroupVideoCallingControlsRendered(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Seed users and group conversation
	_, err := database.DB.Exec(`
		INSERT INTO conversations (id, type, title, created_by) VALUES (70, 'group', 'Youth Praise Team', 1);
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (70, 1, 'owner');
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (70, 2, 'member');
	`)
	if err != nil {
		t.Fatalf("Failed to seed group conversation: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/messages?c=70", nil)
	initReq := httptest.NewRequest(http.MethodGet, "/messages", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		req.AddCookie(cookie)
	}

	rr := httptest.NewRecorder()
	ChatInbox(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from ChatInbox, got %d", rr.Code)
	}

	html := rr.Body.String()

	// Verify group call modal, grid, controls, and incoming call UI
	expectedGroupIDs := []string{
		`id="startGroupVideoCallBtn"`,
		`id="startGroupAudioCallBtn"`,
		`id="chatGroupCallDropdownBtn"`,
		`id="groupCallModal"`,
		`id="groupCallVideoGrid"`,
		`id="groupLocalVideo"`,
		`id="groupLocalAvatar"`,
		`id="groupToggleMicBtn"`,
		`id="groupToggleCamBtn"`,
		`id="groupLeaveCallBtn"`,
		`id="incomingGroupCallModal"`,
		`id="activeGroupCallBanner"`,
	}

	for _, expectedID := range expectedGroupIDs {
		if !strings.Contains(html, expectedID) {
			t.Errorf("Expected group chat.html to render %s, but was not found", expectedID)
		}
	}

	// Verify joinActiveGroupCallBtn is rendered in DOM
	if !strings.Contains(html, `id="joinActiveGroupCallBtn"`) {
		t.Errorf("Expected joinActiveGroupCallBtn to be rendered in group chat template")
	}

	// Verify remoteAudio is rendered in activeCallModal
	if !strings.Contains(html, `id="remoteAudio"`) {
		t.Errorf("Expected remoteAudio element to be rendered in activeCallModal")
	}
}

func TestGroupAudioCalling_FullLifecycleAndLateJoin(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Seed users and group conversation
	_, err := database.DB.Exec(`
		INSERT INTO conversations (id, type, title, created_by) VALUES (80, 'group', 'Youth Prayer Group', 1);
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (80, 1, 'owner');
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (80, 2, 'member');
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (80, 3, 'member');
	`)
	if err != nil {
		t.Fatalf("Failed to seed group conversation: %v", err)
	}

	// 1. User 1 starts group audio call
	room, err := services.GlobalGroupCallManager.StartGroupCall(80, 1, "audio")
	if err != nil {
		t.Fatalf("StartGroupCall audio failed: %v", err)
	}
	if room.CallType != "audio" {
		t.Errorf("Expected CallType=audio, got %s", room.CallType)
	}
	if room.Title != "Youth Prayer Group" {
		t.Errorf("Expected Title='Youth Prayer Group', got %s", room.Title)
	}

	// 2. Query GetUserActiveGroupCallsAPI for User 2 (member)
	user2Req, user2RR := makeAuthReq(http.MethodGet, "/messages/call/group/user-active", "", "", 2)
	GetUserActiveGroupCallsAPI(user2RR, user2Req)
	if user2RR.Code != http.StatusOK {
		t.Fatalf("GetUserActiveGroupCallsAPI failed: %d", user2RR.Code)
	}
	var res map[string]any
	if err := json.Unmarshal(user2RR.Body.Bytes(), &res); err != nil {
		t.Fatalf("Failed to parse user-active response: %v", err)
	}
	rooms, ok := res["rooms"].([]any)
	if !ok || len(rooms) != 1 {
		t.Fatalf("Expected 1 active room for User 2, got %v", res["rooms"])
	}

	// 3. User 2 and User 3 join audio call
	_, p2, err := services.GlobalGroupCallManager.JoinGroupCall(80, 2)
	if err != nil {
		t.Fatalf("User 2 join failed: %v", err)
	}
	if !p2.IsCamOff {
		t.Errorf("Expected participant 2 IsCamOff=true for audio call")
	}

	_, p3, err := services.GlobalGroupCallManager.JoinGroupCall(80, 3)
	if err != nil {
		t.Fatalf("User 3 join failed: %v", err)
	}
	if !p3.IsCamOff {
		t.Errorf("Expected participant 3 IsCamOff=true for audio call")
	}

	// 4. Verify mesh participant membership
	callID, inCall := services.GlobalGroupCallManager.AreParticipantsInCall(80, 1, 2, 3)
	if !inCall || callID <= 0 {
		t.Errorf("Expected all 3 users in active group audio call, got inCall=%v callID=%d", inCall, callID)
	}

	// 5. Test speaking indicator and mute state transitions
	updatedP2, err := services.GlobalGroupCallManager.UpdateParticipantMediaState(80, 2, true, true, false)
	if err != nil {
		t.Fatalf("UpdateParticipantMediaState failed: %v", err)
	}
	if !updatedP2.IsMuted {
		t.Errorf("Expected User 2 to be muted")
	}

	// 6. User 2 and User 3 leave
	_, _, err = services.GlobalGroupCallManager.LeaveGroupCall(80, 2)
	if err != nil {
		t.Fatalf("LeaveGroupCall for User 2 failed: %v", err)
	}
	_, _, err = services.GlobalGroupCallManager.LeaveGroupCall(80, 3)
	if err != nil {
		t.Fatalf("LeaveGroupCall for User 3 failed: %v", err)
	}

	// Still active because User 1 is remaining
	activeRoom, activeExists := services.GlobalGroupCallManager.GetActiveGroupCall(80)
	if !activeExists || len(activeRoom.Participants) != 1 {
		t.Errorf("Expected 1 participant remaining in group call")
	}

	// User 1 leaves -> room automatically ends
	_, _, err = services.GlobalGroupCallManager.LeaveGroupCall(80, 1)
	if err != nil {
		t.Fatalf("LeaveGroupCall for User 1 failed: %v", err)
	}

	// Verify no active room remains
	_, stillExists := services.GlobalGroupCallManager.GetActiveGroupCall(80)
	if stillExists {
		t.Errorf("Expected group call to be cleaned up after all participants left")
	}

	// Verify GetUserActiveGroupCallsAPI now returns 0 rooms
	user2ReqEmpty, user2RREmpty := makeAuthReq(http.MethodGet, "/messages/call/group/user-active", "", "", 2)
	GetUserActiveGroupCallsAPI(user2RREmpty, user2ReqEmpty)
	if user2RREmpty.Code != http.StatusOK {
		t.Fatalf("GetUserActiveGroupCallsAPI failed: %d", user2RREmpty.Code)
	}
	var resEmpty map[string]any
	_ = json.Unmarshal(user2RREmpty.Body.Bytes(), &resEmpty)
	roomsEmpty, _ := resEmpty["rooms"].([]any)
	if len(roomsEmpty) != 0 {
		t.Errorf("Expected 0 active rooms after call ended, got %d", len(roomsEmpty))
	}
}




