package service

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vvb13a/godam/storage"
)

func TestStatsIncludeConversions(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	conversions, err := storage.NewLocalConversions(filepath.Join(t.TempDir(), "conversions"), "")
	if err != nil {
		t.Fatalf("conversions: %v", err)
	}
	if err := conversions.Save(ctx, "asset-1/thumb.webp", strings.NewReader("abc")); err != nil {
		t.Fatalf("save conversion: %v", err)
	}

	stats, err := NewStatsService(db, conversions).Get(ctx)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.Conversions != 1 || stats.ConversionBytes != 3 {
		t.Fatalf("conversion stats = %d / %d", stats.Conversions, stats.ConversionBytes)
	}
}

func TestStatsCacheInvalidation(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	conversions, err := storage.NewLocalConversions(filepath.Join(t.TempDir(), "conversions"), "")
	if err != nil {
		t.Fatalf("conversions: %v", err)
	}
	statsService := NewStatsService(db, conversions)

	if err := conversions.Save(ctx, "asset-1/a.webp", strings.NewReader("a")); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := statsService.Get(ctx); err != nil {
		t.Fatalf("stats: %v", err)
	}
	if _, err := conversions.Purge(ctx); err != nil {
		t.Fatalf("purge: %v", err)
	}
	statsService.Invalidate()
	stats, err := statsService.Get(ctx)
	if err != nil {
		t.Fatalf("stats after purge: %v", err)
	}
	if stats.Conversions != 0 || stats.ConversionBytes != 0 {
		t.Fatalf("conversion stats after purge = %d / %d", stats.Conversions, stats.ConversionBytes)
	}
}
