package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
	"strings"
)

func GetUserProfile(userID int) (*models.User, error) {

	query := `
	SELECT
		id,
		full_name,
		username,
		email,
		phone,
		role,
		bio,
		gender,
		birth_date,
		location,
		website,
		profile_photo,
		cover_photo,
		is_verified,
		is_active,
		followers,
		following,
		created_at,
		updated_at
	FROM users
	WHERE id = ?
	`

	var user models.User

	err := database.DB.QueryRow(query, userID).Scan(
		&user.ID,
		&user.FullName,
		&user.Username,
		&user.Email,
		&user.Phone,
		&user.Role,
		&user.Bio,
		&user.Gender,
		&user.BirthDate,
		&user.Location,
		&user.Website,
		&user.ProfilePhoto,
		&user.CoverPhoto,
		&user.IsVerified,
		&user.IsActive,
		&user.Followers,
		&user.Following,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Generate initials from the full name.
	parts := strings.Fields(user.FullName)

	switch len(parts) {

	case 0:
		user.Initials = "?"

	case 1:
		user.Initials = strings.ToUpper(parts[0][:1])

	default:
		user.Initials = strings.ToUpper(
			parts[0][:1] + parts[len(parts)-1][:1],
		)
	}

	return &user, nil
}

func UpdateUserProfile(user *models.User) error {

	query := `
	UPDATE users
	SET
		full_name = ?,
		bio = ?,
		location = ?,
		website = ?,
		gender = ?,
		birth_date = ?,
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?
	`

	_, err := database.DB.Exec(
		query,
		user.FullName,
		user.Bio,
		user.Location,
		user.Website,
		user.Gender,
		user.BirthDate,
		user.ID,
	)

	return err
}
func UpdateProfilePhoto(
	userID int,
	filename string,
) error {

	query := `
	UPDATE users
	SET
		profile_photo = ?,
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?
	`

	_, err := database.DB.Exec(
		query,
		filename,
		userID,
	)

	return err
}

func UpdateCoverPhoto(
	userID int,
	filename string,
) error {

	query := `
	UPDATE users
	SET
		cover_photo = ?,
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?
	`

	_, err := database.DB.Exec(
		query,
		filename,
		userID,
	)

	return err
}
