package store

import (
	"encoding/json"
	"time"

	"github.com/vvb13a/godam/domain"
)

// Preset mirrors the "presets" table. Parameters is the JSON form of the
// processing pipeline so the schema does not need to change as the catalog
// grows.
type Preset struct {
	ID         string    `gorm:"column:id;primaryKey;type:text"`
	Label      string    `gorm:"column:label;type:text;not null"`
	Name       string    `gorm:"column:name;type:text;not null;uniqueIndex"`
	Parameters string    `gorm:"column:parameters;type:text;not null"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	ModifiedAt time.Time `gorm:"column:modified_at"`
}

func (Preset) TableName() string { return "presets" }

func PresetModel(p *domain.Preset) (*Preset, error) {
	parameters, err := json.Marshal(p.Parameters)
	if err != nil {
		return nil, err
	}
	return &Preset{
		ID:         p.ID,
		Label:      p.Label,
		Name:       p.Name,
		Parameters: string(parameters),
		CreatedAt:  p.CreatedAt,
		ModifiedAt: p.ModifiedAt,
	}, nil
}

func (m *Preset) ToDomain() *domain.Preset {
	parameters := domain.PresetParameters{}
	if m.Parameters != "" {
		_ = json.Unmarshal([]byte(m.Parameters), &parameters)
	}
	return &domain.Preset{
		ID:         m.ID,
		Label:      m.Label,
		Name:       m.Name,
		Parameters: parameters,
		CreatedAt:  m.CreatedAt,
		ModifiedAt: m.ModifiedAt,
	}
}
