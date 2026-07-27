package repository

import "convention-management-system/database"

func FollowUser(followerID, followingID int) error {

	query := `
	INSERT OR IGNORE INTO follows (
		follower_id,
		following_id
	)
	VALUES (?, ?)
	`

	_, err := database.DB.Exec(
		query,
		followerID,
		followingID,
	)

	return err
}

func UnfollowUser(followerID, followingID int) error {

	query := `
	DELETE FROM follows
	WHERE follower_id = ?
	  AND following_id = ?
	`

	_, err := database.DB.Exec(
		query,
		followerID,
		followingID,
	)

	return err
}

func IsFollowing(followerID, followingID int) (bool, error) {

	query := `
	SELECT COUNT(*)
	FROM follows
	WHERE follower_id = ?
	  AND following_id = ?
	`

	var count int

	err := database.DB.QueryRow(
		query,
		followerID,
		followingID,
	).Scan(&count)

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func GetFollowersCount(userID int) (int, error) {

	query := `
	SELECT COUNT(*)
	FROM follows
	WHERE following_id = ?
	`

	var count int

	err := database.DB.QueryRow(
		query,
		userID,
	).Scan(&count)

	return count, err
}

func GetFollowingCount(userID int) (int, error) {

	query := `
	SELECT COUNT(*)
	FROM follows
	WHERE follower_id = ?
	`

	var count int

	err := database.DB.QueryRow(
		query,
		userID,
	).Scan(&count)

	return count, err
}
