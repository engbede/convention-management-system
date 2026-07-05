package repository

import "convention-management-system/database"

func DeleteNotice(id int) error {

	_, err := database.DB.Exec(`
		DELETE FROM notices
		WHERE id=$1
	`, id)

	return err
}
