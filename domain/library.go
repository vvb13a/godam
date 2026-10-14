package domain

import "time"

// Library is the top-level scope owning assets, tags, tag groups and
// collections. Membership is many-to-many (see library_users). Sort is a
// user-defined ordering: the library with the lowest sort value is the one
// loaded on start.
type Library struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	Sort      int       `json:"sort"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
