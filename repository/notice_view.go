package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func GetNoticeByID(id int) (models.Notice, error) {

	var notice models.Notice

	err := database.DB.QueryRow(`
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
		WHERE id=$1
	`, id).Scan(
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

	return notice, err
}
