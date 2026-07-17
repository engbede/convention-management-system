package repository

import "convention-management-system/database"

func DeleteDocument(id int) error {

	_, err := database.DB.Exec(
		"DELETE FROM documents WHERE id = $1",
		id,
	)

	return err
}
