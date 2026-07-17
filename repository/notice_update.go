package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func UpdateNotice(n models.Notice) error {

	_, err := database.DB.Exec(`
UPDATE notices
SET
	title = $1,
	message = $2,
	audience = $3,
	priority = $4,
	pinned = $5,
	start_date = $6,
	end_date = $7
WHERE id = $8
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
