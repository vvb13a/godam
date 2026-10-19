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
	svc := NewConversionService(assets, proxy, conversions)

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
}
