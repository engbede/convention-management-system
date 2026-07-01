package repository

import (
	"convention-management-system/database"
	"convention-management-system/helpers"
	"convention-management-system/models"
)

func CreateRegistration(reg models.Registration) error {

	query := `
	INSERT INTO registrations(
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
    bible_study_group
)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
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
		&reg.ArrivalDate,
		&reg.FirstTimeAttendee,
		&reg.BibleStudyGroup,
	)

	return err
}

func GetAllRegistrations() ([]models.Registration, error) {

	rows, err := database.DB.Query(`
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
		ORDER BY id DESC
	`)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var registrations []models.Registration

	for rows.Next() {

		var reg models.Registration

		err := rows.Scan(
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

		if err != nil {
			return nil, err
		}

		reg.RegistrationNumber = helpers.RegistrationNumber(reg.ID)

		registrations = append(
			registrations,
			reg,
		)
	}

	return registrations, nil
}

func GetRegistrationsPaginated(
	page int,
	pageSize int,
) ([]models.Registration, error) {

	offset := (page - 1) * pageSize

	rows, err := database.DB.Query(`
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
		ORDER BY id DESC
		LIMIT $1
		OFFSET $2
	`, pageSize, offset)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var registrations []models.Registration

	for rows.Next() {

		var reg models.Registration

		err := rows.Scan(
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

		if err != nil {
			return nil, err
		}

		reg.RegistrationNumber = helpers.RegistrationNumber(reg.ID)

		registrations = append(
			registrations,
			reg,
		)
	}

	return registrations, nil
}

func CountRegistrations() (int, error) {

	var total int

	err := database.DB.QueryRow(`
		SELECT COUNT(*)
		FROM registrations
	`).Scan(&total)

	if err != nil {
		return 0, err
	}

	return total, nil
}

func SearchRegistrations(
	search string,
) ([]models.Registration, error) {

	query := `
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
	WHERE
	full_name ILIKE $1
	OR phone ILIKE $2
	ORDER BY id DESC
	`

	rows, err := database.DB.Query(
		query,
		"%"+search+"%",
		"%"+search+"%",
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var registrations []models.Registration

	for rows.Next() {

		var reg models.Registration

		err := rows.Scan(
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

		if err != nil {
			return nil, err
		}

		reg.RegistrationNumber = helpers.RegistrationNumber(reg.ID)

		registrations = append(
			registrations,
			reg,
		)
	}

	return registrations, nil
}

func GetDashboardStats() (models.DashboardStats, error) {

	var stats models.DashboardStats

	row := database.DB.QueryRow(`
		SELECT
    COUNT(*),

    COALESCE(SUM(CASE WHEN gender='Male' THEN 1 ELSE 0 END), 0),

    COALESCE(SUM(CASE WHEN gender='Female' THEN 1 ELSE 0 END), 0),

    COALESCE(SUM(CASE WHEN membership='Member' THEN 1 ELSE 0 END), 0),

    COALESCE(SUM(CASE WHEN membership='Non-Member' THEN 1 ELSE 0 END), 0),

    COALESCE(SUM(CASE WHEN first_time_attendee = TRUE THEN 1 ELSE 0 END),0),

    COALESCE(SUM(CASE WHEN marital_status='Married' THEN 1 ELSE 0 END), 0),

    COALESCE(SUM(CASE WHEN marital_status='Single' THEN 1 ELSE 0 END), 0),

    COALESCE(SUM(CASE WHEN marital_status='Divorce' THEN 1 ELSE 0 END), 0)

FROM registrations
	`)

	err := row.Scan(

		&stats.TotalRegistrations,

		&stats.MaleCount,

		&stats.FemaleCount,

		&stats.MemberCount,

		&stats.NonMemberCount,

		&stats.FirstTimeCount,

		&stats.MarriedCount,

		&stats.SingleCount,

		&stats.DivorcedCount,
	)

	return stats, err
}

func GetRegistrationsByCircuit() ([]models.CircuitStat, error) {

	rows, err := database.DB.Query(`
		SELECT
			circuit,
			COUNT(*)
		FROM registrations
		GROUP BY circuit
		ORDER BY COUNT(*) DESC
	`)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var circuits []models.CircuitStat

	for rows.Next() {

		var c models.CircuitStat

		err := rows.Scan(
			&c.Circuit,
			&c.Count,
		)

		if err != nil {
			return nil, err
		}

		circuits = append(
			circuits,
			c,
		)
	}

	return circuits, nil
}
