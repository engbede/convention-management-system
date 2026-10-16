package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"convention-management-system/models"
	"convention-management-system/services"
	"convention-management-system/sessions"
)

// ReactToPost adds, modifies, or toggles off a reaction on a post.
func ReactToPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/community", http.StatusSeeOther)
		return
	}

	session, _ := sessions.Store.Get(r, "youth-community")
	userID, ok := session.Values["user_id"].(int)
	if !ok || userID == 0 {
		if strings.Contains(r.Header.Get("Accept"), "application/json") || strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Unauthorized"})
			return
		}
		http.Redirect(w, r, "/community/login", http.StatusSeeOther)
		return
	}

	var postID int
	var reaction string
	isJSON := strings.Contains(r.Header.Get("Content-Type"), "application/json")
	if isJSON {
		var payload struct {
			PostID   int    `json:"post_id"`
			Reaction string `json:"reaction"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		postID = payload.PostID
		reaction = payload.Reaction
	} else {
		_ = r.ParseForm()
		postID, _ = strconv.Atoi(r.FormValue("post_id"))
		reaction = r.FormValue("reaction")
	}

	isAJAX := strings.Contains(r.Header.Get("Accept"), "application/json") ||
		isJSON ||
		r.Header.Get("X-Requested-With") == "XMLHttpRequest"

	if postID <= 0 || strings.TrimSpace(reaction) == "" {
		if isAJAX {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Invalid post or reaction"})
			return
		}
		http.Redirect(w, r, "/community", http.StatusSeeOther)
		return
	}

	err := services.ReactToPost(postID, userID, reaction)
	if err != nil {
		if isAJAX {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if isAJAX {
		stats, err := services.GetPostReactionData(postID, userID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "post_id": postID})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"stats":   stats,
		})
		return
	}

	http.Redirect(w, r, "/community", http.StatusSeeOther)
}

// GetPostReactionsAPI returns the catalog and reacted users for a post.
func GetPostReactionsAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	postID, err := strconv.Atoi(r.URL.Query().Get("post_id"))
	if err != nil || postID <= 0 {
		http.Error(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	stats, err := services.GetPostReactionData(postID, currentUserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"stats":   stats,
		"catalog": models.GetGroupedReactionCatalog(),
	})
}
