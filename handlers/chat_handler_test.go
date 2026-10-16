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
