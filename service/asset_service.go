package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/storage"
	"github.com/vvb13a/godam/store"
	"golang.org/x/image/draw"
	"gorm.io/gorm"
)

var (
	ErrInvalidInput      = errors.New("invalid input")
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
	uploads    *uploadManager
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
	stagingDir := ""
	if cfg != nil && cfg.StoragePath != "" {
		stagingDir = filepath.Join(cfg.StoragePath, ".uploads")
	}
	return &AssetService{
		cfg:        cfg,
		db:         db,
		registry:   registry,
		local:      local,
		extractor:  extractor,
		previewGen: previewGen,
		tags:       tags,
		backup:     backup,
		uploads:    newUploadManager(stagingDir),
	}
}

func (s *AssetService) GetByID(ctx context.Context, id string) (*domain.Asset, error) {
	var m store.Asset
	err := s.db.WithContext(ctx).
		Preload("Tags").
		Preload("Collections").
		Preload("Colors", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		First(&m, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return m.ToDomain(), nil
}

const (
	defaultAssetPageSize = 100
	maxAssetPageSize     = 200
	maxAssetTotal        = 1000
	minColorFilterPct    = 10.0
)

type assetSortSpec struct {
	key       string
	column    string
	expr      string
	direction string
	kind      string
}

type assetCursor struct {
	Sort  string `json:"sort"`
	Value string `json:"value,omitempty"`
	Size  int64  `json:"size,omitempty"`
	ID    string `json:"id"`
}

func assetSort(sort string) assetSortSpec {
	switch sort {
	case "date-asc":
		return assetSortSpec{"date-asc", "created_at", "created_at", "ASC", "time"}
	case "modified-desc":
		return assetSortSpec{"modified-desc", "updated_at", "updated_at", "DESC", "time"}
	case "modified-asc":
		return assetSortSpec{"modified-asc", "updated_at", "updated_at", "ASC", "time"}
	case "name-asc":
		return assetSortSpec{"name-asc", "filename", "filename COLLATE NOCASE", "ASC", "name"}
	case "name-desc":
		return assetSortSpec{"name-desc", "filename", "filename COLLATE NOCASE", "DESC", "name"}
	case "size-desc":
		return assetSortSpec{"size-desc", "byte_size", "byte_size", "DESC", "size"}
	case "size-asc":
		return assetSortSpec{"size-asc", "byte_size", "byte_size", "ASC", "size"}
	default:
		return assetSortSpec{"date-desc", "created_at", "created_at", "DESC", "time"}
	}
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}

func (s *AssetService) applyAssetFilters(query *gorm.DB, filter domain.AssetFilter) *gorm.DB {
	query = query.Where("assets.role = ?", domain.AssetRoleOriginal)
	if len(filter.CollectionIDs) > 0 || filter.Uncategorized {
		clauses := make([]string, 0, 2)
		args := make([]any, 0, 1)
		if len(filter.CollectionIDs) > 0 {
			clauses = append(clauses,
				"assets.id IN (SELECT asset_id FROM asset_collections WHERE collection_id IN ?)")
			args = append(args, filter.CollectionIDs)
		}
		if filter.Uncategorized {
			clauses = append(clauses,
				"assets.id NOT IN (SELECT asset_id FROM asset_collections)")
		}
		query = query.Where("("+strings.Join(clauses, " OR ")+")", args...)
	}

	if len(filter.TagIDs) > 0 || filter.Untagged {
		clauses := make([]string, 0, 2)
		args := make([]any, 0, 1)
		if len(filter.TagIDs) > 0 {
			clauses = append(clauses,
				"assets.id IN (SELECT asset_id FROM asset_tags WHERE tag_id IN ?)")
			args = append(args, filter.TagIDs)
		}
		if filter.Untagged {
			clauses = append(clauses,
				"assets.id NOT IN (SELECT asset_id FROM asset_tags)")
		}
		query = query.Where("("+strings.Join(clauses, " OR ")+")", args...)
	}

	if filter.PresetID != "" {
		allowed := `(
			EXISTS (SELECT 1 FROM presets p WHERE p.id = ? AND p.library_wide = 1)
			OR assets.id IN (
				SELECT ac.asset_id FROM asset_collections ac
				JOIN preset_collections pc ON pc.collection_id = ac.collection_id
				WHERE pc.preset_id = ?
			)
			OR assets.id IN (
				SELECT at.asset_id FROM asset_tags at
				JOIN preset_tags pt ON pt.tag_id = at.tag_id
				WHERE pt.preset_id = ?
			)
		)`
		if filter.PresetExclude {
			query = query.Where("NOT "+allowed, filter.PresetID, filter.PresetID, filter.PresetID)
		} else {
			query = query.Where(allowed, filter.PresetID, filter.PresetID, filter.PresetID)
		}
	}

	if len(filter.Colors) > 0 {
		colors := make([]string, 0, len(filter.Colors))
		seen := make(map[string]bool, len(filter.Colors))
		for _, color := range filter.Colors {
			if color == "teal" {
				color = "green"
			}
			if color != "" && !seen[color] {
				seen[color] = true
				colors = append(colors, color)
			}
		}
		if len(colors) > 0 {
			query = query.Where(
				`assets.id IN (
					SELECT asset_id FROM asset_colors
					WHERE bucket IN ?
					GROUP BY asset_id, bucket
					HAVING SUM(percentage) >= ?
				)`,
				colors, minColorFilterPct,
			)
		}
	}

	if len(filter.MimeTypes) > 0 {
		query = query.Where("assets.mime_type IN ?", filter.MimeTypes)
	}
	if len(filter.AllowedMimeTypes) > 0 {
		clauses := make([]string, 0, len(filter.AllowedMimeTypes))
		args := make([]any, 0, len(filter.AllowedMimeTypes))
		for _, pattern := range filter.AllowedMimeTypes {
			pattern = strings.ToLower(strings.TrimSpace(pattern))
			if pattern == "" {
				continue
			}
			if strings.HasSuffix(pattern, "/*") {
				clauses = append(clauses, "assets.mime_type LIKE ?")
				args = append(args, strings.TrimSuffix(pattern, "*")+"%")
				continue
			}
			clauses = append(clauses, "assets.mime_type = ?")
			args = append(args, pattern)
		}
		if len(clauses) > 0 {
			query = query.Where("("+strings.Join(clauses, " OR ")+")", args...)
		}
	}
	if filter.MinDuration != nil {
		query = query.Where("assets.duration_sec >= ?", *filter.MinDuration)
	}
	if filter.MaxDuration != nil {
		query = query.Where("assets.duration_sec <= ?", *filter.MaxDuration)
	}
	if filter.MinPages != nil {
		query = query.Where("assets.page_count >= ?", *filter.MinPages)
	}
	if filter.MaxPages != nil {
		query = query.Where("assets.page_count <= ?", *filter.MaxPages)
	}
	if filter.MinWidth != nil {
		query = query.Where("assets.width >= ?", *filter.MinWidth)
	}
	if filter.MaxWidth != nil {
		query = query.Where("assets.width <= ?", *filter.MaxWidth)
	}
	if filter.MinHeight != nil {
		query = query.Where("assets.height >= ?", *filter.MinHeight)
	}
	if filter.MaxHeight != nil {
		query = query.Where("assets.height <= ?", *filter.MaxHeight)
	}
	if filter.MinSize != nil {
		query = query.Where("assets.byte_size >= ?", *filter.MinSize)
	}
	if filter.MaxSize != nil {
		query = query.Where("assets.byte_size <= ?", *filter.MaxSize)
	}
	if filter.AddedAfter != nil {
		query = query.Where("assets.created_at >= ?", *filter.AddedAfter)
	}
	if filter.AddedBefore != nil {
		query = query.Where("assets.created_at < ?", *filter.AddedBefore)
	}
	if filter.ModifiedAfter != nil {
		query = query.Where("assets.updated_at >= ?", *filter.ModifiedAfter)
	}
	if filter.ModifiedBefore != nil {
		query = query.Where("assets.updated_at < ?", *filter.ModifiedBefore)
	}

	if term := strings.TrimSpace(filter.Search); term != "" {
		if utf8.RuneCountInString(term) >= 3 {
			match := `"` + strings.ReplaceAll(term, `"`, `""`) + `"`
			query = query.Where(
				"assets.rowid IN (SELECT rowid FROM asset_search WHERE asset_search MATCH ?)",
				match,
			)
		} else {
			like := "%" + escapeLike(strings.ToLower(term)) + "%"
			query = query.Where(
				`(LOWER(assets.filename) LIKE ? ESCAPE '\' OR LOWER(assets.title) LIKE ? ESCAPE '\' OR LOWER(assets.note) LIKE ? ESCAPE '\')`,
				like, like, like,
			)
		}
	}

	if len(filter.Types) > 0 {
		clauses := make([]string, 0, len(filter.Types))
		args := make([]any, 0, 2)
		for _, kind := range filter.Types {
			switch kind {
			case "image":
				clauses = append(clauses, "assets.mime_type LIKE 'image/%'")
			case "video":
				clauses = append(clauses, "assets.mime_type LIKE 'video/%'")
			case "audio":
				clauses = append(clauses,
					"(assets.mime_type LIKE 'audio/%' OR assets.mime_type = 'application/ogg')")
			case "document":
				clauses = append(clauses,
					"(assets.mime_type LIKE 'text/%' OR assets.mime_type IN ?)")
				args = append(args, documentMimes)
			case "other":
				clauses = append(clauses, `(assets.mime_type NOT LIKE 'image/%'
				 AND assets.mime_type NOT LIKE 'video/%'
				 AND assets.mime_type NOT LIKE 'audio/%'
				 AND assets.mime_type NOT LIKE 'text/%'
				 AND assets.mime_type <> 'application/ogg'
				 AND assets.mime_type NOT IN ?)`)
				args = append(args, documentMimes)
			}
		}
		if len(clauses) > 0 {
			query = query.Where("("+strings.Join(clauses, " OR ")+")", args...)
		}
	}

	return query
}

func encodeAssetCursor(cursor assetCursor) string {
	encoded, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func decodeAssetCursor(value string) (assetCursor, error) {
	var cursor assetCursor
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return cursor, fmt.Errorf("%w: invalid cursor", ErrInvalidInput)
	}
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return cursor, fmt.Errorf("%w: invalid cursor", ErrInvalidInput)
	}
	return cursor, nil
}

type assetListRow struct {
	store.Asset
	CreatedAtRaw string `gorm:"column:created_at_raw"`
	UpdatedAtRaw string `gorm:"column:updated_at_raw"`
}

func cursorForAsset(row assetListRow, spec assetSortSpec) assetCursor {
	cursor := assetCursor{Sort: spec.key, ID: row.ID}
	switch spec.kind {
	case "time":
		if spec.column == "updated_at" {
			cursor.Value = row.UpdatedAtRaw
		} else {
			cursor.Value = row.CreatedAtRaw
		}
	case "name":
		cursor.Value = row.Filename
	case "size":
		cursor.Size = row.ByteSize
	}
	return cursor
}

func applyAssetCursor(query *gorm.DB, spec assetSortSpec, value string) (*gorm.DB, error) {
	if value == "" {
		return query, nil
	}
	cursor, err := decodeAssetCursor(value)
	if err != nil {
		return nil, err
	}
	if cursor.Sort != spec.key || cursor.ID == "" {
		return nil, fmt.Errorf("%w: cursor does not match sort", ErrInvalidInput)
	}
	op := ">"
	if spec.direction == "DESC" {
		op = "<"
	}
	switch spec.kind {
	case "time":
		query = query.Where(
			fmt.Sprintf("(%s %s ? OR (%s = ? AND id %s ?))", spec.column, op, spec.column, op),
			cursor.Value, cursor.Value, cursor.ID,
		)
	case "name":
		query = query.Where(
			fmt.Sprintf("(filename COLLATE NOCASE %s ? OR (filename COLLATE NOCASE = ? AND id %s ?))", op, op),
			cursor.Value, cursor.Value, cursor.ID,
		)
	case "size":
		query = query.Where(
			fmt.Sprintf("(byte_size %s ? OR (byte_size = ? AND id %s ?))", op, op),
			cursor.Size, cursor.Size, cursor.ID,
		)
	}
	return query, nil
}

func (s *AssetService) List(ctx context.Context, filter domain.AssetFilter) (*domain.AssetPage, error) {
	spec := assetSort(filter.Sort)

	countBase := s.applyAssetFilters(s.db.WithContext(ctx).Model(&store.Asset{}), filter)
	var total int64
	countSub := countBase.Session(&gorm.Session{}).Select("1").Limit(maxAssetTotal + 1)
	if err := s.db.WithContext(ctx).Table("(?) as filtered_assets", countSub).Count(&total).Error; err != nil {
		return nil, err
	}
	capped := total > maxAssetTotal
	if capped {
		total = maxAssetTotal
	}
	var sum struct {
		TotalBytes int64
	}
	if err := countBase.Session(&gorm.Session{}).
		Select("COALESCE(SUM(assets.byte_size), 0) as total_bytes").
		Scan(&sum).Error; err != nil {
		return nil, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultAssetPageSize
	}
	if limit > maxAssetPageSize {
		limit = maxAssetPageSize
	}

	query := s.applyAssetFilters(s.db.WithContext(ctx).Model(&store.Asset{}), filter)
	query, err := applyAssetCursor(query, spec, filter.Cursor)
	if err != nil {
		return nil, err
	}

	var models []assetListRow
	if err := query.
		Select(
			`assets.id, assets.uploaded_by_name,
			 assets.filename, assets.title, assets.note,
			 assets.mime_type, assets.byte_size, assets.width, assets.height,
			 assets.duration_sec, assets.page_count, assets.created_at, assets.updated_at,
			 CAST(assets.created_at AS TEXT) AS created_at_raw,
			 CAST(assets.updated_at AS TEXT) AS updated_at_raw`,
		).
		Order(fmt.Sprintf("%s %s, id %s", spec.expr, spec.direction, spec.direction)).
		Limit(limit + 1).
		Find(&models).Error; err != nil {
		return nil, err
	}

	next := ""
	if len(models) > limit {
		next = encodeAssetCursor(cursorForAsset(models[limit-1], spec))
		models = models[:limit]
	}

	items := make([]domain.Asset, 0, len(models))
	for i := range models {
		items = append(items, *models[i].ToDomain())
	}
	return &domain.AssetPage{
		Items:       items,
		NextCursor:  next,
		Total:       total,
		TotalBytes:  sum.TotalBytes,
		TotalCapped: capped,
	}, nil
}

func (s *AssetService) colorsForData(mimeType string, data []byte) []domain.AssetColor {
	return s.colorsForReader(mimeType, bytes.NewReader(data))
}

func (s *AssetService) colorsForReader(mimeType string, reader io.Reader) []domain.AssetColor {
	if !strings.HasPrefix(mimeType, "image/") {
		return nil
	}
	colors, _ := extractImageColorsReader(reader)
	return colors
}

func (s *AssetService) saveColors(ctx context.Context, id string, colors []domain.AssetColor) {
	_ = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("asset_id = ?", id).Delete(&store.AssetColor{}).Error; err != nil {
			return err
		}
		if len(colors) == 0 {
			return nil
		}
		rows := make([]store.AssetColor, 0, len(colors))
		for i, color := range colors {
			rows = append(rows, store.AssetColor{
				AssetID:    id,
				Position:   i,
				Color:      color.Hex,
				Bucket:     color.Bucket,
				Percentage: color.Percentage,
			})
		}
		return tx.Create(&rows).Error
	})
}

func (s *AssetService) BackfillColors(ctx context.Context) {
	s.recomputeColorBuckets(ctx)
	var assets []store.Asset
	if err := s.db.WithContext(ctx).
		Select("id, storage_driver, storage_key, mime_type").
		Where("mime_type LIKE 'image/%'").
		Where("id NOT IN (SELECT asset_id FROM asset_colors)").
		Find(&assets).Error; err != nil {
		return
	}
	for i := range assets {
		if ctx.Err() != nil {
			return
		}
		driver, err := s.registry.Get(assets[i].StorageDriver)
		if err != nil {
			continue
		}
		rc, err := driver.Open(ctx, assets[i].StorageKey)
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			continue
		}
		colors, err := extractImageColors(data)
		if err != nil {
			continue
		}
		s.saveColors(ctx, assets[i].ID, colors)
	}
}

func (s *AssetService) recomputeColorBuckets(ctx context.Context) {
	var rows []store.AssetColor
	if err := s.db.WithContext(ctx).
		Select("asset_id", "position", "color", "bucket").
		Find(&rows).Error; err != nil {
		return
	}
	for _, row := range rows {
		bucket, ok := bucketForHex(row.Color)
		if !ok || bucket == row.Bucket {
			continue
		}
		_ = s.db.WithContext(ctx).
			Model(&store.AssetColor{}).
			Where("asset_id = ? AND position = ?", row.AssetID, row.Position).
			Update("bucket", bucket).Error
	}
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

// deleteLegacyPreviews removes preview files written by the old stored-preview
// pipeline. Derived previews are now assets and are deleted with their parent.
func (s *AssetService) deleteLegacyPreviews(ctx context.Context, id string) {
	_ = s.local.Delete(ctx, fmt.Sprintf("previews/%s.jpg", id))
	_ = s.local.Delete(ctx, fmt.Sprintf("previews/%s.png", id))
}

// StartUpload creates a resumable upload session. Large files are sent in
// chunks so the server never needs to hold the whole asset in memory.
func (s *AssetService) StartUpload(ctx context.Context, actorID, driverName, filename, mimeType string, size int64) (*UploadSession, error) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return nil, fmt.Errorf("%w: filename is required", ErrInvalidInput)
	}
	if size < 0 {
		return nil, fmt.Errorf("%w: upload size cannot be negative", ErrInvalidInput)
	}
	if _, err := s.registry.Get(driverName); err != nil {
		return nil, err
	}
	maxBytes := s.uploadPolicy(ctx).streamLimit()
	if maxBytes > 0 && size > maxBytes {
		return nil, fmt.Errorf("%w: %d bytes (limit: %d)", ErrFileTooLarge, size, maxBytes)
	}
	return s.uploads.create(actorID, driverName, filename, mimeType, size)
}

// WriteUploadChunk appends one sequential chunk to an upload session.
func (s *AssetService) WriteUploadChunk(ctx context.Context, id, actorID string, offset int64, reader io.Reader) (int64, error) {
	return s.uploads.write(id, actorID, offset, reader)
}

// CompleteUpload validates the assembled file against the upload policy and
// stores it as a new asset.
func (s *AssetService) CompleteUpload(ctx context.Context, id, actorID string) (*domain.Asset, error) {
	session, err := s.uploads.detach(id, actorID)
	if err != nil {
		return nil, err
	}
	defer os.Remove(session.Path)

	if session.Received != session.Size {
		return nil, fmt.Errorf("%w: upload incomplete (%d of %d bytes)", ErrInvalidInput, session.Received, session.Size)
	}
	return s.storeUploadFile(ctx, session.Driver, session.Filename, session.MimeType, session.Received, session.Path)
}

// CancelUpload discards an in-progress upload and removes its staging file.
func (s *AssetService) CancelUpload(id, actorID string) error {
	return s.uploads.remove(id, actorID)
}

func (s *AssetService) Upload(ctx context.Context, driverName, filename, mimeType string, size int64, reader io.Reader) (*domain.Asset, error) {
	path, written, err := s.spoolUpload(ctx, reader)
	if err != nil {
		return nil, err
	}
	defer os.Remove(path)
	return s.storeUploadFile(ctx, driverName, filename, mimeType, written, path)
}

// spoolUpload streams a request body to a staging file, enforcing the library
// upload limit as it copies so a large request cannot exhaust memory or disk.
func (s *AssetService) spoolUpload(ctx context.Context, reader io.Reader) (string, int64, error) {
	if err := os.MkdirAll(s.uploads.dir, 0o700); err != nil {
		return "", 0, fmt.Errorf("create upload staging directory: %w", err)
	}
	file, err := os.CreateTemp(s.uploads.dir, "incoming-*.part")
	if err != nil {
		return "", 0, err
	}
	path := file.Name()

	maxBytes := s.uploadPolicy(ctx).streamLimit()
	var written int64
	if maxBytes > 0 {
		written, err = io.Copy(file, io.LimitReader(reader, maxBytes+1))
	} else {
		written, err = io.Copy(file, reader)
	}
	closeErr := file.Close()
	if err != nil {
		_ = os.Remove(path)
		return "", 0, err
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return "", 0, closeErr
	}
	if maxBytes > 0 && written > maxBytes {
		_ = os.Remove(path)
		return "", 0, fmt.Errorf("%w: %d bytes (limit: %d)", ErrFileTooLarge, written, maxBytes)
	}
	return path, written, nil
}

const uploadSniffBytes int64 = 1 << 20

// storeUploadFile finalises a staged file: it sniffs the content, stores the
// original through the selected driver, creates a derived visual asset for
// non-image types, and records the asset plus its metadata and colors.
func (s *AssetService) storeUploadFile(ctx context.Context, driverName, filename, mimeType string, size int64, path string) (*domain.Asset, error) {
	driver, err := s.registry.Get(driverName)
	if err != nil {
		return nil, err
	}
	if driverName == "" {
		driverName = "local"
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	cleanMime, meta, colors, err := s.inspectUpload(ctx, file, mimeType, size)
	if err != nil {
		return nil, err
	}

	id := uuid.New().String()
	ext := filepath.Ext(filename)
	storageKey := fmt.Sprintf("uploads/%s%s", id, ext)

	// 1. Save original to target storage driver (Local, SFTP, S3)
	if err := driver.Save(ctx, storageKey, file); err != nil {
		return nil, fmt.Errorf("failed to save to %s storage: %w", driverName, err)
	}

	uploaderName := ""
	if actor := domain.ActorFromContext(ctx); actor != nil {
		uploaderName = actor.Name
	}

	now := time.Now().UTC()
	asset := &domain.Asset{
		ID:             id,
		UploadedByName: uploaderName,
		Filename:       filename,
		StorageDriver:  driverName,
		StorageKey:     storageKey,
		MimeType:       cleanMime,
		ByteSize:       size,
		Role:           domain.AssetRoleOriginal,
		Metadata:       meta,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.db.WithContext(ctx).Create(store.AssetModel(asset)).Error; err != nil {
		_ = driver.Delete(ctx, storageKey)
		return nil, err
	}
	s.saveColors(ctx, id, colors)

	// 2. Store a derived visual asset for non-image types. Images are their
	// own visual source and are rendered on demand by imgproxy.
	if _, err := file.Seek(0, io.SeekStart); err == nil {
		_ = s.refreshVisualAsset(ctx, asset, file)
	}

	if s.backup != nil {
		_ = s.backup.EnqueueAsset(ctx, id, driverName, storageKey, size)
	}

	return s.GetByID(ctx, id)
}

// inspectUpload detects the real type, validates it against the upload policy,
// and extracts metadata and colors without reading the whole file into memory.
func (s *AssetService) inspectUpload(ctx context.Context, file *os.File, providedMime string, size int64) (string, domain.Metadata, []domain.AssetColor, error) {
	head, err := readHead(file, uploadSniffBytes)
	if err != nil {
		return "", domain.Metadata{}, nil, err
	}
	cleanMime := classifyContainerMime(sniffMime(providedMime, head), head)

	policy := s.uploadPolicy(ctx)
	if !mimeAllowed(cleanMime, policy.allowed) {
		return "", domain.Metadata{}, nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, cleanMime)
	}
	maxBytes := policy.limitFor(cleanMime)
	if maxBytes > 0 && size > maxBytes {
		return "", domain.Metadata{}, nil, fmt.Errorf("%w: %d bytes (limit: %d)", ErrFileTooLarge, size, maxBytes)
	}

	meta, _ := s.extractor.Extract(ctx, cleanMime, file, size)
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", domain.Metadata{}, nil, err
	}
	colors := s.colorsForReader(cleanMime, file)
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", domain.Metadata{}, nil, err
	}
	return cleanMime, meta, colors, nil
}

func readHead(file *os.File, limit int64) ([]byte, error) {
	buf := make([]byte, limit)
	n, err := io.ReadFull(file, buf)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
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
// example after an in-browser edit), regenerating the derived visual asset and
// metadata and refreshing the derived columns. The storage key is kept, so the
// served content type comes from the stored mime type.
func (s *AssetService) ReplaceContent(ctx context.Context, id, mimeType string, reader io.Reader) (*domain.Asset, error) {
	asset, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	driver, err := s.registry.Get(asset.StorageDriver)
	if err != nil {
		return nil, err
	}

	path, size, err := s.spoolUpload(ctx, reader)
	if err != nil {
		return nil, err
	}
	defer os.Remove(path)

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	cleanMime, meta, colors, err := s.inspectUpload(ctx, file, mimeType, size)
	if err != nil {
		return nil, err
	}

	if err := driver.Save(ctx, asset.StorageKey, file); err != nil {
		return nil, fmt.Errorf("failed to save to %s storage: %w", asset.StorageDriver, err)
	}

	if err := s.db.WithContext(ctx).
		Model(&store.Asset{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"mime_type":    cleanMime,
			"byte_size":    size,
			"width":        meta.Width,
			"height":       meta.Height,
			"duration_sec": meta.DurationSec,
			"page_count":   meta.PageCount,
			"raw_metadata": meta.RawJSON,
			"updated_at":   time.Now().UTC(),
		}).Error; err != nil {
		return nil, err
	}
	s.saveColors(ctx, id, colors)
	updated, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := file.Seek(0, io.SeekStart); err == nil {
		_ = s.refreshVisualAsset(ctx, updated, file)
	}
	if s.backup != nil {
		_ = s.backup.EnqueueAsset(ctx, id, asset.StorageDriver, asset.StorageKey, size)
	}
	return updated, nil
}

// RegeneratePreview re-renders the derived visual asset for an asset, e.g.
// after preview settings change. The original file is untouched; updated_at is
// bumped so clients re-fetch the library preview.
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

	// Re-extract metadata too, so existing assets pick up new fields (e.g.
	// audio/video duration and video dimensions).
	meta, _ := s.extractor.Extract(ctx, mimeType, bytes.NewReader(data), int64(len(data)))
	colors := s.colorsForData(mimeType, data)
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
	s.saveColors(ctx, id, colors)
	updated, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.refreshVisualAsset(ctx, updated, bytes.NewReader(data))
	return updated, nil
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
	if err := s.db.WithContext(ctx).Select("id").First(&asset, "id = ?", assetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNotFound
		}
		return err
	}
	tag, err := s.tags.GetOrCreate(ctx, tagName)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).
		Exec("INSERT OR IGNORE INTO asset_tags (asset_id, tag_id) VALUES (?, ?)", assetID, tag.ID).Error
}

// AddTagByID attaches an existing tag to an asset, so callers that already know
// the tag id are not affected by name normalization.
func (s *AssetService) AddTagByID(ctx context.Context, assetID, tagID string) error {
	var asset store.Asset
	if err := s.db.WithContext(ctx).Select("id").First(&asset, "id = ?", assetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNotFound
		}
		return err
	}
	var tag store.Tag
	if err := s.db.WithContext(ctx).Select("id").First(&tag, "id = ?", tagID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNotFound
		}
		return err
	}
	return s.db.WithContext(ctx).
		Exec("INSERT OR IGNORE INTO asset_tags (asset_id, tag_id) VALUES (?, ?)", assetID, tagID).Error
}

func (s *AssetService) RemoveTag(ctx context.Context, assetID, tagID string) error {
	return s.db.WithContext(ctx).
		Exec("DELETE FROM asset_tags WHERE asset_id = ? AND tag_id = ?", assetID, tagID).Error
}

// AddToCollection adds an asset to a collection.
func (s *AssetService) AddToCollection(ctx context.Context, assetID, collectionID string) error {
	var asset store.Asset
	if err := s.db.WithContext(ctx).Select("id").First(&asset, "id = ?", assetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNotFound
		}
		return err
	}
	var collection store.Collection
	if err := s.db.WithContext(ctx).Select("id").First(&collection, "id = ?", collectionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNotFound
		}
		return err
	}
	return s.db.WithContext(ctx).
		Exec("INSERT OR IGNORE INTO asset_collections (asset_id, collection_id) VALUES (?, ?)", assetID, collectionID).Error
}

func (s *AssetService) RemoveFromCollection(ctx context.Context, assetID, collectionID string) error {
	return s.db.WithContext(ctx).
		Exec("DELETE FROM asset_collections WHERE asset_id = ? AND collection_id = ?", assetID, collectionID).Error
}

// AddToCollectionBulk adds many assets to one collection in a single statement.
func (s *AssetService) AddToCollectionBulk(ctx context.Context, collectionID string, assetIDs []string) error {
	return s.ApplyCollectionsBulk(ctx, assetIDs, []string{collectionID}, "add")
}

// ApplyCollectionsBulk adds or replaces collection membership for many assets.
// "replace" makes collectionIDs the exact collection set of every given asset.
func (s *AssetService) ApplyCollectionsBulk(ctx context.Context, assetIDs, collectionIDs []string, mode string) error {
	if len(assetIDs) == 0 {
		return nil
	}
	if mode == "replace" {
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(
				`DELETE FROM asset_collections WHERE asset_id IN ?`,
				assetIDs,
			).Error; err != nil {
				return err
			}
			if len(collectionIDs) == 0 {
				return nil
			}
			return tx.Exec(
				`INSERT OR IGNORE INTO asset_collections (asset_id, collection_id)
				 SELECT a.id, c.id FROM assets a, collections c
				 WHERE a.id IN ? AND c.id IN ?`,
				assetIDs, collectionIDs,
			).Error
		})
	}
	if len(collectionIDs) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Exec(
		`INSERT OR IGNORE INTO asset_collections (asset_id, collection_id)
		 SELECT a.id, c.id FROM assets a, collections c
		 WHERE a.id IN ? AND c.id IN ?`,
		assetIDs, collectionIDs,
	).Error
}

// AddTagsBulk attaches many tags to many assets in one statement, scoped to the
// library so assets and tags from other libraries are never linked. Existing
// links are ignored, so the operation is idempotent.
func (s *AssetService) AddTagsBulk(ctx context.Context, assetIDs, tagIDs []string) error {
	if len(assetIDs) == 0 || len(tagIDs) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Exec(
		`INSERT OR IGNORE INTO asset_tags (asset_id, tag_id)
		 SELECT a.id, t.id FROM assets a, tags t
		 WHERE a.id IN ? AND t.id IN ?`,
		assetIDs, tagIDs,
	).Error
}

// ReplaceTagsBulk makes tagIDs the exact tag set of every given asset (scoped to
// the library), in one transaction. Pass an empty tagIDs to clear tags.
func (s *AssetService) ReplaceTagsBulk(ctx context.Context, assetIDs, tagIDs []string) error {
	if len(assetIDs) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			`DELETE FROM asset_tags WHERE asset_id IN ?`,
			assetIDs,
		).Error; err != nil {
			return err
		}
		if len(tagIDs) == 0 {
			return nil
		}
		return tx.Exec(
			`INSERT OR IGNORE INTO asset_tags (asset_id, tag_id)
			 SELECT a.id, t.id FROM assets a, tags t
			 WHERE a.id IN ? AND t.id IN ?`,
			assetIDs, tagIDs,
		).Error
	})
}

func (s *AssetService) Delete(ctx context.Context, id string) error {
	asset, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if s.backup != nil {
		_ = s.backup.EnqueueDelete(ctx, asset.ID, asset.StorageKey)
	}

	res := s.db.WithContext(ctx).Delete(&store.Asset{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}

	// A deleted asset can no longer back the instance cover or watermark.
	_ = s.db.WithContext(ctx).
		Model(&store.Instance{}).
		Where("cover_asset_id = ?", id).
		Update("cover_asset_id", nil).Error
	_ = s.db.WithContext(ctx).
		Model(&store.Instance{}).
		Where("watermark_asset_id = ?", id).
		Update("watermark_asset_id", nil).Error

	driver, err := s.registry.Get(asset.StorageDriver)
	if err == nil {
		_ = driver.Delete(ctx, asset.StorageKey)
	}
	_ = s.deleteVisualAsset(ctx, id)
	s.deleteLegacyPreviews(ctx, id)

	return nil
}
