package store

import (
	"time"

	"github.com/vvb13a/godam/domain"
)

// Asset mirrors the "assets" table. It holds the storage location and the
// metadata extracted at upload time. Tags are joined through the asset_tags
// table and loaded with a Preload.
type Asset struct {
	ID            string    `gorm:"column:id;primaryKey;type:text"`
	Filename      string    `gorm:"column:filename;type:text;not null"`
	StorageDriver string    `gorm:"column:storage_driver;type:text;not null;default:local"`
	StorageKey    string    `gorm:"column:storage_key;type:text;not null"`
	MimeType      string    `gorm:"column:mime_type;type:text;not null"`
	ByteSize      int64     `gorm:"column:byte_size;not null"`
	Width         *int      `gorm:"column:width"`
	Height        *int      `gorm:"column:height"`
	DurationSec   *float64  `gorm:"column:duration_sec"`
	PageCount     *int      `gorm:"column:page_count"`
	RawMetadata   string    `gorm:"column:raw_metadata;type:text"`
	CreatedAt     time.Time `gorm:"column:created_at"`

	Tags []Tag `gorm:"many2many:asset_tags;"`
}

func (Asset) TableName() string { return "assets" }

// AssetModel converts a domain asset into its row model.
func AssetModel(a *domain.Asset) *Asset {
	return &Asset{
		ID:            a.ID,
		Filename:      a.Filename,
		StorageDriver: a.StorageDriver,
		StorageKey:    a.StorageKey,
		MimeType:      a.MimeType,
		ByteSize:      a.ByteSize,
		Width:         a.Metadata.Width,
		Height:        a.Metadata.Height,
		DurationSec:   a.Metadata.DurationSec,
		PageCount:     a.Metadata.PageCount,
		RawMetadata:   a.Metadata.RawJSON,
		CreatedAt:     a.CreatedAt,
	}
}

func (m *Asset) ToDomain() *domain.Asset {
	tags := make([]domain.Tag, 0, len(m.Tags))
	for i := range m.Tags {
		tags = append(tags, *m.Tags[i].ToDomain())
	}
	return &domain.Asset{
		ID:            m.ID,
		Filename:      m.Filename,
		StorageDriver: m.StorageDriver,
		StorageKey:    m.StorageKey,
		MimeType:      m.MimeType,
		ByteSize:      m.ByteSize,
		Metadata: domain.Metadata{
			Width:       m.Width,
			Height:      m.Height,
			DurationSec: m.DurationSec,
			PageCount:   m.PageCount,
			RawJSON:     m.RawMetadata,
		},
		Tags:      tags,
		CreatedAt: m.CreatedAt,
	}
}
