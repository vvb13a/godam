package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/storage"
	"github.com/vvb13a/godam/store"
)

func newDeliveryTestService(t *testing.T) (*AssetService, *StatsService) {
	t.Helper()
	db := newTestDB(t)
	dir := t.TempDir()
	local, err := storage.NewLocal(filepath.Join(dir, "assets"))
	if err != nil {
		t.Fatalf("local storage: %v", err)
	}
	registry := storage.NewRegistry("local")
	registry.Register("local", local)
	cfg := DefaultConfig()
	cfg.StoragePath = filepath.Join(dir, "assets")
	assets := NewAssetService(
		cfg, db, registry, local,
		NewCompositeExtractor(), NewPreviewGenerator(600, 85),
		nil,
	)
	return assets, NewStatsService(db, nil)
}

func insertAsset(t *testing.T, assets *AssetService, id, mimeType, role, imageOf string) {
	t.Helper()
	now := time.Now().UTC()
	asset := &domain.Asset{
		ID:            id,
		Filename:      id,
		StorageDriver: "local",
		StorageKey:    "uploads/" + id,
		MimeType:      mimeType,
		ByteSize:      10,
		Role:          role,
		ImageOf:       imageOf,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := assets.db.Create(store.AssetModel(asset)).Error; err != nil {
		t.Fatalf("insert %s: %v", id, err)
	}
}

func TestDefaultPreviewParameters(t *testing.T) {
	processing, err := BuildProcessing(defaultPreviewParameters)
	if err != nil {
		t.Fatalf("build preview processing: %v", err)
	}
	if processing != "resize:fit:400:400:0:0/quality:80" {
		t.Fatalf("preview processing = %q", processing)
	}
}

func TestDeliveryResolutionAndExclusions(t *testing.T) {
	ctx := context.Background()
	assets, stats := newDeliveryTestService(t)

	insertAsset(t, assets, "image", "image/jpeg", domain.AssetRoleOriginal, "")
	insertAsset(t, assets, "video", "video/mp4", domain.AssetRoleOriginal, "")
	insertAsset(t, assets, "video-visual", "image/jpeg", domain.AssetRoleVisual, "video")
	insertAsset(t, assets, "orphan-video", "video/mp4", domain.AssetRoleOriginal, "")

	resolved, err := assets.ResolveDeliveryImage(ctx, "image")
	if err != nil || resolved.ID != "image" {
		t.Fatalf("image resolve: %+v err=%v", resolved, err)
	}
	resolved, err = assets.ResolveDeliveryImage(ctx, "video")
	if err != nil || resolved.ID != "video-visual" {
		t.Fatalf("video resolve: %+v err=%v", resolved, err)
	}
	if _, err := assets.ResolveDeliveryImage(ctx, "orphan-video"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("video without preview should be not found, got %v", err)
	}

	if _, err := assets.ResolveSourceAsset(ctx, "video"); err != nil {
		t.Fatalf("video source: %v", err)
	}
	if _, err := assets.ResolveSourceAsset(ctx, "image"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("image source should be not found, got %v", err)
	}
	if _, err := assets.ResolveSourceAsset(ctx, "video-visual"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("visual source should be not found, got %v", err)
	}

	page, err := assets.List(ctx, domain.AssetFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if page.Total != 3 || len(page.Items) != 3 {
		t.Fatalf("list should contain only originals, got %d/%d", page.Total, len(page.Items))
	}
	for _, item := range page.Items {
		if item.Role != domain.AssetRoleOriginal {
			t.Fatalf("preview leaked into list: %+v", item)
		}
	}

	statsResult, err := stats.Get(ctx)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if statsResult.Assets != 3 {
		t.Fatalf("stats assets = %d, want 3", statsResult.Assets)
	}
	if statsResult.StorageBytes != 30 {
		t.Fatalf("stats bytes = %d, want 30", statsResult.StorageBytes)
	}
}
