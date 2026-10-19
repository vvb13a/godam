package storage

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestLocalConversions(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalConversions(dir, "https://assets.example.com/")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	ctx := context.Background()

	if err := store.Save(ctx, "asset/800x600.jpg", strings.NewReader("jpeg-bytes")); err != nil {
		t.Fatalf("save: %v", err)
	}
	rc, err := store.Open(ctx, "asset/800x600.jpg")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	data, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil || string(data) != "jpeg-bytes" {
		t.Fatalf("read = %q err=%v", data, err)
	}
	if got := store.URL("asset/800x600.jpg"); got != "https://assets.example.com/asset/800x600.jpg" {
		t.Fatalf("url = %q", got)
	}

	if err := store.Save(ctx, "asset/10x10.png", strings.NewReader("x")); err != nil {
		t.Fatalf("save second: %v", err)
	}
	entries, err := store.List(ctx, "asset")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %+v", entries)
	}
	if entries[0].Key != "asset/10x10.png" || entries[0].ByteSize != 1 {
		t.Fatalf("first entry = %+v", entries[0])
	}
	if entries[1].Key != "asset/800x600.jpg" || entries[1].ByteSize != int64(len("jpeg-bytes")) {
		t.Fatalf("second entry = %+v", entries[1])
	}
	if missing, err := store.List(ctx, "missing"); err != nil || missing != nil {
		t.Fatalf("missing prefix = %+v err=%v", missing, err)
	}

	if err := store.Delete(ctx, "asset/800x600.jpg"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.Open(ctx, "asset/800x600.jpg"); err == nil {
		t.Fatal("expected missing file")
	}

	if err := store.Save(ctx, "asset/a.webp", strings.NewReader("a")); err != nil {
		t.Fatalf("save a: %v", err)
	}
	if err := store.Save(ctx, "asset/b.webp", strings.NewReader("bb")); err != nil {
		t.Fatalf("save b: %v", err)
	}
	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.Count != 3 || stats.Bytes != 4 {
		t.Fatalf("stats = %+v, want count=3 bytes=4", stats)
	}

	removed, err := store.Purge(ctx)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if removed != 3 {
		t.Fatalf("removed = %d, want 3", removed)
	}
	entries, err = store.List(ctx, "asset")
	if err != nil || len(entries) != 0 {
		t.Fatalf("entries after purge = %+v err=%v", entries, err)
	}
}

func TestLocalConversionsRejectsTraversal(t *testing.T) {
	store, err := NewLocalConversions(t.TempDir(), "")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := store.Save(context.Background(), "../escape.jpg", strings.NewReader("x")); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
}
