package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func CreateConvention(c models.Convention) error {
	_, err := database.DB.Exec(`
		INSERT INTO conventions
		(name, theme, venue, start_date, end_date, year, active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		c.Name,
		c.Theme,
		c.Venue,
		c.StartDate,
		c.EndDate,
		c.Year,
		c.Active,
	)

	return err
}

func GetActiveConvention() (models.Convention, error) {

	var c models.Convention

	err := database.DB.QueryRow(`
		SELECT
			id,
			name,
			theme,
			venue,
			start_date,
			end_date,
			year,
			active
		FROM conventions
		WHERE active = TRUE
		LIMIT 1
	`).Scan(
		&c.ID,
		&c.Name,
		&c.Theme,
		&c.Venue,
		&c.StartDate,
		&c.EndDate,
		&c.Year,
		&c.Active,
	)

	return c, err
}

func GetAllConventions() ([]models.Convention, error) {

	rows, err := database.DB.Query(`
		SELECT
			id,
			name,
			theme,
			venue,
			start_date,
			end_date,
			year,
			active
		FROM conventions
		ORDER BY year DESC
	`)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var conventions []models.Convention

	for rows.Next() {

		var c models.Convention

		err := rows.Scan(
			&c.ID,
			&c.Name,
			&c.Theme,
			&c.Venue,
			&c.StartDate,
			&c.EndDate,
			&c.Year,
			&c.Active,
		)

		if err != nil {
			return nil, err
		}

		conventions = append(conventions, c)
	}

	return conventions, nil
}

func GetConventionByID(id int) (models.Convention, error) {

	var c models.Convention

	query := `
	SELECT
		id,
		year,
		name,
		theme,
		venue,
		start_date,
		end_date,
		active
	FROM conventions
	WHERE id = ?
	`

	err := database.DB.QueryRow(query, id).Scan(
		&c.ID,
		&c.Year,
		&c.Name,
		&c.Theme,
		&c.Venue,
		&c.StartDate,
		&c.EndDate,
		&c.Active,
	)

	return c, err
}
func UpdateConvention(c models.Convention) error {

	query := `
	UPDATE conventions
	SET
		year = ?,
		name = ?,
		theme = ?,
		venue = ?,
		start_date = ?,
		end_date = ?
	WHERE id = ?
	`

	_, err := database.DB.Exec(
		query,
		c.Year,
		c.Name,
		c.Theme,
		c.Venue,
		c.StartDate,
		c.EndDate,
		c.ID,
	)

	return err
}
func ActivateConvention(id int) error {

	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}

	// deactivate every convention
	_, err = tx.Exec(`
		UPDATE conventions
		SET active = 0
	`)
	if err != nil {
		tx.Rollback()
		return err
	}

	// activate selected convention
	_, err = tx.Exec(`
		UPDATE conventions
		SET active = 1
		WHERE id = ?
	`, id)

	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}
func DeleteConvention(id int) error {

	_, err := database.DB.Exec(`
		DELETE FROM conventions
		WHERE id = ?
	`, id)

	return err
}
