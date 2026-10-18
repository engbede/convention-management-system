package services

import (
	"bytes"
	"database/sql"
	"mime/multipart"
	"os"
	"strings"
	"testing"

	"convention-management-system/database"
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
