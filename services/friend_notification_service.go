package services

import (
	"convention-management-system/models"
	"convention-management-system/repository"
)

func NotifyFriendRequest(senderID, receiverID int) error {

	notification := models.Notification{
		SenderID:   senderID,
		ReceiverID: receiverID,
		Type:       "friend_request",
		Message:    "sent you a friend request.",
	}

	return repository.CreateNotification(notification)
}

func NotifyFriendAccepted(senderID, receiverID int) error {

	notification := models.Notification{
		SenderID:   senderID,
		ReceiverID: receiverID,
		Type:       "friend_accept",
		Message:    "accepted your friend request.",
	}

	return repository.CreateNotification(notification)
}
