package services

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"convention-management-system/database"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/websocket"
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

func TestDirectVideoCall_WebSocketOfferDelivery(t *testing.T) {
	cleanup := setupHubTestDB(t)
	defer cleanup()

	hub := GlobalHub

	// Create test server with websocket handler
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uidStr := r.URL.Query().Get("uid")
		uid := 1
		if uidStr == "2" {
			uid = 2
		}
		name := "Alice Walker"
		uname := "alice"
		if uid == 2 {
			name = "Bob Smith"
			uname = "bob"
		}
		handler := websocket.Handler(func(ws *websocket.Conn) {
			ServeWebSocket(ws, uid, uname, name)
		})
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()

	wsURL := "ws://" + server.Listener.Addr().String()

	// Connect User 2 (callee)
	ws2, err := websocket.Dial(wsURL+"?uid=2", "", "http://localhost/")
	if err != nil {
		t.Fatalf("Failed to connect User 2 websocket: %v", err)
	}
	defer ws2.Close()

	// Wait for connected confirmation for User 2
	var welcome2 WSEvent
	if err := websocket.JSON.Receive(ws2, &welcome2); err != nil {
		t.Fatalf("User 2 failed receiving welcome: %v", err)
	}
	if welcome2.Type != "connected" {
		t.Fatalf("Expected welcome event for User 2, got %s", welcome2.Type)
	}

	// Connect User 1 (caller)
	ws1, err := websocket.Dial(wsURL+"?uid=1", "", "http://localhost/")
	if err != nil {
		t.Fatalf("Failed to connect User 1 websocket: %v", err)
	}
	defer ws1.Close()

	var welcome1 WSEvent
	if err := websocket.JSON.Receive(ws1, &welcome1); err != nil {
		t.Fatalf("User 1 failed receiving welcome: %v", err)
	}

	// Check online presence
	if !hub.IsUserOnline(2) {
		t.Fatalf("Expected User 2 to be online in hub")
	}

	// User 1 sends video call offer to User 2
	offerEvent := map[string]any{
		"type":            "call_offer",
		"conversation_id": 1,
		"target_user_id":  2,
		"call_type":       "video",
		"sdp":             "v=0\r\no=- 456 2 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\nm=video 9 UDP/TLS/RTP/SAVPF 96\r\n",
	}
	if err := websocket.JSON.Send(ws1, offerEvent); err != nil {
		t.Fatalf("Failed to send call_offer: %v", err)
	}

	// Helper to receive next non-presence event
	recvNonPresence := func(ws *websocket.Conn) (WSEvent, error) {
		for {
			var ev WSEvent
			_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
			if err := websocket.JSON.Receive(ws, &ev); err != nil {
				return ev, err
			}
			if ev.Type != "presence" {
				return ev, nil
			}
		}
	}

	// User 1 should receive "call_initiated"
	callerEv, err := recvNonPresence(ws1)
	if err != nil {
		t.Fatalf("User 1 failed to receive call_initiated: %v", err)
	}
	if callerEv.Type != "call_initiated" {
		t.Errorf("Expected User 1 to receive 'call_initiated', got %s", callerEv.Type)
	}
	if callerEv.CallType != "video" {
		t.Errorf("Expected CallType 'video', got %s", callerEv.CallType)
	}
	if callerEv.CallID <= 0 {
		t.Errorf("Expected positive CallID, got %d", callerEv.CallID)
	}

	// User 2 should receive "call_offer"
	calleeEv, err := recvNonPresence(ws2)
	if err != nil {
		t.Fatalf("User 2 failed to receive call_offer: %v", err)
	}
	if calleeEv.Type != "call_offer" {
		t.Errorf("Expected User 2 to receive 'call_offer', got %s", calleeEv.Type)
	}
	if calleeEv.CallType != "video" {
		t.Errorf("Expected User 2 CallType 'video', got %s", calleeEv.CallType)
	}
	if calleeEv.CallID != callerEv.CallID {
		t.Errorf("Expected matching CallID %d, got %d", callerEv.CallID, calleeEv.CallID)
	}
	if !strings.Contains(calleeEv.SDP, "m=video") {
		t.Errorf("Expected video SDP in callee event, got %q", calleeEv.SDP)
	}

	// Callee User 2 sends call_answer with answer SDP
	answerEvent := map[string]any{
		"type":            "call_answer",
		"conversation_id": 1,
		"target_user_id":  1,
		"call_id":         calleeEv.CallID,
		"sdp":             "v=0\r\no=- 789 2 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\nm=video 9 UDP/TLS/RTP/SAVPF 96\r\n",
	}
	if err := websocket.JSON.Send(ws2, answerEvent); err != nil {
		t.Fatalf("Failed to send call_answer: %v", err)
	}

	// Caller User 1 should receive "call_answer"
	callerAnswerEv, err := recvNonPresence(ws1)
	if err != nil {
		t.Fatalf("User 1 failed to receive call_answer: %v", err)
	}
	if callerAnswerEv.Type != "call_answer" {
		t.Errorf("Expected User 1 to receive 'call_answer', got %s", callerAnswerEv.Type)
	}
	if callerAnswerEv.CallID != calleeEv.CallID {
		t.Errorf("Expected call_answer CallID %d, got %d", calleeEv.CallID, callerAnswerEv.CallID)
	}
}

func TestDirectCall_DeclineDelivery(t *testing.T) {
	cleanup := setupHubTestDB(t)
	defer cleanup()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uidStr := r.URL.Query().Get("uid")
		uid := 1
		if uidStr == "2" {
			uid = 2
		}
		handler := websocket.Handler(func(ws *websocket.Conn) {
			ServeWebSocket(ws, uid, fmt.Sprintf("user%d", uid), fmt.Sprintf("User %d", uid))
		})
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()

	wsURL := "ws://" + server.Listener.Addr().String()

	ws2, err := websocket.Dial(wsURL+"?uid=2", "", "http://localhost/")
	if err != nil {
		t.Fatalf("Failed to connect User 2: %v", err)
	}
	defer ws2.Close()

	var w2 WSEvent
	_ = websocket.JSON.Receive(ws2, &w2)

	ws1, err := websocket.Dial(wsURL+"?uid=1", "", "http://localhost/")
	if err != nil {
		t.Fatalf("Failed to connect User 1: %v", err)
	}
	defer ws1.Close()

	var w1 WSEvent
	_ = websocket.JSON.Receive(ws1, &w1)

	recvNonPresence := func(ws *websocket.Conn) (WSEvent, error) {
		for {
			var ev WSEvent
			_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
			if err := websocket.JSON.Receive(ws, &ev); err != nil {
				return ev, err
			}
			if ev.Type != "presence" {
				return ev, nil
			}
		}
	}

	// User 1 initiates call to User 2
	_ = websocket.JSON.Send(ws1, map[string]any{
		"type":            "call_offer",
		"conversation_id": 1,
		"target_user_id":  2,
		"call_type":       "video",
		"sdp":             "offer-sdp",
	})

	_, _ = recvNonPresence(ws1) // call_initiated
	calleeEv, err := recvNonPresence(ws2)
	if err != nil || calleeEv.Type != "call_offer" {
		t.Fatalf("Callee failed to receive offer: %v", err)
	}

	// Callee declines call
	_ = websocket.JSON.Send(ws2, map[string]any{
		"type":            "call_decline",
		"conversation_id": 1,
		"target_user_id":  1,
		"call_id":         calleeEv.CallID,
	})

	callerDeclineEv, err := recvNonPresence(ws1)
	if err != nil {
		t.Fatalf("Caller failed to receive decline event: %v", err)
	}
	if callerDeclineEv.Type != "call_decline" {
		t.Errorf("Expected 'call_decline', got '%s'", callerDeclineEv.Type)
	}
}

func TestDirectAudioCall_OfferAndAnswer_Regression(t *testing.T) {
	cleanup := setupHubTestDB(t)
	defer cleanup()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uidStr := r.URL.Query().Get("uid")
		uid := 1
		if uidStr == "2" {
			uid = 2
		}
		handler := websocket.Handler(func(ws *websocket.Conn) {
			ServeWebSocket(ws, uid, fmt.Sprintf("user%d", uid), fmt.Sprintf("User %d", uid))
		})
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()

	wsURL := "ws://" + server.Listener.Addr().String()

	ws2, _ := websocket.Dial(wsURL+"?uid=2", "", "http://localhost/")
	defer ws2.Close()
	var w2 WSEvent
	_ = websocket.JSON.Receive(ws2, &w2)

	ws1, _ := websocket.Dial(wsURL+"?uid=1", "", "http://localhost/")
	defer ws1.Close()
	var w1 WSEvent
	_ = websocket.JSON.Receive(ws1, &w1)

	recvNonPresence := func(ws *websocket.Conn) (WSEvent, error) {
		for {
			var ev WSEvent
			_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
			if err := websocket.JSON.Receive(ws, &ev); err != nil {
				return ev, err
			}
			if ev.Type != "presence" {
				return ev, nil
			}
		}
	}

	// Audio call offer
	_ = websocket.JSON.Send(ws1, map[string]any{
		"type":            "call_offer",
		"conversation_id": 1,
		"target_user_id":  2,
		"call_type":       "audio",
		"sdp":             "m=audio 9 UDP/TLS/RTP/SAVPF 111",
	})

	callerInit, err := recvNonPresence(ws1)
	if err != nil || callerInit.Type != "call_initiated" {
		t.Fatalf("Expected call_initiated: %v", err)
	}
	if callerInit.CallType != "audio" {
		t.Errorf("Expected audio call_type, got %s", callerInit.CallType)
	}

	calleeOffer, err := recvNonPresence(ws2)
	if err != nil || calleeOffer.Type != "call_offer" {
		t.Fatalf("Expected call_offer: %v", err)
	}
	if calleeOffer.CallType != "audio" {
		t.Errorf("Expected audio call_type, got %s", calleeOffer.CallType)
	}
}

func TestDirectCall_OfflineCallee_EdgeCase(t *testing.T) {
	cleanup := setupHubTestDB(t)
	defer cleanup()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler := websocket.Handler(func(ws *websocket.Conn) {
			ServeWebSocket(ws, 1, "alice", "Alice Walker")
		})
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()

	wsURL := "ws://" + server.Listener.Addr().String()
	ws1, err := websocket.Dial(wsURL, "", "http://localhost/")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer ws1.Close()

	var welcome WSEvent
	_ = websocket.JSON.Receive(ws1, &welcome)

	// User 1 calls User 99 who is offline
	_ = websocket.JSON.Send(ws1, map[string]any{
		"type":            "call_offer",
		"conversation_id": 1,
		"target_user_id":  99,
		"call_type":       "video",
		"sdp":             "sdp-data",
	})

	for {
		var ev WSEvent
		_ = ws1.SetReadDeadline(time.Now().Add(2 * time.Second))
		if err := websocket.JSON.Receive(ws1, &ev); err != nil {
			t.Fatalf("Failed to receive offline callee response: %v", err)
		}
		if ev.Type == "call_unavailable" {
			if !strings.Contains(ev.Error, "offline") {
				t.Errorf("Expected offline error message, got: %s", ev.Error)
			}
			break
		}
	}
}
