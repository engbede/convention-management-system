package repository

import "convention-management-system/database"

func SaveQRCode(
	id int,
	qrPath string,
) error {

	_, err := database.DB.Exec(
		`
		UPDATE registrations
		SET qr_code=$1
		WHERE id=$2
		`,
		qrPath,
		id,
	)

	return err
}
