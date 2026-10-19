package service

import (
	"bytes"
	"context"
	"errors"
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
		NewTagService(db), nil,
	)

	var source bytes.Buffer
	if err := png.Encode(&source, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	asset, err := assets.Upload(ctx, "user", "", "test.png", "image/png", int64(source.Len()), bytes.NewReader(source.Bytes()))
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

	spec, err := ParseConversionSpec("100x50.webp")
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}

	first, err := svc.Convert(ctx, asset.ID, spec)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	data, err := io.ReadAll(first.Reader)
	_ = first.Reader.Close()
	if err != nil || string(data) != "webp-bytes" {
		t.Fatalf("first result = %q err=%v", data, err)
	}
	if first.CacheHit {
		t.Fatal("first conversion should be a miss")
	}

	second, err := svc.Convert(ctx, asset.ID, spec)
	if err != nil {
		t.Fatalf("second convert: %v", err)
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
	if got := conversions.URL(spec.Key(asset.ID)); got != "https://assets.example.com/"+spec.Key(asset.ID) {
		t.Fatalf("url = %q", got)
	}

	if err := conversions.Save(ctx, asset.ID+"/10x10.png", strings.NewReader("x")); err != nil {
		t.Fatalf("save second conversion: %v", err)
	}
	list, err := svc.List(ctx, asset.ID)
	if err != nil {
		t.Fatalf("list conversions: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("conversions = %+v", list)
	}
	if list[0].Width != 10 || list[0].Height != 10 || list[0].Format != "png" || list[0].ByteSize != 1 {
		t.Fatalf("smallest conversion = %+v", list[0])
	}
	if list[0].URL != "https://assets.example.com/"+list[0].Key {
		t.Fatalf("smallest URL = %q", list[0].URL)
	}
	if list[1].Key != spec.Key(asset.ID) {
		t.Fatalf("second conversion = %+v", list[1])
	}

	preset, err := presets.Create(ctx, "user", "Thumb", "thumb", domain.PresetParameters{
		Format: "webp",
		Options: []domain.PresetOption{
			{Name: "resize", Args: []string{"fit", "200", "0"}},
			{Name: "quality", Args: []string{"82"}},
		},
	}, true, nil, nil)
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

	other, err := NewCollectionService(db).Create(ctx, nil, "Other", "", "")
	if err != nil {
		t.Fatalf("create other collection: %v", err)
	}
	if _, err := presets.Create(ctx, "user", "Restricted", "restricted", domain.PresetParameters{
		Format: "webp",
		Options: []domain.PresetOption{
			{Name: "resize", Args: []string{"fit", "100", "0"}},
		},
	}, false, []string{other.ID}, nil); err != nil {
		t.Fatalf("create restricted preset: %v", err)
	}
	if _, err := svc.ConvertPreset(ctx, asset.ID, "restricted.webp"); !errors.Is(err, ErrPresetNotAllowed) {
		t.Fatalf("expected restricted preset to be denied, got %v", err)
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
