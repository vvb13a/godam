package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/store"
	"gorm.io/gorm"
)

// defaultSmartCollections are seeded for every library so the built-in
// Uncategorized/Untagged views are ordinary, editable smart collections.
var defaultSmartCollections = []struct {
	name   string
	color  string
	filter string
}{
	{name: "Uncategorized", color: "8b5cf6", filter: `{"collection":"uncategorized"}`},
	{name: "Untagged", color: "8b5cf6", filter: `{"tag":"untagged"}`},
}

// seedDefaultSmartCollections inserts the built-in presets, appended after any
// existing rows. It runs inside the library-creation transaction.
func seedDefaultSmartCollections(tx *gorm.DB, now time.Time) error {
	for i, def := range defaultSmartCollections {
		model := &store.SmartCollection{
			ID:        uuid.New().String(),
			Name:      def.name,
			Color:     def.color,
			Sort:      i,
			Filter:    def.filter,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := tx.Create(model).Error; err != nil {
			return err
		}
	}
	return nil
}

// SmartCollectionSort is a single smart collection's new position.
type SmartCollectionSort struct {
	ID   string `json:"id"`
	Sort int    `json:"sort"`
}

// documentMimes mirrors the frontend's document media category so a smart
// collection's "document" type filter counts the same assets the view shows.
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

// SmartCollectionService manages flat, library-scoped saved filter presets.
type SmartCollectionService struct {
	db *gorm.DB
}

func NewSmartCollectionService(db *gorm.DB) *SmartCollectionService {
	return &SmartCollectionService{db: db}
}

func (s *SmartCollectionService) List(ctx context.Context) ([]domain.SmartCollection, error) {
	var models []store.SmartCollection
	if err := s.db.WithContext(ctx).
		Order("sort ASC, name ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}
	collections := make([]domain.SmartCollection, 0, len(models))
	for i := range models {
		collection := models[i].ToDomain()
		count, err := s.countAssets(ctx, collection.Filter)
		if err != nil {
			return nil, err
		}
		collection.AssetCount = count
		collections = append(collections, *collection)
	}
	return collections, nil
}

// countAssets returns how many assets in the library match a saved filter. The
// special collection/tag values "uncategorized"/"untagged" and the broad media
// categories mirror the asset list's filtering.
func (s *SmartCollectionService) countAssets(ctx context.Context, filter domain.SmartCollectionFilter) (int, error) {
	query := s.db.WithContext(ctx).Model(&store.Asset{})

	switch filter.Collection {
	case "":
	case "uncategorized":
		query = query.Where("id NOT IN (SELECT asset_id FROM asset_collections)")
	default:
		query = query.Where(
			"id IN (SELECT asset_id FROM asset_collections WHERE collection_id = ?)",
			filter.Collection,
		)
	}

	switch filter.Tag {
	case "":
	case "untagged":
		query = query.Where("id NOT IN (SELECT asset_id FROM asset_tags)")
	default:
		ids := splitCSV(filter.Tag)
		if len(ids) > 0 {
			query = query.Where(
				"id IN (SELECT asset_id FROM asset_tags WHERE tag_id IN ?)",
				ids,
			)
		}
	}

	switch filter.Type {
	case "":
	case "image":
		query = query.Where("mime_type LIKE 'image/%'")
	case "video":
		query = query.Where("mime_type LIKE 'video/%'")
	case "audio":
		query = query.Where("mime_type LIKE 'audio/%'")
	case "document":
		query = query.Where("mime_type LIKE 'text/%' OR mime_type IN ?", documentMimes)
	case "other":
		query = query.
			Where("mime_type NOT LIKE 'image/%'").
			Where("mime_type NOT LIKE 'video/%'").
			Where("mime_type NOT LIKE 'audio/%'").
			Where("mime_type NOT LIKE 'text/%'").
			Where("mime_type NOT IN ?", documentMimes)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	ids := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			ids = append(ids, trimmed)
		}
	}
	return ids
}

func (s *SmartCollectionService) GetByID(ctx context.Context, id string) (*domain.SmartCollection, error) {
	var m store.SmartCollection
	err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return m.ToDomain(), nil
}

// Create stores a new filter preset, appended after its siblings.
func (s *SmartCollectionService) Create(ctx context.Context, name, description, color string, filter domain.SmartCollectionFilter) (*domain.SmartCollection, error) {
	unique, err := s.uniqueName(ctx, name)
	if err != nil {
		return nil, err
	}
	sort, err := s.nextSort(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	collection := &domain.SmartCollection{
		ID:          uuid.New().String(),
		Name:        unique,
		Description: description,
		Color:       color,
		Sort:        sort,
		Filter:      filter,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.db.WithContext(ctx).Create(store.SmartCollectionModel(collection)).Error; err != nil {
		return nil, err
	}
	return collection, nil
}

// Update sets a smart collection's name, description and colour. Callers send
// the complete desired state; the filter is immutable here.
func (s *SmartCollectionService) Update(ctx context.Context, id, name, description, color string) (*domain.SmartCollection, error) {
	res := s.db.WithContext(ctx).
		Model(&store.SmartCollection{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"name":        name,
			"description": description,
			"color":       color,
			"updated_at":  time.Now().UTC(),
		})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, domain.ErrNotFound
	}
	return s.GetByID(ctx, id)
}

func (s *SmartCollectionService) Delete(ctx context.Context, id string) error {
	res := s.db.WithContext(ctx).Delete(&store.SmartCollection{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Reorder applies a batch of sort positions in one transaction so a drag
// reorder is a single request and never leaves the order half-applied. Only
// rows in the given library are affected.
func (s *SmartCollectionService) Reorder(ctx context.Context, items []SmartCollectionSort) error {
	if len(items) == 0 {
		return nil
	}
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			res := tx.Model(&store.SmartCollection{}).
				Where("id = ?", item.ID).
				Updates(map[string]any{"sort": item.Sort, "updated_at": now})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return domain.ErrNotFound
			}
		}
		return nil
	})
}

func (s *SmartCollectionService) nextSort(ctx context.Context) (int, error) {
	var row struct {
		Max *int
	}
	if err := s.db.WithContext(ctx).
		Model(&store.SmartCollection{}).
		Select("MAX(sort) AS max").
		Scan(&row).Error; err != nil {
		return 0, err
	}
	if row.Max == nil {
		return 0, nil
	}
	return *row.Max + 1, nil
}

func (s *SmartCollectionService) uniqueName(ctx context.Context, base string) (string, error) {
	name := base
	for i := 2; ; i++ {
		var count int64
		if err := s.db.WithContext(ctx).Model(&store.SmartCollection{}).
			Where("name = ?", name).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return name, nil
		}
		name = fmt.Sprintf("%s %d", base, i)
	}
}
