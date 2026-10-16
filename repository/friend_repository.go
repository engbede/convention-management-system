package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
	"convention-management-system/utils"
)

// AreFriends checks whether two users are already friends.
func AreFriends(user1ID, user2ID int) (bool, error) {

	query := `
	SELECT COUNT(*)
	FROM friends
	WHERE
		(user1_id = $1 AND user2_id = $2)
		OR
		(user1_id = $3 AND user2_id = $4)
	`

	var count int

	err := database.DB.QueryRow(
		query,
		user1ID,
		user2ID,
		user2ID,
		user1ID,
	).Scan(&count)

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// HasPendingRequest checks if a pending friend request already exists.
func HasPendingRequest(senderID, receiverID int) (bool, error) {

	query := `
	SELECT COUNT(*)
	FROM friend_requests
	WHERE
		sender_id = $1
	AND
		receiver_id = $2
	AND
		status = 'pending'
	`

	var count int

	err := database.DB.QueryRow(
		query,
		senderID,
		receiverID,
	).Scan(&count)

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetFriends returns every friend of the specified user.
func GetFriends(userID int) ([]models.Friend, error) {

	query := `
	SELECT
		f.id,

		f.user1_id,
		f.user2_id,

		f.created_at,

		u.id,
		u.full_name,
		u.username,
		u.profile_photo

	FROM friends f

	INNER JOIN users u
	ON u.id =
		CASE
			WHEN f.user1_id = $1
			THEN f.user2_id
			ELSE f.user1_id
		END

	WHERE
		f.user1_id = $2
	OR
		f.user2_id = $3

	ORDER BY u.full_name ASC
	`

	rows, err := database.DB.Query(
		query,
		userID,
		userID,
		userID,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var friends []models.Friend

	for rows.Next() {

		var friend models.Friend

		err := rows.Scan(
			&friend.ID,
			&friend.User1ID,
			&friend.User2ID,
			&friend.CreatedAt,

			&friend.User.ID,
			&friend.User.FullName,
			&friend.User.Username,
			&friend.User.ProfilePhoto,
		)

		if err != nil {
			return nil, err
		}

		friend.User.Initials = utils.Initials(
			friend.User.FullName,
		)

		friends = append(friends, friend)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return friends, nil
}

// RemoveFriend deletes a friendship between two users.
func RemoveFriend(user1ID, user2ID int) error {

	query := `
	DELETE FROM friends
	WHERE
		(user1_id = $1 AND user2_id = $2)
	OR
		(user1_id = $3 AND user2_id = $4)
	`

	_, err := database.DB.Exec(
		query,
		user1ID,
		user2ID,
		user2ID,
		user1ID,
	)

	return err
}

// GetFriendCount returns the total number of friends a user has.
func GetFriendCount(userID int) (int, error) {

	query := `
	SELECT COUNT(*)
	FROM friends
	WHERE
		user1_id = $1
	OR
		user2_id = $2
	`

	var count int

	err := database.DB.QueryRow(
		query,
		userID,
		userID,
	).Scan(&count)

	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetMutualFriends returns mutual friends between two users.
func GetMutualFriends(user1ID, user2ID int) ([]models.User, error) {

	query := `
	SELECT DISTINCT
		u.id,
		u.full_name,
		u.username,
		u.profile_photo

	FROM users u

	WHERE u.id IN (

		SELECT
			CASE
				WHEN user1_id = $1
				THEN user2_id
				ELSE user1_id
			END
		FROM friends
		WHERE user1_id = $2
		   OR user2_id = $3

	)

	AND u.id IN (

		SELECT
			CASE
				WHEN user1_id = $4
				THEN user2_id
				ELSE user1_id
			END
		FROM friends
		WHERE user1_id = $5
		   OR user2_id = $6

	)

	ORDER BY u.full_name
	`

	rows, err := database.DB.Query(
		query,

		user1ID,
		user1ID,
		user1ID,

		user2ID,
		user2ID,
		user2ID,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var users []models.User

	for rows.Next() {

		var user models.User

		err := rows.Scan(
			&user.ID,
			&user.FullName,
			&user.Username,
			&user.ProfilePhoto,
		)

		if err != nil {
			return nil, err
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func GetPendingFriendRequests(userID int) ([]models.FriendRequest, error) {

	query := `
	SELECT
		fr.id,
		fr.sender_id,
		fr.receiver_id,
		fr.status,
		fr.created_at,

		u.id,
		u.full_name,
		u.username,
		u.profile_photo

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

	return requests, nil
}
