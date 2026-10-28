package service

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"path/filepath"
	"testing"
	"time"

	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/store"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

// testPNG returns a valid 1x1 PNG used by upload tests, since uploads are
// restricted to image/audio/pdf/video types.
func testPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestInstance(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	instances := NewInstanceService(db, nil)

	inst, err := instances.Ensure(ctx, "Test")
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	if inst.Name != "Test" || inst.ID == "" {
		t.Fatalf("unexpected instance: %+v", inst)
	}
	again, err := instances.Ensure(ctx, "Ignored")
	if err != nil || again.ID != inst.ID {
		t.Fatalf("ensure should be idempotent: %+v err=%v", again, err)
	}
	updated, err := instances.Update(ctx, "Renamed", "ef4444")
	if err != nil || updated.Name != "Renamed" || updated.Color != "ef4444" {
		t.Fatalf("update: %+v err=%v", updated, err)
	}
}

func TestSmartCollections(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	instances := NewInstanceService(db, nil)
	smart := NewSmartCollectionService(db)

	if _, err := instances.Ensure(ctx, "L"); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	created, err := smart.Create(ctx, "Videos", "", "3b82f6", domain.SmartCollectionFilter{
		Type: "video",
		Sort: "name-asc",
	})
	if err != nil {
		t.Fatalf("create smart collection: %v", err)
	}

	list, err := smart.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: n=%d err=%v", len(list), err)
	}
	var videos *domain.SmartCollection
	for i := range list {
		if list[i].Name == "Videos" {
			videos = &list[i]
		}
	}
	if videos == nil {
		t.Fatalf("created smart collection missing from list: %+v", list)
	}
	if videos.Filter.Type != "video" || videos.Filter.Sort != "name-asc" {
		t.Fatalf("filter not persisted: %+v", videos.Filter)
	}

	if _, err := smart.Create(ctx, "Videos", "", "3b82f6", domain.SmartCollectionFilter{Type: "video"}); err != nil {
		t.Fatalf("duplicate name should be auto-suffixed: %v", err)
	}

	updated, err := smart.Update(ctx, created.ID, "Clips", "", "ef4444")
	if err != nil || updated.Name != "Clips" || updated.Color != "ef4444" {
		t.Fatalf("update: %+v err=%v", updated, err)
	}

	if err := smart.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := smart.GetByID(ctx, created.ID); err == nil {
		t.Fatal("expected deleted smart collection to be gone")
	}
}

func TestCollectionHierarchy(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	cols := NewCollectionService(db)

	root, err := cols.Create(ctx, nil, "Root", "", "8b5cf6")
	if err != nil {
		t.Fatalf("create root: %v", err)
	}
	child, err := cols.Create(ctx, &root.ID, "Child", "", "")
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	grandchild, err := cols.Create(ctx, &child.ID, "Grandchild", "", "")
	if err != nil {
		t.Fatalf("create grandchild: %v", err)
	}

	if _, err := cols.Create(ctx, nil, "Root", "", ""); err != nil {
		t.Fatalf("duplicate name should be auto-suffixed: %v", err)
	}
	list, err := cols.List(ctx)
	if err != nil || len(list) != 4 {
		t.Fatalf("expected 4 collections, got %d err=%v", len(list), err)
	}

	// Moving a root under its own descendant must be rejected.
	if err := cols.Move(ctx, root.ID, &grandchild.ID, 1); err == nil {
		t.Fatal("cycle should be rejected")
	}

	// Reparent the grandchild to the root and insert it first.
	if err := cols.Move(ctx, grandchild.ID, nil, 0); err != nil {
		t.Fatalf("move to root: %v", err)
	}
	moved, err := cols.GetByID(ctx, grandchild.ID)
	if err != nil || moved.ParentID != nil {
		t.Fatalf("grandchild should be a root now: %+v err=%v", moved, err)
	}

	// Sibling ordering: the moved node should be first among roots.
	var firstRoot struct {
		ID string
	}
	if err := db.Model(&struct{}{}).
		Table("collections").
		Select("id").
		Where("parent_id IS NULL").
		Order("sort ASC").
		Limit(1).
		Scan(&firstRoot).Error; err != nil {
		t.Fatalf("query first root: %v", err)
	}
	if firstRoot.ID != grandchild.ID {
		t.Fatalf("expected moved node first, got %s", firstRoot.ID)
	}
}

func TestBulkAssetRelations(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	cols := NewCollectionService(db)
	assets := &AssetService{db: db}

	col, err := cols.Create(ctx, nil, "C", "", "")
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	for _, id := range []string{"a1", "a2"} {
		if err := db.Create(&store.Asset{
			ID: id, Filename: id + ".jpg",
			StorageDriver: "local", StorageKey: id, MimeType: "image/jpeg",
			ByteSize: 1, CreatedAt: time.Now().UTC(),
		}).Error; err != nil {
			t.Fatalf("create asset %s: %v", id, err)
		}
	}

	countRows := func(table, column, value string) int64 {
		t.Helper()
		var n int64
		if err := db.Table(table).Where(column+" = ?", value).Count(&n).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return n
	}

	// Bulk add to collection links both assets in one call.
	if err := assets.AddToCollectionBulk(ctx, col.ID, []string{"a1", "a2"}); err != nil {
		t.Fatalf("add to collection bulk: %v", err)
	}
	if n := countRows("asset_collections", "collection_id", col.ID); n != 2 {
		t.Fatalf("expected 2 collection links, got %d", n)
	}

	// Replacing with an empty set clears collection membership.
	if err := assets.ApplyCollectionsBulk(ctx, []string{"a1", "a2"}, nil, "replace"); err != nil {
		t.Fatalf("clear collections bulk: %v", err)
	}
	if n := countRows("asset_collections", "collection_id", col.ID); n != 0 {
		t.Fatalf("expected cleared collections, got %d", n)
	}
}
