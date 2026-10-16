package store

import (
	"encoding/json"
	"time"

	"github.com/vvb13a/godam/domain"
)

// SmartCollection mirrors the "smart_collections" table. The filter is stored
// as a JSON blob.
type SmartCollection struct {
	ID          string    `gorm:"column:id;primaryKey;type:text"`
	LibraryID   string    `gorm:"column:library_id;type:text;not null;index"`
	Name        string    `gorm:"column:name;type:text;not null"`
	Description string    `gorm:"column:description;type:text;not null;default:''"`
	Color       string    `gorm:"column:color;type:text;not null;default:''"`
	Sort        int       `gorm:"column:sort;not null;default:0"`
	Filter      string    `gorm:"column:filter;type:text;not null;default:''"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (SmartCollection) TableName() string { return "smart_collections" }

func SmartCollectionModel(s *domain.SmartCollection) *SmartCollection {
	encoded, _ := json.Marshal(s.Filter)
	return &SmartCollection{
		ID:          s.ID,
		LibraryID:   s.LibraryID,
		Name:        s.Name,
		Description: s.Description,
		Color:       s.Color,
		Sort:        s.Sort,
		Filter:      string(encoded),
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

func (m *SmartCollection) ToDomain() *domain.SmartCollection {
	var filter domain.SmartCollectionFilter
	if m.Filter != "" {
		_ = json.Unmarshal([]byte(m.Filter), &filter)
	}
	return &domain.SmartCollection{
		ID:          m.ID,
		LibraryID:   m.LibraryID,
		Name:        m.Name,
		Description: m.Description,
		Color:       m.Color,
		Sort:        m.Sort,
		Filter:      filter,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}
