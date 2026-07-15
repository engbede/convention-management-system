package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func CreateInquiry(inquiry models.Inquiry) error {

	_, err := database.DB.Exec(

		`
		INSERT INTO inquiries
		(
			name,
			phone,
			email,
			subject,
			message
		)
		VALUES
		(?,?,?,?,?)
		`,

		inquiry.Name,
		inquiry.Phone,
		inquiry.Email,
		inquiry.Subject,
		inquiry.Message,
	)

	return err
}
func GetAllInquiries() ([]models.Inquiry, error) {

	rows, err := database.DB.Query(

		`
		SELECT
			id,
			name,
			phone,
			email,
			subject,
			message,
			status,
			created_at
		FROM inquiries
		ORDER BY created_at DESC
		`,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var inquiries []models.Inquiry

	for rows.Next() {

		var inquiry models.Inquiry

		err := rows.Scan(

			&inquiry.ID,
			&inquiry.Name,
			&inquiry.Phone,
			&inquiry.Email,
			&inquiry.Subject,
			&inquiry.Message,
			&inquiry.Status,
			&inquiry.CreatedAt,
		)

		if err != nil {
			return nil, err
		}

		inquiries = append(
			inquiries,
			inquiry,
		)
	}

	return inquiries, nil
}
