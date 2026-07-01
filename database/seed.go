package database

import (
	"database/sql"
	"os"

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

	// Read admin credentials from environment variables.
	username := os.Getenv("ADMIN_USERNAME")
	if username == "" {
		username = "superadmin"
	}

	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		password = "YouthConvention@2026"
	}

	// Hash the password.
	hashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return err
	}

	// Insert the admin.
	_, err = DB.Exec(
		`INSERT INTO admins(username, password)
		 VALUES($1, $2)`,
		username,
		string(hashedPassword),
	)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	return nil
}