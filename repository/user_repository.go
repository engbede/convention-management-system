package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func CreateUser(user *models.User) error {

	query := `
	INSERT INTO users (
		full_name,
		username,
		email,
		phone,
		password_hash,
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
		following
	)
	VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`

	_, err := database.DB.Exec(
		query,
		user.FullName,
		user.Username,
		user.Email,
		user.Phone,
		user.PasswordHash,
		user.Role,
		user.Bio,
		user.Gender,
		user.BirthDate,
		user.Location,
		user.Website,
		user.ProfilePhoto,
		user.CoverPhoto,
		user.IsVerified,
		user.IsActive,
		user.Followers,
		user.Following,
	)

	return err
}

func GetUserByUsername(username string) (*models.User, error) {

	query := `
	SELECT
		id,
		full_name,
		username,
		email,
		phone,
		password_hash,
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
	WHERE username = ?
	`

	var user models.User

	err := database.DB.QueryRow(query, username).Scan(
		&user.ID,
		&user.FullName,
		&user.Username,
		&user.Email,
		&user.Phone,
		&user.PasswordHash,
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

	return &user, nil
}

func GetUserByID(id int) (models.User, error) {

	query := `
	SELECT
		id,
		full_name,
		username,
		email,
		phone,
		password_hash,
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

	err := database.DB.QueryRow(query, id).Scan(
		&user.ID,
		&user.FullName,
		&user.Username,
		&user.Email,
		&user.Phone,
		&user.PasswordHash,
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

	return user, err
}
