package services

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"regexp"
	"strings"
	"time"

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

	// Hydrate quoted reply if present
	if msg.ReplyToMessageID != nil && *msg.ReplyToMessageID > 0 {
		if rep, rErr := repository.GetMessageByID(*msg.ReplyToMessageID); rErr == nil {
			msg.ReplyToMessage = rep
		}
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

	// Process @mentions for conversation members
	processMentions(conversationID, senderID, msg.Sender.FullName, conv, msg.Body)

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

	// Hydrate quoted reply if present
	if msg.ReplyToMessageID != nil && *msg.ReplyToMessageID > 0 {
		if rep, rErr := repository.GetMessageByID(*msg.ReplyToMessageID); rErr == nil {
			msg.ReplyToMessage = rep
		}
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

	// Process @mentions for conversation members
	processMentions(conversationID, senderID, msg.Sender.FullName, conv, msg.Body)

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

	// If user is not yet part of any conversations, ensure they are enrolled in the open default Youth General Community
	if len(conversations) == 0 {
		if err := repository.EnsureDefaultCommunityMembership(userID); err == nil {
			if reloaded, rErr := repository.GetUserConversations(userID); rErr == nil {
				conversations = reloaded
			}
		}
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

// SetConversationMute sets the mute preference for a user in a conversation with duration options ("1h", "8h", "1w", "indefinite", "unmute").
func SetConversationMute(conversationID, userID int, option string) (*time.Time, bool, error) {
	isMember, err := repository.IsConversationMember(conversationID, userID)
	if err != nil {
		return nil, false, err
	}
	if !isMember {
		return nil, false, errors.New("access denied: not a member of conversation")
	}
	return repository.SetConversationMute(conversationID, userID, option)
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

// SetDisappearingMessages updates the disappearing message timer for a conversation and creates a system notice.
func SetDisappearingMessages(actingUserID, conversationID, durationSeconds int) (*models.Conversation, error) {
	// 1. Verify membership
	isMember, err := repository.IsConversationMember(conversationID, actingUserID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, errors.New("access denied: not a participant in this conversation")
	}

	// 2. Validate duration: 0 (off), 86400 (24h), 604800 (7d), 7776000 (90d)
	validDurations := map[int]bool{
		0:       true,
		86400:   true,
		604800:  true,
		7776000: true,
	}
	if !validDurations[durationSeconds] {
		return nil, errors.New("invalid disappearing duration: must be 0, 86400 (24h), 604800 (7d), or 7776000 (90d)")
	}

	// 3. Fetch conversation
	conv, err := repository.GetConversationByID(conversationID)
	if err != nil {
		return nil, err
	}

	// 4. Role validation
	if conv.Type == "group" {
		member, err := repository.GetConversationMember(conversationID, actingUserID)
		if err != nil || member == nil {
			return nil, errors.New("access denied: member record not found")
		}
		actingUser, _ := repository.GetUserByID(actingUserID)
		isAdminOrMod := member.Role == "owner" || member.Role == "admin" || member.Role == "moderator" || actingUser.Role == "admin"
		if !isAdminOrMod {
			return nil, errors.New("permission denied: only group administrators can change disappearing message settings")
		}
	} else {
		// Direct conversation: verify not blocked
		otherUser, err := repository.GetConversationOtherParticipant(conversationID, actingUserID)
		if err == nil && otherUser != nil {
			blocked, bErr := repository.IsCommunicationBlocked(actingUserID, otherUser.ID)
			if bErr == nil && blocked {
				return nil, errors.New("cannot change settings: communication between these members is blocked")
			}
		}
	}

	// 5. Update database
	if err := repository.UpdateConversationDisappearingMessages(conversationID, durationSeconds); err != nil {
		return nil, err
	}
	conv.DisappearingDuration = durationSeconds

	// 6. Generate human-readable label
	var label string
	switch durationSeconds {
	case 86400:
		label = "24 hours"
	case 604800:
		label = "7 days"
	case 7776000:
		label = "90 days"
	default:
		label = "Off"
	}

	// 7. Post system message into conversation
	actingUser, _ := repository.GetUserByID(actingUserID)
	userName := actingUser.FullName
	if strings.TrimSpace(userName) == "" {
		userName = actingUser.Username
	}
	if strings.TrimSpace(userName) == "" {
		userName = "A member"
	}

	var noticeText string
	if durationSeconds == 0 {
		noticeText = fmt.Sprintf("%s turned off disappearing messages.", userName)
	} else {
		noticeText = fmt.Sprintf("%s set messages to disappear after %s.", userName, label)
	}

	systemMsg := &models.Message{
		ConversationID: conversationID,
		SenderID:       actingUserID,
		MessageType:    "system",
		Body:           noticeText,
	}
	_ = repository.SaveMessage(systemMsg)
	systemMsg.Sender = actingUser

	// 8. Broadcast realtime WebSocket event
	GlobalHub.BroadcastToConversation(conversationID, WSEvent{
		Type:                 "disappearing_messages_updated",
		ConversationID:       conversationID,
		DisappearingDuration: durationSeconds,
		FormattedDuration:    label,
		Message:              systemMsg,
	})

	return conv, nil
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

// GetConversationMemberSettings gets mute, archive, and pin states for a member.
func GetConversationMemberSettings(conversationID, userID int) (bool, bool, bool, error) {
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
	seen := make(map[int]bool)
	var candidates []models.User

	// First add existing friends
	friends, err := GetFriends(currentUserID)
	if err == nil && len(friends) > 0 {
		for _, f := range friends {
			if f.User.ID != currentUserID && f.User.ID != 0 && !seen[f.User.ID] {
				seen[f.User.ID] = true
				candidates = append(candidates, f.User)
			}
		}
	}

	// Also include all other registered community members
	discoverUsers, _ := repository.GetDiscoverUsers(currentUserID)
	for _, u := range discoverUsers {
		if u.ID != currentUserID && u.ID != 0 && !seen[u.ID] {
			seen[u.ID] = true
			candidates = append(candidates, u)
		}
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

// ReportAbuse records an abuse report against a member or message with evidence preservation and spam protection.
func ReportAbuse(reporterID, reportedUserID int, convID, msgID *int, reason, details string) (*models.ChatReport, error) {
	if reporterID <= 0 {
		return nil, errors.New("invalid reporter user ID")
	}

	var evidenceText string
	var evidenceURL string

	if msgID != nil && *msgID > 0 {
		msg, err := repository.GetMessageByID(*msgID)
		if err != nil || msg == nil {
			return nil, errors.New("reported message not found")
		}
		// If reportedUserID was not set accurately, ensure it points to the message author
		if reportedUserID <= 0 || (reportedUserID != msg.SenderID && msg.SenderID > 0) {
			reportedUserID = msg.SenderID
		}
		if reporterID == reportedUserID {
			return nil, errors.New("cannot report your own message")
		}
		if convID == nil || *convID <= 0 {
			convID = &msg.ConversationID
		}
		evidenceText = msg.Body
		evidenceURL = msg.AttachmentURL
	}

	if reportedUserID <= 0 {
		return nil, errors.New("invalid reported user ID")
	}
	if reporterID == reportedUserID {
		return nil, errors.New("cannot report yourself")
	}

	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("report reason is required")
	}

	// Prevent duplicate / spam reports while an existing report is pending
	hasPending, err := repository.HasPendingChatReport(reporterID, reportedUserID, msgID)
	if err != nil {
		return nil, err
	}
	if hasPending {
		return nil, errors.New("a report for this member or message is already pending moderation review")
	}

	report := &models.ChatReport{
		ReporterID:     reporterID,
		ReportedUserID: reportedUserID,
		ConversationID: convID,
		MessageID:      msgID,
		Reason:         reason,
		Details:        strings.TrimSpace(details),
		EvidenceText:   evidenceText,
		EvidenceURL:    evidenceURL,
		Status:         "pending",
	}

	if err := repository.CreateChatReport(report); err != nil {
		return nil, err
	}
	return report, nil
}

// GetChatReportsForModeration returns abuse reports if the requesting user is an admin or moderator.
func GetChatReportsForModeration(reviewerUserID int, status string, limit, offset int) ([]models.ChatReport, int, error) {
	reviewer, err := repository.GetUserByID(reviewerUserID)
	if err != nil || reviewer.ID == 0 {
		return nil, 0, errors.New("user not found")
	}
	if reviewer.Role != "admin" && reviewer.Role != "moderator" {
		return nil, 0, errors.New("access denied: only administrators and moderators can view chat reports")
	}

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	return repository.GetChatReports(status, limit, offset)
}

// ResolveChatReport updates a report with resolution decision, notes, and executes moderation actions.
func ResolveChatReport(reviewerUserID, reportID int, status, resolutionAction, notes string) error {
	reviewer, err := repository.GetUserByID(reviewerUserID)
	if err != nil || reviewer.ID == 0 {
		return errors.New("user not found")
	}
	if reviewer.Role != "admin" && reviewer.Role != "moderator" {
		return errors.New("access denied: only administrators and moderators can resolve chat reports")
	}

	report, err := repository.GetChatReportByID(reportID)
	if err != nil || report == nil {
		return errors.New("chat report not found")
	}

	status = strings.ToLower(strings.TrimSpace(status))
	if status != "resolved" && status != "dismissed" {
		status = "resolved"
	}

	resolutionAction = strings.ToLower(strings.TrimSpace(resolutionAction))
	notes = strings.TrimSpace(notes)

	// Execute moderation actions based on decision
	switch resolutionAction {
	case "delete_message":
		if report.MessageID != nil && *report.MessageID > 0 {
			_ = repository.SoftDeleteMessage(*report.MessageID, reviewerUserID)
			if report.ConversationID != nil && *report.ConversationID > 0 {
				GlobalHub.BroadcastToConversation(*report.ConversationID, WSEvent{
					Type:           "message_deleted",
					ConversationID: *report.ConversationID,
					Message: &models.Message{
						ID:             *report.MessageID,
						ConversationID: *report.ConversationID,
						IsDeleted:      true,
						Body:           "This message was removed by a moderator following an abuse report.",
					},
				})
			}
		}
	case "warn_user":
		_ = CreateNotification(models.Notification{
			SenderID:   reviewerUserID,
			ReceiverID: report.ReportedUserID,
			Type:       "system_warning",
			Message:    "You have received an official moderation warning regarding community guidelines compliance.",
		})
	}

	return repository.ResolveChatReport(reportID, reviewerUserID, status, resolutionAction, notes)
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

		conv, otherUser, err := GetConversationDetails(*convID, userID)
		if err == nil && conv != nil && conv.Type == "direct" && otherUser != nil {
			blocked, bErr := repository.IsCommunicationBlocked(userID, otherUser.ID)
			if bErr == nil && blocked {
				return []*models.Message{}, nil
			}
		}
	}

	return repository.SearchMessages(userID, query, convID, limit, offset)
}

// GetSharedMedia retrieves media and attachments for a conversation.
func GetSharedMedia(conversationID, userID int) ([]models.Message, error) {
	return GetSharedMediaCategorized(conversationID, userID, "", 50, 0)
}

// GetSharedMediaCategorized retrieves media and attachments for a conversation by category with pagination.
func GetSharedMediaCategorized(conversationID, userID int, category string, limit, offset int) ([]models.Message, error) {
	isMember, err := repository.IsConversationMember(conversationID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, errors.New("access denied: you are not a participant in this conversation")
	}

	return repository.GetConversationSharedMediaCategorized(conversationID, category, limit, offset)
}

// GetSharedMediaOverview groups attachments for a conversation into the 4 core categories.
func GetSharedMediaOverview(conversationID, userID int) (*models.SharedMediaOverview, error) {
	isMember, err := repository.IsConversationMember(conversationID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, errors.New("access denied: you are not a participant in this conversation")
	}

	items, err := repository.GetConversationSharedMediaCategorized(conversationID, "", 100, 0)
	if err != nil {
		return nil, err
	}

	overview := &models.SharedMediaOverview{
		Photos:    make([]models.Message, 0),
		Videos:    make([]models.Message, 0),
		Documents: make([]models.Message, 0),
		Voice:     make([]models.Message, 0),
		Total:     len(items),
	}

	for _, m := range items {
		if m.IsImage() {
			overview.Photos = append(overview.Photos, m)
		} else if m.IsVideo() {
			overview.Videos = append(overview.Videos, m)
		} else if m.IsVoice() {
			overview.Voice = append(overview.Voice, m)
		} else {
			overview.Documents = append(overview.Documents, m)
		}
	}
	return overview, nil
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

// JoinGroup adds an authenticated user to a public community group if join_policy is open.
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
	if conv.JoinPolicy != "open" {
		return errors.New("this group requires admin approval; please submit a membership request")
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

// RequestGroupMembership creates a membership request for a group requiring approval.
func RequestGroupMembership(userID, groupID int, message string) (*models.GroupMembershipRequest, error) {
	conv, err := repository.GetConversationByID(groupID)
	if err != nil {
		return nil, err
	}
	if conv.Type != "group" {
		return nil, errors.New("cannot request membership for a direct conversation")
	}
	if !conv.IsPublic {
		return nil, errors.New("this group is private and requires an invitation")
	}

	isMember, _ := repository.IsConversationMember(groupID, userID)
	if isMember {
		return nil, errors.New("you are already a member of this group")
	}

	existing, _ := repository.GetPendingMembershipRequest(groupID, userID)
	if existing != nil {
		return nil, errors.New("a membership request is already pending for this group")
	}

	req, err := repository.CreateGroupMembershipRequest(groupID, userID, message)
	if err != nil {
		return nil, err
	}

	// Notify group administrators
	user, _ := repository.GetUserByID(userID)
	userName := "A user"
	if user.FullName != "" {
		userName = user.FullName
	}
	admins, _ := repository.GetGroupAdministrators(groupID)
	for _, admin := range admins {
		_ = CreateNotification(models.Notification{
			SenderID:   userID,
			ReceiverID: admin.UserID,
			Type:       "group_request",
			Message:    fmt.Sprintf("%s requested to join %s.", userName, conv.Title),
		})
	}

	return req, nil
}

// ReviewGroupMembershipRequest allows an authorized group administrator to approve or reject a membership request.
func ReviewGroupMembershipRequest(actingUserID, requestID int, action string) error {
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "approve" && action != "reject" {
		return errors.New("invalid review action: must be 'approve' or 'reject'")
	}

	req, err := repository.GetGroupMembershipRequestByID(requestID)
	if err != nil || req == nil {
		return errors.New("membership request not found")
	}

	if req.Status != "pending" {
		return fmt.Errorf("this request has already been %s", req.Status)
	}

	// Permission check: acting user must be owner or admin of the group
	actingMember, err := repository.GetConversationMember(req.ConversationID, actingUserID)
	if err != nil || (actingMember.Role != "owner" && actingMember.Role != "admin") {
		return errors.New("access denied: only group administrators can review membership requests")
	}

	// Prevent self-approval
	if req.UserID == actingUserID {
		return errors.New("cannot approve or reject your own membership request")
	}

	conv, err := repository.GetConversationByID(req.ConversationID)
	if err != nil || conv == nil {
		return errors.New("group not found")
	}

	if action == "approve" {
		if err := repository.UpdateGroupMembershipRequestStatus(requestID, actingUserID, "approved"); err != nil {
			return err
		}

		if err := repository.AddConversationMember(req.ConversationID, req.UserID, "member"); err != nil {
			return err
		}

		user, _ := repository.GetUserByID(req.UserID)
		userName := "A new member"
		if user.FullName != "" {
			userName = user.FullName
		}

		sysMsg := &models.Message{
			ConversationID: req.ConversationID,
			SenderID:       actingUserID,
			MessageType:    "system",
			Body:           fmt.Sprintf("%s was approved to join the group.", userName),
		}
		_ = repository.SaveMessage(sysMsg)

		GlobalHub.BroadcastToConversation(req.ConversationID, WSEvent{
			Type:           "new_message",
			ConversationID: req.ConversationID,
			Message:        sysMsg,
		})

		_ = CreateNotification(models.Notification{
			SenderID:   actingUserID,
			ReceiverID: req.UserID,
			Type:       "group_request",
			Message:    fmt.Sprintf("Your request to join %s was approved!", conv.Title),
		})
	} else {
		if err := repository.UpdateGroupMembershipRequestStatus(requestID, actingUserID, "rejected"); err != nil {
			return err
		}

		_ = CreateNotification(models.Notification{
			SenderID:   actingUserID,
			ReceiverID: req.UserID,
			Type:       "group_request",
			Message:    fmt.Sprintf("Your request to join %s was declined.", conv.Title),
		})
	}

	return nil
}

// CancelGroupMembershipRequest cancels a pending request by the requester.
func CancelGroupMembershipRequest(actingUserID, groupID int) error {
	req, err := repository.GetPendingMembershipRequest(groupID, actingUserID)
	if err != nil || req == nil {
		return errors.New("no pending request found for this group")
	}
	return repository.CancelGroupMembershipRequest(req.ID, actingUserID)
}

// GetGroupMembershipRequests retrieves pending or all requests for a group if acting user is admin.
func GetGroupMembershipRequests(actingUserID, groupID int, status string) ([]models.GroupMembershipRequest, error) {
	actingMember, err := repository.GetConversationMember(groupID, actingUserID)
	if err != nil || (actingMember.Role != "owner" && actingMember.Role != "admin") {
		return nil, errors.New("access denied: only group administrators can view membership requests")
	}
	return repository.GetGroupMembershipRequests(groupID, status)
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

	// Disconnect from active group call room if participating
	if GlobalGroupCallManager != nil {
		_, _, _ = GlobalGroupCallManager.LeaveGroupCall(groupID, userID)
	}

	GlobalHub.BroadcastToConversation(groupID, WSEvent{
		Type:           "new_message",
		ConversationID: groupID,
		Message:        sysMsg,
	})

	GlobalHub.BroadcastToConversation(groupID, WSEvent{
		Type:           "member_left",
		ConversationID: groupID,
		UserID:         userID,
		Data: map[string]interface{}{
			"user_id":         userID,
			"conversation_id": groupID,
		},
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

// ConvertDirectToGroupConversation transitions an individual (direct) chat into a group conversation
// while preserving all existing messages, call history, and participants.
// It sets the initiator as group 'owner' and adds the selected registered members.
func ConvertDirectToGroupConversation(convID int, initiatorUserID int, newMemberIDs []int, customTitle string) (*models.Conversation, error) {
	if convID <= 0 || initiatorUserID <= 0 {
		return nil, errors.New("invalid conversation or user ID")
	}

	conv, err := repository.GetConversationByID(convID)
	if err != nil || conv == nil {
		return nil, errors.New("conversation not found")
	}

	// If already a group, delegate to AddGroupMembers
	if conv.Type == "group" {
		if err := AddGroupMembers(initiatorUserID, convID, newMemberIDs); err != nil {
			return nil, err
		}
		return repository.GetConversationByID(convID)
	}

	// Verify initiator is a participant of this conversation
	isMember, err := repository.IsConversationMember(convID, initiatorUserID)
	if err != nil || !isMember {
		return nil, errors.New("access denied: you are not a participant in this conversation")
	}

	initiator, err := repository.GetUserByID(initiatorUserID)
	if err != nil || initiator.ID <= 0 {
		return nil, errors.New("initiator user not found")
	}

	// Get other participant in the direct conversation
	otherParticipant, _ := repository.GetConversationOtherParticipant(convID, initiatorUserID)

	// Fetch current members to avoid duplicates
	currentMembers, err := repository.GetConversationMembers(convID)
	if err != nil {
		return nil, err
	}
	existingMembersMap := make(map[int]bool)
	for _, m := range currentMembers {
		existingMembersMap[m.UserID] = true
	}

	// Filter and validate candidates
	var validUserIDs []int
	var validUserNames []string
	seen := make(map[int]bool)

	for _, targetID := range newMemberIDs {
		if targetID <= 0 || targetID == initiatorUserID || existingMembersMap[targetID] || seen[targetID] {
			continue
		}
		seen[targetID] = true

		// Check communication block
		blocked, _ := repository.IsCommunicationBlocked(initiatorUserID, targetID)
		if blocked {
			continue
		}

		// Verify target user actually exists in database
		u, uErr := repository.GetUserByID(targetID)
		if uErr != nil || u.ID <= 0 {
			continue
		}

		validUserIDs = append(validUserIDs, targetID)
		validUserNames = append(validUserNames, u.FullName)
	}

	if len(validUserIDs) == 0 {
		return nil, errors.New("no valid registered members were selected to add")
	}

	// Determine group title
	finalTitle := strings.TrimSpace(customTitle)
	if finalTitle == "" {
		otherName := "Member"
		if otherParticipant != nil && otherParticipant.FullName != "" {
			otherName = otherParticipant.FullName
		}
		if len(validUserNames) == 1 {
			finalTitle = fmt.Sprintf("%s, %s & %s", initiator.FullName, otherName, validUserNames[0])
		} else {
			finalTitle = fmt.Sprintf("%s, %s + %d others", initiator.FullName, otherName, len(validUserNames))
		}
		if len(finalTitle) > 100 {
			finalTitle = finalTitle[:97] + "..."
		}
	}

	// Execute conversion in repository transaction
	if err := repository.ConvertDirectToGroup(convID, finalTitle, initiatorUserID, validUserIDs); err != nil {
		return nil, err
	}

	// Insert system message into the conversation
	sysMsg := &models.Message{
		ConversationID: convID,
		SenderID:       initiatorUserID,
		MessageType:    "system",
		Body:           fmt.Sprintf("%s converted this conversation into a group and added %s.", initiator.FullName, strings.Join(validUserNames, ", ")),
	}
	_ = repository.SaveMessage(sysMsg)

	// Broadcast updates via WebSocket
	GlobalHub.BroadcastToConversation(convID, WSEvent{
		Type:           "new_message",
		ConversationID: convID,
		Message:        sysMsg,
	})

	for _, newUID := range validUserIDs {
		GlobalHub.BroadcastToUser(newUID, WSEvent{
			Type:           "group_created",
			ConversationID: convID,
			CallerName:     initiator.FullName,
		})
	}

	return repository.GetConversationByID(convID)
}

// RemoveGroupMember removes a participant from a group (admin or owner privilege required).
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

	// Cannot remove owner
	if targetMember.Role == "owner" {
		return errors.New("cannot remove the group owner; transfer ownership first")
	}

	// If target is admin, check if they are the last administrator
	if targetMember.Role == "admin" {
		adminCount, err := repository.CountGroupAdministrators(groupID)
		if err == nil && adminCount <= 1 {
			return errors.New("cannot remove the last administrator of the group")
		}
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

	conv, _ := repository.GetConversationByID(groupID)
	groupTitle := "the group"
	if conv != nil && conv.Title != "" {
		groupTitle = conv.Title
	}

	sysMsg := &models.Message{
		ConversationID: groupID,
		SenderID:       actingUserID,
		MessageType:    "system",
		Body:           fmt.Sprintf("%s was removed from the group.", targetMember.User.FullName),
	}
	_ = repository.SaveMessage(sysMsg)

	// Disconnect target user from active call if participating
	if GlobalGroupCallManager != nil {
		_, _, _ = GlobalGroupCallManager.LeaveGroupCall(groupID, targetUserID)
	}

	GlobalHub.BroadcastToConversation(groupID, WSEvent{
		Type:           "new_message",
		ConversationID: groupID,
		Message:        sysMsg,
	})

	removalEvt := WSEvent{
		Type:           "member_removed",
		ConversationID: groupID,
		UserID:         targetUserID,
		Data: map[string]interface{}{
			"user_id":         targetUserID,
			"conversation_id": groupID,
		},
	}
	GlobalHub.BroadcastToConversation(groupID, removalEvt)
	GlobalHub.BroadcastToUser(targetUserID, removalEvt)

	_ = CreateNotification(models.Notification{
		SenderID:   actingUserID,
		ReceiverID: targetUserID,
		Type:       "group_removal",
		Message:    fmt.Sprintf("You were removed from %s by an administrator.", groupTitle),
	})

	return nil
}

// UpdateGroupMemberRole modifies a participant's role (owner or admin privileges required).
func UpdateGroupMemberRole(actingUserID, groupID, targetUserID int, newRole string) error {
	newRole = strings.ToLower(strings.TrimSpace(newRole))
	if newRole != "owner" && newRole != "admin" && newRole != "moderator" && newRole != "member" {
		return errors.New("invalid role: must be owner, admin, moderator, or member")
	}

	actingMember, err := repository.GetConversationMember(groupID, actingUserID)
	if err != nil || (actingMember.Role != "owner" && actingMember.Role != "admin") {
		return errors.New("access denied: only group administrators can change member roles")
	}

	targetMember, err := repository.GetConversationMember(groupID, targetUserID)
	if err != nil {
		return errors.New("target is not a member of this group")
	}

	if targetUserID == actingUserID {
		return errors.New("cannot change your own role directly")
	}

	conv, _ := repository.GetConversationByID(groupID)
	groupTitle := "the group"
	if conv != nil && conv.Title != "" {
		groupTitle = conv.Title
	}

	// Handling ownership transfer
	if newRole == "owner" {
		if actingMember.Role != "owner" {
			return errors.New("access denied: only the group owner can transfer ownership")
		}
		// Transfer ownership: target becomes owner, acting user becomes admin
		if err := repository.UpdateConversationMemberRole(groupID, targetUserID, "owner"); err != nil {
			return err
		}
		if err := repository.UpdateConversationMemberRole(groupID, actingUserID, "admin"); err != nil {
			return err
		}

		sysMsg := &models.Message{
			ConversationID: groupID,
			SenderID:       actingUserID,
			MessageType:    "system",
			Body:           fmt.Sprintf("%s transferred group ownership to %s.", actingMember.User.FullName, targetMember.User.FullName),
		}
		_ = repository.SaveMessage(sysMsg)

		GlobalHub.BroadcastToConversation(groupID, WSEvent{
			Type:           "new_message",
			ConversationID: groupID,
			Message:        sysMsg,
		})

		_ = CreateNotification(models.Notification{
			SenderID:   actingUserID,
			ReceiverID: targetUserID,
			Type:       "role_update",
			Message:    fmt.Sprintf("You are now the owner of %s.", groupTitle),
		})
		return nil
	}

	// If target is currently owner, cannot change their role unless transferring ownership
	if targetMember.Role == "owner" {
		return errors.New("cannot change role of the group owner; transfer ownership instead")
	}

	// Admins cannot change role of other admins (only owner can)
	if actingMember.Role != "owner" && targetMember.Role == "admin" {
		return errors.New("access denied: only the owner can modify administrator roles")
	}

	// If demoting an admin to moderator/member, verify not the last administrator
	if (targetMember.Role == "admin" || targetMember.Role == "owner") && (newRole == "moderator" || newRole == "member") {
		adminCount, err := repository.CountGroupAdministrators(groupID)
		if err == nil && adminCount <= 1 {
			return errors.New("cannot demote the last administrator of the group")
		}
	}

	if err := repository.UpdateConversationMemberRole(groupID, targetUserID, newRole); err != nil {
		return err
	}

	targetUser, err := repository.GetUserByID(targetUserID)
	targetName := targetMember.User.FullName
	if err == nil && targetUser.FullName != "" {
		targetName = targetUser.FullName
	}

	sysMsg := &models.Message{
		ConversationID: groupID,
		SenderID:       actingUserID,
		MessageType:    "system",
		Body:           fmt.Sprintf("%s is now a %s.", targetName, newRole),
	}
	_ = repository.SaveMessage(sysMsg)

	GlobalHub.BroadcastToConversation(groupID, WSEvent{
		Type:           "new_message",
		ConversationID: groupID,
		Message:        sysMsg,
	})

	_ = CreateNotification(models.Notification{
		SenderID:   actingUserID,
		ReceiverID: targetUserID,
		Type:       "role_update",
		Message:    fmt.Sprintf("Your role in %s was updated to %s.", groupTitle, newRole),
	})

	return nil
}

// UpdateGroupSettings updates group title, category, description, call permissions, and join policy.
func UpdateGroupSettings(actingUserID, groupID int, title, category, description, callPermission string, optionalJoinPolicy ...string) error {
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

	joinPolicy := "approval"
	if len(optionalJoinPolicy) > 0 && optionalJoinPolicy[0] != "" {
		if optionalJoinPolicy[0] == "open" {
			joinPolicy = "open"
		}
	} else {
		// Preserve existing join policy if not provided
		conv, cErr := repository.GetConversationByID(groupID)
		if cErr == nil && conv != nil && conv.JoinPolicy != "" {
			joinPolicy = conv.JoinPolicy
		}
	}

	return repository.UpdateGroupDetails(groupID, title, category, description, callPermission, joinPolicy)
}

// ModerateDeleteMessage soft-deletes a message if the user is the author or an authorized group moderator/admin.
func ModerateDeleteMessage(actingUserID, messageID int) error {
	msg, err := repository.GetMessageByID(messageID)
	if err != nil || msg == nil {
		return errors.New("message not found")
	}

	if msg.IsDeleted {
		return nil // already deleted
	}

	conv, err := repository.GetConversationByID(msg.ConversationID)
	if err != nil || conv == nil {
		return errors.New("conversation not found")
	}

	isAuthor := (msg.SenderID == actingUserID)
	isGroupAdminOrMod := false

	if conv.Type == "group" {
		member, mErr := repository.GetConversationMember(msg.ConversationID, actingUserID)
		if mErr == nil && (member.Role == "owner" || member.Role == "admin" || member.Role == "moderator") {
			isGroupAdminOrMod = true
		}
	}

	if !isAuthor && !isGroupAdminOrMod {
		return errors.New("access denied: insufficient permissions to delete this message")
	}

	deletionText := "This message was deleted."
	if !isAuthor && isGroupAdminOrMod {
		deletionText = "This message was removed by a moderator."
	}

	if err := repository.SoftDeleteMessage(messageID, actingUserID, deletionText); err != nil {
		return err
	}

	// Broadcast message deletion event to conversation
	GlobalHub.BroadcastToConversation(msg.ConversationID, WSEvent{
		Type:           "message_deleted",
		ConversationID: msg.ConversationID,
		Message: &models.Message{
			ID:             msg.ID,
			ConversationID: msg.ConversationID,
			IsDeleted:      true,
			Body:           deletionText,
		},
	})

	// If removed by moderator/admin and not the author, notify the author
	if !isAuthor && isGroupAdminOrMod {
		_ = CreateNotification(models.Notification{
			SenderID:   actingUserID,
			ReceiverID: msg.SenderID,
			Type:       "message_deleted",
			Message:    fmt.Sprintf("Your message in %s was removed by a moderator.", conv.Title),
		})
	}

	return nil
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

// GenerateCallInvitation creates an unpredictable, cryptographically random call invitation token valid for 24 hours.
func GenerateCallInvitation(conversationID, userID int, callType string) (*models.CallInvitation, string, error) {
	if conversationID <= 0 || userID <= 0 {
		return nil, "", errors.New("invalid conversation or user ID")
	}

	// Verify conversation exists and user is a member
	isMember, err := repository.IsConversationMember(conversationID, userID)
	if err != nil || !isMember {
		return nil, "", errors.New("access denied: you are not a participant in this conversation")
	}

	conv, err := repository.GetConversationByID(conversationID)
	if err != nil || conv == nil {
		return nil, "", errors.New("conversation not found")
	}

	// In groups, verify call permissions if restricted
	if conv.Type == "group" {
		canCall, _ := CanUserCallInGroup(conversationID, userID)
		if !canCall {
			return nil, "", errors.New("access denied: only group administrators can initiate calls")
		}
	} else if conv.Type == "direct" {
		otherUser, err := repository.GetConversationOtherParticipant(conversationID, userID)
		if err == nil && otherUser != nil {
			blocked, _ := repository.IsCommunicationBlocked(userID, otherUser.ID)
			if blocked {
				return nil, "", errors.New("cannot create call link: communication is blocked between participants")
			}
		}
	}

	callType = strings.ToLower(strings.TrimSpace(callType))
	if callType != "video" {
		callType = "audio"
	}

	randBytes := make([]byte, 16)
	if _, err := rand.Read(randBytes); err != nil {
		return nil, "", fmt.Errorf("failed generating invitation token: %w", err)
	}
	token := hex.EncodeToString(randBytes)

	inv := &models.CallInvitation{
		ConversationID: conversationID,
		Token:          token,
		CallType:       callType,
		CreatedBy:      userID,
		ExpiresAt:      time.Now().Add(24 * time.Hour),
	}

	if err := repository.CreateCallInvitation(inv); err != nil {
		return nil, "", err
	}

	inviteURL := fmt.Sprintf("/messages/call/join?token=%s", token)
	return inv, inviteURL, nil
}

// ValidateCallInvitation validates a call invitation token, checking expiration and membership access.
func ValidateCallInvitation(token string, currentUserID int) (*models.CallInvitation, *models.Conversation, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, nil, errors.New("invalid or empty call invitation token")
	}

	inv, err := repository.GetCallInvitationByToken(token)
	if err != nil || inv == nil {
		return nil, nil, errors.New("call invitation not found or invalid")
	}

	if time.Now().After(inv.ExpiresAt) {
		return nil, nil, errors.New("call invitation has expired")
	}

	conv, err := repository.GetConversationByID(inv.ConversationID)
	if err != nil || conv == nil {
		return nil, nil, errors.New("conversation associated with this invitation was not found")
	}

	// Validate authorization for current user
	if conv.Type == "direct" {
		isMember, err := repository.IsConversationMember(conv.ID, currentUserID)
		if err != nil || !isMember {
			return nil, nil, errors.New("access denied: this call link is private to the participants of this conversation")
		}
		// Check block
		otherUser, _ := repository.GetConversationOtherParticipant(conv.ID, currentUserID)
		if otherUser != nil {
			blocked, _ := repository.IsCommunicationBlocked(currentUserID, otherUser.ID)
			if blocked {
				return nil, nil, errors.New("cannot join call: communication is blocked")
			}
		}
	} else if conv.Type == "group" {
		isMember, err := repository.IsConversationMember(conv.ID, currentUserID)
		if err != nil || !isMember {
			// If not a member, check if open group to auto-join
			if conv.JoinPolicy == "open" {
				_ = repository.AddConversationMember(conv.ID, currentUserID, "member")
			} else {
				return nil, nil, errors.New("access denied: you must be an approved member of this group to join this call")
			}
		}
	}

	return inv, conv, nil
}

func extractMentions(text string) []string {
	re := regexp.MustCompile(`@([a-zA-Z0-9_]+)`)
	matches := re.FindAllStringSubmatch(text, -1)
	var usernames []string
	seen := make(map[string]bool)
	for _, m := range matches {
		if len(m) > 1 {
			u := strings.ToLower(m[1])
			if !seen[u] {
				seen[u] = true
				usernames = append(usernames, u)
			}
		}
	}
	return usernames
}

func processMentions(conversationID, senderID int, senderName string, conv *models.Conversation, body string) {
	usernames := extractMentions(body)
	if len(usernames) == 0 {
		return
	}

	members, err := repository.GetConversationMembers(conversationID)
	if err != nil {
		return
	}

	convTitle := "the chat"
	if conv != nil && conv.Title != "" {
		convTitle = conv.Title
	}

	for _, member := range members {
		if member.UserID == senderID {
			continue
		}
		memUsername := strings.ToLower(member.User.Username)
		for _, u := range usernames {
			if u == memUsername {
				// Verify not blocked
				if blocked, _ := repository.IsCommunicationBlocked(senderID, member.UserID); blocked {
					break
				}
				// Verify not muted
				if repository.IsConversationMutedBy(conversationID, member.UserID) {
					break
				}

				_ = CreateNotification(models.Notification{
					SenderID:   senderID,
					ReceiverID: member.UserID,
					Type:       "mention",
					Message:    fmt.Sprintf("%s mentioned you in %s", senderName, convTitle),
				})
				break
			}
		}
	}
}

// ClearChat clears message history for a user in a conversation.
func ClearChat(conversationID, userID int) error {
	return repository.ClearConversationHistory(conversationID, userID)
}

// TogglePinConversation toggles the pinned state of a conversation for a user.
func TogglePinConversation(conversationID, userID int) (bool, error) {
	return repository.ToggleConversationPin(conversationID, userID)
}

// DeleteMessageForMe hides a message specifically for the requesting user.
func DeleteMessageForMe(userID, messageID int) error {
	return repository.DeleteMessageForMe(messageID, userID)
}

func truncateText(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ExportChat generates an authorized transcript in "txt" or "json" format.
func ExportChat(conversationID, userID int, format string) (string, []byte, error) {
	conv, messages, err := repository.GetConversationMessagesForExport(conversationID, userID)
	if err != nil {
		return "", nil, err
	}

	timestamp := time.Now().Format("20060102150405")
	format = strings.ToLower(strings.TrimSpace(format))

	convTitle := conv.Title
	if convTitle == "" {
		if conv.Type == "direct" {
			convTitle = "Direct Chat"
		} else {
			convTitle = fmt.Sprintf("Conversation #%d", conversationID)
		}
	}

	if format == "json" {
		type ExportedSender struct {
			ID       int    `json:"id"`
			FullName string `json:"full_name"`
			Username string `json:"username"`
		}
		type ExportedMessage struct {
			ID               int            `json:"id"`
			Sender           ExportedSender `json:"sender"`
			MessageType      string         `json:"message_type"`
			Body             string         `json:"body"`
			ReplyToMessageID *int           `json:"reply_to_message_id,omitempty"`
			AttachmentURL    string         `json:"attachment_url,omitempty"`
			AttachmentName   string         `json:"attachment_name,omitempty"`
			AttachmentSize   int64          `json:"attachment_size,omitempty"`
			CreatedAt        string         `json:"created_at"`
		}
		type ExportPayload struct {
			ConversationID int               `json:"conversation_id"`
			Title          string            `json:"title"`
			Type           string            `json:"type"`
			ExportedAt     string            `json:"exported_at"`
			TotalMessages  int               `json:"total_messages"`
			Messages       []ExportedMessage `json:"messages"`
		}

		exportMsgs := make([]ExportedMessage, len(messages))
		for i, m := range messages {
			exportMsgs[i] = ExportedMessage{
				ID: m.ID,
				Sender: ExportedSender{
					ID:       m.Sender.ID,
					FullName: m.Sender.FullName,
					Username: m.Sender.Username,
				},
				MessageType:      m.MessageType,
				Body:             m.Body,
				ReplyToMessageID: m.ReplyToMessageID,
				AttachmentURL:    m.AttachmentURL,
				AttachmentName:   m.AttachmentName,
				AttachmentSize:   m.AttachmentSize,
				CreatedAt:        m.CreatedAt.Format(time.RFC3339),
			}
		}

		payload := ExportPayload{
			ConversationID: conversationID,
			Title:          convTitle,
			Type:           conv.Type,
			ExportedAt:     time.Now().Format(time.RFC3339),
			TotalMessages:  len(messages),
			Messages:       exportMsgs,
		}

		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return "", nil, err
		}
		filename := fmt.Sprintf("chat-export-%d-%s.json", conversationID, timestamp)
		return filename, data, nil
	}

	// Default to plain text format
	var buf strings.Builder
	buf.WriteString("================================================================================\n")
	buf.WriteString(fmt.Sprintf("Chat Transcript: %s\n", convTitle))
	buf.WriteString(fmt.Sprintf("Conversation ID: %d\n", conversationID))
	buf.WriteString(fmt.Sprintf("Export Date: %s\n", time.Now().Format("2006-01-02 15:04:05 MST")))
	buf.WriteString(fmt.Sprintf("Total Messages: %d\n", len(messages)))
	buf.WriteString("================================================================================\n\n")

	for _, m := range messages {
		timeStr := m.CreatedAt.Format("2006-01-02 15:04:05")
		replyInfo := ""
		if m.ReplyToMessage != nil {
			replyInfo = fmt.Sprintf(" (replying to %s: \"%s\")", m.ReplyToMessage.Sender.FullName, truncateText(m.ReplyToMessage.Body, 30))
		}
		attachInfo := ""
		if m.AttachmentURL != "" {
			attachInfo = fmt.Sprintf(" [Attachment: %s (%s)]", m.AttachmentName, m.AttachmentURL)
		}
		buf.WriteString(fmt.Sprintf("[%s] %s%s: %s%s\n", timeStr, m.Sender.FullName, replyInfo, m.Body, attachInfo))
	}

	filename := fmt.Sprintf("chat-export-%d-%s.txt", conversationID, timestamp)
	return filename, []byte(buf.String()), nil
}
