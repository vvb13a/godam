package store

import (
	"time"

	"github.com/vvb13a/godam/domain"
)

// Tag mirrors the "tags" table. Assets are joined through the asset_tags
// table and a tag optionally belongs to a tag group.
type Tag struct {
	ID        string    `gorm:"column:id;primaryKey;type:text"`
	Name      string    `gorm:"column:name;type:text;not null;uniqueIndex"`
	GroupID   *string   `gorm:"column:group_id;type:text"`
	Starred   bool      `gorm:"column:starred;not null;default:false"`
	CreatedAt time.Time `gorm:"column:created_at"`

	Assets []Asset `gorm:"many2many:asset_tags;"`
}

func (Tag) TableName() string { return "tags" }

func TagModel(t *domain.Tag) *Tag {
	return &Tag{
		ID:        t.ID,
		Name:      t.Name,
		GroupID:   t.GroupID,
		Starred:   t.Starred,
		CreatedAt: t.CreatedAt,
	}
}

func (m *Tag) ToDomain() *domain.Tag {
	return &domain.Tag{
		ID:        m.ID,
		Name:      m.Name,
		GroupID:   m.GroupID,
		Starred:   m.Starred,
		CreatedAt: m.CreatedAt,
	}
}
