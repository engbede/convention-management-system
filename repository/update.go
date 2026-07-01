package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func UpdateRegistration(reg models.Registration) error {

	query := `
	UPDATE registrations
	SET
		full_name = $1,
		gender = $2,
		age = $3,
		phone = $4,
		circuit = $5,
		local_church = $6,
		membership = $7,
		position = $8,
		marital_status = $9,
		occupation = $10,
		emergency_contact_name = $11,
		emergency_contact_phone = $12,
		arrival_date = $13,
		first_time_attendee = $14,
		bible_study_group = $15
	WHERE id = $16
	`

	_, err := database.DB.Exec(
		query,
		reg.FullName,
		reg.Gender,
		reg.Age,
		reg.Phone,
		reg.Circuit,
		reg.LocalChurch,
		reg.Membership,
		reg.Position,
		reg.MaritalStatus,
		reg.Occupation,
		reg.EmergencyContactName,
		reg.EmergencyContactPhone,
		reg.ArrivalDate,
		reg.FirstTimeAttendee,
		reg.BibleStudyGroup,
		reg.ID,
	)

	return err
}