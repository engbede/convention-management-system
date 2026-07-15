package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func SaveDocumentFile(file models.DocumentFile) error {

	_, err := database.DB.Exec(

		`INSERT INTO document_files
		(
			document_id,
			file_name,
			file_path,
			file_type
		)
		VALUES (?, ?, ?, ?)`,

		file.DocumentID,
		file.FileName,
		file.FilePath,
		file.FileType,
	)

	return err
}

func GetDocumentFiles(documentID int) ([]models.DocumentFile, error) {

	rows, err := database.DB.Query(

		`SELECT
			id,
			document_id,
			file_name,
			file_path,
			file_type,
			uploaded_at
		FROM document_files
		WHERE document_id=?
		ORDER BY uploaded_at DESC`,

		documentID,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var files []models.DocumentFile

	for rows.Next() {

		var f models.DocumentFile

		err := rows.Scan(

			&f.ID,
			&f.DocumentID,
			&f.FileName,
			&f.FilePath,
			&f.FileType,
			&f.UploadedAt,
		)

		if err != nil {
			return nil, err
		}

		files = append(files, f)
	}

	return files, nil
}
