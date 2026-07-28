package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
	"database/sql"
	"errors"
)

var (
	ErrSelfFriendRequest = errors.New("you cannot send a friend request to yourself")
	ErrAlreadyFriends = errors.New("you are already friends")
	ErrFriendRequestExists = errors.New("friend request already exists")
	ErrFriendRequestNotFound = errors.New("friend request not found")
)

func SendFriendRequest(senderID, receiverID int) error {

	if senderID == receiverID {
		return ErrSelfFriendRequest
	}

	friend, err := AreFriends(senderID, receiverID)
	if err != nil {
		return err
	}

	if friend {
		return ErrAlreadyFriends
	}

	pending, err := HasPendingRequest(senderID, receiverID)
	if err != nil {
		return err
	}

	if pending {
		return ErrFriendRequestExists
	}

	query := `
	INSERT INTO friend_requests(
		sender_id,
		receiver_id,
		status
	)
	VALUES(?,?,?)
	`

	_, err = database.DB.Exec(
		query,
		senderID,
		receiverID,
		"pending",
	)

	return err
}

func AcceptFriendRequest(requestID int) error {

	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	var senderID int
	var receiverID int

	err = tx.QueryRow(`
	SELECT
		sender_id,
		receiver_id
	FROM friend_requests
	WHERE id = ?
	AND status = 'pending'
	`,
		requestID,
	).Scan(
		&senderID,
		&receiverID,
	)

	if err == sql.ErrNoRows {
		return ErrFriendRequestNotFound
	}

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
	INSERT INTO friends(
		user1_id,
		user2_id
	)
	VALUES(?,?)
	`,
		senderID,
		receiverID,
	)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
	UPDATE friend_requests
	SET
		status='accepted',
		updated_at=CURRENT_TIMESTAMP
	WHERE id=?
	`,
		requestID,
	)

	if err != nil {
		return err
	}

	return tx.Commit()
}

func DeclineFriendRequest(requestID int) error {

	result, err := database.DB.Exec(`
	UPDATE friend_requests
	SET
		status='declined',
		updated_at=CURRENT_TIMESTAMP
	WHERE id=?
	AND status='pending'
	`,
		requestID,
	)

	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()

	if rows == 0 {
		return ErrFriendRequestNotFound
	}

	return nil
}

func CancelFriendRequest(senderID, receiverID int) error {

	result, err := database.DB.Exec(`
	UPDATE friend_requests
	SET
		status='cancelled',
		updated_at=CURRENT_TIMESTAMP
	WHERE
		sender_id=?
	AND
		receiver_id=?
	AND
		status='pending'
	`,
		senderID,
		receiverID,
	)

	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()

	if rows == 0 {
		return ErrFriendRequestNotFound
	}

	return nil
}

func GetIncomingRequests(userID int) ([]models.FriendRequest, error) {

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
		ON fr.sender_id=u.id

	WHERE
		fr.receiver_id=?
	AND
		fr.status='pending'

	ORDER BY fr.created_at DESC
	`

	rows, err := database.DB.Query(query, userID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var requests []models.FriendRequest

	for rows.Next() {

		var req models.FriendRequest

		err := rows.Scan(
			&req.ID,
			&req.SenderID,
			&req.ReceiverID,
			&req.Status,
			&req.CreatedAt,
			&req.UpdatedAt,

			&req.Sender.ID,
			&req.Sender.FullName,
			&req.Sender.Username,
			&req.Sender.ProfilePhoto,
		)

		if err != nil {
			return nil, err
		}

		requests = append(requests, req)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return requests, nil
}

func GetOutgoingRequests(userID int) ([]models.FriendRequest, error) {

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
		ON fr.receiver_id=u.id

	WHERE
		fr.sender_id=?
	AND
		fr.status='pending'

	ORDER BY fr.created_at DESC
	`

	rows, err := database.DB.Query(query, userID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var requests []models.FriendRequest

	for rows.Next() {

		var req models.FriendRequest

		err := rows.Scan(
			&req.ID,
			&req.SenderID,
			&req.ReceiverID,
			&req.Status,
			&req.CreatedAt,
			&req.UpdatedAt,

			&req.Receiver.ID,
			&req.Receiver.FullName,
			&req.Receiver.Username,
			&req.Receiver.ProfilePhoto,
		)

		if err != nil {
			return nil, err
		}

		requests = append(requests, req)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return requests, nil
}