package domain

import "time"

type Asset struct {
	ID            string    `json:"id"`
	Filename      string    `json:"filename"`
	StorageDriver string    `json:"storage_driver"`
	StorageKey    string    `json:"storage_key"`
	MimeType      string    `json:"mime_type"`
	ByteSize      int64     `json:"byte_size"`
	Metadata      Metadata  `json:"metadata"`
	Tags          []Tag     `json:"tags,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}
