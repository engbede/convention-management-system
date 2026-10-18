package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/sessions"
)

// SubmitCommunityPrayerRequest handles new prayer request submissions from the community sidebar
func SubmitCommunityPrayerRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/community", http.StatusSeeOther)
		return
	}

	session, err := sessions.Store.Get(r, "youth-community")
	if err != nil {
		http.Redirect(w, r, "/community/login", http.StatusSeeOther)
		return
	}

	userID, ok := session.Values["user_id"].(int)
	if !ok || userID <= 0 {
		http.Redirect(w, r, "/community/login", http.StatusSeeOther)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	description := strings.TrimSpace(r.FormValue("description"))
	privacyScope := strings.TrimSpace(r.FormValue("privacy_scope"))

	if title == "" || description == "" {
		if r.Header.Get("X-Requested-With") == "XMLHttpRequest" || strings.Contains(r.Header.Get("Accept"), "application/json") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   "Please enter both a title and description for your prayer request.",
			})
			return
		}
		http.Redirect(w, r, "/community?error=empty_prayer", http.StatusSeeOther)
		return
	}

	// Validate privacy scope
	validScopes := map[string]bool{
		"public":     true,
		"fellowship": true,
		"pastoral":   true,
		"private":    true,
	}
	if !validScopes[privacyScope] {
		privacyScope = "public"
	}

	prayerReq := &models.CommunityPrayerRequest{
		UserID:       userID,
		Title:        title,
		Description:  description,
		PrivacyScope: privacyScope,
		Status:       "active",
	}

	if err := repository.CreateCommunityPrayerRequest(prayerReq); err != nil {
		if r.Header.Get("X-Requested-With") == "XMLHttpRequest" || strings.Contains(r.Header.Get("Accept"), "application/json") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   "Failed to submit prayer request. Please try again later.",
			})
			return
		}
		http.Redirect(w, r, "/community?error=db_error", http.StatusSeeOther)
		return
	}

	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" || strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"message": "Prayer request shared with the fellowship.",
			"id":      prayerReq.ID,
		})
		return
	}

	http.Redirect(w, r, "/community?success=prayer_submitted", http.StatusSeeOther)
}

// TogglePrayerIntercessionAPI allows logged-in members to indicate they have prayed for a request
func TogglePrayerIntercessionAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session, err := sessions.Store.Get(r, "youth-community")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Unauthorized"})
		return
	}

	userID, ok := session.Values["user_id"].(int)
	if !ok || userID <= 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Unauthorized"})
		return
	}

	reqIDStr := r.FormValue("request_id")
	reqID, err := strconv.Atoi(reqIDStr)
	if err != nil || reqID <= 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Invalid request ID"})
		return
	}

	hasPrayed, newCount, err := repository.TogglePrayerRequestPray(reqID, userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":    true,
		"has_prayed": hasPrayed,
		"pray_count": newCount,
	})
}

