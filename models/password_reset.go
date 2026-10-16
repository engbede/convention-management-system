package models

import "time"

type PasswordReset struct {
	ID        int
	UserID    int
	Email     string
	TokenHash string
	ExpiresAt time.Time
	Used      bool
	CreatedAt time.Time
}
