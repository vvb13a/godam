package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/store"
	"gorm.io/gorm"
)

var (
	ErrPresetNameTaken  = errors.New("preset name already in use")
	ErrPresetNotAllowed = errors.New("preset is not available for this asset")
)

const maxPresetTargets = 1000

// PresetService manages named imgproxy processing pipelines and the
// collections that make them available to assets.
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

	collections := s.targets(ctx, "preset_collections", "collection_id")
	for i := range presets {
		presets[i].CollectionIDs = collections[presets[i].ID]
		if presets[i].CollectionIDs == nil {
			presets[i].CollectionIDs = []string{}
		}
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
	preset := model.ToDomain()
	collectionIDs, err := s.loadTargets(ctx, id)
	if err != nil {
		return nil, err
	}
	preset.CollectionIDs = collectionIDs
	return preset, nil
}

// GetByName is the conversion hot path; it deliberately does not load the
// assignment lists. Use AllowsAsset to evaluate availability.
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

func (s *PresetService) Create(ctx context.Context, label, name string, params domain.PresetParameters, libraryWide bool, collectionIDs []string) (*domain.Preset, error) {
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

	collectionIDs, err = normalizeTargets(libraryWide, collectionIDs)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	preset := &domain.Preset{
		ID:            uuid.New().String(),
		Label:         label,
		Name:          name,
		Parameters:    params,
		LibraryWide:   libraryWide,
		CollectionIDs: collectionIDs,
		CreatedAt:     now,
		ModifiedAt:    now,
	}
	model, err := store.PresetModel(preset)
	if err != nil {
		return nil, err
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateTargets(tx, collectionIDs); err != nil {
			return err
		}
		if err := tx.Create(model).Error; err != nil {
			return err
		}
		return replaceTargets(tx, preset.ID, collectionIDs)
	}); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, preset.ID)
}

func (s *PresetService) Update(ctx context.Context, id, label, name string, params domain.PresetParameters, libraryWide bool, collectionIDs []string) (*domain.Preset, error) {
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

	collectionIDs, err = normalizeTargets(libraryWide, collectionIDs)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateTargets(tx, collectionIDs); err != nil {
			return err
		}
		if err := tx.Model(&store.Preset{}).
			Where("id = ?", id).
			Updates(map[string]any{
				"label":        label,
				"name":         name,
				"parameters":   string(encoded),
				"library_wide": libraryWide,
				"modified_at":  time.Now().UTC(),
			}).Error; err != nil {
			return err
		}
		return replaceTargets(tx, id, collectionIDs)
	}); err != nil {
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

// AllowsAsset reports whether an asset may use a preset. Library-wide presets
// always pass; otherwise the asset inherits the preset from a collection
// membership.
func (s *PresetService) AllowsAsset(ctx context.Context, presetID, assetID string) (bool, error) {
	var preset store.Preset
	err := s.db.WithContext(ctx).
		Select("id", "library_wide").
		First(&preset, "id = ?", presetID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, domain.ErrNotFound
	}
	if err != nil {
		return false, err
	}
	if preset.LibraryWide {
		return true, nil
	}

	var count int64
	if err := s.db.WithContext(ctx).Raw(
		`SELECT COUNT(*) FROM asset_collections ac
		 JOIN preset_collections pc ON pc.collection_id = ac.collection_id
		 WHERE ac.asset_id = ? AND pc.preset_id = ?`,
		assetID, presetID,
	).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// AllowedForAsset returns every preset the asset may use, ordered by name.
func (s *PresetService) AllowedForAsset(ctx context.Context, assetID string) ([]domain.Preset, error) {
	var models []store.Preset
	if err := s.db.WithContext(ctx).Raw(
		`SELECT * FROM presets p
		 WHERE p.library_wide = 1
		    OR p.id IN (
		        SELECT pc.preset_id FROM preset_collections pc
		        JOIN asset_collections ac ON ac.collection_id = pc.collection_id
		        WHERE ac.asset_id = ?
		    )
		 ORDER BY p.name ASC`,
		assetID,
	).Scan(&models).Error; err != nil {
		return nil, err
	}
	presets := make([]domain.Preset, 0, len(models))
	for i := range models {
		presets = append(presets, *models[i].ToDomain())
	}
	return presets, nil
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

func (s *PresetService) loadTargets(ctx context.Context, presetID string) ([]string, error) {
	var collectionIDs []string
	if err := s.db.WithContext(ctx).
		Table("preset_collections").
		Where("preset_id = ?", presetID).
		Order("collection_id").
		Pluck("collection_id", &collectionIDs).Error; err != nil {
		return nil, err
	}
	if collectionIDs == nil {
		collectionIDs = []string{}
	}
	return collectionIDs, nil
}

// targets returns presetID -> targetID for one assignment table.
func (s *PresetService) targets(ctx context.Context, table, column string) map[string][]string {
	var rows []struct {
		PresetID string `gorm:"column:preset_id"`
		TargetID string `gorm:"column:target_id"`
	}
	query := fmt.Sprintf("SELECT preset_id, %s AS target_id FROM %s ORDER BY %s", column, table, column)
	out := make(map[string][]string)
	if err := s.db.WithContext(ctx).Raw(query).Scan(&rows).Error; err != nil {
		return out
	}
	for _, row := range rows {
		out[row.PresetID] = append(out[row.PresetID], row.TargetID)
	}
	return out
}

func normalizeTargets(libraryWide bool, collectionIDs []string) ([]string, error) {
	if libraryWide {
		return []string{}, nil
	}
	collections := normalizeIDs(collectionIDs)
	if len(collections) > maxPresetTargets {
		return nil, fmt.Errorf("%w: too many preset targets", ErrInvalidInput)
	}
	return collections, nil
}

func normalizeIDs(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func validateTargets(tx *gorm.DB, collectionIDs []string) error {
	if len(collectionIDs) > 0 {
		var count int64
		if err := tx.Model(&store.Collection{}).Where("id IN ?", collectionIDs).Count(&count).Error; err != nil {
			return err
		}
		if int(count) != len(collectionIDs) {
			return fmt.Errorf("%w: one or more collections do not exist", ErrInvalidInput)
		}
	}
	return nil
}

func replaceTargets(tx *gorm.DB, presetID string, collectionIDs []string) error {
	if err := tx.Exec(`DELETE FROM preset_collections WHERE preset_id = ?`, presetID).Error; err != nil {
		return err
	}
	for _, id := range collectionIDs {
		if err := tx.Exec(`INSERT OR IGNORE INTO preset_collections (preset_id, collection_id) VALUES (?, ?)`, presetID, id).Error; err != nil {
			return err
		}
	}
	return nil
}
