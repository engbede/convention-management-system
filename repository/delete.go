package repository

import "convention-management-system/database"

func DeleteRegistration(id int) error {

	_, err := database.DB.Exec(
		`DELETE FROM registrations
		 WHERE id = $1`,
		id,
	)

	return err
}