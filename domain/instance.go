package domain

import "time"

// UploadLimits holds per-category upload size limits in bytes. Zero means the
// category has no size limit.
type UploadLimits struct {
	ImageBytes int64 `json:"image_bytes"`
	AudioBytes int64 `json:"audio_bytes"`
	VideoBytes int64 `json:"video_bytes"`
	PDFBytes   int64 `json:"pdf_bytes"`
}

// DefaultUploadLimits is applied to a new library. Every category starts at
// the previous app-wide default so limits are always explicit.
func DefaultUploadLimits() UploadLimits {
	const megabyte = int64(1 << 20)
	return UploadLimits{
		ImageBytes: 100 * megabyte,
		AudioBytes: 100 * megabyte,
		VideoBytes: 100 * megabyte,
		PDFBytes:   100 * megabyte,
	}
}

// Instance is the single library served by this godam process. The instance id
// is stable and is used in backup manifests and fleet inventory; it is not a
// per-row scope.
type Instance struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Color            string       `json:"color"`
	CoverAssetID     string       `json:"cover_asset_id,omitempty"`
	WatermarkAssetID string       `json:"watermark_asset_id,omitempty"`
	AllowedMimeTypes []string     `json:"allowed_mime_types,omitempty"`
	UploadLimits     UploadLimits `json:"upload_limits"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
}
