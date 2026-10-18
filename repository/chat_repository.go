package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"convention-management-system/database"
	"convention-management-system/helpers"
	"convention-management-system/models"
	"convention-management-system/utils"
)

func parseSQLTime(val any) time.Time {
	if val == nil {
		return time.Time{}
	}
	switch v := val.(type) {
	case time.Time:
		return v
	case string:
		layouts := []string{
			"2006-01-02 15:04:05",
			"2006-01-02 15:04:05.999999999",
			"2006-01-02T15:04:05Z07:00",
			time.RFC3339,
			"2006-01-02",
		}
		for _, l := range layouts {
			if t, err := time.Parse(l, v); err == nil {
				return t
			}
		}
	case []byte:
		return parseSQLTime(string(v))
	}
	return time.Time{}
}

// CreateOrGetDirectConversation finds an existing 1-to-1 conversation between user1 and user2,
// or creates one inside a database transaction to prevent race conditions.
func CreateOrGetDirectConversation(user1ID, user2ID int) (*models.Conversation, error) {
	if user1ID == user2ID {
		return nil, errors.New("cannot create conversation with yourself")
	}

	// 1. Check if a direct conversation already exists between both users
	findQuery := fmt.Sprintf(`
		SELECT c.id, c.type, COALESCE(c.title, ''), c.created_by, c.last_message_at, c.created_at, c.updated_at
		FROM conversations c
		JOIN conversation_members cm1 ON c.id = cm1.conversation_id AND cm1.user_id = %s
		JOIN conversation_members cm2 ON c.id = cm2.conversation_id AND cm2.user_id = %s
		WHERE c.type = 'direct'
		LIMIT 1
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	var conv models.Conversation
	var lastMsg, createdAtRaw, updatedAtRaw any
	var title string

	err := database.DB.QueryRow(findQuery, user1ID, user2ID).Scan(
		&conv.ID,
		&conv.Type,
		&title,
		&conv.CreatedBy,
		&lastMsg,
		&createdAtRaw,
		&updatedAtRaw,
	)

	if err == nil {
		conv.Title = title
		conv.CreatedAt = parseSQLTime(createdAtRaw)
		conv.UpdatedAt = parseSQLTime(updatedAtRaw)
		if t := parseSQLTime(lastMsg); !t.IsZero() {
			conv.LastMessageAt = &t
		}
		return &conv, nil
	} else if err != sql.ErrNoRows {
		return nil, err
	}

	// 2. Create new conversation in a transaction
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	insertConvQuery := fmt.Sprintf(`
		INSERT INTO conversations (type, title, created_by, last_message_at, created_at, updated_at)
		VALUES ('direct', '', %s, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, helpers.Placeholder(1))

	var convID int64
	// In SQLite / Postgres: Exec and LastInsertId or RETURNING
	result, err := tx.Exec(insertConvQuery, user1ID)
	if err != nil {
		return nil, err
	}
	convID, err = result.LastInsertId()
	if err != nil || convID == 0 {
		// Fallback for drivers that don't support LastInsertId
		err = tx.QueryRow(fmt.Sprintf("SELECT id FROM conversations WHERE created_by = %s ORDER BY id DESC LIMIT 1", helpers.Placeholder(1)), user1ID).Scan(&convID)
		if err != nil {
			return nil, err
		}
	}

	// Insert member 1
	insertMemberQuery := fmt.Sprintf(`
		INSERT INTO conversation_members (conversation_id, user_id, role, joined_at)
		VALUES (%s, %s, 'member', CURRENT_TIMESTAMP)
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	if _, err := tx.Exec(insertMemberQuery, convID, user1ID); err != nil {
		return nil, err
	}

	// Insert member 2
	if _, err := tx.Exec(insertMemberQuery, convID, user2ID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	now := time.Now()
	return &models.Conversation{
		ID:            int(convID),
		Type:          "direct",
		CreatedBy:     user1ID,
		LastMessageAt: &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// IsConversationMember verifies whether a given user is a member of the conversation.
func IsConversationMember(conversationID, userID int) (bool, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM conversation_members
		WHERE conversation_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	var count int
	err := database.DB.QueryRow(query, conversationID, userID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetConversationByID retrieves a conversation record by ID.
func GetConversationByID(conversationID int) (*models.Conversation, error) {
	query := fmt.Sprintf(`
		SELECT id, type, COALESCE(title, ''), COALESCE(category, 'general'),
		       COALESCE(description, ''), COALESCE(avatar_url, ''),
		       COALESCE(call_permission, 'all'), COALESCE(is_public, TRUE),
		       COALESCE(join_policy, 'approval'),
		       created_by, last_message_at, created_at, updated_at
		FROM conversations
		WHERE id = %s
	`, helpers.Placeholder(1))

	var conv models.Conversation
	var lastMsg, createdAtRaw, updatedAtRaw any
	var title, category, description, avatarURL, callPermission, joinPolicy string
	var isPublic bool

	err := database.DB.QueryRow(query, conversationID).Scan(
		&conv.ID,
		&conv.Type,
		&title,
		&category,
		&description,
		&avatarURL,
		&callPermission,
		&isPublic,
		&joinPolicy,
		&conv.CreatedBy,
		&lastMsg,
		&createdAtRaw,
		&updatedAtRaw,
	)
	if err != nil {
		return nil, err
	}

	conv.Title = title
	conv.Category = category
	conv.Description = description
	conv.AvatarURL = avatarURL
	conv.CallPermission = callPermission
	conv.IsPublic = isPublic
	conv.JoinPolicy = joinPolicy
	conv.CreatedAt = parseSQLTime(createdAtRaw)
	conv.UpdatedAt = parseSQLTime(updatedAtRaw)
	if t := parseSQLTime(lastMsg); !t.IsZero() {
		conv.LastMessageAt = &t
	}
	return &conv, nil
}

// GetConversationOtherParticipant finds the other member in a direct conversation.
func GetConversationOtherParticipant(conversationID, currentUserID int) (*models.User, error) {
	query := fmt.Sprintf(`
		SELECT u.id, u.full_name, u.username, COALESCE(u.profile_photo, ''), COALESCE(u.role, 'member')
		FROM conversation_members cm
		JOIN users u ON cm.user_id = u.id
		WHERE cm.conversation_id = %s AND cm.user_id != %s
		LIMIT 1
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	var user models.User
	var photo string

	err := database.DB.QueryRow(query, conversationID, currentUserID).Scan(
		&user.ID,
		&user.FullName,
		&user.Username,
		&photo,
		&user.Role,
	)
	if err != nil {
		return nil, err
	}

	user.ProfilePhoto = photo
	user.Initials = utils.Initials(user.FullName)
	return &user, nil
}

// SaveMessage persists a chat message with idempotency checks.
func SaveMessage(msg *models.Message) error {
	msg.Body = strings.TrimSpace(msg.Body)
	if msg.Body == "" {
		return errors.New("message content cannot be empty")
	}

	// 1. Idempotency check: if client_message_id is provided, check if already saved
	if msg.ClientMessageID != "" {
		checkQuery := fmt.Sprintf(`
			SELECT id, created_at FROM messages
			WHERE conversation_id = %s AND client_message_id = %s
			LIMIT 1
		`, helpers.Placeholder(1), helpers.Placeholder(2))

		var existingID int
		var createdAt time.Time
		err := database.DB.QueryRow(checkQuery, msg.ConversationID, msg.ClientMessageID).Scan(&existingID, &createdAt)
		if err == nil {
			msg.ID = existingID
			msg.CreatedAt = createdAt
			return nil
		}
	}

	// 2. Insert message
	insertQuery := fmt.Sprintf(`
		INSERT INTO messages (
			conversation_id, sender_id, message_type, body, client_message_id, reply_to_message_id,
			attachment_url, attachment_name, attachment_size, attachment_type,
			is_edited, is_deleted, created_at, updated_at
		)
		VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, FALSE, FALSE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`,
		helpers.Placeholder(1),
		helpers.Placeholder(2),
		helpers.Placeholder(3),
		helpers.Placeholder(4),
		helpers.Placeholder(5),
		helpers.Placeholder(6),
		helpers.Placeholder(7),
		helpers.Placeholder(8),
		helpers.Placeholder(9),
		helpers.Placeholder(10),
	)

	result, err := database.DB.Exec(
		insertQuery,
		msg.ConversationID,
		msg.SenderID,
		msg.MessageType,
		msg.Body,
		msg.ClientMessageID,
		msg.ReplyToMessageID,
		msg.AttachmentURL,
		msg.AttachmentName,
		msg.AttachmentSize,
		msg.AttachmentType,
	)
	if err != nil {
		return err
	}

	msgID, err := result.LastInsertId()
	if err != nil || msgID == 0 {
		database.DB.QueryRow(fmt.Sprintf("SELECT id FROM messages WHERE conversation_id = %s ORDER BY id DESC LIMIT 1", helpers.Placeholder(1)), msg.ConversationID).Scan(&msgID)
	}
	msg.ID = int(msgID)
	msg.CreatedAt = time.Now()

	// 3. Update conversation's last_message_at
	updateConvQuery := fmt.Sprintf(`
		UPDATE conversations
		SET last_message_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = %s
	`, helpers.Placeholder(1))
	database.DB.Exec(updateConvQuery, msg.ConversationID)

	// 4. Update sender's read marker
	updateMemberQuery := fmt.Sprintf(`
		UPDATE conversation_members
		SET last_read_message_id = %s, last_read_at = CURRENT_TIMESTAMP
		WHERE conversation_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3))
	database.DB.Exec(updateMemberQuery, msg.ID, msg.ConversationID, msg.SenderID)

	return nil
}

// GetConversationMessages returns ordered messages for a conversation with sender details and read status.
func GetConversationMessages(conversationID int, limit, offset int, currentUserID int) ([]models.Message, error) {
	if limit <= 0 {
		limit = 50
	}

	// 1. Get other participant's last_read_message_id for read receipts
	var otherLastReadID int
	queryOther := fmt.Sprintf(`
		SELECT COALESCE(last_read_message_id, 0)
		FROM conversation_members
		WHERE conversation_id = %s AND user_id != %s
		LIMIT 1
	`, helpers.Placeholder(1), helpers.Placeholder(2))
	_ = database.DB.QueryRow(queryOther, conversationID, currentUserID).Scan(&otherLastReadID)

	query := fmt.Sprintf(`
		SELECT 
			m.id, m.conversation_id, m.sender_id, m.message_type, m.body,
			COALESCE(m.client_message_id, ''), m.reply_to_message_id,
			m.is_edited, m.is_deleted,
			COALESCE(m.attachment_url, ''), COALESCE(m.attachment_name, ''),
			COALESCE(m.attachment_size, 0), COALESCE(m.attachment_type, ''),
			m.created_at, m.updated_at,
			u.id, u.full_name, u.username, COALESCE(u.profile_photo, '')
		FROM messages m
		JOIN users u ON m.sender_id = u.id
		WHERE m.conversation_id = %s
		ORDER BY m.created_at ASC, m.id ASC
		LIMIT %s OFFSET %s
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3))

	rows, err := database.DB.Query(query, conversationID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		var replyTo sql.NullInt64
		var photo string

		var createdAtRaw, updatedAtRaw any
		err := rows.Scan(
			&m.ID,
			&m.ConversationID,
			&m.SenderID,
			&m.MessageType,
			&m.Body,
			&m.ClientMessageID,
			&replyTo,
			&m.IsEdited,
			&m.IsDeleted,
			&m.AttachmentURL,
			&m.AttachmentName,
			&m.AttachmentSize,
			&m.AttachmentType,
			&createdAtRaw,
			&updatedAtRaw,
			&m.Sender.ID,
			&m.Sender.FullName,
			&m.Sender.Username,
			&photo,
		)
		if err != nil {
			return nil, err
		}

		m.CreatedAt = parseSQLTime(createdAtRaw)
		m.UpdatedAt = parseSQLTime(updatedAtRaw)

		// Compute read receipt
		if m.SenderID == currentUserID && otherLastReadID >= m.ID {
			m.IsReadByRecipient = true
		}

		if replyTo.Valid {
			repID := int(replyTo.Int64)
			m.ReplyToMessageID = &repID
		}

		m.Sender.ProfilePhoto = photo
		m.Sender.Initials = utils.Initials(m.Sender.FullName)
		messages = append(messages, m)
	}

	return messages, nil
}

// GetUserConversations returns all active conversations for a user with unread counts, mute, and archive flags.
func GetUserConversations(userID int) ([]models.ConversationItem, error) {
	query := fmt.Sprintf(`
		SELECT 
			c.id,
			c.type,
			COALESCE(c.title, ''),
			COALESCE(c.category, 'general'),
			COALESCE(c.description, ''),
			COALESCE(c.avatar_url, ''),
			COALESCE(c.call_permission, 'all'),
			COALESCE(c.is_public, TRUE),
			(SELECT COUNT(*) FROM conversation_members WHERE conversation_id = c.id) as member_count,
			COALESCE(c.last_message_at, c.created_at) as sort_time,
			cm.last_read_message_id,
			COALESCE(cm.is_muted, FALSE),
			COALESCE(cm.is_archived, FALSE),
			-- Other user details for direct chat
			COALESCE(u.id, 0),
			COALESCE(u.full_name, ''),
			COALESCE(u.username, ''),
			COALESCE(u.profile_photo, ''),
			-- Latest message
			COALESCE(latest.id, 0),
			COALESCE(latest.body, ''),
			COALESCE(latest.sender_id, 0),
			COALESCE(latest.created_at, c.created_at)
		FROM conversation_members cm
		JOIN conversations c ON cm.conversation_id = c.id
		-- Find the other participant in direct conversations ONLY
		LEFT JOIN conversation_members other_cm 
			ON other_cm.conversation_id = c.id AND other_cm.user_id != %s AND c.type = 'direct'
		LEFT JOIN users u ON other_cm.user_id = u.id AND c.type = 'direct'
		-- Subquery for the most recent message in the conversation
		LEFT JOIN (
			SELECT m1.conversation_id, m1.id, m1.body, m1.sender_id, m1.created_at
			FROM messages m1
			INNER JOIN (
				SELECT conversation_id, MAX(id) as max_id
				FROM messages
				GROUP BY conversation_id
			) m2 ON m1.conversation_id = m2.conversation_id AND m1.id = m2.max_id
		) latest ON latest.conversation_id = c.id
		WHERE cm.user_id = %s
		ORDER BY sort_time DESC
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	rows, err := database.DB.Query(query, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.ConversationItem
	for rows.Next() {
		var item models.ConversationItem
		var lastReadID int
		var isMuted, isArchived bool
		var isPublic bool
		var sortTimeRaw any
		var otherID int
		var otherName, otherUsername, otherPhoto string
		var latestMsgID, latestSenderID int
		var latestBody string
		var latestTimeRaw any

		err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.Title,
			&item.Category,
			&item.Description,
			&item.AvatarURL,
			&item.CallPermission,
			&isPublic,
			&item.MemberCount,
			&sortTimeRaw,
			&lastReadID,
			&isMuted,
			&isArchived,
			&otherID,
			&otherName,
			&otherUsername,
			&otherPhoto,
			&latestMsgID,
			&latestBody,
			&latestSenderID,
			&latestTimeRaw,
		)
		if err != nil {
			return nil, err
		}

		item.IsPublic = isPublic
		item.IsMuted = isMuted
		item.IsArchived = isArchived

		if item.Type == "direct" && otherID != 0 {
			item.OtherUser = models.User{
				ID:           otherID,
				FullName:     otherName,
				Username:     otherUsername,
				ProfilePhoto: otherPhoto,
				Initials:     utils.Initials(otherName),
			}
		}

		item.LastMessage = latestBody
		item.LastMessageSenderID = latestSenderID
		item.LastMessageTime = parseSQLTime(latestTimeRaw)

		// Calculate unread count
		if latestMsgID > lastReadID {
			unreadQuery := fmt.Sprintf(`
				SELECT COUNT(*) FROM messages
				WHERE conversation_id = %s AND id > %s AND sender_id != %s
			`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3))
			var unread int
			database.DB.QueryRow(unreadQuery, item.ID, lastReadID, userID).Scan(&unread)
			item.UnreadCount = unread
		}

		items = append(items, item)
	}

	return items, nil
}

// MarkConversationAsRead advances the user's read marker to the latest message.
func MarkConversationAsRead(conversationID, userID int) error {
	var maxID int
	queryMax := fmt.Sprintf(`SELECT COALESCE(MAX(id), 0) FROM messages WHERE conversation_id = %s`, helpers.Placeholder(1))
	err := database.DB.QueryRow(queryMax, conversationID).Scan(&maxID)
	if err != nil {
		return err
	}

	updateQuery := fmt.Sprintf(`
		UPDATE conversation_members
		SET last_read_message_id = %s, last_read_at = CURRENT_TIMESTAMP
		WHERE conversation_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3))

	_, err = database.DB.Exec(updateQuery, maxID, conversationID, userID)
	return err
}

// CountTotalUnreadMessages counts all unread messages for a user across all conversations.
func CountTotalUnreadMessages(userID int) (int, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(m.id)
		FROM messages m
		JOIN conversation_members cm ON m.conversation_id = cm.conversation_id
		WHERE cm.user_id = %s AND m.sender_id != %s AND m.id > cm.last_read_message_id
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	var count int
	err := database.DB.QueryRow(query, userID, userID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ToggleMuteConversation toggles the is_muted state for a user in a conversation.
func ToggleMuteConversation(convID, userID int) (bool, error) {
	var currentMuted bool
	querySelect := fmt.Sprintf(`
		SELECT COALESCE(is_muted, FALSE)
		FROM conversation_members
		WHERE conversation_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))
	err := database.DB.QueryRow(querySelect, convID, userID).Scan(&currentMuted)
	if err != nil {
		return false, err
	}

	newMuted := !currentMuted

	queryUpdate := fmt.Sprintf(`
		UPDATE conversation_members
		SET is_muted = %s
		WHERE conversation_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3))
	_, err = database.DB.Exec(queryUpdate, newMuted, convID, userID)
	if err != nil {
		return false, err
	}

	return newMuted, nil
}

// ToggleArchiveConversation toggles the is_archived state for a user in a conversation.
func ToggleArchiveConversation(convID, userID int) (bool, error) {
	var currentArchived bool
	querySelect := fmt.Sprintf(`
		SELECT COALESCE(is_archived, FALSE)
		FROM conversation_members
		WHERE conversation_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))
	err := database.DB.QueryRow(querySelect, convID, userID).Scan(&currentArchived)
	if err != nil {
		return false, err
	}

	newArchived := !currentArchived

	queryUpdate := fmt.Sprintf(`
		UPDATE conversation_members
		SET is_archived = %s
		WHERE conversation_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3))
	_, err = database.DB.Exec(queryUpdate, newArchived, convID, userID)
	if err != nil {
		return false, err
	}

	return newArchived, nil
}

// GetConversationMemberSettings gets the mute and archive status for a member.
func GetConversationMemberSettings(convID, userID int) (isMuted, isArchived bool, err error) {
	query := fmt.Sprintf(`
		SELECT COALESCE(is_muted, FALSE), COALESCE(is_archived, FALSE)
		FROM conversation_members
		WHERE conversation_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))
	err = database.DB.QueryRow(query, convID, userID).Scan(&isMuted, &isArchived)
	if err != nil {
		return false, false, err
	}
	return isMuted, isArchived, nil
}

// IsConversationMutedBy checks if a conversation is muted by a specific user.
func IsConversationMutedBy(convID, userID int) bool {
	var muted bool
	query := fmt.Sprintf(`
		SELECT COALESCE(is_muted, FALSE)
		FROM conversation_members
		WHERE conversation_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))
	err := database.DB.QueryRow(query, convID, userID).Scan(&muted)
	if err != nil {
		return false
	}
	return muted
}

// CountMessagesInConversation returns the total number of messages in a conversation.
func CountMessagesInConversation(convID int) (int, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM messages WHERE conversation_id = %s`, helpers.Placeholder(1))
	var count int
	err := database.DB.QueryRow(query, convID).Scan(&count)
	return count, err
}

// MarkMessageNotificationsAsRead marks notifications of type 'message' from a sender as read.
func MarkMessageNotificationsAsRead(receiverID, senderID int) error {
	query := fmt.Sprintf(`
		UPDATE notifications
		SET is_read = TRUE
		WHERE receiver_id = %s AND sender_id = %s AND type = 'message' AND is_read = FALSE
	`, helpers.Placeholder(1), helpers.Placeholder(2))
	_, err := database.DB.Exec(query, receiverID, senderID)
	return err
}

// BlockUser records that blockerID has blocked blockedID.
func BlockUser(blockerID, blockedID int, reason string) error {
	if blockerID == blockedID {
		return errors.New("cannot block yourself")
	}

	isBlocked, err := IsUserBlocked(blockerID, blockedID)
	if err != nil {
		return err
	}
	if isBlocked {
		return nil
	}

	query := fmt.Sprintf(`
		INSERT INTO blocked_users (blocker_id, blocked_id, reason, created_at)
		VALUES (%s, %s, %s, CURRENT_TIMESTAMP)
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3))
	_, err = database.DB.Exec(query, blockerID, blockedID, reason)
	return err
}

// UnblockUser removes a block between blockerID and blockedID.
func UnblockUser(blockerID, blockedID int) error {
	query := fmt.Sprintf(`DELETE FROM blocked_users WHERE blocker_id = %s AND blocked_id = %s`, helpers.Placeholder(1), helpers.Placeholder(2))
	_, err := database.DB.Exec(query, blockerID, blockedID)
	return err
}

// IsUserBlocked checks if blockerID has blocked blockedID.
func IsUserBlocked(blockerID, blockedID int) (bool, error) {
	query := fmt.Sprintf(`SELECT COUNT(1) FROM blocked_users WHERE blocker_id = %s AND blocked_id = %s`, helpers.Placeholder(1), helpers.Placeholder(2))
	var count int
	err := database.DB.QueryRow(query, blockerID, blockedID).Scan(&count)
	return count > 0, err
}

// IsCommunicationBlocked checks if user1 has blocked user2 OR user2 has blocked user1.
func IsCommunicationBlocked(user1ID, user2ID int) (bool, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(1) FROM blocked_users 
		WHERE (blocker_id = %s AND blocked_id = %s) OR (blocker_id = %s AND blocked_id = %s)
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4))
	var count int
	err := database.DB.QueryRow(query, user1ID, user2ID, user2ID, user1ID).Scan(&count)
	return count > 0, err
}

// GetBlockedUsers returns the list of users blocked by blockerID.
func GetBlockedUsers(blockerID int) ([]models.User, error) {
	query := fmt.Sprintf(`
		SELECT u.id, u.full_name, u.username, COALESCE(u.profile_photo, ''), COALESCE(u.church_name, '')
		FROM users u
		JOIN blocked_users b ON u.id = b.blocked_id
		WHERE b.blocker_id = %s
		ORDER BY b.created_at DESC
	`, helpers.Placeholder(1))
	rows, err := database.DB.Query(query, blockerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.FullName, &u.Username, &u.ProfilePhoto, &u.ChurchName); err == nil {
			u.Initials = utils.Initials(u.FullName)
			users = append(users, u)
		}
	}
	return users, nil
}

// CreateChatReport persists an abuse report against a user or message.
func CreateChatReport(report *models.ChatReport) error {
	query := fmt.Sprintf(`
		INSERT INTO chat_reports (reporter_id, reported_user_id, conversation_id, message_id, reason, details, status, created_at)
		VALUES (%s, %s, %s, %s, %s, %s, 'pending', CURRENT_TIMESTAMP)
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4), helpers.Placeholder(5), helpers.Placeholder(6))
	res, err := database.DB.Exec(query, report.ReporterID, report.ReportedUserID, report.ConversationID, report.MessageID, report.Reason, report.Details)
	if err != nil {
		return err
	}
	if id, err := res.LastInsertId(); err == nil && id > 0 {
		report.ID = int(id)
	}
	return nil
}

// SearchMessages searches messages containing query across user's conversations or a specific conversation.
func SearchMessages(userID int, query string, conversationID *int, limit, offset int) ([]*models.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}

	likePattern := "%" + query + "%"
	var sqlStr string
	var args []any

	if conversationID != nil && *conversationID > 0 {
		sqlStr = fmt.Sprintf(`
			SELECT m.id, m.conversation_id, m.sender_id, m.message_type, m.body,
			       COALESCE(m.attachment_url, ''), COALESCE(m.attachment_name, ''),
			       COALESCE(m.attachment_size, 0), COALESCE(m.attachment_type, ''),
			       m.created_at, u.full_name, u.username, COALESCE(u.profile_photo, '')
			FROM messages m
			JOIN conversation_members cm ON m.conversation_id = cm.conversation_id AND cm.user_id = %s
			JOIN users u ON m.sender_id = u.id
			WHERE m.conversation_id = %s
			  AND (m.body LIKE %s OR m.attachment_name LIKE %s)
			  AND m.is_deleted = FALSE
			ORDER BY m.created_at DESC
			LIMIT %s OFFSET %s
		`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4), helpers.Placeholder(5), helpers.Placeholder(6))
		args = []any{userID, *conversationID, likePattern, likePattern, limit, offset}
	} else {
		sqlStr = fmt.Sprintf(`
			SELECT m.id, m.conversation_id, m.sender_id, m.message_type, m.body,
			       COALESCE(m.attachment_url, ''), COALESCE(m.attachment_name, ''),
			       COALESCE(m.attachment_size, 0), COALESCE(m.attachment_type, ''),
			       m.created_at, u.full_name, u.username, COALESCE(u.profile_photo, '')
			FROM messages m
			JOIN conversation_members cm ON m.conversation_id = cm.conversation_id AND cm.user_id = %s
			JOIN users u ON m.sender_id = u.id
			WHERE (m.body LIKE %s OR m.attachment_name LIKE %s)
			  AND m.is_deleted = FALSE
			ORDER BY m.created_at DESC
			LIMIT %s OFFSET %s
		`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4), helpers.Placeholder(5))
		args = []any{userID, likePattern, likePattern, limit, offset}
	}

	rows, err := database.DB.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		var m models.Message
		var createdAtRaw any
		err := rows.Scan(
			&m.ID, &m.ConversationID, &m.SenderID, &m.MessageType, &m.Body,
			&m.AttachmentURL, &m.AttachmentName, &m.AttachmentSize, &m.AttachmentType,
			&createdAtRaw, &m.Sender.FullName, &m.Sender.Username, &m.Sender.ProfilePhoto,
		)
		if err != nil {
			return nil, err
		}
		m.CreatedAt = parseSQLTime(createdAtRaw)
		m.Sender.ID = m.SenderID
		m.Sender.Initials = utils.Initials(m.Sender.FullName)
		messages = append(messages, &m)
	}
	return messages, nil
}

// GetConversationSharedMedia retrieves messages containing attachments in conversationID.
func GetConversationSharedMedia(conversationID int, limit int) ([]models.Message, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	query := fmt.Sprintf(`
		SELECT m.id, m.conversation_id, m.sender_id, m.message_type, m.body,
		       COALESCE(m.attachment_url, ''), COALESCE(m.attachment_name, ''),
		       COALESCE(m.attachment_size, 0), COALESCE(m.attachment_type, ''),
		       m.created_at, u.full_name, u.username
		FROM messages m
		JOIN users u ON m.sender_id = u.id
		WHERE m.conversation_id = %s
		  AND m.attachment_url IS NOT NULL
		  AND m.attachment_url != ''
		  AND m.is_deleted = FALSE
		ORDER BY m.created_at DESC
		LIMIT %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	rows, err := database.DB.Query(query, conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Message
	for rows.Next() {
		var m models.Message
		var createdAtRaw any
		err := rows.Scan(
			&m.ID, &m.ConversationID, &m.SenderID, &m.MessageType, &m.Body,
			&m.AttachmentURL, &m.AttachmentName, &m.AttachmentSize, &m.AttachmentType,
			&createdAtRaw, &m.Sender.FullName, &m.Sender.Username,
		)
		if err != nil {
			return nil, err
		}
		m.CreatedAt = parseSQLTime(createdAtRaw)
		m.Sender.ID = m.SenderID
		m.Sender.Initials = utils.Initials(m.Sender.FullName)
		list = append(list, m)
	}
	return list, nil
}

// CreateCallLog persists a new call session record.
func CreateCallLog(call *models.CallLog) error {
	var receiverVal any = call.ReceiverID
	if call.ReceiverID <= 0 {
		receiverVal = nil
	}
	query := fmt.Sprintf(`
		INSERT INTO call_logs (conversation_id, caller_id, receiver_id, call_type, status, duration_seconds, started_at)
		VALUES (%s, %s, %s, %s, %s, 0, CURRENT_TIMESTAMP)
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4), helpers.Placeholder(5))
	res, err := database.DB.Exec(query, call.ConversationID, call.CallerID, receiverVal, call.CallType, call.Status)
	if err != nil {
		return err
	}
	if id, err := res.LastInsertId(); err == nil && id > 0 {
		call.ID = int(id)
	} else {
		_ = database.DB.QueryRow(fmt.Sprintf("SELECT id FROM call_logs WHERE conversation_id = %s ORDER BY id DESC LIMIT 1", helpers.Placeholder(1)), call.ConversationID).Scan(&call.ID)
	}
	return nil
}

// UpdateCallLog updates status and duration of an ended call session.
func UpdateCallLog(callID int, status string, durationSeconds int) error {
	query := fmt.Sprintf(`
		UPDATE call_logs
		SET status = %s, duration_seconds = %s, ended_at = CURRENT_TIMESTAMP
		WHERE id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3))
	_, err := database.DB.Exec(query, status, durationSeconds, callID)
	return err
}

// AddCallParticipant records a member joining an audio/video call session.
func AddCallParticipant(callID, userID int) (*models.CallParticipant, error) {
	query := fmt.Sprintf(`
		INSERT INTO call_participants (call_id, user_id, joined_at, duration_seconds)
		VALUES (%s, %s, CURRENT_TIMESTAMP, 0)
	`, helpers.Placeholder(1), helpers.Placeholder(2))
	res, err := database.DB.Exec(query, callID, userID)
	if err != nil {
		return nil, err
	}
	pID, _ := res.LastInsertId()
	return &models.CallParticipant{
		ID:       int(pID),
		CallID:   callID,
		UserID:   userID,
		JoinedAt: time.Now(),
	}, nil
}

// UpdateCallParticipantLeft records when a participant leaves a call and their individual duration.
func UpdateCallParticipantLeft(callID, userID int, durationSeconds int) error {
	query := fmt.Sprintf(`
		UPDATE call_participants
		SET left_at = CURRENT_TIMESTAMP, duration_seconds = %s
		WHERE call_id = %s AND user_id = %s AND left_at IS NULL
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3))
	_, err := database.DB.Exec(query, durationSeconds, callID, userID)
	return err
}

// GetCallParticipants returns all participants who joined a call session.
func GetCallParticipants(callID int) ([]models.CallParticipant, error) {
	query := fmt.Sprintf(`
		SELECT cp.id, cp.call_id, cp.user_id, cp.joined_at, cp.left_at, cp.duration_seconds,
		       u.full_name, u.username, COALESCE(u.profile_photo, '')
		FROM call_participants cp
		JOIN users u ON cp.user_id = u.id
		WHERE cp.call_id = %s
		ORDER BY cp.joined_at ASC
	`, helpers.Placeholder(1))
	rows, err := database.DB.Query(query, callID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var participants []models.CallParticipant
	for rows.Next() {
		var p models.CallParticipant
		var joinedRaw, leftRaw any
		var photo string
		err := rows.Scan(
			&p.ID, &p.CallID, &p.UserID, &joinedRaw, &leftRaw, &p.DurationSeconds,
			&p.User.FullName, &p.User.Username, &photo,
		)
		if err != nil {
			return nil, err
		}
		p.JoinedAt = parseSQLTime(joinedRaw)
		if leftRaw != nil {
			t := parseSQLTime(leftRaw)
			if !t.IsZero() {
				p.LeftAt = &t
			}
		}
		p.User.ID = p.UserID
		p.User.ProfilePhoto = photo
		p.User.Initials = utils.Initials(p.User.FullName)
		participants = append(participants, p)
	}
	return participants, nil
}

// GetConversationCallLogs returns recent call logs for a conversation (direct or group).
func GetConversationCallLogs(conversationID int, limit int) ([]models.CallLog, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	query := fmt.Sprintf(`
		SELECT cl.id, cl.conversation_id, cl.caller_id, COALESCE(cl.receiver_id, 0), cl.call_type, cl.status,
		       cl.duration_seconds, cl.started_at, cl.ended_at,
		       u1.full_name, u1.username, COALESCE(u1.profile_photo, ''),
		       COALESCE(u2.full_name, ''), COALESCE(u2.username, ''), COALESCE(u2.profile_photo, '')
		FROM call_logs cl
		JOIN users u1 ON cl.caller_id = u1.id
		LEFT JOIN users u2 ON cl.receiver_id = u2.id
		WHERE cl.conversation_id = %s
		ORDER BY cl.started_at DESC
		LIMIT %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	rows, err := database.DB.Query(query, conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.CallLog
	for rows.Next() {
		var cl models.CallLog
		var startedAtRaw, endedAtRaw any
		err := rows.Scan(
			&cl.ID, &cl.ConversationID, &cl.CallerID, &cl.ReceiverID, &cl.CallType, &cl.Status,
			&cl.DurationSeconds, &startedAtRaw, &endedAtRaw,
			&cl.Caller.FullName, &cl.Caller.Username, &cl.Caller.ProfilePhoto,
			&cl.Receiver.FullName, &cl.Receiver.Username, &cl.Receiver.ProfilePhoto,
		)
		if err != nil {
			return nil, err
		}
		cl.StartedAt = parseSQLTime(startedAtRaw)
		if endedAtRaw != nil {
			t := parseSQLTime(endedAtRaw)
			if !t.IsZero() {
				cl.EndedAt = &t
			}
		}
		cl.Caller.ID = cl.CallerID
		cl.Receiver.ID = cl.ReceiverID
		cl.Caller.Initials = utils.Initials(cl.Caller.FullName)
		if cl.ReceiverID > 0 {
			cl.Receiver.Initials = utils.Initials(cl.Receiver.FullName)
		}
		logs = append(logs, cl)
	}
	return logs, nil
}

// CreateGroupConversation creates a new group chat and enrolls the creator as owner plus any initial members.
func CreateGroupConversation(title, category, description, avatarURL, callPermission string, isPublic bool, creatorID int, initialMemberIDs []int) (*models.Conversation, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, errors.New("group title cannot be empty")
	}
	if category == "" {
		category = "general"
	}
	if callPermission == "" {
		callPermission = "all"
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	insertQuery := fmt.Sprintf(`
		INSERT INTO conversations (type, title, category, description, avatar_url, call_permission, is_public, join_policy, created_by, created_at, updated_at)
		VALUES ('group', %s, %s, %s, %s, %s, %s, 'approval', %s, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4), helpers.Placeholder(5), helpers.Placeholder(6), helpers.Placeholder(7))

	res, err := tx.Exec(insertQuery, title, category, description, avatarURL, callPermission, isPublic, creatorID)
	if err != nil {
		return nil, err
	}

	convID, err := res.LastInsertId()
	if err != nil || convID == 0 {
		err = tx.QueryRow(fmt.Sprintf("SELECT id FROM conversations WHERE created_by = %s ORDER BY id DESC LIMIT 1", helpers.Placeholder(1)), creatorID).Scan(&convID)
		if err != nil {
			return nil, err
		}
	}

	// Insert creator as owner
	insertMemberQuery := fmt.Sprintf(`
		INSERT INTO conversation_members (conversation_id, user_id, role, joined_at)
		VALUES (%s, %s, 'owner', CURRENT_TIMESTAMP)
	`, helpers.Placeholder(1), helpers.Placeholder(2))
	if _, err := tx.Exec(insertMemberQuery, convID, creatorID); err != nil {
		return nil, err
	}

	// Insert initial members
	for _, memberID := range initialMemberIDs {
		if memberID <= 0 || memberID == creatorID {
			continue
		}
		memberInsert := fmt.Sprintf(`
			INSERT INTO conversation_members (conversation_id, user_id, role, joined_at)
			SELECT %s, %s, 'member', CURRENT_TIMESTAMP
			WHERE NOT EXISTS (
				SELECT 1 FROM conversation_members WHERE conversation_id = %s AND user_id = %s
			)
		`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4))
		_, _ = tx.Exec(memberInsert, convID, memberID, convID, memberID)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return GetConversationByID(int(convID))
}

// GetConversationMembers returns all participants in a conversation with roles.
func GetConversationMembers(conversationID int) ([]models.ConversationMember, error) {
	query := fmt.Sprintf(`
		SELECT cm.id, cm.conversation_id, cm.user_id, cm.role,
		       COALESCE(cm.last_read_message_id, 0),
		       COALESCE(cm.is_muted, FALSE), COALESCE(cm.is_archived, FALSE), cm.joined_at,
		       u.id, u.full_name, u.username, COALESCE(u.profile_photo, ''), COALESCE(u.church_name, '')
		FROM conversation_members cm
		JOIN users u ON cm.user_id = u.id
		WHERE cm.conversation_id = %s
		ORDER BY 
			CASE cm.role 
				WHEN 'owner' THEN 1 
				WHEN 'admin' THEN 2 
				WHEN 'moderator' THEN 3 
				ELSE 4 
			END, 
			u.full_name ASC
	`, helpers.Placeholder(1))

	rows, err := database.DB.Query(query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []models.ConversationMember
	for rows.Next() {
		var m models.ConversationMember
		var isMuted, isArchived bool
		var joinedAtRaw any
		var photo, church string

		err := rows.Scan(
			&m.ID, &m.ConversationID, &m.UserID, &m.Role,
			&m.LastReadMessageID, &isMuted, &isArchived, &joinedAtRaw,
			&m.User.ID, &m.User.FullName, &m.User.Username, &photo, &church,
		)
		if err != nil {
			return nil, err
		}

		m.IsMuted = isMuted
		m.IsArchived = isArchived
		m.JoinedAt = parseSQLTime(joinedAtRaw)
		m.User.ProfilePhoto = photo
		m.User.ChurchName = church
		m.User.Initials = utils.Initials(m.User.FullName)
		members = append(members, m)
	}

	return members, nil
}

// GetConversationMember gets a single user's membership details in a conversation.
func GetConversationMember(conversationID, userID int) (*models.ConversationMember, error) {
	query := fmt.Sprintf(`
		SELECT cm.id, cm.conversation_id, cm.user_id, cm.role,
		       COALESCE(cm.last_read_message_id, 0),
		       COALESCE(cm.is_muted, FALSE), COALESCE(cm.is_archived, FALSE), cm.joined_at,
		       u.id, u.full_name, u.username, COALESCE(u.profile_photo, '')
		FROM conversation_members cm
		JOIN users u ON cm.user_id = u.id
		WHERE cm.conversation_id = %s AND cm.user_id = %s
		LIMIT 1
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	var m models.ConversationMember
	var isMuted, isArchived bool
	var joinedAtRaw any
	var photo string

	err := database.DB.QueryRow(query, conversationID, userID).Scan(
		&m.ID, &m.ConversationID, &m.UserID, &m.Role,
		&m.LastReadMessageID, &isMuted, &isArchived, &joinedAtRaw,
		&m.User.ID, &m.User.FullName, &m.User.Username, &photo,
	)
	if err != nil {
		return nil, err
	}

	m.IsMuted = isMuted
	m.IsArchived = isArchived
	m.JoinedAt = parseSQLTime(joinedAtRaw)
	m.User.ProfilePhoto = photo
	m.User.Initials = utils.Initials(m.User.FullName)
	return &m, nil
}

// AddConversationMember adds a user to a conversation with a specified role.
func AddConversationMember(conversationID, userID int, role string) error {
	if role == "" {
		role = "member"
	}
	query := fmt.Sprintf(`
		INSERT INTO conversation_members (conversation_id, user_id, role, joined_at)
		SELECT %s, %s, %s, CURRENT_TIMESTAMP
		WHERE NOT EXISTS (
			SELECT 1 FROM conversation_members WHERE conversation_id = %s AND user_id = %s
		)
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4), helpers.Placeholder(5))

	_, err := database.DB.Exec(query, conversationID, userID, role, conversationID, userID)
	return err
}

// RemoveConversationMember removes a user from a conversation.
func RemoveConversationMember(conversationID, userID int) error {
	query := fmt.Sprintf(`
		DELETE FROM conversation_members
		WHERE conversation_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	_, err := database.DB.Exec(query, conversationID, userID)
	return err
}

// UpdateConversationMemberRole updates the role of a user in a conversation.
func UpdateConversationMemberRole(conversationID, userID int, role string) error {
	query := fmt.Sprintf(`
		UPDATE conversation_members
		SET role = %s
		WHERE conversation_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3))

	_, err := database.DB.Exec(query, role, conversationID, userID)
	return err
}

// UpdateGroupDetails updates the title, category, description, call permissions, and join policy for a group.
func UpdateGroupDetails(conversationID int, title, category, description, callPermission, joinPolicy string) error {
	if joinPolicy == "" {
		joinPolicy = "approval"
	}
	query := fmt.Sprintf(`
		UPDATE conversations
		SET title = %s, category = %s, description = %s, call_permission = %s, join_policy = %s, updated_at = CURRENT_TIMESTAMP
		WHERE id = %s AND type = 'group'
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4), helpers.Placeholder(5), helpers.Placeholder(6))

	_, err := database.DB.Exec(query, title, category, description, callPermission, joinPolicy, conversationID)
	return err
}

// ConvertDirectToGroup atomically transitions a direct conversation into a group conversation,
// updates the initiator's role to 'owner', sets other existing participants to 'member',
// and adds the new members to conversation_members.
func ConvertDirectToGroup(conversationID int, title string, creatorID int, newMemberIDs []int) error {
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Update conversation type and title
	updateConvQuery := fmt.Sprintf(`
		UPDATE conversations
		SET type = 'group', title = %s, created_by = %s, updated_at = CURRENT_TIMESTAMP
		WHERE id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3))

	if _, err := tx.Exec(updateConvQuery, title, creatorID, conversationID); err != nil {
		return err
	}

	// 2. Set creator's role to 'owner'
	updateCreatorRole := fmt.Sprintf(`
		UPDATE conversation_members
		SET role = 'owner'
		WHERE conversation_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))
	if _, err := tx.Exec(updateCreatorRole, conversationID, creatorID); err != nil {
		return err
	}

	// 3. Ensure other existing members remain 'member'
	updateOtherRole := fmt.Sprintf(`
		UPDATE conversation_members
		SET role = 'member'
		WHERE conversation_id = %s AND user_id != %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))
	if _, err := tx.Exec(updateOtherRole, conversationID, creatorID); err != nil {
		return err
	}

	// 4. Insert each new member
	for _, memberID := range newMemberIDs {
		if memberID <= 0 || memberID == creatorID {
			continue
		}
		insertMember := fmt.Sprintf(`
			INSERT INTO conversation_members (conversation_id, user_id, role, joined_at)
			SELECT %s, %s, 'member', CURRENT_TIMESTAMP
			WHERE NOT EXISTS (
				SELECT 1 FROM conversation_members WHERE conversation_id = %s AND user_id = %s
			)
		`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4))
		if _, err := tx.Exec(insertMember, conversationID, memberID, conversationID, memberID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetDiscoverableGroups retrieves public groups optionally filtered by category with membership and request status.
func GetDiscoverableGroups(userID int, category string) ([]models.ConversationItem, error) {
	var args []any
	args = append(args, userID, userID, userID)

	whereClause := "WHERE c.type = 'group' AND c.is_public = TRUE"
	if category != "" && category != "all" {
		whereClause += fmt.Sprintf(" AND c.category = %s", helpers.Placeholder(len(args)+1))
		args = append(args, category)
	}

	query := fmt.Sprintf(`
		SELECT c.id, c.type, COALESCE(c.title, ''), COALESCE(c.category, 'general'),
		       COALESCE(c.description, ''), COALESCE(c.avatar_url, ''), COALESCE(c.call_permission, 'all'),
		       COALESCE(c.join_policy, 'approval'),
		       (SELECT COUNT(*) FROM conversation_members WHERE conversation_id = c.id) as member_count,
		       (SELECT COUNT(*) FROM conversation_members WHERE conversation_id = c.id AND user_id = %s) as is_member,
		       COALESCE((SELECT status FROM group_membership_requests WHERE conversation_id = c.id AND user_id = %s AND status = 'pending' ORDER BY id DESC LIMIT 1), 'none') as req_status,
		       COALESCE((SELECT id FROM group_membership_requests WHERE conversation_id = c.id AND user_id = %s AND status = 'pending' ORDER BY id DESC LIMIT 1), 0) as req_id,
		       c.created_at
		FROM conversations c
		%s
		ORDER BY c.created_at DESC
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), whereClause)

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []models.ConversationItem
	for rows.Next() {
		var g models.ConversationItem
		var isMember int
		var createdAtRaw any

		err := rows.Scan(
			&g.ID, &g.Type, &g.Title, &g.Category,
			&g.Description, &g.AvatarURL, &g.CallPermission,
			&g.JoinPolicy,
			&g.MemberCount, &isMember,
			&g.MembershipRequestStatus, &g.MembershipRequestID,
			&createdAtRaw,
		)
		if err != nil {
			return nil, err
		}
		g.IsActive = isMember > 0 // true if user is already a member
		g.LastMessageTime = parseSQLTime(createdAtRaw)
		groups = append(groups, g)
	}

	return groups, nil
}

// GetMessageByID retrieves a message by its primary key ID.
func GetMessageByID(messageID int) (*models.Message, error) {
	query := fmt.Sprintf(`
		SELECT
			m.id, m.conversation_id, m.sender_id, m.message_type, m.body,
			COALESCE(m.client_message_id, ''), m.reply_to_message_id,
			m.is_edited, m.is_deleted, m.deleted_by, m.deleted_at,
			COALESCE(m.attachment_url, ''), COALESCE(m.attachment_name, ''),
			COALESCE(m.attachment_size, 0), COALESCE(m.attachment_type, ''),
			m.created_at, m.updated_at,
			u.id, u.full_name, u.username, COALESCE(u.profile_photo, '')
		FROM messages m
		JOIN users u ON m.sender_id = u.id
		WHERE m.id = %s
	`, helpers.Placeholder(1))

	var m models.Message
	var replyTo sql.NullInt64
	var deletedBy sql.NullInt64
	var deletedAtRaw, createdAtRaw, updatedAtRaw any
	var photo string

	err := database.DB.QueryRow(query, messageID).Scan(
		&m.ID, &m.ConversationID, &m.SenderID, &m.MessageType, &m.Body,
		&m.ClientMessageID, &replyTo,
		&m.IsEdited, &m.IsDeleted, &deletedBy, &deletedAtRaw,
		&m.AttachmentURL, &m.AttachmentName, &m.AttachmentSize, &m.AttachmentType,
		&createdAtRaw, &updatedAtRaw,
		&m.Sender.ID, &m.Sender.FullName, &m.Sender.Username, &photo,
	)
	if err != nil {
		return nil, err
	}

	if replyTo.Valid {
		id := int(replyTo.Int64)
		m.ReplyToMessageID = &id
	}
	if deletedBy.Valid {
		id := int(deletedBy.Int64)
		m.DeletedBy = &id
	}
	if t := parseSQLTime(deletedAtRaw); !t.IsZero() {
		m.DeletedAt = &t
	}
	m.CreatedAt = parseSQLTime(createdAtRaw)
	m.UpdatedAt = parseSQLTime(updatedAtRaw)
	m.Sender.ProfilePhoto = photo
	m.Sender.Initials = utils.Initials(m.Sender.FullName)
	return &m, nil
}

// SoftDeleteMessage marks a message as deleted by a moderator or sender.
func SoftDeleteMessage(messageID, deletedByID int) error {
	query := fmt.Sprintf(`
		UPDATE messages
		SET is_deleted = TRUE,
		    deleted_by = %s,
		    deleted_at = CURRENT_TIMESTAMP,
		    body = 'This message was removed by a moderator.',
		    attachment_url = '',
		    attachment_name = '',
		    attachment_size = 0,
		    attachment_type = '',
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	_, err := database.DB.Exec(query, deletedByID, messageID)
	return err
}

// CountGroupAdministrators counts how many members in a group have role 'owner' or 'admin'.
func CountGroupAdministrators(convID int) (int, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM conversation_members
		WHERE conversation_id = %s AND role IN ('owner', 'admin')
	`, helpers.Placeholder(1))

	var count int
	err := database.DB.QueryRow(query, convID).Scan(&count)
	return count, err
}

// GetGroupAdministrators returns all administrators (owner and admin) of a group.
func GetGroupAdministrators(convID int) ([]models.ConversationMember, error) {
	query := fmt.Sprintf(`
		SELECT cm.id, cm.conversation_id, cm.user_id, cm.role, cm.joined_at,
		       u.id, u.full_name, u.username, COALESCE(u.profile_photo, '')
		FROM conversation_members cm
		JOIN users u ON cm.user_id = u.id
		WHERE cm.conversation_id = %s AND cm.role IN ('owner', 'admin')
		ORDER BY CASE cm.role WHEN 'owner' THEN 1 ELSE 2 END, u.full_name ASC
	`, helpers.Placeholder(1))

	rows, err := database.DB.Query(query, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var admins []models.ConversationMember
	for rows.Next() {
		var m models.ConversationMember
		var joinedRaw any
		var photo string
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.UserID, &m.Role, &joinedRaw, &m.User.ID, &m.User.FullName, &m.User.Username, &photo); err == nil {
			m.JoinedAt = parseSQLTime(joinedRaw)
			m.User.ProfilePhoto = photo
			m.User.Initials = utils.Initials(m.User.FullName)
			admins = append(admins, m)
		}
	}
	return admins, nil
}

// CreateGroupMembershipRequest creates a pending membership request for an approval-based group.
func CreateGroupMembershipRequest(convID, userID int, message string) (*models.GroupMembershipRequest, error) {
	existing, err := GetPendingMembershipRequest(convID, userID)
	if err == nil && existing != nil {
		return nil, errors.New("a membership request is already pending for this group")
	}

	query := fmt.Sprintf(`
		INSERT INTO group_membership_requests (conversation_id, user_id, status, request_message, created_at, updated_at)
		VALUES (%s, %s, 'pending', %s, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3))

	res, err := database.DB.Exec(query, convID, userID, message)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil || id == 0 {
		err = database.DB.QueryRow(fmt.Sprintf("SELECT id FROM group_membership_requests WHERE conversation_id = %s AND user_id = %s ORDER BY id DESC LIMIT 1", helpers.Placeholder(1), helpers.Placeholder(2)), convID, userID).Scan(&id)
		if err != nil {
			return nil, err
		}
	}

	return GetGroupMembershipRequestByID(int(id))
}

// GetPendingMembershipRequest retrieves a pending request for a user in a group.
func GetPendingMembershipRequest(convID, userID int) (*models.GroupMembershipRequest, error) {
	query := fmt.Sprintf(`
		SELECT id, conversation_id, user_id, status, request_message, created_at, updated_at
		FROM group_membership_requests
		WHERE conversation_id = %s AND user_id = %s AND status = 'pending'
		ORDER BY id DESC LIMIT 1
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	var req models.GroupMembershipRequest
	var createdRaw, updatedRaw any

	err := database.DB.QueryRow(query, convID, userID).Scan(
		&req.ID, &req.ConversationID, &req.UserID, &req.Status, &req.RequestMessage, &createdRaw, &updatedRaw,
	)
	if err != nil {
		return nil, err
	}
	req.CreatedAt = parseSQLTime(createdRaw)
	req.UpdatedAt = parseSQLTime(updatedRaw)
	return &req, nil
}

// GetGroupMembershipRequestByID retrieves a single request by its ID.
func GetGroupMembershipRequestByID(requestID int) (*models.GroupMembershipRequest, error) {
	query := fmt.Sprintf(`
		SELECT r.id, r.conversation_id, r.user_id, r.status, r.request_message,
		       r.reviewed_by, r.reviewed_at, r.created_at, r.updated_at,
		       u.id, u.full_name, u.username, COALESCE(u.profile_photo, ''),
		       rev.id, rev.full_name, rev.username
		FROM group_membership_requests r
		JOIN users u ON r.user_id = u.id
		LEFT JOIN users rev ON r.reviewed_by = rev.id
		WHERE r.id = %s
	`, helpers.Placeholder(1))

	var req models.GroupMembershipRequest
	var reviewedBy, reviewerID sql.NullInt64
	var reviewerName, reviewerUsername sql.NullString
	var reviewedAtRaw, createdRaw, updatedRaw any
	var userPhoto string

	err := database.DB.QueryRow(query, requestID).Scan(
		&req.ID, &req.ConversationID, &req.UserID, &req.Status, &req.RequestMessage,
		&reviewedBy, &reviewedAtRaw, &createdRaw, &updatedRaw,
		&req.User.ID, &req.User.FullName, &req.User.Username, &userPhoto,
		&reviewerID, &reviewerName, &reviewerUsername,
	)
	if err != nil {
		return nil, err
	}

	req.User.ProfilePhoto = userPhoto
	req.User.Initials = utils.Initials(req.User.FullName)
	req.CreatedAt = parseSQLTime(createdRaw)
	req.UpdatedAt = parseSQLTime(updatedRaw)

	if reviewedBy.Valid {
		id := int(reviewedBy.Int64)
		req.ReviewedBy = &id
	}
	if t := parseSQLTime(reviewedAtRaw); !t.IsZero() {
		req.ReviewedAt = &t
	}
	if reviewerID.Valid {
		req.Reviewer = &models.User{
			ID:       int(reviewerID.Int64),
			FullName: reviewerName.String,
			Username: reviewerUsername.String,
		}
	}
	return &req, nil
}

// GetGroupMembershipRequests retrieves membership requests for a group, optionally filtered by status.
func GetGroupMembershipRequests(convID int, status string) ([]models.GroupMembershipRequest, error) {
	var args []any
	args = append(args, convID)

	whereClause := fmt.Sprintf("WHERE r.conversation_id = %s", helpers.Placeholder(1))
	if status != "" && status != "all" {
		whereClause += fmt.Sprintf(" AND r.status = %s", helpers.Placeholder(len(args)+1))
		args = append(args, status)
	}

	query := fmt.Sprintf(`
		SELECT r.id, r.conversation_id, r.user_id, r.status, r.request_message,
		       r.reviewed_by, r.reviewed_at, r.created_at, r.updated_at,
		       u.id, u.full_name, u.username, COALESCE(u.profile_photo, ''),
		       rev.id, rev.full_name, rev.username
		FROM group_membership_requests r
		JOIN users u ON r.user_id = u.id
		LEFT JOIN users rev ON r.reviewed_by = rev.id
		%s
		ORDER BY
			CASE r.status WHEN 'pending' THEN 1 ELSE 2 END,
			r.created_at DESC
	`, whereClause)

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.GroupMembershipRequest
	for rows.Next() {
		var req models.GroupMembershipRequest
		var reviewedBy, reviewerID sql.NullInt64
		var reviewerName, reviewerUsername sql.NullString
		var reviewedAtRaw, createdRaw, updatedRaw any
		var userPhoto string

		err := rows.Scan(
			&req.ID, &req.ConversationID, &req.UserID, &req.Status, &req.RequestMessage,
			&reviewedBy, &reviewedAtRaw, &createdRaw, &updatedRaw,
			&req.User.ID, &req.User.FullName, &req.User.Username, &userPhoto,
			&reviewerID, &reviewerName, &reviewerUsername,
		)
		if err != nil {
			return nil, err
		}

		req.User.ProfilePhoto = userPhoto
		req.User.Initials = utils.Initials(req.User.FullName)
		req.CreatedAt = parseSQLTime(createdRaw)
		req.UpdatedAt = parseSQLTime(updatedRaw)

		if reviewedBy.Valid {
			id := int(reviewedBy.Int64)
			req.ReviewedBy = &id
		}
		if t := parseSQLTime(reviewedAtRaw); !t.IsZero() {
			req.ReviewedAt = &t
		}
		if reviewerID.Valid {
			req.Reviewer = &models.User{
				ID:       int(reviewerID.Int64),
				FullName: reviewerName.String,
				Username: reviewerUsername.String,
			}
		}

		list = append(list, req)
	}
	return list, nil
}

// UpdateGroupMembershipRequestStatus updates the status of a request and records the reviewer.
func UpdateGroupMembershipRequestStatus(requestID, reviewerID int, newStatus string) error {
	query := fmt.Sprintf(`
		UPDATE group_membership_requests
		SET status = %s, reviewed_by = %s, reviewed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3))

	_, err := database.DB.Exec(query, newStatus, reviewerID, requestID)
	return err
}

// CancelGroupMembershipRequest cancels a pending request by the requester.
func CancelGroupMembershipRequest(requestID, userID int) error {
	query := fmt.Sprintf(`
		UPDATE group_membership_requests
		SET status = 'cancelled', updated_at = CURRENT_TIMESTAMP
		WHERE id = %s AND user_id = %s AND status = 'pending'
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	res, err := database.DB.Exec(query, requestID, userID)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("no pending request found to cancel")
	}
	return nil
}

// CountPendingGroupMembershipRequests returns the count of pending requests for a group.
func CountPendingGroupMembershipRequests(convID int) (int, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM group_membership_requests
		WHERE conversation_id = %s AND status = 'pending'
	`, helpers.Placeholder(1))

	var count int
	err := database.DB.QueryRow(query, convID).Scan(&count)
	return count, err
}

// EnsureDefaultCommunityMembership enrolls the user into the open default community group, creating it if needed.
func EnsureDefaultCommunityMembership(userID int) error {
	if database.DB == nil {
		return errors.New("database not initialized")
	}
	var generalConvID int
	query := "SELECT id FROM conversations WHERE type = 'group' AND title = 'Youth General Community' LIMIT 1"
	err := database.DB.QueryRow(query).Scan(&generalConvID)
	if err == sql.ErrNoRows {
		insertQuery := fmt.Sprintf(`
			INSERT INTO conversations (type, title, category, description, call_permission, is_public, join_policy, created_by, created_at, updated_at)
			VALUES ('group', 'Youth General Community', 'general', 'Open fellowship space for all convention youth, announcements, and general discussions.', 'anyone', TRUE, 'open', %s, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`, helpers.Placeholder(1))
		res, insErr := database.DB.Exec(insertQuery, userID)
		if insErr != nil {
			return insErr
		}
		if lastID, lErr := res.LastInsertId(); lErr == nil && lastID > 0 {
			generalConvID = int(lastID)
		} else {
			_ = database.DB.QueryRow("SELECT id FROM conversations WHERE type = 'group' AND title = 'Youth General Community' LIMIT 1").Scan(&generalConvID)
		}
	} else if err != nil {
		return err
	}
	return AddConversationMember(generalConvID, userID, "member")
}
