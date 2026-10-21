package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"convention-management-system/database"
	"convention-management-system/services"
	"convention-management-system/sessions"
)

func isFriendJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/json") || r.Header.Get("X-Requested-With") == "XMLHttpRequest"
}

func sendFriendJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func sendFriendJSONError(w http.ResponseWriter, status int, message string) {
	sendFriendJSON(w, status, map[string]any{
		"success": false,
		"error":   message,
	})
}

// SendFriendRequest handles sending a friend request.
func SendFriendRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/friends", http.StatusSeeOther)
		return
	}

	session, _ := sessions.Store.Get(r, "youth-community")
	senderID, ok := session.Values["user_id"].(int)
	if !ok || senderID <= 0 {
		if isFriendJSON(r) {
			sendFriendJSONError(w, http.StatusUnauthorized, "Authentication required")
			return
		}
		http.Redirect(w, r, "/community/login", http.StatusSeeOther)
		return
	}

	receiver := r.FormValue("receiver_id")
	if receiver == "" {
		receiver = r.FormValue("user_id")
	}

	receiverID, err := strconv.Atoi(receiver)
	if err != nil || receiverID <= 0 {
		if isFriendJSON(r) {
			sendFriendJSONError(w, http.StatusBadRequest, "Invalid recipient ID")
			return
		}
		http.Error(w, "Invalid recipient", http.StatusBadRequest)
		return
	}

	if senderID == receiverID {
		if isFriendJSON(r) {
			sendFriendJSONError(w, http.StatusBadRequest, "You cannot send a friend request to yourself")
			return
		}
		http.Error(w, "You cannot send a friend request to yourself", http.StatusBadRequest)
		return
	}

	err = services.SendFriendRequest(senderID, receiverID)
	if err != nil {
		if isFriendJSON(r) {
			sendFriendJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if isFriendJSON(r) {
		sendFriendJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"status":  "pending",
			"message": "Friend request sent successfully",
		})
		return
	}

	if username := r.FormValue("username"); username != "" {
		http.Redirect(w, r, "/user/"+username, http.StatusSeeOther)
		return
	}

	if ref := r.Header.Get("Referer"); ref != "" && !strings.Contains(ref, "/friend") {
		http.Redirect(w, r, ref, http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/friends", http.StatusSeeOther)
}

// AcceptFriendRequest accepts an incoming pending friend request.
// Only the intended recipient may accept the request.
func AcceptFriendRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/friends", http.StatusSeeOther)
		return
	}

	session, _ := sessions.Store.Get(r, "youth-community")
	receiverID, ok := session.Values["user_id"].(int)
	if !ok || receiverID <= 0 {
		if isFriendJSON(r) {
			sendFriendJSONError(w, http.StatusUnauthorized, "Authentication required")
			return
		}
		http.Redirect(w, r, "/community/login", http.StatusSeeOther)
		return
	}

	requestID, err := strconv.Atoi(r.FormValue("request_id"))
	if err != nil || requestID <= 0 {
		senderID, _ := strconv.Atoi(r.FormValue("sender_id"))
		if senderID <= 0 {
			senderID, _ = strconv.Atoi(r.FormValue("user_id"))
		}
		if senderID > 0 && database.DB != nil {
			_ = database.DB.QueryRow(`
				SELECT id FROM friend_requests
				WHERE sender_id = $1 AND receiver_id = $2 AND status = 'pending'
			`, senderID, receiverID).Scan(&requestID)
		}
	}
	if requestID <= 0 {
		if isFriendJSON(r) {
			sendFriendJSONError(w, http.StatusBadRequest, "Invalid request ID")
			return
		}
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	err = services.AcceptFriendRequest(requestID, receiverID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "unauthorized") {
			status = http.StatusForbidden
		} else if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no longer pending") {
			status = http.StatusBadRequest
		}

		if isFriendJSON(r) {
			sendFriendJSONError(w, status, err.Error())
			return
		}
		http.Error(w, err.Error(), status)
		return
	}

	if isFriendJSON(r) {
		sendFriendJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"status":  "friends",
			"message": "Friend request accepted",
		})
		return
	}

	target := "/friends"
	if ref := r.Header.Get("Referer"); ref != "" && strings.Contains(ref, "friend-requests") {
		target = "/friend-requests"
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// DeclineFriendRequest rejects an incoming pending friend request.
// Only the intended recipient may decline the request.
func DeclineFriendRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/friends", http.StatusSeeOther)
		return
	}

	session, _ := sessions.Store.Get(r, "youth-community")
	receiverID, ok := session.Values["user_id"].(int)
	if !ok || receiverID <= 0 {
		if isFriendJSON(r) {
			sendFriendJSONError(w, http.StatusUnauthorized, "Authentication required")
			return
		}
		http.Redirect(w, r, "/community/login", http.StatusSeeOther)
		return
	}

	requestID, err := strconv.Atoi(r.FormValue("request_id"))
	if err != nil || requestID <= 0 {
		senderID, _ := strconv.Atoi(r.FormValue("sender_id"))
		if senderID <= 0 {
			senderID, _ = strconv.Atoi(r.FormValue("user_id"))
		}
		if senderID > 0 && database.DB != nil {
			_ = database.DB.QueryRow(`
				SELECT id FROM friend_requests
				WHERE sender_id = $1 AND receiver_id = $2 AND status = 'pending'
			`, senderID, receiverID).Scan(&requestID)
		}
	}
	if requestID <= 0 {
		if isFriendJSON(r) {
			sendFriendJSONError(w, http.StatusBadRequest, "Invalid request ID")
			return
		}
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	err = services.DeclineFriendRequest(requestID, receiverID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "unauthorized") {
			status = http.StatusForbidden
		} else if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no longer pending") {
			status = http.StatusBadRequest
		}

		if isFriendJSON(r) {
			sendFriendJSONError(w, status, err.Error())
			return
		}
		http.Error(w, err.Error(), status)
		return
	}

	if isFriendJSON(r) {
		sendFriendJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"status":  "none",
			"message": "Friend request declined",
		})
		return
	}

	target := "/friends"
	if ref := r.Header.Get("Referer"); ref != "" && strings.Contains(ref, "friend-requests") {
		target = "/friend-requests"
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// CancelFriendRequest cancels an outgoing pending friend request.
// Only the original sender may cancel the request.
func CancelFriendRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/friends", http.StatusSeeOther)
		return
	}

	session, _ := sessions.Store.Get(r, "youth-community")
	senderID, ok := session.Values["user_id"].(int)
	if !ok || senderID <= 0 {
		if isFriendJSON(r) {
			sendFriendJSONError(w, http.StatusUnauthorized, "Authentication required")
			return
		}
		http.Redirect(w, r, "/community/login", http.StatusSeeOther)
		return
	}

	requestIDStr := r.FormValue("request_id")
	receiver := r.FormValue("receiver_id")
	if receiver == "" {
		receiver = r.FormValue("user_id")
	}

	var cancelErr error
	if requestIDStr != "" {
		if reqID, err := strconv.Atoi(requestIDStr); err == nil && reqID > 0 {
			cancelErr = services.CancelFriendRequestByID(reqID, senderID)
		} else {
			cancelErr = err
		}
	} else if receiver != "" {
		if recID, err := strconv.Atoi(receiver); err == nil && recID > 0 {
			cancelErr = services.CancelFriendRequest(senderID, recID)
		} else {
			cancelErr = err
		}
	} else {
		cancelErr = errors.New("friend request not found")
	}

	if cancelErr != nil {
		status := http.StatusInternalServerError
		if strings.Contains(cancelErr.Error(), "unauthorized") {
			status = http.StatusForbidden
		} else if strings.Contains(cancelErr.Error(), "not found") || strings.Contains(cancelErr.Error(), "no longer pending") {
			status = http.StatusBadRequest
		}

		if isFriendJSON(r) {
			sendFriendJSONError(w, status, cancelErr.Error())
			return
		}
		http.Error(w, cancelErr.Error(), status)
		return
	}

	if isFriendJSON(r) {
		sendFriendJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"status":  "none",
			"message": "Friend request cancelled",
		})
		return
	}

	if username := r.FormValue("username"); username != "" {
		http.Redirect(w, r, "/user/"+username, http.StatusSeeOther)
		return
	}

	if ref := r.Header.Get("Referer"); ref != "" && !strings.Contains(ref, "/friend") {
		http.Redirect(w, r, ref, http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/friends", http.StatusSeeOther)
}

// RemoveFriend terminates a friendship between two users.
func RemoveFriend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/friends", http.StatusSeeOther)
		return
	}

	session, _ := sessions.Store.Get(r, "youth-community")
	user1ID, ok := session.Values["user_id"].(int)
	if !ok || user1ID <= 0 {
		if isFriendJSON(r) {
			sendFriendJSONError(w, http.StatusUnauthorized, "Authentication required")
			return
		}
		http.Redirect(w, r, "/community/login", http.StatusSeeOther)
		return
	}

	user2ID, err := strconv.Atoi(r.FormValue("user_id"))
	if err != nil || user2ID <= 0 {
		user2ID, err = strconv.Atoi(r.FormValue("friend_id"))
	}
	if err != nil || user2ID <= 0 {
		if isFriendJSON(r) {
			sendFriendJSONError(w, http.StatusBadRequest, "Invalid user")
			return
		}
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	err = services.RemoveFriend(user1ID, user2ID)
	if err != nil {
		if isFriendJSON(r) {
			sendFriendJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if isFriendJSON(r) {
		sendFriendJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"status":  "none",
			"message": "Friend removed",
		})
		return
	}

	if username := r.FormValue("username"); username != "" {
		http.Redirect(w, r, "/user/"+username, http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/friends", http.StatusSeeOther)
}
