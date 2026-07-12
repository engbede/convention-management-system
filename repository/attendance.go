package repository

import "convention-management-system/database"

func CheckInRegistration(id int) error {

	_, err := database.DB.Exec(`
		UPDATE registrations
		SET checked_in = TRUE
		WHERE id = $1
	`, id)

	return err
}
