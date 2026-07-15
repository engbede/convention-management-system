package models

import "time"

type Inquiry struct {
	ID int

	Name string

	Phone string

	Email string

	Subject string

	Message string

	Status string

	CreatedAt time.Time
}
