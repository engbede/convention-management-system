package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func GetAdminByUsername(username string) (models.Admin, error) {

	var admin models.Admin

	err := database.DB.QueryRow(`
		SELECT
			id,
			username,
			password,
			COALESCE(email, '')
		FROM admins
		WHERE username = $1
	`, username).Scan(
		&admin.ID,
		&admin.Username,
		&admin.Password,
		&admin.Email,
	)

	return admin, err
}

func GetAdminByID(id int) (models.Admin, error) {
	var admin models.Admin

	err := database.DB.QueryRow(`
		SELECT
			id,
			username,
			password,
			COALESCE(email, '')
		FROM admins
		WHERE id = $1
	`, id).Scan(
		&admin.ID,
		&admin.Username,
		&admin.Password,
		&admin.Email,
	)

	return admin, err
}

func GetAdminByEmail(email string) (models.Admin, error) {
	var admin models.Admin

	err := database.DB.QueryRow(`
		SELECT
			id,
			username,
			password,
			COALESCE(email, '')
		FROM admins
		WHERE LOWER(email) = LOWER($1)
		LIMIT 1
	`, email).Scan(
		&admin.ID,
		&admin.Username,
		&admin.Password,
		&admin.Email,
	)

	return admin, err
}

func UpdateAdminPassword(id int, newHashedPassword string) error {
	_, err := database.DB.Exec(`
		UPDATE admins
		SET password = $1
		WHERE id = $2
	`, newHashedPassword, id)
	return err
}

func UpdateAdminEmail(id int, email string) error {
	_, err := database.DB.Exec(`
		UPDATE admins
		SET email = $1
		WHERE id = $2
	`, email, id)
	return err
}

