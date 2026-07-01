package database

import (
	"database/sql"

	"golang.org/x/crypto/bcrypt"
)

func SeedAdmin() error {

	var count int

	err := DB.QueryRow(
		"SELECT COUNT(*) FROM admins",
	).Scan(&count)

	if err != nil {
		return err
	}

	// If an admin already exists, do nothing.
	if count > 0 {
		return nil
	}

	// Hash the default password.
	hashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte("admin123"),
		bcrypt.DefaultCost,
	)

	if err != nil {
		return err
	}

	_, err = DB.Exec(
	`INSERT INTO admins(username, password)
	 VALUES($1, $2)`,
	"admin",
	string(hashedPassword),
)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	return nil
}
