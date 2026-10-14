package repository

import (
	"database/sql"
	"os"
	"testing"

	"convention-management-system/database"
	"convention-management-system/models"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) func() {
	t.Helper()
	testDBPath := "test_convention.db"
	_ = os.Remove(testDBPath)

	db, err := sql.Open("sqlite3", testDBPath)
	if err != nil {
		t.Fatalf("Failed to open test db: %v", err)
	}
	database.DB = db

	// Run SQLite migrations
	if err := database.MigrateSQLite(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Insert test users
	_, err = db.Exec(`
		INSERT INTO users (id, full_name, username, password_hash)
		VALUES 
			(1, 'Alice Test', 'alice', 'hash1'),
			(2, 'Bob Test', 'bob', 'hash2'),
			(3, 'Charlie Test', 'charlie', 'hash3');
	`)
	if err != nil {
		t.Fatalf("Failed to insert test users: %v", err)
	}

	return func() {
		db.Close()
		_ = os.Remove(testDBPath)
	}
}

func TestCreateOrGetDirectConversation(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Cannot chat with self
	_, err := CreateOrGetDirectConversation(1, 1)
	if err == nil {
		t.Errorf("Expected error when creating conversation with self, got nil")
	}

	// 2. Create direct conversation between user 1 and user 2
	conv1, err := CreateOrGetDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("Failed to create direct conversation: %v", err)
	}
	if conv1.ID <= 0 {
		t.Fatalf("Expected valid conversation ID, got %d", conv1.ID)
	}

	// 3. Verify deduplication: calling again with (2, 1) should return the SAME conversation
	conv2, err := CreateOrGetDirectConversation(2, 1)
	if err != nil {
		t.Fatalf("Failed to get existing direct conversation: %v", err)
	}
	if conv2.ID != conv1.ID {
		t.Errorf("Expected conversation deduplication ID %d, got %d", conv1.ID, conv2.ID)
	}

	// 4. Verify membership
	isMember1, _ := IsConversationMember(conv1.ID, 1)
	isMember2, _ := IsConversationMember(conv1.ID, 2)
	isMember3, _ := IsConversationMember(conv1.ID, 3)

	if !isMember1 || !isMember2 {
		t.Errorf("Participants 1 and 2 should be members of conversation %d", conv1.ID)
	}
	if isMember3 {
		t.Errorf("User 3 should NOT be a member of conversation %d", conv1.ID)
	}
}

func TestSaveAndRetrieveMessages(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	conv, err := CreateOrGetDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("Setup conversation failed: %v", err)
	}

	// 1. Save a message
	msg := &models.Message{
		ConversationID:  conv.ID,
		SenderID:        1,
		MessageType:     "text",
		Body:            "Hello Bob!",
		ClientMessageID: "client-uuid-1",
	}

	if err := SaveMessage(msg); err != nil {
		t.Fatalf("SaveMessage failed: %v", err)
	}
	if msg.ID <= 0 {
		t.Errorf("Expected positive message ID, got %d", msg.ID)
	}

	// 2. Test Idempotency with same client_message_id
	duplicateMsg := &models.Message{
		ConversationID:  conv.ID,
		SenderID:        1,
		MessageType:     "text",
		Body:            "Hello Bob!",
		ClientMessageID: "client-uuid-1",
	}
	if err := SaveMessage(duplicateMsg); err != nil {
		t.Fatalf("SaveMessage duplicate should succeed idempotently: %v", err)
	}
	if duplicateMsg.ID != msg.ID {
		t.Errorf("Expected duplicate message to return existing ID %d, got %d", msg.ID, duplicateMsg.ID)
	}

	// 3. Retrieve messages
	msgs, err := GetConversationMessages(conv.ID, 10, 0, 1)
	if err != nil {
		t.Fatalf("GetConversationMessages failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Body != "Hello Bob!" {
		t.Errorf("Expected body 'Hello Bob!', got %q", msgs[0].Body)
	}
	if msgs[0].Sender.Username != "alice" {
		t.Errorf("Expected sender username 'alice', got %q", msgs[0].Sender.Username)
	}
	if msgs[0].IsReadByRecipient {
		t.Errorf("Expected IsReadByRecipient to be false before Bob reads message")
	}

	// 4. Test unread count: Bob (user 2) has 1 unread message
	unreadBob, err := CountTotalUnreadMessages(2)
	if err != nil {
		t.Fatalf("CountTotalUnreadMessages failed: %v", err)
	}
	if unreadBob != 1 {
		t.Errorf("Expected Bob to have 1 unread message, got %d", unreadBob)
	}

	// 5. Mark as read by Bob
	if err := MarkConversationAsRead(conv.ID, 2); err != nil {
		t.Fatalf("MarkConversationAsRead failed: %v", err)
	}
	unreadBobAfter, _ := CountTotalUnreadMessages(2)
	if unreadBobAfter != 0 {
		t.Errorf("Expected Bob to have 0 unread messages after reading, got %d", unreadBobAfter)
	}

	// 6. Test Read Receipt: Now Alice should see message marked as read
	msgsAfter, err := GetConversationMessages(conv.ID, 10, 0, 1)
	if err != nil {
		t.Fatalf("GetConversationMessages failed: %v", err)
	}
	if !msgsAfter[0].IsReadByRecipient {
		t.Errorf("Expected IsReadByRecipient to be true after Bob read the message")
	}

	// 7. Test Mute and Archive toggle
	isMuted, err := ToggleMuteConversation(conv.ID, 1)
	if err != nil || !isMuted {
		t.Fatalf("Expected conversation to be muted: %v", err)
	}
	isMutedAgain, err := ToggleMuteConversation(conv.ID, 1)
	if err != nil || isMutedAgain {
		t.Fatalf("Expected conversation to be unmuted: %v", err)
	}

	isArchived, err := ToggleArchiveConversation(conv.ID, 1)
	if err != nil || !isArchived {
		t.Fatalf("Expected conversation to be archived: %v", err)
	}

	// 8. Test CountMessagesInConversation
	totalCount, err := CountMessagesInConversation(conv.ID)
	if err != nil || totalCount != 1 {
		t.Fatalf("Expected total 1 message, got %d (err: %v)", totalCount, err)
	}
}

