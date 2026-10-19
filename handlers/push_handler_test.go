package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"convention-management-system/database"
	"convention-management-system/models"
	"convention-management-system/repository"
)

func TestGetVAPIDPublicKeyAPI(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/push/vapid-public-key", nil)
	rr := httptest.NewRecorder()

	GetVAPIDPublicKeyAPI(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rr.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	pubKey := resp["public_key"]
	if pubKey == "" {
		t.Fatalf("Expected non-empty public_key in JSON")
	}
}

func TestSubscribePushAPI_AuthAndValidation(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Unauthenticated request -> 401
	unauthReq := httptest.NewRequest(http.MethodPost, "/push/subscribe", strings.NewReader(`{}`))
	unauthRR := httptest.NewRecorder()
	SubscribePushAPI(unauthRR, unauthReq)
	if unauthRR.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", unauthRR.Code)
	}

	// 2. Authenticated but empty payload -> 400
	req, rr := makeAuthReq(http.MethodPost, "/push/subscribe", `{}`, "application/json", 1)
	SubscribePushAPI(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for empty payload, got %d", rr.Code)
	}

	// 3. Authenticated valid subscription
	validBody := `{
		"endpoint": "https://updates.push.services.mozilla.com/wpush/v2/test-endpoint-abc",
		"keys": {
			"p256dh": "BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QT9t0PPM6lx0gkcmWBnf6UfStvWt9bDDlzpqsofV",
			"auth": "tBHItJI5svbpez7KI4CCXg"
		},
		"user_agent": "Mozilla/5.0 (Android; Mobile)"
	}`
	req2, rr2 := makeAuthReq(http.MethodPost, "/push/subscribe", validBody, "application/json", 1)
	SubscribePushAPI(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", rr2.Code, rr2.Body.String())
	}

	// Verify persistence in DB
	subs, err := repository.GetPushSubscriptionsByUserID(1)
	if err != nil {
		t.Fatalf("GetPushSubscriptionsByUserID failed: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("Expected 1 subscription, got %d", len(subs))
	}
	if subs[0].Endpoint != "https://updates.push.services.mozilla.com/wpush/v2/test-endpoint-abc" {
		t.Errorf("Unexpected endpoint: %s", subs[0].Endpoint)
	}
}

func TestUnsubscribePushAPI_Ownership(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Seed subscription for user 1
	sub := &models.PushSubscription{
		UserID:   1,
		Endpoint: "https://fcm.googleapis.com/fcm/send/phone-1",
		P256dh:   "key1",
		Auth:     "auth1",
	}
	if err := repository.SavePushSubscription(sub); err != nil {
		t.Fatalf("Failed to save sub: %v", err)
	}

	// User 2 attempts to unsubscribe user 1's endpoint -> should not delete user 1's sub
	unsubBody := `{"endpoint": "https://fcm.googleapis.com/fcm/send/phone-1"}`
	reqOther, rrOther := makeAuthReq(http.MethodPost, "/push/unsubscribe", unsubBody, "application/json", 2)
	UnsubscribePushAPI(rrOther, reqOther)
	if rrOther.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", rrOther.Code)
	}

	subsStillExist, _ := repository.GetPushSubscriptionsByUserID(1)
	if len(subsStillExist) != 1 {
		t.Errorf("User 2 should not be able to delete User 1 subscription!")
	}

	// User 1 unsubscribes own endpoint
	reqSelf, rrSelf := makeAuthReq(http.MethodPost, "/push/unsubscribe", unsubBody, "application/json", 1)
	UnsubscribePushAPI(rrSelf, reqSelf)
	if rrSelf.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", rrSelf.Code)
	}

	subsDeleted, _ := repository.GetPushSubscriptionsByUserID(1)
	if len(subsDeleted) != 0 {
		t.Errorf("Expected 0 subscriptions after self-unsubscribe, got %d", len(subsDeleted))
	}
}

func TestGetCallStatusAPI(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Seed conversation between user 1 and user 2
	_, _ = database.DB.Exec(`
		INSERT INTO conversations (id, type, title, created_by) VALUES (10, 'direct', '', 1);
		INSERT INTO conversation_members (conversation_id, user_id, role) VALUES (10, 1, 'member'), (10, 2, 'member');
	`)

	// 2. Create active call
	expiresAt := time.Now().Add(45 * time.Second)
	activeCall := &models.CallLog{
		ConversationID: 10,
		CallerID:       1,
		ReceiverID:     2,
		CallType:       "audio",
		Status:         "initiated",
		ExpiresAt:      &expiresAt,
	}
	_ = repository.CreateCallLog(activeCall)

	// 3. User 2 checks status -> active
	urlActive := fmt.Sprintf("/messages/call/status?call_id=%d", activeCall.ID)
	reqActive, rrActive := makeAuthReq(http.MethodGet, urlActive, "", "", 2)
	GetCallStatusAPI(rrActive, reqActive)
	if rrActive.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", rrActive.Code, rrActive.Body.String())
	}

	var statusResp struct {
		CallID   int    `json:"call_id"`
		Status   string `json:"status"`
		IsActive bool   `json:"is_active"`
		CallType string `json:"call_type"`
	}
	if err := json.Unmarshal(rrActive.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("Failed to parse status response: %v", err)
	}
	if !statusResp.IsActive || statusResp.Status != "initiated" {
		t.Errorf("Expected is_active=true and status='initiated', got %+v", statusResp)
	}

	// 4. User 3 (not in conversation, not caller, not receiver) attempts to check -> 403 Forbidden
	reqForbidden, rrForbidden := makeAuthReq(http.MethodGet, urlActive, "", "", 3)
	GetCallStatusAPI(rrForbidden, reqForbidden)
	if rrForbidden.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for non-participant user 3, got %d", rrForbidden.Code)
	}

	// 5. Create expired call (ExpiresAt in the past)
	pastExpiry := time.Now().Add(-10 * time.Second)
	expiredCall := &models.CallLog{
		ConversationID: 10,
		CallerID:       1,
		ReceiverID:     2,
		CallType:       "video",
		Status:         "initiated",
		ExpiresAt:      &pastExpiry,
	}
	_ = repository.CreateCallLog(expiredCall)

	urlExpired := fmt.Sprintf("/messages/call/status?call_id=%d", expiredCall.ID)
	reqExpired, rrExpired := makeAuthReq(http.MethodGet, urlExpired, "", "", 2)
	GetCallStatusAPI(rrExpired, reqExpired)
	if rrExpired.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rrExpired.Code)
	}

	var expiredResp struct {
		CallID   int    `json:"call_id"`
		Status   string `json:"status"`
		IsActive bool   `json:"is_active"`
	}
	_ = json.Unmarshal(rrExpired.Body.Bytes(), &expiredResp)
	if expiredResp.IsActive || expiredResp.Status != "missed" {
		t.Errorf("Expected expired call to have is_active=false and status='missed', got %+v", expiredResp)
	}
}

