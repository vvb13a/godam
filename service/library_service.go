package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/store"
	"gorm.io/gorm"
)

// LibraryService manages the top-level Library scopes. Deleting a library
// cascades to every asset, tag, tag group and collection it owns, including
// the stored asset files.
type LibraryService struct {
	db     *gorm.DB
	assets *AssetService
}

func NewLibraryService(db *gorm.DB, assets *AssetService) *LibraryService {
	return &LibraryService{db: db, assets: assets}
}

// LibrarySort is a single library's new position in the sort order.
type LibrarySort struct {
	ID   string `json:"id"`
	Sort int    `json:"sort"`
}

// Reorder applies a batch of sort positions in one transaction so a drag
// reorder is a single request and never leaves the order half-applied.
func (s *LibraryService) Reorder(ctx context.Context, items []LibrarySort) error {
	if len(items) == 0 {
		return nil
	}
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			res := tx.Model(&store.Library{}).
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

// List returns every library ordered by sort (ascending) so the first element
// is the one loaded on start.
func (s *LibraryService) List(ctx context.Context) ([]domain.Library, error) {
	var models []store.Library
	if err := s.db.WithContext(ctx).
		Order("sort ASC, created_at ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}
	libraries := make([]domain.Library, 0, len(models))
	for i := range models {
		libraries = append(libraries, *models[i].ToDomain())
	}
	return libraries, nil
}

func (s *LibraryService) GetByID(ctx context.Context, id string) (*domain.Library, error) {
	var m store.Library
	err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return m.ToDomain(), nil
}

func (s *LibraryService) Create(ctx context.Context, name, color string, sort int) (*domain.Library, error) {
	now := time.Now().UTC()
	lib := &domain.Library{
		ID:        uuid.New().String(),
		Name:      name,
		Color:     color,
		Sort:      sort,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(store.LibraryModel(lib)).Error; err != nil {
		return nil, err
	}
	return lib, nil
}

func (s *LibraryService) Update(ctx context.Context, id, name, color string, sort int) (*domain.Library, error) {
	res := s.db.WithContext(ctx).
		Model(&store.Library{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"name":       name,
			"color":      color,
			"sort":       sort,
			"updated_at": time.Now().UTC(),
		})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, domain.ErrNotFound
	}
	return s.GetByID(ctx, id)
}

// Delete removes a library and everything scoped to it. Asset files are
// deleted first (the rows cascade away with the library).
func (s *LibraryService) Delete(ctx context.Context, id string) error {	if _, err := s.GetByID(ctx, id); err != nil {
		return err
	}
	if err := s.assets.DeleteByLibrary(ctx, id); err != nil {
		return err
	}
	res := s.db.WithContext(ctx).Delete(&store.Library{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
