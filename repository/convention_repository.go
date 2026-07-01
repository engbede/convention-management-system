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