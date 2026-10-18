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

// GetOrCreate trims the name and returns the matching tag within the library,
// inserting it when it does not exist yet. The name is preserved as the user
// typed it; names are unique per library.
func (s *TagService) GetOrCreate(ctx context.Context, name string) (*domain.Tag, error) {
	cleaned := strings.TrimSpace(name)
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
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.db.WithContext(ctx).Create(&tag).Error; err != nil {
		return nil, err
	}
	return tag.ToDomain(), nil
}

func (s *TagService) List(ctx context.Context) ([]domain.Tag, error) {
	var models []store.Tag
	if err := s.db.WithContext(ctx).
		Order("name ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}

	type countRow struct {
		TagID string
		Count int64
	}
	var rows []countRow
	if err := s.db.WithContext(ctx).
		Table("asset_tags").
		Select("tag_id, COUNT(*) AS count").
		Group("tag_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[string]int, len(rows))
	for _, r := range rows {
		counts[r.TagID] = int(r.Count)
	}

	tags := make([]domain.Tag, 0, len(models))
	for i := range models {
		tag := models[i].ToDomain()
		tag.AssetCount = counts[tag.ID]
		tags = append(tags, *tag)
	}
	return tags, nil
}

func (s *TagService) GetByID(ctx context.Context, id string) (*domain.Tag, error) {
	var m store.Tag
	err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	tag := m.ToDomain()
	var count int64
	if err := s.db.WithContext(ctx).
		Table("asset_tags").
		Where("tag_id = ?", id).
		Count(&count).Error; err != nil {
		return nil, err
	}
	tag.AssetCount = int(count)
	return tag, nil
}

// Update sets a tag's name, description, group (nil for uncategorized) and
// starred flag. Callers send the complete desired state. The name is trimmed
// the same way as on creation, preserving the casing the user chose.
func (s *TagService) Update(ctx context.Context, id, name, description string, groupID *string, starred bool) (*domain.Tag, error) {
	cleaned := strings.TrimSpace(name)
	if cleaned == "" {
		return nil, domain.ErrNotFound
	}
	var group any
	if groupID != nil {
		group = *groupID
	}
	if err := s.db.WithContext(ctx).
		Model(&store.Tag{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"name":        cleaned,
			"description": strings.TrimSpace(description),
			"group_id":    group,
			"starred":     starred,
			"updated_at":  time.Now().UTC(),
		}).Error; err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
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
