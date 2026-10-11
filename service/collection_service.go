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

type CollectionService struct {
	db *gorm.DB
}

func NewCollectionService(db *gorm.DB) *CollectionService {
	return &CollectionService{db: db}
}

func (s *CollectionService) Create(ctx context.Context, name, description string) (*domain.Collection, error) {
	now := time.Now().UTC()
	col := &domain.Collection{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.db.WithContext(ctx).Create(store.CollectionModel(col)).Error; err != nil {
		return nil, err
	}
	return col, nil
}

func (s *CollectionService) Update(ctx context.Context, id, name, description string) (*domain.Collection, error) {
	res := s.db.WithContext(ctx).
		Model(&store.Collection{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"name":        name,
			"description": description,
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
	collections := make([]domain.Collection, 0, len(models))
	for i := range models {
		collections = append(collections, *models[i].ToDomain())
	}
	return collections, nil
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
