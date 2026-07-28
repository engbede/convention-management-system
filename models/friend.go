package models

import "time"

type FriendRequest struct {
	ID         int
	SenderID   int
	ReceiverID int
	Status     string

	Sender   User
	Receiver User

	CreatedAt time.Time
	UpdatedAt time.Time
}

type Friend struct {
	ID int

	User1ID int
	User2ID int

	User User

	CreatedAt time.Time
}
