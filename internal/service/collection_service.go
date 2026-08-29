package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/vvb13a/godam/internal/domain"
	"github.com/vvb13a/godam/internal/ports"
)

type CollectionService struct {
	repo ports.CollectionRepository
}

func NewCollectionService(repo ports.CollectionRepository) *CollectionService {
	return &CollectionService{repo: repo}
}

func (s *CollectionService) Create(ctx context.Context, name, description string) (*domain.Collection, error) {
	col := &domain.Collection{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, col); err != nil {
		return nil, err
	}
	return col, nil
}

func (s *CollectionService) Update(ctx context.Context, id, name, description string) (*domain.Collection, error) {
	col, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	col.Name = name
	col.Description = description
	col.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, col); err != nil {
		return nil, err
	}
	return col, nil
}

func (s *CollectionService) List(ctx context.Context) ([]domain.Collection, error) {
	return s.repo.List(ctx)
}

func (s *CollectionService) GetByID(ctx context.Context, id string) (*domain.Collection, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *CollectionService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
