package service

import (
	"context"

	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/store"
	"gorm.io/gorm"
)

// ensureLibraryExists verifies a library exists before persisting scoped rows,
// so an unknown library yields ErrNotFound (404) instead of a foreign key
// error (500). An empty id is treated as not found.
func ensureLibraryExists(ctx context.Context, db *gorm.DB, libraryID string) error {
	if libraryID == "" {
		return domain.ErrNotFound
	}
	var count int64
	if err := db.WithContext(ctx).
		Model(&store.Library{}).
		Where("id = ?", libraryID).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return domain.ErrNotFound
	}
	return nil
}
