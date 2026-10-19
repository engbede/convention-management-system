package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
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
		       COALESCE(join_policy, 'approval'), COALESCE(disappearing_duration, 0),
		       created_by, last_message_at, created_at, updated_at
		FROM conversations
		WHERE id = %s
	`, helpers.Placeholder(1))

	var conv models.Conversation
	var lastMsg, createdAtRaw, updatedAtRaw any
	var title, category, description, avatarURL, callPermission, joinPolicy string
	var isPublic bool
	var disappearingDuration int

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
		&disappearingDuration,
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
	conv.DisappearingDuration = disappearingDuration
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
	if msg.ExpiresAt == nil && msg.ConversationID > 0 {
		var disappearingDuration int
		_ = database.DB.QueryRow(fmt.Sprintf(`SELECT COALESCE(disappearing_duration, 0) FROM conversations WHERE id = %s`, helpers.Placeholder(1)), msg.ConversationID).Scan(&disappearingDuration)
		if disappearingDuration > 0 {
			exp := time.Now().UTC().Add(time.Duration(disappearingDuration) * time.Second)
			msg.ExpiresAt = &exp
		}
	}

	insertQuery := fmt.Sprintf(`
		INSERT INTO messages (
			conversation_id, sender_id, message_type, body, client_message_id, reply_to_message_id,
			attachment_url, attachment_name, attachment_size, attachment_type,
			is_edited, is_deleted, expires_at, created_at, updated_at
		)
		VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, FALSE, FALSE, %s, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
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
		helpers.Placeholder(11),
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
		msg.ExpiresAt,
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
// GetMessagesByIDs retrieves multiple messages by their IDs, useful for batch hydration of quoted replies.
func GetMessagesByIDs(ids []int) (map[int]models.Message, error) {
	if len(ids) == 0 {
		return make(map[int]models.Message), nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = helpers.Placeholder(i + 1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT 
			m.id, m.conversation_id, m.sender_id, m.message_type, m.body,
			COALESCE(m.client_message_id, ''), m.reply_to_message_id,
			m.is_edited, m.is_deleted,
			COALESCE(m.attachment_url, ''), COALESCE(m.attachment_name, ''),
			COALESCE(m.attachment_size, 0), COALESCE(m.attachment_type, ''),
			m.expires_at,
			m.created_at, m.updated_at,
			u.id, u.full_name, u.username, COALESCE(u.profile_photo, '')
		FROM messages m
		JOIN users u ON m.sender_id = u.id
		WHERE m.id IN (%s)
	`, strings.Join(placeholders, ", "))

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]models.Message)
	for rows.Next() {
		var m models.Message
		var replyTo sql.NullInt64
		var photo string
		var expiresAtRaw any
		var createdAtRaw, updatedAtRaw any

		err := rows.Scan(
			&m.ID, &m.ConversationID, &m.SenderID, &m.MessageType, &m.Body,
			&m.ClientMessageID, &replyTo,
			&m.IsEdited, &m.IsDeleted,
			&m.AttachmentURL, &m.AttachmentName, &m.AttachmentSize, &m.AttachmentType,
			&expiresAtRaw,
			&createdAtRaw, &updatedAtRaw,
			&m.Sender.ID, &m.Sender.FullName, &m.Sender.Username, &photo,
		)
		if err != nil {
			return nil, err
		}
		if t := parseSQLTime(expiresAtRaw); !t.IsZero() {
			m.ExpiresAt = &t
		}
		m.CreatedAt = parseSQLTime(createdAtRaw)
		m.UpdatedAt = parseSQLTime(updatedAtRaw)
		if replyTo.Valid {
			repID := int(replyTo.Int64)
			m.ReplyToMessageID = &repID
		}
		m.Sender.ProfilePhoto = photo
		m.Sender.Initials = utils.Initials(m.Sender.FullName)
		result[m.ID] = m
	}
	return result, nil
}

// GetConversationMessages returns ordered messages for a conversation with sender details, read status, and hydration.
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
			m.expires_at,
			m.created_at, m.updated_at,
			u.id, u.full_name, u.username, COALESCE(u.profile_photo, '')
		FROM messages m
		JOIN users u ON m.sender_id = u.id
		LEFT JOIN conversation_members cm_curr ON cm_curr.conversation_id = m.conversation_id AND cm_curr.user_id = %s
		LEFT JOIN message_deletions md ON md.message_id = m.id AND md.user_id = %s
		WHERE m.conversation_id = %s
		  AND md.id IS NULL
		  AND (cm_curr.cleared_at IS NULL OR m.created_at > cm_curr.cleared_at)
		  AND (m.expires_at IS NULL OR m.expires_at > CURRENT_TIMESTAMP)
		ORDER BY m.created_at ASC, m.id ASC
		LIMIT %s OFFSET %s
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4), helpers.Placeholder(5))

	rows, err := database.DB.Query(query, currentUserID, currentUserID, conversationID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		var replyTo sql.NullInt64
		var photo string
		var expiresAtRaw any
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
			&expiresAtRaw,
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

		if t := parseSQLTime(expiresAtRaw); !t.IsZero() {
			m.ExpiresAt = &t
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

	// Hydrate quoted replies
	replyIDs := make([]int, 0)
	for _, m := range messages {
		if m.ReplyToMessageID != nil && *m.ReplyToMessageID > 0 {
			replyIDs = append(replyIDs, *m.ReplyToMessageID)
		}
	}
	if len(replyIDs) > 0 {
		replyMap, err := GetMessagesByIDs(replyIDs)
		if err == nil {
			for i := range messages {
				if messages[i].ReplyToMessageID != nil {
					if rep, ok := replyMap[*messages[i].ReplyToMessageID]; ok {
						repCopy := rep
						messages[i].ReplyToMessage = &repCopy
					}
				}
			}
		}
	}

	return messages, nil
}

// GetUserConversations returns all active conversations for a user with unread counts, mute, archive, and pin flags.
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
			COALESCE(c.disappearing_duration, 0),
			(SELECT COUNT(*) FROM conversation_members WHERE conversation_id = c.id) as member_count,
			COALESCE(c.last_message_at, c.created_at) as sort_time,
			cm.last_read_message_id,
			COALESCE(cm.is_muted, FALSE),
			cm.muted_until,
			COALESCE(cm.is_archived, FALSE),
			COALESCE(cm.is_pinned, FALSE),
			cm.cleared_at,
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
		-- Subquery for the most recent message in the conversation (excluding expired messages)
		LEFT JOIN (
			SELECT m1.conversation_id, m1.id, m1.body, m1.sender_id, m1.created_at
			FROM messages m1
			INNER JOIN (
				SELECT conversation_id, MAX(id) as max_id
				FROM messages
				WHERE (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
				GROUP BY conversation_id
			) m2 ON m1.conversation_id = m2.conversation_id AND m1.id = m2.max_id
		) latest ON latest.conversation_id = c.id
		WHERE cm.user_id = %s
		ORDER BY COALESCE(cm.is_pinned, FALSE) DESC, sort_time DESC
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
		var isMuted, isArchived, isPinned bool
		var mutedUntilRaw, clearedAtRaw any
		var disappearingDuration int
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
			&disappearingDuration,
			&item.MemberCount,
			&sortTimeRaw,
			&lastReadID,
			&isMuted,
			&mutedUntilRaw,
			&isArchived,
			&isPinned,
			&clearedAtRaw,
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
		item.DisappearingDuration = disappearingDuration
		item.IsPinned = isPinned
		if t := parseSQLTime(mutedUntilRaw); !t.IsZero() {
			item.MutedUntil = &t
		}

		if isMuted {
			if item.MutedUntil == nil {
				item.IsMuted = true
			} else {
				item.IsMuted = item.MutedUntil.After(time.Now().UTC())
			}
		} else {
			item.IsMuted = false
		}
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

		// Suppress latest message if it was sent prior to cleared_at
		if t := parseSQLTime(clearedAtRaw); !t.IsZero() {
			if !item.LastMessageTime.IsZero() && !item.LastMessageTime.After(t) {
				item.LastMessage = ""
				item.LastMessageSenderID = 0
				item.LastMessageTime = time.Time{}
			}
		}

		// Calculate unread count (excluding expired and deleted/cleared messages)
		if latestMsgID > lastReadID {
			unreadQuery := fmt.Sprintf(`
				SELECT COUNT(*) FROM messages m
				LEFT JOIN message_deletions md ON md.message_id = m.id AND md.user_id = %s
				WHERE m.conversation_id = %s AND m.id > %s AND m.sender_id != %s
				  AND md.id IS NULL
				  AND (m.expires_at IS NULL OR m.expires_at > CURRENT_TIMESTAMP)
			`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4))
			args := []any{userID, item.ID, lastReadID, userID}
			if t := parseSQLTime(clearedAtRaw); !t.IsZero() {
				unreadQuery += fmt.Sprintf(" AND m.created_at > %s", helpers.Placeholder(5))
				args = append(args, t)
			}
			var unread int
			_ = database.DB.QueryRow(unreadQuery, args...).Scan(&unread)
			item.UnreadCount = unread
		}

		items = append(items, item)
	}

	return items, nil
}

// MarkConversationAsRead advances the user's read marker to the latest message.
func MarkConversationAsRead(conversationID, userID int) error {
	var maxID int
	queryMax := fmt.Sprintf(`SELECT COALESCE(MAX(id), 0) FROM messages WHERE conversation_id = %s AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)`, helpers.Placeholder(1))
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

// CountTotalUnreadMessages counts all unread messages for a user across all conversations (excluding expired messages).
func CountTotalUnreadMessages(userID int) (int, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(m.id)
		FROM messages m
		JOIN conversation_members cm ON m.conversation_id = cm.conversation_id
		WHERE cm.user_id = %s AND m.sender_id != %s AND m.id > cm.last_read_message_id
		  AND (m.expires_at IS NULL OR m.expires_at > CURRENT_TIMESTAMP)
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	var count int
	err := database.DB.QueryRow(query, userID, userID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// SetConversationMute sets the mute preference for a user in a conversation with optional duration.
// Options: "1h", "8h", "1w", "indefinite", "forever", "unmute".
func SetConversationMute(convID, userID int, option string) (*time.Time, bool, error) {
	option = strings.ToLower(strings.TrimSpace(option))
	var isMuted bool
	var mutedUntil *time.Time

	now := time.Now().UTC()
	switch option {
	case "1h", "1_hour", "1 hour":
		isMuted = true
		t := now.Add(1 * time.Hour)
		mutedUntil = &t
	case "8h", "8_hours", "8 hours":
		isMuted = true
		t := now.Add(8 * time.Hour)
		mutedUntil = &t
	case "1w", "1_week", "7d", "1 week", "7 days":
		isMuted = true
		t := now.Add(7 * 24 * time.Hour)
		mutedUntil = &t
	case "indefinite", "forever", "until_unmuted", "always":
		isMuted = true
		mutedUntil = nil
	case "unmute", "off", "0":
		isMuted = false
		mutedUntil = nil
	default:
		return nil, false, fmt.Errorf("invalid mute option %q: must be 1h, 8h, 1w, indefinite, or unmute", option)
	}

	query := fmt.Sprintf(`
		UPDATE conversation_members
		SET is_muted = %s, muted_until = %s
		WHERE conversation_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4))
	_, err := database.DB.Exec(query, isMuted, mutedUntil, convID, userID)
	if err != nil {
		return nil, false, err
	}
	return mutedUntil, isMuted, nil
}

// ToggleMuteConversation toggles the is_muted state for a user in a conversation.
func ToggleMuteConversation(convID, userID int) (bool, error) {
	currentlyMuted := IsConversationMutedBy(convID, userID)
	if currentlyMuted {
		_, isMuted, err := SetConversationMute(convID, userID, "unmute")
		return isMuted, err
	}
	_, isMuted, err := SetConversationMute(convID, userID, "indefinite")
	return isMuted, err
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

// IsUserInConversation checks if a user is an active participant or member of a conversation.
func IsUserInConversation(convID, userID int) (bool, error) {
	var count int
	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM conversation_members
		WHERE conversation_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))
	err := database.DB.QueryRow(query, convID, userID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetConversationMemberSettings gets the mute, archive, and pin status for a member.
func GetConversationMemberSettings(convID, userID int) (isMuted, isArchived, isPinned bool, err error) {
	var muted bool
	var mutedUntilRaw any
	query := fmt.Sprintf(`
		SELECT COALESCE(is_muted, FALSE), muted_until, COALESCE(is_archived, FALSE), COALESCE(is_pinned, FALSE)
		FROM conversation_members
		WHERE conversation_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))
	err = database.DB.QueryRow(query, convID, userID).Scan(&muted, &mutedUntilRaw, &isArchived, &isPinned)
	if err != nil {
		return false, false, false, err
	}
	if !muted {
		return false, isArchived, isPinned, nil
	}
	if mutedUntilRaw == nil {
		return true, isArchived, isPinned, nil
	}
	t := parseSQLTime(mutedUntilRaw)
	if t.IsZero() {
		return true, isArchived, isPinned, nil
	}
	return t.After(time.Now().UTC()), isArchived, isPinned, nil
}

// IsConversationMutedBy checks if a conversation is muted by a specific user (taking expiration into account).
func IsConversationMutedBy(convID, userID int) bool {
	var muted bool
	var mutedUntilRaw any
	query := fmt.Sprintf(`
		SELECT COALESCE(is_muted, FALSE), muted_until
		FROM conversation_members
		WHERE conversation_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))
	err := database.DB.QueryRow(query, convID, userID).Scan(&muted, &mutedUntilRaw)
	if err != nil || !muted {
		return false
	}
	if mutedUntilRaw == nil {
		return true // Indefinite mute
	}
	t := parseSQLTime(mutedUntilRaw)
	if t.IsZero() {
		return true
	}
	return t.After(time.Now().UTC())
}

// UpdateConversationDisappearingMessages updates the disappearing message timer for a conversation.
func UpdateConversationDisappearingMessages(convID int, durationSeconds int) error {
	query := fmt.Sprintf(`
		UPDATE conversations
		SET disappearing_duration = %s, updated_at = CURRENT_TIMESTAMP
		WHERE id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))
	_, err := database.DB.Exec(query, durationSeconds, convID)
	return err
}

// DeleteExpiredMessages purges expired messages and cleans up local attachments if applicable.
func DeleteExpiredMessages() (int64, error) {
	// 1. Find local attachments of expired messages before deleting
	querySelect := `
		SELECT attachment_url FROM messages
		WHERE expires_at IS NOT NULL AND expires_at <= CURRENT_TIMESTAMP AND attachment_url IS NOT NULL AND attachment_url != ''
	`
	rows, err := database.DB.Query(querySelect)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var attachURL string
			if err := rows.Scan(&attachURL); err == nil && attachURL != "" {
				// Safety check: only delete files within /uploads/
				if strings.HasPrefix(attachURL, "/uploads/") && !strings.Contains(attachURL, "..") {
					localPath := "." + attachURL
					_ = os.Remove(localPath)
				}
			}
		}
	}

	// 2. Delete expired messages from database
	queryDelete := `DELETE FROM messages WHERE expires_at IS NOT NULL AND expires_at <= CURRENT_TIMESTAMP`
	res, err := database.DB.Exec(queryDelete)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// CountMessagesInConversation returns the total number of non-expired messages in a conversation.
func CountMessagesInConversation(convID int) (int, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM messages WHERE conversation_id = %s AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)`, helpers.Placeholder(1))
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

// HasPendingChatReport checks if a pending report from reporterID on the target already exists.
func HasPendingChatReport(reporterID, reportedUserID int, messageID *int) (bool, error) {
	var count int
	var query string
	var err error
	if messageID != nil && *messageID > 0 {
		query = fmt.Sprintf(`
			SELECT COUNT(1) FROM chat_reports 
			WHERE reporter_id = %s AND reported_user_id = %s AND message_id = %s AND status = 'pending'
		`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3))
		err = database.DB.QueryRow(query, reporterID, reportedUserID, *messageID).Scan(&count)
	} else {
		query = fmt.Sprintf(`
			SELECT COUNT(1) FROM chat_reports 
			WHERE reporter_id = %s AND reported_user_id = %s AND (message_id IS NULL OR message_id = 0) AND status = 'pending'
		`, helpers.Placeholder(1), helpers.Placeholder(2))
		err = database.DB.QueryRow(query, reporterID, reportedUserID).Scan(&count)
	}
	return count > 0, err
}

// CreateChatReport persists an abuse report against a user or message.
func CreateChatReport(report *models.ChatReport) error {
	query := fmt.Sprintf(`
		INSERT INTO chat_reports (reporter_id, reported_user_id, conversation_id, message_id, reason, details, status, evidence_text, evidence_url, created_at)
		VALUES (%s, %s, %s, %s, %s, %s, 'pending', %s, %s, CURRENT_TIMESTAMP)
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4), helpers.Placeholder(5), helpers.Placeholder(6), helpers.Placeholder(7), helpers.Placeholder(8))
	res, err := database.DB.Exec(query, report.ReporterID, report.ReportedUserID, report.ConversationID, report.MessageID, report.Reason, report.Details, report.EvidenceText, report.EvidenceURL)
	if err != nil {
		return err
	}
	if id, err := res.LastInsertId(); err == nil && id > 0 {
		report.ID = int(id)
	}
	return nil
}

// CountPendingChatReports returns the total number of unreviewed reports.
func CountPendingChatReports() (int, error) {
	var count int
	err := database.DB.QueryRow("SELECT COUNT(1) FROM chat_reports WHERE status = 'pending'").Scan(&count)
	return count, err
}

// GetChatReports retrieves moderation reports with optional status filtering and pagination.
func GetChatReports(status string, limit, offset int) ([]models.ChatReport, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}

	whereClause := ""
	var args []any
	argIndex := 1

	if status != "" {
		whereClause = fmt.Sprintf("WHERE cr.status = %s", helpers.Placeholder(argIndex))
		args = append(args, status)
		argIndex++
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(1) FROM chat_reports cr %s", whereClause)
	var total int
	err := database.DB.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	selectQuery := fmt.Sprintf(`
		SELECT cr.id, cr.reporter_id, cr.reported_user_id, cr.conversation_id, cr.message_id,
		       cr.reason, COALESCE(cr.details, ''), cr.status,
		       COALESCE(cr.evidence_text, ''), COALESCE(cr.evidence_url, ''),
		       cr.reviewed_by, cr.reviewed_at,
		       COALESCE(cr.resolution_notes, ''), COALESCE(cr.resolution_action, 'none'),
		       cr.created_at,
		       u_rep.id, u_rep.full_name, u_rep.username, COALESCE(u_rep.profile_photo, ''),
		       u_tgt.id, u_tgt.full_name, u_tgt.username, COALESCE(u_tgt.profile_photo, ''),
		       COALESCE(u_rev.full_name, '')
		FROM chat_reports cr
		JOIN users u_rep ON cr.reporter_id = u_rep.id
		JOIN users u_tgt ON cr.reported_user_id = u_tgt.id
		LEFT JOIN users u_rev ON cr.reviewed_by = u_rev.id
		%s
		ORDER BY cr.created_at DESC
		LIMIT %s OFFSET %s
	`, whereClause, helpers.Placeholder(argIndex), helpers.Placeholder(argIndex+1))

	queryArgs := append(args, limit, offset)
	rows, err := database.DB.Query(selectQuery, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	reports := make([]models.ChatReport, 0)
	for rows.Next() {
		var r models.ChatReport
		var rep models.User
		var tgt models.User
		var reviewerName string
		var reviewedAtRaw any
		var createdAtRaw any

		err := rows.Scan(
			&r.ID, &r.ReporterID, &r.ReportedUserID, &r.ConversationID, &r.MessageID,
			&r.Reason, &r.Details, &r.Status,
			&r.EvidenceText, &r.EvidenceURL,
			&r.ReviewedBy, &reviewedAtRaw,
			&r.ResolutionNotes, &r.ResolutionAction,
			&createdAtRaw,
			&rep.ID, &rep.FullName, &rep.Username, &rep.ProfilePhoto,
			&tgt.ID, &tgt.FullName, &tgt.Username, &tgt.ProfilePhoto,
			&reviewerName,
		)
		if err != nil {
			return nil, 0, err
		}
		r.CreatedAt = parseSQLTime(createdAtRaw)
		if reviewedAtRaw != nil {
			t := parseSQLTime(reviewedAtRaw)
			r.ReviewedAt = &t
		}
		rep.Initials = utils.Initials(rep.FullName)
		tgt.Initials = utils.Initials(tgt.FullName)
		r.Reporter = &rep
		r.ReportedUser = &tgt
		if r.ReviewedBy != nil {
			r.Reviewer = &models.User{
				ID:       *r.ReviewedBy,
				FullName: reviewerName,
			}
		}
		reports = append(reports, r)
	}

	return reports, total, nil
}

// GetChatReportByID retrieves a single report with hydrated users.
func GetChatReportByID(id int) (*models.ChatReport, error) {
	query := fmt.Sprintf(`
		SELECT cr.id, cr.reporter_id, cr.reported_user_id, cr.conversation_id, cr.message_id,
		       cr.reason, COALESCE(cr.details, ''), cr.status,
		       COALESCE(cr.evidence_text, ''), COALESCE(cr.evidence_url, ''),
		       cr.reviewed_by, cr.reviewed_at,
		       COALESCE(cr.resolution_notes, ''), COALESCE(cr.resolution_action, 'none'),
		       cr.created_at,
		       u_rep.id, u_rep.full_name, u_rep.username, COALESCE(u_rep.profile_photo, ''),
		       u_tgt.id, u_tgt.full_name, u_tgt.username, COALESCE(u_tgt.profile_photo, '')
		FROM chat_reports cr
		JOIN users u_rep ON cr.reporter_id = u_rep.id
		JOIN users u_tgt ON cr.reported_user_id = u_tgt.id
		WHERE cr.id = %s
	`, helpers.Placeholder(1))

	var r models.ChatReport
	var rep models.User
	var tgt models.User
	var reviewedAtRaw any
	var createdAtRaw any

	err := database.DB.QueryRow(query, id).Scan(
		&r.ID, &r.ReporterID, &r.ReportedUserID, &r.ConversationID, &r.MessageID,
		&r.Reason, &r.Details, &r.Status,
		&r.EvidenceText, &r.EvidenceURL,
		&r.ReviewedBy, &reviewedAtRaw,
		&r.ResolutionNotes, &r.ResolutionAction,
		&createdAtRaw,
		&rep.ID, &rep.FullName, &rep.Username, &rep.ProfilePhoto,
		&tgt.ID, &tgt.FullName, &tgt.Username, &tgt.ProfilePhoto,
	)
	if err != nil {
		return nil, err
	}
	r.CreatedAt = parseSQLTime(createdAtRaw)
	if reviewedAtRaw != nil {
		t := parseSQLTime(reviewedAtRaw)
		r.ReviewedAt = &t
	}
	rep.Initials = utils.Initials(rep.FullName)
	tgt.Initials = utils.Initials(tgt.FullName)
	r.Reporter = &rep
	r.ReportedUser = &tgt

	return &r, nil
}

// ResolveChatReport updates a report's review status and recorded actions.
func ResolveChatReport(reportID, reviewerID int, status, resolutionAction, notes string) error {
	query := fmt.Sprintf(`
		UPDATE chat_reports
		SET status = %s, reviewed_by = %s, reviewed_at = CURRENT_TIMESTAMP,
		    resolution_action = %s, resolution_notes = %s
		WHERE id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4), helpers.Placeholder(5))
	_, err := database.DB.Exec(query, status, reviewerID, resolutionAction, notes, reportID)
	return err
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
			  AND (m.is_deleted = FALSE OR m.is_deleted = 0)
			  AND m.deleted_at IS NULL
			  AND (m.expires_at IS NULL OR m.expires_at > CURRENT_TIMESTAMP)
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
			  AND (m.is_deleted = FALSE OR m.is_deleted = 0)
			  AND m.deleted_at IS NULL
			  AND (m.expires_at IS NULL OR m.expires_at > CURRENT_TIMESTAMP)
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
	return GetConversationSharedMediaCategorized(conversationID, "", limit, 0)
}

// GetConversationSharedMediaCategorized retrieves messages containing attachments in conversationID with optional category filtering and pagination.
func GetConversationSharedMediaCategorized(conversationID int, category string, limit, offset int) ([]models.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	filterClause := ""
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "photos", "photo", "image", "images":
		filterClause = " AND (m.message_type = 'image' OR m.attachment_type LIKE 'image/%')"
	case "videos", "video":
		filterClause = " AND (m.message_type = 'video' OR m.attachment_type LIKE 'video/%')"
	case "voice", "audio", "voicenote", "voicenotes", "voice_note", "voice_notes":
		filterClause = " AND (m.message_type = 'voice' OR m.attachment_type LIKE 'audio/%')"
	case "documents", "document", "doc", "docs", "files", "file":
		filterClause = " AND (m.message_type = 'file' OR (m.message_type NOT IN ('image', 'video', 'voice') AND m.attachment_type NOT LIKE 'image/%' AND m.attachment_type NOT LIKE 'video/%' AND m.attachment_type NOT LIKE 'audio/%'))"
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
		  AND (m.is_deleted = FALSE OR m.is_deleted = 0)
		  AND m.deleted_at IS NULL
		  AND (m.expires_at IS NULL OR m.expires_at > CURRENT_TIMESTAMP)
		  %s
		ORDER BY m.created_at DESC
		LIMIT %s OFFSET %s
	`, helpers.Placeholder(1), filterClause, helpers.Placeholder(2), helpers.Placeholder(3))

	rows, err := database.DB.Query(query, conversationID, limit, offset)
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
	var expiresVal any = call.ExpiresAt
	query := fmt.Sprintf(`
		INSERT INTO call_logs (conversation_id, caller_id, receiver_id, call_type, status, duration_seconds, started_at, expires_at)
		VALUES (%s, %s, %s, %s, %s, 0, CURRENT_TIMESTAMP, %s)
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4), helpers.Placeholder(5), helpers.Placeholder(6))
	res, err := database.DB.Exec(query, call.ConversationID, call.CallerID, receiverVal, call.CallType, call.Status, expiresVal)
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

// UpdateCallLogStatus transitions the call state without ending it.
func UpdateCallLogStatus(callID int, status string) error {
	query := fmt.Sprintf(`
		UPDATE call_logs
		SET status = %s
		WHERE id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))
	_, err := database.DB.Exec(query, status, callID)
	return err
}

// GetCallLogByID retrieves an authoritative call log by its unique ID.
func GetCallLogByID(callID int) (*models.CallLog, error) {
	query := fmt.Sprintf(`
		SELECT id, conversation_id, caller_id, COALESCE(receiver_id, 0), call_type, status, duration_seconds, started_at, ended_at, expires_at
		FROM call_logs
		WHERE id = %s
	`, helpers.Placeholder(1))
	var cl models.CallLog
	var endedAt sql.NullTime
	var expiresAt sql.NullTime
	err := database.DB.QueryRow(query, callID).Scan(
		&cl.ID,
		&cl.ConversationID,
		&cl.CallerID,
		&cl.ReceiverID,
		&cl.CallType,
		&cl.Status,
		&cl.DurationSeconds,
		&cl.StartedAt,
		&endedAt,
		&expiresAt,
	)
	if err != nil {
		return nil, err
	}
	if endedAt.Valid {
		cl.EndedAt = &endedAt.Time
	}
	if expiresAt.Valid {
		cl.ExpiresAt = &expiresAt.Time
	}
	return &cl, nil
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
		       COALESCE(cm.is_muted, FALSE), cm.muted_until, COALESCE(cm.is_archived, FALSE),
		       COALESCE(cm.is_pinned, FALSE), cm.cleared_at, cm.joined_at,
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
		var isMuted, isArchived, isPinned bool
		var mutedUntilRaw, clearedAtRaw any
		var joinedAtRaw any
		var photo, church string

		err := rows.Scan(
			&m.ID, &m.ConversationID, &m.UserID, &m.Role,
			&m.LastReadMessageID, &isMuted, &mutedUntilRaw, &isArchived,
			&isPinned, &clearedAtRaw, &joinedAtRaw,
			&m.User.ID, &m.User.FullName, &m.User.Username, &photo, &church,
		)
		if err != nil {
			return nil, err
		}

		m.IsMuted = isMuted
		if t := parseSQLTime(mutedUntilRaw); !t.IsZero() {
			m.MutedUntil = &t
		}
		m.IsArchived = isArchived
		m.IsPinned = isPinned
		if t := parseSQLTime(clearedAtRaw); !t.IsZero() {
			m.ClearedAt = &t
		}
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
		       COALESCE(cm.is_muted, FALSE), cm.muted_until, COALESCE(cm.is_archived, FALSE),
		       COALESCE(cm.is_pinned, FALSE), cm.cleared_at, cm.joined_at,
		       u.id, u.full_name, u.username, COALESCE(u.profile_photo, '')
		FROM conversation_members cm
		JOIN users u ON cm.user_id = u.id
		WHERE cm.conversation_id = %s AND cm.user_id = %s
		LIMIT 1
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	var m models.ConversationMember
	var isMuted, isArchived, isPinned bool
	var mutedUntilRaw, clearedAtRaw any
	var joinedAtRaw any
	var photo string

	err := database.DB.QueryRow(query, conversationID, userID).Scan(
		&m.ID, &m.ConversationID, &m.UserID, &m.Role,
		&m.LastReadMessageID, &isMuted, &mutedUntilRaw, &isArchived,
		&isPinned, &clearedAtRaw, &joinedAtRaw,
		&m.User.ID, &m.User.FullName, &m.User.Username, &photo,
	)
	if err != nil {
		return nil, err
	}

	m.IsMuted = isMuted
	if t := parseSQLTime(mutedUntilRaw); !t.IsZero() {
		m.MutedUntil = &t
	}
	m.IsArchived = isArchived
	m.IsPinned = isPinned
	if t := parseSQLTime(clearedAtRaw); !t.IsZero() {
		m.ClearedAt = &t
	}
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
			m.expires_at,
			m.created_at, m.updated_at,
			u.id, u.full_name, u.username, COALESCE(u.profile_photo, '')
		FROM messages m
		JOIN users u ON m.sender_id = u.id
		WHERE m.id = %s
	`, helpers.Placeholder(1))

	var m models.Message
	var replyTo sql.NullInt64
	var deletedBy sql.NullInt64
	var deletedAtRaw, createdAtRaw, updatedAtRaw, expiresAtRaw any
	var photo string

	err := database.DB.QueryRow(query, messageID).Scan(
		&m.ID, &m.ConversationID, &m.SenderID, &m.MessageType, &m.Body,
		&m.ClientMessageID, &replyTo,
		&m.IsEdited, &m.IsDeleted, &deletedBy, &deletedAtRaw,
		&m.AttachmentURL, &m.AttachmentName, &m.AttachmentSize, &m.AttachmentType,
		&expiresAtRaw,
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
	if t := parseSQLTime(expiresAtRaw); !t.IsZero() {
		m.ExpiresAt = &t
	}
	m.CreatedAt = parseSQLTime(createdAtRaw)
	m.UpdatedAt = parseSQLTime(updatedAtRaw)
	m.Sender.ProfilePhoto = photo
	m.Sender.Initials = utils.Initials(m.Sender.FullName)
	return &m, nil
}

// SoftDeleteMessage marks a message as deleted by a moderator or sender.
func SoftDeleteMessage(messageID, deletedByID int, customText ...string) error {
	bodyText := "This message was deleted."
	if len(customText) > 0 && strings.TrimSpace(customText[0]) != "" {
		bodyText = customText[0]
	}
	query := fmt.Sprintf(`
		UPDATE messages
		SET is_deleted = TRUE,
		    deleted_by = %s,
		    deleted_at = CURRENT_TIMESTAMP,
		    body = %s,
		    attachment_url = '',
		    attachment_name = '',
		    attachment_size = 0,
		    attachment_type = '',
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3))

	_, err := database.DB.Exec(query, deletedByID, bodyText, messageID)
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

// CreateCallInvitation persists a new call invitation token.
func CreateCallInvitation(inv *models.CallInvitation) error {
	query := fmt.Sprintf(`
		INSERT INTO call_invitations (conversation_id, token, call_type, created_by, expires_at, created_at)
		VALUES (%s, %s, %s, %s, %s, CURRENT_TIMESTAMP)
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3), helpers.Placeholder(4), helpers.Placeholder(5))

	res, err := database.DB.Exec(query, inv.ConversationID, inv.Token, inv.CallType, inv.CreatedBy, inv.ExpiresAt)
	if err != nil {
		return err
	}
	if lastID, lErr := res.LastInsertId(); lErr == nil && lastID > 0 {
		inv.ID = int(lastID)
	}
	return nil
}

// GetCallInvitationByToken fetches an active invitation by token.
func GetCallInvitationByToken(token string) (*models.CallInvitation, error) {
	query := fmt.Sprintf(`
		SELECT id, conversation_id, token, call_type, created_by, expires_at, created_at
		FROM call_invitations
		WHERE token = %s
	`, helpers.Placeholder(1))

	row := database.DB.QueryRow(query, token)
	var inv models.CallInvitation
	var expiresAtRaw, createdAtRaw any
	if err := row.Scan(&inv.ID, &inv.ConversationID, &inv.Token, &inv.CallType, &inv.CreatedBy, &expiresAtRaw, &createdAtRaw); err != nil {
		return nil, err
	}
	inv.ExpiresAt = parseSQLTime(expiresAtRaw)
	inv.CreatedAt = parseSQLTime(createdAtRaw)
	return &inv, nil
}

// ClearConversationHistory updates cm.cleared_at to CURRENT_TIMESTAMP for the specified user in the conversation.
func ClearConversationHistory(conversationID, userID int) error {
	query := fmt.Sprintf(`
		UPDATE conversation_members
		SET cleared_at = CURRENT_TIMESTAMP
		WHERE conversation_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))
	res, err := database.DB.Exec(query, conversationID, userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return errors.New("user is not a member of this conversation")
	}
	return nil
}

// ToggleConversationPin toggles the is_pinned state for a user in a conversation and returns the new state.
func ToggleConversationPin(conversationID, userID int) (bool, error) {
	var currentPinned bool
	queryCheck := fmt.Sprintf(`
		SELECT COALESCE(is_pinned, FALSE)
		FROM conversation_members
		WHERE conversation_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))

	err := database.DB.QueryRow(queryCheck, conversationID, userID).Scan(&currentPinned)
	if err == sql.ErrNoRows {
		return false, errors.New("user is not a member of this conversation")
	} else if err != nil {
		return false, err
	}

	newPinned := !currentPinned
	queryUpdate := fmt.Sprintf(`
		UPDATE conversation_members
		SET is_pinned = %s
		WHERE conversation_id = %s AND user_id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3))

	_, err = database.DB.Exec(queryUpdate, newPinned, conversationID, userID)
	if err != nil {
		return false, err
	}
	return newPinned, nil
}

// DeleteMessageForMe records a row in message_deletions hiding the message only for this user.
func DeleteMessageForMe(messageID, userID int) error {
	var convID int
	queryCheck := fmt.Sprintf(`
		SELECT m.conversation_id
		FROM messages m
		JOIN conversation_members cm ON cm.conversation_id = m.conversation_id AND cm.user_id = %s
		WHERE m.id = %s
	`, helpers.Placeholder(1), helpers.Placeholder(2))
	err := database.DB.QueryRow(queryCheck, userID, messageID).Scan(&convID)
	if err != nil {
		return errors.New("message not found or user not authorized")
	}

	query := fmt.Sprintf(`
		INSERT INTO message_deletions (message_id, user_id, deleted_at)
		VALUES (%s, %s, CURRENT_TIMESTAMP)
	`, helpers.Placeholder(1), helpers.Placeholder(2))
	_, err = database.DB.Exec(query, messageID, userID)
	if err != nil {
		// Unique constraint or duplicate deletion is treated as success
		return nil
	}
	return nil
}

// GetConversationMessagesForExport returns the conversation and its visible messages for export.
func GetConversationMessagesForExport(conversationID, currentUserID int) (*models.Conversation, []models.Message, error) {
	conv, err := GetConversationByID(conversationID)
	if err != nil {
		return nil, nil, err
	}

	member, err := GetConversationMember(conversationID, currentUserID)
	if err != nil || member == nil {
		return nil, nil, errors.New("unauthorized: not a conversation member")
	}

	query := fmt.Sprintf(`
		SELECT 
			m.id, m.conversation_id, m.sender_id, m.message_type, m.body,
			COALESCE(m.client_message_id, ''), m.reply_to_message_id,
			m.is_edited, m.is_deleted,
			COALESCE(m.attachment_url, ''), COALESCE(m.attachment_name, ''),
			COALESCE(m.attachment_size, 0), COALESCE(m.attachment_type, ''),
			m.expires_at,
			m.created_at, m.updated_at,
			u.id, u.full_name, u.username, COALESCE(u.profile_photo, '')
		FROM messages m
		JOIN users u ON m.sender_id = u.id
		LEFT JOIN conversation_members cm_curr ON cm_curr.conversation_id = m.conversation_id AND cm_curr.user_id = %s
		LEFT JOIN message_deletions md ON md.message_id = m.id AND md.user_id = %s
		WHERE m.conversation_id = %s
		  AND md.id IS NULL
		  AND (cm_curr.cleared_at IS NULL OR m.created_at > cm_curr.cleared_at)
		  AND (m.expires_at IS NULL OR m.expires_at > CURRENT_TIMESTAMP)
		ORDER BY m.created_at ASC, m.id ASC
	`, helpers.Placeholder(1), helpers.Placeholder(2), helpers.Placeholder(3))

	rows, err := database.DB.Query(query, currentUserID, currentUserID, conversationID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		var replyTo sql.NullInt64
		var photo string
		var expiresAtRaw any
		var createdAtRaw, updatedAtRaw any

		err := rows.Scan(
			&m.ID, &m.ConversationID, &m.SenderID, &m.MessageType, &m.Body,
			&m.ClientMessageID, &replyTo,
			&m.IsEdited, &m.IsDeleted,
			&m.AttachmentURL, &m.AttachmentName, &m.AttachmentSize, &m.AttachmentType,
			&expiresAtRaw,
			&createdAtRaw, &updatedAtRaw,
			&m.Sender.ID, &m.Sender.FullName, &m.Sender.Username, &photo,
		)
		if err != nil {
			return nil, nil, err
		}

		if t := parseSQLTime(expiresAtRaw); !t.IsZero() {
			m.ExpiresAt = &t
		}
		m.CreatedAt = parseSQLTime(createdAtRaw)
		m.UpdatedAt = parseSQLTime(updatedAtRaw)
		if replyTo.Valid {
			repID := int(replyTo.Int64)
			m.ReplyToMessageID = &repID
		}
		m.Sender.ProfilePhoto = photo
		m.Sender.Initials = utils.Initials(m.Sender.FullName)
		messages = append(messages, m)
	}

	// Hydrate quoted replies
	replyIDs := make([]int, 0)
	for _, m := range messages {
		if m.ReplyToMessageID != nil && *m.ReplyToMessageID > 0 {
			replyIDs = append(replyIDs, *m.ReplyToMessageID)
		}
	}
	if len(replyIDs) > 0 {
		replyMap, err := GetMessagesByIDs(replyIDs)
		if err == nil {
			for i := range messages {
				if messages[i].ReplyToMessageID != nil {
					if rep, ok := replyMap[*messages[i].ReplyToMessageID]; ok {
						repCopy := rep
						messages[i].ReplyToMessage = &repCopy
					}
				}
			}
		}
	}

	return conv, messages, nil
}
