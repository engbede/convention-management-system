package repository

import (
	"fmt"

	"convention-management-system/database"
	"convention-management-system/helpers"
	"convention-management-system/models"
)

func CreateInquiry(inquiry models.Inquiry) error {

	query := fmt.Sprintf(`
INSERT INTO inquiries
(
	name,
	phone,
	email,
	subject,
	message
)
VALUES (%s,%s,%s,%s,%s)
`,
		helpers.Placeholder(1),
		helpers.Placeholder(2),
		helpers.Placeholder(3),
		helpers.Placeholder(4),
		helpers.Placeholder(5),
	)

	_, err := database.DB.Exec(
		query,
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

	query := fmt.Sprintf(`
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
WHERE id = %s
`, helpers.Placeholder(1))

	err := database.DB.QueryRow(
		query,
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

	query := fmt.Sprintf(`
UPDATE inquiries
SET status = %s
WHERE id = %s
`,
		helpers.Placeholder(1),
		helpers.Placeholder(2),
	)

	_, err := database.DB.Exec(
		query,
		status,
		id,
	)

	return err
}

func DeleteInquiry(id int) error {

	query := fmt.Sprintf(`
DELETE
FROM inquiries
WHERE id = %s
`, helpers.Placeholder(1))

	_, err := database.DB.Exec(
		query,
		id,
	)

	return err
}

func GetInquiryStats() (models.InquiryStats, error) {

	var stats models.InquiryStats

	err := database.DB.QueryRow(`
		SELECT
			COUNT(*) AS total,

			COALESCE(
				SUM(
					CASE
						WHEN status = 'Pending' THEN 1
						ELSE 0
					END
				), 0
			) AS pending,

			COALESCE(
				SUM(
					CASE
						WHEN status = 'Resolved' THEN 1
						ELSE 0
					END
				), 0
			) AS resolved

		FROM inquiries
	`).Scan(
		&stats.Total,
		&stats.Pending,
		&stats.Resolved,
	)

	return stats, err
}

func SearchInquiries(search string) ([]models.Inquiry, error) {

	query := fmt.Sprintf(`
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
	name LIKE %s
	OR phone LIKE %s
	OR email LIKE %s
	OR subject LIKE %s
ORDER BY created_at DESC
`,
		helpers.Placeholder(1),
		helpers.Placeholder(2),
		helpers.Placeholder(3),
		helpers.Placeholder(4),
	)

	like := "%" + search + "%"

	rows, err := database.DB.Query(
		query,
		like,
		like,
		like,
		like,
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

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return inquiries, nil
}

func GetInquiriesByStatus(
	status string,
) ([]models.Inquiry, error) {

	query := fmt.Sprintf(`
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
WHERE status = %s
ORDER BY created_at DESC
`, helpers.Placeholder(1))

	rows, err := database.DB.Query(
		query,
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

	if err := rows.Err(); err != nil {
		return nil, err
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
	p := 1

	if search != "" {

		like := "%" + search + "%"

		query += fmt.Sprintf(`
		AND (
			name LIKE %s
			OR phone LIKE %s
			OR email LIKE %s
			OR subject LIKE %s
		)
		`,
			helpers.Placeholder(p),
			helpers.Placeholder(p+1),
			helpers.Placeholder(p+2),
			helpers.Placeholder(p+3),
		)

		args = append(
			args,
			like,
			like,
			like,
			like,
		)

		p += 4
	}

	if status != "" {

		query += fmt.Sprintf(`
		AND status = %s
		`, helpers.Placeholder(p))

		args = append(args, status)

		p++
	}

	query += fmt.Sprintf(`
	ORDER BY created_at DESC
	LIMIT %s
	OFFSET %s
	`,
		helpers.Placeholder(p),
		helpers.Placeholder(p+1),
	)

	args = append(
		args,
		pageSize,
		offset,
	)

	rows, err := database.DB.Query(query, args...)
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

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return inquiries, nil
}

func GetInquiriesPage(
	page int,
	pageSize int,
) ([]models.Inquiry, error) {

	offset := (page - 1) * pageSize

	query := fmt.Sprintf(`
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
LIMIT %s
OFFSET %s
`,
		helpers.Placeholder(1),
		helpers.Placeholder(2),
	)

	rows, err := database.DB.Query(
		query,
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

	if err := rows.Err(); err != nil {
		return nil, err
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
	p := 1

	if search != "" {

		like := "%" + search + "%"

		query += fmt.Sprintf(`
		AND (
			name LIKE %s
			OR phone LIKE %s
			OR email LIKE %s
			OR subject LIKE %s
		)
		`,
			helpers.Placeholder(p),
			helpers.Placeholder(p+1),
			helpers.Placeholder(p+2),
			helpers.Placeholder(p+3),
		)

		args = append(
			args,
			like,
			like,
			like,
			like,
		)

		p += 4
	}

	if status != "" {

		query += fmt.Sprintf(`
		AND status = %s
		`, helpers.Placeholder(p))

		args = append(
			args,
			status,
		)

		p++
	}

	var total int

	err := database.DB.QueryRow(
		query,
		args...,
	).Scan(&total)

	return total, err
}
