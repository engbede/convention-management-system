package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
	"convention-management-system/utils"
)

func GetDiscoverUsers(currentUserID int) ([]models.User, error) {

	query := `
	SELECT
		id,
		full_name,
		username,
		bio,
		location,
		profile_photo
	FROM users
	WHERE id != ?
	ORDER BY full_name
	`

	rows, err := database.DB.Query(query, currentUserID)
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
			&user.Bio,
			&user.Location,
			&user.ProfilePhoto,
		)

		if err != nil {
			return nil, err
		}

		user.Initials = utils.Initials(user.FullName)

		// Already friends?
		areFriends, _ := AreFriends(currentUserID, user.ID)

		if areFriends {

			user.FriendStatus = "friends"

		} else {

			// I sent request?
			sent, _ := HasPendingRequest(currentUserID, user.ID)

			// They sent request?
			received, _ := HasPendingRequest(user.ID, currentUserID)

			switch {

			case sent:
				user.FriendStatus = "pending"

			case received:
				user.FriendStatus = "received"

			default:
				user.FriendStatus = "none"
			}
		}

		users = append(users, user)
	}

	return users, nil
}