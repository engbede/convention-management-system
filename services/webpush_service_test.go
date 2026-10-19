package services_test

import (
	"database/sql"
	"os"
	"testing"
	"time"

	"convention-management-system/database"
	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/services"

	_ "github.com/mattn/go-sqlite3"
)

func setupPushServiceTestDB(t *testing.T) func() {
	t.Helper()
	testDBPath := "test_push_service.db"
	_ = os.Remove(testDBPath)

	db, err := sql.Open("sqlite3", testDBPath)
	if err != nil {
		t.Fatalf("Failed to open test db: %v", err)
	}
	database.DB = db

	if err := database.MigrateSQLite(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	return func() {
		db.Close()
		_ = os.Remove(testDBPath)
	}
}

func TestVAPIDKeyConfiguration(t *testing.T) {
	pubKey := services.GetVAPIDPublicKey()
	if pubKey == "" {
		t.Fatalf("Expected non-empty VAPID public key")
	}
	if len(pubKey) < 30 {
		t.Errorf("VAPID public key seems unusually short: %q", pubKey)
	}
}

func TestPushSubscriptionLifecycle(t *testing.T) {
	cleanup := setupPushServiceTestDB(t)
	defer cleanup()

	// 1. Seed test users
	_, err := database.DB.Exec(`
		INSERT INTO users (id, full_name, username, email, password_hash, role)
		VALUES (1, 'User One', 'userone', 'user1@example.com', 'hash', 'member'),
		       (2, 'User Two', 'usertwo', 'user2@example.com', 'hash', 'member')
	`)
	if err != nil {
		t.Fatalf("Failed to seed users: %v", err)
	}

	sub1 := &models.PushSubscription{
		UserID:    1,
		Endpoint:  "https://fcm.googleapis.com/fcm/send/phone-endpoint-1",
		P256dh:    "BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QT9t0PPM6lx0gkcmWBnf6UfStvWt9bDDlzpqsofV",
		Auth:      "tBHItJI5svbpez7KI4CCXg",
		UserAgent: "Mozilla/5.0 (Android; Mobile; rv:120.0)",
	}

	// 2. Save subscription
	if err := repository.SavePushSubscription(sub1); err != nil {
		t.Fatalf("SavePushSubscription failed: %v", err)
	}
	if sub1.ID <= 0 {
		t.Errorf("Expected positive subscription ID, got %d", sub1.ID)
	}

	// 3. Retrieve subscriptions by user ID
	subs, err := repository.GetPushSubscriptionsByUserID(1)
	if err != nil {
		t.Fatalf("GetPushSubscriptionsByUserID failed: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("Expected 1 subscription, got %d", len(subs))
	}
	if subs[0].Endpoint != sub1.Endpoint {
		t.Errorf("Expected endpoint %s, got %s", sub1.Endpoint, subs[0].Endpoint)
	}

	// 4. Update subscription (same endpoint, new auth)
	sub1Update := &models.PushSubscription{
		UserID:    1,
		Endpoint:  "https://fcm.googleapis.com/fcm/send/phone-endpoint-1",
		P256dh:    sub1.P256dh,
		Auth:      "newUpdatedAuthToken123",
		UserAgent: "Mozilla/5.0 (Android; Mobile; rv:121.0)",
	}
	if err := repository.SavePushSubscription(sub1Update); err != nil {
		t.Fatalf("Update subscription failed: %v", err)
	}

	subsAfterUpdate, _ := repository.GetPushSubscriptionsByUserID(1)
	if len(subsAfterUpdate) != 1 {
		t.Errorf("Expected still 1 subscription after update, got %d", len(subsAfterUpdate))
	}
	if subsAfterUpdate[0].Auth != "newUpdatedAuthToken123" {
		t.Errorf("Expected updated auth, got %s", subsAfterUpdate[0].Auth)
	}

	// 5. Delete subscription by endpoint
	if err := repository.DeletePushSubscriptionByEndpoint(sub1.Endpoint); err != nil {
		t.Fatalf("DeletePushSubscriptionByEndpoint failed: %v", err)
	}
	subsAfterDelete, _ := repository.GetPushSubscriptionsByUserID(1)
	if len(subsAfterDelete) != 0 {
		t.Errorf("Expected 0 subscriptions after delete, got %d", len(subsAfterDelete))
	}
}

func TestCallStateTransitionsAndAuthoritativeExpiry(t *testing.T) {
	cleanup := setupPushServiceTestDB(t)
	defer cleanup()

	// 1. Seed users and conversation
	_, err := database.DB.Exec(`
		INSERT INTO users (id, full_name, username, email, password_hash, role)
		VALUES (1, 'Caller John', 'john', 'john@example.com', 'hash', 'member'),
		       (2, 'Receiver Mary', 'mary', 'mary@example.com', 'hash', 'member');
		INSERT INTO conversations (id, type, title, created_by)
		VALUES (1, 'direct', '', 1);
		INSERT INTO conversation_members (conversation_id, user_id, role)
		VALUES (1, 1, 'member'), (1, 2, 'member');
	`)
	if err != nil {
		t.Fatalf("Failed to seed: %v", err)
	}

	// 2. Create initiated call with 45s expiration
	expiresAt := time.Now().Add(45 * time.Second)
	call := &models.CallLog{
		ConversationID: 1,
		CallerID:       1,
		ReceiverID:     2,
		CallType:       "audio",
		Status:         "initiated",
		ExpiresAt:      &expiresAt,
	}
	if err := repository.CreateCallLog(call); err != nil {
		t.Fatalf("CreateCallLog failed: %v", err)
	}
	if call.ID <= 0 {
		t.Fatalf("Expected positive Call ID, got %d", call.ID)
	}

	// 3. Query authoritative call log
	cl, err := repository.GetCallLogByID(call.ID)
	if err != nil {
		t.Fatalf("GetCallLogByID failed: %v", err)
	}
	if cl.Status != "initiated" {
		t.Errorf("Expected status 'initiated', got %s", cl.Status)
	}
	if cl.ExpiresAt == nil || cl.ExpiresAt.IsZero() {
		t.Errorf("Expected non-zero ExpiresAt")
	}

	// 4. Update status to connected
	if err := repository.UpdateCallLogStatus(call.ID, "connected"); err != nil {
		t.Fatalf("UpdateCallLogStatus failed: %v", err)
	}
	clConnected, _ := repository.GetCallLogByID(call.ID)
	if clConnected.Status != "connected" {
		t.Errorf("Expected status 'connected', got %s", clConnected.Status)
	}

	// 5. Complete call with duration
	if err := repository.UpdateCallLog(call.ID, "completed", 120); err != nil {
		t.Fatalf("UpdateCallLog failed: %v", err)
	}
	clCompleted, _ := repository.GetCallLogByID(call.ID)
	if clCompleted.Status != "completed" || clCompleted.DurationSeconds != 120 {
		t.Errorf("Expected completed status with 120s duration, got %s / %d", clCompleted.Status, clCompleted.DurationSeconds)
	}
	if clCompleted.EndedAt == nil {
		t.Errorf("Expected non-nil EndedAt")
	}
}

func TestCallerCancellationMarksCancelledNotMissed(t *testing.T) {
	cleanup := setupPushServiceTestDB(t)
	defer cleanup()

	_, err := database.DB.Exec(`
		INSERT INTO users (id, full_name, username, email, password_hash, role)
		VALUES (1, 'Caller John', 'john', 'john@example.com', 'hash', 'member'),
		       (2, 'Receiver Mary', 'mary', 'mary@example.com', 'hash', 'member');
		INSERT INTO conversations (id, type, title, created_by)
		VALUES (1, 'direct', '', 1);
		INSERT INTO conversation_members (conversation_id, user_id, role)
		VALUES (1, 1, 'member'), (1, 2, 'member');
	`)
	if err != nil {
		t.Fatalf("Failed to seed: %v", err)
	}

	call := &models.CallLog{
		ConversationID: 1,
		CallerID:       1,
		ReceiverID:     2,
		CallType:       "video",
		Status:         "initiated",
	}
	_ = repository.CreateCallLog(call)

	// When caller cancels before answer:
	if err := repository.UpdateCallLog(call.ID, "cancelled", 0); err != nil {
		t.Fatalf("UpdateCallLog cancelled failed: %v", err)
	}

	cl, _ := repository.GetCallLogByID(call.ID)
	if cl.Status != "cancelled" {
		t.Errorf("Expected status 'cancelled', got %s", cl.Status)
	}
}

