package service

import (
	"context"
	"errors"
	"testing"
	"time"

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

	created, err := presets.Create(ctx, "user-1", "Thumbnail", "thumbnail", testPresetParameters(), true, nil, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Name != "thumbnail" || created.Parameters.Format != "webp" {
		t.Fatalf("created = %+v", created)
	}

	if _, err := presets.Create(ctx, "user-1", "Duplicate", "thumbnail", testPresetParameters(), true, nil, nil); !errors.Is(err, ErrPresetNameTaken) {
		t.Fatalf("duplicate error = %v", err)
	}

	found, err := presets.GetByName(ctx, "thumbnail")
	if err != nil || found.ID != created.ID {
		t.Fatalf("get by name: %+v err=%v", found, err)
	}

	updated, err := presets.Update(ctx, created.ID, "Small thumbnail", "small", testPresetParameters(), true, nil, nil)
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

	if _, err := presets.Create(ctx, "user-1", "", "empty-label", testPresetParameters(), true, nil, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty label error = %v", err)
	}
	if _, err := presets.Create(ctx, "user-1", "Label", "Bad Name", testPresetParameters(), true, nil, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("bad name error = %v", err)
	}
	if _, err := presets.Create(ctx, "user-1", "Label", "800x600", testPresetParameters(), true, nil, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("reserved name error = %v", err)
	}
}

func TestPresetAvailability(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	presets := NewPresetService(db)
	collections := NewCollectionService(db)
	tags := NewTagService(db)

	collection, err := collections.Create(ctx, nil, "Campaign", "", "")
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	tag, err := tags.GetOrCreate(ctx, "Needs thumbnail")
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}

	now := time.Now().UTC()
	for _, id := range []string{"asset-1", "asset-2"} {
		if err := db.Exec(
			`INSERT INTO assets (id, filename, storage_driver, storage_key, mime_type, byte_size, created_at, updated_at)
			 VALUES (?, ?, 'local', ?, 'image/jpeg', 1, ?, ?)`,
			id, id+".jpg", "uploads/"+id+".jpg", now, now,
		).Error; err != nil {
			t.Fatalf("insert asset: %v", err)
		}
	}
	if err := db.Exec(`INSERT INTO asset_collections (asset_id, collection_id) VALUES (?, ?)`, "asset-1", collection.ID).Error; err != nil {
		t.Fatalf("link collection: %v", err)
	}
	if err := db.Exec(`INSERT INTO asset_tags (asset_id, tag_id) VALUES (?, ?)`, "asset-2", tag.ID).Error; err != nil {
		t.Fatalf("link tag: %v", err)
	}

	restricted, err := presets.Create(ctx, "user", "Restricted", "restricted", testPresetParameters(), false, []string{collection.ID}, []string{tag.ID})
	if err != nil {
		t.Fatalf("create restricted preset: %v", err)
	}
	if len(restricted.CollectionIDs) != 1 || len(restricted.TagIDs) != 1 {
		t.Fatalf("targets = %+v", restricted)
	}

	for _, assetID := range []string{"asset-1", "asset-2"} {
		allowed, err := presets.AllowsAsset(ctx, restricted.ID, assetID)
		if err != nil || !allowed {
			t.Fatalf("%s should be allowed: %v", assetID, err)
		}
	}
	if allowed, err := presets.AllowsAsset(ctx, restricted.ID, "asset-3"); err != nil || allowed {
		t.Fatalf("asset-3 should not be allowed: allowed=%v err=%v", allowed, err)
	}

	for _, assetID := range []string{"asset-1", "asset-2"} {
		allowed, err := presets.AllowedForAsset(ctx, assetID)
		if err != nil {
			t.Fatalf("allowed for %s: %v", assetID, err)
		}
		found := false
		for _, preset := range allowed {
			if preset.ID == restricted.ID {
				found = true
			}
		}
		if !found {
			t.Fatalf("restricted preset missing from %s", assetID)
		}
	}
	if allowed, err := presets.AllowedForAsset(ctx, "asset-3"); err != nil || len(allowed) != 0 {
		t.Fatalf("asset-3 presets = %+v err=%v", allowed, err)
	}

	global, err := presets.Create(ctx, "user", "Global", "global", testPresetParameters(), true, nil, nil)
	if err != nil {
		t.Fatalf("create global preset: %v", err)
	}
	if allowed, err := presets.AllowsAsset(ctx, global.ID, "asset-3"); err != nil || !allowed {
		t.Fatalf("global preset should allow any asset: allowed=%v err=%v", allowed, err)
	}
}
