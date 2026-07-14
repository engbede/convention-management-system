package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func UpdateNotice(n models.Notice) error {

	_, err := database.DB.Exec(`
UPDATE notices
SET
    title=?,
    message=?,
    audience=?,
    priority=?,
    pinned=?,
    start_date=?,
    end_date=?
WHERE id=?
`,
		n.Title,
		n.Message,
		n.Audience,
		n.Priority,
		n.Pinned,
		n.StartDate,
		n.EndDate,
		n.ID,
	)

	return err
}
