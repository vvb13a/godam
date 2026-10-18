package domain

import "time"

type Tag struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	GroupID     *string   `json:"group_id,omitempty"`
	Starred     bool      `json:"starred"`
	AssetCount  int       `json:"asset_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
