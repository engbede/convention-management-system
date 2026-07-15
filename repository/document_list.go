package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func GetAllDocuments() ([]models.Document, error) {

	rows, err := database.DB.Query(`
SELECT
    d.id,
    d.title,
    d.category,
    d.convention,
    d.year,
    d.description,
    d.content,
    d.created_by,
    d.created_at,
    COUNT(f.id) AS attachment_count
FROM documents d
LEFT JOIN document_files f
ON d.id = f.document_id
GROUP BY
    d.id,
    d.title,
    d.category,
    d.convention,
    d.year,
    d.description,
    d.content,
    d.created_by,
    d.created_at
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
			&d.AttachmentCount,
		)

		if err != nil {
			return nil, err
		}

		documents = append(documents, d)
	}

	return documents, nil
}
