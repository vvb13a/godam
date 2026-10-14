package domain

import "time"

// User is an authenticated account. It owns libraries (and, transitively, the
// assets, tags, collections and tag groups inside them). The password hash is
// never part of the domain type so it cannot be serialized to clients.
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
