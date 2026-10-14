package services

import (
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"convention-management-system/database"

	_ "github.com/mattn/go-sqlite3"
)

func setupHubTestDB(t *testing.T) func() {
	t.Helper()
	origWd, _ := os.Getwd()
	if strings.HasSuffix(origWd, "services") {
		_ = os.Chdir("..")
	}

	testDBPath := "test_hub.db"
	_ = os.Remove(testDBPath)

	db, err := sql.Open("sqlite3", testDBPath)
	if err != nil {
		t.Fatalf("Failed to open test db: %v", err)
	}
	database.DB = db

	if err := database.MigrateSQLite(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Seed users and conversation
	_, err = db.Exec(`
		INSERT INTO users (id, full_name, username, password_hash)
		VALUES 
			(1, 'Alice Walker', 'alice', 'hash1'),
			(2, 'Bob Smith', 'bob', 'hash2'),
			(3, 'Charlie Brown', 'charlie', 'hash3');

		INSERT INTO conversations (id, type, created_by)
		VALUES (1, 'direct', 1);

		INSERT INTO conversation_members (conversation_id, user_id, role)
		VALUES 
			(1, 1, 'member'),
			(1, 2, 'member');
	`)
	if err != nil {
		t.Fatalf("Failed to seed database: %v", err)
	}

	return func() {
		db.Close()
		_ = os.Remove(testDBPath)
		_ = os.Chdir(origWd)
	}
}

func TestChatHubPresenceAndRegistration(t *testing.T) {
	hub := NewChatHub()

	// Initially offline
	if hub.IsUserOnline(1) {
		t.Errorf("Expected user 1 to be offline initially")
	}

	client1 := &WSClient{
		UserID:   1,
		Username: "alice",
		FullName: "Alice Walker",
	}

	// Register client
	hub.Register(client1)
	if !hub.IsUserOnline(1) {
		t.Errorf("Expected user 1 to be online after registration")
	}

	// Register a second tab for the same user
	client1Tab2 := &WSClient{
		UserID:   1,
		Username: "alice",
		FullName: "Alice Walker",
	}
	hub.Register(client1Tab2)
	if !hub.IsUserOnline(1) {
		t.Errorf("Expected user 1 to still be online with 2 tabs")
	}

	// Unregister 1 tab
	hub.Unregister(client1Tab2)
	if !hub.IsUserOnline(1) {
		t.Errorf("Expected user 1 to still be online with 1 remaining tab")
	}

	// Unregister final tab
	hub.Unregister(client1)
	if hub.IsUserOnline(1) {
		t.Errorf("Expected user 1 to be offline after unregistering all tabs")
	}
}

func TestChatHubSubscribeAndAuthorization(t *testing.T) {
	cleanup := setupHubTestDB(t)
	defer cleanup()

	hub := NewChatHub()

	// Alice (user 1) is a member of conversation 1
	aliceClient := &WSClient{
		UserID:   1,
		Username: "alice",
	}
	if err := hub.Subscribe(aliceClient, 1); err != nil {
		t.Fatalf("Alice should successfully subscribe to conversation 1: %v", err)
	}
	if aliceClient.SubscribedConvID != 1 {
		t.Errorf("Expected SubscribedConvID to be 1, got %d", aliceClient.SubscribedConvID)
	}

	// Charlie (user 3) is NOT a member of conversation 1
	charlieClient := &WSClient{
		UserID:   3,
		Username: "charlie",
	}
	if err := hub.Subscribe(charlieClient, 1); err == nil {
		t.Fatalf("Charlie should be rejected from subscribing to conversation 1")
	}
}

func TestRealTimeMessageBroadcastOnSendMessage(t *testing.T) {
	cleanup := setupHubTestDB(t)
	defer cleanup()

	// Check that sending a message executes cleanly and notifies
	msg, err := SendMessage(1, 1, "Real-time hello!", "client-rt-1", nil)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if msg.Body != "Real-time hello!" {
		t.Errorf("Expected body 'Real-time hello!', got %q", msg.Body)
	}

	// Small pause to let async broadcasts finish
	time.Sleep(50 * time.Millisecond)

	// Verify message in database
	messages, err := GetConversationMessages(1, 1, 10, 0)
	if err != nil {
		t.Fatalf("GetConversationMessages failed: %v", err)
	}
	if len(messages) == 0 {
		t.Fatalf("Expected at least 1 message")
	}
}
