package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func GetRegistrationByNumber(number string) (models.Registration, error) {

	var reg models.Registration

	err := database.DB.QueryRow(`
		SELECT
			id,
			registration_number,
			full_name,
			local_church,
			circuit,
			checked_in
		FROM registrations
		WHERE registration_number=$1
	`, number).Scan(
		&reg.ID,
		&reg.RegistrationNumber,
		&reg.FullName,
		&reg.LocalChurch,
		&reg.Circuit,
		&reg.CheckedIn,
	)

	return reg, err
}

func MarkCheckedIn(id int) error {

	_, err := database.DB.Exec(`
		UPDATE registrations
		SET checked_in=TRUE
		WHERE id=$1
	`, id)

	return err
}
