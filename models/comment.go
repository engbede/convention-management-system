package models

import "time"

type Comment struct {
	ID int

	PostID int

	UserID int

	ParentID *int

	Content string

	IsEdited bool

	CreatedAt time.Time

	UpdatedAt time.Time

	User User
}
