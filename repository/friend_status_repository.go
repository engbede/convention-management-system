package repository

import "convention-management-system/database"

const (
	FriendStatusNone     = "none"
	FriendStatusPending  = "pending"
	FriendStatusReceived = "received"
	FriendStatusFriends  = "friends"
)

// GetFriendStatus returns the relationship between viewer and profile owner.
func GetFriendStatus(viewerID, profileUserID int) (string, error) {

	if viewerID == profileUserID {
		return "self", nil
	}

	// Already friends?
	areFriends, err := AreFriends(viewerID, profileUserID)
	if err != nil {
		return "", err
	}

	if areFriends {
		return FriendStatusFriends, nil
	}

	// Viewer already sent request?
	var count int

	err = database.DB.QueryRow(`
		SELECT COUNT(*)
		FROM friend_requests
		WHERE sender_id = ?
		  AND receiver_id = ?
		  AND status='pending'
	`, viewerID, profileUserID).Scan(&count)

	if err != nil {
		return "", err
	}

	if count > 0 {
		return FriendStatusPending, nil
	}

	// Profile owner sent request?
	err = database.DB.QueryRow(`
		SELECT COUNT(*)
		FROM friend_requests
		WHERE sender_id = ?
		  AND receiver_id = ?
		  AND status='pending'
	`, profileUserID, viewerID).Scan(&count)

	if err != nil {
		return "", err
	}

	if count > 0 {
		return FriendStatusReceived, nil
	}

	return FriendStatusNone, nil
}