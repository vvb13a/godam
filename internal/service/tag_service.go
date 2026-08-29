package service

import (
	"context"
	"strings"

	"github.com/vvb13a/godam/internal/domain"
	"github.com/vvb13a/godam/internal/ports"
)

type TagService struct {
	repo ports.TagRepository
}

func NewTagService(repo ports.TagRepository) *TagService {
	return &TagService{repo: repo}
}

func (s *TagService) GetOrCreate(ctx context.Context, name string) (*domain.Tag, error) {
	cleaned := strings.ToLower(strings.TrimSpace(name))
	cleaned = strings.TrimPrefix(cleaned, "#")
	return s.repo.GetOrCreate(ctx, cleaned)
}

func (s *TagService) List(ctx context.Context) ([]domain.Tag, error) {
	return s.repo.List(ctx)
}
