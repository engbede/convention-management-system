package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func CreateNotice(n models.Notice) error {

	_, err := database.DB.Exec(`
	INSERT INTO notices(
		title,
		message,
		audience,
		priority,
		pinned,
		start_date,
		end_date,
		created_by
	)
	VALUES(?,?,?,?,?,?,?,?)
	`,
		n.Title,
		n.Message,
		n.Audience,
		n.Priority,
		n.Pinned,
		n.StartDate,
		n.EndDate,
		n.CreatedBy,
	)

	return err
}
