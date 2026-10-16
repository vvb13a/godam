package store

import (
	"time"

	"github.com/vvb13a/godam/domain"
)

// Tag mirrors the "tags" table. Assets are joined through the asset_tags
// table and a tag optionally belongs to a tag group.
type Tag struct {
	ID          string    `gorm:"column:id;primaryKey;type:text"`
	LibraryID   string    `gorm:"column:library_id;type:text;not null;index"`
	Name        string    `gorm:"column:name;type:text;not null"`
	Description string    `gorm:"column:description;type:text;not null;default:''"`
	GroupID     *string   `gorm:"column:group_id;type:text"`
	Starred     bool      `gorm:"column:starred;not null;default:false"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`

	Assets []Asset `gorm:"many2many:asset_tags;"`
}

func (Tag) TableName() string { return "tags" }

func TagModel(t *domain.Tag) *Tag {
	return &Tag{
		ID:          t.ID,
		LibraryID:   t.LibraryID,
		Name:        t.Name,
		Description: t.Description,
		GroupID:     t.GroupID,
		Starred:     t.Starred,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

func (m *Tag) ToDomain() *domain.Tag {
	return &domain.Tag{
		ID:          m.ID,
		LibraryID:   m.LibraryID,
		Name:        m.Name,
		Description: m.Description,
		GroupID:     m.GroupID,
		Starred:     m.Starred,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}
