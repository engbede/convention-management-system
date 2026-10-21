package services

import (
	"database/sql"
	"convention-management-system/database"
	"convention-management-system/models"
)

// NotifyFriendRequest records a persisted notification and emits a real-time WebSocket event
// to the recipient when a friend request is sent.
func NotifyFriendRequest(senderID, receiverID int) error {
	var senderUsername, senderFullName string
	if database.DB != nil {
		_ = database.DB.QueryRow(`SELECT username, full_name FROM users WHERE id = $1`, senderID).Scan(&senderUsername, &senderFullName)
	}

	notification := models.Notification{
		SenderID:   senderID,
		ReceiverID: receiverID,
		Type:       "friend_request_received",
		Message:    "sent you a friend request.",
	}

	if err := CreateNotification(notification); err != nil {
		return err
	}

	// Real-time WebSocket event to active recipient sessions
	GlobalHub.BroadcastToUser(receiverID, WSEvent{
		Type:     "friend_request_received",
		UserID:   senderID,
		Username: senderUsername,
		Data: map[string]any{
			"sender_id":       senderID,
			"sender_username": senderUsername,
			"sender_name":     senderFullName,
			"message":         "@" + senderUsername + " sent you a friend request.",
		},
	})

	return nil
}

// NotifyFriendAccepted records a persisted notification and emits a real-time WebSocket event
// to the requester when their friend request is accepted.
func NotifyFriendAccepted(accepterID, requesterID int) error {
	var accepterUsername, accepterFullName string
	if database.DB != nil {
		_ = database.DB.QueryRow(`SELECT username, full_name FROM users WHERE id = $1`, accepterID).Scan(&accepterUsername, &accepterFullName)
	}

	notification := models.Notification{
		SenderID:   accepterID,
		ReceiverID: requesterID,
		Type:       "friend_request_accepted",
		Message:    "accepted your friend request.",
	}

	if err := CreateNotification(notification); err != nil {
		return err
	}

	// Real-time WebSocket event to active requester sessions
	GlobalHub.BroadcastToUser(requesterID, WSEvent{
		Type:     "friend_request_accepted",
		UserID:   accepterID,
		Username: accepterUsername,
		Data: map[string]any{
			"accepted_by_id":       accepterID,
			"accepted_by_username": accepterUsername,
			"accepted_by_name":     accepterFullName,
			"message":              "@" + accepterUsername + " accepted your friend request.",
		},
	})

	return nil
}

// Unused dummy import anchor to prevent compiler errors if database is mock
var _ = sql.ErrNoRows
