package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func GetDocumentByID(id int) (models.Document, error) {

	var d models.Document

	err := database.DB.QueryRow(`
SELECT
	id,
	title,
	category,
	convention,
	year,
	description,
	content,
	created_by,
	created_at
FROM documents
WHERE id = $1
`,
		id,
	).Scan(
		&d.ID,
		&d.Title,
		&d.Category,
		&d.Convention,
		&d.Year,
		&d.Description,
		&d.Content,
		&d.CreatedBy,
		&d.CreatedAt,
	)

	return d, err
}
