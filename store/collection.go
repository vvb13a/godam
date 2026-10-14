package store

import (
	"time"

	"github.com/vvb13a/godam/domain"
)

// Collection mirrors the "collections" table. Assets are joined through the
// asset_collections table; ParentID forms the folder hierarchy.
type Collection struct {
	ID          string    `gorm:"column:id;primaryKey;type:text"`
	LibraryID   string    `gorm:"column:library_id;type:text;not null;index"`
	ParentID    *string   `gorm:"column:parent_id;type:text;index"`
	Name        string    `gorm:"column:name;type:text;not null"`
	Description string    `gorm:"column:description;type:text"`
	Color       string    `gorm:"column:color;type:text;not null;default:''"`
	Sort        int       `gorm:"column:sort;not null;default:0"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`

	Assets []Asset `gorm:"many2many:asset_collections;"`
}

func (Collection) TableName() string { return "collections" }

func CollectionModel(c *domain.Collection) *Collection {
	return &Collection{
		ID:          c.ID,
		LibraryID:   c.LibraryID,
		ParentID:    c.ParentID,
		Name:        c.Name,
		Description: c.Description,
		Color:       c.Color,
		Sort:        c.Sort,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

func (m *Collection) ToDomain() *domain.Collection {
	return &domain.Collection{
		ID:          m.ID,
		LibraryID:   m.LibraryID,
		ParentID:    m.ParentID,
		Name:        m.Name,
		Description: m.Description,
		Color:       m.Color,
		Sort:        m.Sort,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}
