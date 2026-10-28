package service

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"path/filepath"
	"testing"

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
