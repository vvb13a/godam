package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vvb13a/godam/internal/config"
	"github.com/vvb13a/godam/internal/domain"
	"github.com/vvb13a/godam/internal/ports"
	"golang.org/x/image/draw"
)

type ProgressCallback func(percent float64)

type progressReader struct {
	r        io.Reader
	total    int64
	read     int64
	callback ProgressCallback
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	if n > 0 {
		pr.read += int64(n)
		if pr.callback != nil && pr.total > 0 {
			pr.callback(float64(pr.read) / float64(pr.total))
		}
	}
	return n, err
}

var (
	ErrUnsupportedFormat = errors.New("file format not supported")
	ErrFileTooLarge      = errors.New("file exceeds maximum allowed size")
)

type AssetService struct {
	cfg          *config.Config
	repo         ports.AssetRepository
	registry     *ports.StorageRegistry
	localPreview ports.StorageDriver
	extractor    ports.MetadataExtractor
	tagRepo      ports.TagRepository
	previewGen   ports.PreviewGenerator
}

func NewAssetService(
	cfg *config.Config,
	repo ports.AssetRepository,
	registry *ports.StorageRegistry,
	localPreview ports.StorageDriver,
	extractor ports.MetadataExtractor,
	tagRepo ports.TagRepository,
	previewGen ports.PreviewGenerator,
) *AssetService {
	return &AssetService{
		cfg:          cfg,
		repo:         repo,
		registry:     registry,
		localPreview: localPreview,
		extractor:    extractor,
		tagRepo:      tagRepo,
		previewGen:   previewGen,
	}
}

func (s *AssetService) Open(ctx context.Context, id string) (io.ReadCloser, error) {
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	driver, err := s.registry.Get(asset.StorageDriver)
	if err != nil {
		return nil, err
	}
	return driver.Open(ctx, asset.StorageKey)
}

func (s *AssetService) OpenPreview(ctx context.Context, id string) (io.ReadCloser, error) {
	previewKey := fmt.Sprintf("previews/%s.jpg", id)
	return s.localPreview.Open(ctx, previewKey)
}

func (s *AssetService) UploadFromSource(ctx context.Context, source string, onProgress ProgressCallback) (*domain.Asset, error) {
	var reader io.Reader
	var filename string
	var size int64
	var mimeType string

	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		resp, err := http.Get(source)
		if err != nil {
			return nil, fmt.Errorf("download failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %s", resp.Status)
		}

		size = resp.ContentLength
		parsedURL, _ := url.Parse(source)
		filename = filepath.Base(parsedURL.Path)
		if filename == "" || filename == "/" || filename == "." {
			filename = "downloaded_asset"
		}

		pr := &progressReader{r: resp.Body, total: size, callback: onProgress}
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, pr)
		if err != nil {
			return nil, err
		}

		mimeType = resp.Header.Get("Content-Type")
		if mimeType == "" || strings.Contains(mimeType, "octet-stream") {
			mimeType = http.DetectContentType(buf.Bytes())
		}
		reader = buf
		size = int64(buf.Len())
	} else {
		cleanPath := filepath.Clean(source)
		if strings.HasPrefix(cleanPath, "~/") {
			home, _ := os.UserHomeDir()
			cleanPath = filepath.Join(home, cleanPath[2:])
		}

		file, err := os.Open(cleanPath)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil {
			return nil, err
		}
		size = info.Size()
		filename = filepath.Base(cleanPath)

		buf := make([]byte, 512)
		n, _ := file.Read(buf)
		mimeType = http.DetectContentType(buf[:n])
		_, _ = file.Seek(0, 0)

		pr := &progressReader{r: file, total: size, callback: onProgress}
		memBuf := new(bytes.Buffer)
		_, err = io.Copy(memBuf, pr)
		if err != nil {
			return nil, err
		}
		reader = memBuf
	}

	// Uses default driver ("")
	return s.Upload(ctx, "", filename, mimeType, size, reader)
}

func (s *AssetService) Upload(ctx context.Context, driverName, filename, mimeType string, size int64, reader io.Reader) (*domain.Asset, error) {
	cleanMime := strings.Split(mimeType, ";")[0]
	if s.cfg != nil {
		if !s.cfg.IsAllowedMime(cleanMime) {
			return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, cleanMime)
		}
		if s.cfg.MaxUploadSizeBytes > 0 && size > s.cfg.MaxUploadSizeBytes {
			return nil, fmt.Errorf("%w: %d bytes (limit: %d)", ErrFileTooLarge, size, s.cfg.MaxUploadSizeBytes)
		}
	}

	driver, err := s.registry.Get(driverName)
	if err != nil {
		return nil, err
	}
	if driverName == "" {
		driverName = "local"
	}

	id := uuid.New().String()
	ext := filepath.Ext(filename)
	storageKey := fmt.Sprintf("uploads/%s%s", id, ext)
	previewKey := fmt.Sprintf("previews/%s.jpg", id)

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, reader); err != nil {
		return nil, err
	}
	bytesData := buf.Bytes()

	// 1. Save original to target storage driver (Local, SFTP, S3)
	if err := driver.Save(ctx, storageKey, bytes.NewReader(bytesData)); err != nil {
		return nil, fmt.Errorf("failed to save to %s storage: %w", driverName, err)
	}

	// 2. Save preview to local storage for zero-latency TUI/SPA previews
	previewReader, err := s.previewGen.Generate(ctx, cleanMime, bytes.NewReader(bytesData))
	if err == nil && previewReader != nil {
		_ = s.localPreview.Save(ctx, previewKey, previewReader)
	}

	// 3. Extract metadata
	meta, _ := s.extractor.Extract(ctx, cleanMime, bytes.NewReader(bytesData), int64(len(bytesData)))

	asset := &domain.Asset{
		ID:            id,
		Filename:      filename,
		StorageDriver: driverName,
		StorageKey:    storageKey,
		MimeType:      cleanMime,
		ByteSize:      int64(len(bytesData)),
		Metadata:      meta,
		CreatedAt:     time.Now().UTC(),
	}

	if err := s.repo.Save(ctx, asset); err != nil {
		_ = driver.Delete(ctx, storageKey)
		_ = s.localPreview.Delete(ctx, previewKey)
		return nil, err
	}

	return asset, nil
}

func (s *AssetService) Download(ctx context.Context, assetID, destPath string, targetWidth int) error {
	asset, err := s.repo.GetByID(ctx, assetID)
	if err != nil {
		return err
	}

	driver, err := s.registry.Get(asset.StorageDriver)
	if err != nil {
		return err
	}

	rc, err := driver.Open(ctx, asset.StorageKey)
	if err != nil {
		return err
	}
	defer rc.Close()

	if strings.HasPrefix(destPath, "~/") {
		home, _ := os.UserHomeDir()
		destPath = filepath.Join(home, destPath[2:])
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	if targetWidth > 0 && strings.HasPrefix(asset.MimeType, "image/") {
		srcImg, _, err := image.Decode(rc)
		if err != nil {
			return fmt.Errorf("failed to decode image for resize: %w", err)
		}

		origBounds := srcImg.Bounds()
		origW := origBounds.Dx()
		origH := origBounds.Dy()

		targetHeight := (targetWidth * origH) / origW
		dstImg := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))

		draw.BiLinear.Scale(dstImg, dstImg.Bounds(), srcImg, origBounds, draw.Over, nil)

		if strings.Contains(asset.MimeType, "png") {
			return png.Encode(out, dstImg)
		}
		return jpeg.Encode(out, dstImg, &jpeg.Options{Quality: 90})
	}

	_, err = io.Copy(out, rc)
	return err
}

func (s *AssetService) GetByID(ctx context.Context, id string) (*domain.Asset, error) {
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	tags, _ := s.repo.GetTagsForAsset(ctx, id)
	asset.Tags = tags
	return asset, nil
}

func (s *AssetService) List(ctx context.Context, collectionID *string) ([]domain.Asset, error) {
	assets, err := s.repo.List(ctx, collectionID)
	if err != nil {
		return nil, err
	}
	for i := range assets {
		tags, _ := s.repo.GetTagsForAsset(ctx, assets[i].ID)
		assets[i].Tags = tags
	}
	return assets, nil
}

func (s *AssetService) AddTag(ctx context.Context, assetID, tagName string) error {
	tag, err := s.tagRepo.GetOrCreate(ctx, tagName)
	if err != nil {
		return err
	}
	return s.repo.AddTag(ctx, assetID, tag.ID)
}

func (s *AssetService) RemoveTag(ctx context.Context, assetID, tagID string) error {
	return s.repo.RemoveTag(ctx, assetID, tagID)
}

func (s *AssetService) AddToCollection(ctx context.Context, assetID, collectionID string) error {
	return s.repo.AddToCollection(ctx, assetID, collectionID)
}

func (s *AssetService) RemoveFromCollection(ctx context.Context, assetID, collectionID string) error {
	return s.repo.RemoveFromCollection(ctx, assetID, collectionID)
}

func (s *AssetService) Delete(ctx context.Context, id string) error {
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	driver, err := s.registry.Get(asset.StorageDriver)
	if err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	_ = driver.Delete(ctx, asset.StorageKey)
	previewKey := fmt.Sprintf("previews/%s.jpg", id)
	_ = s.localPreview.Delete(ctx, previewKey)

	return nil
}
