package store

import (
	"time"

	"github.com/vvb13a/godam/domain"
)

// Library mirrors the "libraries" table. Every asset, tag, tag group and
// collection belongs to exactly one library. Sort drives the load order.
type Library struct {
	ID        string    `gorm:"column:id;primaryKey;type:text"`
	Name      string    `gorm:"column:name;type:text;not null;uniqueIndex"`
	Color     string    `gorm:"column:color;type:text;not null;default:''"`
	Sort      int       `gorm:"column:sort;not null;default:0"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (Library) TableName() string { return "libraries" }

func LibraryModel(l *domain.Library) *Library {
	return &Library{
		ID:        l.ID,
		Name:      l.Name,
		Color:     l.Color,
		Sort:      l.Sort,
		CreatedAt: l.CreatedAt,
		UpdatedAt: l.UpdatedAt,
	}
}

func (m *Library) ToDomain() *domain.Library {
	return &domain.Library{
		ID:        m.ID,
		Name:      m.Name,
		Color:     m.Color,
		Sort:      m.Sort,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}
