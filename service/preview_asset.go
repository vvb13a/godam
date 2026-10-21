package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/store"
	"gorm.io/gorm"
)

// refreshVisualAsset keeps the derived visual asset in sync with its parent.
// Images are their own visual source, so any stale derived visual is removed.
func (s *AssetService) refreshVisualAsset(ctx context.Context, parent *domain.Asset, source io.Reader) error {
	if strings.HasPrefix(parent.MimeType, "image/") {
		return s.deleteVisualAsset(ctx, parent.ID)
	}
	data, contentType, err := s.generateVisualAsset(ctx, parent.MimeType, source)
	if err != nil {
		return err
	}
	meta, _ := s.extractor.Extract(ctx, contentType, bytes.NewReader(data), int64(len(data)))
	return s.upsertVisualAsset(ctx, parent, data, contentType, meta)
}

func (s *AssetService) generateVisualAsset(ctx context.Context, mimeType string, source io.Reader) ([]byte, string, error) {
	var reader io.Reader
	if out, err := s.previewGen.Generate(ctx, mimeType, source); err == nil && out != nil {
		reader = out
	} else if strings.HasPrefix(mimeType, "audio/") {
		reader = s.previewGen.pngFallback()
	} else {
		reader = s.previewGen.GetFallback(mimeType)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", err
	}
	ext := previewExtension(mimeType)
	contentType := "image/jpeg"
	if ext == ".png" {
		contentType = "image/png"
	}
	return data, contentType, nil
}

func (s *AssetService) upsertVisualAsset(ctx context.Context, parent *domain.Asset, data []byte, contentType string, meta domain.Metadata) error {
	ext := ".jpg"
	if contentType == "image/png" {
		ext = ".png"
	}
	now := time.Now().UTC()

	var existing store.Asset
	err := s.db.WithContext(ctx).
		Where("image_of = ? AND role = ?", parent.ID, domain.AssetRoleVisual).
		Order("created_at DESC").
		First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		id := uuid.NewString()
		key := fmt.Sprintf("uploads/%s%s", id, ext)
		if err := s.local.Save(ctx, key, bytes.NewReader(data)); err != nil {
			return err
		}
		visual := &domain.Asset{
			ID:            id,
			UploadedBy:    parent.UploadedBy,
			Filename:      visualFilename(parent.Filename, ext),
			StorageDriver: "local",
			StorageKey:    key,
			MimeType:      contentType,
			ByteSize:      int64(len(data)),
			Role:          domain.AssetRoleVisual,
			ImageOf:       parent.ID,
			Metadata:      meta,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := s.db.WithContext(ctx).Create(store.AssetModel(visual)).Error; err != nil {
			_ = s.local.Delete(ctx, key)
			return err
		}
		return nil
	}

	key := existing.StorageKey
	oldKey := key
	if filepath.Ext(key) != ext {
		key = fmt.Sprintf("uploads/%s%s", existing.ID, ext)
	}
	if err := s.local.Save(ctx, key, bytes.NewReader(data)); err != nil {
		return err
	}
	if oldKey != key {
		_ = s.local.Delete(ctx, oldKey)
	}
	return s.db.WithContext(ctx).
		Model(&store.Asset{}).
		Where("id = ?", existing.ID).
		Updates(map[string]any{
			"filename":       visualFilename(parent.Filename, ext),
			"storage_driver": "local",
			"storage_key":    key,
			"mime_type":      contentType,
			"byte_size":      int64(len(data)),
			"width":          meta.Width,
			"height":         meta.Height,
			"raw_metadata":   meta.RawJSON,
			"updated_at":     now,
		}).Error
}

func (s *AssetService) deleteVisualAsset(ctx context.Context, parentID string) error {
	var visuals []store.Asset
	if err := s.db.WithContext(ctx).
		Where("image_of = ? AND role = ?", parentID, domain.AssetRoleVisual).
		Find(&visuals).Error; err != nil {
		return err
	}
	for _, visual := range visuals {
		_ = s.local.Delete(ctx, visual.StorageKey)
		if err := s.db.WithContext(ctx).Delete(&store.Asset{}, "id = ?", visual.ID).Error; err != nil {
			return err
		}
	}
	return nil
}

func supportsVisualAsset(mimeType string) bool {
	return strings.HasPrefix(mimeType, "audio/") ||
		strings.HasPrefix(mimeType, "video/") ||
		mimeType == "application/pdf"
}

func (s *AssetService) ensureVisualAsset(ctx context.Context, parent *domain.Asset) error {
	rc, err := s.Open(ctx, parent.ID)
	if err != nil {
		return err
	}
	defer rc.Close()
	return s.refreshVisualAsset(ctx, parent, rc)
}

func visualFilename(name, ext string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if base == "" {
		base = "visual"
	}
	return base + "-visual" + ext
}

// ResolveDeliveryImage returns the image asset that represents id: the asset
// itself for images, or the derived visual asset linked through image_of for
// audio/video/pdf.
func (s *AssetService) ResolveDeliveryImage(ctx context.Context, id string) (*domain.Asset, error) {
	var model store.Asset
	err := s.db.WithContext(ctx).First(&model, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	asset := model.ToDomain()
	if strings.HasPrefix(asset.MimeType, "image/") {
		return asset, nil
	}

	var visual store.Asset
	query := func() error {
		return s.db.WithContext(ctx).
			Where("image_of = ? AND role = ?", id, domain.AssetRoleVisual).
			Order("created_at DESC").
			First(&visual).Error
	}
	err = query()
	if errors.Is(err, gorm.ErrRecordNotFound) && supportsVisualAsset(asset.MimeType) {
		// Older assets and failed uploads may not have a derived visual yet.
		// Generate it lazily so /img and the app preview both heal themselves.
		if genErr := s.ensureVisualAsset(ctx, asset); genErr == nil {
			err = query()
		}
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	resolved := visual.ToDomain()
	if !strings.HasPrefix(resolved.MimeType, "image/") {
		return nil, domain.ErrNotFound
	}
	return resolved, nil
}

// ResolveSourceAsset returns an original that can be streamed by /src. Images
// and derived visual assets are not source deliverables.
func (s *AssetService) ResolveSourceAsset(ctx context.Context, id string) (*domain.Asset, error) {
	var model store.Asset
	err := s.db.WithContext(ctx).First(&model, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	asset := model.ToDomain()
	if asset.Role == domain.AssetRoleVisual || strings.HasPrefix(asset.MimeType, "image/") {
		return nil, domain.ErrNotFound
	}
	switch {
	case strings.HasPrefix(asset.MimeType, "audio/"),
		strings.HasPrefix(asset.MimeType, "video/"),
		asset.MimeType == "application/pdf":
		return asset, nil
	}
	return nil, domain.ErrNotFound
}
