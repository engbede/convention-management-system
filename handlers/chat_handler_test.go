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

	"convention-management-system/database"
	"convention-management-system/middleware"
	"convention-management-system/sessions"

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
	`)
	if err != nil {
		t.Fatalf("Failed to insert test users: %v", err)
	}

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

	// 4. UpdateGroupSettingsAPI (Owner updates group)
	updateJSON := fmt.Sprintf(`{"group_id": %d, "title": "Youth Prayer Ministry", "category": "prayer", "description": "Updated description", "call_permission": "admin_only"}`, groupID)
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






