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
	"sort"
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
	ErrForbidden         = errors.New("forbidden")
)

// AssetService owns the asset lifecycle: upload, metadata extraction, preview
// generation, and storage. Persistence goes through the gorm-backed store
// package.
type AssetService struct {
	cfg        *Config
	db         *gorm.DB
	registry   *storage.Registry
	local      storage.Driver
	extractor  *CompositeExtractor
	previewGen *DefaultPreviewGenerator
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
		backup:     backup,
		uploads:    newUploadManager(stagingDir),
	}
}

func (s *AssetService) GetByID(ctx context.Context, id string) (*domain.Asset, error) {
	var m store.Asset
	err := s.db.WithContext(ctx).
		Preload("Colors", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		Preload("Attributes").
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

// documentMimes mirrors the frontend's document media category so listing
// filters count the same assets the view shows.
var documentMimes = []string{
	"application/pdf",
	"application/msword",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"application/vnd.ms-excel",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"application/vnd.ms-powerpoint",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation",
	"application/rtf",
}

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
			// Color metadata only exists for images. Leave other media types
			// untouched so a color filter does not hide unrelated assets.
			query = query.Where(
				`(assets.mime_type NOT LIKE 'image/%' OR assets.id IN (
					SELECT asset_id FROM asset_colors
					WHERE bucket IN ?
					GROUP BY asset_id, bucket
					HAVING SUM(percentage) >= ?
				))`,
				colors, minColorFilterPct,
			)
		}
	}

	if len(filter.MimeTypes) > 0 {
		query = query.Where("assets.mime_type IN ?", filter.MimeTypes)
	}
	for key, values := range filter.Attributes {
		key = strings.TrimSpace(key)
		if key == "" || len(values) == 0 {
			continue
		}
		clauses := make([]string, 0, 2)
		args := make([]any, 0, 3)
		valueArgs := make([]string, 0, len(values))
		unassigned := false
		for _, value := range values {
			if value == domain.AttributeUnassigned {
				unassigned = true
				continue
			}
			valueArgs = append(valueArgs, value)
		}
		if len(valueArgs) > 0 {
			clauses = append(clauses,
				`EXISTS (SELECT 1 FROM asset_attributes aa
				 WHERE aa.asset_id = assets.id AND aa.key = ? AND aa.value IN ?)`)
			args = append(args, key, valueArgs)
		}
		if unassigned {
			clauses = append(clauses,
				`NOT EXISTS (SELECT 1 FROM asset_attributes aa
				 WHERE aa.asset_id = assets.id AND aa.key = ?)`)
			args = append(args, key)
		}
		if len(clauses) > 0 {
			query = query.Where(
				"("+strings.Join(clauses, " OR ")+")", args...,
			)
		}
	}
	// Duration only applies to audio/video. Other media types pass through so
	// the filter narrows the relevant formats instead of hiding everything else.
	if filter.MinDuration != nil || filter.MaxDuration != nil {
		conditions := make([]string, 0, 2)
		args := make([]any, 0, 2)
		if filter.MinDuration != nil {
			conditions = append(conditions, "assets.duration_sec >= ?")
			args = append(args, *filter.MinDuration)
		}
		if filter.MaxDuration != nil {
			conditions = append(conditions, "assets.duration_sec <= ?")
			args = append(args, *filter.MaxDuration)
		}
		query = query.Where(
			`((assets.mime_type NOT LIKE 'audio/%' AND assets.mime_type NOT LIKE 'video/%' AND assets.mime_type <> 'application/ogg') OR (`+
				strings.Join(conditions, " AND ")+`))`,
			args...,
		)
	}

	// Page counts only apply to PDFs.
	if filter.MinPages != nil || filter.MaxPages != nil {
		conditions := make([]string, 0, 2)
		args := make([]any, 0, 2)
		if filter.MinPages != nil {
			conditions = append(conditions, "assets.page_count >= ?")
			args = append(args, *filter.MinPages)
		}
		if filter.MaxPages != nil {
			conditions = append(conditions, "assets.page_count <= ?")
			args = append(args, *filter.MaxPages)
		}
		query = query.Where(
			`((assets.mime_type <> 'application/pdf') OR (`+
				strings.Join(conditions, " AND ")+`))`,
			args...,
		)
	}

	// Width/height only apply to images and videos.
	if filter.MinWidth != nil || filter.MaxWidth != nil ||
		filter.MinHeight != nil || filter.MaxHeight != nil {
		conditions := make([]string, 0, 4)
		args := make([]any, 0, 4)
		if filter.MinWidth != nil {
			conditions = append(conditions, "assets.width >= ?")
			args = append(args, *filter.MinWidth)
		}
		if filter.MaxWidth != nil {
			conditions = append(conditions, "assets.width <= ?")
			args = append(args, *filter.MaxWidth)
		}
		if filter.MinHeight != nil {
			conditions = append(conditions, "assets.height >= ?")
			args = append(args, *filter.MinHeight)
		}
		if filter.MaxHeight != nil {
			conditions = append(conditions, "assets.height <= ?")
			args = append(args, *filter.MaxHeight)
		}
		query = query.Where(
			`((assets.mime_type NOT LIKE 'image/%' AND assets.mime_type NOT LIKE 'video/%') OR (`+
				strings.Join(conditions, " AND ")+`))`,
			args...,
		)
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
				`(assets.rowid IN (SELECT rowid FROM asset_search WHERE asset_search MATCH ?)
				  OR assets.id IN (
				    SELECT asset_id FROM asset_attribute_search
				    WHERE asset_attribute_search MATCH ?
				      AND key IN (
				        SELECT key FROM attributes_metadata
				        WHERE value = '' AND searchable = 1
				      )
				  ))`,
				match, match,
			)
		} else {
			like := "%" + escapeLike(strings.ToLower(term)) + "%"
			query = query.Where(
				`(LOWER(assets.filename) LIKE ? ESCAPE '\' OR LOWER(assets.title) LIKE ? ESCAPE '\' OR LOWER(assets.note) LIKE ? ESCAPE '\'
				  OR EXISTS (
				    SELECT 1 FROM asset_attributes aa
				    JOIN attributes_metadata m ON m.key = aa.key AND m.value = ''
				    WHERE aa.asset_id = assets.id AND m.searchable = 1
				      AND LOWER(aa.value) LIKE ? ESCAPE '\'
				  ))`,
				like, like, like, like,
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

// AttributeFacets returns the attribute keys and values currently in use for
// the library, limited to what the current actor may filter on. Direct access
// sees everything; JWT actors need an explicit active grant or the wildcard.
func (s *AssetService) AttributeFacets(ctx context.Context) ([]domain.AttributeFacet, error) {
	return s.attributeFacets(ctx, false)
}

func (s *AssetService) attributeFacets(
	ctx context.Context,
	includeAllUnassigned bool,
) ([]domain.AttributeFacet, error) {
	actor := domain.ActorFromContext(ctx)
	wildcard := true
	allowed := make(map[string]bool)
	blocked := make(map[string]bool)
	enforced := make(map[string][]string)
	if actor != nil && actor.Token {
		wildcard = actor.Scope.AllAttributes
		for _, grant := range actor.Scope.Attributes {
			switch grant.Mode {
			case domain.AttributeModeActive:
				allowed[grant.Key] = true
			case domain.AttributeModeLocked, domain.AttributeModeHidden:
				blocked[grant.Key] = true
				if len(grant.Values) > 0 {
					enforced[grant.Key] = grant.Values
				}
			}
		}
		if !wildcard && len(allowed) == 0 {
			return []domain.AttributeFacet{}, nil
		}
	}

	type facetRow struct {
		Key   string
		Value string
		Count int64
	}
	query := s.db.WithContext(ctx).
		Table("asset_attributes AS aa").
		Select("aa.key AS key, aa.value AS value, COUNT(DISTINCT aa.asset_id) AS count").
		Joins("JOIN assets ON assets.id = aa.asset_id").
		Where("assets.role = ?", domain.AssetRoleOriginal)
	for key, values := range enforced {
		query = query.Where(
			`EXISTS (SELECT 1 FROM asset_attributes enforced
			 WHERE enforced.asset_id = assets.id AND enforced.key = ? AND enforced.value IN ?)`,
			key, values,
		)
	}
	if wildcard {
		if len(blocked) > 0 {
			keys := make([]string, 0, len(blocked))
			for key := range blocked {
				keys = append(keys, key)
			}
			query = query.Where("aa.key NOT IN ?", keys)
		}
	} else {
		keys := make([]string, 0, len(allowed))
		for key := range allowed {
			keys = append(keys, key)
		}
		query = query.Where("aa.key IN ?", keys)
	}

	var rows []facetRow
	if err := query.
		Group("aa.key, aa.value").
		Order("aa.key ASC, count DESC, aa.value ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	facets := make([]domain.AttributeFacet, 0)
	for _, row := range rows {
		if len(facets) == 0 || facets[len(facets)-1].Key != row.Key {
			facets = append(facets, domain.AttributeFacet{Key: row.Key})
		}
		last := &facets[len(facets)-1]
		last.Values = append(last.Values, domain.AttributeFacetValue{
			Value: row.Value,
			Count: row.Count,
		})
	}

	metadata, err := store.ListAttributeMetadata(s.db.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	keyLabels := make(map[string]string)
	keySorts := make(map[string]string)
	keyRenders := make(map[string]string)
	keySearchable := make(map[string]bool)
	keyMulti := make(map[string]bool)
	keyUnassigned := make(map[string]bool)
	keyUnassignedLabels := make(map[string]string)
	keyParents := make(map[string]string)
	keyValuesLocked := make(map[string]bool)
	keyValueSearch := make(map[string]bool)
	keyDescriptions := make(map[string]string)
	keyColors := make(map[string]string)
	keyPositions := make(map[string]int)
	valueLabels := make(map[string]string)
	valueDescriptions := make(map[string]string)
	valueColors := make(map[string]string)
	valuePositions := make(map[string]int)
	for _, item := range metadata {
		if item.Value == "" {
			if item.Label != "" {
				keyLabels[item.Key] = item.Label
			}
			if item.Sort != "" {
				keySorts[item.Key] = item.Sort
			}
			if item.Render != "" {
				keyRenders[item.Key] = item.Render
			}
			keySearchable[item.Key] = item.Searchable
			keyMulti[item.Key] = item.Multi
			keyUnassigned[item.Key] = item.IncludeUnassigned
			if item.UnassignedLabel != "" {
				keyUnassignedLabels[item.Key] = item.UnassignedLabel
			}
			if item.ParentKey != "" {
				keyParents[item.Key] = item.ParentKey
			}
			keyValuesLocked[item.Key] = item.ValuesLocked
			keyValueSearch[item.Key] = item.ValueSearch
			if item.Description != "" {
				keyDescriptions[item.Key] = item.Description
			}
			if item.Color != "" {
				keyColors[item.Key] = item.Color
			}
			keyPositions[item.Key] = item.Position
			continue
		}
		valueKey := item.Key + "\x00" + item.Value
		if item.Label != "" {
			valueLabels[valueKey] = item.Label
		}
		if item.Description != "" {
			valueDescriptions[valueKey] = item.Description
		}
		if item.Color != "" {
			valueColors[valueKey] = item.Color
		}
		valuePositions[valueKey] = item.Position
	}
	for i := range facets {
		facet := &facets[i]
		facet.Label = keyLabels[facet.Key]
		if facet.Label == "" {
			facet.Label = facet.Key
		}
		facet.Sort = keySorts[facet.Key]
		if !domain.ValidAttributeSort(facet.Sort) {
			facet.Sort = domain.AttributeDefaultSort
		}
		facet.Render = keyRenders[facet.Key]
		if !domain.ValidAttributeRender(facet.Render) {
			facet.Render = domain.AttributeDefaultRender
		}
		facet.Searchable = keySearchable[facet.Key]
		if multi, ok := keyMulti[facet.Key]; ok {
			facet.Multi = multi
		} else {
			facet.Multi = true
		}
		facet.IncludeUnassigned = keyUnassigned[facet.Key]
		facet.ParentKey = keyParents[facet.Key]
		facet.ValuesLocked = keyValuesLocked[facet.Key]
		facet.ValueSearch = keyValueSearch[facet.Key]
		facet.Description = keyDescriptions[facet.Key]
		facet.Color = keyColors[facet.Key]
		facet.Position = keyPositions[facet.Key]
		for j := range facet.Values {
			value := &facet.Values[j]
			valueKey := facet.Key + "\x00" + value.Value
			value.Label = valueLabels[valueKey]
			if value.Label == "" {
				value.Label = value.Value
			}
			value.Description = valueDescriptions[valueKey]
			value.Color = valueColors[valueKey]
			value.Position = valuePositions[valueKey]
		}
		if facet.IncludeUnassigned || includeAllUnassigned {
			countQuery := s.db.WithContext(ctx).
				Model(&store.Asset{}).
				Where("assets.role = ?", domain.AssetRoleOriginal).
				Where(
					`NOT EXISTS (SELECT 1 FROM asset_attributes aa
					 WHERE aa.asset_id = assets.id AND aa.key = ?)`,
					facet.Key,
				)
			for key, values := range enforced {
				countQuery = countQuery.Where(
					`EXISTS (SELECT 1 FROM asset_attributes enforced
					 WHERE enforced.asset_id = assets.id AND enforced.key = ? AND enforced.value IN ?)`,
					key, values,
				)
			}
			var count int64
			if err := countQuery.Count(&count).Error; err != nil {
				return nil, err
			}
			label := keyUnassignedLabels[facet.Key]
			if label == "" {
				label = "Unassigned"
			}
			valueKey := facet.Key + "\x00" + domain.AttributeUnassigned
			facet.Values = append(facet.Values, domain.AttributeFacetValue{
				Value:       domain.AttributeUnassigned,
				Label:       label,
				Description: valueDescriptions[valueKey],
				Color:       valueColors[valueKey],
				Position:    valuePositions[valueKey],
				Count:       count,
			})
		}
	}
	return facets, nil
}

// CreateAttribute creates a new key-level attribute definition.
func (s *AssetService) CreateAttribute(ctx context.Context, key, label string) error {
	key = strings.TrimSpace(key)
	label = strings.TrimSpace(label)
	if key == "" {
		return fmt.Errorf("%w: attribute key is required", ErrInvalidInput)
	}
	if !canCreateAttributeKeys(ctx) {
		return ErrForbidden
	}
	rules, err := s.attributeRules(ctx, key)
	if err != nil {
		return err
	}
	if rules.Exists {
		return fmt.Errorf("%w: attribute key already exists", ErrInvalidInput)
	}
	return store.InsertAttributeKey(s.db.WithContext(ctx), key, label, time.Now().UTC())
}

// SetAttributeValuesLocked toggles creation of new values for a key.
func (s *AssetService) SetAttributeValuesLocked(ctx context.Context, key string, locked bool) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("%w: attribute key is required", ErrInvalidInput)
	}
	return store.UpsertAttributeValuesLocked(
		s.db.WithContext(ctx), key, locked, time.Now().UTC(),
	)
}

// SetAttributeLabel stores a display label for an attribute key (value == "")
// or one of its values. The underlying key/value strings are not changed.
func (s *AssetService) SetAttributeLabel(
	ctx context.Context,
	key, value, label string,
) error {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	label = strings.TrimSpace(label)
	if key == "" {
		return fmt.Errorf("%w: attribute key is required", ErrInvalidInput)
	}
	return store.UpsertAttributeLabel(
		s.db.WithContext(ctx), key, value, label, time.Now().UTC(),
	)
}

// SetAttributeSort stores the key-level ordering setting. "open" lets each user
// choose an order without the settings:attributes capability.
func (s *AssetService) SetAttributeSort(
	ctx context.Context,
	key, sort string,
) error {
	key = strings.TrimSpace(key)
	sort = strings.TrimSpace(sort)
	if key == "" {
		return fmt.Errorf("%w: attribute key is required", ErrInvalidInput)
	}
	if !domain.ValidAttributeSort(sort) {
		return fmt.Errorf("%w: invalid attribute sort", ErrInvalidInput)
	}
	return store.UpsertAttributeSort(
		s.db.WithContext(ctx), key, sort, time.Now().UTC(),
	)
}

// SetAttributeRender stores the key-level rendering setting. "open" lets each
// user choose a renderer without the settings:attributes capability.
func (s *AssetService) SetAttributeRender(
	ctx context.Context,
	key, render string,
) error {
	key = strings.TrimSpace(key)
	render = strings.TrimSpace(render)
	if key == "" {
		return fmt.Errorf("%w: attribute key is required", ErrInvalidInput)
	}
	if !domain.ValidAttributeRender(render) {
		return fmt.Errorf("%w: invalid attribute render", ErrInvalidInput)
	}
	return store.UpsertAttributeRender(
		s.db.WithContext(ctx), key, render, time.Now().UTC(),
	)
}

func sortAttributeFacets(facets []domain.AttributeFacet) {
	sort.SliceStable(facets, func(i, j int) bool {
		if facets[i].Position != facets[j].Position {
			return facets[i].Position < facets[j].Position
		}
		left := facets[i].Label
		if left == "" {
			left = facets[i].Key
		}
		right := facets[j].Label
		if right == "" {
			right = facets[j].Key
		}
		return left < right
	})
}

func attributeKeyVisible(actor *domain.Actor, key string) bool {
	if actor == nil || !actor.Token {
		return true
	}
	if actor.Can(domain.CapabilitySettingsAttributes) ||
		actor.Can(domain.CapabilityAttributesCreateKeys) {
		return true
	}
	for _, grant := range actor.Scope.Attributes {
		if grant.Key == key {
			return grant.Mode == domain.AttributeModeActive
		}
	}
	return actor.Scope.AllAttributes
}

// AttributeCatalog returns the filter facets plus metadata-only values for
// keys that are already in use. It is used by the management dialog and the
// inspector so newly created vocabulary is selectable before any asset uses it.
func (s *AssetService) AttributeCatalog(ctx context.Context) ([]domain.AttributeFacet, error) {
	facets, err := s.attributeFacets(ctx, true)
	if err != nil {
		return nil, err
	}
	index := make(map[string]int, len(facets))
	for i := range facets {
		index[facets[i].Key] = i
	}
	metadata, err := store.ListAttributeMetadata(s.db.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	actor := domain.ActorFromContext(ctx)
	for _, item := range metadata {
		if item.Value == "" {
			if _, ok := index[item.Key]; ok {
				continue
			}
			if !attributeKeyVisible(actor, item.Key) {
				continue
			}
			facet := domain.AttributeFacet{
				Key:               item.Key,
				Label:             item.Label,
				Sort:              item.Sort,
				Render:            item.Render,
				Searchable:        item.Searchable,
				Multi:             item.Multi,
				IncludeUnassigned: item.IncludeUnassigned,
				ValuesLocked:      item.ValuesLocked,
				ValueSearch:       item.ValueSearch,
				ParentKey:         item.ParentKey,
				Description:       item.Description,
				Color:             item.Color,
				Position:          item.Position,
			}
			if facet.Label == "" {
				facet.Label = facet.Key
			}
			if !domain.ValidAttributeSort(facet.Sort) {
				facet.Sort = domain.AttributeDefaultSort
			}
			if !domain.ValidAttributeRender(facet.Render) {
				facet.Render = domain.AttributeDefaultRender
			}
			index[item.Key] = len(facets)
			facets = append(facets, facet)
			continue
		}
		position, ok := index[item.Key]
		if !ok {
			continue
		}
		facet := &facets[position]
		if item.Value == domain.AttributeUnassigned && !facet.IncludeUnassigned {
			continue
		}
		exists := false
		for _, value := range facet.Values {
			if value.Value == item.Value {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		label := item.Label
		if label == "" {
			if item.Value == domain.AttributeUnassigned {
				label = "Unassigned"
			} else {
				label = item.Value
			}
		}
		facet.Values = append(facet.Values, domain.AttributeFacetValue{
			Value:       item.Value,
			Label:       label,
			Description: item.Description,
			Color:       item.Color,
			Position:    item.Position,
			Count:       0,
		})
	}
	sortAttributeFacets(facets)
	return facets, nil
}

// AddAttributeValues creates metadata-only values for an existing key. The
// values become available for selection even before an asset uses them.
func (s *AssetService) AddAttributeValues(
	ctx context.Context,
	key string,
	values []string,
) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("%w: attribute key is required", ErrInvalidInput)
	}
	if containsAttributeUnassigned(values) {
		return fmt.Errorf("%w: unassigned is not a real attribute value", ErrInvalidInput)
	}
	cleaned := cleanAttributeValues(values)
	if len(cleaned) == 0 {
		return fmt.Errorf("%w: at least one attribute value is required", ErrInvalidInput)
	}
	var exists int64
	if err := s.db.WithContext(ctx).
		Model(&store.AssetAttribute{}).
		Where("key = ?", key).
		Count(&exists).Error; err != nil {
		return err
	}
	if exists == 0 {
		var keyRows int64
		if err := s.db.WithContext(ctx).
			Model(&store.AttributeMetadata{}).
			Where("key = ? AND value = ''", key).
			Count(&keyRows).Error; err != nil {
			return err
		}
		if keyRows == 0 {
			return fmt.Errorf("%w: unknown attribute key", ErrInvalidInput)
		}
	}
	if !canCreateAttributeValues(ctx) {
		return ErrForbidden
	}
	rules, err := s.attributeRules(ctx, key)
	if err != nil {
		return err
	}
	if rules.Locked {
		return fmt.Errorf("%w: attribute %q is locked", ErrForbidden, key)
	}
	return store.InsertAttributeValues(
		s.db.WithContext(ctx), key, cleaned, time.Now().UTC(),
	)
}

// SetAssetAttributeValues replaces the values for one attribute key on an
// asset. An empty values slice removes the key from the asset.
func (s *AssetService) SetAssetAttributeValues(
	ctx context.Context,
	assetID, key string,
	values []string,
) (*domain.Asset, error) {
	assetID = strings.TrimSpace(assetID)
	key = strings.TrimSpace(key)
	if assetID == "" || key == "" {
		return nil, fmt.Errorf("%w: asset id and attribute key are required", ErrInvalidInput)
	}
	if !actorCanWriteAttribute(ctx, key) {
		return nil, ErrForbidden
	}
	if containsAttributeUnassigned(values) {
		return nil, fmt.Errorf("%w: unassigned is not a real attribute value", ErrInvalidInput)
	}
	cleaned := cleanAttributeValues(values)
	if err := s.validateAttributeAssignment(ctx, key, cleaned); err != nil {
		return nil, err
	}

	var asset store.Asset
	if err := s.db.WithContext(ctx).First(&asset, "id = ?", assetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	now := time.Now().UTC()
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer func() { _ = tx.Rollback() }()

	if err := store.ReplaceAssetAttributeKey(tx, assetID, key, cleaned); err != nil {
		return nil, err
	}
	if err := tx.Model(&store.Asset{}).
		Where("id = ?", assetID).
		Update("updated_at", now).Error; err != nil {
		return nil, err
	}

	var visuals []string
	if err := tx.Model(&store.Asset{}).
		Where("image_of = ? AND role = ?", assetID, domain.AssetRoleVisual).
		Pluck("id", &visuals).Error; err != nil {
		return nil, err
	}
	for _, visualID := range visuals {
		if err := store.ReplaceAssetAttributeKey(tx, visualID, key, cleaned); err != nil {
			return nil, err
		}
		if err := tx.Model(&store.Asset{}).
			Where("id = ?", visualID).
			Update("updated_at", now).Error; err != nil {
			return nil, err
		}
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return s.GetByID(ctx, assetID)
}

func containsAttributeUnassigned(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == domain.AttributeUnassigned {
			return true
		}
	}
	return false
}

func cleanAttributeValues(values []string) []string {
	seen := make(map[string]bool, len(values))
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		cleaned = append(cleaned, value)
	}
	return cleaned
}

type attributeRules struct {
	Exists  bool
	Locked  bool
	Multi   bool
	Allowed map[string]bool
}

func (s *AssetService) attributeRules(ctx context.Context, key string) (attributeRules, error) {
	rules := attributeRules{Multi: true, Allowed: make(map[string]bool)}
	var rows []store.AttributeMetadata
	if err := s.db.WithContext(ctx).
		Where("key = ?", key).
		Find(&rows).Error; err != nil {
		return rules, err
	}
	if len(rows) > 0 {
		rules.Exists = true
	}
	for _, row := range rows {
		if row.Value == "" {
			rules.Locked = row.ValuesLocked
			rules.Multi = row.Multi
			continue
		}
		rules.Allowed[row.Value] = true
	}
	var used []string
	if err := s.db.WithContext(ctx).
		Model(&store.AssetAttribute{}).
		Where("key = ?", key).
		Distinct().
		Pluck("value", &used).Error; err != nil {
		return rules, err
	}
	if len(used) > 0 {
		rules.Exists = true
	}
	for _, value := range used {
		rules.Allowed[value] = true
	}
	return rules, nil
}

func canCreateAttributeKeys(ctx context.Context) bool {
	actor := domain.ActorFromContext(ctx)
	if actor == nil {
		return true
	}
	return actor.Can(domain.CapabilitySettingsAttributes) ||
		actor.Can(domain.CapabilityAttributesCreateKeys)
}

func canCreateAttributeValues(ctx context.Context) bool {
	actor := domain.ActorFromContext(ctx)
	if actor == nil {
		return true
	}
	return actor.Can(domain.CapabilitySettingsAttributes) ||
		actor.Can(domain.CapabilityAttributesCreateValues)
}

func (s *AssetService) uploadAttributePolicy(ctx context.Context) (bool, bool, error) {
	var instance store.Instance
	err := s.db.WithContext(ctx).Order("created_at ASC").First(&instance).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true, true, nil
	}
	if err != nil {
		return false, false, err
	}
	return instance.UploadAttributeKeys, instance.UploadAttributeValues, nil
}

func (s *AssetService) validateUploadAttributes(
	ctx context.Context,
	attributes []domain.AssetAttribute,
) error {
	if len(attributes) == 0 {
		return nil
	}
	allowKeys, allowValues, err := s.uploadAttributePolicy(ctx)
	if err != nil {
		return err
	}
	groups := make(map[string]map[string]bool)
	for _, attribute := range attributes {
		key := strings.TrimSpace(attribute.Key)
		value := strings.TrimSpace(attribute.Value)
		if key == "" || value == "" {
			continue
		}
		if groups[key] == nil {
			groups[key] = make(map[string]bool)
		}
		groups[key][value] = true
	}
	for key, valueSet := range groups {
		rules, err := s.attributeRules(ctx, key)
		if err != nil {
			return err
		}
		if !rules.Exists && !allowKeys {
			return fmt.Errorf("%w: new attribute keys are not allowed on upload", ErrForbidden)
		}
		if !rules.Multi && len(valueSet) > 1 {
			return fmt.Errorf("%w: attribute %q is single-valued", ErrInvalidInput, key)
		}
		for value := range valueSet {
			if rules.Allowed[value] {
				continue
			}
			if rules.Locked {
				return fmt.Errorf("%w: attribute %q is locked", ErrForbidden, key)
			}
			if !allowValues {
				return fmt.Errorf("%w: new attribute values are not allowed on upload", ErrForbidden)
			}
		}
	}
	return nil
}

func (s *AssetService) validateAttributeAssignment(
	ctx context.Context,
	key string,
	values []string,
) error {
	rules, err := s.attributeRules(ctx, key)
	if err != nil {
		return err
	}
	if !rules.Exists {
		return fmt.Errorf("%w: unknown attribute key", ErrInvalidInput)
	}
	if !rules.Multi && len(values) > 1 {
		return fmt.Errorf("%w: attribute %q is single-valued", ErrInvalidInput, key)
	}
	for _, value := range values {
		if rules.Allowed[value] {
			continue
		}
		if rules.Locked {
			return fmt.Errorf("%w: attribute %q is locked", ErrForbidden, key)
		}
		if !canCreateAttributeValues(ctx) {
			return ErrForbidden
		}
	}
	return nil
}

func actorCanWriteAttribute(ctx context.Context, key string) bool {
	actor := domain.ActorFromContext(ctx)
	if actor == nil || !actor.Token {
		return true
	}
	for _, grant := range actor.Scope.Attributes {
		if grant.Key != key {
			continue
		}
		if grant.Mode == domain.AttributeModeActive {
			return true
		}
		return false
	}
	return actor.Scope.AllAttributes
}

// SetAttributeSearchable toggles whether the key participates in asset search.
func (s *AssetService) SetAttributeSearchable(
	ctx context.Context,
	key string,
	searchable bool,
) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("%w: attribute key is required", ErrInvalidInput)
	}
	return store.UpsertAttributeSearchable(
		s.db.WithContext(ctx), key, searchable, time.Now().UTC(),
	)
}

// SetAttributeMulti controls whether renderers add or replace values on click.
func (s *AssetService) SetAttributeMulti(
	ctx context.Context,
	key string,
	multi bool,
) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("%w: attribute key is required", ErrInvalidInput)
	}
	return store.UpsertAttributeMulti(
		s.db.WithContext(ctx), key, multi, time.Now().UTC(),
	)
}

// SetAttributeUnassigned controls whether renderers expose an Unassigned
// option for assets with no values for the key.
func (s *AssetService) SetAttributeUnassigned(
	ctx context.Context,
	key string,
	include bool,
) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("%w: attribute key is required", ErrInvalidInput)
	}
	return store.UpsertAttributeUnassigned(
		s.db.WithContext(ctx), key, include, time.Now().UTC(),
	)
}

// SetAttributeUnassignedLabel overrides the synthetic Unassigned label.
func (s *AssetService) SetAttributeUnassignedLabel(
	ctx context.Context,
	key, label string,
) error {
	key = strings.TrimSpace(key)
	label = strings.TrimSpace(label)
	if key == "" {
		return fmt.Errorf("%w: attribute key is required", ErrInvalidInput)
	}
	return store.UpsertAttributeUnassignedLabel(
		s.db.WithContext(ctx), key, label, time.Now().UTC(),
	)
}

// SetAttributeDescription stores a key-level or value-level description.
func (s *AssetService) SetAttributeDescription(
	ctx context.Context,
	key, value, description string,
) error {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" {
		return fmt.Errorf("%w: attribute key is required", ErrInvalidInput)
	}
	return store.UpsertAttributeDescription(
		s.db.WithContext(ctx), key, value, strings.TrimSpace(description), time.Now().UTC(),
	)
}

// SetAttributeColor stores a key-level or value-level color.
func (s *AssetService) SetAttributeColor(
	ctx context.Context,
	key, value, color string,
) error {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" {
		return fmt.Errorf("%w: attribute key is required", ErrInvalidInput)
	}
	return store.UpsertAttributeColor(
		s.db.WithContext(ctx), key, value, strings.TrimSpace(color), time.Now().UTC(),
	)
}

// SetAttributeValueOrder stores the custom value order for a key.
func (s *AssetService) SetAttributeValueOrder(
	ctx context.Context,
	key string,
	values []string,
) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("%w: attribute key is required", ErrInvalidInput)
	}
	cleaned := cleanAttributeValues(values)
	return store.SetAttributeValuePositions(
		s.db.WithContext(ctx), key, cleaned, time.Now().UTC(),
	)
}

// SetAttributeKeyOrder stores the display order of attribute keys.
func (s *AssetService) SetAttributeKeyOrder(
	ctx context.Context,
	keys []string,
) error {
	cleaned := make([]string, 0, len(keys))
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, key)
	}
	if len(cleaned) == 0 {
		return fmt.Errorf("%w: at least one attribute key is required", ErrInvalidInput)
	}
	return store.SetAttributeKeyPositions(
		s.db.WithContext(ctx), cleaned, time.Now().UTC(),
	)
}

// SetAttributeValueSearch toggles the per-key sidebar value search box.
func (s *AssetService) SetAttributeValueSearch(
	ctx context.Context,
	key string,
	enabled bool,
) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("%w: attribute key is required", ErrInvalidInput)
	}
	return store.UpsertAttributeValueSearch(
		s.db.WithContext(ctx), key, enabled, time.Now().UTC(),
	)
}

// SetAttributeParent stores the key-level parent used by the sidebar tree.
func (s *AssetService) SetAttributeParent(
	ctx context.Context,
	key, parent string,
) error {
	key = strings.TrimSpace(key)
	parent = strings.TrimSpace(parent)
	if key == "" {
		return fmt.Errorf("%w: attribute key is required", ErrInvalidInput)
	}
	if key == parent {
		return fmt.Errorf("%w: an attribute cannot be its own parent", ErrInvalidInput)
	}
	metadata, err := store.ListAttributeMetadata(s.db.WithContext(ctx))
	if err != nil {
		return err
	}
	exists := make(map[string]bool)
	parents := make(map[string]string)
	for _, item := range metadata {
		exists[item.Key] = true
		if item.Value == "" && item.ParentKey != "" {
			parents[item.Key] = item.ParentKey
		}
	}
	if parent != "" {
		if !exists[parent] {
			var count int64
			if err := s.db.WithContext(ctx).
				Model(&store.AssetAttribute{}).
				Where("key = ?", parent).
				Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return fmt.Errorf("%w: parent attribute does not exist", ErrInvalidInput)
			}
		}
		seen := map[string]bool{key: true}
		current := parent
		for current != "" {
			if seen[current] {
				return fmt.Errorf("%w: attribute parent cycle", ErrInvalidInput)
			}
			seen[current] = true
			current = parents[current]
		}
	}
	return store.UpsertAttributeParent(
		s.db.WithContext(ctx), key, parent, time.Now().UTC(),
	)
}

// AttributeTree builds the nested sidebar trees from parent_key metadata.
func (s *AssetService) AttributeTree(ctx context.Context) ([]domain.AttributeTree, error) {
	facets, err := s.AttributeCatalog(ctx)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]*domain.AttributeFacet, len(facets))
	for i := range facets {
		byKey[facets[i].Key] = &facets[i]
	}
	children := make(map[string][]string)
	keys := make([]string, 0, len(facets))
	for i := range facets {
		facet := &facets[i]
		if facet.ParentKey != "" {
			if _, ok := byKey[facet.ParentKey]; ok && facet.ParentKey != facet.Key {
				children[facet.ParentKey] = append(children[facet.ParentKey], facet.Key)
			}
		}
		keys = append(keys, facet.Key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		if byKey[keys[i]].Position != byKey[keys[j]].Position {
			return byKey[keys[i]].Position < byKey[keys[j]].Position
		}
		return byKey[keys[i]].Label < byKey[keys[j]].Label
	})
	enforced := attributeEnforced(ctx)
	trees := make([]domain.AttributeTree, 0, len(keys))
	for _, key := range keys {
		facet := byKey[key]
		nodes := make([]domain.AttributeTreeNode, 0, len(facet.Values))
		for _, value := range facet.Values {
			if value.Value == domain.AttributeUnassigned && !facet.IncludeUnassigned {
				continue
			}
			color := value.Color
			if color == "" {
				color = facet.Color
			}
			node := domain.AttributeTreeNode{
				Key:         facet.Key,
				Value:       value.Value,
				Label:       value.Label,
				Description: value.Description,
				Color:       color,
				Position:    value.Position,
				Count:       value.Count,
			}
			node.Children = s.attributeTreeChildren(
				ctx, facet.Key, value.Value, nil, byKey, children, enforced, map[string]bool{},
			)
			nodes = append(nodes, node)
		}
		sortAttributeTreeNodes(nodes, byKey)
		trees = append(trees, domain.AttributeTree{
			Key: facet.Key, Label: facet.Label, Values: nodes,
		})
	}
	return trees, nil
}

func attributeEnforced(ctx context.Context) map[string][]string {
	enforced := make(map[string][]string)
	actor := domain.ActorFromContext(ctx)
	if actor == nil || !actor.Token {
		return enforced
	}
	for _, grant := range actor.Scope.Attributes {
		if (grant.Mode == domain.AttributeModeLocked || grant.Mode == domain.AttributeModeHidden) &&
			len(grant.Values) > 0 {
			enforced[grant.Key] = grant.Values
		}
	}
	return enforced
}

func (s *AssetService) attributeTreeChildren(
	ctx context.Context,
	parentKey, parentValue string,
	constraints []domain.AssetAttribute,
	byKey map[string]*domain.AttributeFacet,
	children map[string][]string,
	enforced map[string][]string,
	visited map[string]bool,
) []domain.AttributeTreeNode {
	path := append(append([]domain.AssetAttribute{}, constraints...),
		domain.AssetAttribute{Key: parentKey, Value: parentValue})
	if parentValue == domain.AttributeUnassigned {
		return nil
	}
	nodes := make([]domain.AttributeTreeNode, 0)
	for _, childKey := range children[parentKey] {
		if visited[childKey] {
			continue
		}
		childFacet := byKey[childKey]
		if childFacet == nil {
			continue
		}
		nextVisited := make(map[string]bool, len(visited)+1)
		for key := range visited {
			nextVisited[key] = true
		}
		nextVisited[childKey] = true
		labels := make(map[string]string, len(childFacet.Values))
		descriptions := make(map[string]string, len(childFacet.Values))
		colors := make(map[string]string, len(childFacet.Values))
		positions := make(map[string]int, len(childFacet.Values))
		for _, value := range childFacet.Values {
			labels[value.Value] = value.Label
			descriptions[value.Value] = value.Description
			colors[value.Value] = value.Color
			positions[value.Value] = value.Position
		}
		values, err := s.attributeTreeValues(ctx, childKey, path, enforced)
		if err != nil {
			return nodes
		}
		childNodes := make([]domain.AttributeTreeNode, 0, len(values)+1)
		for _, value := range values {
			if value.Value == domain.AttributeUnassigned {
				continue
			}
			label := labels[value.Value]
			if label == "" {
				label = value.Value
			}
			color := colors[value.Value]
			if color == "" {
				color = childFacet.Color
			}
			node := domain.AttributeTreeNode{
				Key:         childKey,
				Value:       value.Value,
				Label:       label,
				Description: descriptions[value.Value],
				Color:       color,
				Position:    positions[value.Value],
				Count:       value.Count,
			}
			node.Children = s.attributeTreeChildren(
				ctx, childKey, value.Value, path, byKey, children, enforced, nextVisited,
			)
			childNodes = append(childNodes, node)
		}
		if childFacet.IncludeUnassigned {
			count, err := s.attributeTreeUnassigned(ctx, childKey, path, enforced)
			if err == nil {
				label := "Unassigned"
				description := ""
				color := childFacet.Color
				position := positions[domain.AttributeUnassigned]
				for _, value := range childFacet.Values {
					if value.Value == domain.AttributeUnassigned {
						if value.Label != "" {
							label = value.Label
						}
						description = value.Description
						if value.Color != "" {
							color = value.Color
						}
						position = value.Position
					}
				}
				childNodes = append(childNodes, domain.AttributeTreeNode{
					Key:         childKey,
					Value:       domain.AttributeUnassigned,
					Label:       label,
					Description: description,
					Color:       color,
					Position:    position,
					Count:       count,
				})
			}
		}
		sortAttributeTreeNodes(childNodes, byKey)
		nodes = append(nodes, childNodes...)
	}
	return nodes
}

func (s *AssetService) attributeTreeValues(
	ctx context.Context,
	key string,
	constraints []domain.AssetAttribute,
	enforced map[string][]string,
) ([]domain.AttributeFacetValue, error) {
	type treeRow struct {
		Value string
		Count int64
	}
	var rows []treeRow
	query := s.db.WithContext(ctx).
		Table("asset_attributes AS aa").
		Select("aa.value AS value, COUNT(DISTINCT aa.asset_id) AS count").
		Joins("JOIN assets ON assets.id = aa.asset_id").
		Where("assets.role = ? AND aa.key = ?", domain.AssetRoleOriginal, key)
	for _, constraint := range constraints {
		query = query.Where(
			`EXISTS (SELECT 1 FROM asset_attributes anc
			 WHERE anc.asset_id = assets.id AND anc.key = ? AND anc.value = ?)`,
			constraint.Key, constraint.Value,
		)
	}
	for enforcedKey, values := range enforced {
		query = query.Where(
			`EXISTS (SELECT 1 FROM asset_attributes enforced
			 WHERE enforced.asset_id = assets.id AND enforced.key = ? AND enforced.value IN ?)`,
			enforcedKey, values,
		)
	}
	if err := query.Group("aa.value").
		Order("count DESC, aa.value ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	values := make([]domain.AttributeFacetValue, 0, len(rows))
	for _, row := range rows {
		if row.Value == "" {
			continue
		}
		values = append(values, domain.AttributeFacetValue{Value: row.Value, Count: row.Count})
	}
	return values, nil
}

func (s *AssetService) attributeTreeUnassigned(
	ctx context.Context,
	key string,
	constraints []domain.AssetAttribute,
	enforced map[string][]string,
) (int64, error) {
	query := s.db.WithContext(ctx).
		Model(&store.Asset{}).
		Where("assets.role = ?", domain.AssetRoleOriginal).
		Where(
			`NOT EXISTS (SELECT 1 FROM asset_attributes aa
			 WHERE aa.asset_id = assets.id AND aa.key = ?)`, key,
		)
	for _, constraint := range constraints {
		query = query.Where(
			`EXISTS (SELECT 1 FROM asset_attributes anc
			 WHERE anc.asset_id = assets.id AND anc.key = ? AND anc.value = ?)`,
			constraint.Key, constraint.Value,
		)
	}
	for enforcedKey, values := range enforced {
		query = query.Where(
			`EXISTS (SELECT 1 FROM asset_attributes enforced
			 WHERE enforced.asset_id = assets.id AND enforced.key = ? AND enforced.value IN ?)`,
			enforcedKey, values,
		)
	}
	var count int64
	err := query.Count(&count).Error
	return count, err
}

func sortAttributeTreeNodes(
	nodes []domain.AttributeTreeNode,
	byKey map[string]*domain.AttributeFacet,
) {
	if len(nodes) == 0 {
		return
	}
	facet := byKey[nodes[0].Key]
	sort.SliceStable(nodes, func(i, j int) bool {
		if facet == nil {
			return nodes[i].Label < nodes[j].Label
		}
		switch facet.Sort {
		case domain.AttributeSortCountAsc:
			if nodes[i].Count != nodes[j].Count {
				return nodes[i].Count < nodes[j].Count
			}
		case domain.AttributeSortABCAsc:
			return nodes[i].Label < nodes[j].Label
		case domain.AttributeSortABCDesc:
			return nodes[i].Label > nodes[j].Label
		case domain.AttributeSortCustom:
			if nodes[i].Position != nodes[j].Position {
				return nodes[i].Position < nodes[j].Position
			}
		default:
			if nodes[i].Count != nodes[j].Count {
				return nodes[i].Count > nodes[j].Count
			}
		}
		return nodes[i].Label < nodes[j].Label
	})
	for i := range nodes {
		sortAttributeTreeNodes(nodes[i].Children, byKey)
	}
}

// DeleteAttributeValue removes a value from metadata and from every asset.
func (s *AssetService) DeleteAttributeValue(
	ctx context.Context,
	key, value string,
) error {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" || value == domain.AttributeUnassigned {
		return fmt.Errorf("%w: attribute key and value are required", ErrInvalidInput)
	}
	now := time.Now().UTC()
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() { _ = tx.Rollback() }()

	var ids []string
	if err := tx.Model(&store.AssetAttribute{}).
		Where("key = ? AND value = ?", key, value).
		Pluck("asset_id", &ids).Error; err != nil {
		return err
	}
	if err := store.DeleteAttributeValue(tx, key, value); err != nil {
		return err
	}
	if len(ids) > 0 {
		if err := tx.Model(&store.Asset{}).
			Where("id IN ?", ids).
			Update("updated_at", now).Error; err != nil {
			return err
		}
	}
	return tx.Commit().Error
}

// DeleteAttributeKey removes a key from metadata and from every asset.
func (s *AssetService) DeleteAttributeKey(
	ctx context.Context,
	key string,
) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("%w: attribute key is required", ErrInvalidInput)
	}
	now := time.Now().UTC()
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() { _ = tx.Rollback() }()

	var ids []string
	if err := tx.Model(&store.AssetAttribute{}).
		Where("key = ?", key).
		Pluck("asset_id", &ids).Error; err != nil {
		return err
	}
	if err := store.DeleteAttributeKey(tx, key); err != nil {
		return err
	}
	if len(ids) > 0 {
		if err := tx.Model(&store.Asset{}).
			Where("id IN ?", ids).
			Update("updated_at", now).Error; err != nil {
			return err
		}
	}
	return tx.Commit().Error
}

// ApplyAssetAttributesBulk applies one attribute key to many assets. Mode
// "add" merges values into existing values; "replace" sets the exact set.
func (s *AssetService) ApplyAssetAttributesBulk(
	ctx context.Context,
	assetIDs []string,
	key string,
	values []string,
	mode string,
) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("%w: attribute key is required", ErrInvalidInput)
	}
	if mode != "add" && mode != "replace" {
		return fmt.Errorf("%w: attribute mode must be add or replace", ErrInvalidInput)
	}
	if !actorCanWriteAttribute(ctx, key) {
		return ErrForbidden
	}
	if containsAttributeUnassigned(values) {
		return fmt.Errorf("%w: unassigned is not a real attribute value", ErrInvalidInput)
	}
	cleanedValues := cleanAttributeValues(values)
	cleanedIDs := make([]string, 0, len(assetIDs))
	seen := make(map[string]bool, len(assetIDs))
	for _, id := range assetIDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		cleanedIDs = append(cleanedIDs, id)
	}
	if len(cleanedIDs) == 0 {
		return fmt.Errorf("%w: at least one asset is required", ErrInvalidInput)
	}
	if mode == "add" && len(cleanedValues) == 0 {
		return nil
	}
	if err := s.validateAttributeAssignment(ctx, key, cleanedValues); err != nil {
		return err
	}
	if mode == "add" && len(cleanedValues) > 0 {
		rules, err := s.attributeRules(ctx, key)
		if err != nil {
			return err
		}
		if !rules.Multi {
			var conflicts int64
			if err := s.db.WithContext(ctx).
				Model(&store.AssetAttribute{}).
				Where("asset_id IN ? AND key = ? AND value <> ?", cleanedIDs, key, cleanedValues[0]).
				Count(&conflicts).Error; err != nil {
				return err
			}
			if conflicts > 0 {
				return fmt.Errorf("%w: attribute %q is single-valued", ErrInvalidInput, key)
			}
		}
	}

	now := time.Now().UTC()
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() { _ = tx.Rollback() }()

	var visuals []struct {
		ID      string
		ImageOf string
	}
	if err := tx.Model(&store.Asset{}).
		Select("id, image_of").
		Where("image_of IN ? AND role = ?", cleanedIDs, domain.AssetRoleVisual).
		Scan(&visuals).Error; err != nil {
		return err
	}
	visualByParent := make(map[string][]string)
	for _, visual := range visuals {
		visualByParent[visual.ImageOf] = append(visualByParent[visual.ImageOf], visual.ID)
	}

	apply := func(tx *gorm.DB, id string) error {
		if mode == "replace" {
			return store.ReplaceAssetAttributeKey(tx, id, key, cleanedValues)
		}
		return store.AddAssetAttributeKeyValues(tx, id, key, cleanedValues)
	}

	for _, id := range cleanedIDs {
		if err := apply(tx, id); err != nil {
			return err
		}
		for _, visualID := range visualByParent[id] {
			if err := apply(tx, visualID); err != nil {
				return err
			}
		}
	}

	allIDs := append([]string{}, cleanedIDs...)
	for _, visual := range visuals {
		allIDs = append(allIDs, visual.ID)
	}
	if err := tx.Model(&store.Asset{}).
		Where("id IN ?", allIDs).
		Update("updated_at", now).Error; err != nil {
		return err
	}
	return tx.Commit().Error
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

	uploaderName := ""
	var attributes []domain.AssetAttribute
	if actor := domain.ActorFromContext(ctx); actor != nil {
		uploaderName = actor.Name
		attributes = actor.Scope.UploadAttributes
	}
	if err := s.validateUploadAttributes(ctx, attributes); err != nil {
		return nil, err
	}

	id := uuid.New().String()
	ext := filepath.Ext(filename)
	storageKey := fmt.Sprintf("uploads/%s%s", id, ext)

	// 1. Save original to target storage driver (Local, SFTP, S3)
	if err := driver.Save(ctx, storageKey, file); err != nil {
		return nil, fmt.Errorf("failed to save to %s storage: %w", driverName, err)
	}

	now := time.Now().UTC()
	asset := &domain.Asset{
		ID:             id,
		UploadedByName: uploaderName,
		Attributes:     attributes,
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
	if err := store.ReplaceAssetAttributes(s.db.WithContext(ctx), id, attributes); err != nil {
		_ = s.db.WithContext(ctx).Delete(&store.Asset{}, "id = ?", id).Error
		_ = driver.Delete(ctx, storageKey)
		return nil, err
	}
	s.saveColors(ctx, id, colors)

	// 2. Store a derived visual asset for non-image types. Images are their
	// own visual source and are rendered on demand by imgproxy.
	if _, err := file.Seek(0, io.SeekStart); err == nil {
		_ = s.refreshVisualAsset(ctx, asset, file, PreviewOptions{})
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
		_ = s.refreshVisualAsset(ctx, updated, file, PreviewOptions{})
	}
	if s.backup != nil {
		_ = s.backup.EnqueueAsset(ctx, id, asset.StorageDriver, asset.StorageKey, size)
	}
	return updated, nil
}

// RegeneratePreview re-renders the derived visual asset for an asset, e.g.
// after preview settings change. The original file is untouched; updated_at is
// bumped so clients re-fetch the library preview.
func (s *AssetService) RegeneratePreview(ctx context.Context, id string, opts PreviewOptions) (*domain.Asset, error) {
	asset, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if opts.Page < 0 || (opts.Timestamp != nil && *opts.Timestamp < 0) {
		return nil, ErrInvalidInput
	}
	if opts.Page > 0 && asset.MimeType != "application/pdf" {
		return nil, fmt.Errorf("%w: page only applies to PDFs", ErrInvalidInput)
	}
	if opts.Timestamp != nil && !strings.HasPrefix(asset.MimeType, "video/") {
		return nil, fmt.Errorf("%w: timestamp only applies to videos", ErrInvalidInput)
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
	_ = s.refreshVisualAsset(ctx, updated, bytes.NewReader(data), opts)
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
