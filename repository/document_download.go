package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func GetDocumentFileByID(
	id int,
) (
	models.DocumentFile,
	error,
) {

	var file models.DocumentFile

	err := database.DB.QueryRow(

		`SELECT
			id,
			document_id,
			file_name,
			file_path,
			file_type,
			uploaded_at
		FROM document_files
		WHERE id=?`,

		id,
	).Scan(

		&file.ID,
		&file.DocumentID,
		&file.FileName,
		&file.FilePath,
		&file.FileType,
		&file.UploadedAt,
	)

	return file, err
}
