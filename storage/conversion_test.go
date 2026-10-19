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
	if err := store.Delete(ctx, "asset/800x600.jpg"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.Open(ctx, "asset/800x600.jpg"); err == nil {
		t.Fatal("expected missing file")
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
