package repository

import "convention-management-system/database"

func SaveRegistrationNumber(
	id int,
	number string,
) error {

	_, err := database.DB.Exec(
		`
		UPDATE registrations
		SET registration_number=$1
		WHERE id=$2
		`,
		number,
		id,
	)

	return err
}
