package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/store"
	"gorm.io/gorm"
)

type TagService struct {
	db *gorm.DB
}

func NewTagService(db *gorm.DB) *TagService {
	return &TagService{db: db}
}

// GetOrCreate normalizes the name (lowercased, trimmed, no leading #) and
// returns the matching tag, inserting it when it does not exist yet.
func (s *TagService) GetOrCreate(ctx context.Context, name string) (*domain.Tag, error) {
	cleaned := strings.ToLower(strings.TrimSpace(name))
	cleaned = strings.TrimPrefix(cleaned, "#")
	if cleaned == "" {
		return nil, domain.ErrNotFound
	}

	var tag store.Tag
	err := s.db.WithContext(ctx).Where("name = ?", cleaned).First(&tag).Error
	if err == nil {
		return tag.ToDomain(), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	tag = store.Tag{
		ID:        uuid.New().String(),
		Name:      cleaned,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.db.WithContext(ctx).Create(&tag).Error; err != nil {
		return nil, err
	}
	return tag.ToDomain(), nil
}

func (s *TagService) List(ctx context.Context) ([]domain.Tag, error) {
	var models []store.Tag
	if err := s.db.WithContext(ctx).Order("name ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	tags := make([]domain.Tag, 0, len(models))
	for i := range models {
		tags = append(tags, *models[i].ToDomain())
	}
	return tags, nil
}

func (s *TagService) Delete(ctx context.Context, id string) error {
	res := s.db.WithContext(ctx).Delete(&store.Tag{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
