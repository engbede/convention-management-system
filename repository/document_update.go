package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func UpdateDocument(d models.Document) error {

	_, err := database.DB.Exec(`
UPDATE documents
SET
	title=?,
	category=?,
	convention=?,
	year=?,
	description=?,
	content=?
WHERE id=?
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
