package domain

import "time"

// Collection is a library-scoped folder. ParentID is nil for a root folder and
// otherwise points at exactly one parent, forming a tree. Sort orders siblings.
type Collection struct {
	ID          string    `json:"id"`
	ParentID    *string   `json:"parent_id,omitempty"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Color       string    `json:"color"`
	Sort        int       `json:"sort"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// AssetCount is derived on read (not stored): how many assets belong
	// directly to the collection.
	AssetCount int `json:"asset_count"`
}
