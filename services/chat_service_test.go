package services

import (
	"bytes"
	"database/sql"
	"mime/multipart"
	"os"
	"strings"
	"testing"
	"time"

	"convention-management-system/database"
	"convention-management-system/models"
	"convention-management-system/repository"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) func() {
	t.Helper()
	testDBPath := "test_service.db"
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
			(1, 'Sender User', 'sender', 'hash1'),
			(2, 'Receiver User', 'receiver', 'hash2'),
			(3, 'Outsider User', 'outsider', 'hash3');
	`)
	if err != nil {
		t.Fatalf("Failed to insert test users: %v", err)
	}

	return func() {
		db.Close()
		_ = os.Remove(testDBPath)
	}
}

func TestStartDirectConversationValidation(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Cannot chat with self
	_, err := StartDirectConversation(1, 1)
	if err == nil {
		t.Errorf("Expected error when chatting with self, got nil")
	}

	// 2. Cannot chat with non-existent member
	_, err = StartDirectConversation(1, 9999)
	if err == nil {
		t.Errorf("Expected error for non-existent target, got nil")
	}

	// 3. Valid conversation creation
	conv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}
	if conv.ID <= 0 {
		t.Errorf("Expected valid ID, got %d", conv.ID)
	}
}

func TestSendMessageValidationAndAuthorization(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("Setup conversation failed: %v", err)
	}

	// 1. Reject empty message
	_, err = SendMessage(conv.ID, 1, "   ", "", nil)
	if err == nil {
		t.Errorf("Expected error for empty message, got nil")
	}

	// 2. Reject oversized message (> 2000 chars)
	longBody := strings.Repeat("A", 2001)
	_, err = SendMessage(conv.ID, 1, longBody, "", nil)
	if err == nil {
		t.Errorf("Expected error for message > 2000 chars, got nil")
	}

	// 3. Reject sender who is not a member of the conversation (User 3)
	_, err = SendMessage(conv.ID, 3, "Hello from intruder", "", nil)
	if err == nil {
		t.Errorf("Expected unauthorized error for non-member sender, got nil")
	}

	// 4. Accept valid message from authorized member
	msg, err := SendMessage(conv.ID, 1, "Valid greeting", "idemp-key-1", nil)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if msg.Body != "Valid greeting" {
		t.Errorf("Expected body 'Valid greeting', got %q", msg.Body)
	}
}

func TestGetConversationMessagesAuthorization(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("Setup conversation failed: %v", err)
	}

	_, err = SendMessage(conv.ID, 1, "Private message", "", nil)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// 1. Outsider (User 3) should be denied access to conversation messages
	_, err = GetConversationMessages(conv.ID, 3, 50, 0)
	if err == nil {
		t.Errorf("Expected access denied error for outsider user 3, got nil")
	}

	// 2. Recipient (User 2) should successfully read messages
	msgs, err := GetConversationMessages(conv.ID, 2, 50, 0)
	if err != nil {
		t.Fatalf("GetConversationMessages failed for user 2: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("Expected 1 message, got %d", len(msgs))
	}
}

func TestSendAttachmentMessage(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()
	defer os.RemoveAll("uploads/chat")

	conv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("Setup conversation failed: %v", err)
	}

	// 1. Construct multipart file for voice note (.mp3)
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	part, _ := w.CreateFormFile("attachment", "voice_note.mp3")
	part.Write([]byte("fake-mp3-audio-data-stream"))
	w.Close()

	r := multipart.NewReader(buf, w.Boundary())
	form, err := r.ReadForm(10 << 20)
	if err != nil {
		t.Fatalf("ReadForm failed: %v", err)
	}
	defer form.RemoveAll()

	fileHeaders := form.File["attachment"]
	if len(fileHeaders) == 0 {
		t.Fatalf("No file headers found")
	}

	file, err := fileHeaders[0].Open()
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}

	msg, err := SendAttachmentMessage(conv.ID, 1, file, fileHeaders[0], "Listen to this note", "voice-123", nil)
	if err != nil {
		t.Fatalf("SendAttachmentMessage failed: %v", err)
	}

	if msg.MessageType != "voice" {
		t.Errorf("Expected message_type 'voice', got %q", msg.MessageType)
	}
	if msg.AttachmentName != "voice_note.mp3" {
		t.Errorf("Expected attachment_name 'voice_note.mp3', got %q", msg.AttachmentName)
	}
	if !strings.HasPrefix(msg.AttachmentURL, "/uploads/chat/") {
		t.Errorf("Expected attachment_url to start with '/uploads/chat/', got %q", msg.AttachmentURL)
	}
	if msg.Body != "Listen to this note" {
		t.Errorf("Expected caption 'Listen to this note', got %q", msg.Body)
	}
}

func TestSendVoiceNote_DirectAndGroup_WithoutBody(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Setup Direct Conversation
	directConv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	// Helper to create a fake WebM audio file header
	createAudioFile := func(filename string) (multipart.File, *multipart.FileHeader, func()) {
		buf := &bytes.Buffer{}
		w := multipart.NewWriter(buf)
		part, _ := w.CreateFormFile("attachment", filename)
		part.Write([]byte("fake-webm-opus-audio-bytes-1234567890"))
		w.Close()

		r := multipart.NewReader(buf, w.Boundary())
		form, err := r.ReadForm(10 << 20)
		if err != nil {
			t.Fatalf("ReadForm failed: %v", err)
		}
		headers := form.File["attachment"]
		file, err := headers[0].Open()
		if err != nil {
			t.Fatalf("Failed to open file: %v", err)
		}
		return file, headers[0], func() {
			file.Close()
			form.RemoveAll()
		}
	}

	file1, header1, clean1 := createAudioFile("voice-note-direct.webm")
	defer clean1()

	// Send voice note with EMPTY body/caption in direct conversation
	msgDirect, err := SendAttachmentMessage(directConv.ID, 1, file1, header1, "", "client-voice-direct-1", nil)
	if err != nil {
		t.Fatalf("SendAttachmentMessage for voice note failed: %v", err)
	}
	if msgDirect.MessageType != "voice" {
		t.Errorf("Expected message_type 'voice', got %q", msgDirect.MessageType)
	}
	if msgDirect.AttachmentType != "audio/webm" {
		t.Errorf("Expected attachment_type 'audio/webm', got %q", msgDirect.AttachmentType)
	}
	if msgDirect.Body != "" {
		t.Errorf("Expected empty body, got %q", msgDirect.Body)
	}
	if !strings.HasPrefix(msgDirect.AttachmentURL, "/uploads/chat/") {
		t.Errorf("Expected attachment URL starting with '/uploads/chat/', got %q", msgDirect.AttachmentURL)
	}

	// 2. Setup Group Conversation and test sending voice note
	groupConv, err := repository.CreateGroupConversation("Fellowship Group", "general", "Weekly Fellowship", "", "anyone", false, 1, []int{1, 2})
	if err != nil {
		t.Fatalf("CreateGroupConversation failed: %v", err)
	}

	file2, header2, clean2 := createAudioFile("voice-note-group.webm")
	defer clean2()

	msgGroup, err := SendAttachmentMessage(groupConv.ID, 2, file2, header2, "", "client-voice-group-1", nil)
	if err != nil {
		t.Fatalf("SendAttachmentMessage in group conversation failed: %v", err)
	}
	if msgGroup.MessageType != "voice" {
		t.Errorf("Expected group voice message_type 'voice', got %q", msgGroup.MessageType)
	}
	if msgGroup.AttachmentType != "audio/webm" {
		t.Errorf("Expected group voice attachment_type 'audio/webm', got %q", msgGroup.AttachmentType)
	}

	// 3. Unauthorized sender (User 3 is not in direct conversation or group)
	file3, header3, clean3 := createAudioFile("voice-intruder.webm")
	defer clean3()

	_, err = SendAttachmentMessage(directConv.ID, 3, file3, header3, "", "client-voice-intruder", nil)
	if err == nil {
		t.Errorf("Expected error when non-member sends attachment to direct conversation, got nil")
	}
}

func TestBlockAndUnblockUser(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	// 1. Cannot block self
	err = BlockUser(1, 1, "test")
	if err == nil {
		t.Errorf("Expected error when blocking self, got nil")
	}

	// 2. User 1 blocks User 2
	err = BlockUser(1, 2, "Stop messaging me")
	if err != nil {
		t.Fatalf("BlockUser failed: %v", err)
	}

	// Verify block state
	isBlocked, err := IsUserBlocked(1, 2)
	if err != nil || !isBlocked {
		t.Errorf("Expected user 2 to be blocked by user 1, got isBlocked=%v, err=%v", isBlocked, err)
	}

	isCommBlocked, err := IsCommunicationBlocked(1, 2)
	if err != nil || !isCommBlocked {
		t.Errorf("Expected communication to be blocked between 1 and 2, got %v", isCommBlocked)
	}

	// Reverse direction check
	isCommBlockedRev, err := IsCommunicationBlocked(2, 1)
	if err != nil || !isCommBlockedRev {
		t.Errorf("Expected communication to be blocked in reverse (2, 1), got %v", isCommBlockedRev)
	}

	// 3. User 1 tries to send message: should be rejected
	_, err = SendMessage(conv.ID, 1, "Blocked message from 1", "", nil)
	if err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Errorf("Expected blocked error when sending message, got %v", err)
	}

	// 4. User 2 tries to send message: should be rejected
	_, err = SendMessage(conv.ID, 2, "Blocked message from 2", "", nil)
	if err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Errorf("Expected blocked error when sending message, got %v", err)
	}

	// 5. User 1 unblocks User 2
	err = UnblockUser(1, 2)
	if err != nil {
		t.Fatalf("UnblockUser failed: %v", err)
	}

	isCommBlockedAfter, _ := IsCommunicationBlocked(1, 2)
	if isCommBlockedAfter {
		t.Errorf("Expected communication to no longer be blocked after unblock")
	}

	// 6. Message can now be sent successfully
	msg, err := SendMessage(conv.ID, 1, "Welcome back", "", nil)
	if err != nil {
		t.Fatalf("SendMessage failed after unblock: %v", err)
	}
	if msg.Body != "Welcome back" {
		t.Errorf("Expected body 'Welcome back', got %q", msg.Body)
	}
}

func TestReportAbuse(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Cannot report self
	_, err := ReportAbuse(1, 1, nil, nil, "Spam", "test details")
	if err == nil {
		t.Errorf("Expected error reporting self, got nil")
	}

	// 2. Empty reason should fail
	_, err = ReportAbuse(1, 2, nil, nil, "   ", "test details")
	if err == nil {
		t.Errorf("Expected error for empty reason, got nil")
	}

	// 3. Valid report
	convID := 1
	report, err := ReportAbuse(1, 2, &convID, nil, "Inappropriate Content", "Sent abusive message")
	if err != nil {
		t.Fatalf("ReportAbuse failed: %v", err)
	}
	if report.ID <= 0 {
		t.Errorf("Expected valid report ID, got %d", report.ID)
	}
	if report.Status != "pending" {
		t.Errorf("Expected status 'pending', got %q", report.Status)
	}
}

func TestSearchMessages(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("Setup conversation failed: %v", err)
	}

	// Send messages
	_, _ = SendMessage(conv.ID, 1, "Welcome to youth Bible study tonight!", "", nil)
	_, _ = SendMessage(conv.ID, 2, "Sounds great, don't forget your Bible", "", nil)
	_, _ = SendMessage(conv.ID, 1, "Prayer breakfast tomorrow morning", "", nil)

	// 1. Search for "Bible" as user 1
	results, err := SearchMessages(1, "Bible", nil, 10, 0)
	if err != nil {
		t.Fatalf("SearchMessages failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 search results for 'Bible', got %d", len(results))
	}

	// 2. Search for "Prayer" as user 2
	results, err = SearchMessages(2, "Prayer", nil, 10, 0)
	if err != nil {
		t.Fatalf("SearchMessages failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 search result for 'Prayer', got %d", len(results))
	}

	// 3. Outsider user 3 should see 0 results
	results, err = SearchMessages(3, "Bible", nil, 10, 0)
	if err != nil {
		t.Fatalf("SearchMessages failed for outsider: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 search results for outsider user 3, got %d", len(results))
	}
}

func TestCallLogsAndICEServers(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	// 1. Create Call Log
	call, err := CreateCallLog(conv.ID, 1, 2, "video", "initiated")
	if err != nil {
		t.Fatalf("CreateCallLog failed: %v", err)
	}
	if call.ID <= 0 {
		t.Errorf("Expected valid call ID, got %d", call.ID)
	}
	if call.CallType != "video" || call.Status != "initiated" {
		t.Errorf("Unexpected call attributes: %+v", call)
	}

	// 2. Update Call Log
	err = UpdateCallLog(call.ID, "completed", 45)
	if err != nil {
		t.Fatalf("UpdateCallLog failed: %v", err)
	}

	// 3. Get Conversation Call Logs
	logs, err := GetConversationCallLogs(conv.ID, 1)
	if err != nil {
		t.Fatalf("GetConversationCallLogs failed: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("Expected 1 call log, got %d", len(logs))
	}
	if logs[0].Status != "completed" || logs[0].DurationSeconds != 45 {
		t.Errorf("Unexpected call log details: %+v", logs[0])
	}

	// 4. Unauthorized participant cannot view call logs
	_, err = GetConversationCallLogs(conv.ID, 3)
	if err == nil {
		t.Errorf("Expected error for non-participant user 3, got nil")
	}

	// 5. ICE Servers
	servers := GetICEServers()
	if len(servers) == 0 {
		t.Errorf("Expected ICE servers list to not be empty")
	}
}

func TestCommunityGroups(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Validation: Empty title should fail
	_, err := CreateGroup(1, "   ", "general", "Test desc", "all", true, nil)
	if err == nil {
		t.Errorf("Expected error for empty group title, got nil")
	}

	// 2. Create group: Creator 1 adds member 2
	group, err := CreateGroup(1, "Youth Bible Study", "bible_study", "Weekly fellowship and scriptures", "admin_only", true, []int{2})
	if err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	if group.ID <= 0 {
		t.Fatalf("Expected valid group ID, got %d", group.ID)
	}
	if group.Type != "group" || group.Category != "bible_study" || group.CallPermission != "admin_only" {
		t.Errorf("Unexpected group properties: %+v", group)
	}

	// 3. GetGroupMembers
	members, err := GetGroupMembers(group.ID, 1)
	if err != nil {
		t.Fatalf("GetGroupMembers failed: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("Expected 2 group members (creator + member 2), got %d", len(members))
	}

	// Verify roles
	var creatorRole, member2Role string
	for _, m := range members {
		if m.UserID == 1 {
			creatorRole = m.Role
		} else if m.UserID == 2 {
			member2Role = m.Role
		}
	}
	if creatorRole != "owner" {
		t.Errorf("Expected creator role 'owner', got %q", creatorRole)
	}
	if member2Role != "member" {
		t.Errorf("Expected user 2 role 'member', got %q", member2Role)
	}

	// 4. Test Call Permissions
	// Creator (owner) should be able to call
	canCallOwner, err := CanUserCallInGroup(group.ID, 1)
	if err != nil || !canCallOwner {
		t.Errorf("Expected owner canCall=true, got %v (err: %v)", canCallOwner, err)
	}
	// Regular member should NOT be able to call since callPermission = 'admin_only'
	canCallMember, err := CanUserCallInGroup(group.ID, 2)
	if err != nil || canCallMember {
		t.Errorf("Expected regular member canCall=false, got %v (err: %v)", canCallMember, err)
	}

	// 5. UpdateGroupMemberRole: Promote user 2 to 'admin'
	err = UpdateGroupMemberRole(1, group.ID, 2, "admin")
	if err != nil {
		t.Fatalf("UpdateGroupMemberRole failed: %v", err)
	}
	canCallNowAdmin, err := CanUserCallInGroup(group.ID, 2)
	if err != nil || !canCallNowAdmin {
		t.Errorf("Expected promoted admin canCall=true, got %v (err: %v)", canCallNowAdmin, err)
	}

	// Demote back to member
	err = UpdateGroupMemberRole(1, group.ID, 2, "member")
	if err != nil {
		t.Fatalf("Demoting back to member failed: %v", err)
	}

	// 6. UpdateGroupSettings: Change call permission to 'all' and join_policy to 'open'
	err = UpdateGroupSettings(1, group.ID, "Youth Bible Study Revised", "bible_study", "New description", "all", "open")
	if err != nil {
		t.Fatalf("UpdateGroupSettings failed: %v", err)
	}
	canCallAfterAll, _ := CanUserCallInGroup(group.ID, 2)
	if !canCallAfterAll {
		t.Errorf("Expected member canCall=true when call_permission='all'")
	}

	// Non-admin cannot update group settings
	err = UpdateGroupSettings(2, group.ID, "Hacked Title", "bible_study", "", "all")
	if err == nil {
		t.Errorf("Expected error when non-admin tries to update settings, got nil")
	}

	// 7. AddGroupMembers: Owner adds user 3
	err = AddGroupMembers(1, group.ID, []int{3})
	if err != nil {
		t.Fatalf("AddGroupMembers failed: %v", err)
	}
	membersAfterAdd, _ := GetGroupMembers(group.ID, 1)
	if len(membersAfterAdd) != 3 {
		t.Errorf("Expected 3 members after add, got %d", len(membersAfterAdd))
	}

	// 8. RemoveGroupMember: Owner removes user 3
	err = RemoveGroupMember(1, group.ID, 3)
	if err != nil {
		t.Fatalf("RemoveGroupMember failed: %v", err)
	}
	membersAfterRemove, _ := GetGroupMembers(group.ID, 1)
	if len(membersAfterRemove) != 2 {
		t.Errorf("Expected 2 members after remove, got %d", len(membersAfterRemove))
	}

	// 9. LeaveGroup: User 2 leaves group
	err = LeaveGroup(2, group.ID)
	if err != nil {
		t.Fatalf("LeaveGroup failed: %v", err)
	}
	membersAfterLeave, _ := GetGroupMembers(group.ID, 1)
	if len(membersAfterLeave) != 1 {
		t.Errorf("Expected 1 member after leave, got %d", len(membersAfterLeave))
	}

	// 10. JoinGroup: User 2 rejoins public group
	err = JoinGroup(2, group.ID)
	if err != nil {
		t.Fatalf("JoinGroup failed: %v", err)
	}
	membersAfterRejoin, _ := GetGroupMembers(group.ID, 1)
	if len(membersAfterRejoin) != 2 {
		t.Errorf("Expected 2 members after rejoin, got %d", len(membersAfterRejoin))
	}

	// 11. GetDiscoverableGroups
	discoverable, err := GetDiscoverableGroups(3, "bible_study")
	if err != nil {
		t.Fatalf("GetDiscoverableGroups failed: %v", err)
	}
	found := false
	for _, dg := range discoverable {
		if dg.ID == group.ID {
			found = true
			if dg.Title != "Youth Bible Study Revised" {
				t.Errorf("Expected title 'Youth Bible Study Revised', got %q", dg.Title)
			}
			break
		}
	}
	if !found {
		t.Errorf("Expected group to be in discoverable list for category 'bible_study'")
	}
}

func TestGroupGovernanceAndModeration(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Create a group requiring approval (default policy)
	group, err := CreateGroup(1, "Governance Test Group", "prayer", "Prayer and governance", "admin_only", true, nil)
	if err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}

	// Direct joining must fail because join_policy == 'approval'
	err = JoinGroup(2, group.ID)
	if err == nil {
		t.Fatalf("Expected JoinGroup to fail for approval-required group, got nil")
	}

	// 2. Submit membership request
	req, err := RequestGroupMembership(2, group.ID, "I want to join the prayer team")
	if err != nil {
		t.Fatalf("RequestGroupMembership failed: %v", err)
	}
	if req.Status != "pending" {
		t.Errorf("Expected status 'pending', got %q", req.Status)
	}

	// Duplicate request should fail
	_, err = RequestGroupMembership(2, group.ID, "Another try")
	if err == nil {
		t.Errorf("Expected duplicate RequestGroupMembership to fail, got nil")
	}

	// Requester cancels request
	err = CancelGroupMembershipRequest(2, group.ID)
	if err != nil {
		t.Fatalf("CancelGroupMembershipRequest failed: %v", err)
	}

	// Re-submit request
	req, err = RequestGroupMembership(2, group.ID, "Requesting again")
	if err != nil {
		t.Fatalf("Re-submitting RequestGroupMembership failed: %v", err)
	}

	// Non-admin (user 3) cannot review request
	err = ReviewGroupMembershipRequest(3, req.ID, "approve")
	if err == nil {
		t.Errorf("Expected non-admin review to fail, got nil")
	}

	// Requester cannot review own request
	err = ReviewGroupMembershipRequest(2, req.ID, "approve")
	if err == nil {
		t.Errorf("Expected self-review to fail, got nil")
	}

	// Owner approves request
	err = ReviewGroupMembershipRequest(1, req.ID, "approve")
	if err != nil {
		t.Fatalf("Owner ReviewGroupMembershipRequest approve failed: %v", err)
	}

	// Verify user 2 is now member
	isMember, _ := repository.IsConversationMember(group.ID, 2)
	if !isMember {
		t.Errorf("Expected user 2 to be a member after approval")
	}

	// User 3 requests and is rejected
	req3, err := RequestGroupMembership(3, group.ID, "User 3 request")
	if err != nil {
		t.Fatalf("User 3 RequestGroupMembership failed: %v", err)
	}
	err = ReviewGroupMembershipRequest(1, req3.ID, "reject")
	if err != nil {
		t.Fatalf("ReviewGroupMembershipRequest reject failed: %v", err)
	}
	isMember3, _ := repository.IsConversationMember(group.ID, 3)
	if isMember3 {
		t.Errorf("Expected user 3 to not be a member after rejection")
	}

	// 3. Group Administration & Ownership Transfer
	// Owner promotes user 2 to admin
	err = UpdateGroupMemberRole(1, group.ID, 2, "admin")
	if err != nil {
		t.Fatalf("Promoting user 2 to admin failed: %v", err)
	}

	// User 2 (admin) cannot demote owner user 1
	err = UpdateGroupMemberRole(2, group.ID, 1, "member")
	if err == nil {
		t.Errorf("Expected admin demoting owner to fail, got nil")
	}

	// User 1 transfers ownership to user 2
	err = UpdateGroupMemberRole(1, group.ID, 2, "owner")
	if err != nil {
		t.Fatalf("Ownership transfer failed: %v", err)
	}

	// Check new roles: user 2 is owner, user 1 is admin
	m2, _ := repository.GetConversationMember(group.ID, 2)
	m1, _ := repository.GetConversationMember(group.ID, 1)
	if m2.Role != "owner" {
		t.Errorf("Expected user 2 to be owner, got %q", m2.Role)
	}
	if m1.Role != "admin" {
		t.Errorf("Expected user 1 to be admin, got %q", m1.Role)
	}

	// User 1 cannot remove owner user 2
	err = RemoveGroupMember(1, group.ID, 2)
	if err == nil {
		t.Errorf("Expected RemoveGroupMember on owner to fail, got nil")
	}

	// User 2 (owner) demotes user 1 to member
	err = UpdateGroupMemberRole(2, group.ID, 1, "member")
	if err != nil {
		t.Fatalf("Demoting user 1 to member failed: %v", err)
	}

	// User 2 cannot demote self or change own role
	err = UpdateGroupMemberRole(2, group.ID, 2, "member")
	if err == nil {
		t.Errorf("Expected UpdateGroupMemberRole on self to fail, got nil")
	}
	// Check that admin count is still 1
	adminCount, _ := repository.CountGroupAdministrators(group.ID)
	if adminCount != 1 {
		t.Errorf("Expected exactly 1 administrator, got %d", adminCount)
	}

	// 4. Message Moderation
	// User 1 sends a message
	msg1, err := SendMessage(group.ID, 1, "Hello from user 1", "client-mod-1", nil)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// User 3 (non-member) cannot delete user 1's message
	err = ModerateDeleteMessage(3, msg1.ID)
	if err == nil {
		t.Errorf("Expected non-member delete to fail, got nil")
	}

	// User 2 (owner) moderates/deletes user 1's message
	err = ModerateDeleteMessage(2, msg1.ID)
	if err != nil {
		t.Fatalf("ModerateDeleteMessage by owner failed: %v", err)
	}

	deletedMsg, err := repository.GetMessageByID(msg1.ID)
	if err != nil {
		t.Fatalf("GetMessageByID failed: %v", err)
	}
	if !deletedMsg.IsDeleted {
		t.Errorf("Expected msg to be deleted, got IsDeleted=false")
	}
	if deletedMsg.Body != "This message was removed by a moderator." {
		t.Errorf("Expected redacted body, got %q", deletedMsg.Body)
	}

	// Author deletes their own message
	msg2, err := SendMessage(group.ID, 1, "Another message by user 1", "client-mod-2", nil)
	if err != nil {
		t.Fatalf("SendMessage 2 failed: %v", err)
	}
	err = ModerateDeleteMessage(1, msg2.ID)
	if err != nil {
		t.Fatalf("Author deleting own message failed: %v", err)
	}
}

func TestGetUserInbox_AutoEnrollment(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Initially, user 3 is not enrolled in any direct or custom groups.
	// But MigrateSQLite seeded "Youth General Community" and auto-enrolled existing users.
	// Let's create user 4 who has no conversations at all.
	_, err := database.DB.Exec(`
		INSERT INTO users (id, full_name, username, password_hash)
		VALUES (4, 'New Youth Member', 'newyouth', 'hash4');
	`)
	if err != nil {
		t.Fatalf("Failed to insert user 4: %v", err)
	}

	// Verify user 4 has 0 conversations in DB before calling GetUserInbox
	convs, err := repository.GetUserConversations(4)
	if err != nil {
		t.Fatalf("GetUserConversations failed: %v", err)
	}
	if len(convs) != 0 {
		t.Fatalf("Expected 0 conversations for user 4 initially, got %d", len(convs))
	}

	// Calling GetUserInbox(4) should auto-enroll user 4 into Youth General Community
	items, unread, err := GetUserInbox(4)
	if err != nil {
		t.Fatalf("GetUserInbox failed: %v", err)
	}

	if len(items) == 0 {
		t.Errorf("Expected user 4 to be auto-enrolled into default Youth General Community, got 0 items")
	}

	foundGeneral := false
	for _, item := range items {
		if item.Title == "Youth General Community" {
			foundGeneral = true
			break
		}
	}
	if !foundGeneral {
		t.Errorf("Expected 'Youth General Community' group in user 4's inbox items")
	}

	_ = unread
}

func TestConvertDirectToGroupConversation(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Insert additional users
	_, _ = database.DB.Exec(`
		INSERT INTO users (id, full_name, username, password_hash)
		VALUES 
			(4, 'Fourth User', 'fourth', 'hash4'),
			(5, 'Blocked User', 'blocked', 'hash5');
	`)

	// 1. Create direct conversation between user 1 and user 2
	conv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}
	if conv.Type != "direct" {
		t.Fatalf("Expected direct conversation, got %s", conv.Type)
	}

	// 2. Send messages
	msg1, err := SendMessage(conv.ID, 1, "Hello from user 1", "c1", nil)
	if err != nil {
		t.Fatalf("SendMessage 1 failed: %v", err)
	}
	msg2, err := SendMessage(conv.ID, 2, "Hello back from user 2", "c2", nil)
	if err != nil {
		t.Fatalf("SendMessage 2 failed: %v", err)
	}

	// 3. Unauthorized outsider cannot convert
	_, err = ConvertDirectToGroupConversation(conv.ID, 4, []int{3}, "Hacked Group")
	if err == nil {
		t.Errorf("Expected error when non-participant tries to convert conversation, got nil")
	}

	// 4. Cannot convert with empty / non-existent user IDs
	_, err = ConvertDirectToGroupConversation(conv.ID, 1, []int{99999}, "New Group")
	if err == nil {
		t.Errorf("Expected error when no valid registered users selected, got nil")
	}

	// 5. Convert direct conversation to group adding user 3 and user 4
	updatedConv, err := ConvertDirectToGroupConversation(conv.ID, 1, []int{3, 4}, "Youth Fellowship Team")
	if err != nil {
		t.Fatalf("ConvertDirectToGroupConversation failed: %v", err)
	}

	if updatedConv.Type != "group" {
		t.Errorf("Expected conversation type 'group', got %s", updatedConv.Type)
	}
	if updatedConv.Title != "Youth Fellowship Team" {
		t.Errorf("Expected title 'Youth Fellowship Team', got %s", updatedConv.Title)
	}

	// 6. Verify membership roles
	members, err := repository.GetConversationMembers(conv.ID)
	if err != nil {
		t.Fatalf("GetConversationMembers failed: %v", err)
	}
	if len(members) != 4 {
		t.Fatalf("Expected 4 members in group, got %d", len(members))
	}

	roles := make(map[int]string)
	for _, m := range members {
		roles[m.UserID] = m.Role
	}

	if roles[1] != "owner" {
		t.Errorf("Expected user 1 to be owner, got %s", roles[1])
	}
	if roles[2] != "member" {
		t.Errorf("Expected user 2 to be member, got %s", roles[2])
	}
	if roles[3] != "member" {
		t.Errorf("Expected user 3 to be member, got %s", roles[3])
	}
	if roles[4] != "member" {
		t.Errorf("Expected user 4 to be member, got %s", roles[4])
	}

	// 7. Verify message history was preserved
	msgs, err := GetConversationMessages(conv.ID, 1, 50, 0)
	if err != nil {
		t.Fatalf("GetConversationMessages failed: %v", err)
	}

	foundMsg1 := false
	foundMsg2 := false
	foundSysMsg := false
	for _, m := range msgs {
		if m.ID == msg1.ID && m.Body == "Hello from user 1" {
			foundMsg1 = true
		}
		if m.ID == msg2.ID && m.Body == "Hello back from user 2" {
			foundMsg2 = true
		}
		if m.MessageType == "system" && strings.Contains(m.Body, "converted this conversation into a group") {
			foundSysMsg = true
		}
	}

	if !foundMsg1 || !foundMsg2 {
		t.Errorf("Historical direct chat messages were lost during conversion")
	}
	if !foundSysMsg {
		t.Errorf("System announcement message not found after conversion")
	}
}

func TestCallInvitationWorkflow(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Direct conversation invitation
	conv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	inv, inviteURL, err := GenerateCallInvitation(conv.ID, 1, "audio")
	if err != nil {
		t.Fatalf("GenerateCallInvitation failed: %v", err)
	}
	if inv.Token == "" {
		t.Fatalf("Expected non-empty invitation token")
	}
	if inv.CallType != "audio" {
		t.Errorf("Expected call_type 'audio', got %q", inv.CallType)
	}
	if !strings.Contains(inviteURL, "/messages/call/join?token=") {
		t.Errorf("Expected inviteURL to contain '/messages/call/join?token=', got %q", inviteURL)
	}

	// User 2 validates invitation
	validatedInv, validatedConv, err := ValidateCallInvitation(inv.Token, 2)
	if err != nil {
		t.Fatalf("ValidateCallInvitation failed for participant: %v", err)
	}
	if validatedConv.ID != conv.ID {
		t.Errorf("Expected conv ID %d, got %d", conv.ID, validatedConv.ID)
	}
	if validatedInv.Token != inv.Token {
		t.Errorf("Expected token %s, got %s", inv.Token, validatedInv.Token)
	}

	// User 3 (not a member of this direct chat) cannot validate
	_, _, err = ValidateCallInvitation(inv.Token, 3)
	if err == nil {
		t.Errorf("Expected ValidateCallInvitation to fail for non-member user 3, got nil")
	}

	// Invalid token
	_, _, err = ValidateCallInvitation("invalid-token-12345", 2)
	if err == nil {
		t.Errorf("Expected ValidateCallInvitation to fail for non-existent token, got nil")
	}

	// 2. Group conversation invitation
	group, err := CreateGroup(1, "Call Test Fellowship", "fellowship", "Fellowship call room", "admin_only", true, nil)
	if err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	_ = AddGroupMembers(1, group.ID, []int{2})

	groupInv, groupInviteURL, err := GenerateCallInvitation(group.ID, 1, "video")
	if err != nil {
		t.Fatalf("GenerateCallInvitation for group failed: %v", err)
	}
	if groupInv.CallType != "video" {
		t.Errorf("Expected call_type 'video', got %q", groupInv.CallType)
	}
	if !strings.Contains(groupInviteURL, groupInv.Token) {
		t.Errorf("Expected URL to contain token %s", groupInv.Token)
	}

	// Member validates
	_, gConv, err := ValidateCallInvitation(groupInv.Token, 2)
	if err != nil {
		t.Fatalf("ValidateCallInvitation for group member failed: %v", err)
	}
	if gConv.ID != group.ID {
		t.Errorf("Expected group ID %d, got %d", group.ID, gConv.ID)
	}

	// Non-member cannot validate group invitation
	_, _, err = ValidateCallInvitation(groupInv.Token, 3)
	if err == nil {
		t.Errorf("Expected ValidateCallInvitation to fail for non-member in group, got nil")
	}

	// 3. Blocking enforcement
	err = BlockUser(1, 2, "Test block")
	if err != nil {
		t.Fatalf("BlockUser failed: %v", err)
	}
	_, _, err = ValidateCallInvitation(inv.Token, 2)
	if err == nil {
		t.Errorf("Expected ValidateCallInvitation to fail when user 2 is blocked, got nil")
	}

	// 4. Token expiration enforcement
	// Artificially expire the token in database
	_, err = database.DB.Exec(`UPDATE call_invitations SET expires_at = ? WHERE token = ?`, time.Now().Add(-10*time.Minute), inv.Token)
	if err != nil {
		t.Fatalf("Failed to artificially expire token: %v", err)
	}
	_, _, err = ValidateCallInvitation(inv.Token, 2)
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Errorf("Expected ValidateCallInvitation to fail for expired token, got %v", err)
	}
}

func TestSearchMessagesScopingAndFiltering(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Create two conversations: Conv 1 (Users 1 & 2), Conv 2 (Users 1 & 3)
	conv1, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation conv1 failed: %v", err)
	}
	conv2, err := StartDirectConversation(1, 3)
	if err != nil {
		t.Fatalf("StartDirectConversation conv2 failed: %v", err)
	}

	// 2. Seed messages
	// Conv 1 messages
	_, err = database.DB.Exec(`
		INSERT INTO messages (id, conversation_id, sender_id, message_type, body, created_at, updated_at, is_deleted)
		VALUES 
			(101, ?, 1, 'text', 'Annual Youth Fellowship retreat details', datetime('now', '-3 hours'), datetime('now', '-3 hours'), 0),
			(102, ?, 2, 'text', 'Fellowship choir practice timing', datetime('now', '-2 hours'), datetime('now', '-2 hours'), 0),
			(103, ?, 1, 'text', 'Fellowship confidential deleted text', datetime('now', '-1 hours'), datetime('now', '-1 hours'), 1);
	`, conv1.ID, conv1.ID, conv1.ID)
	if err != nil {
		t.Fatalf("Failed to seed conv1 messages: %v", err)
	}

	// Conv 2 messages
	_, err = database.DB.Exec(`
		INSERT INTO messages (id, conversation_id, sender_id, message_type, body, created_at, updated_at, is_deleted)
		VALUES 
			(104, ?, 3, 'text', 'Fellowship announcement for external diocese', datetime('now', '-30 minutes'), datetime('now', '-30 minutes'), 0);
	`, conv2.ID)
	if err != nil {
		t.Fatalf("Failed to seed conv2 messages: %v", err)
	}

	// 3. Test Conversation-Scoped Search for Conv 1
	c1ID := conv1.ID
	results, err := SearchMessages(1, "Fellowship", &c1ID, 10, 0)
	if err != nil {
		t.Fatalf("SearchMessages scoped to conv1 failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Expected 2 matching messages in conv1, got %d", len(results))
	}
	for _, m := range results {
		if m.ConversationID != conv1.ID {
			t.Errorf("Expected conversation_id %d, got %d", conv1.ID, m.ConversationID)
		}
		if m.IsDeleted {
			t.Errorf("Deleted message %d was returned in search results", m.ID)
		}
	}

	// 4. Test unauthorized search on Conv 1 by User 3 (not a member of Conv 1)
	_, err = SearchMessages(3, "Fellowship", &c1ID, 10, 0)
	if err == nil {
		t.Errorf("Expected error when non-member searches conversation, got nil")
	}

	// 5. Test global search (convID = nil) for User 1 (member of both conv1 and conv2)
	globalResults, err := SearchMessages(1, "Fellowship", nil, 10, 0)
	if err != nil {
		t.Fatalf("Global SearchMessages failed: %v", err)
	}
	if len(globalResults) != 3 {
		t.Fatalf("Expected 3 matching messages across user 1 conversations, got %d", len(globalResults))
	}
	for _, m := range globalResults {
		if m.IsDeleted {
			t.Errorf("Soft-deleted message %d leaked into global search results", m.ID)
		}
	}

	// 6. Test block enforcement: User 1 blocks User 2
	if err := BlockUser(1, 2, "Harassment"); err != nil {
		t.Fatalf("BlockUser failed: %v", err)
	}
	blockedResults, err := SearchMessages(2, "Fellowship", &c1ID, 10, 0)
	if err != nil {
		t.Fatalf("SearchMessages failed: %v", err)
	}
	if len(blockedResults) != 0 {
		t.Errorf("Expected 0 results for blocked user, got %d", len(blockedResults))
	}
}

func TestGetSharedMediaCategorizedWorkflow(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	// Seed 4 distinct media types: photo, video, document, voice
	_, err = database.DB.Exec(`
		INSERT INTO messages (id, conversation_id, sender_id, message_type, body, attachment_url, attachment_name, attachment_type, attachment_size, created_at, updated_at)
		VALUES 
			(201, ?, 1, 'image', 'Banner image', '/uploads/chat/banner.jpg', 'banner.jpg', 'image/jpeg', 204800, datetime('now', '-4 hours'), datetime('now', '-4 hours')),
			(202, ?, 2, 'video', 'Sermon recording', '/uploads/chat/sermon.mp4', 'sermon.mp4', 'video/mp4', 5242880, datetime('now', '-3 hours'), datetime('now', '-3 hours')),
			(203, ?, 1, 'file', 'Youth handbook', '/uploads/chat/handbook.pdf', 'handbook.pdf', 'application/pdf', 1048576, datetime('now', '-2 hours'), datetime('now', '-2 hours')),
			(204, ?, 2, 'voice', 'Prayer voice note', '/uploads/chat/prayer.webm', 'prayer.webm', 'audio/webm', 65536, datetime('now', '-1 hours'), datetime('now', '-1 hours'));
	`, conv.ID, conv.ID, conv.ID, conv.ID)
	if err != nil {
		t.Fatalf("Failed to seed shared media: %v", err)
	}

	// 1. GetSharedMediaOverview
	overview, err := GetSharedMediaOverview(conv.ID, 1)
	if err != nil {
		t.Fatalf("GetSharedMediaOverview failed: %v", err)
	}
	if overview.TotalCount() != 4 {
		t.Errorf("Expected total count 4, got %d", overview.TotalCount())
	}
	if overview.PhotoCount() != 1 {
		t.Errorf("Expected 1 photo, got %d", overview.PhotoCount())
	}
	if overview.VideoCount() != 1 {
		t.Errorf("Expected 1 video, got %d", overview.VideoCount())
	}
	if overview.DocCount() != 1 {
		t.Errorf("Expected 1 document, got %d", overview.DocCount())
	}
	if overview.VoiceCount() != 1 {
		t.Errorf("Expected 1 voice note, got %d", overview.VoiceCount())
	}

	// 2. GetSharedMediaCategorized: photo
	photos, err := GetSharedMediaCategorized(conv.ID, 1, "photo", 10, 0)
	if err != nil {
		t.Fatalf("GetSharedMediaCategorized photos failed: %v", err)
	}
	if len(photos) != 1 || photos[0].AttachmentName != "banner.jpg" {
		t.Errorf("Expected 1 photo 'banner.jpg', got %+v", photos)
	}

	// 3. GetSharedMediaCategorized: video
	videos, err := GetSharedMediaCategorized(conv.ID, 1, "video", 10, 0)
	if err != nil {
		t.Fatalf("GetSharedMediaCategorized videos failed: %v", err)
	}
	if len(videos) != 1 || videos[0].AttachmentName != "sermon.mp4" {
		t.Errorf("Expected 1 video 'sermon.mp4', got %+v", videos)
	}

	// 4. GetSharedMediaCategorized: document
	docs, err := GetSharedMediaCategorized(conv.ID, 1, "document", 10, 0)
	if err != nil {
		t.Fatalf("GetSharedMediaCategorized documents failed: %v", err)
	}
	if len(docs) != 1 || docs[0].AttachmentName != "handbook.pdf" {
		t.Errorf("Expected 1 doc 'handbook.pdf', got %+v", docs)
	}

	// 5. GetSharedMediaCategorized: voice
	voiceNotes, err := GetSharedMediaCategorized(conv.ID, 1, "voice", 10, 0)
	if err != nil {
		t.Fatalf("GetSharedMediaCategorized voice failed: %v", err)
	}
	if len(voiceNotes) != 1 || voiceNotes[0].AttachmentName != "prayer.webm" {
		t.Errorf("Expected 1 voice note 'prayer.webm', got %+v", voiceNotes)
	}

	// 6. Access control check for outsider (User 3)
	_, err = GetSharedMediaOverview(conv.ID, 3)
	if err == nil {
		t.Errorf("Expected GetSharedMediaOverview to fail for non-member user 3, got nil")
	}
	_, err = GetSharedMediaCategorized(conv.ID, 3, "photo", 10, 0)
	if err == nil {
		t.Errorf("Expected GetSharedMediaCategorized to fail for non-member user 3, got nil")
	}
}

func TestReportAbuseAndDuplicateSpamPrevention(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	msg, err := SendMessage(conv.ID, 2, "Inappropriate test message", "cmsg_bad_1", nil)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// 1. Self-reporting is prevented
	_, err = ReportAbuse(1, 1, &conv.ID, nil, "Spam", "")
	if err == nil || !strings.Contains(err.Error(), "cannot report yourself") {
		t.Errorf("Expected cannot report yourself error, got %v", err)
	}

	// 2. Reporting own message is prevented
	_, err = ReportAbuse(2, 2, &conv.ID, &msg.ID, "Spam", "")
	if err == nil || !strings.Contains(err.Error(), "cannot report your own message") {
		t.Errorf("Expected cannot report your own message error, got %v", err)
	}

	// 3. Valid report with message evidence snapshotting
	report, err := ReportAbuse(1, 2, &conv.ID, &msg.ID, "Inappropriate Content", "Offensive language used in fellowship chat")
	if err != nil {
		t.Fatalf("ReportAbuse failed: %v", err)
	}
	if report.ID <= 0 {
		t.Errorf("Expected positive report ID, got %d", report.ID)
	}
	if report.EvidenceText != "Inappropriate test message" {
		t.Errorf("Expected EvidenceText 'Inappropriate test message', got '%s'", report.EvidenceText)
	}
	if report.Status != "pending" {
		t.Errorf("Expected pending status, got '%s'", report.Status)
	}

	// 4. Duplicate report prevention: submitting duplicate pending report should be rejected
	_, err = ReportAbuse(1, 2, &conv.ID, &msg.ID, "Inappropriate Content", "Duplicate submission attempt")
	if err == nil || !strings.Contains(err.Error(), "already pending moderation review") {
		t.Errorf("Expected duplicate pending review error, got: %v", err)
	}
}

func TestModerationReportsWorkflow(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Set user 1 as regular member, user 2 as admin
	_, _ = database.DB.Exec("UPDATE users SET role = 'member' WHERE id = 1")
	_, _ = database.DB.Exec("UPDATE users SET role = 'admin' WHERE id = 2")

	conv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	msg, err := SendMessage(conv.ID, 1, "Offensive message to delete", "cmsg_mod_1", nil)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	report, err := ReportAbuse(2, 1, &conv.ID, &msg.ID, "Harassment or Bullying", "Harassing fellow youth member")
	if err != nil {
		t.Fatalf("ReportAbuse failed: %v", err)
	}

	// 1. Regular user (User 1) cannot view moderation reports
	_, _, err = GetChatReportsForModeration(1, "pending", 10, 0)
	if err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Errorf("Expected access denied for non-moderator, got: %v", err)
	}

	// 2. Admin (User 2) can view moderation reports
	reports, total, err := GetChatReportsForModeration(2, "pending", 10, 0)
	if err != nil {
		t.Fatalf("GetChatReportsForModeration failed for admin: %v", err)
	}
	if total < 1 || len(reports) < 1 {
		t.Fatalf("Expected at least 1 pending report, got total=%d len=%d", total, len(reports))
	}
	if reports[0].ID != report.ID {
		t.Errorf("Expected report ID %d, got %d", report.ID, reports[0].ID)
	}

	// 3. Regular user cannot resolve reports
	err = ResolveChatReport(1, report.ID, "resolved", "delete_message", "Unauthorized resolution")
	if err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Errorf("Expected access denied for non-moderator resolving report, got: %v", err)
	}

	// 4. Admin resolves report with delete_message action
	err = ResolveChatReport(2, report.ID, "resolved", "delete_message", "Message violates community rules")
	if err != nil {
		t.Fatalf("ResolveChatReport failed: %v", err)
	}

	// Verify message was soft-deleted
	reloadedMsg, err := repository.GetMessageByID(msg.ID)
	if err != nil {
		t.Fatalf("GetMessageByID failed: %v", err)
	}
	if !reloadedMsg.IsDeleted {
		t.Errorf("Expected message to be marked IsDeleted=true after moderation action")
	}

	// 5. Submit second report and test warn_user resolution
	msg2, _ := SendMessage(conv.ID, 1, "Second offensive message", "cmsg_mod_2", nil)
	report2, err := ReportAbuse(2, 1, &conv.ID, &msg2.ID, "Inappropriate Content", "Second offense")
	if err != nil {
		t.Fatalf("ReportAbuse for second report failed: %v", err)
	}

	err = ResolveChatReport(2, report2.ID, "resolved", "warn_user", "Issued community guideline warning")
	if err != nil {
		t.Fatalf("ResolveChatReport with warn_user failed: %v", err)
	}

	// Verify warning notification was dispatched to reported user (User 1)
	notifications, err := repository.GetNotificationsByUser(1)
	if err != nil {
		t.Fatalf("GetNotificationsByUser failed: %v", err)
	}
	foundWarning := false
	for _, n := range notifications {
		if n.Type == "system_warning" && strings.Contains(n.Message, "official moderation warning") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("Expected system_warning notification to be delivered to User 1")
	}
}

func TestBlockUserAndReciprocalEnforcement(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Cannot block self
	err := BlockUser(1, 1, "Self block")
	if err == nil || !strings.Contains(err.Error(), "cannot block yourself") {
		t.Errorf("Expected cannot block yourself error, got: %v", err)
	}

	// 2. Block member (User 1 blocks User 2)
	err = BlockUser(1, 2, "Unwanted messages")
	if err != nil {
		t.Fatalf("BlockUser failed: %v", err)
	}

	// 3. Directional block check
	blockedByUser1, err := IsUserBlocked(1, 2)
	if err != nil || !blockedByUser1 {
		t.Errorf("Expected User 1 to have blocked User 2, got %v, err=%v", blockedByUser1, err)
	}
	blockedByUser2, err := IsUserBlocked(2, 1)
	if err != nil || blockedByUser2 {
		t.Errorf("Expected User 2 NOT to have blocked User 1, got %v, err=%v", blockedByUser2, err)
	}

	// 4. Reciprocal communication block check
	commBlocked1, err := IsCommunicationBlocked(1, 2)
	if err != nil || !commBlocked1 {
		t.Errorf("Expected communication to be blocked (1->2)")
	}
	commBlocked2, err := IsCommunicationBlocked(2, 1)
	if err != nil || !commBlocked2 {
		t.Errorf("Expected communication to be blocked (2->1)")
	}

	// 5. GetBlockedUsers verification
	blockedUsers, err := GetBlockedUsers(1)
	if err != nil {
		t.Fatalf("GetBlockedUsers failed: %v", err)
	}
	if len(blockedUsers) != 1 || blockedUsers[0].ID != 2 {
		t.Errorf("Expected User 2 in User 1's blocked list, got %+v", blockedUsers)
	}

	// 6. Direct messaging enforcement: SendMessage fails when blocked
	conv, _ := repository.CreateOrGetDirectConversation(1, 2)
	_, err = SendMessage(conv.ID, 2, "Should be blocked", "cmsg_blocked", nil)
	if err == nil || !strings.Contains(err.Error(), "communication between these members is blocked") {
		t.Errorf("Expected communication blocked error for SendMessage, got: %v", err)
	}

	// 7. Notification suppression between blocked users
	notifErr := CreateNotification(models.Notification{
		SenderID:   2,
		ReceiverID: 1,
		Type:       "follow",
		Message:    "started following you",
	})
	if notifErr != nil {
		t.Fatalf("CreateNotification errored unexpectedly: %v", notifErr)
	}
	// Verify notification was silently dropped and NOT inserted into database
	recipientNotifs, _ := repository.GetNotificationsByUser(1)
	for _, n := range recipientNotifs {
		if n.SenderID == 2 {
			t.Errorf("Expected notification from blocked user to be suppressed, but found: %+v", n)
		}
	}

	// 8. Unblock contact restores communication
	err = UnblockUser(1, 2)
	if err != nil {
		t.Fatalf("UnblockUser failed: %v", err)
	}

	stillBlocked, err := IsCommunicationBlocked(1, 2)
	if err != nil || stillBlocked {
		t.Errorf("Expected communication to no longer be blocked after unblock, got %v", stillBlocked)
	}

	// Now message can be sent successfully
	msg, err := SendMessage(conv.ID, 2, "Unblocked message", "cmsg_unblocked", nil)
	if err != nil {
		t.Errorf("Expected SendMessage to succeed after unblock, got: %v", err)
	}
	if msg == nil || msg.ID <= 0 {
		t.Errorf("Expected valid message after unblock")
	}
}

func TestSetConversationMute_DurationsAndDynamicEvaluation(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	// 1. Outsider cannot mute conversation
	_, _, err = SetConversationMute(conv.ID, 3, "1h")
	if err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Errorf("Expected access denied for non-member, got: %v", err)
	}

	// 2. Invalid mute option rejected
	_, _, err = SetConversationMute(conv.ID, 1, "invalid_option")
	if err == nil || !strings.Contains(err.Error(), "invalid mute option") {
		t.Errorf("Expected invalid mute option error, got: %v", err)
	}

	// 3. Option "1h"
	mutedUntil, isMuted, err := SetConversationMute(conv.ID, 1, "1h")
	if err != nil {
		t.Fatalf("SetConversationMute 1h failed: %v", err)
	}
	if !isMuted || mutedUntil == nil {
		t.Errorf("Expected isMuted=true and valid mutedUntil, got isMuted=%v, until=%v", isMuted, mutedUntil)
	}
	if mutedUntil.Before(time.Now().UTC().Add(50 * time.Minute)) {
		t.Errorf("Expected mutedUntil to be approx 1 hour in future, got %v", mutedUntil)
	}
	if !repository.IsConversationMutedBy(conv.ID, 1) {
		t.Errorf("Expected IsConversationMutedBy to return true for user 1")
	}
	if repository.IsConversationMutedBy(conv.ID, 2) {
		t.Errorf("Expected IsConversationMutedBy to return false for user 2 (mute is per-user)")
	}

	// 4. Option "8h"
	mutedUntil, isMuted, err = SetConversationMute(conv.ID, 1, "8h")
	if err != nil || !isMuted || mutedUntil == nil || mutedUntil.Before(time.Now().UTC().Add(7*time.Hour)) {
		t.Errorf("SetConversationMute 8h failed or invalid until: %v, %v", err, mutedUntil)
	}

	// 5. Option "1w"
	mutedUntil, isMuted, err = SetConversationMute(conv.ID, 1, "1w")
	if err != nil || !isMuted || mutedUntil == nil || mutedUntil.Before(time.Now().UTC().Add(6*24*time.Hour)) {
		t.Errorf("SetConversationMute 1w failed or invalid until: %v, %v", err, mutedUntil)
	}

	// 6. Option "indefinite"
	mutedUntil, isMuted, err = SetConversationMute(conv.ID, 1, "indefinite")
	if err != nil {
		t.Fatalf("SetConversationMute indefinite failed: %v", err)
	}
	if !isMuted || mutedUntil != nil {
		t.Errorf("Expected isMuted=true and mutedUntil=nil for indefinite, got %v, %v", isMuted, mutedUntil)
	}
	if !repository.IsConversationMutedBy(conv.ID, 1) {
		t.Errorf("Expected IsConversationMutedBy to return true for indefinite mute")
	}

	// 7. Dynamic evaluation: Simulate expired mute timestamp in database
	_, err = database.DB.Exec("UPDATE conversation_members SET is_muted = 1, muted_until = datetime('now', '-10 minutes') WHERE conversation_id = ? AND user_id = 1", conv.ID)
	if err != nil {
		t.Fatalf("Failed to simulate expired mute in DB: %v", err)
	}
	if repository.IsConversationMutedBy(conv.ID, 1) {
		t.Errorf("Expected IsConversationMutedBy to return false for expired mute timestamp")
	}

	// 8. Option "unmute"
	mutedUntil, isMuted, err = SetConversationMute(conv.ID, 1, "unmute")
	if err != nil {
		t.Fatalf("SetConversationMute unmute failed: %v", err)
	}
	if isMuted || mutedUntil != nil {
		t.Errorf("Expected isMuted=false and mutedUntil=nil, got %v, %v", isMuted, mutedUntil)
	}
	if repository.IsConversationMutedBy(conv.ID, 1) {
		t.Errorf("Expected IsConversationMutedBy to return false after unmute")
	}

	// 9. Notification suppression while actively muted, and restoration after unmute
	// Actively mute for user 2
	_, _, err = SetConversationMute(conv.ID, 2, "1h")
	if err != nil {
		t.Fatalf("SetConversationMute for user 2 failed: %v", err)
	}

	// User 1 sends message -> notification for User 2 should be suppressed
	_, err = SendMessage(conv.ID, 1, "Message during active mute", "cmsg_mute_suppress", nil)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	notifs, _ := repository.GetNotificationsByUser(2)
	for _, n := range notifs {
		if strings.Contains(n.Message, "Message during active mute") {
			t.Errorf("Notification should have been suppressed while conversation is muted, but was found: %+v", n)
		}
	}

	// Unmute user 2
	_, _, err = SetConversationMute(conv.ID, 2, "unmute")
	if err != nil {
		t.Fatalf("Unmute for user 2 failed: %v", err)
	}

	// User 1 sends another message -> notification for User 2 should be delivered
	_, err = SendMessage(conv.ID, 1, "Message after unmuting", "cmsg_mute_restore", nil)
	if err != nil {
		t.Fatalf("SendMessage after unmute failed: %v", err)
	}

	notifsAfter, _ := repository.GetNotificationsByUser(2)
	foundDelivered := false
	for _, n := range notifsAfter {
		if n.Type == "message" && n.SenderID == 1 && n.ReceiverID == 2 {
			foundDelivered = true
			break
		}
	}
	if !foundDelivered {
		t.Errorf("Expected notification to be delivered after unmuting, but none was found")
	}
}

func TestDisappearingMessages_ConfigurationAndPermissions(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Direct conversation: Both participants can configure disappearing timer
	conv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	// User 1 sets 24 hours (86400)
	updatedConv, err := SetDisappearingMessages(1, conv.ID, 86400)
	if err != nil {
		t.Fatalf("SetDisappearingMessages 86400 failed: %v", err)
	}
	if updatedConv.DisappearingDuration != 86400 {
		t.Errorf("Expected duration 86400, got %d", updatedConv.DisappearingDuration)
	}
	if updatedConv.FormattedDisappearingDuration() != "24 hours" {
		t.Errorf("Expected label '24 hours', got %q", updatedConv.FormattedDisappearingDuration())
	}

	// Verify system notice was logged in conversation messages
	msgs, err := GetConversationMessages(conv.ID, 1, 50, 0)
	if err != nil || len(msgs) == 0 {
		t.Fatalf("Failed to fetch conversation messages: %v", err)
	}
	lastMsg := msgs[len(msgs)-1]
	if lastMsg.MessageType != "system" || !strings.Contains(lastMsg.Body, "disappear") || !strings.Contains(lastMsg.Body, "24 hours") {
		t.Errorf("Expected system notice for timer change, got: %+v", lastMsg)
	}

	// User 2 turns timer Off (0)
	updatedConv2, err := SetDisappearingMessages(2, conv.ID, 0)
	if err != nil {
		t.Fatalf("SetDisappearingMessages 0 failed: %v", err)
	}
	if updatedConv2.DisappearingDuration != 0 {
		t.Errorf("Expected duration 0, got %d", updatedConv2.DisappearingDuration)
	}

	// 2. Group conversation: Only group admins and owners can configure
	group, err := CreateGroup(1, "Youth Bible Group", "bible_study", "Study circle", "all", true, []int{2})
	if err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}
	// Add User 3 as regular member
	err = AddGroupMembers(1, group.ID, []int{3})
	if err != nil {
		t.Fatalf("AddGroupMembers failed: %v", err)
	}

	// Regular member (User 3) cannot change timer
	_, err = SetDisappearingMessages(3, group.ID, 604800)
	if err == nil || !strings.Contains(err.Error(), "only group administrators") {
		t.Errorf("Expected admin restriction error for regular member, got: %v", err)
	}

	// Group owner (User 1) can change timer to 7 days (604800)
	updatedGroup, err := SetDisappearingMessages(1, group.ID, 604800)
	if err != nil {
		t.Fatalf("SetDisappearingMessages by owner failed: %v", err)
	}
	if updatedGroup.DisappearingDuration != 604800 {
		t.Errorf("Expected 604800, got %d", updatedGroup.DisappearingDuration)
	}

	// Promote User 2 to admin
	err = UpdateGroupMemberRole(1, group.ID, 2, "admin")
	if err != nil {
		t.Fatalf("UpdateGroupMemberRole to admin failed: %v", err)
	}

	// Promoted admin (User 2) can change timer to 90 days (7776000)
	updatedGroup2, err := SetDisappearingMessages(2, group.ID, 7776000)
	if err != nil {
		t.Fatalf("SetDisappearingMessages by admin failed: %v", err)
	}
	if updatedGroup2.DisappearingDuration != 7776000 {
		t.Errorf("Expected 7776000, got %d", updatedGroup2.DisappearingDuration)
	}

	// 3. Invalid duration option rejected
	_, err = SetDisappearingMessages(1, group.ID, 12345)
	if err == nil || !strings.Contains(err.Error(), "invalid disappearing duration") {
		t.Errorf("Expected invalid duration error, got: %v", err)
	}
}

func TestDisappearingMessages_ExpirationFilteringAndPurge(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	// Enable 24-hour disappearing messages
	_, err = SetDisappearingMessages(1, conv.ID, 86400)
	if err != nil {
		t.Fatalf("SetDisappearingMessages failed: %v", err)
	}

	// 1. Send active message -> verify ExpiresAt is auto-computed
	msgActive, err := SendMessage(conv.ID, 1, "Active message with 24h timer", "cmsg_active_1", nil)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if msgActive.ExpiresAt == nil {
		t.Fatalf("Expected msgActive.ExpiresAt to be set automatically")
	}
	if msgActive.ExpiresAt.Before(time.Now().UTC().Add(23 * time.Hour)) {
		t.Errorf("Expected ExpiresAt to be ~24 hours ahead, got %v", msgActive.ExpiresAt)
	}
	if msgActive.IsExpired() {
		t.Errorf("Active message should not be expired")
	}

	// 2. Manually insert an expired message into database
	pastTime := time.Now().UTC().Add(-10 * time.Minute)
	res, err := database.DB.Exec(`
		INSERT INTO messages (conversation_id, sender_id, message_type, body, expires_at, created_at)
		VALUES (?, 1, 'text', 'Expired message content', ?, ?)
	`, conv.ID, pastTime, pastTime)
	if err != nil {
		t.Fatalf("Failed to insert expired test message: %v", err)
	}
	expiredMsgID, _ := res.LastInsertId()

	// 3. GetConversationMessages must filter out the expired message
	msgs, err := GetConversationMessages(conv.ID, 1, 50, 0)
	if err != nil {
		t.Fatalf("GetConversationMessages failed: %v", err)
	}
	for _, m := range msgs {
		if m.ID == int(expiredMsgID) {
			t.Errorf("GetConversationMessages returned expired message ID %d", m.ID)
		}
	}

	// 4. SearchMessages must filter out the expired message
	searchResults, err := SearchMessages(2, "Expired", &conv.ID, 50, 0)
	if err != nil {
		t.Fatalf("SearchMessages failed: %v", err)
	}
	for _, m := range searchResults {
		if m.ID == int(expiredMsgID) {
			t.Errorf("SearchMessages returned expired message ID %d", m.ID)
		}
	}

	// 5. Unread count query must not count the expired message
	unreadCount, err := repository.CountTotalUnreadMessages(2)
	if err != nil {
		t.Fatalf("CountTotalUnreadMessages failed: %v", err)
	}
	// Active message is unread, expired message is not counted
	if unreadCount < 1 {
		t.Errorf("Expected at least 1 unread message for active message, got %d", unreadCount)
	}

	// 6. Report Abuse before deletion -> Preserves snapshot in moderation report
	report, err := ReportAbuse(2, 1, &conv.ID, &msgActive.ID, "Inappropriate Content", "Test snapshot preservation")
	if err != nil {
		t.Fatalf("ReportAbuse failed: %v", err)
	}
	if report.EvidenceText == "" {
		t.Errorf("Expected moderation report to capture snapshot in EvidenceText")
	}

	// 7. Test DeleteExpiredMessages
	deletedCount, err := repository.DeleteExpiredMessages()
	if err != nil {
		t.Fatalf("DeleteExpiredMessages failed: %v", err)
	}
	if deletedCount < 1 {
		t.Errorf("Expected at least 1 message to be deleted, got %d", deletedCount)
	}

	// Verify row was physically removed from DB
	var rowCount int
	_ = database.DB.QueryRow("SELECT COUNT(*) FROM messages WHERE id = ?", expiredMsgID).Scan(&rowCount)
	if rowCount != 0 {
		t.Errorf("Expected expired message to be removed from DB, found count %d", rowCount)
	}

	// Verify active message was NOT deleted
	var activeRowCount int
	_ = database.DB.QueryRow("SELECT COUNT(*) FROM messages WHERE id = ?", msgActive.ID).Scan(&activeRowCount)
	if activeRowCount != 1 {
		t.Errorf("Expected active message to remain in DB, found count %d", activeRowCount)
	}

	// Verify moderation report evidence remains intact
	reloadedReport, err := repository.GetChatReportByID(report.ID)
	if err != nil || reloadedReport == nil {
		t.Fatalf("Failed to reload moderation report: %v", err)
	}
	if reloadedReport.EvidenceText != "Active message with 24h timer" {
		t.Errorf("Expected evidence text to be preserved, got %q", reloadedReport.EvidenceText)
	}
}

func TestClearChatAndMessageVisibility(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	// 1. Send messages
	_, err = SendMessage(conv.ID, 1, "First message from User 1", "", nil)
	if err != nil {
		t.Fatalf("SendMessage 1 failed: %v", err)
	}
	_, err = SendMessage(conv.ID, 2, "Second message from User 2", "", nil)
	if err != nil {
		t.Fatalf("SendMessage 2 failed: %v", err)
	}

	// 2. Unauthorized clear
	err = ClearChat(conv.ID, 3)
	if err == nil {
		t.Errorf("Expected unauthorized error for User 3, got nil")
	}

	// Small pause so clear timestamp is recorded
	time.Sleep(100 * time.Millisecond)

	// 3. User 1 clears chat
	err = ClearChat(conv.ID, 1)
	if err != nil {
		t.Fatalf("ClearChat failed for User 1: %v", err)
	}

	// Wait 1100ms so that SQLite CURRENT_TIMESTAMP second advances past cleared_at
	time.Sleep(1100 * time.Millisecond)

	// 4. User 2 sends a new message after clear
	msg3, err := SendMessage(conv.ID, 2, "Third message after clear", "", nil)
	if err != nil {
		t.Fatalf("SendMessage 3 failed: %v", err)
	}

	// 5. User 1 should ONLY see messages after cleared_at (msg3)
	msgsUser1, err := GetConversationMessages(conv.ID, 1, 50, 0)
	if err != nil {
		t.Fatalf("GetConversationMessages User 1 failed: %v", err)
	}
	if len(msgsUser1) != 1 {
		t.Errorf("Expected 1 message for User 1 after clear, got %d", len(msgsUser1))
	} else if msgsUser1[0].ID != msg3.ID {
		t.Errorf("Expected msg3 ID %d, got %d", msg3.ID, msgsUser1[0].ID)
	}

	// 6. User 2 should see ALL 3 messages
	msgsUser2, err := GetConversationMessages(conv.ID, 2, 50, 0)
	if err != nil {
		t.Fatalf("GetConversationMessages User 2 failed: %v", err)
	}
	if len(msgsUser2) != 3 {
		t.Errorf("Expected 3 messages for User 2 (clear should not affect them), got %d", len(msgsUser2))
	}
}

func TestTogglePinConversation(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	// 1. Unauthorized toggle
	_, err = TogglePinConversation(conv.ID, 3)
	if err == nil {
		t.Errorf("Expected unauthorized error for User 3, got nil")
	}

	// 2. User 1 pins conversation
	isPinned, err := TogglePinConversation(conv.ID, 1)
	if err != nil {
		t.Fatalf("TogglePinConversation failed: %v", err)
	}
	if !isPinned {
		t.Errorf("Expected isPinned = true, got false")
	}

	// Verify User 1 inbox has it marked as pinned
	inbox, _, err := GetUserInbox(1)
	if err != nil {
		t.Fatalf("GetUserInbox failed: %v", err)
	}
	if len(inbox) == 0 || !inbox[0].IsPinned {
		t.Errorf("Expected conversation to be pinned in User 1 inbox")
	}

	// User 2 inbox should NOT have it pinned (pin is per-member)
	inbox2, _, err := GetUserInbox(2)
	if err != nil {
		t.Fatalf("GetUserInbox User 2 failed: %v", err)
	}
	if len(inbox2) == 0 || inbox2[0].IsPinned {
		t.Errorf("Expected conversation NOT to be pinned in User 2 inbox")
	}

	// 3. User 1 unpins conversation
	isPinned, err = TogglePinConversation(conv.ID, 1)
	if err != nil {
		t.Fatalf("TogglePinConversation unpin failed: %v", err)
	}
	if isPinned {
		t.Errorf("Expected isPinned = false after unpin, got true")
	}
}

func TestDeleteMessageForMe(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	msg, err := SendMessage(conv.ID, 1, "Confidential note", "", nil)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// 1. Unauthorized delete for me (User 3)
	err = DeleteMessageForMe(3, msg.ID)
	if err == nil {
		t.Errorf("Expected unauthorized error for User 3, got nil")
	}

	// 2. User 2 deletes message for me
	err = DeleteMessageForMe(2, msg.ID)
	if err != nil {
		t.Fatalf("DeleteMessageForMe failed: %v", err)
	}

	// 3. Verify User 2 cannot see the message anymore
	msgsUser2, err := GetConversationMessages(conv.ID, 2, 50, 0)
	if err != nil {
		t.Fatalf("GetConversationMessages User 2 failed: %v", err)
	}
	if len(msgsUser2) != 0 {
		t.Errorf("Expected 0 messages for User 2, got %d", len(msgsUser2))
	}

	// 4. Verify User 1 STILL sees the message intact
	msgsUser1, err := GetConversationMessages(conv.ID, 1, 50, 0)
	if err != nil {
		t.Fatalf("GetConversationMessages User 1 failed: %v", err)
	}
	if len(msgsUser1) != 1 || msgsUser1[0].Body != "Confidential note" {
		t.Errorf("Expected message to remain visible for User 1")
	}
}

func TestExportChat(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	_, err = SendMessage(conv.ID, 1, "Meeting transcript entry", "", nil)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// 1. Unauthorized export (User 3)
	_, _, err = ExportChat(conv.ID, 3, "txt")
	if err == nil {
		t.Errorf("Expected unauthorized error for User 3, got nil")
	}

	// 2. Export as TXT
	filename, data, err := ExportChat(conv.ID, 1, "txt")
	if err != nil {
		t.Fatalf("ExportChat TXT failed: %v", err)
	}
	if !strings.HasSuffix(filename, ".txt") {
		t.Errorf("Expected .txt suffix, got %s", filename)
	}
	txtContent := string(data)
	if !strings.Contains(txtContent, "Meeting transcript entry") {
		t.Errorf("Expected transcript content to include message, got %s", txtContent)
	}

	// 3. Export as JSON
	filenameJSON, dataJSON, err := ExportChat(conv.ID, 1, "json")
	if err != nil {
		t.Fatalf("ExportChat JSON failed: %v", err)
	}
	if !strings.HasSuffix(filenameJSON, ".json") {
		t.Errorf("Expected .json suffix, got %s", filenameJSON)
	}
	jsonContent := string(dataJSON)
	if !strings.Contains(jsonContent, "Meeting transcript entry") || !strings.Contains(jsonContent, `"messages"`) {
		t.Errorf("Expected valid JSON export payload, got %s", jsonContent)
	}
}

func TestExtractMentionsAndNotifications(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Unit test extractMentions
	mentions := extractMentions("Hello @sender and @receiver, please check this out! Also @sender again.")
	if len(mentions) != 2 {
		t.Fatalf("Expected 2 unique mentions, got %d: %v", len(mentions), mentions)
	}
	if mentions[0] != "sender" || mentions[1] != "receiver" {
		t.Errorf("Expected [sender, receiver], got %v", mentions)
	}

	// 2. Group conversation with mention notification
	groupConv, err := CreateGroup(1, "Youth Council", "general", "Discussion", "all", true, []int{2, 3})
	if err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}

	// User 1 sends message mentioning receiver
	_, err = SendMessage(groupConv.ID, 1, "Hey @receiver, please take notes!", "", nil)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// Verify notification was recorded in notifications table for User 2 (receiver)
	var notifCount int
	err = database.DB.QueryRow("SELECT COUNT(*) FROM notifications WHERE receiver_id = ? AND type = 'mention'", 2).Scan(&notifCount)
	if err != nil {
		t.Fatalf("Failed to query notifications: %v", err)
	}
	if notifCount < 1 {
		t.Errorf("Expected at least 1 mention notification for User 2, got %d", notifCount)
	}
}

func TestQuotedReplyMessage(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := StartDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("StartDirectConversation failed: %v", err)
	}

	// 1. Send parent message
	parentMsg, err := SendMessage(conv.ID, 1, "Original query", "", nil)
	if err != nil {
		t.Fatalf("SendMessage parent failed: %v", err)
	}

	// 2. Send reply quoting parent
	replyMsg, err := SendMessage(conv.ID, 2, "Quoted response", "", &parentMsg.ID)
	if err != nil {
		t.Fatalf("SendMessage reply failed: %v", err)
	}
	if replyMsg.ReplyToMessageID == nil || *replyMsg.ReplyToMessageID != parentMsg.ID {
		t.Fatalf("Expected reply_to_message_id %d, got %v", parentMsg.ID, replyMsg.ReplyToMessageID)
	}
	if replyMsg.ReplyToMessage == nil || replyMsg.ReplyToMessage.Body != "Original query" {
		t.Errorf("Expected hydrated reply_to_message with body 'Original query'")
	}

	// 3. Fetch conversation messages and verify batch-hydration of ReplyToMessage
	msgs, err := GetConversationMessages(conv.ID, 1, 50, 0)
	if err != nil {
		t.Fatalf("GetConversationMessages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(msgs))
	}
	foundReply := false
	for _, m := range msgs {
		if m.ID == replyMsg.ID {
			foundReply = true
			if m.ReplyToMessage == nil || m.ReplyToMessage.Body != "Original query" {
				t.Errorf("Expected ReplyToMessage to be hydrated in GetConversationMessages, got %v", m.ReplyToMessage)
			}
		}
	}
	if !foundReply {
		t.Errorf("Reply message was not found in conversation messages")
	}
}



