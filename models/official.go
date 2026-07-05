package models

import "time"

type Official struct {
	ID int

	FullName string
	Gender   string

	Phone string
	Email string

	Circuit     string
	LocalChurch string

	Position   string
	Department string

	Status string

	Photo string

	CreatedAt time.Time
}
