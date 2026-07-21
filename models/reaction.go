package models

import "time"

type Reaction struct {
	ID int

	PostID int

	UserID int

	Reaction string

	CreatedAt time.Time
}
