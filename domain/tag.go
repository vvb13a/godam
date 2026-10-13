package domain

import "time"

type Tag struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	GroupID    *string   `json:"group_id,omitempty"`
	Starred    bool      `json:"starred"`
	AssetCount int       `json:"asset_count"`
	CreatedAt  time.Time `json:"created_at"`
}
