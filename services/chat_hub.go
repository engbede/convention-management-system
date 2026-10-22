package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"convention-management-system/models"
	"convention-management-system/repository"

	"golang.org/x/net/websocket"
)

// WSEvent represents the standard envelope for real-time WebSocket messages and WebRTC signaling.
type WSEvent struct {
	Type                 string                        `json:"type"` // "new_message", "typing", "read_receipt", "subscribe", "presence", "error", "call_offer", "call_answer", "ice_candidate", "call_decline", "call_end", "call_busy", "call_unavailable", "group_call_started", "group_call_participant_joined", "group_call_participant_left", "group_call_state_updated", "group_call_signal", "group_call_kicked", "group_call_ended"
	ConversationID       int                           `json:"conversation_id,omitempty"`
	UserID               int                           `json:"user_id,omitempty"`
	Username             string                        `json:"username,omitempty"`
	TargetUserID         int                           `json:"target_user_id,omitempty"`
	IsTyping             bool                          `json:"is_typing,omitempty"`
	IsOnline             bool                          `json:"is_online,omitempty"`
	CallType             string                        `json:"call_type,omitempty"` // "audio" or "video"
	CallID               int                           `json:"call_id,omitempty"`
	CallerName           string                        `json:"caller_name,omitempty"`
	CallerAvatar         string                        `json:"caller_avatar,omitempty"`
	SDP                  string                        `json:"sdp,omitempty"`
	Candidate            any                           `json:"candidate,omitempty"`
	Duration             int                           `json:"duration,omitempty"`
	Message              *models.Message               `json:"message,omitempty"`
	Error                string                        `json:"error,omitempty"`
	Room                 *models.GroupCallRoom         `json:"room,omitempty"`
	Participant          *models.CallParticipantInfo   `json:"participant,omitempty"`
	Participants         []*models.CallParticipantInfo `json:"participants,omitempty"`
	IsMuted              bool                          `json:"is_muted,omitempty"`
	IsCamOff             bool                          `json:"is_cam_off,omitempty"`
	IsSpeaking           bool                          `json:"is_speaking,omitempty"`
	SignalType           string                        `json:"signal_type,omitempty"`
	DisappearingDuration int                           `json:"disappearing_duration,omitempty"`
	FormattedDuration    string                        `json:"formatted_duration,omitempty"`
	Data                 any                           `json:"data,omitempty"`
}

// WSClient represents a single connected WebSocket client session.
type WSClient struct {
	UserID           int
	Username         string
	FullName         string
	Conn             *websocket.Conn
	SubscribedConvID int
	mu               sync.Mutex
}

// Send sends a JSON event to the client safely under a mutex.
func (c *WSClient) Send(event any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Conn == nil {
		return errors.New("nil connection")
	}

	return websocket.JSON.Send(c.Conn, event)
}

// ChatHub manages all active WebSocket connections and conversation routing.
type ChatHub struct {
	// Map of userID -> set of active clients (supports multiple tabs/devices)
	clients map[int]map[*WSClient]bool
	mu      sync.RWMutex
}

// GlobalHub is the singleton hub instance used by handlers and services.
var GlobalHub = NewChatHub()

func NewChatHub() *ChatHub {
	return &ChatHub{
		clients: make(map[int]map[*WSClient]bool),
	}
}

// Register adds a new client to the hub and broadcasts presence if user came online.
func (h *ChatHub) Register(client *WSClient) {
	h.mu.Lock()
	wasOffline := len(h.clients[client.UserID]) == 0
	if h.clients[client.UserID] == nil {
		h.clients[client.UserID] = make(map[*WSClient]bool)
	}
	h.clients[client.UserID][client] = true
	h.mu.Unlock()

	if wasOffline {
		go h.BroadcastAll(WSEvent{
			Type:     "presence",
			UserID:   client.UserID,
			Username: client.Username,
			IsOnline: true,
		})
	}
}

// Unregister safely removes a client from the hub and broadcasts presence if user went offline.
func (h *ChatHub) Unregister(client *WSClient) {
	h.mu.Lock()
	if userClients, ok := h.clients[client.UserID]; ok {
		delete(userClients, client)
		if len(userClients) == 0 {
			delete(h.clients, client.UserID)
		}
	}
	nowOffline := len(h.clients[client.UserID]) == 0
	h.mu.Unlock()

	if nowOffline {
		// Clean up any active group call participation for this user
		GlobalGroupCallManager.mu.RLock()
		var roomsToLeave []int
		for convID, room := range GlobalGroupCallManager.rooms {
			if _, ok := room.Participants[client.UserID]; ok {
				roomsToLeave = append(roomsToLeave, convID)
			}
		}
		GlobalGroupCallManager.mu.RUnlock()

		for _, convID := range roomsToLeave {
			go func(cID int) {
				_, _, _ = GlobalGroupCallManager.LeaveGroupCall(cID, client.UserID)
			}(convID)
		}

		go h.BroadcastAll(WSEvent{
			Type:     "presence",
			UserID:   client.UserID,
			Username: client.Username,
			IsOnline: false,
		})
	}
}

// IsUserOnline checks if a user currently has active WebSocket connections.
func (h *ChatHub) IsUserOnline(userID int) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[userID]) > 0
}

// BroadcastAll sends an event to all connected clients.
func (h *ChatHub) BroadcastAll(event WSEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, userClients := range h.clients {
		for client := range userClients {
			go func(c *WSClient) {
				_ = c.Send(event)
			}(client)
		}
	}
}

// Subscribe authorizes and attaches the client to an active conversation for live updates.
func (h *ChatHub) Subscribe(client *WSClient, conversationID int) error {
	if conversationID <= 0 {
		return errors.New("invalid conversation ID")
	}

	// Verify membership on the database level
	isMember, err := repository.IsConversationMember(conversationID, client.UserID)
	if err != nil {
		return err
	}
	if !isMember {
		return errors.New("access denied: you are not a member of this conversation")
	}

	client.mu.Lock()
	client.SubscribedConvID = conversationID
	client.mu.Unlock()

	// Advance read marker in background
	_ = MarkConversationAsRead(conversationID, client.UserID)

	return nil
}

// BroadcastToConversation broadcasts an event to all participants of a conversation who are connected.
func (h *ChatHub) BroadcastToConversation(conversationID int, event WSEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for userID, userClients := range h.clients {
		// Check membership
		isMember, err := repository.IsConversationMember(conversationID, userID)
		if err == nil && isMember {
			for client := range userClients {
				go func(c *WSClient) {
					_ = c.Send(event)
				}(client)
			}
		}
	}
}

// BroadcastToUser sends an event directly to all active sockets of a specific user.
func (h *ChatHub) BroadcastToUser(userID int, event WSEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if userClients, ok := h.clients[userID]; ok {
		for client := range userClients {
			go func(c *WSClient) {
				_ = c.Send(event)
			}(client)
		}
	}
}

// HandleClientMessage parses incoming WebSocket frames from a client.
func (h *ChatHub) HandleClientMessage(client *WSClient, payload []byte) {
	var event struct {
		Type            string `json:"type"`
		ConversationID  int    `json:"conversation_id"`
		TargetUserID    int    `json:"target_user_id"`
		Body            string `json:"body"`
		ClientMessageID string `json:"client_message_id"`
		IsTyping        bool   `json:"is_typing"`
		CallType        string `json:"call_type"`
		CallID          int    `json:"call_id"`
		SDP             string `json:"sdp"`
		Candidate       any    `json:"candidate"`
		Duration        int    `json:"duration"`
		IsMuted         bool   `json:"is_muted"`
		IsCamOff        bool   `json:"is_cam_off"`
		IsSpeaking      bool   `json:"is_speaking"`
		SignalType      string `json:"signal_type"`
	}

	if err := json.Unmarshal(payload, &event); err != nil {
		_ = client.Send(WSEvent{Type: "error", Error: "invalid JSON format"})
		return
	}

	switch event.Type {
	case "subscribe":
		if err := h.Subscribe(client, event.ConversationID); err != nil {
			_ = client.Send(WSEvent{
				Type:  "error",
				Error: err.Error(),
			})
			return
		}
		_ = client.Send(WSEvent{
			Type:           "subscribed",
			ConversationID: event.ConversationID,
		})

	case "send_message":
		convID := event.ConversationID
		if convID <= 0 {
			convID = client.SubscribedConvID
		}
		if convID <= 0 {
			_ = client.Send(WSEvent{Type: "error", Error: "no conversation selected"})
			return
		}

		// Persist message using the canonical service logic (which automatically broadcasts)
		_, err := SendMessage(convID, client.UserID, event.Body, event.ClientMessageID, nil)
		if err != nil {
			_ = client.Send(WSEvent{Type: "error", Error: err.Error()})
			return
		}

	case "typing":
		convID := event.ConversationID
		if convID <= 0 {
			convID = client.SubscribedConvID
		}
		if convID > 0 {
			isMember, err := repository.IsConversationMember(convID, client.UserID)
			if err == nil && isMember {
				conv, otherUser, cErr := GetConversationDetails(convID, client.UserID)
				if cErr == nil && conv != nil && conv.Type == "direct" && otherUser != nil {
					blocked, bErr := repository.IsCommunicationBlocked(client.UserID, otherUser.ID)
					if bErr == nil && blocked {
						return
					}
				}
				// Broadcast typing state to other members
				h.BroadcastToConversation(convID, WSEvent{
					Type:           "typing",
					ConversationID: convID,
					UserID:         client.UserID,
					Username:       client.Username,
					IsTyping:       event.IsTyping,
				})
			}
		}

	case "read":
		convID := event.ConversationID
		if convID <= 0 {
			convID = client.SubscribedConvID
		}
		if convID > 0 {
			_ = MarkConversationAsRead(convID, client.UserID)
			h.BroadcastToConversation(convID, WSEvent{
				Type:           "read_receipt",
				ConversationID: convID,
				UserID:         client.UserID,
			})
		}

	case "call_offer":
		convID := event.ConversationID
		targetUserID := event.TargetUserID

		if targetUserID <= 0 && convID > 0 {
			conv, otherUser, err := GetConversationDetails(convID, client.UserID)
			if err == nil && conv != nil && otherUser != nil {
				targetUserID = otherUser.ID
			}
		}

		if targetUserID <= 0 {
			_ = client.Send(WSEvent{Type: "error", Error: "recipient not found for call"})
			return
		}

		// Check privacy block
		blocked, bErr := repository.IsCommunicationBlocked(client.UserID, targetUserID)
		if bErr == nil && blocked {
			_ = client.Send(WSEvent{Type: "error", Error: "communication between these members is blocked"})
			return
		}

		callType := event.CallType
		if callType == "" {
			callType = "audio"
		}

		// Check if target user is online or has registered push subscriptions
		isOnline := h.IsUserOnline(targetUserID)
		pushSubs, _ := repository.GetPushSubscriptionsByUserID(targetUserID)
		hasPush := len(pushSubs) > 0

		if !isOnline && !hasPush {
			missedCall := &models.CallLog{
				ConversationID: convID,
				CallerID:       client.UserID,
				ReceiverID:     targetUserID,
				CallType:       callType,
				Status:         "missed",
			}
			_ = repository.CreateCallLog(missedCall)

			_ = client.Send(WSEvent{
				Type:           "call_unavailable",
				ConversationID: convID,
				UserID:         targetUserID,
				Error:          "Member is currently offline",
			})
			return
		}

		expiresAt := time.Now().Add(45 * time.Second)
		callLog := &models.CallLog{
			ConversationID: convID,
			CallerID:       client.UserID,
			ReceiverID:     targetUserID,
			CallType:       callType,
			Status:         "initiated",
			ExpiresAt:      &expiresAt,
		}
		_ = repository.CreateCallLog(callLog)

		callerUser, _ := repository.GetUserByID(client.UserID)
		avatar := ""
		if callerUser.ProfilePhoto != "" {
			avatar = callerUser.ProfilePhoto
		}

		// Send real-time WebSocket event to active recipient sessions
		if isOnline {
			h.BroadcastToUser(targetUserID, WSEvent{
				Type:           "call_offer",
				ConversationID: convID,
				UserID:         client.UserID,
				Username:       client.Username,
				TargetUserID:   targetUserID,
				CallerName:     client.FullName,
				CallerAvatar:   avatar,
				CallType:       callType,
				CallID:         callLog.ID,
				SDP:            event.SDP,
			})
		}

		// Acknowledge initiation back to caller so client tracks authoritative CallID
		_ = client.Send(WSEvent{
			Type:           "call_initiated",
			ConversationID: convID,
			UserID:         client.UserID,
			TargetUserID:   targetUserID,
			CallType:       callType,
			CallID:         callLog.ID,
		})

		// Trigger Web Push notification for background delivery
		pushPayload := &models.CallNotificationPayload{
			Type:           "call_offer",
			CallID:         callLog.ID,
			ConversationID: convID,
			CallerID:       client.UserID,
			CallerName:     client.FullName,
			CallerAvatar:   avatar,
			CallType:       callType,
			Title:          fmt.Sprintf("Incoming %s call", callType),
			Body:           fmt.Sprintf("%s is calling on Youth Community...", client.FullName),
			URL:            fmt.Sprintf("/messages?c=%d&call_id=%d&autocall=%s", convID, callLog.ID, callType),
			Tag:            fmt.Sprintf("call-%d", callLog.ID),
			ExpiresAt:      expiresAt.Unix(),
		}
		_ = SendCallPushNotification(targetUserID, pushPayload)

		// Start authoritative ringing timer (45 seconds)
		go func(cID, callerID, targetID, callID int) {
			time.Sleep(45 * time.Second)
			cl, err := repository.GetCallLogByID(callID)
			if err == nil && cl != nil && (cl.Status == "initiated" || cl.Status == "ringing") {
				_ = repository.UpdateCallLog(callID, "missed", 0)
				_ = SendCallCancelPush(targetID, callID, cID)
				GlobalHub.BroadcastToUser(callerID, WSEvent{
					Type:           "call_unavailable",
					ConversationID: cID,
					UserID:         targetID,
					CallID:         callID,
					Error:          "No answer",
				})
				GlobalHub.BroadcastToUser(targetID, WSEvent{
					Type:           "call_cancel",
					ConversationID: cID,
					UserID:         callerID,
					TargetUserID:   targetID,
					CallID:         callID,
				})
			}
		}(convID, client.UserID, targetUserID, callLog.ID)

	case "call_answer":
		if event.CallID > 0 {
			cl, err := repository.GetCallLogByID(event.CallID)
			if err == nil && cl != nil {
				if cl.Status != "initiated" && cl.Status != "ringing" {
					_ = client.Send(WSEvent{
						Type:           "error",
						CallID:         event.CallID,
						ConversationID: event.ConversationID,
						Error:          "Call is no longer active",
					})
					return
				}
				_ = repository.UpdateCallLogStatus(event.CallID, "connected")
			}
		}
		if event.TargetUserID > 0 {
			h.BroadcastToUser(event.TargetUserID, WSEvent{
				Type:           "call_answer",
				ConversationID: event.ConversationID,
				UserID:         client.UserID,
				TargetUserID:   event.TargetUserID,
				CallID:         event.CallID,
				SDP:            event.SDP,
			})
		}

	case "ice_candidate":
		if event.TargetUserID > 0 {
			h.BroadcastToUser(event.TargetUserID, WSEvent{
				Type:           "ice_candidate",
				ConversationID: event.ConversationID,
				UserID:         client.UserID,
				TargetUserID:   event.TargetUserID,
				CallID:         event.CallID,
				Candidate:      event.Candidate,
			})
		}

	case "call_decline":
		if event.CallID <= 0 && event.TargetUserID > 0 {
			if latest, lErr := repository.GetLatestInitiatedCall(event.TargetUserID, client.UserID); lErr == nil && latest != nil {
				event.CallID = latest.ID
			}
		}
		if event.CallID > 0 {
			_ = repository.UpdateCallLog(event.CallID, "declined", 0)
			_ = SendCallCancelPush(client.UserID, event.CallID, event.ConversationID)
		}
		if event.TargetUserID > 0 {
			_ = SendCallCancelPush(event.TargetUserID, event.CallID, event.ConversationID)
			h.BroadcastToUser(event.TargetUserID, WSEvent{
				Type:           "call_decline",
				ConversationID: event.ConversationID,
				UserID:         client.UserID,
				TargetUserID:   event.TargetUserID,
				CallID:         event.CallID,
			})
		}
	case "call_cancel":
		if event.CallID <= 0 && event.TargetUserID > 0 {
			if latest, lErr := repository.GetLatestInitiatedCall(client.UserID, event.TargetUserID); lErr == nil && latest != nil {
				event.CallID = latest.ID
			}
		}
		if event.CallID > 0 {
			_ = repository.UpdateCallLog(event.CallID, "cancelled", 0)
			_ = SendCallCancelPush(client.UserID, event.CallID, event.ConversationID)
		}
		if event.TargetUserID > 0 {
			_ = SendCallCancelPush(event.TargetUserID, event.CallID, event.ConversationID)
			h.BroadcastToUser(event.TargetUserID, WSEvent{
				Type:           "call_cancel",
				ConversationID: event.ConversationID,
				UserID:         client.UserID,
				TargetUserID:   event.TargetUserID,
				CallID:         event.CallID,
			})
		}

	case "call_end":
		if event.CallID <= 0 && event.TargetUserID > 0 {
			if latest, lErr := repository.GetLatestInitiatedCall(client.UserID, event.TargetUserID); lErr == nil && latest != nil {
				event.CallID = latest.ID
			} else if latest2, lErr2 := repository.GetLatestInitiatedCall(event.TargetUserID, client.UserID); lErr2 == nil && latest2 != nil {
				event.CallID = latest2.ID
			}
		}
		if event.CallID > 0 {
			status := "completed"
			if event.Duration == 0 {
				status = "missed"
			}
			_ = repository.UpdateCallLog(event.CallID, status, event.Duration)
		}
		if event.TargetUserID > 0 {
			h.BroadcastToUser(event.TargetUserID, WSEvent{
				Type:           "call_end",
				ConversationID: event.ConversationID,
				UserID:         client.UserID,
				TargetUserID:   event.TargetUserID,
				CallID:         event.CallID,
				Duration:       event.Duration,
			})
		}

	case "call_busy":
		if event.CallID > 0 {
			_ = repository.UpdateCallLog(event.CallID, "busy", 0)
		}
		if event.TargetUserID > 0 {
			h.BroadcastToUser(event.TargetUserID, WSEvent{
				Type:           "call_busy",
				ConversationID: event.ConversationID,
				UserID:         client.UserID,
				TargetUserID:   event.TargetUserID,
				CallID:         event.CallID,
			})
		}

	case "group_call_start":
		convID := event.ConversationID
		if convID <= 0 {
			convID = client.SubscribedConvID
		}
		if convID > 0 {
			_, err := GlobalGroupCallManager.StartGroupCall(convID, client.UserID, event.CallType)
			if err != nil {
				_ = client.Send(WSEvent{Type: "error", Error: err.Error()})
			}
		}

	case "group_call_join":
		convID := event.ConversationID
		if convID <= 0 {
			convID = client.SubscribedConvID
		}
		if convID > 0 {
			_, _, err := GlobalGroupCallManager.JoinGroupCall(convID, client.UserID)
			if err != nil {
				_ = client.Send(WSEvent{Type: "error", Error: err.Error()})
			}
		}

	case "group_call_leave":
		convID := event.ConversationID
		if convID <= 0 {
			convID = client.SubscribedConvID
		}
		if convID > 0 {
			_, _, _ = GlobalGroupCallManager.LeaveGroupCall(convID, client.UserID)
		}

	case "group_call_state_update":
		convID := event.ConversationID
		if convID <= 0 {
			convID = client.SubscribedConvID
		}
		if convID > 0 {
			_, _ = GlobalGroupCallManager.UpdateParticipantMediaState(convID, client.UserID, event.IsMuted, event.IsCamOff, event.IsSpeaking)
		}

	case "group_call_signal":
		// Route WebRTC peer negotiation signals between participants in the group room
		convID := event.ConversationID
		if convID <= 0 {
			convID = client.SubscribedConvID
		}
		if convID > 0 && event.TargetUserID > 0 {
			room, exists := GlobalGroupCallManager.GetActiveGroupCall(convID)
			if exists {
				GlobalGroupCallManager.mu.RLock()
				_, senderInRoom := room.Participants[client.UserID]
				_, targetInRoom := room.Participants[event.TargetUserID]
				GlobalGroupCallManager.mu.RUnlock()

				if senderInRoom && targetInRoom {
					h.BroadcastToUser(event.TargetUserID, WSEvent{
						Type:           "group_call_signal",
						ConversationID: convID,
						UserID:         client.UserID,
						Username:       client.Username,
						CallerName:     client.FullName,
						TargetUserID:   event.TargetUserID,
						CallID:         room.CallID,
						SignalType:     event.SignalType,
						SDP:            event.SDP,
						Candidate:      event.Candidate,
					})
				}
			}
		}

	case "group_call_mute":
		convID := event.ConversationID
		if convID <= 0 {
			convID = client.SubscribedConvID
		}
		if convID > 0 && event.TargetUserID > 0 {
			_ = GlobalGroupCallManager.MuteParticipantInGroupCall(convID, client.UserID, event.TargetUserID)
		}

	case "group_call_kick":
		convID := event.ConversationID
		if convID <= 0 {
			convID = client.SubscribedConvID
		}
		if convID > 0 && event.TargetUserID > 0 {
			_ = GlobalGroupCallManager.KickParticipantFromGroupCall(convID, client.UserID, event.TargetUserID)
		}

	case "group_call_end":
		convID := event.ConversationID
		if convID <= 0 {
			convID = client.SubscribedConvID
		}
		if convID > 0 {
			_, _ = GlobalGroupCallManager.EndGroupCall(convID, client.UserID)
		}
	}
}

// ServeWebSocket manages the lifetime of a connected client.
func ServeWebSocket(ws *websocket.Conn, userID int, username, fullName string) {
	client := &WSClient{
		UserID:   userID,
		Username: username,
		FullName: fullName,
		Conn:     ws,
	}

	GlobalHub.Register(client)
	defer func() {
		GlobalHub.Unregister(client)
		_ = ws.Close()
	}()

	// Send initial connected confirmation
	_ = client.Send(WSEvent{
		Type:     "connected",
		UserID:   userID,
		Username: username,
	})

	for {
		var payload []byte
		err := websocket.Message.Receive(ws, &payload)
		if err != nil {
			break
		}

		GlobalHub.HandleClientMessage(client, payload)
	}
}

// Also hook SendMessage to notify GlobalHub in case HTTP API was used to send a message
func init() {
	// Hook function in chat_service.go can call GlobalHub.BroadcastToConversation
}
