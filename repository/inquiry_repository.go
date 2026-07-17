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
		VALUES ($1, $2, $3, $4, $5)
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
func GetInquiryByID(id int) (models.Inquiry, error) {

	var inquiry models.Inquiry

	err := database.DB.QueryRow(
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
		WHERE id = $1
		`,
		id,
	).Scan(
		&inquiry.ID,
		&inquiry.Name,
		&inquiry.Phone,
		&inquiry.Email,
		&inquiry.Subject,
		&inquiry.Message,
		&inquiry.Status,
		&inquiry.CreatedAt,
	)

	return inquiry, err
}
func UpdateInquiryStatus(
	id int,
	status string,
) error {

	_, err := database.DB.Exec(
		`
		UPDATE inquiries
		SET status = $1
		WHERE id = $2
		`,
		status,
		id,
	)

	return err
}
func DeleteInquiry(id int) error {

	_, err := database.DB.Exec(
		`
		DELETE
		FROM inquiries
		WHERE id = $1
		`,
		id,
	)

	return err
}
func GetInquiryStats() (models.InquiryStats, error) {

	var stats models.InquiryStats

	err := database.DB.QueryRow(
		`
		SELECT

			COUNT(*) AS total,

			SUM(
				CASE
					WHEN status = 'Pending'
					THEN 1
					ELSE 0
				END
			),

			SUM(
				CASE
					WHEN status = 'Resolved'
					THEN 1
					ELSE 0
				END
			)

		FROM inquiries
		`,
	).Scan(
		&stats.Total,
		&stats.Pending,
		&stats.Resolved,
	)

	return stats, err
}
func SearchInquiries(search string) ([]models.Inquiry, error) {

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
		WHERE
   	 		name LIKE $1
    		OR phone LIKE $2
    		OR email LIKE $3
    		OR subject LIKE $4
		ORDER BY created_at DESC
		`,
		"%"+search+"%",
		"%"+search+"%",
		"%"+search+"%",
		"%"+search+"%",
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

		inquiries = append(inquiries, inquiry)
	}

	return inquiries, nil
}

func GetInquiriesByStatus(
	status string,
) ([]models.Inquiry, error) {

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
		WHERE status = $1
		ORDER BY created_at DESC
		`,
		status,
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

func FilterInquiries(
	search string,
	status string,
	page int,
	pageSize int,
) ([]models.Inquiry, error) {

	offset := (page - 1) * pageSize

	query := `
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
	WHERE 1=1
	`

	var args []interface{}

	if search != "" {

		query += `
		AND (
			name LIKE $1
    		OR phone LIKE $2
    		OR email LIKE $3
    		OR subject LIKE $4
		)
		`

		like := "%" + search + "%"

		args = append(
			args,
			like,
			like,
			like,
			like,
		)
	}

	if status != "" {

		query += `
		AND status = $1
		`

		args = append(
			args,
			status,
		)
	}

	query += `
	ORDER BY created_at DESC
	LIMIT $1
	OFFSET $2
	`

	args = append(
		args,
		pageSize,
		offset,
	)

	rows, err := database.DB.Query(
		query,
		args...,
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

func GetInquiriesPage(
	page int,
	pageSize int,
) ([]models.Inquiry, error) {

	offset := (page - 1) * pageSize

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
		LIMIT $1
		OFFSET $2
		`,
		pageSize,
		offset,
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

func CountInquiries(
	search string,
	status string,
) (int, error) {

	query := `
	SELECT COUNT(*)
	FROM inquiries
	WHERE 1=1
	`

	var args []interface{}

	if search != "" {

		query += `
		AND (
			name LIKE $1
    		OR phone LIKE $2
    		OR email LIKE $3
    		OR subject LIKE $4
		)
		`

		like := "%" + search + "%"

		args = append(
			args,
			like,
			like,
			like,
			like,
		)
	}

	if status != "" {

		query += `
		AND status = $1
		`

		args = append(
			args,
			status,
		)
	}

	var total int

	err := database.DB.QueryRow(
		query,
		args...,
	).Scan(&total)

	return total, err
}
