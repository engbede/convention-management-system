package models

import "time"

type Notice struct {
	ID        int
	Title     string
	Message   string
	Audience  string
	Priority  string
	Pinned    bool
	StartDate string
	EndDate   string
	CreatedBy string
	CreatedAt time.Time
}
