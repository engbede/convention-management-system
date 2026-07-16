package models

import "time"

type InquiryReply struct {
	ID        int
	InquiryID int
	AdminName string
	Message   string
	CreatedAt time.Time
}
