package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func GetPublicNotices() ([]models.Notice, error) {

	rows, err := database.DB.Query(`
	SELECT
		id,
		title,
		message,
		audience,
		priority,
		pinned,
		start_date,
		end_date
	FROM notices
	WHERE
		date('now') BETWEEN start_date AND end_date
	ORDER BY
		pinned DESC,
		priority DESC,
		id DESC
	`)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var notices []models.Notice

	for rows.Next() {

		var n models.Notice

		rows.Scan(
			&n.ID,
			&n.Title,
			&n.Message,
			&n.Audience,
			&n.Priority,
			&n.Pinned,
			&n.StartDate,
			&n.EndDate,
		)

		notices = append(notices, n)
	}

	return notices, nil
}
