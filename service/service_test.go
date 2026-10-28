package service

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"path/filepath"
	"testing"

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
