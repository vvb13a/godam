package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/store"
	"gorm.io/gorm"
)

var ErrPresetNameTaken = errors.New("preset name already in use")

// PresetService manages named imgproxy processing pipelines.
type PresetService struct {
	db *gorm.DB
}

func NewPresetService(db *gorm.DB) *PresetService {
	return &PresetService{db: db}
}

func (s *PresetService) List(ctx context.Context) ([]domain.Preset, error) {
	var models []store.Preset
	if err := s.db.WithContext(ctx).Order("name ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	presets := make([]domain.Preset, 0, len(models))
	for i := range models {
		presets = append(presets, *models[i].ToDomain())
	}
	return presets, nil
}

func (s *PresetService) GetByID(ctx context.Context, id string) (*domain.Preset, error) {
	var model store.Preset
	err := s.db.WithContext(ctx).First(&model, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return model.ToDomain(), nil
}

// GetByName is the conversion hot path.
func (s *PresetService) GetByName(ctx context.Context, name string) (*domain.Preset, error) {
	var model store.Preset
	err := s.db.WithContext(ctx).First(&model, "name = ?", strings.ToLower(strings.TrimSpace(name))).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return model.ToDomain(), nil
}

func (s *PresetService) Create(ctx context.Context, label, name string, params domain.PresetParameters) (*domain.Preset, error) {
	label, name, params, err := ValidatePreset(label, name, params)
	if err != nil {
		return nil, err
	}
	exists, err := s.nameExists(ctx, name, "")
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrPresetNameTaken
	}

	now := time.Now().UTC()
	preset := &domain.Preset{
		ID:         uuid.New().String(),
		Label:      label,
		Name:       name,
		Parameters: params,
		CreatedAt:  now,
		ModifiedAt: now,
	}
	model, err := store.PresetModel(preset)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Create(model).Error; err != nil {
		return nil, err
	}
	return preset, nil
}

func (s *PresetService) Update(ctx context.Context, id, label, name string, params domain.PresetParameters) (*domain.Preset, error) {
	if _, err := s.GetByID(ctx, id); err != nil {
		return nil, err
	}
	label, name, params, err := ValidatePreset(label, name, params)
	if err != nil {
		return nil, err
	}
	exists, err := s.nameExists(ctx, name, id)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrPresetNameTaken
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).
		Model(&store.Preset{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"label":       label,
			"name":        name,
			"parameters":  string(encoded),
			"modified_at": time.Now().UTC(),
		}).Error; err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

func (s *PresetService) Delete(ctx context.Context, id string) error {
	res := s.db.WithContext(ctx).Delete(&store.Preset{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// AllowsAsset reports whether an asset may use a preset. Presets are
// library-wide, so any existing preset is available to every asset.
func (s *PresetService) AllowsAsset(ctx context.Context, presetID, _ string) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&store.Preset{}).
		Where("id = ?", presetID).Count(&count).Error; err != nil {
		return false, err
	}
	if count == 0 {
		return false, domain.ErrNotFound
	}
	return true, nil
}

// AllowedForAsset returns every preset, ordered by name.
func (s *PresetService) AllowedForAsset(ctx context.Context, _ string) ([]domain.Preset, error) {
	return s.List(ctx)
}

func (s *PresetService) nameExists(ctx context.Context, name, excludeID string) (bool, error) {
	query := s.db.WithContext(ctx).Model(&store.Preset{}).Where("name = ?", name)
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
