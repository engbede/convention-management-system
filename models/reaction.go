package models

import "time"

type Reaction struct {
	ID int

	PostID int

	UserID int

	Reaction string

	CreatedAt time.Time
}

type ReactionSummary struct {
	Like int

	Love int

	Amen int

	Clap int

	Fire int

	Celebrate int

	Total int
}
