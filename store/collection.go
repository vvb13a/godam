package store

import (
	"time"

	"github.com/vvb13a/godam/domain"
)

// Collection mirrors the "collections" table. Assets are joined through the
// asset_collections table.
type Collection struct {
	ID          string    `gorm:"column:id;primaryKey;type:text"`
	Name        string    `gorm:"column:name;type:text;not null;uniqueIndex"`
	Description string    `gorm:"column:description;type:text"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`

	Assets []Asset `gorm:"many2many:asset_collections;"`
}

func (Collection) TableName() string { return "collections" }

func CollectionModel(c *domain.Collection) *Collection {
	return &Collection{
		ID:          c.ID,
		Name:        c.Name,
		Description: c.Description,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

func (m *Collection) ToDomain() *domain.Collection {
	return &domain.Collection{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}
