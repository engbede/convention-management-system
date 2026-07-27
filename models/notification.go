package models

import "time"

type Notification struct {
	ID         int
	SenderID   int
	ReceiverID int

	PostID    *int
	CommentID *int

	Type    string
	Message string

	IsRead bool

	CreatedAt time.Time

	Sender User
}
