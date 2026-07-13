package repository

import (
	"convention-management-system/database"
)

func GetAttendeePhones() ([]string, error) {

	rows, err := database.DB.Query(`
		SELECT phone
		FROM registrations
		WHERE phone <> ''
	`)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var phones []string

	for rows.Next() {

		var phone string

		rows.Scan(&phone)

		phones = append(phones, phone)
	}

	return phones, nil
}

func GetOfficialPhones() ([]string, error) {

	rows, err := database.DB.Query(`
		SELECT phone
		FROM officials
		WHERE phone <> ''
	`)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var phones []string

	for rows.Next() {

		var phone string

		rows.Scan(&phone)

		phones = append(phones, phone)
	}

	return phones, nil
}

func GetOfficialEmails() ([]string, error) {

	rows, err := database.DB.Query(`
		SELECT email
		FROM officials
		WHERE email <> ''
	`)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var emails []string

	for rows.Next() {

		var email string

		rows.Scan(&email)

		emails = append(emails, email)
	}

	return emails, nil
}
