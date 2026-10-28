package service

import (
	"context"
	"errors"
	"testing"

	"github.com/vvb13a/godam/domain"
)

func testPresetParameters() domain.PresetParameters {
	return domain.PresetParameters{
		Format: "webp",
		Options: []domain.PresetOption{
			{Name: "resize", Args: []string{"fit", "400", "300"}},
			{Name: "quality", Args: []string{"82"}},
		},
	}
}

func TestPresetServiceCRUD(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	presets := NewPresetService(db)

	created, err := presets.Create(ctx, "Thumbnail", "thumbnail", testPresetParameters())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Name != "thumbnail" || created.Parameters.Format != "webp" {
		t.Fatalf("created = %+v", created)
	}

	if _, err := presets.Create(ctx, "Duplicate", "thumbnail", testPresetParameters()); !errors.Is(err, ErrPresetNameTaken) {
		t.Fatalf("duplicate error = %v", err)
	}

	found, err := presets.GetByName(ctx, "thumbnail")
	if err != nil || found.ID != created.ID {
		t.Fatalf("get by name: %+v err=%v", found, err)
	}

	updated, err := presets.Update(ctx, created.ID, "Small thumbnail", "small", testPresetParameters())
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "small" || updated.Label != "Small thumbnail" {
		t.Fatalf("updated = %+v", updated)
	}

	list, err := presets.List(ctx)
	if err != nil || len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("list = %+v err=%v", list, err)
	}

	if err := presets.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := presets.GetByID(ctx, created.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestPresetServiceValidation(t *testing.T) {
	db := newTestDB(t)
	presets := NewPresetService(db)
	ctx := context.Background()

	if _, err := presets.Create(ctx, "", "empty-label", testPresetParameters()); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty label error = %v", err)
	}
	if _, err := presets.Create(ctx, "Label", "Bad Name", testPresetParameters()); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("bad name error = %v", err)
	}
	if _, err := presets.Create(ctx, "Label", "800x600", testPresetParameters()); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("reserved name error = %v", err)
	}
}

func TestPresetAvailability(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	presets := NewPresetService(db)

	created, err := presets.Create(ctx, "Thumbnail", "thumbnail", testPresetParameters())
	if err != nil {
		t.Fatalf("create preset: %v", err)
	}
	for _, assetID := range []string{"asset-1", "asset-2"} {
		allowed, err := presets.AllowsAsset(ctx, created.ID, assetID)
		if err != nil || !allowed {
			t.Fatalf("%s should be allowed: %v", assetID, err)
		}
	}
	allowed, err := presets.AllowedForAsset(ctx, "asset-1")
	if err != nil || len(allowed) != 1 || allowed[0].ID != created.ID {
		t.Fatalf("allowed presets = %+v err=%v", allowed, err)
	}
}
