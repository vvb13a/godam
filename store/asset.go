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
	UploadedBy    *string   `gorm:"column:uploaded_by;type:text;index"`
	Filename      string    `gorm:"column:filename;type:text;not null"`
	Title         string    `gorm:"column:title;type:text;not null;default:''"`
	Note          string    `gorm:"column:note;type:text;not null;default:''"`
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
	UpdatedAt     time.Time `gorm:"column:updated_at"`

	Tags []Tag `gorm:"many2many:asset_tags;"`

	Collections []Collection `gorm:"many2many:asset_collections;"`
}

func (Asset) TableName() string { return "assets" }

// AssetModel converts a domain asset into its row model.
func AssetModel(a *domain.Asset) *Asset {
	var uploadedBy *string
	if a.UploadedBy != "" {
		uploadedBy = &a.UploadedBy
	}
	return &Asset{
		ID:            a.ID,
		UploadedBy:    uploadedBy,
		Filename:      a.Filename,
		Title:         a.Title,
		Note:          a.Note,
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
		UpdatedAt:     a.UpdatedAt,
	}
}

func (m *Asset) ToDomain() *domain.Asset {
	tags := make([]domain.Tag, 0, len(m.Tags))
	for i := range m.Tags {
		tags = append(tags, *m.Tags[i].ToDomain())
	}
	collections := make([]domain.Collection, 0, len(m.Collections))
	for i := range m.Collections {
		collections = append(collections, *m.Collections[i].ToDomain())
	}
	var uploadedBy string
	if m.UploadedBy != nil {
		uploadedBy = *m.UploadedBy
	}
	return &domain.Asset{
		ID:            m.ID,
		UploadedBy:    uploadedBy,
		Filename:      m.Filename,
		Title:         m.Title,
		Note:          m.Note,
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
		Tags:        tags,
		Collections: collections,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}
