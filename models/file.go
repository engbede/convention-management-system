package models

import "time"

type DocumentFile struct {
	ID         int
	DocumentID int
	FileName   string
	FilePath   string
	FileType   string
	UploadedAt time.Time
}
