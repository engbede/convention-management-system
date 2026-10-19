package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/services"
)

// GetVAPIDPublicKeyAPI returns the server's public VAPID key in JSON format.
func GetVAPIDPublicKeyAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"public_key": services.GetVAPIDPublicKey(),
	})
}

// SubscribePushAPI registers or updates a Web Push subscription for the authenticated user.
func SubscribePushAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload models.PushSubscriptionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid subscription JSON payload", http.StatusBadRequest)
		return
	}

	payload.Endpoint = strings.TrimSpace(payload.Endpoint)
	payload.Keys.P256dh = strings.TrimSpace(payload.Keys.P256dh)
	payload.Keys.Auth = strings.TrimSpace(payload.Keys.Auth)

	if payload.Endpoint == "" || payload.Keys.P256dh == "" || payload.Keys.Auth == "" {
		http.Error(w, "Endpoint and cryptographic keys (p256dh, auth) are required", http.StatusBadRequest)
		return
	}

	userAgent := r.UserAgent()
	if payload.UserAgent != "" {
		userAgent = payload.UserAgent
	}

	sub := &models.PushSubscription{
		UserID:    currentUserID,
		Endpoint:  payload.Endpoint,
		P256dh:    payload.Keys.P256dh,
		Auth:      payload.Keys.Auth,
		UserAgent: userAgent,
	}

	if err := repository.SavePushSubscription(sub); err != nil {
		http.Error(w, "Failed to persist push subscription", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Push subscription registered successfully",
	})
}

// UnsubscribePushAPI removes a registered Web Push subscription for the authenticated user.
func UnsubscribePushAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	endpoint := strings.TrimSpace(req.Endpoint)
	if endpoint == "" {
		http.Error(w, "Endpoint required", http.StatusBadRequest)
		return
	}

	if err := repository.DeletePushSubscriptionByUserAndEndpoint(currentUserID, endpoint); err != nil {
		http.Error(w, "Failed to delete push subscription", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Push subscription removed successfully",
	})
}

// GetCallStatusAPI checks the authoritative backend lifecycle state of a call.
func GetCallStatusAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	callIDStr := r.URL.Query().Get("call_id")
	callID, err := strconv.Atoi(callIDStr)
	if err != nil || callID <= 0 {
		http.Error(w, "Valid call_id required", http.StatusBadRequest)
		return
	}

	callLog, err := repository.GetCallLogByID(callID)
	if err != nil || callLog == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"call_id":   callID,
			"is_active": false,
			"status":    "not_found",
			"error":     "Call record not found",
		})
		return
	}

	// Verify authorization: current user must be caller, receiver, or a conversation member
	if callLog.CallerID != currentUserID && callLog.ReceiverID != currentUserID {
		isMember, mErr := repository.IsUserInConversation(callLog.ConversationID, currentUserID)
		if mErr != nil || !isMember {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	// Check if call has timed out
	isActive := callLog.Status == "initiated" || callLog.Status == "ringing"
	if isActive && callLog.ExpiresAt != nil && time.Now().After(*callLog.ExpiresAt) {
		isActive = false
		_ = repository.UpdateCallLog(callID, "missed", 0)
		callLog.Status = "missed"
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"call_id":         callLog.ID,
		"conversation_id": callLog.ConversationID,
		"caller_id":       callLog.CallerID,
		"receiver_id":     callLog.ReceiverID,
		"call_type":       callLog.CallType,
		"status":          callLog.Status,
		"is_active":       isActive,
		"started_at":      callLog.StartedAt,
		"expires_at":      callLog.ExpiresAt,
	})
}

