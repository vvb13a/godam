package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/store"
	"gorm.io/gorm"
)

// SmartCollectionService manages flat, library-scoped saved filter presets.
type SmartCollectionService struct {
	db *gorm.DB
}

func NewSmartCollectionService(db *gorm.DB) *SmartCollectionService {
	return &SmartCollectionService{db: db}
}

func (s *SmartCollectionService) List(ctx context.Context, libraryID string) ([]domain.SmartCollection, error) {
	var models []store.SmartCollection
	if err := s.db.WithContext(ctx).
		Where("library_id = ?", libraryID).
		Order("sort ASC, name ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}
	collections := make([]domain.SmartCollection, 0, len(models))
	for i := range models {
		collections = append(collections, *models[i].ToDomain())
	}
	return collections, nil
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
func (s *SmartCollectionService) Create(ctx context.Context, libraryID, name, color string, filter domain.SmartCollectionFilter) (*domain.SmartCollection, error) {
	if err := ensureLibraryExists(ctx, s.db, libraryID); err != nil {
		return nil, err
	}
	unique, err := s.uniqueName(ctx, libraryID, name)
	if err != nil {
		return nil, err
	}
	sort, err := s.nextSort(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	collection := &domain.SmartCollection{
		ID:        uuid.New().String(),
		LibraryID: libraryID,
		Name:      unique,
		Color:     color,
		Sort:      sort,
		Filter:    filter,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(store.SmartCollectionModel(collection)).Error; err != nil {
		return nil, err
	}
	return collection, nil
}

func (s *SmartCollectionService) Update(ctx context.Context, id, name, color string) (*domain.SmartCollection, error) {
	res := s.db.WithContext(ctx).
		Model(&store.SmartCollection{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"name":       name,
			"color":      color,
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

func (s *SmartCollectionService) nextSort(ctx context.Context, libraryID string) (int, error) {
	var row struct {
		Max *int
	}
	if err := s.db.WithContext(ctx).
		Model(&store.SmartCollection{}).
		Where("library_id = ?", libraryID).
		Select("MAX(sort) AS max").
		Scan(&row).Error; err != nil {
		return 0, err
	}
	if row.Max == nil {
		return 0, nil
	}
	return *row.Max + 1, nil
}

func (s *SmartCollectionService) uniqueName(ctx context.Context, libraryID, base string) (string, error) {
	name := base
	for i := 2; ; i++ {
		var count int64
		if err := s.db.WithContext(ctx).Model(&store.SmartCollection{}).
			Where("library_id = ? AND name = ?", libraryID, name).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return name, nil
		}
		name = fmt.Sprintf("%s %d", base, i)
	}
}
