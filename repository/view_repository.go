package repository

import (
	"database/sql"

	"convention-management-system/database"
	"convention-management-system/helpers"
	"convention-management-system/models"
)

func GetRegistrationByID(id int) (models.Registration, error) {

	var reg models.Registration

	err := database.DB.QueryRow(`
		SELECT
			id,
			full_name,
			gender,
			age,
			phone,
			circuit,
			local_church,
			membership,
			position,
			marital_status,
			occupation,
			emergency_contact_name,
			emergency_contact_phone,
			arrival_date,
			first_time_attendee,
			checked_in,
			bible_study_group
		FROM registrations
		WHERE id = ?
	`, id).Scan(
		&reg.ID,
		&reg.FullName,
		&reg.Gender,
		&reg.Age,
		&reg.Phone,
		&reg.Circuit,
		&reg.LocalChurch,
		&reg.Membership,
		&reg.Position,
		&reg.MaritalStatus,
		&reg.Occupation,
		&reg.EmergencyContactName,
		&reg.EmergencyContactPhone,
		&reg.ArrivalDate,
		&reg.FirstTimeAttendee,
		&reg.CheckedIn,
		&reg.BibleStudyGroup,
	)

	if err == sql.ErrNoRows {
		return reg, nil
	}

	if err != nil {
		return reg, err
	}

	reg.RegistrationNumber = helpers.RegistrationNumber(reg.ID)

	return reg, nil
}
