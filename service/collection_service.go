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

type CollectionService struct {
	db *gorm.DB
}

func NewCollectionService(db *gorm.DB) *CollectionService {
	return &CollectionService{db: db}
}

func (s *CollectionService) Create(ctx context.Context, name, description, icon string) (*domain.Collection, error) {
	now := time.Now().UTC()
	col := &domain.Collection{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Icon:        icon,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.db.WithContext(ctx).Create(store.CollectionModel(col)).Error; err != nil {
		return nil, err
	}
	return col, nil
}

func (s *CollectionService) Update(ctx context.Context, id, name, description, icon string) (*domain.Collection, error) {
	res := s.db.WithContext(ctx).
		Model(&store.Collection{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"name":        name,
			"description": description,
			"icon":        icon,
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

func (s *CollectionService) GetByID(ctx context.Context, id string) (*domain.Collection, error) {
	var m store.Collection
	err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return m.ToDomain(), nil
}

func (s *CollectionService) List(ctx context.Context) ([]domain.Collection, error) {
	var models []store.Collection
	if err := s.db.WithContext(ctx).Order("name ASC").Find(&models).Error; err != nil {
		return nil, err
	}

	// Count the assets of every collection in one grouped query and attach the
	// totals, so the collections panel can show badges without a request each.
	var rows []struct {
		CollectionID string
		Count        int64
	}
	if err := s.db.WithContext(ctx).
		Table("asset_collections").
		Select("collection_id, COUNT(*) AS count").
		Group("collection_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[string]int, len(rows))
	for _, row := range rows {
		counts[row.CollectionID] = int(row.Count)
	}

	collections := make([]domain.Collection, 0, len(models))
	for i := range models {
		collection := models[i].ToDomain()
		collection.AssetCount = counts[collection.ID]
		collections = append(collections, *collection)
	}
	return collections, nil
}

// Duplicate clones a collection (name, description, icon) under a unique name
// and copies its asset membership, so the clone starts with the same assets.
func (s *CollectionService) Duplicate(ctx context.Context, id string) (*domain.Collection, error) {
	src, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	name, err := s.uniqueName(ctx, src.Name+" copy")
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	dup := &domain.Collection{
		ID:          uuid.New().String(),
		Name:        name,
		Description: src.Description,
		Icon:        src.Icon,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.db.WithContext(ctx).Create(store.CollectionModel(dup)).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Exec(
		"INSERT INTO asset_collections (asset_id, collection_id) SELECT asset_id, ? FROM asset_collections WHERE collection_id = ?",
		dup.ID, id,
	).Error; err != nil {
		return nil, err
	}

	var count int64
	if err := s.db.WithContext(ctx).Table("asset_collections").
		Where("collection_id = ?", dup.ID).Count(&count).Error; err != nil {
		return nil, err
	}
	dup.AssetCount = int(count)
	return dup, nil
}

// uniqueName appends an incrementing suffix until the name is free.
func (s *CollectionService) uniqueName(ctx context.Context, base string) (string, error) {
	name := base
	for i := 2; ; i++ {
		var count int64
		if err := s.db.WithContext(ctx).Model(&store.Collection{}).
			Where("name = ?", name).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return name, nil
		}
		name = fmt.Sprintf("%s %d", base, i)
	}
}

func (s *CollectionService) Delete(ctx context.Context, id string) error {
	res := s.db.WithContext(ctx).Delete(&store.Collection{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
