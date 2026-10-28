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
	// MaxConversionDimension caps a single rendered edge. Presets are validated
	// against catalog limits as well.
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

// ConversionSpec is the legacy path-encoded size transform, e.g.
// "800x600.webp". It is only used to describe old cache entries; new
// conversions use presets.
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
	assets  *AssetService
	presets *PresetService
	proxy   *ImgProxy
	store   storage.ConversionStore
}

func NewConversionService(assets *AssetService, presets *PresetService, proxy *ImgProxy, store storage.ConversionStore) *ConversionService {
	return &ConversionService{assets: assets, presets: presets, proxy: proxy, store: store}
}

// ConvertPreset resolves a named preset and renders it.
func (s *ConversionService) ConvertPreset(ctx context.Context, assetID, segment string) (*ConversionResult, error) {
	return s.ConvertPresetSource(ctx, assetID, assetID, segment)
}

// ConvertPresetSource renders sourceAssetID. The leading argument is retained
// for callers that resolve a preset through a related asset.
func (s *ConversionService) ConvertPresetSource(ctx context.Context, _, sourceAssetID, segment string) (*ConversionResult, error) {
	name, format, err := splitConversionSegment(segment)
	if err != nil {
		return nil, err
	}
	if !presetNamePattern.MatchString(name) || isSizeName(name) {
		return nil, fmt.Errorf("%w: unknown conversion preset", ErrInvalidInput)
	}
	if s.presets == nil {
		return nil, fmt.Errorf("%w: unknown conversion preset", domain.ErrNotFound)
	}
	preset, err := s.presets.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if format != preset.Parameters.Format {
		return nil, fmt.Errorf("%w: preset %q produces %s, not %s", ErrInvalidInput, name, preset.Parameters.Format, format)
	}
	processing, err := BuildProcessing(preset.Parameters)
	if err != nil {
		return nil, err
	}
	return s.convert(ctx, sourceAssetID, processing, format, name)
}

// ConvertSigned verifies an ad-hoc signed pipeline and renders it.
func (s *ConversionService) ConvertSigned(ctx context.Context, assetID, segment string) (*ConversionResult, error) {
	return s.ConvertSignedSource(ctx, assetID, assetID, segment)
}

// ConvertSignedSource verifies a segment signed for signAssetID but renders
// sourceAssetID. This lets /img/{parent}/{signed} resolve to a derived visual
// asset while keeping the signature tied to the ID the caller knows.
func (s *ConversionService) ConvertSignedSource(ctx context.Context, signAssetID, sourceAssetID, segment string) (*ConversionResult, error) {
	cacheName, format, err := splitConversionSegment(segment)
	if err != nil {
		return nil, err
	}
	_, processing, err := s.proxy.VerifyRequest(signAssetID, segment)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	options, err := ParseProcessing(processing)
	if err != nil {
		return nil, err
	}
	processing, err = BuildProcessing(domain.PresetParameters{Format: format, Options: options})
	if err != nil {
		return nil, err
	}
	return s.convert(ctx, sourceAssetID, processing, format, cacheName)
}

var defaultPreviewParameters = domain.PresetParameters{
	Format: "webp",
	Options: []domain.PresetOption{
		{Name: "resize", Args: []string{"fit", "400", "400", "0", "0"}},
		{Name: "quality", Args: []string{"80"}},
	},
}

// PreviewDefault renders the built-in preview used by the delivery and
// authenticated preview routes. The output is cached like any other conversion.
func (s *ConversionService) PreviewDefault(ctx context.Context, assetID string) (*ConversionResult, error) {
	processing, err := BuildProcessing(defaultPreviewParameters)
	if err != nil {
		return nil, err
	}
	// Underscore-prefixed cache names are not valid preset names, so this
	// library-only preview cannot be addressed through /convert or /img.
	return s.convert(ctx, assetID, processing, "webp", "_library-preview")
}

// Preview renders a preset pipeline for an asset without writing the image to
// the conversion cache. The caller owns the returned stream.
func (s *ConversionService) Preview(ctx context.Context, assetID string, params domain.PresetParameters) (io.ReadCloser, string, error) {
	normalized, err := ValidatePresetParameters(params)
	if err != nil {
		return nil, "", err
	}
	asset, err := s.assets.GetByID(ctx, assetID)
	if err != nil {
		return nil, "", err
	}
	if !strings.HasPrefix(asset.MimeType, "image/") {
		return nil, "", fmt.Errorf("%w: %s", ErrNotImage, asset.MimeType)
	}
	processing, err := BuildProcessing(normalized)
	if err != nil {
		return nil, "", err
	}
	resp, err := s.proxy.Get(ctx, s.proxy.URL(assetID, processing, normalized.Format))
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrConversionFailed, err)
	}
	if resp.StatusCode != http.StatusOK {
		defer Drain(resp.Body)
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		log.Printf("imgproxy: preview %s failed with %s: %s", assetID, resp.Status, strings.TrimSpace(string(body)))
		return nil, "", fmt.Errorf("%w: upstream status %d", ErrConversionFailed, resp.StatusCode)
	}
	return resp.Body, conversionContentTypes[normalized.Format], nil
}

// Purge removes every generated conversion from the conversion store.
func (s *ConversionService) Purge(ctx context.Context) (int, error) {
	return s.store.Purge(ctx)
}

// WarmPreset renders a preset for an asset and stores it in the conversion
// cache. The returned image is not read; the store owns the file.
func (s *ConversionService) WarmPreset(ctx context.Context, assetID string, preset domain.Preset) error {
	processing, err := BuildProcessing(preset.Parameters)
	if err != nil {
		return err
	}
	result, err := s.convert(ctx, assetID, processing, preset.Parameters.Format, preset.Name)
	if err != nil {
		return err
	}
	return result.Reader.Close()
}

// Sign validates an ad-hoc processing pipeline and returns a signed path
// segment for the asset. The segment is accepted by ConvertSigned.
func (s *ConversionService) Sign(ctx context.Context, assetID, format, processing string) (string, error) {
	asset, err := s.assets.GetByID(ctx, assetID)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(asset.MimeType, "image/") {
		return "", fmt.Errorf("%w: %s", ErrNotImage, asset.MimeType)
	}
	options, err := ParseProcessing(processing)
	if err != nil {
		return "", err
	}
	normalized, err := BuildProcessing(domain.PresetParameters{Format: format, Options: options})
	if err != nil {
		return "", err
	}
	return s.proxy.SignRequest(assetID, format, normalized)
}

func (s *ConversionService) convert(ctx context.Context, assetID, processing, format, cacheName string) (*ConversionResult, error) {
	asset, err := s.assets.GetByID(ctx, assetID)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(asset.MimeType, "image/") {
		return nil, fmt.Errorf("%w: %s", ErrNotImage, asset.MimeType)
	}
	if processing == "" {
		return nil, fmt.Errorf("%w: empty processing pipeline", ErrInvalidInput)
	}

	key := fmt.Sprintf("%s/%s.%s", assetID, cacheName, format)
	if rc, err := s.store.Open(ctx, key); err == nil {
		return &ConversionResult{
			Reader:      rc,
			ContentType: conversionContentTypes[format],
			Key:         key,
			CacheHit:    true,
		}, nil
	}

	resp, err := s.proxy.Get(ctx, s.proxy.URL(assetID, processing, format))
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
		ContentType: conversionContentTypes[format],
		Key:         key,
	}, nil
}

// List returns every conversion currently stored for an asset, ordered by file
// size (smallest first). Files that do not match a known key format are
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
		base := path.Base(entry.Key)
		stem, format, err := splitConversionSegment(base)
		if err != nil {
			continue
		}

		conversion := domain.Conversion{
			Key:        entry.Key,
			Kind:       "preset",
			Preset:     stem,
			Format:     format,
			ByteSize:   entry.ByteSize,
			ModifiedAt: entry.ModifiedAt,
		}
		switch {
		case strings.HasPrefix(stem, "~"):
			conversion.Kind = "signed"
			conversion.Preset = ""
			conversion.Label = "Signed request"
		case isSizeName(stem):
			spec, err := ParseConversionSpec(base)
			if err != nil {
				continue
			}
			conversion.Kind = "size"
			conversion.Preset = ""
			conversion.Width = spec.Width
			conversion.Height = spec.Height
		default:
			if !presetNamePattern.MatchString(stem) {
				continue
			}
			if s.presets != nil {
				if preset, err := s.presets.GetByName(ctx, stem); err == nil {
					conversion.Label = preset.Label
				}
			}
		}

		url := s.store.URL(entry.Key)
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			url = ""
		}
		conversion.URL = url
		conversions = append(conversions, conversion)
	}

	sort.Slice(conversions, func(i, j int) bool {
		if conversions[i].ByteSize == conversions[j].ByteSize {
			return conversions[i].Key < conversions[j].Key
		}
		return conversions[i].ByteSize < conversions[j].ByteSize
	})
	return conversions, nil
}

func splitConversionSegment(segment string) (string, string, error) {
	dot := strings.LastIndex(segment, ".")
	if dot <= 0 || dot == len(segment)-1 {
		return "", "", fmt.Errorf("%w: conversion is missing a format", ErrInvalidInput)
	}
	format, err := NormalizeFormat(segment[dot+1:])
	if err != nil {
		return "", "", err
	}
	return segment[:dot], format, nil
}

func isSizeName(name string) bool {
	width, height, ok := strings.Cut(name, "x")
	if !ok {
		return false
	}
	if _, err := strconv.Atoi(width); err != nil {
		return false
	}
	if _, err := strconv.Atoi(height); err != nil {
		return false
	}
	return true
}
