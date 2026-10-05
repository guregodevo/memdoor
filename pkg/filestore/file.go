package filestore

import (
	"time"

	"memdoor/pkg/shared"
)

// FileAttachment is the domain entity for an uploaded file
type FileAttachment struct {
	ID          string
	MessageID   *int64
	ChannelID   string
	UploaderID  shared.ActorID
	Filename    string
	MimeType    string
	SizeBytes   int64
	StoragePath string
	CreatedAt   time.Time
}

// FileRef is a lightweight reference to a file attachment (for embedding in messages)
type FileRef struct {
	ID        string `json:"id"`
	Filename  string `json:"filename"`
	MimeType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
}

// IsImage returns true if the file is an image type
func (f *FileAttachment) IsImage() bool {
	switch f.MimeType {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/svg+xml":
		return true
	}
	return false
}
