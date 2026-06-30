package repository

import (
	"convention-management-system/database"
	"convention-management-system/models"
)

func UpdateRegistration(reg models.Registration) error {

	query := `
	UPDATE registrations
	SET
		full_name = ?,
		gender = ?,
		age = ?,
		phone = ?,
		circuit = ?,
		local_church = ?,
		membership = ?,
		position = ?,
		marital_status = ?,
		occupation = ?,
		emergency_contact_name = ?,
		emergency_contact_phone = ?,
		arrival_date = ?,
		first_time_attendee = ?,
		bible_study_group = ?
	WHERE id = ?
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