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
			password
		FROM admins
		WHERE username = $1
	`, username).Scan(
		&admin.ID,
		&admin.Username,
		&admin.Password,
	)

	return admin, err
}
