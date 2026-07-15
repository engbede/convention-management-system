package models

import "time"

type Document struct {
	ID              int
	Title           string
	Category        string
	Convention      string
	Year            int
	Description     string
	Content         string
	CreatedBy       string
	CreatedAt       time.Time
	AttachmentCount int
}
