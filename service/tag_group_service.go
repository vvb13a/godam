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

type TagGroupService struct {
	db *gorm.DB
}

func NewTagGroupService(db *gorm.DB) *TagGroupService {
	return &TagGroupService{db: db}
}

func (s *TagGroupService) List(ctx context.Context) ([]domain.TagGroup, error) {
	var models []store.TagGroup
	if err := s.db.WithContext(ctx).Order("rowid ASC").Find(&models).Error; err != nil {
		return nil, err
	}

	var rows []struct {
		GroupID string
		Count   int64
	}
	if err := s.db.WithContext(ctx).
		Table("tags").
		Select("group_id, COUNT(*) AS count").
		Where("group_id IS NOT NULL").
		Group("group_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[string]int, len(rows))
	for _, row := range rows {
		counts[row.GroupID] = int(row.Count)
	}

	groups := make([]domain.TagGroup, 0, len(models))
	for i := range models {
		group := models[i].ToDomain()
		group.TagCount = counts[group.ID]
		groups = append(groups, *group)
	}
	return groups, nil
}

func (s *TagGroupService) GetByID(ctx context.Context, id string) (*domain.TagGroup, error) {
	var m store.TagGroup
	err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return m.ToDomain(), nil
}

func (s *TagGroupService) Create(ctx context.Context, name, description, color string) (*domain.TagGroup, error) {
	now := time.Now().UTC()
	group := &domain.TagGroup{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Color:       color,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.db.WithContext(ctx).Create(store.TagGroupModel(group)).Error; err != nil {
		return nil, err
	}
	return group, nil
}

func (s *TagGroupService) Update(ctx context.Context, id, name, description, color string) (*domain.TagGroup, error) {
	res := s.db.WithContext(ctx).
		Model(&store.TagGroup{}).
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

// Delete removes the group; its tags are kept and become uncategorized.
func (s *TagGroupService) Delete(ctx context.Context, id string) error {
	if err := s.db.WithContext(ctx).
		Model(&store.Tag{}).
		Where("group_id = ?", id).
		Update("group_id", nil).Error; err != nil {
		return err
	}
	res := s.db.WithContext(ctx).Delete(&store.TagGroup{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
