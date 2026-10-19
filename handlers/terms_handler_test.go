package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"convention-management-system/database"
	"convention-management-system/middleware"
	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/services"
	"convention-management-system/sessions"
)

func createAuthenticatedRequest(method, target string, body []byte, userID int) *http.Request {
	req := httptest.NewRequest(method, target, bytes.NewBuffer(body))
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	initReq := httptest.NewRequest(http.MethodGet, "/", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = userID
	_ = session.Save(initReq, saveRec)

	for _, cookie := range saveRec.Result().Cookies() {
		req.AddCookie(cookie)
	}

	return req
}

func setupTermsHandlerTestDB(t *testing.T) func() {
	t.Helper()
	cleanup := setupTestDB(t)
	if database.DB != nil {
		_, _ = database.DB.Exec("DELETE FROM terms_acceptances")
		_, _ = database.DB.Exec("DELETE FROM settings WHERE key = 'active_terms_version'")
	}
	return cleanup
}

func TestTermsStatusAPI_Unauthenticated(t *testing.T) {
	cleanup := setupTermsHandlerTestDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/terms/status", nil)
	rr := httptest.NewRecorder()

	TermsStatusAPI(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized for unauthenticated /terms/status, got %d", rr.Code)
	}
}

func TestTermsAcceptance_FullFlow(t *testing.T) {
	cleanup := setupTermsHandlerTestDB(t)
	defer cleanup()

	// 1. Check status when user has not yet accepted terms
	req := createAuthenticatedRequest(http.MethodGet, "/terms/status", nil, 1)
	rr := httptest.NewRecorder()

	TermsStatusAPI(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /terms/status, got %d", rr.Code)
	}

	var status models.TermsStatusResponse
	if err := json.NewDecoder(rr.Body).Decode(&status); err != nil {
		t.Fatalf("Failed to decode TermsStatusResponse: %v", err)
	}

	if status.Accepted {
		t.Errorf("Expected accepted=false initially")
	}
	if status.RequiredVersion != "v1.0" {
		t.Errorf("Expected required_version=v1.0, got %s", status.RequiredVersion)
	}

	// Create conversation between user 1 and user 2
	conv, err := repository.CreateOrGetDirectConversation(1, 2)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// 2. Sending message without terms acceptance must be blocked with 403 Forbidden
	sendPayload := map[string]interface{}{
		"conversation_id": conv.ID,
		"body":            "Hello fellowship!",
	}
	payloadBytes, _ := json.Marshal(sendPayload)
	sendReq := createAuthenticatedRequest(http.MethodPost, "/messages/send", payloadBytes, 1)
	sendRR := httptest.NewRecorder()

	SendMessageAPI(sendRR, sendReq)

	if sendRR.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for SendMessageAPI before accepting terms, got %d", sendRR.Code)
	}

	var sendErrResp map[string]interface{}
	_ = json.NewDecoder(sendRR.Body).Decode(&sendErrResp)
	if sendErrResp["error"] != "terms_acceptance_required" {
		t.Errorf("Expected error 'terms_acceptance_required', got %v", sendErrResp["error"])
	}

	// 3. Accept terms via POST /terms/accept
	acceptPayload := map[string]string{
		"version": "v1.0",
	}
	acceptBytes, _ := json.Marshal(acceptPayload)
	acceptReq := createAuthenticatedRequest(http.MethodPost, "/terms/accept", acceptBytes, 1)
	acceptReq.Header.Set("User-Agent", "TestTermsSuite/1.0")
	acceptReq.RemoteAddr = "192.168.1.42:38421"
	acceptRR := httptest.NewRecorder()

	AcceptTermsAPI(acceptRR, acceptReq)

	if acceptRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /terms/accept, got %d (body: %s)", acceptRR.Code, acceptRR.Body.String())
	}

	var acceptResp map[string]interface{}
	_ = json.NewDecoder(acceptRR.Body).Decode(&acceptResp)
	if acceptResp["success"] != true {
		t.Errorf("Expected success=true in accept response")
	}

	// 4. Verify status now reports accepted = true
	checkReq := createAuthenticatedRequest(http.MethodGet, "/terms/status", nil, 1)
	checkRR := httptest.NewRecorder()

	TermsStatusAPI(checkRR, checkReq)

	if checkRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK after acceptance, got %d", checkRR.Code)
	}

	var updatedStatus models.TermsStatusResponse
	_ = json.NewDecoder(checkRR.Body).Decode(&updatedStatus)
	if !updatedStatus.Accepted {
		t.Errorf("Expected accepted=true after acceptance")
	}
	if updatedStatus.UserVersion != "v1.0" {
		t.Errorf("Expected UserVersion=v1.0, got %s", updatedStatus.UserVersion)
	}
	if updatedStatus.Record == nil || updatedStatus.Record.IPAddress != "192.168.1.42" {
		t.Errorf("Expected recorded IP 192.168.1.42, got %+v", updatedStatus.Record)
	}

	// 5. Sending message is now permitted
	sendPermittedReq := createAuthenticatedRequest(http.MethodPost, "/messages/send", payloadBytes, 1)
	sendPermittedRR := httptest.NewRecorder()

	SendMessageAPI(sendPermittedRR, sendPermittedReq)

	if sendPermittedRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for SendMessageAPI after accepting terms, got %d (body: %s)", sendPermittedRR.Code, sendPermittedRR.Body.String())
	}

	// 6. Test Version Bump: Admin releases v2.0
	err = services.SetActiveTermsVersion("v2.0")
	if err != nil {
		t.Fatalf("SetActiveTermsVersion failed: %v", err)
	}

	// User status should now report accepted=false with required_version=v2.0
	statusV2Req := createAuthenticatedRequest(http.MethodGet, "/terms/status", nil, 1)
	statusV2RR := httptest.NewRecorder()

	TermsStatusAPI(statusV2RR, statusV2Req)

	var statusV2 models.TermsStatusResponse
	_ = json.NewDecoder(statusV2RR.Body).Decode(&statusV2)
	if statusV2.Accepted {
		t.Errorf("Expected accepted=false after terms version was bumped to v2.0")
	}
	if statusV2.RequiredVersion != "v2.0" {
		t.Errorf("Expected required_version=v2.0, got %s", statusV2.RequiredVersion)
	}
	if statusV2.UserVersion != "v1.0" {
		t.Errorf("Expected previous accepted version to be v1.0, got %s", statusV2.UserVersion)
	}

	// Sending message is blocked again until v2.0 is accepted
	sendBlockedV2Req := createAuthenticatedRequest(http.MethodPost, "/messages/send", payloadBytes, 1)
	sendBlockedV2RR := httptest.NewRecorder()

	SendMessageAPI(sendBlockedV2RR, sendBlockedV2Req)

	if sendBlockedV2RR.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for SendMessageAPI on unaccepted v2.0 terms, got %d", sendBlockedV2RR.Code)
	}
}

func TestDeclineTermsAPI_GracefulLogout(t *testing.T) {
	cleanup := setupTermsHandlerTestDB(t)
	defer cleanup()

	// 1. Decline via HTML form POST redirects to /terms?declined=1
	form := url.Values{}
	declineReq := httptest.NewRequest(http.MethodPost, "/terms/decline", strings.NewReader(form.Encode()))
	declineReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	declineRR := httptest.NewRecorder()

	DeclineTermsAPI(declineRR, declineReq)

	if declineRR.Code != http.StatusSeeOther {
		t.Fatalf("Expected 303 See Other for decline redirect, got %d", declineRR.Code)
	}
	location := declineRR.Header().Get("Location")
	if location != "/terms?declined=1" {
		t.Errorf("Expected redirect to /terms?declined=1, got %s", location)
	}

	// 2. Decline via JSON request returns clean JSON
	jsonReq := httptest.NewRequest(http.MethodPost, "/terms/decline", nil)
	jsonReq.Header.Set("Accept", "application/json")
	jsonRR := httptest.NewRecorder()

	DeclineTermsAPI(jsonRR, jsonReq)

	if jsonRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for JSON decline, got %d", jsonRR.Code)
	}

	var jsonResp map[string]interface{}
	_ = json.NewDecoder(jsonRR.Body).Decode(&jsonResp)
	if jsonResp["declined"] != true {
		t.Errorf("Expected declined=true in JSON response")
	}

	// 3. Visiting /terms?declined=1 renders informative notice without errors
	termsReq := httptest.NewRequest(http.MethodGet, "/terms?declined=1", nil)
	termsRR := httptest.NewRecorder()

	TermsOfUseHandler(termsRR, termsReq)

	if termsRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /terms?declined=1, got %d", termsRR.Code)
	}
	body := termsRR.Body.String()
	if !strings.Contains(body, "Terms Update Declined") {
		t.Errorf("Expected 'Terms Update Declined' alert in rendered terms template")
	}
	if !strings.Contains(body, "Active Version:") {
		t.Errorf("Expected active terms version badge in rendered terms template")
	}
}

func TestRequireTermsAcceptanceMiddleware(t *testing.T) {
	cleanup := setupTermsHandlerTestDB(t)
	defer cleanup()

	protectedCalled := false
	dummyHandler := func(w http.ResponseWriter, r *http.Request) {
		protectedCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("protected content"))
	}

	wrapped := middleware.RequireTermsAcceptance(dummyHandler)

	// User 1 has not accepted terms yet
	pageReq := createAuthenticatedRequest(http.MethodGet, "/community/dashboard", nil, 1)
	pageRR := httptest.NewRecorder()

	wrapped(pageRR, pageReq)

	if protectedCalled {
		t.Errorf("Protected handler should not have been called for unaccepted user")
	}
	if pageRR.Code != http.StatusSeeOther {
		t.Errorf("Expected 303 redirect for unaccepted user on page request, got %d", pageRR.Code)
	}
	if loc := pageRR.Header().Get("Location"); loc != "/terms?prompt=1" {
		t.Errorf("Expected redirect to /terms?prompt=1, got %s", loc)
	}

	// JSON request blocked with 403 Forbidden
	jsonReq := createAuthenticatedRequest(http.MethodGet, "/messages/data", nil, 1)
	jsonReq.Header.Set("Accept", "application/json")
	jsonRR := httptest.NewRecorder()

	wrapped(jsonRR, jsonReq)

	if jsonRR.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for JSON request to unaccepted user, got %d", jsonRR.Code)
	}

	// User accepts terms
	_ = services.AcceptTerms(1, "v1.0", "127.0.0.1", "TestRunner")

	// Protected handler should now execute
	okReq := createAuthenticatedRequest(http.MethodGet, "/community/dashboard", nil, 1)
	okRR := httptest.NewRecorder()

	wrapped(okRR, okReq)

	if !protectedCalled {
		t.Errorf("Expected protected handler to execute after terms acceptance")
	}
	if okRR.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for accepted user, got %d", okRR.Code)
	}
}
