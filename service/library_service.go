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
// reorder is a single request and never leaves the order half-applied. Only
// libraries the user is a member of are affected.
func (s *LibraryService) Reorder(ctx context.Context, userID string, items []LibrarySort) error {
	if len(items) == 0 {
		return nil
	}
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			res := tx.Exec(
				`UPDATE libraries SET sort = ?, updated_at = ?
				 WHERE id = ? AND EXISTS (
					SELECT 1 FROM library_users
					WHERE library_users.library_id = libraries.id AND library_users.user_id = ?
				 )`,
				item.Sort, now, item.ID, userID,
			)
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

// List returns the libraries the user is a member of, ordered by sort
// (ascending) so the first element is the one loaded on start.
func (s *LibraryService) List(ctx context.Context, userID string) ([]domain.Library, error) {
	var models []store.Library
	if err := s.db.WithContext(ctx).
		Select("libraries.*").
		Joins("JOIN library_users ON library_users.library_id = libraries.id").
		Where("library_users.user_id = ?", userID).
		Order("libraries.sort ASC, libraries.created_at ASC").
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

// VerifyOwned returns ErrNotFound unless the user is a member of the library,
// so callers never leak another account's data.
func (s *LibraryService) VerifyOwned(ctx context.Context, libraryID, userID string) error {
	if libraryID == "" {
		return domain.ErrNotFound
	}
	var count int64
	if err := s.db.WithContext(ctx).
		Table("library_users").
		Where("library_id = ? AND user_id = ?", libraryID, userID).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Create inserts a library and enrols the creator as its first member.
func (s *LibraryService) Create(ctx context.Context, userID, name, color string, sort int) (*domain.Library, error) {
	now := time.Now().UTC()
	lib := &domain.Library{
		ID:        uuid.New().String(),
		Name:      name,
		Color:     color,
		Sort:      sort,
		CreatedAt: now,
		UpdatedAt: now,
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(store.LibraryModel(lib)).Error; err != nil {
			return err
		}
		if err := tx.Exec(
			"INSERT INTO library_users (library_id, user_id) VALUES (?, ?)",
			lib.ID, userID,
		).Error; err != nil {
			return err
		}
		return seedDefaultSmartCollections(tx, lib.ID, now)
	})
	if err != nil {
		return nil, err
	}
	return lib, nil
}

func (s *LibraryService) Update(ctx context.Context, id, userID, name, color string, sort int) (*domain.Library, error) {
	if err := s.VerifyOwned(ctx, id, userID); err != nil {
		return nil, err
	}
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
	return s.GetByID(ctx, id)
}

// Delete removes a library the user is a member of and everything scoped to
// it. Asset files are deleted first (the rows cascade away with the library).
func (s *LibraryService) Delete(ctx context.Context, id, userID string) error {
	if err := s.VerifyOwned(ctx, id, userID); err != nil {
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
