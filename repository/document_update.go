package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func UpdateDocument(d models.Document) error {

	_, err := database.DB.Exec(`
UPDATE documents
SET
	title = $1,
	category = $2,
	convention = $3,
	year = $4,
	description = $5,
	content = $6
WHERE id = $7
`,
		d.Title,
		d.Category,
		d.Convention,
		d.Year,
		d.Description,
		d.Content,
		d.ID,
	)

	return err
}
