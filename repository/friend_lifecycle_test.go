package repository

import (
	"sync"
	"testing"

	"convention-management-system/database"
	"convention-management-system/models"
)

func TestFriendLifecycle_Send_List_Accept_Cancel_Reciprocal(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Ensure 3 test users exist: 1 (Alice), 2 (Bob), 3 (Charlie)
	// Seed user 3 if not present
	_, _ = databaseTestExec(`INSERT OR IGNORE INTO users (id, full_name, username, password_hash) VALUES (3, 'Charlie Test', 'charlie', 'hash3');`)

	// 1. Sending a valid friend request: Alice (1) -> Bob (2)
	res, err := SendFriendRequest(1, 2)
	if err != nil {
		t.Fatalf("1. SendFriendRequest failed: %v", err)
	}
	if res.RequestID <= 0 {
		t.Fatalf("Expected valid RequestID, got %d", res.RequestID)
	}
	if res.IsMutualFriend {
		t.Fatalf("Expected pending request, got mutual friend")
	}

	// 2. Rejecting self-requests
	_, err = SendFriendRequest(1, 1)
	if err == nil {
		t.Fatalf("2. Expected error for self friend request, got nil")
	}

	// 3. Preventing duplicate pending requests
	_, err = SendFriendRequest(1, 2)
	if err == nil {
		t.Fatalf("3. Expected duplicate friend request to fail, got nil")
	}

	// 4. Receiving a persisted notification
	notif := models.Notification{
		SenderID:   1,
		ReceiverID: 2,
		Type:       "friend_request_received",
		Message:    "sent you a friend request.",
	}
	if err := CreateNotification(notif); err != nil {
		t.Fatalf("4. CreateNotification failed: %v", err)
	}
	unreadCount, err := CountUnreadNotifications(2)
	if err != nil || unreadCount != 1 {
		t.Fatalf("4. Expected 1 unread notification for Bob, got %d (err: %v)", unreadCount, err)
	}

	// 5. Listing incoming requests (Bob should see Alice's request)
	incoming, err := GetIncomingRequests(2)
	if err != nil {
		t.Fatalf("5. GetIncomingRequests failed: %v", err)
	}
	if len(incoming) != 1 {
		t.Fatalf("5. Expected 1 incoming request for Bob, got %d", len(incoming))
	}
	if incoming[0].SenderID != 1 || incoming[0].Sender.Username != "alice" {
		t.Errorf("5. Expected incoming sender to be alice (1), got %s (%d)", incoming[0].Sender.Username, incoming[0].SenderID)
	}
	if incoming[0].Sender.Initials == "" {
		t.Errorf("5. Expected non-empty sender initials")
	}

	// 6. Listing outgoing pending requests (Alice should see outgoing to Bob)
	outgoing, err := GetOutgoingRequests(1)
	if err != nil {
		t.Fatalf("6. GetOutgoingRequests failed: %v", err)
	}
	if len(outgoing) != 1 {
		t.Fatalf("6. Expected 1 outgoing request for Alice, got %d", len(outgoing))
	}
	if outgoing[0].ReceiverID != 2 || outgoing[0].Receiver.Username != "bob" {
		t.Errorf("6. Expected receiver to be bob (2), got %s (%d)", outgoing[0].Receiver.Username, outgoing[0].ReceiverID)
	}

	// 11. Security: Unauthorized acceptance (Charlie trying to accept Alice -> Bob request)
	reqID := res.RequestID
	_, _, _, err = AcceptFriendRequest(reqID, 3) // 3 is not receiver
	if err == nil {
		t.Fatalf("11. Expected unauthorized acceptance to fail, got nil")
	}

	// 11. Security: Unauthorized decline (Charlie trying to decline Alice -> Bob request)
	err = DeclineFriendRequest(reqID, 3)
	if err == nil {
		t.Fatalf("11. Expected unauthorized decline to fail, got nil")
	}

	// 11. Security: Unauthorized cancel (Bob or Charlie trying to cancel Alice's request)
	err = CancelFriendRequest(reqID, 2) // Bob is receiver, not sender
	if err == nil {
		t.Fatalf("11. Expected non-sender cancellation to fail, got nil")
	}

	// 7. Accepting a request: Bob (2) accepts request from Alice (1)
	senderID, senderUsername, receiverUsername, err := AcceptFriendRequest(reqID, 2)
	if err != nil {
		t.Fatalf("7. AcceptFriendRequest failed: %v", err)
	}
	if senderID != 1 {
		t.Errorf("7. Expected senderID 1, got %d", senderID)
	}
	if senderUsername != "alice" || receiverUsername != "bob" {
		t.Errorf("7. Expected alice and bob, got %s and %s", senderUsername, receiverUsername)
	}

	// Friendship verified
	areFriends, err := AreFriends(1, 2)
	if err != nil || !areFriends {
		t.Fatalf("7. Expected Alice and Bob to be friends, got %v (err: %v)", areFriends, err)
	}
	areFriendsReverse, err := AreFriends(2, 1)
	if err != nil || !areFriendsReverse {
		t.Fatalf("7. Expected Bob and Alice to be mutual friends, got %v (err: %v)", areFriendsReverse, err)
	}

	// 12. Repeated accept / duplicate click on already accepted request
	_, _, _, err = AcceptFriendRequest(reqID, 2)
	if err == nil {
		t.Fatalf("12. Expected repeated accept on accepted request to fail, got nil")
	}

	// 10. Preventing acceptance after cancellation or rejection
	// Create another request: Alice (1) -> Charlie (3)
	resAC, err := SendFriendRequest(1, 3)
	if err != nil {
		t.Fatalf("SendFriendRequest to Charlie failed: %v", err)
	}
	// Charlie rejects it (8. Rejecting a request)
	if err := DeclineFriendRequest(resAC.RequestID, 3); err != nil {
		t.Fatalf("8. DeclineFriendRequest failed: %v", err)
	}
	// Try accepting rejected request
	_, _, _, err = AcceptFriendRequest(resAC.RequestID, 3)
	if err == nil {
		t.Fatalf("10. Expected accepting rejected request to fail, got nil")
	}

	// Test 9: Cancelling a pending outgoing request
	// Charlie (3) -> Alice (1)
	resCA, err := SendFriendRequest(3, 1)
	if err != nil {
		t.Fatalf("SendFriendRequest Charlie -> Alice failed: %v", err)
	}
	// Charlie cancels it
	if err := CancelFriendRequest(resCA.RequestID, 3); err != nil {
		t.Fatalf("9. CancelFriendRequest failed: %v", err)
	}
	// Verify it's no longer in Alice's incoming requests
	incomingAlice, err := GetIncomingRequests(1)
	if err != nil {
		t.Fatalf("GetIncomingRequests for Alice failed: %v", err)
	}
	for _, inc := range incomingAlice {
		if inc.ID == resCA.RequestID {
			t.Fatalf("Cancelled request %d still present in incoming requests", resCA.RequestID)
		}
	}
	// Try accepting cancelled request
	_, _, _, err = AcceptFriendRequest(resCA.RequestID, 1)
	if err == nil {
		t.Fatalf("10. Expected accepting cancelled request to fail, got nil")
	}

	// Test 13: Reciprocal Requests
	// Reset test users Bob (2) and Charlie (3)
	// Bob sends to Charlie
	resBC, err := SendFriendRequest(2, 3)
	if err != nil {
		t.Fatalf("SendFriendRequest Bob -> Charlie failed: %v", err)
	}
	if resBC.IsMutualFriend {
		t.Fatalf("First request should not be mutual")
	}
	// Charlie now sends to Bob (reciprocal request!)
	resCB, err := SendFriendRequest(3, 2)
	if err != nil {
		t.Fatalf("13. Reciprocal SendFriendRequest Charlie -> Bob failed: %v", err)
	}
	if !resCB.IsMutualFriend {
		t.Fatalf("13. Expected reciprocal request to be auto-accepted as mutual friend!")
	}
	// Verify Bob and Charlie are now mutual friends immediately
	areBCFriends, err := AreFriends(2, 3)
	if err != nil || !areBCFriends {
		t.Fatalf("13. Expected Bob and Charlie to be friends after reciprocal request, got %v", areBCFriends)
	}

	// Test 16: Accurate notification counts and MarkNotificationsAsRead
	// Insert unread notifications for Charlie
	_ = CreateNotification(models.Notification{SenderID: 1, ReceiverID: 3, Type: "follow", Message: "followed you"})
	_ = CreateNotification(models.Notification{SenderID: 2, ReceiverID: 3, Type: "friend_request_accepted", Message: "accepted your friend request."})
	countBefore, err := CountUnreadNotifications(3)
	if err != nil || countBefore != 2 {
		t.Fatalf("16. Expected 2 unread notifications for Charlie, got %d", countBefore)
	}
	if err := MarkNotificationsAsRead(3); err != nil {
		t.Fatalf("16. MarkNotificationsAsRead failed: %v", err)
	}
	countAfter, err := CountUnreadNotifications(3)
	if err != nil || countAfter != 0 {
		t.Fatalf("16. Expected 0 unread notifications after mark as read, got %d", countAfter)
	}
}

func TestFriendLifecycle_Concurrency(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, _ = databaseTestExec(`INSERT OR IGNORE INTO users (id, full_name, username, password_hash) VALUES (3, 'Charlie Test', 'charlie', 'hash3');`)

	// Concurrently attempt to create friendship / reciprocal requests between Alice (1) and Bob (2)
	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			sender := 1
			receiver := 2
			if idx%2 == 1 {
				sender = 2
				receiver = 1
			}
			_, err := SendFriendRequest(sender, receiver)
			errs[idx] = err
		}()
	}
	wg.Wait()

	// There should be exactly 0 or 1 row in friends table between (1, 2), never duplicates
	var friendRows int
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM friends WHERE (user1_id = 1 AND user2_id = 2) OR (user1_id = 2 AND user2_id = 1)`).Scan(&friendRows)
	if friendRows > 1 {
		t.Fatalf("14. Concurrency test created duplicate friendship records: %d", friendRows)
	}
}

func databaseTestExec(query string, args ...any) (any, error) {
	return database.DB.Exec(query, args...)
}
