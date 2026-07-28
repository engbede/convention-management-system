package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func CreateNotification(notification models.Notification) error {

	query := `
	INSERT INTO notifications (
		sender_id,
		receiver_id,
		post_id,
		comment_id,
		type,
		message
	)
	VALUES (?, ?, ?, ?, ?, ?)
	`
	println("================================")
	println("Creating notification")
	println("SenderID   =", notification.SenderID)
	println("ReceiverID =", notification.ReceiverID)
	println("Type       =", notification.Type)
	println("================================")

	_, err := database.DB.Exec(
		query,
		notification.SenderID,
		notification.ReceiverID,
		notification.PostID,
		notification.CommentID,
		notification.Type,
		notification.Message,
	)

	if err != nil {
		println("================================")
		println("CreateNotification failed:")
		println(err.Error())
		println("================================")
	} else {
		println("Notification inserted successfully")
	}

	return err
}

func CountUnreadNotifications(userID int) (int, error) {

	query := `
	SELECT COUNT(*)
	FROM notifications
	WHERE receiver_id = ?
	AND is_read = 0
	`

	var count int

	err := database.DB.QueryRow(
		query,
		userID,
	).Scan(&count)

	return count, err
}

func GetNotificationsByUser(userID int) ([]models.Notification, error) {

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
		u.profile_photo
	FROM notifications n
	LEFT JOIN users u
	ON n.sender_id = u.id
	WHERE n.receiver_id = ?
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
			println("================================")
			println("Scan error:")
			println(err.Error())
			println("================================")
			return nil, err
		}

		notifications = append(notifications, notification)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	println("================================")
	println("Notifications found:", len(notifications))
	println("================================")

	return notifications, nil
}
