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
		email = $5,
		circuit = $6,
		local_church = $7,
		membership = $8,
		position = $9,
		marital_status = $10,
		occupation = $11,
		emergency_contact_name = $12,
		emergency_contact_phone = $13,
		relationahip = $14,
		address = &15,
		arrival_date = $16,
		first_time_attendee = $17,
		bible_study_group = $18,
	WHERE id = $19
	`

	_, err := database.DB.Exec(
		query,
		reg.FullName,
		reg.Gender,
		reg.Age,
		reg.Phone,
		reg.Email,
		reg.Circuit,
		reg.LocalChurch,
		reg.Membership,
		reg.Position,
		reg.MaritalStatus,
		reg.Occupation,
		reg.EmergencyContactName,
		reg.EmergencyContactPhone,
		reg.Relationship,
		reg.Address,
		reg.ArrivalDate,
		reg.FirstTimeAttendee,
		reg.BibleStudyGroup,
		reg.ID,
	)

	return err
}
