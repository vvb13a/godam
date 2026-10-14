package domain

import "time"

type Collection struct {
	ID          string    `json:"id"`
	LibraryID   string    `json:"library_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Icon        string    `json:"icon,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// AssetCount is derived on read (not stored): how many assets belong to the
	// collection.
	AssetCount int `json:"asset_count"`
}
