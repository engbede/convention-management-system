package models

import "time"

type User struct {
	// Identity
	ID       int
	FullName string
	Username string

	// Contact
	Email string
	Phone string

	// Authentication
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
	Initials     string

	// Account Status
	IsVerified bool
	IsActive   bool

	// Social
	Followers int
	Following int

	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time
}
