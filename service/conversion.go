package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/storage"
)

const (
	// MaxConversionDimension caps a single rendered edge. Presets will tighten
	// this further once they exist.
	MaxConversionDimension = 4096
	// ConversionCacheControl is deliberately conservative: without cache
	// busting yet, a long TTL would keep stale conversions around after an
	// asset is replaced.
	ConversionCacheControl = "public, max-age=86400"
)

var (
	ErrNotImage         = errors.New("asset is not an image")
	ErrConversionFailed = errors.New("image conversion failed")
)

var conversionContentTypes = map[string]string{
	"jpg":  "image/jpeg",
	"png":  "image/png",
	"webp": "image/webp",
	"gif":  "image/gif",
	"avif": "image/avif",
}

// ConversionSpec is the path-encoded transform, e.g. "800x600.webp". A zero
// width or height lets imgproxy preserve the source aspect ratio.
type ConversionSpec struct {
	Width  int
	Height int
	Format string
}

func ParseConversionSpec(raw string) (ConversionSpec, error) {
	var spec ConversionSpec

	sizePart, ext, ok := strings.Cut(raw, ".")
	if !ok {
		return spec, fmt.Errorf("%w: conversion is missing a format", ErrInvalidInput)
	}
	ext = strings.ToLower(ext)
	if ext == "jpeg" {
		ext = "jpg"
	}
	if _, ok := conversionContentTypes[ext]; !ok {
		return spec, fmt.Errorf("%w: unsupported conversion format %q", ErrInvalidInput, ext)
	}

	widthRaw, heightRaw, ok := strings.Cut(sizePart, "x")
	if !ok {
		return spec, fmt.Errorf("%w: conversion size must be WIDTHxHEIGHT", ErrInvalidInput)
	}
	width, err := strconv.Atoi(widthRaw)
	if err != nil {
		return spec, fmt.Errorf("%w: invalid conversion width", ErrInvalidInput)
	}
	height, err := strconv.Atoi(heightRaw)
	if err != nil {
		return spec, fmt.Errorf("%w: invalid conversion height", ErrInvalidInput)
	}
	if width < 0 || height < 0 {
		return spec, fmt.Errorf("%w: conversion dimensions cannot be negative", ErrInvalidInput)
	}
	if width > MaxConversionDimension || height > MaxConversionDimension {
		return spec, fmt.Errorf("%w: conversion dimension exceeds %d", ErrInvalidInput, MaxConversionDimension)
	}

	return ConversionSpec{Width: width, Height: height, Format: ext}, nil
}

func (s ConversionSpec) ContentType() string {
	return conversionContentTypes[s.Format]
}

// Key is both the cache key and the path served by Caddy from the assets
// subdomain.
func (s ConversionSpec) Key(assetID string) string {
	return fmt.Sprintf("%s/%dx%d.%s", assetID, s.Width, s.Height, s.Format)
}

// ConversionResult is a rendered conversion, either read from the conversion
// store or produced by imgproxy on this request.
type ConversionResult struct {
	Reader      io.ReadCloser
	ContentType string
	Key         string
	CacheHit    bool
}

// ConversionService renders image conversions through imgproxy and stores the
// successful output in the configured conversion store.
type ConversionService struct {
	assets *AssetService
	proxy  *ImgProxy
	store  storage.ConversionStore
}

func NewConversionService(assets *AssetService, proxy *ImgProxy, store storage.ConversionStore) *ConversionService {
	return &ConversionService{assets: assets, proxy: proxy, store: store}
}

func (s *ConversionService) Convert(ctx context.Context, assetID string, spec ConversionSpec) (*ConversionResult, error) {
	asset, err := s.assets.GetByID(ctx, assetID)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(asset.MimeType, "image/") {
		return nil, fmt.Errorf("%w: %s", ErrNotImage, asset.MimeType)
	}

	key := spec.Key(assetID)
	if rc, err := s.store.Open(ctx, key); err == nil {
		return &ConversionResult{
			Reader:      rc,
			ContentType: spec.ContentType(),
			Key:         key,
			CacheHit:    true,
		}, nil
	}

	processing := "format:" + spec.Format
	if spec.Width > 0 || spec.Height > 0 {
		processing = fmt.Sprintf("rs:fit:%d:%d/format:%s", spec.Width, spec.Height, spec.Format)
	}

	resp, err := s.proxy.Get(ctx, s.proxy.URL(assetID, processing, spec.Format))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConversionFailed, err)
	}
	defer Drain(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		log.Printf("imgproxy: conversion %s failed with %s: %s", key, resp.Status, strings.TrimSpace(string(body)))
		return nil, fmt.Errorf("%w: upstream status %d", ErrConversionFailed, resp.StatusCode)
	}

	if err := s.store.Save(ctx, key, resp.Body); err != nil {
		return nil, fmt.Errorf("cache conversion: %w", err)
	}
	rc, err := s.store.Open(ctx, key)
	if err != nil {
		return nil, err
	}
	return &ConversionResult{
		Reader:      rc,
		ContentType: spec.ContentType(),
		Key:         key,
	}, nil
}

// List returns every conversion currently stored for an asset, ordered by file
// size (smallest first). Files that do not match the conversion key format are
// ignored.
func (s *ConversionService) List(ctx context.Context, assetID string) ([]domain.Conversion, error) {
	asset, err := s.assets.GetByID(ctx, assetID)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(asset.MimeType, "image/") {
		return nil, fmt.Errorf("%w: %s", ErrNotImage, asset.MimeType)
	}

	entries, err := s.store.List(ctx, assetID)
	if err != nil {
		return nil, err
	}

	conversions := make([]domain.Conversion, 0, len(entries))
	for _, entry := range entries {
		spec, err := ParseConversionSpec(path.Base(entry.Key))
		if err != nil {
			continue
		}
		url := s.store.URL(entry.Key)
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			url = ""
		}
		conversions = append(conversions, domain.Conversion{
			Key:        entry.Key,
			URL:        url,
			Width:      spec.Width,
			Height:     spec.Height,
			Format:     spec.Format,
			ByteSize:   entry.ByteSize,
			ModifiedAt: entry.ModifiedAt,
		})
	}

	sort.Slice(conversions, func(i, j int) bool {
		if conversions[i].ByteSize == conversions[j].ByteSize {
			return conversions[i].Key < conversions[j].Key
		}
		return conversions[i].ByteSize < conversions[j].ByteSize
	})
	return conversions, nil
}
