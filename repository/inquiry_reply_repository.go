package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func CreateInquiryReply(
	reply models.InquiryReply,
) error {

	_, err := database.DB.Exec(
		`
		INSERT INTO inquiry_replies
		(
			inquiry_id,
			admin_name,
			message
		)
		VALUES
		(
			$1,
			$2,
			$3
		)
		`,
		reply.InquiryID,
		reply.AdminName,
		reply.Message,
	)

	return err
}

func GetRepliesByInquiry(
	inquiryID int,
) ([]models.InquiryReply, error) {

	rows, err := database.DB.Query(
		`
		SELECT
			id,
			inquiry_id,
			admin_name,
			message,
			created_at
		FROM inquiry_replies
		WHERE inquiry_id = $1
		ORDER BY created_at ASC
		`,
		inquiryID,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var replies []models.InquiryReply

	for rows.Next() {

		var reply models.InquiryReply

		err := rows.Scan(
			&reply.ID,
			&reply.InquiryID,
			&reply.AdminName,
			&reply.Message,
			&reply.CreatedAt,
		)

		if err != nil {
			return nil, err
		}

		replies = append(replies, reply)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return replies, nil
}
