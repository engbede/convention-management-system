package repository

import (
	"database/sql"
	"errors"

	"convention-management-system/database"
	"convention-management-system/models"
	"convention-management-system/utils"
)

var (
	ErrSelfFriendRequest     = errors.New("you cannot send a friend request to yourself")
	ErrAlreadyFriends        = errors.New("you are already friends")
	ErrFriendRequestExists   = errors.New("friend request already exists")
	ErrFriendRequestNotFound = errors.New("friend request not found")
	ErrUnauthorizedRequest   = errors.New("unauthorized friend request action")
	ErrUserNotFound          = errors.New("user not found")
)

// SendFriendRequestResult encapsulates the outcome of sending a friend request.
type SendFriendRequestResult struct {
	RequestID        int
	IsMutualFriend   bool
	SenderUsername   string
	ReceiverUsername string
}

// SendFriendRequest creates or reactivates a friend request from senderID to receiverID.
// If receiverID has already sent a pending request to senderID, it atomically resolves
// the reciprocal request, establishing mutual friendship immediately.
func SendFriendRequest(senderID, receiverID int) (*SendFriendRequestResult, error) {
	if senderID == receiverID {
		return nil, ErrSelfFriendRequest
	}
	if senderID <= 0 || receiverID <= 0 {
		return nil, ErrUserNotFound
	}

	// 1. Verify receiver exists
	var receiverUsername string
	err := database.DB.QueryRow(`SELECT username FROM users WHERE id = $1`, receiverID).Scan(&receiverUsername)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	} else if err != nil {
		return nil, err
	}

	// 2. Fetch sender username
	var senderUsername string
	_ = database.DB.QueryRow(`SELECT username FROM users WHERE id = $1`, senderID).Scan(&senderUsername)

	// 3. Are they already friends?
	friend, err := AreFriends(senderID, receiverID)
	if err != nil {
		return nil, err
	}
	if friend {
		return nil, ErrAlreadyFriends
	}

	// 4. Has sender already sent an active pending request to receiver?
	pending, err := HasPendingRequest(senderID, receiverID)
	if err != nil {
		return nil, err
	}
	if pending {
		return nil, ErrFriendRequestExists
	}

	// 5. Check reciprocal request: Did receiver already send a pending request to sender?
	var reciprocalID int
	rErr := database.DB.QueryRow(`
		SELECT id
		FROM friend_requests
		WHERE sender_id = $1 AND receiver_id = $2 AND status = 'pending'
	`, receiverID, senderID).Scan(&reciprocalID)

	if rErr == nil && reciprocalID > 0 {
		// Reciprocal request exists! Auto-accept into mutual friendship atomically
		tx, txErr := database.DB.Begin()
		if txErr != nil {
			return nil, txErr
		}
		defer tx.Rollback()

		// Mark reciprocal request accepted
		_, txErr = tx.Exec(`
			UPDATE friend_requests
			SET status = 'accepted', updated_at = CURRENT_TIMESTAMP
			WHERE id = $1
		`, reciprocalID)
		if txErr != nil {
			return nil, txErr
		}

		// Also upsert sender's request as accepted
		_, txErr = tx.Exec(`
			INSERT INTO friend_requests(sender_id, receiver_id, status, created_at, updated_at)
			VALUES($1, $2, 'accepted', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT (sender_id, receiver_id)
			DO UPDATE SET status = 'accepted', updated_at = CURRENT_TIMESTAMP
		`, senderID, receiverID)
		if txErr != nil {
			return nil, txErr
		}

		// Create canonical friendship
		u1, u2 := senderID, receiverID
		if u1 > u2 {
			u1, u2 = u2, u1
		}
		_, txErr = tx.Exec(`
			INSERT INTO friends(user1_id, user2_id, created_at)
			VALUES($1, $2, CURRENT_TIMESTAMP)
			ON CONFLICT (user1_id, user2_id) DO NOTHING
		`, u1, u2)
		if txErr != nil {
			return nil, txErr
		}

		if cErr := tx.Commit(); cErr != nil {
			return nil, cErr
		}

		return &SendFriendRequestResult{
			RequestID:        reciprocalID,
			IsMutualFriend:   true,
			SenderUsername:   senderUsername,
			ReceiverUsername: receiverUsername,
		}, nil
	}

	// 6. Normal request insertion / reactivate previous cancelled or declined request
	_, err = database.DB.Exec(`
		INSERT INTO friend_requests(sender_id, receiver_id, status, created_at, updated_at)
		VALUES($1, $2, 'pending', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (sender_id, receiver_id)
		DO UPDATE SET
			status = 'pending',
			updated_at = CURRENT_TIMESTAMP
	`, senderID, receiverID)
	if err != nil {
		return nil, err
	}

	var requestID int
	_ = database.DB.QueryRow(`
		SELECT id FROM friend_requests
		WHERE sender_id = $1 AND receiver_id = $2
	`, senderID, receiverID).Scan(&requestID)

	return &SendFriendRequestResult{
		RequestID:        requestID,
		IsMutualFriend:   false,
		SenderUsername:   senderUsername,
		ReceiverUsername: receiverUsername,
	}, nil
}

// AcceptFriendRequest atomically transitions a pending request to accepted and creates the friendship.
// It verifies that receiverID matches the request's intended recipient to prevent IDOR attacks.
func AcceptFriendRequest(requestID, receiverID int) (senderID int, senderUsername, receiverUsername string, err error) {
	tx, err := database.DB.Begin()
	if err != nil {
		return 0, "", "", err
	}
	defer tx.Rollback()

	var actualSenderID, actualReceiverID int
	var status string
	err = tx.QueryRow(`
		SELECT sender_id, receiver_id, status
		FROM friend_requests
		WHERE id = $1
	`, requestID).Scan(&actualSenderID, &actualReceiverID, &status)

	if err == sql.ErrNoRows {
		return 0, "", "", ErrFriendRequestNotFound
	}
	if err != nil {
		return 0, "", "", err
	}

	if actualReceiverID != receiverID {
		return 0, "", "", ErrUnauthorizedRequest
	}
	if status != "pending" {
		return 0, "", "", errors.New("friend request is no longer pending")
	}

	// Canonicalize user IDs (user1_id < user2_id) to prevent duplicate reverse friendships
	u1, u2 := actualSenderID, actualReceiverID
	if u1 > u2 {
		u1, u2 = u2, u1
	}

	_, err = tx.Exec(`
		INSERT INTO friends(user1_id, user2_id, created_at)
		VALUES($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (user1_id, user2_id) DO NOTHING
	`, u1, u2)
	if err != nil {
		return 0, "", "", err
	}

	_, err = tx.Exec(`
		UPDATE friend_requests
		SET status = 'accepted', updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`, requestID)
	if err != nil {
		return 0, "", "", err
	}

	// Also mark any reciprocal request between them as accepted
	_, _ = tx.Exec(`
		UPDATE friend_requests
		SET status = 'accepted', updated_at = CURRENT_TIMESTAMP
		WHERE sender_id = $1 AND receiver_id = $2 AND status = 'pending'
	`, actualReceiverID, actualSenderID)

	if err = tx.Commit(); err != nil {
		return 0, "", "", err
	}

	_ = database.DB.QueryRow(`SELECT username FROM users WHERE id = $1`, actualSenderID).Scan(&senderUsername)
	_ = database.DB.QueryRow(`SELECT username FROM users WHERE id = $1`, actualReceiverID).Scan(&receiverUsername)

	return actualSenderID, senderUsername, receiverUsername, nil
}

// DeclineFriendRequest marks a pending request as declined. Only the intended recipient may decline.
func DeclineFriendRequest(requestID, receiverID int) error {
	var actualReceiverID int
	var status string
	err := database.DB.QueryRow(`
		SELECT receiver_id, status
		FROM friend_requests
		WHERE id = $1
	`, requestID).Scan(&actualReceiverID, &status)

	if err == sql.ErrNoRows {
		return ErrFriendRequestNotFound
	}
	if err != nil {
		return err
	}

	if actualReceiverID != receiverID {
		return ErrUnauthorizedRequest
	}
	if status != "pending" {
		return errors.New("friend request is no longer pending")
	}

	res, err := database.DB.Exec(`
		UPDATE friend_requests
		SET status = 'declined', updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND receiver_id = $2 AND status = 'pending'
	`, requestID, receiverID)
	if err != nil {
		return err
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrFriendRequestNotFound
	}

	return nil
}

// CancelFriendRequest marks an outgoing pending request as cancelled by request ID.
// Only the original sender may cancel their request.
func CancelFriendRequest(requestID, senderID int) error {
	var actualSenderID int
	var status string
	err := database.DB.QueryRow(`
		SELECT sender_id, status
		FROM friend_requests
		WHERE id = $1
	`, requestID).Scan(&actualSenderID, &status)

	if err == sql.ErrNoRows {
		return ErrFriendRequestNotFound
	}
	if err != nil {
		return err
	}

	if actualSenderID != senderID {
		return ErrUnauthorizedRequest
	}
	if status != "pending" {
		return errors.New("friend request is no longer pending")
	}

	res, err := database.DB.Exec(`
		UPDATE friend_requests
		SET status = 'cancelled', updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND sender_id = $2 AND status = 'pending'
	`, requestID, senderID)
	if err != nil {
		return err
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrFriendRequestNotFound
	}

	return nil
}

// CancelFriendRequestByUsers marks an outgoing pending request from sender to receiver as cancelled.
func CancelFriendRequestByUsers(senderID, receiverID int) error {
	res, err := database.DB.Exec(`
		UPDATE friend_requests
		SET status = 'cancelled', updated_at = CURRENT_TIMESTAMP
		WHERE sender_id = $1 AND receiver_id = $2 AND status = 'pending'
	`, senderID, receiverID)
	if err != nil {
		return err
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrFriendRequestNotFound
	}

	return nil
}

// GetIncomingRequests returns all pending incoming requests for a user.
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
		COALESCE(u.profile_photo, '')

	FROM friend_requests fr

	INNER JOIN users u
		ON fr.sender_id = u.id

	WHERE
		fr.receiver_id = $1
	AND
		fr.status = 'pending'

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
		req.Sender.Initials = utils.Initials(req.Sender.FullName)
		requests = append(requests, req)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return requests, nil
}

// GetOutgoingRequests returns all pending outgoing requests sent by a user.
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
		COALESCE(u.profile_photo, '')

	FROM friend_requests fr

	INNER JOIN users u
		ON fr.receiver_id = u.id

	WHERE
		fr.sender_id = $1
	AND
		fr.status = 'pending'

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
		req.Receiver.Initials = utils.Initials(req.Receiver.FullName)
		requests = append(requests, req)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return requests, nil
}
