package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func CreateDocument(d models.Document) error {

	_, err := database.DB.Exec(`
INSERT INTO documents
(
title,
category,
convention,
year,
description,
content,
created_by
)
VALUES
(?,?,?,?,?,?,?)
`,
		d.Title,
		d.Category,
		d.Convention,
		d.Year,
		d.Description,
		d.Content,
		d.CreatedBy,
	)

	return err
}
