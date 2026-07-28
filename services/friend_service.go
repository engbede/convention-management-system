package services

import (
	"convention-management-system/database"
	"convention-management-system/models"
	"convention-management-system/repository"
)

func GetFriendCount(userID int) (int, error) {
	return repository.GetFriendCount(userID)
}

func GetFriends(userID int) ([]models.Friend, error) {

	return repository.GetFriends(
		userID,
	)
}

// GetPendingFriendRequests returns incoming friend requests.
func GetPendingFriendRequests(userID int) ([]models.FriendRequest, error) {

	query := `
	SELECT
		fr.id,
		fr.sender_id,
		fr.receiver_id,
		fr.status,
		fr.created_at,
		fr.updated_at,

		u.id,
		u.full_name,
		u.username,
		u.profile_photo

	FROM friend_requests fr

	INNER JOIN users u
	ON fr.sender_id = u.id

	WHERE
		fr.receiver_id = ?
	AND
		fr.status = 'pending'

	ORDER BY fr.created_at DESC
	`

	rows, err := database.DB.Query(
		query,
		userID,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var requests []models.FriendRequest

	for rows.Next() {

		var request models.FriendRequest

		err := rows.Scan(
			&request.ID,
			&request.SenderID,
			&request.ReceiverID,
			&request.Status,
			&request.CreatedAt,
			&request.UpdatedAt,

			&request.Sender.ID,
			&request.Sender.FullName,
			&request.Sender.Username,
			&request.Sender.ProfilePhoto,
		)

		if err != nil {
			return nil, err
		}

		requests = append(
			requests,
			request,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return requests, nil
}

// GetSentFriendRequests returns outgoing friend requests.
func GetSentFriendRequests(userID int) ([]models.FriendRequest, error) {

	query := `
	SELECT
		fr.id,
		fr.sender_id,
		fr.receiver_id,
		fr.status,
		fr.created_at,
		fr.updated_at,

		u.id,
		u.full_name,
		u.username,
		u.profile_photo

	FROM friend_requests fr

	INNER JOIN users u
	ON fr.receiver_id = u.id

	WHERE
		fr.sender_id = ?
	AND
		fr.status = 'pending'

	ORDER BY fr.created_at DESC
	`

	rows, err := database.DB.Query(
		query,
		userID,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var requests []models.FriendRequest

	for rows.Next() {

		var request models.FriendRequest

		err := rows.Scan(
			&request.ID,
			&request.SenderID,
			&request.ReceiverID,
			&request.Status,
			&request.CreatedAt,
			&request.UpdatedAt,

			&request.Receiver.ID,
			&request.Receiver.FullName,
			&request.Receiver.Username,
			&request.Receiver.ProfilePhoto,
		)

		if err != nil {
			return nil, err
		}

		requests = append(
			requests,
			request,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return requests, nil
}

func SendFriendRequest(senderID, receiverID int) error {

	err := repository.SendFriendRequest(
		senderID,
		receiverID,
	)

	if err != nil {
		return err
	}

	return NotifyFriendRequest(
		senderID,
		receiverID,
	)
}

func AcceptFriendRequest(requestID, senderID, receiverID int) error {

	err := repository.AcceptFriendRequest(
		requestID,
	)

	if err != nil {
		return err
	}

	return NotifyFriendAccepted(
		receiverID,
		senderID,
	)
}

func DeclineFriendRequest(requestID int) error {

	return repository.DeclineFriendRequest(
		requestID,
	)
}

func CancelFriendRequest(senderID, receiverID int) error {

	return repository.CancelFriendRequest(
		senderID,
		receiverID,
	)
}

func RemoveFriend(user1ID, user2ID int) error {

	return repository.RemoveFriend(
		user1ID,
		user2ID,
	)
}

func GetFriendStatus(currentUserID, otherUserID int) (string, error) {
	return repository.GetFriendStatus(currentUserID, otherUserID)
}
