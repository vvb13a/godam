package domain

import "time"

// SmartCollectionFilter is a saved asset-view filter. Empty fields mean "no
// constraint". Tag holds the raw tag query value (comma-separated ids or
// "untagged"); Collection holds a collection id or "uncategorized".
type SmartCollectionFilter struct {
	Collection string `json:"collection,omitempty"`
	Tag        string `json:"tag,omitempty"`
	Type       string `json:"type,omitempty"`
	Sort       string `json:"sort,omitempty"`
}

// SmartCollection is a named, coloured filter preset within a library.
type SmartCollection struct {
	ID          string                `json:"id"`
	LibraryID   string                `json:"library_id"`
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Color       string                `json:"color"`
	Sort        int                   `json:"sort"`
	Filter      SmartCollectionFilter `json:"filter"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`

	// AssetCount is derived on read (not stored): how many assets in the
	// library match the smart collection's filter.
	AssetCount int `json:"asset_count"`
}
