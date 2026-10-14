package store

import (
	"time"

	"github.com/vvb13a/godam/domain"
)

// Collection mirrors the "collections" table. Assets are joined through the
// asset_collections table.
type Collection struct {
	ID          string    `gorm:"column:id;primaryKey;type:text"`
	LibraryID   string    `gorm:"column:library_id;type:text;not null;index"`
	Name        string    `gorm:"column:name;type:text;not null"`
	Description string    `gorm:"column:description;type:text"`
	Icon        string    `gorm:"column:icon;type:text;not null;default:''"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`

	Assets []Asset `gorm:"many2many:asset_collections;"`
}

func (Collection) TableName() string { return "collections" }

func CollectionModel(c *domain.Collection) *Collection {
	return &Collection{
		ID:          c.ID,
		LibraryID:   c.LibraryID,
		Name:        c.Name,
		Description: c.Description,
		Icon:        c.Icon,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

func (m *Collection) ToDomain() *domain.Collection {
	return &domain.Collection{
		ID:          m.ID,
		LibraryID:   m.LibraryID,
		Name:        m.Name,
		Description: m.Description,
		Icon:        m.Icon,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}
