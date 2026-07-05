package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func GetAllNotices() ([]models.Notice, error) {

	rows, err := database.DB.Query(`
		SELECT
			id,
			title,
			message,
			audience,
			priority,
			pinned,
			start_date,
			end_date,
			created_by,
			created_at
		FROM notices
		ORDER BY
			pinned DESC,
			created_at DESC
	`)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var notices []models.Notice

	for rows.Next() {

		var notice models.Notice

		err := rows.Scan(
			&notice.ID,
			&notice.Title,
			&notice.Message,
			&notice.Audience,
			&notice.Priority,
			&notice.Pinned,
			&notice.StartDate,
			&notice.EndDate,
			&notice.CreatedBy,
			&notice.CreatedAt,
		)

		if err != nil {
			return nil, err
		}

		notices = append(notices, notice)
	}

	return notices, nil
}
