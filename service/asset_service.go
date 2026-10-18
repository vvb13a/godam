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
	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/storage"
	"github.com/vvb13a/godam/store"
	"golang.org/x/image/draw"
	"gorm.io/gorm"
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

// AssetService owns the asset lifecycle: upload, metadata extraction, preview
// generation, storage and the collection/tag relations. Persistence goes
// through the gorm-backed store package.
type AssetService struct {
	cfg        *Config
	db         *gorm.DB
	registry   *storage.Registry
	local      storage.Driver
	extractor  *CompositeExtractor
	previewGen *DefaultPreviewGenerator
	tags       *TagService
	backup     *BackupService
}

func NewAssetService(
	cfg *Config,
	db *gorm.DB,
	registry *storage.Registry,
	local storage.Driver,
	extractor *CompositeExtractor,
	previewGen *DefaultPreviewGenerator,
	tags *TagService,
	backup *BackupService,
) *AssetService {
	return &AssetService{
		cfg:        cfg,
		db:         db,
		registry:   registry,
		local:      local,
		extractor:  extractor,
		previewGen: previewGen,
		tags:       tags,
		backup:     backup,
	}
}

func (s *AssetService) GetByID(ctx context.Context, id string) (*domain.Asset, error) {
	var m store.Asset
	err := s.db.WithContext(ctx).Preload("Tags").Preload("Collections").First(&m, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	asset := m.ToDomain()
	if asset.UploadedBy != "" {
		asset.UploadedByName = s.uploaderNames(ctx, []string{asset.UploadedBy})[asset.UploadedBy]
	}
	return asset, nil
}

// uploaderNames resolves a set of user ids to display names in one query. A
// missing or unknown id simply has no entry, so the caller leaves it blank.
func (s *AssetService) uploaderNames(ctx context.Context, ids []string) map[string]string {
	names := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return names
	}
	unique := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return names
	}
	var users []store.User
	if err := s.db.WithContext(ctx).Select("id", "name").Where("id IN ?", unique).Find(&users).Error; err != nil {
		return names
	}
	for i := range users {
		names[users[i].ID] = users[i].Name
	}
	return names
}

func (s *AssetService) List(ctx context.Context, filter domain.AssetFilter) ([]domain.Asset, error) {
	query := s.db.WithContext(ctx).
		Preload("Tags").
		Preload("Collections").
		Where("assets.library_id = ?", filter.LibraryID).
		Order("created_at DESC")
	if filter.CollectionID != nil {
		query = query.
			Joins("JOIN asset_collections ON asset_collections.asset_id = assets.id").
			Where("asset_collections.collection_id = ?", *filter.CollectionID)
	}
	if filter.Uncategorized {
		query = query.Where("assets.id NOT IN (SELECT asset_id FROM asset_collections)")
	}
	if len(filter.TagIDs) > 0 {
		query = query.Where(
			"assets.id IN (SELECT asset_id FROM asset_tags WHERE tag_id IN ?)",
			filter.TagIDs,
		)
	}
	if filter.Untagged {
		query = query.Where("assets.id NOT IN (SELECT asset_id FROM asset_tags)")
	}

	var models []store.Asset
	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}
	assets := make([]domain.Asset, 0, len(models))
	uploaderIDs := make([]string, 0, len(models))
	for i := range models {
		assets = append(assets, *models[i].ToDomain())
		uploaderIDs = append(uploaderIDs, assets[len(assets)-1].UploadedBy)
	}
	names := s.uploaderNames(ctx, uploaderIDs)
	for i := range assets {
		assets[i].UploadedByName = names[assets[i].UploadedBy]
	}
	return assets, nil
}

func (s *AssetService) Open(ctx context.Context, id string) (io.ReadCloser, error) {
	asset, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	driver, err := s.registry.Get(asset.StorageDriver)
	if err != nil {
		return nil, err
	}
	return driver.Open(ctx, asset.StorageKey)
}

// OpenPreview opens an asset's generated preview, returning its content type.
func (s *AssetService) OpenPreview(ctx context.Context, id string) (io.ReadCloser, string, error) {
	if rc, err := s.local.Open(ctx, fmt.Sprintf("previews/%s.png", id)); err == nil {
		return rc, "image/png", nil
	}
	if rc, err := s.local.Open(ctx, fmt.Sprintf("previews/%s.jpg", id)); err == nil {
		return rc, "image/jpeg", nil
	}
	return nil, "", domain.ErrNotFound
}

// savePreview stores a generated preview under the extension for the MIME type,
// removing the other extension so no stale preview lingers.
func (s *AssetService) savePreview(ctx context.Context, id, mimeType string, reader io.Reader) {
	ext := previewExtension(mimeType)
	other := ".jpg"
	if ext == ".jpg" {
		other = ".png"
	}
	_ = s.local.Delete(ctx, fmt.Sprintf("previews/%s%s", id, other))
	_ = s.local.Save(ctx, fmt.Sprintf("previews/%s%s", id, ext), reader)
}

// deletePreviews removes both possible preview files for an asset.
func (s *AssetService) deletePreviews(ctx context.Context, id string) {
	_ = s.local.Delete(ctx, fmt.Sprintf("previews/%s.jpg", id))
	_ = s.local.Delete(ctx, fmt.Sprintf("previews/%s.png", id))
}

// UploadFromSource accepts either a local filesystem path or an http(s) URL,
// streams it through onProgress and stores it with the default driver.
func (s *AssetService) UploadFromSource(ctx context.Context, userID, libraryID, source string, onProgress ProgressCallback) (*domain.Asset, error) {
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
	return s.Upload(ctx, userID, libraryID, "", filename, mimeType, size, reader)
}

func (s *AssetService) Upload(ctx context.Context, userID, libraryID, driverName, filename, mimeType string, size int64, reader io.Reader) (*domain.Asset, error) {
	if err := s.ensureLibrary(ctx, libraryID); err != nil {
		return nil, err
	}

	driver, err := s.registry.Get(driverName)
	if err != nil {
		return nil, err
	}
	if driverName == "" {
		driverName = "local"
	}

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, reader); err != nil {
		return nil, err
	}
	bytesData := buf.Bytes()

	// Sniff the real type, refine audio-only containers, then validate against
	// the library's upload policy.
	cleanMime := classifyContainerMime(sniffMime(mimeType, bytesData), bytesData)
	allowed, maxBytes := s.uploadPolicy(ctx, libraryID)
	if !mimeAllowed(cleanMime, allowed) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, cleanMime)
	}
	if maxBytes > 0 && int64(len(bytesData)) > maxBytes {
		return nil, fmt.Errorf("%w: %d bytes (limit: %d)", ErrFileTooLarge, len(bytesData), maxBytes)
	}

	id := uuid.New().String()
	ext := filepath.Ext(filename)
	storageKey := fmt.Sprintf("uploads/%s%s", id, ext)

	// 1. Save original to target storage driver (Local, SFTP, S3)
	if err := driver.Save(ctx, storageKey, bytes.NewReader(bytesData)); err != nil {
		return nil, fmt.Errorf("failed to save to %s storage: %w", driverName, err)
	}

	// 2. Save preview to local storage for zero-latency previews
	if previewReader, err := s.previewGen.Generate(ctx, cleanMime, bytes.NewReader(bytesData)); err == nil && previewReader != nil {
		s.savePreview(ctx, id, cleanMime, previewReader)
	}

	// 3. Extract metadata
	meta, _ := s.extractor.Extract(ctx, cleanMime, bytes.NewReader(bytesData), int64(len(bytesData)))

	now := time.Now().UTC()
	asset := &domain.Asset{
		ID:            id,
		LibraryID:     libraryID,
		UploadedBy:    userID,
		Filename:      filename,
		StorageDriver: driverName,
		StorageKey:    storageKey,
		MimeType:      cleanMime,
		ByteSize:      int64(len(bytesData)),
		Metadata:      meta,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.db.WithContext(ctx).Create(store.AssetModel(asset)).Error; err != nil {
		_ = driver.Delete(ctx, storageKey)
		s.deletePreviews(ctx, id)
		return nil, err
	}

	if s.backup != nil {
		_ = s.backup.EnqueueAsset(ctx, libraryID, id, driverName, storageKey, int64(len(bytesData)))
	}

	return s.GetByID(ctx, id)
}

// Update changes the editable presentation fields of an asset. An empty title
// means "fall back to the filename" in the UI, so it is stored as-is.
func (s *AssetService) Update(ctx context.Context, id, title, note string) (*domain.Asset, error) {
	res := s.db.WithContext(ctx).
		Model(&store.Asset{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"title":      strings.TrimSpace(title),
			"note":       strings.TrimSpace(note),
			"updated_at": time.Now().UTC(),
		})
	if res.Error != nil {
		return nil, res.Error
	}
	return s.GetByID(ctx, id)
}

// ReplaceContent overwrites the stored file of an asset with new bytes (for
// example after an in-browser edit), regenerating the preview and metadata and
// refreshing the derived columns. The storage key is kept, so the served
// content type comes from the stored mime type.
func (s *AssetService) ReplaceContent(ctx context.Context, id, mimeType string, reader io.Reader) (*domain.Asset, error) {
	asset, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	driver, err := s.registry.Get(asset.StorageDriver)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, reader); err != nil {
		return nil, err
	}
	bytesData := buf.Bytes()

	cleanMime := classifyContainerMime(sniffMime(mimeType, bytesData), bytesData)
	allowed, maxBytes := s.uploadPolicy(ctx, asset.LibraryID)
	if !mimeAllowed(cleanMime, allowed) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, cleanMime)
	}
	if maxBytes > 0 && int64(len(bytesData)) > maxBytes {
		return nil, fmt.Errorf("%w: %d bytes (limit: %d)", ErrFileTooLarge, len(bytesData), maxBytes)
	}

	if err := driver.Save(ctx, asset.StorageKey, bytes.NewReader(bytesData)); err != nil {
		return nil, fmt.Errorf("failed to save to %s storage: %w", asset.StorageDriver, err)
	}

	if previewReader, err := s.previewGen.Generate(ctx, cleanMime, bytes.NewReader(bytesData)); err == nil && previewReader != nil {
		s.savePreview(ctx, id, cleanMime, previewReader)
	}

	meta, _ := s.extractor.Extract(ctx, cleanMime, bytes.NewReader(bytesData), int64(len(bytesData)))
	if err := s.db.WithContext(ctx).
		Model(&store.Asset{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"mime_type":    cleanMime,
			"byte_size":    int64(len(bytesData)),
			"width":        meta.Width,
			"height":       meta.Height,
			"duration_sec": meta.DurationSec,
			"page_count":   meta.PageCount,
			"raw_metadata": meta.RawJSON,
			"updated_at":   time.Now().UTC(),
		}).Error; err != nil {
		return nil, err
	}
	if s.backup != nil {
		_ = s.backup.EnqueueAsset(ctx, asset.LibraryID, id, asset.StorageDriver, asset.StorageKey, int64(len(bytesData)))
	}
	return s.GetByID(ctx, id)
}

// RegeneratePreview re-renders the stored preview for an asset, e.g. after
// preview settings change. The original file is untouched; updated_at is bumped
// so clients re-fetch the (otherwise cached) preview.
func (s *AssetService) RegeneratePreview(ctx context.Context, id string) (*domain.Asset, error) {
	asset, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	driver, err := s.registry.Get(asset.StorageDriver)
	if err != nil {
		return nil, err
	}
	rc, err := driver.Open(ctx, asset.StorageKey)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	// Re-detect the type so audio-only containers are corrected on regenerate
	// too (this is how existing mis-classified assets get fixed).
	mimeType := classifyContainerMime(sniffMime(asset.MimeType, data), data)

	reader, err := s.previewGen.Generate(ctx, mimeType, bytes.NewReader(data))
	if err != nil || reader == nil {
		reader = s.previewGen.GetFallback(mimeType)
	}
	s.savePreview(ctx, id, mimeType, reader)

	// Re-extract metadata too, so existing assets pick up new fields (e.g.
	// audio/video duration and video dimensions).
	meta, _ := s.extractor.Extract(ctx, mimeType, bytes.NewReader(data), int64(len(data)))
	if err := s.db.WithContext(ctx).
		Model(&store.Asset{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"mime_type":    mimeType,
			"width":        meta.Width,
			"height":       meta.Height,
			"duration_sec": meta.DurationSec,
			"page_count":   meta.PageCount,
			"raw_metadata": meta.RawJSON,
			"updated_at":   time.Now().UTC(),
		}).Error; err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

func (s *AssetService) Download(ctx context.Context, assetID, destPath string, targetWidth int) error {
	asset, err := s.GetByID(ctx, assetID)
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

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
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

func (s *AssetService) AddTag(ctx context.Context, assetID, tagName string) error {
	var asset store.Asset
	if err := s.db.WithContext(ctx).Select("library_id").First(&asset, "id = ?", assetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNotFound
		}
		return err
	}
	tag, err := s.tags.GetOrCreate(ctx, asset.LibraryID, tagName)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).
		Exec("INSERT OR IGNORE INTO asset_tags (asset_id, tag_id) VALUES (?, ?)", assetID, tag.ID).Error
}

// AddTagByID attaches an existing tag to an asset, so callers that already know
// the tag id are not affected by name normalization. The tag must belong to the
// same library as the asset.
func (s *AssetService) AddTagByID(ctx context.Context, assetID, tagID string) error {
	var asset store.Asset
	if err := s.db.WithContext(ctx).Select("library_id").First(&asset, "id = ?", assetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNotFound
		}
		return err
	}
	var tag store.Tag
	if err := s.db.WithContext(ctx).Select("library_id").First(&tag, "id = ?", tagID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNotFound
		}
		return err
	}
	if tag.LibraryID != asset.LibraryID {
		return domain.ErrNotFound
	}
	return s.db.WithContext(ctx).
		Exec("INSERT OR IGNORE INTO asset_tags (asset_id, tag_id) VALUES (?, ?)", assetID, tagID).Error
}

func (s *AssetService) RemoveTag(ctx context.Context, assetID, tagID string) error {
	return s.db.WithContext(ctx).
		Exec("DELETE FROM asset_tags WHERE asset_id = ? AND tag_id = ?", assetID, tagID).Error
}

// AddToCollection adds an asset to a collection in the same library.
func (s *AssetService) AddToCollection(ctx context.Context, assetID, collectionID string) error {
	var asset store.Asset
	if err := s.db.WithContext(ctx).Select("library_id").First(&asset, "id = ?", assetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNotFound
		}
		return err
	}
	var collection store.Collection
	if err := s.db.WithContext(ctx).Select("library_id").First(&collection, "id = ?", collectionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNotFound
		}
		return err
	}
	if collection.LibraryID != asset.LibraryID {
		return domain.ErrNotFound
	}
	return s.db.WithContext(ctx).
		Exec("INSERT OR IGNORE INTO asset_collections (asset_id, collection_id) VALUES (?, ?)", assetID, collectionID).Error
}

func (s *AssetService) RemoveFromCollection(ctx context.Context, assetID, collectionID string) error {
	return s.db.WithContext(ctx).
		Exec("DELETE FROM asset_collections WHERE asset_id = ? AND collection_id = ?", assetID, collectionID).Error
}

// AddToCollectionBulk adds many assets to one collection in a single statement.
// Only assets sharing the collection's library are linked, so unknown or
// foreign ids are ignored rather than failing the whole batch.
func (s *AssetService) AddToCollectionBulk(ctx context.Context, collectionID string, assetIDs []string) error {
	if len(assetIDs) == 0 {
		return nil
	}
	var collection store.Collection
	if err := s.db.WithContext(ctx).Select("library_id").First(&collection, "id = ?", collectionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNotFound
		}
		return err
	}
	return s.db.WithContext(ctx).Exec(
		`INSERT OR IGNORE INTO asset_collections (asset_id, collection_id)
		 SELECT a.id, ? FROM assets a WHERE a.id IN ? AND a.library_id = ?`,
		collectionID, assetIDs, collection.LibraryID,
	).Error
}

// AddTagsBulk attaches many tags to many assets in one statement, scoped to the
// library so assets and tags from other libraries are never linked. Existing
// links are ignored, so the operation is idempotent.
func (s *AssetService) AddTagsBulk(ctx context.Context, libraryID string, assetIDs, tagIDs []string) error {
	if len(assetIDs) == 0 || len(tagIDs) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Exec(
		`INSERT OR IGNORE INTO asset_tags (asset_id, tag_id)
		 SELECT a.id, t.id FROM assets a, tags t
		 WHERE a.id IN ? AND t.id IN ? AND a.library_id = ? AND t.library_id = ?`,
		assetIDs, tagIDs, libraryID, libraryID,
	).Error
}

// ReplaceTagsBulk makes tagIDs the exact tag set of every given asset (scoped to
// the library), in one transaction. Pass an empty tagIDs to clear tags.
func (s *AssetService) ReplaceTagsBulk(ctx context.Context, libraryID string, assetIDs, tagIDs []string) error {
	if len(assetIDs) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			`DELETE FROM asset_tags
			 WHERE asset_id IN (SELECT id FROM assets WHERE id IN ? AND library_id = ?)`,
			assetIDs, libraryID,
		).Error; err != nil {
			return err
		}
		if len(tagIDs) == 0 {
			return nil
		}
		return tx.Exec(
			`INSERT OR IGNORE INTO asset_tags (asset_id, tag_id)
			 SELECT a.id, t.id FROM assets a, tags t
			 WHERE a.id IN ? AND t.id IN ? AND a.library_id = ? AND t.library_id = ?`,
			assetIDs, tagIDs, libraryID, libraryID,
		).Error
	})
}

func (s *AssetService) Delete(ctx context.Context, id string) error {
	asset, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if s.backup != nil {
		_ = s.backup.EnqueueDelete(ctx, asset.LibraryID, asset.ID, asset.StorageKey)
	}

	res := s.db.WithContext(ctx).Delete(&store.Asset{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}

	// A deleted asset can no longer back a library cover or watermark.
	_ = s.db.WithContext(ctx).
		Model(&store.Library{}).
		Where("cover_asset_id = ?", id).
		Update("cover_asset_id", nil).Error
	_ = s.db.WithContext(ctx).
		Model(&store.Library{}).
		Where("watermark_asset_id = ?", id).
		Update("watermark_asset_id", nil).Error

	driver, err := s.registry.Get(asset.StorageDriver)
	if err == nil {
		_ = driver.Delete(ctx, asset.StorageKey)
	}
	s.deletePreviews(ctx, id)

	return nil
}

// DeleteByLibrary removes every asset in a library and its stored files. It is
// used when a whole library is deleted.
func (s *AssetService) DeleteByLibrary(ctx context.Context, libraryID string) error {
	var ids []string
	if err := s.db.WithContext(ctx).
		Model(&store.Asset{}).
		Where("library_id = ?", libraryID).
		Pluck("id", &ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.Delete(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// ensureLibrary verifies a library exists before persisting scoped rows, so an
// unknown library yields ErrNotFound (404) instead of a foreign key error.
func (s *AssetService) ensureLibrary(ctx context.Context, libraryID string) error {
	return ensureLibraryExists(ctx, s.db, libraryID)
}
