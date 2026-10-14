package domain

import "time"

// TagGroup is an optional, user-defined bucket for tags. Tags reference a
// group by ID; a tag without one is "uncategorized".
type TagGroup struct {
	ID          string    `json:"id"`
	LibraryID   string    `json:"library_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Color       string    `json:"color"`
	TagCount    int       `json:"tag_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
