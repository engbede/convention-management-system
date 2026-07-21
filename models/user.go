package models

import "time"

type User struct {
	ID int

	FullName string
	Username string
	Email    string
	Phone    string

	PasswordHash string
	Role         string

	// Profile
	Bio          string
	Gender       string
	BirthDate    string
	Location     string
	Website      string
	ProfilePhoto string
	CoverPhoto   string

	// Account Status
	IsVerified bool
	IsActive   bool

	// Social
	Followers int
	Following int

	// Dates
	CreatedAt time.Time
	UpdatedAt time.Time
}
