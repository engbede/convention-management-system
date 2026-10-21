package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"convention-management-system/database"
	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/sessions"
)

func makeFriendAuthRequest(method, urlStr string, form url.Values, userID int, isJSON bool) *http.Request {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, urlStr, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if isJSON {
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
	}

	if userID > 0 {
		rec := httptest.NewRecorder()
		session, _ := sessions.Store.Get(req, "youth-community")
		session.Values["user_id"] = userID
		_ = session.Save(req, rec)
		for _, cookie := range rec.Result().Cookies() {
			req.AddCookie(cookie)
		}
	}

	return req
}

func TestFriendHandlers_FullLifecycle(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Unauthenticated SendFriendRequest
	unauthReq := makeFriendAuthRequest(http.MethodPost, "/friends/request", url.Values{"receiver_id": {"2"}}, 0, true)
	unauthRec := httptest.NewRecorder()
	SendFriendRequest(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized, got %d", unauthRec.Code)
	}

	// 2. Self Friend Request (user 1 -> user 1)
	selfReq := makeFriendAuthRequest(http.MethodPost, "/friends/request", url.Values{"receiver_id": {"1"}}, 1, true)
	selfRec := httptest.NewRecorder()
	SendFriendRequest(selfRec, selfReq)
	if selfRec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request for self request, got %d", selfRec.Code)
	}

	// 3. Valid SendFriendRequest via JSON: user 1 (Alice) -> user 2 (Bob)
	sendReq := makeFriendAuthRequest(http.MethodPost, "/friends/request", url.Values{"receiver_id": {"2"}}, 1, true)
	sendRec := httptest.NewRecorder()
	SendFriendRequest(sendRec, sendReq)
	if sendRec.Code != http.StatusOK {
		t.Fatalf("SendFriendRequest failed: %d (body: %s)", sendRec.Code, sendRec.Body.String())
	}

	var sendResp map[string]any
	if err := json.Unmarshal(sendRec.Body.Bytes(), &sendResp); err != nil {
		t.Fatalf("Failed to decode send JSON: %v", err)
	}
	if sendResp["success"] != true || sendResp["status"] != "pending" {
		t.Errorf("Unexpected send response: %+v", sendResp)
	}

	// Verify incoming request for user 2
	incoming, err := repository.GetIncomingRequests(2)
	if err != nil || len(incoming) != 1 {
		t.Fatalf("Expected 1 incoming request for user 2, got %d (err: %v)", len(incoming), err)
	}
	reqID := incoming[0].ID

	// 4. Unauthorized Accept: user 3 (Charlie) tries to accept request sent to user 2 (Bob)
	unauthAcceptReq := makeFriendAuthRequest(http.MethodPost, "/friends/accept", url.Values{"request_id": {"1"}}, 3, true)
	unauthAcceptRec := httptest.NewRecorder()
	AcceptFriendRequest(unauthAcceptRec, unauthAcceptReq)
	if unauthAcceptRec.Code != http.StatusForbidden && unauthAcceptRec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 403 or 400 for unauthorized accept, got %d (body: %s)", unauthAcceptRec.Code, unauthAcceptRec.Body.String())
	}

	// 5. Authorized Accept: user 2 (Bob) accepts request from user 1 (Alice)
	acceptReq := makeFriendAuthRequest(http.MethodPost, "/friends/accept", url.Values{
		"request_id": {string(rune('0' + reqID))}, // or strconv
		"sender_id":  {"1"},
	}, 2, true)
	acceptRec := httptest.NewRecorder()
	AcceptFriendRequest(acceptRec, acceptReq)
	if acceptRec.Code != http.StatusOK {
		t.Fatalf("AcceptFriendRequest failed: %d (body: %s)", acceptRec.Code, acceptRec.Body.String())
	}

	// Verify both users are friends
	areFriends, err := repository.AreFriends(1, 2)
	if err != nil || !areFriends {
		t.Fatalf("Expected users 1 and 2 to be friends, got %v", areFriends)
	}

	// 6. Test Decline and Cancel on a new request: user 3 -> user 1
	res31, err := repository.SendFriendRequest(3, 1)
	if err != nil {
		t.Fatalf("SendFriendRequest 3->1 failed: %v", err)
	}

	// Non-sender (user 2) tries to cancel user 3's request
	badCancelReq := makeFriendAuthRequest(http.MethodPost, "/friends/cancel", url.Values{
		"request_id": {string(rune('0' + res31.RequestID))},
	}, 2, true)
	badCancelRec := httptest.NewRecorder()
	CancelFriendRequest(badCancelRec, badCancelReq)
	if badCancelRec.Code != http.StatusForbidden && badCancelRec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 403/400 for non-sender cancel, got %d", badCancelRec.Code)
	}

	// Sender (user 3) cancels request
	goodCancelReq := makeFriendAuthRequest(http.MethodPost, "/friends/cancel", url.Values{
		"request_id": {string(rune('0' + res31.RequestID))},
	}, 3, true)
	goodCancelRec := httptest.NewRecorder()
	CancelFriendRequest(goodCancelRec, goodCancelReq)
	if goodCancelRec.Code != http.StatusOK {
		t.Fatalf("CancelFriendRequest failed: %d (body: %s)", goodCancelRec.Code, goodCancelRec.Body.String())
	}

	// 7. Test MarkNotificationsReadAPI
	// Insert unread notification for user 1
	_ = repository.CreateNotification(models.Notification{
		SenderID:   2,
		ReceiverID: 1,
		Type:       "friend_request_accepted",
		Message:    "@bob accepted your friend request.",
	})

	unreadBefore, _ := repository.CountUnreadNotifications(1)
	if unreadBefore < 1 {
		t.Fatalf("Expected unread notification before mark read, got %d", unreadBefore)
	}

	readReq := makeFriendAuthRequest(http.MethodPost, "/api/notifications/read", nil, 1, true)
	readRec := httptest.NewRecorder()
	MarkNotificationsReadAPI(readRec, readReq)
	if readRec.Code != http.StatusOK {
		t.Fatalf("MarkNotificationsReadAPI failed: %d (body: %s)", readRec.Code, readRec.Body.String())
	}

	unreadAfter, _ := repository.CountUnreadNotifications(1)
	if unreadAfter != 0 {
		t.Fatalf("Expected 0 unread notifications after mark read, got %d", unreadAfter)
	}

	// 8. Test FriendsDashboard handler loads without errors
	dashReq := makeFriendAuthRequest(http.MethodGet, "/friends", nil, 1, false)
	dashRec := httptest.NewRecorder()
	FriendsDashboard(dashRec, dashReq)
	if dashRec.Code != http.StatusOK {
		t.Fatalf("FriendsDashboard failed: %d", dashRec.Code)
	}
	bodyStr := dashRec.Body.String()
	if !strings.Contains(bodyStr, "Friends") {
		t.Errorf("FriendsDashboard response missing 'Friends' keyword")
	}

	// 9. Test PublicProfile handler includes FriendStatus
	profReq := makeFriendAuthRequest(http.MethodGet, "/user/user2", nil, 1, false)
	profRec := httptest.NewRecorder()
	PublicProfile(profRec, profReq)
	if profRec.Code != http.StatusOK {
		t.Fatalf("PublicProfile failed: %d", profRec.Code)
	}

	// 10. Test RemoveFriend handler
	removeReq := makeFriendAuthRequest(http.MethodPost, "/friends/remove", url.Values{
		"user_id": {"2"},
	}, 1, true)
	removeRec := httptest.NewRecorder()
	RemoveFriend(removeRec, removeReq)
	if removeRec.Code != http.StatusOK {
		t.Fatalf("RemoveFriend failed: %d (body: %s)", removeRec.Code, removeRec.Body.String())
	}

	// Verify no longer friends
	nowFriends, err := repository.AreFriends(1, 2)
	if err != nil || nowFriends {
		t.Fatalf("Expected users 1 and 2 to no longer be friends, got %v", nowFriends)
	}
}

// Dummy import anchor
var _ = database.DB
