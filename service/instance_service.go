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

// InstanceService manages the single library this process serves.
type InstanceService struct {
	db     *gorm.DB
	assets *AssetService
}

func NewInstanceService(db *gorm.DB, assets *AssetService) *InstanceService {
	return &InstanceService{db: db, assets: assets}
}

// Ensure creates the instance row on first boot and returns it thereafter.
func (s *InstanceService) Ensure(ctx context.Context, name string) (*domain.Instance, error) {
	inst, err := s.Get(ctx)
	if err == nil {
		return inst, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		name = "Library"
	}
	now := time.Now().UTC()
	model := &store.Instance{
		ID:        uuid.New().String(),
		Name:      name,
		Color:     "8b5cf6",
		CreatedAt: now,
		UpdatedAt: now,
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(model).Error; err != nil {
			return err
		}
		return seedDefaultSmartCollections(tx, now)
	})
	if err != nil {
		return nil, err
	}
	return model.ToDomain(), nil
}

func (s *InstanceService) Get(ctx context.Context) (*domain.Instance, error) {
	var model store.Instance
	err := s.db.WithContext(ctx).Order("created_at ASC").First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return model.ToDomain(), nil
}

func (s *InstanceService) Update(ctx context.Context, name, color string) (*domain.Instance, error) {
	inst, err := s.Get(ctx)
	if err != nil {
		return nil, err
	}
	err = s.db.WithContext(ctx).
		Model(&store.Instance{}).
		Where("id = ?", inst.ID).
		Updates(map[string]any{
			"name":       name,
			"color":      color,
			"updated_at": time.Now().UTC(),
		}).Error
	if err != nil {
		return nil, err
	}
	return s.Get(ctx)
}

// SetCover sets (or clears) the instance cover image.
func (s *InstanceService) SetCover(ctx context.Context, assetID string) (*domain.Instance, error) {
	return s.setAssetRef(ctx, assetID, "cover_asset_id")
}

// SetWatermark sets (or clears) the instance default watermark asset.
func (s *InstanceService) SetWatermark(ctx context.Context, assetID string) (*domain.Instance, error) {
	return s.setAssetRef(ctx, assetID, "watermark_asset_id")
}

func (s *InstanceService) SetUploadSettings(ctx context.Context, allowed []string, maxUploadBytes int64) (*domain.Instance, error) {
	inst, err := s.Get(ctx)
	if err != nil {
		return nil, err
	}
	encoded := ""
	cleaned := make([]string, 0, len(allowed))
	for _, m := range allowed {
		if trimmed := strings.TrimSpace(m); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	if len(cleaned) > 0 {
		buf, err := json.Marshal(cleaned)
		if err != nil {
			return nil, err
		}
		encoded = string(buf)
	}
	if maxUploadBytes < 0 {
		maxUploadBytes = 0
	}
	if err := s.db.WithContext(ctx).
		Model(&store.Instance{}).
		Where("id = ?", inst.ID).
		Updates(map[string]any{
			"allowed_mime_types": encoded,
			"max_upload_bytes":   maxUploadBytes,
			"updated_at":         time.Now().UTC(),
		}).Error; err != nil {
		return nil, err
	}
	return s.Get(ctx)
}

func (s *InstanceService) setAssetRef(ctx context.Context, assetID, column string) (*domain.Instance, error) {
	inst, err := s.Get(ctx)
	if err != nil {
		return nil, err
	}
	if assetID != "" {
		var count int64
		if err := s.db.WithContext(ctx).Model(&store.Asset{}).Where("id = ?", assetID).Count(&count).Error; err != nil {
			return nil, err
		}
		if count == 0 {
			return nil, domain.ErrNotFound
		}
	}
	if err := s.db.WithContext(ctx).
		Model(&store.Instance{}).
		Where("id = ?", inst.ID).
		Updates(map[string]any{
			column:       nullableString(assetID),
			"updated_at": time.Now().UTC(),
		}).Error; err != nil {
		return nil, err
	}
	return s.Get(ctx)
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
