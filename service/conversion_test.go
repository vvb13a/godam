package service

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/storage"
	"github.com/vvb13a/godam/store"
)

func TestConversionServiceCachesImgproxyOutput(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	db, err := store.Open(filepath.Join(dir, "conv.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
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

	var source bytes.Buffer
	if err := png.Encode(&source, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	asset, err := assets.Upload(ctx, "", "test.png", "image/png", int64(source.Len()), bytes.NewReader(source.Bytes()))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	var calls atomic.Int32
	imgproxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "image/webp")
		_, _ = w.Write([]byte("webp-bytes"))
	}))
	defer imgproxy.Close()

	proxy, err := NewImgProxy(imgproxy.URL, "http://godam:8080", "736563726574", "68656c6c6f")
	if err != nil {
		t.Fatalf("new imgproxy: %v", err)
	}
	conversions, err := storage.NewLocalConversions(filepath.Join(dir, "cache"), "https://assets.example.com")
	if err != nil {
		t.Fatalf("conversion store: %v", err)
	}
	presets := NewPresetService(db)
	svc := NewConversionService(assets, presets, proxy, conversions)

	first, err := svc.PreviewDefault(ctx, asset.ID, asset.ID)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	data, err := io.ReadAll(first.Reader)
	_ = first.Reader.Close()
	if err != nil || string(data) != "webp-bytes" {
		t.Fatalf("first result = %q err=%v", data, err)
	}
	if first.CacheHit {
		t.Fatal("first conversion should be a miss")
	}

	second, err := svc.PreviewDefault(ctx, asset.ID, asset.ID)
	if err != nil {
		t.Fatalf("second preview: %v", err)
	}
	data, err = io.ReadAll(second.Reader)
	_ = second.Reader.Close()
	if err != nil || string(data) != "webp-bytes" {
		t.Fatalf("second result = %q err=%v", data, err)
	}
	if !second.CacheHit {
		t.Fatal("second conversion should hit the cache")
	}
	if calls.Load() != 1 {
		t.Fatalf("imgproxy calls = %d, want 1", calls.Load())
	}
	previewKey := asset.ID + "/_library-preview.webp"
	if got := conversions.URL(previewKey); got != "https://assets.example.com/"+previewKey {
		t.Fatalf("url = %q", got)
	}

	// Legacy size entries are still describable in the cache listing, but no
	// new size conversions are generated or served.
	if err := conversions.Save(ctx, asset.ID+"/10x10.png", strings.NewReader("x")); err != nil {
		t.Fatalf("save legacy conversion: %v", err)
	}
	list, err := svc.List(ctx, asset.ID)
	if err != nil {
		t.Fatalf("list conversions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("conversions = %+v", list)
	}
	if list[0].Width != 10 || list[0].Height != 10 || list[0].Format != "png" || list[0].ByteSize != 1 {
		t.Fatalf("smallest conversion = %+v", list[0])
	}
	if list[0].URL != "https://assets.example.com/"+list[0].Key {
		t.Fatalf("smallest URL = %q", list[0].URL)
	}

	preset, err := presets.Create(ctx, "Thumb", "thumb", domain.PresetParameters{
		Format: "webp",
		Options: []domain.PresetOption{
			{Name: "resize", Args: []string{"fit", "200", "0"}},
			{Name: "quality", Args: []string{"82"}},
		},
	})
	if err != nil {
		t.Fatalf("create preset: %v", err)
	}
	_ = preset

	presetResult, err := svc.ConvertPreset(ctx, asset.ID, "thumb.webp")
	if err != nil {
		t.Fatalf("convert preset: %v", err)
	}
	presetData, err := io.ReadAll(presetResult.Reader)
	_ = presetResult.Reader.Close()
	if err != nil || string(presetData) != "webp-bytes" {
		t.Fatalf("preset result = %q err=%v", presetData, err)
	}
	if presetResult.Key != asset.ID+"/thumb.webp" {
		t.Fatalf("preset key = %q", presetResult.Key)
	}
	if _, err := svc.ConvertPreset(ctx, asset.ID, "thumb.png"); err == nil {
		t.Fatal("expected preset format mismatch to fail")
	}

	beforePreview, err := svc.List(ctx, asset.ID)
	if err != nil {
		t.Fatalf("list before preview: %v", err)
	}
	preview, contentType, err := svc.Preview(ctx, asset.ID, domain.PresetParameters{
		Format: "webp",
		Options: []domain.PresetOption{
			{Name: "resize", Args: []string{"fit", "50", "50"}},
		},
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	previewData, err := io.ReadAll(preview)
	_ = preview.Close()
	if err != nil || string(previewData) != "webp-bytes" || contentType != "image/webp" {
		t.Fatalf("preview = %q type=%q err=%v", previewData, contentType, err)
	}
	afterPreview, err := svc.List(ctx, asset.ID)
	if err != nil {
		t.Fatalf("list after preview: %v", err)
	}
	if len(afterPreview) != len(beforePreview) {
		t.Fatalf("preview should not be cached: before=%d after=%d", len(beforePreview), len(afterPreview))
	}

	signedSegment, err := proxy.SignRequest(asset.ID, "webp", "rs:fit:300:0")
	if err != nil {
		t.Fatalf("sign request: %v", err)
	}
	signedResult, err := svc.ConvertSigned(ctx, asset.ID, signedSegment)
	if err != nil {
		t.Fatalf("convert signed: %v", err)
	}
	signedData, err := io.ReadAll(signedResult.Reader)
	_ = signedResult.Reader.Close()
	if err != nil || string(signedData) != "webp-bytes" {
		t.Fatalf("signed result = %q err=%v", signedData, err)
	}

	list, err = svc.List(ctx, asset.ID)
	if err != nil {
		t.Fatalf("list conversions after presets: %v", err)
	}
	var foundPreset bool
	for _, conversion := range list {
		if conversion.Kind == "preset" && conversion.Preset == "thumb" && conversion.Label == "Thumb" {
			foundPreset = true
		}
	}
	if !foundPreset {
		t.Fatalf("preset conversion missing from list: %+v", list)
	}
}

func TestConversionServiceInvalidateAsset(t *testing.T) {
	ctx := context.Background()
	conversions, err := storage.NewLocalConversions(t.TempDir(), "")
	if err != nil {
		t.Fatalf("conversion store: %v", err)
	}
	for _, key := range []string{
		"asset-one/_library-preview.webp",
		"asset-one/thumb.webp",
		"asset-two/_library-preview.webp",
	} {
		if err := conversions.Save(ctx, key, strings.NewReader("x")); err != nil {
			t.Fatalf("save %s: %v", key, err)
		}
	}

	svc := &ConversionService{store: conversions}
	if err := svc.InvalidateAsset(ctx, "asset-one"); err != nil {
		t.Fatalf("invalidate: %v", err)
	}

	entries, err := conversions.List(ctx, "asset-one")
	if err != nil {
		t.Fatalf("list invalidated asset: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("asset-one conversions = %+v, want none", entries)
	}
	entries, err = conversions.List(ctx, "asset-two")
	if err != nil {
		t.Fatalf("list other asset: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("asset-two conversions = %+v, want 1", entries)
	}
}

func TestConversionServiceRefreshAssetReplacesCache(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "refresh.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	asset := store.Asset{
		ID:            "asset-1",
		Filename:      "asset.png",
		StorageDriver: "local",
		StorageKey:    "uploads/asset-1.png",
		MimeType:      "image/png",
		ByteSize:      1,
		Role:          domain.AssetRoleOriginal,
	}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}

	imgproxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/webp")
		_, _ = w.Write([]byte("new-bytes"))
	}))
	defer imgproxy.Close()
	proxy, err := NewImgProxy(imgproxy.URL, "http://godam:8080", "736563726574", "68656c6c6f")
	if err != nil {
		t.Fatalf("new imgproxy: %v", err)
	}
	conversions, err := storage.NewLocalConversions(filepath.Join(dir, "cache"), "")
	if err != nil {
		t.Fatalf("conversion store: %v", err)
	}
	presets := NewPresetService(db)
	if _, err := presets.Create(ctx, "Thumb", "thumb", domain.PresetParameters{
		Format: "webp",
		Options: []domain.PresetOption{
			{Name: "resize", Args: []string{"fit", "200", "0"}},
		},
	}); err != nil {
		t.Fatalf("create preset: %v", err)
	}
	if err := conversions.Save(ctx, "asset-1/thumb.webp", strings.NewReader("old-bytes")); err != nil {
		t.Fatalf("seed conversion: %v", err)
	}

	svc := NewConversionService(&AssetService{db: db}, presets, proxy, conversions)
	if err := svc.RefreshAsset(ctx, "asset-1", "asset-1"); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	rc, err := conversions.Open(ctx, "asset-1/thumb.webp")
	if err != nil {
		t.Fatalf("open refreshed conversion: %v", err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read refreshed conversion: %v", err)
	}
	if string(data) != "new-bytes" {
		t.Fatalf("refreshed conversion = %q, want new-bytes", data)
	}
}
