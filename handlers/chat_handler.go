package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/services"
	"convention-management-system/sessions"
	"convention-management-system/utils"

	"golang.org/x/net/websocket"
)

// getCurrentUserID retrieves the authenticated member's ID from session.
func getCurrentUserID(r *http.Request) int {
	session, err := sessions.Store.Get(r, "youth-community")
	if err != nil {
		return 0
	}
	userID, ok := session.Values["user_id"].(int)
	if !ok {
		return 0
	}
	return userID
}

// ChatInbox renders the main Youth Chat interface.
func ChatInbox(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Redirect(w, r, "/community/login", http.StatusSeeOther)
		return
	}

	currentUser, err := repository.GetUserByID(currentUserID)
	if err != nil {
		http.Error(w, "Unable to load user profile", http.StatusInternalServerError)
		return
	}
	currentUser.Initials = utils.Initials(currentUser.FullName)

	// Fetch conversations list
	var inboxError string
	conversations, unreadMsgs, err := services.GetUserInbox(currentUserID)
	if err != nil {
		log.Printf("[Chat Inbox Error] user_id=%d failed to load inbox: %v", currentUserID, err)
		inboxError = "Unable to load conversations at this moment. Please refresh."
		conversations = []models.ConversationItem{}
	}

	// Determine active conversation
	var activeConv *models.Conversation
	var activeRecipient *models.User
	var messages []models.Message
	var isActiveMuted, isActiveArchived, hasOlder bool

	convParam := r.URL.Query().Get("c")
	activeConvID := 0
	if convParam != "" {
		activeConvID, _ = strconv.Atoi(convParam)
	}

	// If no specific conversation requested, pick the first one if available
	if activeConvID == 0 && len(conversations) > 0 {
		activeConvID = conversations[0].ID
	}

	if activeConvID > 0 {
		conv, recipient, err := services.GetConversationDetails(activeConvID, currentUserID)
		if err == nil {
			activeConv = conv
			activeRecipient = recipient
			isActiveMuted, isActiveArchived, _ = services.GetConversationMemberSettings(activeConvID, currentUserID)
			msgs, err := services.GetConversationMessages(activeConvID, currentUserID, 50, 0)
			if err == nil {
				messages = msgs
				hasOlder = services.HasOlderMessages(activeConvID, len(msgs))
			}
		}
	}

	// Mark active conversation in the list
	for i := range conversations {
		if activeConv != nil && conversations[i].ID == activeConv.ID {
			conversations[i].IsActive = true
		}
	}

	// Fetch candidate users to start a new chat with
	eligibleFriends, _ := services.GetEligibleDirectChatUsers(currentUserID)

	// Notification badge count for navbar
	unreadNotifications, _ := services.CountUnreadNotifications(currentUserID)

	var isActiveOnline bool
	var isRecipientBlocked, isBlockedByRecipient bool
	var sharedMedia []models.Message
	var callLogs []models.CallLog
	var activeGroupMembers []models.ConversationMember
	var activeUserRole string
	var canManageMembers bool
	var pendingRequests []models.GroupMembershipRequest
	var pendingRequestsCount int
	canStartCall := true

	if activeRecipient != nil {
		isActiveOnline = services.GlobalHub.IsUserOnline(activeRecipient.ID)
		isRecipientBlocked, _ = services.IsUserBlocked(currentUserID, activeRecipient.ID)
		isBlockedByRecipient, _ = services.IsUserBlocked(activeRecipient.ID, currentUserID)
		canStartCall = !isRecipientBlocked && !isBlockedByRecipient
	}

	if activeConv != nil {
		sharedMedia, _ = services.GetSharedMedia(activeConv.ID, currentUserID)
		callLogs, _ = services.GetConversationCallLogs(activeConv.ID, currentUserID)

		if activeConv.Type == "group" {
			activeGroupMembers, _ = services.GetGroupMembers(activeConv.ID, currentUserID)
			member, _ := repository.GetConversationMember(activeConv.ID, currentUserID)
			if member != nil {
				activeUserRole = member.Role
				canManageMembers = (member.Role == "owner" || member.Role == "admin" || member.Role == "moderator")
				if member.Role == "owner" || member.Role == "admin" {
					reqs, _ := services.GetGroupMembershipRequests(currentUserID, activeConv.ID, "pending")
					pendingRequests = reqs
					pendingRequestsCount = len(reqs)
				}
			}
			canStartCall, _ = services.CanUserCallInGroup(activeConv.ID, currentUserID)
		}
	}

	var activeGroupCall *models.GroupCallRoom
	if activeConv != nil && activeConv.Type == "group" {
		if room, exists := services.GlobalGroupCallManager.GetActiveGroupCall(activeConv.ID); exists {
			activeGroupCall = room
		}
	}

	discoverableGroups, _ := services.GetDiscoverableGroups(currentUserID, "")

	data := models.ChatViewData{
		CurrentUser:          currentUser,
		Error:                inboxError,
		Conversations:        conversations,
		ActiveConversation:   activeConv,
		ActiveRecipient:      activeRecipient,
		ActiveGroupMembers:   activeGroupMembers,
		ActiveUserRole:       activeUserRole,
		CanStartCall:         canStartCall,
		CanManageMembers:     canManageMembers,
		PendingRequestsCount: pendingRequestsCount,
		PendingRequests:      pendingRequests,
		DiscoverableGroups:   discoverableGroups,
		Messages:             messages,
		EligibleFriends:      eligibleFriends,
		UnreadCount:          unreadNotifications,
		UnreadMessagesCount:  unreadMsgs,
		IsActiveMuted:        isActiveMuted,
		IsActiveArchived:     isActiveArchived,
		HasOlderMessages:     hasOlder,
		IsActiveOnline:       isActiveOnline,
		IsRecipientBlocked:   isRecipientBlocked,
		IsBlockedByRecipient: isBlockedByRecipient,
		SharedMedia:          sharedMedia,
		CallLogs:             callLogs,
		ActiveGroupCall:      activeGroupCall,
	}

	Render(w, "chat.html", data)
}

// StartDirectChat starts or opens a direct 1-to-1 conversation with a selected member.
func StartDirectChat(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Redirect(w, r, "/community/login", http.StatusSeeOther)
		return
	}

	targetIDStr := r.URL.Query().Get("user_id")
	if targetIDStr == "" {
		targetIDStr = r.FormValue("user_id")
	}

	targetUserID, err := strconv.Atoi(targetIDStr)
	if err != nil || targetUserID <= 0 {
		http.Error(w, "Invalid target user ID", http.StatusBadRequest)
		return
	}

	conv, err := services.StartDirectConversation(currentUserID, targetUserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", conv.ID), http.StatusSeeOther)
}

// SendMessageAPI handles sending a message via AJAX JSON or standard HTML Form POST.
func SendMessageAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse parameters
	var (
		convID      int
		body        string
		clientMsgID string
		replyToID   *int
		file        multipart.File
		fileHeader  *multipart.FileHeader
	)

	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "multipart/form-data") {
		_ = r.ParseMultipartForm(10 << 20) // 10 MB
		convID, _ = strconv.Atoi(r.FormValue("conversation_id"))
		body = r.FormValue("body")
		clientMsgID = r.FormValue("client_message_id")
		if r.FormValue("reply_to_id") != "" {
			if id, err := strconv.Atoi(r.FormValue("reply_to_id")); err == nil {
				replyToID = &id
			}
		}
		var err error
		file, fileHeader, err = r.FormFile("attachment")
		if err != nil {
			file, fileHeader, _ = r.FormFile("file")
		}
	} else if strings.Contains(contentType, "application/json") {
		var req struct {
			ConversationID  int    `json:"conversation_id"`
			Body            string `json:"body"`
			ClientMessageID string `json:"client_message_id"`
			ReplyToID       *int   `json:"reply_to_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}
		convID = req.ConversationID
		body = req.Body
		clientMsgID = req.ClientMessageID
		replyToID = req.ReplyToID
	} else {
		_ = r.ParseForm()
		convID, _ = strconv.Atoi(r.FormValue("conversation_id"))
		body = r.FormValue("body")
		clientMsgID = r.FormValue("client_message_id")
		if r.FormValue("reply_to_id") != "" {
			if id, err := strconv.Atoi(r.FormValue("reply_to_id")); err == nil {
				replyToID = &id
			}
		}
	}

	if convID <= 0 {
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	var (
		msg *models.Message
		err error
	)

	if file != nil && fileHeader != nil {
		msg, err = services.SendAttachmentMessage(convID, currentUserID, file, fileHeader, body, clientMsgID, replyToID)
	} else {
		msg, err = services.SendMessage(convID, currentUserID, body, clientMsgID, replyToID)
	}

	if err != nil {
		if strings.Contains(err.Error(), "access denied") {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Respond with JSON if requested or AJAX
	isAJAX := strings.Contains(r.Header.Get("Accept"), "application/json") ||
		strings.Contains(contentType, "application/json") ||
		r.Header.Get("X-Requested-With") == "XMLHttpRequest" ||
		r.URL.Path == "/messages/upload"

	if isAJAX {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"message": msg,
		})
		return
	}

	// Otherwise standard redirect back to conversation
	http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", convID), http.StatusSeeOther)
}

// UploadAttachmentAPI handles dedicated multipart upload endpoint for attachments.
func UploadAttachmentAPI(w http.ResponseWriter, r *http.Request) {
	SendMessageAPI(w, r)
}

// GetMessagesAPI retrieves paginated messages for a conversation as JSON.
func GetMessagesAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	convID, err := strconv.Atoi(r.URL.Query().Get("conversation_id"))
	if err != nil || convID <= 0 {
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 && val <= 100 {
			limit = val
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}

	messages, err := services.GetConversationMessages(convID, currentUserID, limit, offset)
	if err != nil {
		if strings.Contains(err.Error(), "access denied") {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":   true,
		"messages":  messages,
		"has_older": services.HasOlderMessages(convID, offset+len(messages)),
	})
}

// MarkReadAPI advances the current user's read marker for a conversation.
func MarkReadAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	convID, err := strconv.Atoi(r.URL.Query().Get("conversation_id"))
	if err != nil || convID <= 0 {
		convID, _ = strconv.Atoi(r.FormValue("conversation_id"))
	}

	if convID <= 0 {
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	isMember, _ := repository.IsConversationMember(convID, currentUserID)
	if !isMember {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	_ = services.MarkConversationAsRead(convID, currentUserID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
}

// ToggleMuteAPI toggles mute for a conversation.
func ToggleMuteAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	convID, err := strconv.Atoi(r.URL.Query().Get("conversation_id"))
	if err != nil || convID <= 0 {
		convID, _ = strconv.Atoi(r.FormValue("conversation_id"))
	}

	if convID <= 0 {
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	isMuted, err := services.ToggleMuteConversation(convID, currentUserID)
	if err != nil {
		if strings.Contains(err.Error(), "access denied") {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if strings.Contains(r.Header.Get("Accept"), "application/json") || strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":  true,
			"is_muted": isMuted,
		})
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", convID), http.StatusSeeOther)
}

// ToggleArchiveAPI toggles archive state for a conversation.
func ToggleArchiveAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	convID, err := strconv.Atoi(r.URL.Query().Get("conversation_id"))
	if err != nil || convID <= 0 {
		convID, _ = strconv.Atoi(r.FormValue("conversation_id"))
	}

	if convID <= 0 {
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	isArchived, err := services.ToggleArchiveConversation(convID, currentUserID)
	if err != nil {
		if strings.Contains(err.Error(), "access denied") {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if strings.Contains(r.Header.Get("Accept"), "application/json") || strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":     true,
			"is_archived": isArchived,
		})
		return
	}

	http.Redirect(w, r, "/messages", http.StatusSeeOther)
}

// ChatWebSocketHandler handles WebSocket upgrade for real-time chat.
func ChatWebSocketHandler(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := repository.GetUserByID(currentUserID)
	if err != nil || user.ID == 0 {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	handler := websocket.Handler(func(ws *websocket.Conn) {
		services.ServeWebSocket(ws, user.ID, user.Username, user.FullName)
	})
	handler.ServeHTTP(w, r)
}

// BlockUserAPI handles blocking a member.
func BlockUserAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	targetIDStr := r.URL.Query().Get("user_id")
	if targetIDStr == "" {
		targetIDStr = r.FormValue("user_id")
	}
	reason := r.FormValue("reason")

	if targetIDStr == "" && strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var payload struct {
			UserID int    `json:"user_id"`
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
			targetIDStr = strconv.Itoa(payload.UserID)
			reason = payload.Reason
		}
	}

	targetUserID, err := strconv.Atoi(targetIDStr)
	if err != nil || targetUserID <= 0 {
		http.Error(w, "Invalid user ID to block", http.StatusBadRequest)
		return
	}

	if err := services.BlockUser(currentUserID, targetUserID, reason); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if strings.Contains(r.Header.Get("Accept"), "application/json") || strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"message": "User has been blocked successfully",
		})
		return
	}

	http.Redirect(w, r, "/messages", http.StatusSeeOther)
}

// UnblockUserAPI handles unblocking a member.
func UnblockUserAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	targetIDStr := r.URL.Query().Get("user_id")
	if targetIDStr == "" {
		targetIDStr = r.FormValue("user_id")
	}

	if targetIDStr == "" && strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var payload struct {
			UserID int `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
			targetIDStr = strconv.Itoa(payload.UserID)
		}
	}

	targetUserID, err := strconv.Atoi(targetIDStr)
	if err != nil || targetUserID <= 0 {
		http.Error(w, "Invalid user ID to unblock", http.StatusBadRequest)
		return
	}

	if err := services.UnblockUser(currentUserID, targetUserID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if strings.Contains(r.Header.Get("Accept"), "application/json") || strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"message": "User has been unblocked successfully",
		})
		return
	}

	http.Redirect(w, r, "/messages", http.StatusSeeOther)
}

// ReportAbuseAPI handles submitting a moderation or abuse report.
func ReportAbuseAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var reportedUserID int
	var convID *int
	var msgID *int
	var reason string
	var details string

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var payload struct {
			ReportedUserID int    `json:"reported_user_id"`
			ConversationID *int   `json:"conversation_id"`
			MessageID      *int   `json:"message_id"`
			Reason         string `json:"reason"`
			Details        string `json:"details"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
			reportedUserID = payload.ReportedUserID
			convID = payload.ConversationID
			msgID = payload.MessageID
			reason = payload.Reason
			details = payload.Details
		}
	} else {
		_ = r.ParseForm()
		reportedUserID, _ = strconv.Atoi(r.FormValue("reported_user_id"))
		if r.FormValue("conversation_id") != "" {
			if id, err := strconv.Atoi(r.FormValue("conversation_id")); err == nil {
				convID = &id
			}
		}
		if r.FormValue("message_id") != "" {
			if id, err := strconv.Atoi(r.FormValue("message_id")); err == nil {
				msgID = &id
			}
		}
		reason = r.FormValue("reason")
		details = r.FormValue("details")
	}

	if reportedUserID <= 0 {
		http.Error(w, "Invalid reported user ID", http.StatusBadRequest)
		return
	}

	report, err := services.ReportAbuse(currentUserID, reportedUserID, convID, msgID, reason, details)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if strings.Contains(r.Header.Get("Accept"), "application/json") || strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"report":  report,
			"message": "Thank you. Your report has been submitted for moderation review.",
		})
		return
	}

	http.Redirect(w, r, "/messages", http.StatusSeeOther)
}

// SearchMessagesAPI returns JSON array of matching messages.
func SearchMessagesAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	var convID *int
	if cStr := r.URL.Query().Get("conversation_id"); cStr != "" {
		if id, err := strconv.Atoi(cStr); err == nil && id > 0 {
			convID = &id
		}
	}

	limit := 30
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if oStr := r.URL.Query().Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			offset = o
		}
	}

	messages, err := services.SearchMessages(currentUserID, query, convID, limit, offset)
	if err != nil {
		if strings.Contains(err.Error(), "access denied") {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":  true,
		"query":    query,
		"messages": messages,
		"count":    len(messages),
	})
}

// ICEServersAPI returns standard WebRTC ICE server configuration as JSON.
func ICEServersAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"iceServers": services.GetICEServers(),
	})
}

// LogCallAPI updates call completion or duration.
func LogCallAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		CallID          int    `json:"call_id"`
		Status          string `json:"status"`
		DurationSeconds int    `json:"duration_seconds"`
	}

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	} else {
		_ = r.ParseForm()
		payload.CallID, _ = strconv.Atoi(r.FormValue("call_id"))
		payload.Status = r.FormValue("status")
		payload.DurationSeconds, _ = strconv.Atoi(r.FormValue("duration_seconds"))
	}

	if payload.CallID <= 0 {
		http.Error(w, "Invalid call ID", http.StatusBadRequest)
		return
	}

	if payload.Status == "" {
		payload.Status = "completed"
	}

	if err := services.UpdateCallLog(payload.CallID, payload.Status, payload.DurationSeconds); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
	})
}

// CreateGroupAPI handles creating a new community group chat.
func CreateGroupAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Title          string `json:"title"`
		Category       string `json:"category"`
		Description    string `json:"description"`
		CallPermission string `json:"call_permission"`
		IsPublic       bool   `json:"is_public"`
		MemberIDs      []int  `json:"member_ids"`
	}

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	} else {
		_ = r.ParseForm()
		payload.Title = r.FormValue("title")
		payload.Category = r.FormValue("category")
		payload.Description = r.FormValue("description")
		payload.CallPermission = r.FormValue("call_permission")
		payload.IsPublic = r.FormValue("is_public") == "1" || r.FormValue("is_public") == "true" || r.FormValue("is_public") == ""
		if mStr := r.FormValue("member_ids"); mStr != "" {
			for _, idStr := range strings.Split(mStr, ",") {
				if id, err := strconv.Atoi(strings.TrimSpace(idStr)); err == nil && id > 0 {
					payload.MemberIDs = append(payload.MemberIDs, id)
				}
			}
		}
	}

	conv, err := services.CreateGroup(currentUserID, payload.Title, payload.Category, payload.Description, payload.CallPermission, payload.IsPublic, payload.MemberIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if strings.Contains(r.Header.Get("Accept"), "application/json") || strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":         true,
			"conversation_id": conv.ID,
			"redirect_url":    fmt.Sprintf("/messages?c=%d", conv.ID),
		})
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", conv.ID), http.StatusSeeOther)
}

// JoinGroupAPI handles a user joining a public community group.
func JoinGroupAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var groupID int
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var payload struct {
			GroupID int `json:"group_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		groupID = payload.GroupID
	} else {
		_ = r.ParseForm()
		groupID, _ = strconv.Atoi(r.FormValue("group_id"))
		if groupID == 0 {
			groupID, _ = strconv.Atoi(r.FormValue("conversation_id"))
		}
	}

	if groupID <= 0 {
		http.Error(w, "Invalid group ID", http.StatusBadRequest)
		return
	}

	if err := services.JoinGroup(currentUserID, groupID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if strings.Contains(r.Header.Get("Accept"), "application/json") || strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":         true,
			"conversation_id": groupID,
			"redirect_url":    fmt.Sprintf("/messages?c=%d", groupID),
		})
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", groupID), http.StatusSeeOther)
}

// LeaveGroupAPI handles a member leaving a community group.
func LeaveGroupAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var groupID int
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var payload struct {
			GroupID int `json:"group_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		groupID = payload.GroupID
	} else {
		_ = r.ParseForm()
		groupID, _ = strconv.Atoi(r.FormValue("group_id"))
		if groupID == 0 {
			groupID, _ = strconv.Atoi(r.FormValue("conversation_id"))
		}
	}

	if groupID <= 0 {
		http.Error(w, "Invalid group ID", http.StatusBadRequest)
		return
	}

	if err := services.LeaveGroup(currentUserID, groupID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if strings.Contains(r.Header.Get("Accept"), "application/json") || strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":      true,
			"redirect_url": "/messages",
		})
		return
	}

	http.Redirect(w, r, "/messages", http.StatusSeeOther)
}

// AddGroupMembersAPI adds selected users to a group.
func AddGroupMembersAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		GroupID      int    `json:"group_id"`
		UserIDs      []int  `json:"user_ids"`
		ManualMember string `json:"manual_member"`
		Username     string `json:"username"`
	}

	isJSON := strings.Contains(r.Header.Get("Content-Type"), "application/json")
	if isJSON {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	} else {
		_ = r.ParseForm()
		payload.GroupID, _ = strconv.Atoi(r.FormValue("group_id"))
		if payload.GroupID == 0 {
			payload.GroupID, _ = strconv.Atoi(r.FormValue("conversation_id"))
		}
		payload.ManualMember = r.FormValue("manual_member")
		if payload.ManualMember == "" {
			payload.ManualMember = r.FormValue("username")
		}
		// Handle multiple user_ids parameters from checkboxes or comma-separated lists
		for _, uVal := range r.Form["user_ids"] {
			for _, idStr := range strings.Split(uVal, ",") {
				if id, err := strconv.Atoi(strings.TrimSpace(idStr)); err == nil && id > 0 {
					payload.UserIDs = append(payload.UserIDs, id)
				}
			}
		}
		if len(payload.UserIDs) == 0 {
			if uStr := r.FormValue("user_ids"); uStr != "" {
				for _, idStr := range strings.Split(uStr, ",") {
					if id, err := strconv.Atoi(strings.TrimSpace(idStr)); err == nil && id > 0 {
						payload.UserIDs = append(payload.UserIDs, id)
					}
				}
			}
		}
		if len(payload.UserIDs) == 0 {
			if uSingle, err := strconv.Atoi(r.FormValue("user_id")); err == nil && uSingle > 0 {
				payload.UserIDs = append(payload.UserIDs, uSingle)
			}
		}
	}

	isAJAX := strings.Contains(r.Header.Get("Accept"), "application/json") ||
		isJSON ||
		r.Header.Get("X-Requested-With") == "XMLHttpRequest"

	// Process manual member input if provided
	manualInput := strings.TrimSpace(payload.ManualMember)
	if manualInput == "" {
		manualInput = strings.TrimSpace(payload.Username)
	}
	if manualInput != "" {
		cleanInput := strings.TrimPrefix(manualInput, "@")
		cleanInput = strings.TrimSpace(cleanInput)
		targetUser, uErr := repository.GetUserByUsername(cleanInput)
		if uErr != nil || targetUser == nil || targetUser.ID <= 0 {
			targetUser, uErr = repository.GetUserByEmail(cleanInput)
		}

		if uErr != nil || targetUser == nil || targetUser.ID <= 0 {
			if len(payload.UserIDs) == 0 {
				if isAJAX {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]any{
						"success": false,
						"error":   fmt.Sprintf("User '%s' was not found. Please check the username or email.", manualInput),
						"message": fmt.Sprintf("User '%s' was not found. Please check the username or email.", manualInput),
					})
					return
				}
				http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", payload.GroupID), http.StatusSeeOther)
				return
			}
		} else {
			// Check if already a member
			existingMember, mErr := repository.GetConversationMember(payload.GroupID, targetUser.ID)
			if mErr == nil && existingMember != nil {
				if len(payload.UserIDs) == 0 {
					if isAJAX {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusBadRequest)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"success": false,
							"error":   fmt.Sprintf("%s (@%s) is already a member of this group.", targetUser.FullName, targetUser.Username),
							"message": fmt.Sprintf("%s (@%s) is already a member of this group.", targetUser.FullName, targetUser.Username),
						})
						return
					}
					http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", payload.GroupID), http.StatusSeeOther)
					return
				}
			} else {
				// Append to UserIDs avoiding duplicates
				alreadyInList := false
				for _, id := range payload.UserIDs {
					if id == targetUser.ID {
						alreadyInList = true
						break
					}
				}
				if !alreadyInList {
					payload.UserIDs = append(payload.UserIDs, targetUser.ID)
				}
			}
		}
	}

	if payload.GroupID <= 0 || len(payload.UserIDs) == 0 {
		if isAJAX {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   "Invalid parameters",
				"message": "Invalid parameters",
			})
			return
		}
		targetID := payload.GroupID
		if targetID <= 0 {
			targetID, _ = strconv.Atoi(r.FormValue("group_id"))
		}
		if targetID <= 0 {
			targetID, _ = strconv.Atoi(r.FormValue("conversation_id"))
		}
		if targetID > 0 {
			http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", targetID), http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/messages", http.StatusSeeOther)
		}
		return
	}

	if err := services.AddGroupMembers(currentUserID, payload.GroupID, payload.UserIDs); err != nil {
		statusCode := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "access denied") {
			statusCode = http.StatusForbidden
		}
		if isAJAX {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   err.Error(),
				"message": err.Error(),
			})
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", payload.GroupID), http.StatusSeeOther)
		return
	}

	if isAJAX {
		var addedUsersList []map[string]any
		for _, uid := range payload.UserIDs {
			if u, err := repository.GetUserByID(uid); err == nil {
				addedUsersList = append(addedUsersList, map[string]any{
					"id":            u.ID,
					"full_name":     u.FullName,
					"username":      u.Username,
					"initials":      u.Initials,
					"profile_photo": u.ProfilePhoto,
				})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":     true,
			"message":     "Members added successfully",
			"added_users": addedUsersList,
		})
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", payload.GroupID), http.StatusSeeOther)
}

// RemoveGroupMemberAPI removes a user from a group.
func RemoveGroupMemberAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var groupID, targetUserID int
	isJSON := strings.Contains(r.Header.Get("Content-Type"), "application/json")
	if isJSON {
		var payload struct {
			GroupID int `json:"group_id"`
			UserID  int `json:"user_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		groupID = payload.GroupID
		targetUserID = payload.UserID
	} else {
		_ = r.ParseForm()
		groupID, _ = strconv.Atoi(r.FormValue("group_id"))
		if groupID == 0 {
			groupID, _ = strconv.Atoi(r.FormValue("conversation_id"))
		}
		targetUserID, _ = strconv.Atoi(r.FormValue("user_id"))
	}

	isAJAX := strings.Contains(r.Header.Get("Accept"), "application/json") ||
		isJSON ||
		r.Header.Get("X-Requested-With") == "XMLHttpRequest"

	if groupID <= 0 || targetUserID <= 0 {
		if isAJAX {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   "Invalid parameters",
				"message": "Invalid parameters",
			})
			return
		}
		if groupID > 0 {
			http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", groupID), http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/messages", http.StatusSeeOther)
		}
		return
	}

	if err := services.RemoveGroupMember(currentUserID, groupID, targetUserID); err != nil {
		statusCode := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "access denied") {
			statusCode = http.StatusForbidden
		}
		if isAJAX {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   err.Error(),
				"message": err.Error(),
			})
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", groupID), http.StatusSeeOther)
		return
	}

	if isAJAX {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"message": "Member removed successfully",
		})
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", groupID), http.StatusSeeOther)
}

// UpdateGroupMemberRoleAPI updates a member's role in a group.
func UpdateGroupMemberRoleAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var groupID, targetUserID int
	var role string
	isJSON := strings.Contains(r.Header.Get("Content-Type"), "application/json")
	if isJSON {
		var payload struct {
			GroupID int    `json:"group_id"`
			UserID  int    `json:"user_id"`
			Role    string `json:"role"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		groupID = payload.GroupID
		targetUserID = payload.UserID
		role = payload.Role
	} else {
		_ = r.ParseForm()
		groupID, _ = strconv.Atoi(r.FormValue("group_id"))
		if groupID == 0 {
			groupID, _ = strconv.Atoi(r.FormValue("conversation_id"))
		}
		targetUserID, _ = strconv.Atoi(r.FormValue("user_id"))
		role = r.FormValue("role")
	}

	isAJAX := strings.Contains(r.Header.Get("Accept"), "application/json") ||
		isJSON ||
		r.Header.Get("X-Requested-With") == "XMLHttpRequest"

	if groupID <= 0 || targetUserID <= 0 || role == "" {
		if isAJAX {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   "Invalid parameters",
				"message": "Invalid parameters",
			})
			return
		}
		if groupID > 0 {
			http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", groupID), http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/messages", http.StatusSeeOther)
		}
		return
	}

	if err := services.UpdateGroupMemberRole(currentUserID, groupID, targetUserID, role); err != nil {
		statusCode := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "access denied") {
			statusCode = http.StatusForbidden
		}
		if isAJAX {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   err.Error(),
				"message": err.Error(),
			})
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", groupID), http.StatusSeeOther)
		return
	}

	if isAJAX {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"message": "Member role updated successfully",
		})
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", groupID), http.StatusSeeOther)
}

// UpdateGroupSettingsAPI updates group title, category, description, and call permissions.
func UpdateGroupSettingsAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var groupID int
	var title, category, description, callPermission, joinPolicy string
	isJSON := strings.Contains(r.Header.Get("Content-Type"), "application/json")
	if isJSON {
		var payload struct {
			GroupID        int    `json:"group_id"`
			Title          string `json:"title"`
			Category       string `json:"category"`
			Description    string `json:"description"`
			CallPermission string `json:"call_permission"`
			JoinPolicy     string `json:"join_policy"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		groupID = payload.GroupID
		title = payload.Title
		category = payload.Category
		description = payload.Description
		callPermission = payload.CallPermission
		joinPolicy = payload.JoinPolicy
	} else {
		_ = r.ParseForm()
		groupID, _ = strconv.Atoi(r.FormValue("group_id"))
		if groupID == 0 {
			groupID, _ = strconv.Atoi(r.FormValue("conversation_id"))
		}
		title = r.FormValue("title")
		category = r.FormValue("category")
		description = r.FormValue("description")
		callPermission = r.FormValue("call_permission")
		joinPolicy = r.FormValue("join_policy")
	}

	isAJAX := strings.Contains(r.Header.Get("Accept"), "application/json") ||
		isJSON ||
		r.Header.Get("X-Requested-With") == "XMLHttpRequest"

	if groupID <= 0 || strings.TrimSpace(title) == "" {
		if isAJAX {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   "Title cannot be empty",
				"message": "Title cannot be empty",
			})
			return
		}
		if groupID > 0 {
			http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", groupID), http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/messages", http.StatusSeeOther)
		}
		return
	}

	if err := services.UpdateGroupSettings(currentUserID, groupID, title, category, description, callPermission, joinPolicy); err != nil {
		statusCode := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "access denied") {
			statusCode = http.StatusForbidden
		}
		if isAJAX {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   err.Error(),
				"message": err.Error(),
			})
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", groupID), http.StatusSeeOther)
		return
	}

	if isAJAX {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":         true,
			"message":         "Group settings updated successfully",
			"group_id":        groupID,
			"title":           title,
			"category":        category,
			"description":     description,
			"call_permission": callPermission,
		})
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", groupID), http.StatusSeeOther)
}

// DiscoverGroupsAPI returns discoverable public community groups as JSON.
func DiscoverGroupsAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	category := r.URL.Query().Get("category")
	groups, err := services.GetDiscoverableGroups(currentUserID, category)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"groups":  groups,
		"count":   len(groups),
	})
}

// StartGroupCallAPI starts a new group audio or video call session.
func StartGroupCallAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		ConversationID int    `json:"conversation_id"`
		CallType       string `json:"call_type"`
	}

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	} else {
		_ = r.ParseForm()
		payload.ConversationID, _ = strconv.Atoi(r.FormValue("conversation_id"))
		payload.CallType = r.FormValue("call_type")
	}

	if payload.ConversationID <= 0 {
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	room, err := services.GlobalGroupCallManager.StartGroupCall(payload.ConversationID, currentUserID, payload.CallType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"room":    room,
	})
}

// JoinGroupCallAPI enrolls an authenticated user into an active group call.
func JoinGroupCallAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		ConversationID int `json:"conversation_id"`
	}

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	} else {
		_ = r.ParseForm()
		payload.ConversationID, _ = strconv.Atoi(r.FormValue("conversation_id"))
	}

	if payload.ConversationID <= 0 {
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	room, participant, err := services.GlobalGroupCallManager.JoinGroupCall(payload.ConversationID, currentUserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":     true,
		"room":        room,
		"participant": participant,
	})
}

// LeaveGroupCallAPI removes the current user from an active group call session.
func LeaveGroupCallAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		ConversationID int `json:"conversation_id"`
	}

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	} else {
		_ = r.ParseForm()
		payload.ConversationID, _ = strconv.Atoi(r.FormValue("conversation_id"))
	}

	if payload.ConversationID <= 0 {
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	_, duration, err := services.GlobalGroupCallManager.LeaveGroupCall(payload.ConversationID, currentUserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":  true,
		"duration": duration,
	})
}

// GetActiveGroupCallAPI returns active room details for a conversation if live.
func GetActiveGroupCallAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	convID, err := strconv.Atoi(r.URL.Query().Get("conversation_id"))
	if err != nil || convID <= 0 {
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	room, exists := services.GlobalGroupCallManager.GetActiveGroupCall(convID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"active": exists,
		"room":   room,
	})
}

// KickGroupCallParticipantAPI removes a member from a live group call (admin privilege required).
func KickGroupCallParticipantAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		ConversationID int `json:"conversation_id"`
		UserID         int `json:"user_id"`
	}

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	} else {
		_ = r.ParseForm()
		payload.ConversationID, _ = strconv.Atoi(r.FormValue("conversation_id"))
		payload.UserID, _ = strconv.Atoi(r.FormValue("user_id"))
	}

	if payload.ConversationID <= 0 || payload.UserID <= 0 {
		http.Error(w, "Invalid parameters", http.StatusBadRequest)
		return
	}

	if err := services.GlobalGroupCallManager.KickParticipantFromGroupCall(payload.ConversationID, currentUserID, payload.UserID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Participant removed from call",
	})
}

// EndGroupCallAPI terminates a live group call session for all participants (host or admin privilege required).
func EndGroupCallAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		ConversationID int `json:"conversation_id"`
	}

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	} else {
		_ = r.ParseForm()
		payload.ConversationID, _ = strconv.Atoi(r.FormValue("conversation_id"))
	}

	if payload.ConversationID <= 0 {
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	_, err := services.GlobalGroupCallManager.EndGroupCall(payload.ConversationID, currentUserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Group call ended",
	})
}

// SubmitGroupMembershipRequestAPI handles submitting a membership request to an approval-required group.
func SubmitGroupMembershipRequestAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var groupID int
	var message string
	isJSON := strings.Contains(r.Header.Get("Content-Type"), "application/json")
	if isJSON {
		var payload struct {
			GroupID        int    `json:"group_id"`
			ConversationID int    `json:"conversation_id"`
			Message        string `json:"message"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		groupID = payload.GroupID
		if groupID == 0 {
			groupID = payload.ConversationID
		}
		message = payload.Message
	} else {
		_ = r.ParseForm()
		groupID, _ = strconv.Atoi(r.FormValue("group_id"))
		if groupID == 0 {
			groupID, _ = strconv.Atoi(r.FormValue("conversation_id"))
		}
		message = r.FormValue("message")
	}

	if groupID <= 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Invalid group ID"})
		return
	}

	req, err := services.RequestGroupMembership(currentUserID, groupID, message)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error(), "message": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Your membership request has been submitted for administrator approval.",
		"request": req,
	})
}

// ReviewGroupMembershipRequestAPI handles approving or rejecting a membership request by group admin.
func ReviewGroupMembershipRequestAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestID int
	var action string
	isJSON := strings.Contains(r.Header.Get("Content-Type"), "application/json")
	if isJSON {
		var payload struct {
			RequestID int    `json:"request_id"`
			Action    string `json:"action"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		requestID = payload.RequestID
		action = payload.Action
	} else {
		_ = r.ParseForm()
		requestID, _ = strconv.Atoi(r.FormValue("request_id"))
		action = r.FormValue("action")
	}

	if requestID <= 0 || (action != "approve" && action != "reject") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Invalid request ID or action"})
		return
	}

	if err := services.ReviewGroupMembershipRequest(currentUserID, requestID, action); err != nil {
		w.Header().Set("Content-Type", "application/json")
		statusCode := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "access denied") {
			statusCode = http.StatusForbidden
		}
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error(), "message": err.Error()})
		return
	}

	msgText := "Membership request approved successfully."
	if action == "reject" {
		msgText = "Membership request declined."
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"action":  action,
		"message": msgText,
	})
}

// CancelGroupMembershipRequestAPI handles cancelling a pending membership request by the requester.
func CancelGroupMembershipRequestAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var groupID int
	isJSON := strings.Contains(r.Header.Get("Content-Type"), "application/json")
	if isJSON {
		var payload struct {
			GroupID        int `json:"group_id"`
			ConversationID int `json:"conversation_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		groupID = payload.GroupID
		if groupID == 0 {
			groupID = payload.ConversationID
		}
	} else {
		_ = r.ParseForm()
		groupID, _ = strconv.Atoi(r.FormValue("group_id"))
		if groupID == 0 {
			groupID, _ = strconv.Atoi(r.FormValue("conversation_id"))
		}
	}

	if groupID <= 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Invalid group ID"})
		return
	}

	if err := services.CancelGroupMembershipRequest(currentUserID, groupID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error(), "message": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Membership request cancelled.",
	})
}

// GetGroupMembershipRequestsAPI returns pending or all requests for a group (group admin only).
func GetGroupMembershipRequestsAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	groupID, err := strconv.Atoi(r.URL.Query().Get("conversation_id"))
	if err != nil || groupID <= 0 {
		groupID, err = strconv.Atoi(r.URL.Query().Get("group_id"))
	}
	if err != nil || groupID <= 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Invalid group ID"})
		return
	}

	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}

	requests, err := services.GetGroupMembershipRequests(currentUserID, groupID, status)
	if err != nil {
		statusCode := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "access denied") {
			statusCode = http.StatusForbidden
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":  true,
		"requests": requests,
		"count":    len(requests),
	})
}

// DeleteMessageAPI handles soft-deleting a message by its author or group moderator/admin.
func DeleteMessageAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var messageID int
	isJSON := strings.Contains(r.Header.Get("Content-Type"), "application/json")
	if isJSON {
		var payload struct {
			MessageID int `json:"message_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		messageID = payload.MessageID
	} else {
		_ = r.ParseForm()
		messageID, _ = strconv.Atoi(r.FormValue("message_id"))
	}

	if messageID <= 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Invalid message ID"})
		return
	}

	if err := services.ModerateDeleteMessage(currentUserID, messageID); err != nil {
		statusCode := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "access denied") {
			statusCode = http.StatusForbidden
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error(), "message": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":    true,
		"message_id": messageID,
		"message":    "Message has been deleted successfully",
	})
}
