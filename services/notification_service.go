package services

import (
	"convention-management-system/models"
	"convention-management-system/repository"
)

func CreateNotification(notification models.Notification) error {
	// Silently suppress notification delivery if communication is blocked between sender and receiver
	if notification.SenderID > 0 && notification.ReceiverID > 0 && notification.SenderID != notification.ReceiverID {
		blocked, err := repository.IsCommunicationBlocked(notification.SenderID, notification.ReceiverID)
		if err == nil && blocked {
			return nil
		}
	}
	return repository.CreateNotification(notification)
}

func GetNotificationsByUser(userID int) ([]models.Notification, error) {
	return repository.GetNotificationsByUser(userID)
}

func CountUnreadNotifications(userID int) (int, error) {
	return repository.CountUnreadNotifications(userID)
}

func NotifyFollow(senderID, receiverID int) error {

	notification := models.Notification{
		SenderID:   senderID,
		ReceiverID: receiverID,
		Type:       "follow",
		Message:    "started following you.",
	}

	return CreateNotification(notification)
}
