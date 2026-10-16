package store

import (
	"encoding/json"
	"time"

	"github.com/vvb13a/godam/domain"
)

// Library mirrors the "libraries" table. Every asset, tag, tag group and
// collection belongs to exactly one library. Sort drives the load order.
type Library struct {
	ID               string    `gorm:"column:id;primaryKey;type:text"`
	Name             string    `gorm:"column:name;type:text;not null;uniqueIndex"`
	Color            string    `gorm:"column:color;type:text;not null;default:''"`
	Sort             int       `gorm:"column:sort;not null;default:0"`
	CoverAssetID     *string   `gorm:"column:cover_asset_id;type:text"`
	WatermarkAssetID *string   `gorm:"column:watermark_asset_id;type:text"`
	AllowedMimeTypes string    `gorm:"column:allowed_mime_types;type:text;not null;default:''"`
	MaxUploadBytes   int64     `gorm:"column:max_upload_bytes;not null;default:0"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func (Library) TableName() string { return "libraries" }

func LibraryModel(l *domain.Library) *Library {
	var cover *string
	if l.CoverAssetID != "" {
		cover = &l.CoverAssetID
	}
	var watermark *string
	if l.WatermarkAssetID != "" {
		watermark = &l.WatermarkAssetID
	}
	allowed := ""
	if len(l.AllowedMimeTypes) > 0 {
		if encoded, err := json.Marshal(l.AllowedMimeTypes); err == nil {
			allowed = string(encoded)
		}
	}
	return &Library{
		ID:               l.ID,
		Name:             l.Name,
		Color:            l.Color,
		Sort:             l.Sort,
		CoverAssetID:     cover,
		WatermarkAssetID: watermark,
		AllowedMimeTypes: allowed,
		MaxUploadBytes:   l.MaxUploadBytes,
		CreatedAt:        l.CreatedAt,
		UpdatedAt:        l.UpdatedAt,
	}
}

func (m *Library) ToDomain() *domain.Library {
	var cover string
	if m.CoverAssetID != nil {
		cover = *m.CoverAssetID
	}
	var watermark string
	if m.WatermarkAssetID != nil {
		watermark = *m.WatermarkAssetID
	}
	var allowed []string
	if m.AllowedMimeTypes != "" {
		_ = json.Unmarshal([]byte(m.AllowedMimeTypes), &allowed)
	}
	return &domain.Library{
		ID:               m.ID,
		Name:             m.Name,
		Color:            m.Color,
		Sort:             m.Sort,
		CoverAssetID:     cover,
		WatermarkAssetID: watermark,
		AllowedMimeTypes: allowed,
		MaxUploadBytes:   m.MaxUploadBytes,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}
}
