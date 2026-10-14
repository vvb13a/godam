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

// Create inserts a folder, optionally under a parent, appended after its
// siblings. Names are made unique within the library.
func (s *CollectionService) Create(ctx context.Context, libraryID string, parentID *string, name, color string) (*domain.Collection, error) {
	if err := ensureLibraryExists(ctx, s.db, libraryID); err != nil {
		return nil, err
	}
	if err := s.ensureValidParent(ctx, libraryID, "", parentID); err != nil {
		return nil, err
	}
	unique, err := s.uniqueName(ctx, libraryID, name)
	if err != nil {
		return nil, err
	}
	sort, err := s.nextSort(ctx, libraryID, parentID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	col := &domain.Collection{
		ID:        uuid.New().String(),
		LibraryID: libraryID,
		ParentID:  parentID,
		Name:      unique,
		Color:     color,
		Sort:      sort,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(store.CollectionModel(col)).Error; err != nil {
		return nil, err
	}
	return col, nil
}

func (s *CollectionService) Update(ctx context.Context, id, name, color string) (*domain.Collection, error) {
	res := s.db.WithContext(ctx).
		Model(&store.Collection{}).
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

func (s *CollectionService) List(ctx context.Context, libraryID string) ([]domain.Collection, error) {
	var models []store.Collection
	if err := s.db.WithContext(ctx).
		Where("library_id = ?", libraryID).
		Order("sort ASC, name ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}

	// Count the assets of every collection in one grouped query and attach the
	// totals, so the tree can show badges without a request each.
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

// Move reparents a collection (parentID may be nil for a root) and reorders it
// among its new siblings at the given position. It rejects cycles and foreign
// parents.
func (s *CollectionService) Move(ctx context.Context, id string, parentID *string, position int) error {
	col, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.ensureValidParent(ctx, col.LibraryID, id, parentID); err != nil {
		return err
	}
	if position < 0 {
		position = 0
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&store.Collection{}).
			Where("id = ?", id).
			Updates(map[string]any{"parent_id": parentID, "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}

		var siblings []store.Collection
		q := tx.Where("library_id = ? AND id <> ?", col.LibraryID, id)
		if parentID == nil {
			q = q.Where("parent_id IS NULL")
		} else {
			q = q.Where("parent_id = ?", *parentID)
		}
		if err := q.Order("sort ASC, name ASC").Find(&siblings).Error; err != nil {
			return err
		}
		if position > len(siblings) {
			position = len(siblings)
		}
		order := make([]string, 0, len(siblings)+1)
		for i, sibling := range siblings {
			if i == position {
				order = append(order, id)
			}
			order = append(order, sibling.ID)
		}
		if position >= len(siblings) {
			order = append(order, id)
		}
		for i, siblingID := range order {
			if err := tx.Model(&store.Collection{}).
				Where("id = ?", siblingID).
				Update("sort", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ensureValidParent checks the parent exists in the library and that id is not
// an ancestor of it (which would create a cycle). id is "" when creating.
func (s *CollectionService) ensureValidParent(ctx context.Context, libraryID, id string, parentID *string) error {
	if parentID == nil {
		return nil
	}
	if id != "" && *parentID == id {
		return ErrInvalidInput
	}
	var parent store.Collection
	err := s.db.WithContext(ctx).First(&parent, "id = ?", *parentID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if parent.LibraryID != libraryID {
		return domain.ErrNotFound
	}
	if id == "" {
		return nil
	}
	for cur := parent.ParentID; cur != nil; {
		if *cur == id {
			return ErrInvalidInput
		}
		var ancestor store.Collection
		if err := s.db.WithContext(ctx).Select("parent_id").First(&ancestor, "id = ?", *cur).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				break
			}
			return err
		}
		cur = ancestor.ParentID
	}
	return nil
}

// nextSort returns one past the highest sibling position.
func (s *CollectionService) nextSort(ctx context.Context, libraryID string, parentID *string) (int, error) {
	var row struct {
		Max *int
	}
	q := s.db.WithContext(ctx).Model(&store.Collection{}).Where("library_id = ?", libraryID)
	if parentID == nil {
		q = q.Where("parent_id IS NULL")
	} else {
		q = q.Where("parent_id = ?", *parentID)
	}
	if err := q.Select("MAX(sort) AS max").Scan(&row).Error; err != nil {
		return 0, err
	}
	if row.Max == nil {
		return 0, nil
	}
	return *row.Max + 1, nil
}

// uniqueName appends an incrementing suffix until the name is free within the
// library.
func (s *CollectionService) uniqueName(ctx context.Context, libraryID, base string) (string, error) {
	name := base
	for i := 2; ; i++ {
		var count int64
		if err := s.db.WithContext(ctx).Model(&store.Collection{}).
			Where("library_id = ? AND name = ?", libraryID, name).Count(&count).Error; err != nil {
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
