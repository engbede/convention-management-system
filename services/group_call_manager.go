package services

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"convention-management-system/models"
	"convention-management-system/repository"
)

// GroupCallManager maintains concurrent, thread-safe in-memory state for active group calls.
type GroupCallManager struct {
	rooms map[int]*models.GroupCallRoom // key: conversation_id
	mu    sync.RWMutex
}

// GlobalGroupCallManager is the singleton instance of the group call manager.
var GlobalGroupCallManager = NewGroupCallManager()

func NewGroupCallManager() *GroupCallManager {
	return &GroupCallManager{
		rooms: make(map[int]*models.GroupCallRoom),
	}
}

// StartGroupCall initiates a new group audio or video call session.
func (m *GroupCallManager) StartGroupCall(convID, initiatorID int, callType string) (*models.GroupCallRoom, error) {
	if convID <= 0 || initiatorID <= 0 {
		return nil, errors.New("invalid conversation or initiator ID")
	}

	conv, err := repository.GetConversationByID(convID)
	if err != nil {
		return nil, err
	}
	if conv == nil || conv.Type != "group" {
		return nil, errors.New("group calling is only available for community group conversations")
	}

	canCall, err := CanUserCallInGroup(convID, initiatorID)
	if err != nil || !canCall {
		return nil, errors.New("access denied: you do not have permission to start a call in this group")
	}

	if callType != "video" {
		callType = "audio"
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// If room already active, join the existing room
	if existingRoom, exists := m.rooms[convID]; exists {
		return existingRoom, nil
	}

	// Persist call log in database
	callLog := &models.CallLog{
		ConversationID: convID,
		CallerID:       initiatorID,
		ReceiverID:     0, // 0 denotes group call
		CallType:       callType,
		Status:         "initiated",
	}
	if err := repository.CreateCallLog(callLog); err != nil {
		return nil, err
	}

	// Record initiator participation in database
	_, _ = repository.AddCallParticipant(callLog.ID, initiatorID)

	initiatorUser, _ := repository.GetUserByID(initiatorID)
	initiatorMember, _ := repository.GetConversationMember(convID, initiatorID)
	role := "member"
	if initiatorMember != nil && initiatorMember.Role != "" {
		role = initiatorMember.Role
	}

	initiatorInfo := &models.CallParticipantInfo{
		UserID:       initiatorID,
		FullName:     initiatorUser.FullName,
		Username:     initiatorUser.Username,
		ProfilePhoto: initiatorUser.ProfilePhoto,
		Role:         role,
		JoinedAt:     time.Now(),
		IsMuted:      false,
		IsCamOff:     callType == "audio",
		IsSpeaking:   false,
	}

	room := &models.GroupCallRoom{
		CallID:         callLog.ID,
		ConversationID: convID,
		InitiatorID:    initiatorID,
		InitiatorName:  initiatorUser.FullName,
		CallType:       callType,
		StartedAt:      time.Now(),
		Participants: map[int]*models.CallParticipantInfo{
			initiatorID: initiatorInfo,
		},
	}

	m.rooms[convID] = room

	// Post system message to the chat
	initiatorName := initiatorUser.FullName
	if initiatorName == "" {
		initiatorName = "Member"
	}
	callName := "audio"
	if callType == "video" {
		callName = "video"
	}
	sysMsg := &models.Message{
		ConversationID: convID,
		SenderID:       initiatorID,
		MessageType:    "system",
		Body:           fmt.Sprintf("%s started a group %s call.", initiatorName, callName),
	}
	_ = repository.SaveMessage(sysMsg)

	// Broadcast call started event to conversation
	go GlobalHub.BroadcastToConversation(convID, WSEvent{
		Type:           "group_call_started",
		ConversationID: convID,
		CallID:         callLog.ID,
		CallType:       callType,
		UserID:         initiatorID,
		Username:       initiatorUser.Username,
		CallerName:     initiatorName,
		CallerAvatar:   initiatorUser.ProfilePhoto,
		Room:           room,
		Message:        sysMsg,
	})

	return room, nil
}

// JoinGroupCall enrolls an authenticated user into an active group call session.
func (m *GroupCallManager) JoinGroupCall(convID, userID int) (*models.GroupCallRoom, *models.CallParticipantInfo, error) {
	if convID <= 0 || userID <= 0 {
		return nil, nil, errors.New("invalid conversation or user ID")
	}

	isMember, err := repository.IsConversationMember(convID, userID)
	if err != nil || !isMember {
		return nil, nil, errors.New("access denied: you are not a member of this group")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	room, exists := m.rooms[convID]
	if !exists {
		return nil, nil, errors.New("no active call in this group")
	}

	// If already in room, return participant info
	if p, ok := room.Participants[userID]; ok {
		return room, p, nil
	}

	// Check maximum capacity limit (16 participants to ensure stability)
	if len(room.Participants) >= 16 {
		return nil, nil, errors.New("group call is full (maximum 16 participants reached)")
	}

	user, err := repository.GetUserByID(userID)
	if err != nil {
		return nil, nil, err
	}

	convMember, _ := repository.GetConversationMember(convID, userID)
	role := "member"
	if convMember != nil && convMember.Role != "" {
		role = convMember.Role
	}

	// Record in database
	_, _ = repository.AddCallParticipant(room.CallID, userID)

	participant := &models.CallParticipantInfo{
		UserID:       userID,
		FullName:     user.FullName,
		Username:     user.Username,
		ProfilePhoto: user.ProfilePhoto,
		Role:         role,
		JoinedAt:     time.Now(),
		IsMuted:      false,
		IsCamOff:     room.CallType == "audio",
		IsSpeaking:   false,
	}

	room.Participants[userID] = participant

	// Convert participants map to slice for clean JSON broadcast
	var roster []*models.CallParticipantInfo
	for _, p := range room.Participants {
		roster = append(roster, p)
	}

	// Broadcast newcomer to all conversation members
	go GlobalHub.BroadcastToConversation(convID, WSEvent{
		Type:           "group_call_participant_joined",
		ConversationID: convID,
		CallID:         room.CallID,
		UserID:         userID,
		Username:       user.Username,
		CallerName:     user.FullName,
		CallerAvatar:   user.ProfilePhoto,
		Participant:    participant,
		Participants:   roster,
		Room:           room,
	})

	return room, participant, nil
}

// LeaveGroupCall removes a user from an active group call session.
func (m *GroupCallManager) LeaveGroupCall(convID, userID int) (*models.GroupCallRoom, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, exists := m.rooms[convID]
	if !exists {
		return nil, 0, nil
	}

	p, ok := room.Participants[userID]
	if !ok {
		return room, 0, nil
	}

	duration := int(time.Since(p.JoinedAt).Seconds())
	if duration <= 0 {
		duration = 1
	}

	// Update individual participant record in database
	_ = repository.UpdateCallParticipantLeft(room.CallID, userID, duration)
	delete(room.Participants, userID)

	// If no more participants remain, terminate the room
	if len(room.Participants) == 0 {
		totalDuration := int(time.Since(room.StartedAt).Seconds())
		_ = repository.UpdateCallLog(room.CallID, "completed", totalDuration)
		delete(m.rooms, convID)

		go GlobalHub.BroadcastToConversation(convID, WSEvent{
			Type:           "group_call_ended",
			ConversationID: convID,
			CallID:         room.CallID,
			Duration:       totalDuration,
		})

		return room, duration, nil
	}

	// Convert remaining participants map to slice
	var roster []*models.CallParticipantInfo
	for _, rp := range room.Participants {
		roster = append(roster, rp)
	}

	go GlobalHub.BroadcastToConversation(convID, WSEvent{
		Type:           "group_call_participant_left",
		ConversationID: convID,
		CallID:         room.CallID,
		UserID:         userID,
		Duration:       duration,
		Participants:   roster,
		Room:           room,
	})

	return room, duration, nil
}

// UpdateParticipantMediaState updates mute, camera, or speaking state of an active participant.
func (m *GroupCallManager) UpdateParticipantMediaState(convID, userID int, isMuted, isCamOff, isSpeaking bool) (*models.CallParticipantInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, exists := m.rooms[convID]
	if !exists {
		return nil, errors.New("no active call in this group")
	}

	p, ok := room.Participants[userID]
	if !ok {
		return nil, errors.New("participant not in call")
	}

	p.IsMuted = isMuted
	p.IsCamOff = isCamOff
	p.IsSpeaking = isSpeaking

	go GlobalHub.BroadcastToConversation(convID, WSEvent{
		Type:           "group_call_state_updated",
		ConversationID: convID,
		CallID:         room.CallID,
		UserID:         userID,
		IsMuted:        isMuted,
		IsCamOff:       isCamOff,
		IsSpeaking:     isSpeaking,
		Participant:    p,
	})

	return p, nil
}

// KickParticipantFromGroupCall removes a user from an active call (owner or admin permission required).
func (m *GroupCallManager) KickParticipantFromGroupCall(convID, actorUserID, targetUserID int) error {
	actorMember, err := repository.GetConversationMember(convID, actorUserID)
	if err != nil || (actorMember.Role != "owner" && actorMember.Role != "admin" && actorMember.Role != "moderator") {
		return errors.New("access denied: only group administrators and moderators can remove participants from a call")
	}

	m.mu.RLock()
	room, exists := m.rooms[convID]
	m.mu.RUnlock()

	if !exists {
		return errors.New("no active call in this group")
	}

	targetMember, _ := repository.GetConversationMember(convID, targetUserID)
	if targetMember != nil && (targetMember.Role == "owner" || (targetMember.Role == "admin" && actorMember.Role != "owner")) {
		return errors.New("access denied: cannot remove a higher-ranking administrator from the call")
	}

	// Remove participant from room
	_, _, err = m.LeaveGroupCall(convID, targetUserID)
	if err != nil {
		return err
	}

	// Send direct notification to kicked user
	go GlobalHub.BroadcastToUser(targetUserID, WSEvent{
		Type:           "group_call_kicked",
		ConversationID: convID,
		CallID:         room.CallID,
		Error:          "You were removed from the call by an administrator",
	})

	return nil
}

// EndGroupCall terminates an active group call for all participants (host or admin permission required).
func (m *GroupCallManager) EndGroupCall(convID, actorUserID int) (*models.GroupCallRoom, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, exists := m.rooms[convID]
	if !exists {
		return nil, errors.New("no active call in this group")
	}

	actorMember, _ := repository.GetConversationMember(convID, actorUserID)
	isAuthorized := actorUserID == room.InitiatorID ||
		(actorMember != nil && (actorMember.Role == "owner" || actorMember.Role == "admin"))

	if !isAuthorized {
		return nil, errors.New("access denied: only the host or group administrators can end the call for everyone")
	}

	totalDuration := int(time.Since(room.StartedAt).Seconds())
	if totalDuration <= 0 {
		totalDuration = 1
	}

	// Record leave for all remaining participants
	for uid, p := range room.Participants {
		pDuration := int(time.Since(p.JoinedAt).Seconds())
		_ = repository.UpdateCallParticipantLeft(room.CallID, uid, pDuration)
	}

	// Update call log
	_ = repository.UpdateCallLog(room.CallID, "completed", totalDuration)
	delete(m.rooms, convID)

	go GlobalHub.BroadcastToConversation(convID, WSEvent{
		Type:           "group_call_ended",
		ConversationID: convID,
		CallID:         room.CallID,
		Duration:       totalDuration,
	})

	return room, nil
}

// GetActiveGroupCall retrieves the active room for a conversation if one exists.
func (m *GroupCallManager) GetActiveGroupCall(convID int) (*models.GroupCallRoom, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	room, exists := m.rooms[convID]
	return room, exists
}
