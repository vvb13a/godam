package store

import (
	"time"

	"github.com/vvb13a/godam/domain"
)

// TagGroup mirrors the "tag_groups" table. Tags point at a group through the
// tags.group_id column.
type TagGroup struct {
	ID          string    `gorm:"column:id;primaryKey;type:text"`
	Name        string    `gorm:"column:name;type:text;not null;uniqueIndex"`
	Description string    `gorm:"column:description;type:text"`
	Color       string    `gorm:"column:color;type:text;not null;default:''"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (TagGroup) TableName() string { return "tag_groups" }

func TagGroupModel(g *domain.TagGroup) *TagGroup {
	return &TagGroup{
		ID:          g.ID,
		Name:        g.Name,
		Description: g.Description,
		Color:       g.Color,
		CreatedAt:   g.CreatedAt,
		UpdatedAt:   g.UpdatedAt,
	}
}

func (m *TagGroup) ToDomain() *domain.TagGroup {
	return &domain.TagGroup{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Color:       m.Color,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}
