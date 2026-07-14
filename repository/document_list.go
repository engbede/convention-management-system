package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func GetAllDocuments() ([]models.Document, error) {

	rows, err := database.DB.Query(`
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
ORDER BY year DESC, created_at DESC
`)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var documents []models.Document

	for rows.Next() {

		var d models.Document

		err := rows.Scan(
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

		if err != nil {
			return nil, err
		}

		documents = append(documents, d)
	}

	return documents, nil
}
