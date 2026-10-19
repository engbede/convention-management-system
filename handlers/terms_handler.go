package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"convention-management-system/models"
	"convention-management-system/services"
	"convention-management-system/sessions"
)

// TermsStatusAPI returns the current user's terms acceptance compliance state.
func TermsStatusAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	accepted, reqVer, latest, err := services.CheckTermsAcceptance(currentUserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Failed to check terms compliance",
		})
		return
	}

	resp := models.TermsStatusResponse{
		Accepted:        accepted,
		RequiredVersion: reqVer,
		Record:          latest,
	}
	if latest != nil {
		resp.UserVersion = latest.TermsVersion
		resp.AcceptedAt = &latest.AcceptedAt
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// AcceptTermsAPI processes a member's agreement to the active terms of use.
func AcceptTermsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var version string
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var req models.TermsAcceptRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			version = req.Version
		}
	} else {
		version = r.FormValue("version")
	}

	// Capture IP Address and User Agent for audit record
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.RemoteAddr
	}
	userAgent := r.UserAgent()

	if err := services.AcceptTerms(currentUserID, version, ip, userAgent); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"version": services.GetActiveTermsVersion(),
		"message": "Terms accepted successfully",
	})
}

// DeclineTermsAPI handles a member gracefully declining terms update without conflating with suspension.
func DeclineTermsAPI(w http.ResponseWriter, r *http.Request) {
	// Log user out cleanly
	session, err := sessions.Store.Get(r, "youth-community")
	if err == nil && session != nil {
		session.Options.MaxAge = -1
		_ = session.Save(r, w)
	}

	isJSON := strings.Contains(r.Header.Get("Accept"), "application/json") ||
		strings.Contains(r.Header.Get("Content-Type"), "application/json")

	if isJSON {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"declined": true,
			"redirect": "/terms?declined=1",
		})
		return
	}

	http.Redirect(w, r, "/terms?declined=1", http.StatusSeeOther)
}

