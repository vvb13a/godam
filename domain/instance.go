package domain

import "time"

// Instance is the single library served by this godam process. The instance id
// is stable and is used in backup manifests and fleet inventory; it is not a
// per-row scope.
type Instance struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Color            string    `json:"color"`
	CoverAssetID     string    `json:"cover_asset_id,omitempty"`
	WatermarkAssetID string    `json:"watermark_asset_id,omitempty"`
	AllowedMimeTypes []string  `json:"allowed_mime_types,omitempty"`
	MaxUploadBytes   int64     `json:"max_upload_bytes,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
