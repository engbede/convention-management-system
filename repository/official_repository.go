package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func CreateOfficial(official models.Official) error {

	query := `
	INSERT INTO officials(
		full_name,
		gender,
		phone,
		email,
		circuit,
		local_church,
		position,
		department,
		status,
		photo
	)
	VALUES(
		$1,$2,$3,$4,$5,
		$6,$7,$8,$9,$10
	)
	`

	_, err := database.DB.Exec(
		query,
		official.FullName,
		official.Gender,
		official.Phone,
		official.Email,
		official.Circuit,
		official.LocalChurch,
		official.Position,
		official.Department,
		official.Status,
		official.Photo,
	)

	return err
}
func GetAllOfficials() ([]models.Official, error) {

	rows, err := database.DB.Query(`
		SELECT
			id,
			full_name,
			gender,
			phone,
			email,
			circuit,
			local_church,
			position,
			department,
			status,
			photo,
			created_at
		FROM officials
		ORDER BY full_name ASC
	`)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var officials []models.Official

	for rows.Next() {

		var official models.Official

		err := rows.Scan(
			&official.ID,
			&official.FullName,
			&official.Gender,
			&official.Phone,
			&official.Email,
			&official.Circuit,
			&official.LocalChurch,
			&official.Position,
			&official.Department,
			&official.Status,
			&official.Photo,
			&official.CreatedAt,
		)

		if err != nil {
			return nil, err
		}

		officials = append(
			officials,
			official,
		)
	}

	return officials, nil
}
func GetOfficialByID(id int) (models.Official, error) {

	var official models.Official

	err := database.DB.QueryRow(`
		SELECT
			id,
			full_name,
			gender,
			phone,
			email,
			circuit,
			local_church,
			position,
			department,
			status,
			photo,
			created_at
		FROM officials
		WHERE id=$1
	`, id).Scan(
		&official.ID,
		&official.FullName,
		&official.Gender,
		&official.Phone,
		&official.Email,
		&official.Circuit,
		&official.LocalChurch,
		&official.Position,
		&official.Department,
		&official.Status,
		&official.Photo,
		&official.CreatedAt,
	)

	return official, err
}
func UpdateOfficial(official models.Official) error {

	_, err := database.DB.Exec(`
		UPDATE officials
		SET
			full_name=$1,
			gender=$2,
			phone=$3,
			email=$4,
			circuit=$5,
			local_church=$6,
			position=$7,
			department=$8,
			status=$9,
			photo=$10
		WHERE id=$11
	`,
		official.FullName,
		official.Gender,
		official.Phone,
		official.Email,
		official.Circuit,
		official.LocalChurch,
		official.Position,
		official.Department,
		official.Status,
		official.Photo,
		official.ID,
	)

	return err
}
func DeleteOfficial(id int) error {

	_, err := database.DB.Exec(`
		DELETE FROM officials
		WHERE id=$1
	`, id)

	return err
}
