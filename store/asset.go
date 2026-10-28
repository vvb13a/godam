package store

import (
	"time"

	"github.com/vvb13a/godam/domain"
)

// Asset mirrors the "assets" table. It holds the storage location and the
// metadata extracted at upload time. Collections are joined through the
// asset_collections table and loaded with a Preload.
type Asset struct {
	ID             string    `gorm:"column:id;primaryKey;type:text"`
	UploadedByName string    `gorm:"column:uploaded_by_name;type:text;not null;default:''"`
	Filename       string    `gorm:"column:filename;type:text;not null"`
	Title          string    `gorm:"column:title;type:text;not null;default:''"`
	Note           string    `gorm:"column:note;type:text;not null;default:''"`
	StorageDriver  string    `gorm:"column:storage_driver;type:text;not null;default:local"`
	StorageKey     string    `gorm:"column:storage_key;type:text;not null"`
	MimeType       string    `gorm:"column:mime_type;type:text;not null"`
	ByteSize       int64     `gorm:"column:byte_size;not null"`
	Role           string    `gorm:"column:role;type:text;not null;default:'original';index"`
	ImageOf        *string   `gorm:"column:image_of;type:text;index"`
	Width          *int      `gorm:"column:width"`
	Height         *int      `gorm:"column:height"`
	DurationSec    *float64  `gorm:"column:duration_sec"`
	PageCount      *int      `gorm:"column:page_count"`
	RawMetadata    string    `gorm:"column:raw_metadata;type:text"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`

	Collections []Collection `gorm:"many2many:asset_collections;"`

	Colors []AssetColor `gorm:"foreignKey:AssetID"`

	Attributes []AssetAttribute `gorm:"foreignKey:AssetID"`
}

func (Asset) TableName() string { return "assets" }

type AssetColor struct {
	AssetID    string  `gorm:"column:asset_id;primaryKey;type:text"`
	Position   int     `gorm:"column:position;primaryKey"`
	Color      string  `gorm:"column:color;type:text;not null"`
	Bucket     string  `gorm:"column:bucket;type:text;not null;index"`
	Percentage float64 `gorm:"column:percentage;not null"`
}

func (AssetColor) TableName() string { return "asset_colors" }

func (c *AssetColor) ToDomain() domain.AssetColor {
	return domain.AssetColor{
		Hex:        c.Color,
		Bucket:     c.Bucket,
		Percentage: c.Percentage,
	}
}

// AssetModel converts a domain asset into its row model.
func AssetModel(a *domain.Asset) *Asset {
	var imageOf *string
	if a.ImageOf != "" {
		value := a.ImageOf
		imageOf = &value
	}
	role := a.Role
	if role == "" {
		role = domain.AssetRoleOriginal
	}
	return &Asset{
		ID:             a.ID,
		UploadedByName: a.UploadedByName,
		Filename:       a.Filename,
		Title:          a.Title,
		Note:           a.Note,
		StorageDriver:  a.StorageDriver,
		StorageKey:     a.StorageKey,
		MimeType:       a.MimeType,
		ByteSize:       a.ByteSize,
		Role:           role,
		ImageOf:        imageOf,
		Width:          a.Metadata.Width,
		Height:         a.Metadata.Height,
		DurationSec:    a.Metadata.DurationSec,
		PageCount:      a.Metadata.PageCount,
		RawMetadata:    a.Metadata.RawJSON,
		CreatedAt:      a.CreatedAt,
		UpdatedAt:      a.UpdatedAt,
	}
}

func (m *Asset) ToDomain() *domain.Asset {
	collections := make([]domain.Collection, 0, len(m.Collections))
	for i := range m.Collections {
		collections = append(collections, *m.Collections[i].ToDomain())
	}
	colors := make([]domain.AssetColor, 0, len(m.Colors))
	for i := range m.Colors {
		colors = append(colors, m.Colors[i].ToDomain())
	}
	attributes := make([]domain.AssetAttribute, 0, len(m.Attributes))
	for i := range m.Attributes {
		attributes = append(attributes, m.Attributes[i].ToDomain())
	}
	var imageOf string
	if m.ImageOf != nil {
		imageOf = *m.ImageOf
	}
	role := m.Role
	if role == "" {
		role = domain.AssetRoleOriginal
	}
	return &domain.Asset{
		ID:             m.ID,
		UploadedByName: m.UploadedByName,
		Filename:       m.Filename,
		Title:          m.Title,
		Note:           m.Note,
		StorageDriver:  m.StorageDriver,
		StorageKey:     m.StorageKey,
		MimeType:       m.MimeType,
		ByteSize:       m.ByteSize,
		Role:           role,
		ImageOf:        imageOf,
		Metadata: domain.Metadata{
			Width:       m.Width,
			Height:      m.Height,
			DurationSec: m.DurationSec,
			PageCount:   m.PageCount,
			RawJSON:     m.RawMetadata,
		},
		Attributes:  attributes,
		Colors:      colors,
		Collections: collections,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}
