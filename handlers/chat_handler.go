package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

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

	var isActivePinned bool
	if activeConvID > 0 {
		conv, recipient, err := services.GetConversationDetails(activeConvID, currentUserID)
		if err == nil {
			activeConv = conv
			activeRecipient = recipient
			isActiveMuted, isActiveArchived, isActivePinned, _ = services.GetConversationMemberSettings(activeConvID, currentUserID)
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
	for i := range eligibleFriends {
		eligibleFriends[i].IsOnline = services.GlobalHub.IsUserOnline(eligibleFriends[i].ID)
	}
	for i := range conversations {
		if conversations[i].Type == "direct" && conversations[i].OtherUser.ID > 0 {
			conversations[i].OtherUser.IsOnline = services.GlobalHub.IsUserOnline(conversations[i].OtherUser.ID)
		}
	}

	// Notification badge count for navbar
	unreadNotifications, _ := services.CountUnreadNotifications(currentUserID)

	var isActiveOnline bool
	var isRecipientBlocked, isBlockedByRecipient bool
	var sharedMedia []models.Message
	var sharedMediaOverview *models.SharedMediaOverview
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

	var activeMutedUntil *time.Time
	var activeDisappearingDuration int
	var activeDisappearingLabel string

	if activeConv != nil {
		activeDisappearingDuration = activeConv.DisappearingDuration
		activeDisappearingLabel = activeConv.FormattedDisappearingDuration()
		if activeMember, mErr := repository.GetConversationMember(activeConv.ID, currentUserID); mErr == nil && activeMember != nil {
			activeMutedUntil = activeMember.MutedUntil
			isActiveMuted = activeMember.IsActiveMuted()
			isActivePinned = activeMember.IsPinned
		}

		sharedMedia, _ = services.GetSharedMedia(activeConv.ID, currentUserID)
		sharedMediaOverview, _ = services.GetSharedMediaOverview(activeConv.ID, currentUserID)
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
	blockedUsers, _ := services.GetBlockedUsers(currentUserID)
	var pendingReportsCount int
	if currentUser.Role == "admin" || currentUser.Role == "moderator" {
		pendingReportsCount, _ = repository.CountPendingChatReports()
	}

	hasAcceptedTerms, activeTermsVer, _, _ := services.CheckTermsAcceptance(currentUserID)

	pendingFriendsCount := 0
	if reqs, err := services.GetPendingFriendRequests(currentUserID); err == nil {
		pendingFriendsCount = len(reqs)
	}

	data := models.ChatViewData{
		CurrentUser:                currentUser,
		Error:                      inboxError,
		Conversations:              conversations,
		ConversationGroups:         models.GroupConversationsChronologically(conversations),
		ActiveConversation:         activeConv,
		ActiveRecipient:            activeRecipient,
		ActiveGroupMembers:         activeGroupMembers,
		ActiveUserRole:             activeUserRole,
		CanStartCall:               canStartCall,
		CanManageMembers:           canManageMembers,
		PendingRequestsCount:       pendingRequestsCount,
		PendingRequests:            pendingRequests,
		DiscoverableGroups:         discoverableGroups,
		Messages:                   messages,
		EligibleFriends:            eligibleFriends,
		UnreadCount:                unreadNotifications,
		UnreadMessagesCount:        unreadMsgs,
		PendingFriendRequestsCount: pendingFriendsCount,
		IsActiveMuted:              isActiveMuted,
		ActiveMutedUntil:           activeMutedUntil,
		ActiveDisappearingDuration: activeDisappearingDuration,
		ActiveDisappearingLabel:    activeDisappearingLabel,
		IsActiveArchived:           isActiveArchived,
		IsActivePinned:             isActivePinned,
		HasOlderMessages:           hasOlder,
		IsActiveOnline:             isActiveOnline,
		IsRecipientBlocked:         isRecipientBlocked,
		IsBlockedByRecipient:       isBlockedByRecipient,
		BlockedUsers:               blockedUsers,
		PendingReportsCount:        pendingReportsCount,
		HasAcceptedTerms:           hasAcceptedTerms,
		ActiveTermsVersion:         activeTermsVer,
		SharedMedia:                sharedMedia,
		SharedMediaOverview:        sharedMediaOverview,
		CallLogs:                   callLogs,
		ActiveGroupCall:            activeGroupCall,
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

	termsAccepted, reqVersion, _, _ := services.CheckTermsAcceptance(currentUserID)
	if !termsAccepted {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":            "terms_acceptance_required",
			"message":          "You must review and accept the latest platform terms of use before sending messages.",
			"required_version": reqVersion,
		})
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
		_ = r.ParseMultipartForm(32 << 20) // 32 MB buffer for voice notes and attachments
		convID, _ = strconv.Atoi(r.FormValue("conversation_id"))
		body = r.FormValue("body")
		if body == "" {
			body = r.FormValue("content")
		}
		clientMsgID = r.FormValue("client_message_id")
		if r.FormValue("reply_to_id") != "" {
			if id, err := strconv.Atoi(r.FormValue("reply_to_id")); err == nil {
				replyToID = &id
			}
		} else if r.FormValue("reply_to_message_id") != "" {
			if id, err := strconv.Atoi(r.FormValue("reply_to_message_id")); err == nil {
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
			ConversationID   int    `json:"conversation_id"`
			Body             string `json:"body"`
			Content          string `json:"content"`
			ClientMessageID  string `json:"client_message_id"`
			ReplyToID        *int   `json:"reply_to_id"`
			ReplyToMessageID *int   `json:"reply_to_message_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}
		convID = req.ConversationID
		body = req.Body
		if body == "" {
			body = req.Content
		}
		clientMsgID = req.ClientMessageID
		if req.ReplyToID != nil {
			replyToID = req.ReplyToID
		} else if req.ReplyToMessageID != nil {
			replyToID = req.ReplyToMessageID
		}
	} else {
		_ = r.ParseForm()
		convID, _ = strconv.Atoi(r.FormValue("conversation_id"))
		body = r.FormValue("body")
		if body == "" {
			body = r.FormValue("content")
		}
		clientMsgID = r.FormValue("client_message_id")
		if r.FormValue("reply_to_id") != "" {
			if id, err := strconv.Atoi(r.FormValue("reply_to_id")); err == nil {
				replyToID = &id
			}
		} else if r.FormValue("reply_to_message_id") != "" {
			if id, err := strconv.Atoi(r.FormValue("reply_to_message_id")); err == nil {
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

	// Determine if AJAX/JSON response is expected
	isAJAX := strings.Contains(r.Header.Get("Accept"), "application/json") ||
		strings.Contains(contentType, "application/json") ||
		r.Header.Get("X-Requested-With") == "XMLHttpRequest" ||
		r.URL.Path == "/messages/upload"

	if err != nil {
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
		http.Error(w, err.Error(), statusCode)
		return
	}

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

// ToggleMuteAPI toggles mute or sets a specific mute duration for a conversation.
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

	duration := r.FormValue("duration")
	if duration == "" {
		duration = r.URL.Query().Get("duration")
	}

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var req struct {
			ConversationID int    `json:"conversation_id"`
			Duration       string `json:"duration"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			if req.ConversationID > 0 {
				convID = req.ConversationID
			}
			if req.Duration != "" {
				duration = req.Duration
			}
		}
	}

	if convID <= 0 {
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	var isMuted bool
	var mutedUntil *time.Time
	if duration != "" {
		mutedUntil, isMuted, err = services.SetConversationMute(convID, currentUserID, duration)
	} else {
		isMuted, err = services.ToggleMuteConversation(convID, currentUserID)
	}

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
			"is_muted":    isMuted,
			"muted_until": mutedUntil,
			"duration":    duration,
		})
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", convID), http.StatusSeeOther)
}

// SetDisappearingMessagesAPI updates the disappearing message timer for a conversation.
func SetDisappearingMessagesAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var convID int
	var durationStr string
	var durationSec int

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var req struct {
			ConversationID int `json:"conversation_id"`
			Duration       any `json:"duration"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			convID = req.ConversationID
			switch v := req.Duration.(type) {
			case float64:
				durationSec = int(v)
			case int:
				durationSec = v
			case string:
				durationStr = v
			}
		}
	} else {
		convID, _ = strconv.Atoi(r.FormValue("conversation_id"))
		if convID <= 0 {
			convID, _ = strconv.Atoi(r.URL.Query().Get("conversation_id"))
		}
		durationStr = r.FormValue("duration")
	}

	if durationStr != "" {
		switch strings.ToLower(strings.TrimSpace(durationStr)) {
		case "off", "none", "0":
			durationSec = 0
		case "24h", "24_hours", "24 hours", "1d", "86400":
			durationSec = 86400
		case "7d", "7_days", "7 days", "1w", "604800":
			durationSec = 604800
		case "90d", "90_days", "90 days", "3m", "7776000":
			durationSec = 7776000
		default:
			if sec, err := strconv.Atoi(durationStr); err == nil {
				durationSec = sec
			}
		}
	}

	if convID <= 0 {
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	conv, err := services.SetDisappearingMessages(currentUserID, convID, durationSec)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "permission denied") || strings.Contains(err.Error(), "access denied") {
			status = http.StatusForbidden
		} else if strings.Contains(err.Error(), "invalid disappearing duration") {
			status = http.StatusBadRequest
		}
		if strings.Contains(r.Header.Get("Accept"), "application/json") || strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		http.Error(w, err.Error(), status)
		return
	}

	if strings.Contains(r.Header.Get("Accept"), "application/json") || strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":               true,
			"conversation_id":       convID,
			"disappearing_duration": conv.DisappearingDuration,
			"formatted_duration":    conv.FormattedDisappearingDuration(),
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
		if strings.Contains(err.Error(), "already pending") {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
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

// GetBlockedUsersAPI returns the list of users blocked by the authenticated member.
func GetBlockedUsersAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	blockedUsers, err := services.GetBlockedUsers(currentUserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":       true,
		"blocked_users": blockedUsers,
	})
}

// GetModerationReportsAPI returns pending/all abuse reports for administrative/moderation review.
func GetModerationReportsAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	status := r.URL.Query().Get("status")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50
	offset := 0
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	reports, total, err := services.GetChatReportsForModeration(currentUserID, status, limit, offset)
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
		"success": true,
		"reports": reports,
		"total":   total,
	})
}

// ResolveModerationReportAPI handles moderation review and decision taking on a reported message or member.
func ResolveModerationReportAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var reportID int
	var status string
	var action string
	var notes string

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var payload struct {
			ReportID int    `json:"report_id"`
			Status   string `json:"status"`
			Action   string `json:"action"`
			Notes    string `json:"notes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
			reportID = payload.ReportID
			status = payload.Status
			action = payload.Action
			notes = payload.Notes
		}
	} else {
		_ = r.ParseForm()
		reportID, _ = strconv.Atoi(r.FormValue("report_id"))
		status = r.FormValue("status")
		action = r.FormValue("action")
		notes = r.FormValue("notes")
	}

	if reportID <= 0 {
		http.Error(w, "Invalid report ID", http.StatusBadRequest)
		return
	}

	if err := services.ResolveChatReport(currentUserID, reportID, status, action, notes); err != nil {
		if strings.Contains(err.Error(), "access denied") {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Report resolved successfully",
	})
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
	cStr := r.URL.Query().Get("conversation_id")
	if cStr == "" {
		cStr = r.URL.Query().Get("c")
	}
	if cStr != "" {
		id, err := strconv.Atoi(cStr)
		if err != nil || id <= 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Invalid conversation ID"})
			return
		}
		convID = &id
	}

	limit := 30
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 && l <= 100 {
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
		if strings.Contains(strings.ToLower(err.Error()), "access denied") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":         true,
		"query":           query,
		"conversation_id": convID,
		"messages":        messages,
		"count":           len(messages),
		"limit":           limit,
		"offset":          offset,
	})
}

// GetSharedMediaAPI returns paginated shared media attachments for a conversation as JSON.
func GetSharedMediaAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	cStr := r.URL.Query().Get("conversation_id")
	if cStr == "" {
		cStr = r.URL.Query().Get("c")
	}
	convID, err := strconv.Atoi(cStr)
	if err != nil || convID <= 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Invalid conversation ID"})
		return
	}

	category := r.URL.Query().Get("category")

	limit := 50
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := 0
	if oStr := r.URL.Query().Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			offset = o
		}
	}

	media, err := services.GetSharedMediaCategorized(convID, currentUserID, category, limit, offset)
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
		"success":         true,
		"conversation_id": convID,
		"category":        category,
		"media":           media,
		"count":           len(media),
		"limit":           limit,
		"offset":          offset,
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

// CurrentUserIDAPI returns the authenticated user's ID as JSON.
func CurrentUserIDAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"user_id": currentUserID,
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

// GenerateCallLinkAPI creates a new invitation token for a call session.
func GenerateCallLinkAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var convID int
	callType := "audio"

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var req struct {
			ConversationID int    `json:"conversation_id"`
			CallType       string `json:"call_type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			convID = req.ConversationID
			if req.CallType != "" {
				callType = req.CallType
			}
		}
	} else {
		_ = r.ParseForm()
		convID, _ = strconv.Atoi(r.FormValue("conversation_id"))
		if r.FormValue("call_type") != "" {
			callType = r.FormValue("call_type")
		}
	}

	if convID <= 0 {
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	inv, inviteURL, err := services.GenerateCallInvitation(convID, currentUserID, callType)
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

	proto := "https"
	if r.TLS == nil && !strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		proto = "http"
	}
	host := r.Host
	if host == "" {
		host = "convention-management-system.onrender.com"
	}
	fullURL := fmt.Sprintf("%s://%s%s", proto, host, inviteURL)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":    true,
		"token":      inv.Token,
		"call_type":  inv.CallType,
		"invite_url": fullURL,
		"path":       inviteURL,
		"expires_at": inv.ExpiresAt.Format(time.RFC3339),
	})
}

// JoinCallLinkHandler verifies an invitation token and admits the user to the active call session.
func JoinCallLinkHandler(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Redirect(w, r, fmt.Sprintf("/user/login?redirect=%s", url.QueryEscape(r.RequestURI)), http.StatusSeeOther)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing call invitation token", http.StatusBadRequest)
		return
	}

	inv, conv, err := services.ValidateCallInvitation(token, currentUserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	redirectURL := fmt.Sprintf("/messages?c=%d&autocall=%s", conv.ID, inv.CallType)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
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

// ConvertDirectToGroupAPI handles adding members from an individual chat to transition it into a group chat.
func ConvertDirectToGroupAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	isAJAX := strings.Contains(r.Header.Get("Accept"), "application/json") ||
		strings.Contains(r.Header.Get("Content-Type"), "application/json") ||
		r.Header.Get("X-Requested-With") == "XMLHttpRequest"

	if currentUserID == 0 {
		if isAJAX {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Unauthorized"})
			return
		}
		http.Redirect(w, r, "/community/login", http.StatusSeeOther)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		ConversationID int    `json:"conversation_id"`
		GroupID        int    `json:"group_id"`
		GroupTitle     string `json:"group_title"`
		Title          string `json:"title"`
		UserIDs        []int  `json:"user_ids"`
		ManualMember   string `json:"manual_member"`
		Username       string `json:"username"`
	}

	isJSON := strings.Contains(r.Header.Get("Content-Type"), "application/json")
	if isJSON {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	} else {
		_ = r.ParseForm()
		payload.ConversationID, _ = strconv.Atoi(r.FormValue("conversation_id"))
		if payload.ConversationID == 0 {
			payload.ConversationID, _ = strconv.Atoi(r.FormValue("c"))
		}
		if payload.ConversationID == 0 {
			payload.ConversationID, _ = strconv.Atoi(r.FormValue("group_id"))
		}
		payload.GroupTitle = r.FormValue("group_title")
		if payload.GroupTitle == "" {
			payload.GroupTitle = r.FormValue("title")
		}
		payload.ManualMember = r.FormValue("manual_member")
		if payload.ManualMember == "" {
			payload.ManualMember = r.FormValue("username")
		}

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

	convID := payload.ConversationID
	if convID == 0 {
		convID = payload.GroupID
	}
	title := payload.GroupTitle
	if title == "" {
		title = payload.Title
	}

	// Resolve manual username / email input if specified
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
				errMsg := fmt.Sprintf("User '%s' was not found. Please verify the username or email.", manualInput)
				if isAJAX {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": errMsg, "message": errMsg})
					return
				}
				http.Redirect(w, r, fmt.Sprintf("/messages?c=%d&error=%s", convID, url.QueryEscape(errMsg)), http.StatusSeeOther)
				return
			}
		} else {
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

	if convID <= 0 || len(payload.UserIDs) == 0 {
		errMsg := "Please select at least one registered member to add."
		if isAJAX {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": errMsg, "message": errMsg})
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", convID), http.StatusSeeOther)
		return
	}

	updatedConv, err := services.ConvertDirectToGroupConversation(convID, currentUserID, payload.UserIDs, title)
	if err != nil {
		statusCode := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "access denied") {
			statusCode = http.StatusForbidden
		}
		if isAJAX {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error(), "message": err.Error()})
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/messages?c=%d&error=%s", convID, url.QueryEscape(err.Error())), http.StatusSeeOther)
		return
	}

	if isAJAX {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":         true,
			"message":         "Group created successfully",
			"conversation_id": updatedConv.ID,
			"redirect_url":    fmt.Sprintf("/messages?c=%d", updatedConv.ID),
		})
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/messages?c=%d", updatedConv.ID), http.StatusSeeOther)
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

// GetUserActiveGroupCallsAPI returns all live group calls for groups the current user is a member of.
func GetUserActiveGroupCallsAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	rooms := services.GlobalGroupCallManager.GetActiveGroupCallsForUser(currentUserID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"rooms":   rooms,
	})
}

// MuteGroupCallParticipantAPI remotely mutes a member in a live group call (host, admin, or moderator privilege required).
func MuteGroupCallParticipantAPI(w http.ResponseWriter, r *http.Request) {
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

	if err := services.GlobalGroupCallManager.MuteParticipantInGroupCall(payload.ConversationID, currentUserID, payload.UserID); err != nil {
		statusCode := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "access denied") {
			statusCode = http.StatusForbidden
		}
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Participant muted successfully",
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
	var mode string
	isJSON := strings.Contains(r.Header.Get("Content-Type"), "application/json")
	if isJSON {
		var payload struct {
			MessageID int    `json:"message_id"`
			Mode      string `json:"mode"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		messageID = payload.MessageID
		mode = payload.Mode
	} else {
		_ = r.ParseForm()
		messageID, _ = strconv.Atoi(r.FormValue("message_id"))
		mode = r.FormValue("mode")
	}

	if messageID <= 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Invalid message ID"})
		return
	}

	if mode == "for_me" {
		if err := services.DeleteMessageForMe(currentUserID, messageID); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":    true,
			"message_id": messageID,
			"mode":       "for_me",
			"message":    "Message deleted for you",
		})
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
		"mode":       "for_everyone",
		"message":    "Message has been deleted successfully",
	})
}

// ForwardMessagesAPI handles forwarding selected messages to specified conversations or user contacts.
func ForwardMessagesAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Unauthorized"})
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	termsAccepted, reqVersion, _, _ := services.CheckTermsAcceptance(currentUserID)
	if !termsAccepted {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":          false,
			"error":            "terms_acceptance_required",
			"message":          "You must review and accept the latest platform terms of use before forwarding messages.",
			"required_version": reqVersion,
		})
		return
	}

	var req struct {
		MessageIDs      []int `json:"message_ids"`
		ConversationIDs []int `json:"conversation_ids"`
		UserIDs         []int `json:"user_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Invalid request body"})
		return
	}

	forwarded, err := services.ForwardMessages(currentUserID, req.MessageIDs, req.ConversationIDs, req.UserIDs)
	if err != nil {
		statusCode := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "access denied") || strings.Contains(strings.ToLower(err.Error()), "unauthorized") {
			statusCode = http.StatusForbidden
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error(), "message": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":         true,
		"forwarded_count": len(forwarded),
		"messages":        forwarded,
	})
}

// ClearChatAPI clears message history for the current user in a conversation.
func ClearChatAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Unauthorized"})
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var conversationID int
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var payload struct {
			ConversationID int `json:"conversation_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		conversationID = payload.ConversationID
	} else {
		_ = r.ParseForm()
		conversationID, _ = strconv.Atoi(r.FormValue("conversation_id"))
	}

	if conversationID <= 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Invalid conversation ID"})
		return
	}

	if err := services.ClearChat(conversationID, currentUserID); err != nil {
		status := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "not a member") || strings.Contains(strings.ToLower(err.Error()), "unauthorized") {
			status = http.StatusForbidden
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":         true,
		"conversation_id": conversationID,
		"message":         "Conversation history cleared successfully",
	})
}

// TogglePinAPI toggles the is_pinned state for the user in a conversation.
func TogglePinAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Unauthorized"})
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var conversationID int
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var payload struct {
			ConversationID int `json:"conversation_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		conversationID = payload.ConversationID
	} else {
		_ = r.ParseForm()
		conversationID, _ = strconv.Atoi(r.FormValue("conversation_id"))
	}

	if conversationID <= 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Invalid conversation ID"})
		return
	}

	newPinned, err := services.TogglePinConversation(conversationID, currentUserID)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "not a member") || strings.Contains(strings.ToLower(err.Error()), "unauthorized") {
			status = http.StatusForbidden
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}

	msgText := "Conversation pinned"
	if !newPinned {
		msgText = "Conversation unpinned"
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":         true,
		"conversation_id": conversationID,
		"is_pinned":       newPinned,
		"message":         msgText,
	})
}

// ExportChatAPI exports conversation history as txt or json transcript for authorized members.
func ExportChatAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Unauthorized"})
		return
	}

	conversationID, _ := strconv.Atoi(r.URL.Query().Get("conversation_id"))
	if conversationID <= 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Invalid conversation ID"})
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "txt"
	}

	filename, data, err := services.ExportChat(conversationID, currentUserID, format)
	if err != nil {
		statusCode := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "unauthorized") || strings.Contains(strings.ToLower(err.Error()), "access denied") {
			statusCode = http.StatusForbidden
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	if strings.HasSuffix(filename, ".json") {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// GetConversationMembersAPI returns member list for @mention autocomplete.
func GetConversationMembersAPI(w http.ResponseWriter, r *http.Request) {
	currentUserID := getCurrentUserID(r)
	if currentUserID == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Unauthorized"})
		return
	}

	conversationID, _ := strconv.Atoi(r.URL.Query().Get("conversation_id"))
	if conversationID <= 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Invalid conversation ID"})
		return
	}

	// Verify current user is a member
	isMember, err := repository.IsConversationMember(conversationID, currentUserID)
	if err != nil || !isMember {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Access denied"})
		return
	}

	members, err := repository.GetConversationMembers(conversationID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}

	type MemberInfo struct {
		ID        int    `json:"id"`
		Username  string `json:"username"`
		FullName  string `json:"full_name"`
		AvatarURL string `json:"avatar_url"`
		Role      string `json:"role"`
	}

	result := make([]MemberInfo, 0, len(members))
	for _, m := range members {
		result = append(result, MemberInfo{
			ID:        m.User.ID,
			Username:  m.User.Username,
			FullName:  m.User.FullName,
			AvatarURL: m.User.ProfilePhoto,
			Role:      m.Role,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"members": result,
	})
}
