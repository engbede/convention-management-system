package repository

import "convention-management-system/database"

func CheckInRegistration(id int) error {

	_, err := database.DB.Exec(`
		UPDATE registrations
		SET checked_in = 1
		WHERE id = ?
	`, id)

	return err
}
