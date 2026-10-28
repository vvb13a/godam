package domain

import "time"

// SmartCollectionFilter is a saved asset-view filter. Empty fields mean "no
// constraint".
type SmartCollectionFilter struct {
	Type string `json:"type,omitempty"`
	Sort string `json:"sort,omitempty"`
}

// SmartCollection is a named, coloured filter preset within a library.
type SmartCollection struct {
	ID          string                `json:"id"`
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
