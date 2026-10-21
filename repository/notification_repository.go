package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
	"convention-management-system/utils"
)

// CreateNotification inserts a new notification for a recipient.
// Avoids duplicate unread friend request notifications.
func CreateNotification(notification models.Notification) error {
	if database.DB == nil {
		return nil
	}

	// Avoid duplicate unread friend request notifications from the same sender
	if notification.Type == "friend_request_received" || notification.Type == "friend_request" {
		var exists int
		err := database.DB.QueryRow(`
			SELECT COUNT(*) FROM notifications
			WHERE sender_id = $1 AND receiver_id = $2
			  AND type IN ('friend_request_received', 'friend_request')
			  AND is_read = FALSE
		`, notification.SenderID, notification.ReceiverID).Scan(&exists)
		if err == nil && exists > 0 {
			return nil
		}
	}

	query := `
	INSERT INTO notifications (
		sender_id,
		receiver_id,
		post_id,
		comment_id,
		type,
		message,
		is_read,
		created_at
	)
	VALUES ($1, $2, $3, $4, $5, $6, FALSE, CURRENT_TIMESTAMP)
	`

	_, err := database.DB.Exec(
		query,
		notification.SenderID,
		notification.ReceiverID,
		notification.PostID,
		notification.CommentID,
		notification.Type,
		notification.Message,
	)

	return err
}

// MarkNotificationsAsRead marks all unread notifications for a user as read.
func MarkNotificationsAsRead(userID int) error {
	if database.DB == nil {
		return nil
	}

	_, err := database.DB.Exec(`
		UPDATE notifications
		SET is_read = TRUE
		WHERE receiver_id = $1
		  AND is_read = FALSE
	`, userID)

	return err
}

// CountUnreadNotifications returns the count of unread notifications for a user.
func CountUnreadNotifications(userID int) (int, error) {
	if database.DB == nil {
		return 0, nil
	}

	query := `
	SELECT COUNT(*)
	FROM notifications
	WHERE receiver_id = $1
	AND is_read = FALSE
	`

	var count int
	err := database.DB.QueryRow(query, userID).Scan(&count)
	return count, err
}

// GetNotificationsByUser returns all notifications for a user in reverse chronological order.
func GetNotificationsByUser(userID int) ([]models.Notification, error) {
	if database.DB == nil {
		return nil, nil
	}

	query := `
	SELECT
		n.id,
		n.sender_id,
		n.receiver_id,
		n.post_id,
		n.comment_id,
		n.type,
		n.message,
		n.is_read,
		n.created_at,
		u.id,
		u.full_name,
		u.username,
		COALESCE(u.profile_photo, '')
	FROM notifications n
	LEFT JOIN users u
	ON n.sender_id = u.id
	WHERE n.receiver_id = $1
	ORDER BY n.created_at DESC
	`

	rows, err := database.DB.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []models.Notification
	for rows.Next() {
		var notification models.Notification
		err := rows.Scan(
			&notification.ID,
			&notification.SenderID,
			&notification.ReceiverID,
			&notification.PostID,
			&notification.CommentID,
			&notification.Type,
			&notification.Message,
			&notification.IsRead,
			&notification.CreatedAt,
			&notification.Sender.ID,
			&notification.Sender.FullName,
			&notification.Sender.Username,
			&notification.Sender.ProfilePhoto,
		)
		if err != nil {
			return nil, err
		}

		notification.Sender.Initials = utils.Initials(notification.Sender.FullName)
		notifications = append(notifications, notification)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return notifications, nil
}
