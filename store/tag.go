package store

import (
	"time"

	"github.com/vvb13a/godam/domain"
)

// Tag mirrors the "tags" table. Assets are joined through the asset_tags
// table.
type Tag struct {
	ID        string    `gorm:"column:id;primaryKey;type:text"`
	Name      string    `gorm:"column:name;type:text;not null;uniqueIndex"`
	CreatedAt time.Time `gorm:"column:created_at"`

	Assets []Asset `gorm:"many2many:asset_tags;"`
}

func (Tag) TableName() string { return "tags" }

func TagModel(t *domain.Tag) *Tag {
	return &Tag{
		ID:        t.ID,
		Name:      t.Name,
		CreatedAt: t.CreatedAt,
	}
}

func (m *Tag) ToDomain() *domain.Tag {
	return &domain.Tag{
		ID:        m.ID,
		Name:      m.Name,
		CreatedAt: m.CreatedAt,
	}
}
