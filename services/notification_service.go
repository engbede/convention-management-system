package services

import (
	"convention-management-system/models"
	"convention-management-system/repository"
)

func CreateNotification(notification models.Notification) error {
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
