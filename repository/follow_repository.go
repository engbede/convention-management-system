package repository

import "convention-management-system/database"

func FollowUser(followerID, followingID int) error {

	query := `
	INSERT INTO follows (
		follower_id,
		following_id
	)
	VALUES ($1, $2)
	ON CONFLICT DO NOTHING
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
	WHERE follower_id = $1
	  AND following_id = $2
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
	WHERE follower_id = $1
	  AND following_id = $2
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
	WHERE following_id = $1
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
	WHERE follower_id = $1
	`

	var count int

	err := database.DB.QueryRow(
		query,
		userID,
	).Scan(&count)

	return count, err
}
