package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCommunityGuidelinesHandler(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/community/guidelines", nil)
	rr := httptest.NewRecorder()

	CommunityGuidelinesHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK for /community/guidelines, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Youth Community Guidelines") {
		t.Errorf("Expected 'Youth Community Guidelines' title in body")
	}
	if !strings.Contains(body, "1 Timothy 4:12") {
		t.Errorf("Expected 1 Timothy 4:12 scripture citation in body")
	}
	if !strings.Contains(body, "Faith") || !strings.Contains(body, "Fellowship") || !strings.Contains(body, "Unity") {
		t.Errorf("Expected core fellowship values in body")
	}
	if !strings.Contains(body, "/community/login") {
		t.Errorf("Expected link to /community/login in body")
	}
}

func TestPrivacyPolicyHandler(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/privacy", nil)
	rr := httptest.NewRecorder()

	PrivacyPolicyHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK for /privacy, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Platform Privacy Policy") {
		t.Errorf("Expected 'Platform Privacy Policy' title in body")
	}
	if !strings.Contains(body, "Information We Collect") {
		t.Errorf("Expected 'Information We Collect' section in body")
	}
	if !strings.Contains(body, "bcrypt") {
		t.Errorf("Expected bcrypt password hashing mention in body")
	}
	if !strings.Contains(body, "/community/login") {
		t.Errorf("Expected link to /community/login in body")
	}
}

func TestTermsOfUseHandler(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/terms", nil)
	rr := httptest.NewRecorder()

	TermsOfUseHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK for /terms, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Terms of Use") {
		t.Errorf("Expected 'Terms of Use' title in body")
	}
	if !strings.Contains(body, "Acceptance of Terms") {
		t.Errorf("Expected 'Acceptance of Terms' section in body")
	}
	if !strings.Contains(body, "/community/login") {
		t.Errorf("Expected link to /community/login in body")
	}
}
