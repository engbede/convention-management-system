package services

import (
	"errors"
	"fmt"
	"mime/multipart"
	"strings"

	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/utils"
)


// StartDirectConversation validates and returns or creates a 1-to-1 conversation.
func StartDirectConversation(currentUserID, targetUserID int) (*models.Conversation, error) {
	if currentUserID <= 0 || targetUserID <= 0 {
		return nil, errors.New("invalid user ID")
	}

	if currentUserID == targetUserID {
		return nil, errors.New("you cannot start a conversation with yourself")
	}

	// Verify target user exists
	targetUser, err := repository.GetUserByID(targetUserID)
	if err != nil {
		return nil, errors.New("target member not found")
	}

	if targetUser.ID == 0 {
		return nil, errors.New("target member account is invalid")
	}

	// Verify communication is not blocked between participants
	blocked, err := repository.IsCommunicationBlocked(currentUserID, targetUserID)
	if err != nil {
		return nil, err
	}
	if blocked {
		return nil, errors.New("cannot start conversation: communication between these members is blocked")
	}

	// Create or retrieve existing direct conversation
	conv, err := repository.CreateOrGetDirectConversation(currentUserID, targetUserID)
	if err != nil {
		return nil, err
	}

	return conv, nil
}

// GetConversationDetails verifies membership and retrieves conversation + recipient details.
func GetConversationDetails(conversationID, currentUserID int) (*models.Conversation, *models.User, error) {
	if conversationID <= 0 || currentUserID <= 0 {
		return nil, nil, errors.New("invalid conversation or user ID")
	}

	isMember, err := repository.IsConversationMember(conversationID, currentUserID)
	if err != nil {
		return nil, nil, err
	}
	if !isMember {
		return nil, nil, errors.New("access denied: you are not a participant in this conversation")
	}

	conv, err := repository.GetConversationByID(conversationID)
	if err != nil {
		return nil, nil, err
	}

	var recipient *models.User
	if conv.Type == "direct" {
		recipient, err = repository.GetConversationOtherParticipant(conversationID, currentUserID)
		if err != nil {
			return nil, nil, err
		}
	}

	return conv, recipient, nil
}

// GetConversationMessages verifies membership, fetches messages, updates the read receipt and clears message notifications.
func GetConversationMessages(conversationID, currentUserID, limit, offset int) ([]models.Message, error) {
	isMember, err := repository.IsConversationMember(conversationID, currentUserID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, errors.New("access denied: you are not a participant in this conversation")
	}

	messages, err := repository.GetConversationMessages(conversationID, limit, offset, currentUserID)
	if err != nil {
		return nil, err
	}

	// Mark conversation as read for the current user
	_ = repository.MarkConversationAsRead(conversationID, currentUserID)

	// Clear unread notifications from other participants for this conversation
	otherUser, err := repository.GetConversationOtherParticipant(conversationID, currentUserID)
	if err == nil && otherUser != nil {
		_ = repository.MarkMessageNotificationsAsRead(currentUserID, otherUser.ID)
	}

	return messages, nil
}

// SendMessage validates input, verifies membership, saves the message, and triggers notifications.
func SendMessage(conversationID, senderID int, body, clientMsgID string, replyTo *int) (*models.Message, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, errors.New("message cannot be empty")
	}

	if len(body) > 2000 {
		return nil, errors.New("message exceeds the 2,000 character limit")
	}

	// Verify membership
	isMember, err := repository.IsConversationMember(conversationID, senderID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, errors.New("access denied: you are not a participant in this conversation")
	}

	// Verify communication is not blocked in direct conversations
	conv, otherUser, err := GetConversationDetails(conversationID, senderID)
	if err == nil && conv.Type == "direct" && otherUser != nil {
		blocked, bErr := repository.IsCommunicationBlocked(senderID, otherUser.ID)
		if bErr == nil && blocked {
			return nil, errors.New("cannot send message: communication between these members is blocked")
		}
	}

	// Construct message
	msg := &models.Message{
		ConversationID:   conversationID,
		SenderID:         senderID,
		MessageType:      "text",
		Body:             body,
		ClientMessageID:  clientMsgID,
		ReplyToMessageID: replyTo,
	}

	if err := repository.SaveMessage(msg); err != nil {
		return nil, err
	}

	// Hydrate sender details
	sender, err := repository.GetUserByID(senderID)
	if err == nil {
		msg.Sender = sender
	}

	// Send notification to the other participant if direct conversation and not muted
	if conv != nil && conv.Type == "direct" && otherUser != nil {
		if !repository.IsConversationMutedBy(conversationID, otherUser.ID) {
			notification := models.Notification{
				SenderID:   senderID,
				ReceiverID: otherUser.ID,
				Type:       "message",
				Message:    "sent you a new message.",
			}
			_ = CreateNotification(notification)
		}
	}

	// Broadcast new message to all active WebSocket clients in real-time
	GlobalHub.BroadcastToConversation(conversationID, WSEvent{
		Type:           "new_message",
		ConversationID: conversationID,
		Message:        msg,
	})

	return msg, nil
}

// SendAttachmentMessage saves an uploaded attachment and creates an attachment message in the conversation.
func SendAttachmentMessage(
	conversationID, senderID int,
	file multipart.File,
	header *multipart.FileHeader,
	caption string,
	clientMsgID string,
	replyTo *int,
) (*models.Message, error) {
	// Verify membership
	isMember, err := repository.IsConversationMember(conversationID, senderID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, errors.New("access denied: you are not a participant in this conversation")
	}

	// Verify communication is not blocked in direct conversations
	conv, otherUser, err := GetConversationDetails(conversationID, senderID)
	if err == nil && conv.Type == "direct" && otherUser != nil {
		blocked, bErr := repository.IsCommunicationBlocked(senderID, otherUser.ID)
		if bErr == nil && blocked {
			return nil, errors.New("cannot send message: communication between these members is blocked")
		}
	}

	// Save file via utils.SaveChatAttachment
	filename, msgType, contentType, err := utils.SaveChatAttachment(file, header, "uploads/chat")
	if err != nil {
		return nil, err
	}

	caption = strings.TrimSpace(caption)
	if len(caption) > 2000 {
		return nil, errors.New("caption exceeds the 2,000 character limit")
	}

	msg := &models.Message{
		ConversationID:   conversationID,
		SenderID:         senderID,
		MessageType:      msgType,
		Body:             caption,
		ClientMessageID:  clientMsgID,
		ReplyToMessageID: replyTo,
		AttachmentURL:    "/uploads/chat/" + filename,
		AttachmentName:   header.Filename,
		AttachmentSize:   header.Size,
		AttachmentType:   contentType,
	}

	if err := repository.SaveMessage(msg); err != nil {
		return nil, err
	}

	sender, err := repository.GetUserByID(senderID)
	if err == nil {
		msg.Sender = sender
	}

	// Send notification if not muted
	if conv != nil && conv.Type == "direct" && otherUser != nil {
		if !repository.IsConversationMutedBy(conversationID, otherUser.ID) {
			notification := models.Notification{
				SenderID:   senderID,
				ReceiverID: otherUser.ID,
				Type:       "message",
				Message:    "sent you an attachment.",
			}
			_ = CreateNotification(notification)
		}
	}

	// Broadcast to active WebSocket clients
	GlobalHub.BroadcastToConversation(conversationID, WSEvent{
		Type:           "new_message",
		ConversationID: conversationID,
		Message:        msg,
	})

	return msg, nil
}

// GetUserInbox returns the member's conversations and unread count.
func GetUserInbox(userID int) ([]models.ConversationItem, int, error) {
	conversations, err := repository.GetUserConversations(userID)
	if err != nil {
		return nil, 0, err
	}

	unread, err := repository.CountTotalUnreadMessages(userID)
	if err != nil {
		unread = 0
	}

	return conversations, unread, nil
}

// MarkConversationAsRead marks messages in a conversation as read by the user and broadcasts read receipt.
func MarkConversationAsRead(conversationID, userID int) error {
	err := repository.MarkConversationAsRead(conversationID, userID)
	if err == nil {
		GlobalHub.BroadcastToConversation(conversationID, WSEvent{
			Type:           "read_receipt",
			ConversationID: conversationID,
			UserID:         userID,
		})
	}
	return err
}

// ToggleMuteConversation toggles the mute state for the user in the specified conversation.
func ToggleMuteConversation(conversationID, userID int) (bool, error) {
	isMember, err := repository.IsConversationMember(conversationID, userID)
	if err != nil {
		return false, err
	}
	if !isMember {
		return false, errors.New("access denied: not a member of conversation")
	}
	return repository.ToggleMuteConversation(conversationID, userID)
}

// ToggleArchiveConversation toggles the archive state for the user in the specified conversation.
func ToggleArchiveConversation(conversationID, userID int) (bool, error) {
	isMember, err := repository.IsConversationMember(conversationID, userID)
	if err != nil {
		return false, err
	}
	if !isMember {
		return false, errors.New("access denied: not a member of conversation")
	}
	return repository.ToggleArchiveConversation(conversationID, userID)
}

// GetConversationMemberSettings gets mute and archive states for a member.
func GetConversationMemberSettings(conversationID, userID int) (bool, bool, error) {
	return repository.GetConversationMemberSettings(conversationID, userID)
}

// HasOlderMessages checks if there are older messages prior to the current offset.
func HasOlderMessages(conversationID, currentCount int) bool {
	total, err := repository.CountMessagesInConversation(conversationID)
	if err != nil {
		return false
	}
	return total > currentCount
}

// GetEligibleDirectChatUsers returns a list of candidate users to chat with, excluding blocked members.
func GetEligibleDirectChatUsers(currentUserID int) ([]models.User, error) {
	var candidates []models.User

	// First try to get existing friends
	friends, err := GetFriends(currentUserID)
	if err == nil && len(friends) > 0 {
		for _, f := range friends {
			if f.User.ID != currentUserID && f.User.ID != 0 {
				candidates = append(candidates, f.User)
			}
		}
	}

	if len(candidates) == 0 {
		candidates, _ = repository.GetDiscoverUsers(currentUserID)
	}

	// Filter out blocked users
	var eligible []models.User
	for _, u := range candidates {
		blocked, _ := repository.IsCommunicationBlocked(currentUserID, u.ID)
		if !blocked {
			eligible = append(eligible, u)
		}
	}

	return eligible, nil
}

// BlockUser blocks targetUserID on behalf of blockerID.
func BlockUser(blockerID, targetUserID int, reason string) error {
	if blockerID <= 0 || targetUserID <= 0 {
		return errors.New("invalid user ID")
	}
	if blockerID == targetUserID {
		return errors.New("you cannot block yourself")
	}

	// Verify target user exists
	targetUser, err := repository.GetUserByID(targetUserID)
	if err != nil || targetUser.ID == 0 {
		return errors.New("member not found")
	}

	return repository.BlockUser(blockerID, targetUserID, reason)
}

// UnblockUser removes a block on targetUserID.
func UnblockUser(blockerID, targetUserID int) error {
	if blockerID <= 0 || targetUserID <= 0 {
		return errors.New("invalid user ID")
	}
	return repository.UnblockUser(blockerID, targetUserID)
}

// IsUserBlocked checks if blockerID has blocked targetUserID.
func IsUserBlocked(blockerID, targetUserID int) (bool, error) {
	return repository.IsUserBlocked(blockerID, targetUserID)
}

// IsCommunicationBlocked checks if user1 has blocked user2 or vice-versa.
func IsCommunicationBlocked(user1ID, user2ID int) (bool, error) {
	return repository.IsCommunicationBlocked(user1ID, user2ID)
}

// GetBlockedUsers returns list of users blocked by blockerID.
func GetBlockedUsers(blockerID int) ([]models.User, error) {
	return repository.GetBlockedUsers(blockerID)
}

// ReportAbuse records an abuse report against a member or message.
func ReportAbuse(reporterID, reportedUserID int, convID, msgID *int, reason, details string) (*models.ChatReport, error) {
	if reporterID <= 0 || reportedUserID <= 0 {
		return nil, errors.New("invalid user ID")
	}
	if reporterID == reportedUserID {
		return nil, errors.New("cannot report yourself")
	}

	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("report reason is required")
	}

	report := &models.ChatReport{
		ReporterID:     reporterID,
		ReportedUserID: reportedUserID,
		ConversationID: convID,
		MessageID:      msgID,
		Reason:         reason,
		Details:        strings.TrimSpace(details),
		Status:         "pending",
	}

	if err := repository.CreateChatReport(report); err != nil {
		return nil, err
	}
	return report, nil
}

// SearchMessages searches messages containing query.
func SearchMessages(userID int, query string, convID *int, limit, offset int) ([]*models.Message, error) {
	query = strings.TrimSpace(query)
	if len(query) < 1 {
		return []*models.Message{}, nil
	}

	if convID != nil && *convID > 0 {
		isMember, err := repository.IsConversationMember(*convID, userID)
		if err != nil {
			return nil, err
		}
		if !isMember {
			return nil, errors.New("access denied: you are not a participant in this conversation")
		}
	}

	return repository.SearchMessages(userID, query, convID, limit, offset)
}

// GetSharedMedia retrieves media and attachments for a conversation.
func GetSharedMedia(conversationID, userID int) ([]models.Message, error) {
	isMember, err := repository.IsConversationMember(conversationID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, errors.New("access denied: you are not a participant in this conversation")
	}

	return repository.GetConversationSharedMedia(conversationID, 20)
}

// CreateCallLog creates and persists a new call session.
func CreateCallLog(convID, callerID, receiverID int, callType, status string) (*models.CallLog, error) {
	if convID <= 0 || callerID <= 0 || receiverID <= 0 {
		return nil, errors.New("invalid call parameters")
	}

	call := &models.CallLog{
		ConversationID: convID,
		CallerID:       callerID,
		ReceiverID:     receiverID,
		CallType:       callType,
		Status:         status,
	}

	if err := repository.CreateCallLog(call); err != nil {
		return nil, err
	}
	return call, nil
}

// UpdateCallLog records call completion and duration.
func UpdateCallLog(callID int, status string, durationSeconds int) error {
	if callID <= 0 {
		return errors.New("invalid call ID")
	}
	return repository.UpdateCallLog(callID, status, durationSeconds)
}

// GetConversationCallLogs retrieves call history for a conversation.
func GetConversationCallLogs(conversationID, userID int) ([]models.CallLog, error) {
	isMember, err := repository.IsConversationMember(conversationID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, errors.New("access denied: you are not a participant in this conversation")
	}

	return repository.GetConversationCallLogs(conversationID, 20)
}

// GetICEServers returns standard WebRTC STUN/TURN server configuration.
func GetICEServers() []map[string]any {
	return []map[string]any{
		{
			"urls": []string{
				"stun:stun.l.google.com:19302",
				"stun:stun1.l.google.com:19302",
				"stun:stun2.l.google.com:19302",
				"stun:stun3.l.google.com:19302",
				"stun:stun4.l.google.com:19302",
			},
		},
	}
}

// CreateGroup creates a new community group with validation, enrollment, and a welcoming system message.
func CreateGroup(creatorID int, title, category, description, callPermission string, isPublic bool, initialMemberIDs []int) (*models.Conversation, error) {
	title = strings.TrimSpace(title)
	if len(title) < 2 || len(title) > 100 {
		return nil, errors.New("group title must be between 2 and 100 characters")
	}

	validCategories := map[string]bool{
		"general":     true,
		"bible_study": true,
		"prayer":      true,
		"evangelism":  true,
		"other":       true,
	}
	if category == "" || !validCategories[category] {
		category = "general"
	}

	if callPermission != "admin_only" {
		callPermission = "all"
	}

	conv, err := repository.CreateGroupConversation(title, category, description, "", callPermission, isPublic, creatorID, initialMemberIDs)
	if err != nil {
		return nil, err
	}

	creator, _ := repository.GetUserByID(creatorID)
	creatorName := "Member"
	if creator.FullName != "" {
		creatorName = creator.FullName
	}

	// Post system message
	sysMsg := &models.Message{
		ConversationID: conv.ID,
		SenderID:       creatorID,
		MessageType:    "system",
		Body:           fmt.Sprintf("%s created the group \"%s\".", creatorName, title),
	}
	_ = repository.SaveMessage(sysMsg)

	return conv, nil
}

// JoinGroup adds an authenticated user to a public community group.
func JoinGroup(userID, groupID int) error {
	conv, err := repository.GetConversationByID(groupID)
	if err != nil {
		return err
	}
	if conv.Type != "group" {
		return errors.New("cannot join a direct conversation")
	}
	if !conv.IsPublic {
		return errors.New("this group is private and requires an invitation")
	}

	isMember, _ := repository.IsConversationMember(groupID, userID)
	if isMember {
		return nil // already a member
	}

	if err := repository.AddConversationMember(groupID, userID, "member"); err != nil {
		return err
	}

	user, _ := repository.GetUserByID(userID)
	userName := "A member"
	if user.FullName != "" {
		userName = user.FullName
	}

	sysMsg := &models.Message{
		ConversationID: groupID,
		SenderID:       userID,
		MessageType:    "system",
		Body:           fmt.Sprintf("%s joined the group.", userName),
	}
	_ = repository.SaveMessage(sysMsg)

	GlobalHub.BroadcastToConversation(groupID, WSEvent{
		Type:           "new_message",
		ConversationID: groupID,
		Message:        sysMsg,
	})

	return nil
}

// LeaveGroup removes the user from the group.
func LeaveGroup(userID, groupID int) error {
	conv, err := repository.GetConversationByID(groupID)
	if err != nil {
		return err
	}
	if conv.Type != "group" {
		return errors.New("cannot leave a direct conversation")
	}

	member, err := repository.GetConversationMember(groupID, userID)
	if err != nil {
		return errors.New("you are not a member of this group")
	}

	// If owner is leaving, transfer ownership to oldest admin or member
	if member.Role == "owner" {
		members, _ := repository.GetConversationMembers(groupID)
		var nextOwnerID int
		for _, m := range members {
			if m.UserID != userID {
				nextOwnerID = m.UserID
				if m.Role == "admin" {
					break
				}
			}
		}
		if nextOwnerID != 0 {
			_ = repository.UpdateConversationMemberRole(groupID, nextOwnerID, "owner")
		}
	}

	if err := repository.RemoveConversationMember(groupID, userID); err != nil {
		return err
	}

	user, _ := repository.GetUserByID(userID)
	userName := "A member"
	if user.FullName != "" {
		userName = user.FullName
	}

	sysMsg := &models.Message{
		ConversationID: groupID,
		SenderID:       userID,
		MessageType:    "system",
		Body:           fmt.Sprintf("%s left the group.", userName),
	}
	_ = repository.SaveMessage(sysMsg)

	GlobalHub.BroadcastToConversation(groupID, WSEvent{
		Type:           "new_message",
		ConversationID: groupID,
		Message:        sysMsg,
	})

	return nil
}

// AddGroupMembers adds new members to a group if acting user has permission (owner, admin, or moderator).
func AddGroupMembers(actingUserID, groupID int, userIDs []int) error {
	actingMember, err := repository.GetConversationMember(groupID, actingUserID)
	if err != nil || (actingMember.Role != "owner" && actingMember.Role != "admin" && actingMember.Role != "moderator") {
		return errors.New("access denied: only group administrators and moderators can add members")
	}

	var addedNames []string
	for _, targetID := range userIDs {
		if targetID <= 0 || targetID == actingUserID {
			continue
		}
		// Check blocking
		blocked, _ := repository.IsCommunicationBlocked(actingUserID, targetID)
		if blocked {
			continue
		}

		if err := repository.AddConversationMember(groupID, targetID, "member"); err == nil {
			u, uErr := repository.GetUserByID(targetID)
			if uErr == nil && u.FullName != "" {
				addedNames = append(addedNames, u.FullName)
			}
		}
	}

	if len(addedNames) > 0 {
		sysMsg := &models.Message{
			ConversationID: groupID,
			SenderID:       actingUserID,
			MessageType:    "system",
			Body:           fmt.Sprintf("%s added %s to the group.", actingMember.User.FullName, strings.Join(addedNames, ", ")),
		}
		_ = repository.SaveMessage(sysMsg)

		GlobalHub.BroadcastToConversation(groupID, WSEvent{
			Type:           "new_message",
			ConversationID: groupID,
			Message:        sysMsg,
		})
	}

	return nil
}

// RemoveGroupMember removes a participant from a group (moderator or admin privilege required).
func RemoveGroupMember(actingUserID, groupID, targetUserID int) error {
	if actingUserID == targetUserID {
		return LeaveGroup(actingUserID, groupID)
	}

	actingMember, err := repository.GetConversationMember(groupID, actingUserID)
	if err != nil {
		return errors.New("access denied: you are not a member of this group")
	}

	targetMember, err := repository.GetConversationMember(groupID, targetUserID)
	if err != nil {
		return errors.New("target is not a member of this group")
	}

	// Owner can remove anyone; Admin can remove moderators and members
	if actingMember.Role == "owner" {
		// allowed
	} else if actingMember.Role == "admin" && targetMember.Role != "owner" && targetMember.Role != "admin" {
		// allowed
	} else {
		return errors.New("access denied: insufficient permissions to remove this member")
	}

	if err := repository.RemoveConversationMember(groupID, targetUserID); err != nil {
		return err
	}

	sysMsg := &models.Message{
		ConversationID: groupID,
		SenderID:       actingUserID,
		MessageType:    "system",
		Body:           fmt.Sprintf("%s was removed from the group.", targetMember.User.FullName),
	}
	_ = repository.SaveMessage(sysMsg)

	GlobalHub.BroadcastToConversation(groupID, WSEvent{
		Type:           "new_message",
		ConversationID: groupID,
		Message:        sysMsg,
	})

	return nil
}

// UpdateGroupMemberRole modifies a participant's role (owner privileges required).
func UpdateGroupMemberRole(actingUserID, groupID, targetUserID int, newRole string) error {
	if newRole != "admin" && newRole != "moderator" && newRole != "member" {
		return errors.New("invalid role")
	}

	actingMember, err := repository.GetConversationMember(groupID, actingUserID)
	if err != nil || actingMember.Role != "owner" {
		return errors.New("access denied: only the group owner can change member roles")
	}

	if targetUserID == actingUserID {
		return errors.New("cannot change owner's own role")
	}

	if err := repository.UpdateConversationMemberRole(groupID, targetUserID, newRole); err != nil {
		return err
	}

	targetUser, _ := repository.GetUserByID(targetUserID)
	sysMsg := &models.Message{
		ConversationID: groupID,
		SenderID:       actingUserID,
		MessageType:    "system",
		Body:           fmt.Sprintf("%s is now a %s.", targetUser.FullName, newRole),
	}
	_ = repository.SaveMessage(sysMsg)

	GlobalHub.BroadcastToConversation(groupID, WSEvent{
		Type:           "new_message",
		ConversationID: groupID,
		Message:        sysMsg,
	})

	return nil
}

// UpdateGroupSettings updates group title, category, description, and call permissions.
func UpdateGroupSettings(actingUserID, groupID int, title, category, description, callPermission string) error {
	title = strings.TrimSpace(title)
	if len(title) < 2 || len(title) > 100 {
		return errors.New("title must be between 2 and 100 characters")
	}

	actingMember, err := repository.GetConversationMember(groupID, actingUserID)
	if err != nil || (actingMember.Role != "owner" && actingMember.Role != "admin") {
		return errors.New("access denied: only group administrators can update group settings")
	}

	validCategories := map[string]bool{
		"general":     true,
		"bible_study": true,
		"prayer":      true,
		"evangelism":  true,
		"other":       true,
	}
	if category == "" || !validCategories[category] {
		category = "general"
	}

	if callPermission != "admin_only" {
		callPermission = "all"
	}

	return repository.UpdateGroupDetails(groupID, title, category, description, callPermission)
}

// GetGroupMembers retrieves members for a conversation if acting user is a participant.
func GetGroupMembers(conversationID, userID int) ([]models.ConversationMember, error) {
	isMember, err := repository.IsConversationMember(conversationID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, errors.New("access denied: not a participant")
	}
	return repository.GetConversationMembers(conversationID)
}

// GetDiscoverableGroups retrieves public community groups.
func GetDiscoverableGroups(userID int, category string) ([]models.ConversationItem, error) {
	return repository.GetDiscoverableGroups(userID, category)
}

// CanUserCallInGroup checks whether a user has permission to initiate a call in the conversation.
func CanUserCallInGroup(convID, userID int) (bool, error) {
	conv, err := repository.GetConversationByID(convID)
	if err != nil {
		return false, err
	}
	if conv.Type == "direct" {
		return true, nil
	}

	member, err := repository.GetConversationMember(convID, userID)
	if err != nil {
		return false, err
	}

	if conv.CallPermission == "admin_only" {
		return member.Role == "owner" || member.Role == "admin", nil
	}

	return true, nil
}



