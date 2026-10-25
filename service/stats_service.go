package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/storage"
	"github.com/vvb13a/godam/store"
	"gorm.io/gorm"
)

// StatsService aggregates library-wide numbers for the dashboard. Conversion
// stats come from the conversion store and are cached briefly because they
// require a filesystem walk.
type StatsService struct {
	db          *gorm.DB
	conversions storage.ConversionStore

	mu        sync.Mutex
	convStats storage.ConversionStats
	convAt    time.Time
}

func NewStatsService(db *gorm.DB, conversions storage.ConversionStore) *StatsService {
	return &StatsService{db: db, conversions: conversions}
}

// Invalidate drops the conversion stats cache after the conversion store
// changes (warm-up or purge).
func (s *StatsService) Invalidate() {
	s.mu.Lock()
	s.convAt = time.Time{}
	s.mu.Unlock()
}

func (s *StatsService) conversionStats(ctx context.Context) (storage.ConversionStats, error) {
	if s.conversions == nil {
		return storage.ConversionStats{}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.convAt.IsZero() && time.Since(s.convAt) < 30*time.Second {
		return s.convStats, nil
	}
	stats, err := s.conversions.Stats(ctx)
	if err != nil {
		return storage.ConversionStats{}, err
	}
	s.convStats = stats
	s.convAt = time.Now()
	return stats, nil
}

func (s *StatsService) Get(ctx context.Context) (*domain.Stats, error) {
	return s.GetScoped(ctx, nil)
}

// GetScoped aggregates only assets matching the enforced attribute set. Locked
// and hidden token attributes should be passed here so tenant counts do not
// include other tenants.
func (s *StatsService) GetScoped(ctx context.Context, attributes map[string][]string) (*domain.Stats, error) {
	db := s.db.WithContext(ctx)

	originals := func() *gorm.DB {
		query := db.Model(&store.Asset{}).Where("role = ?", domain.AssetRoleOriginal)
		for key, values := range attributes {
			if key == "" || len(values) == 0 {
				continue
			}
			query = query.Where(
				`EXISTS (SELECT 1 FROM asset_attributes aa
				 WHERE aa.asset_id = assets.id AND aa.key = ? AND aa.value IN ?)`,
				key, values,
			)
		}
		return query
	}

	var assets, collections, tags int64
	if err := originals().Count(&assets).Error; err != nil {
		return nil, fmt.Errorf("count assets: %w", err)
	}
	if err := db.Model(&store.Collection{}).Count(&collections).Error; err != nil {
		return nil, fmt.Errorf("count collections: %w", err)
	}
	if err := db.Model(&store.Tag{}).Count(&tags).Error; err != nil {
		return nil, fmt.Errorf("count tags: %w", err)
	}

	var uncategorized int64
	if err := originals().
		Where("id NOT IN (SELECT asset_id FROM asset_collections)").
		Count(&uncategorized).Error; err != nil {
		return nil, fmt.Errorf("count uncategorized: %w", err)
	}

	var untagged int64
	if err := originals().
		Where("id NOT IN (SELECT asset_id FROM asset_tags)").
		Count(&untagged).Error; err != nil {
		return nil, fmt.Errorf("count untagged: %w", err)
	}

	var storageBytes int64
	row := originals().
		Select("COALESCE(SUM(byte_size), 0)").Row()
	if err := row.Scan(&storageBytes); err != nil {
		return nil, fmt.Errorf("sum storage: %w", err)
	}

	var rows []struct {
		MimeType string
		Count    int64
		Bytes    int64
	}
	if err := originals().
		Select("mime_type, COUNT(*) AS count, COALESCE(SUM(byte_size), 0) AS bytes").
		Group("mime_type").
		Order("count DESC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("breakdown by type: %w", err)
	}

	types := make([]domain.TypeCount, 0, len(rows))
	for _, r := range rows {
		types = append(types, domain.TypeCount{
			MimeType: r.MimeType,
			Count:    int(r.Count),
			Bytes:    r.Bytes,
		})
	}

	conversionStats, err := s.conversionStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("conversion stats: %w", err)
	}

	return &domain.Stats{
		Assets:          int(assets),
		Collections:     int(collections),
		Tags:            int(tags),
		Uncategorized:   int(uncategorized),
		Untagged:        int(untagged),
		StorageBytes:    storageBytes,
		Conversions:     conversionStats.Count,
		ConversionBytes: conversionStats.Bytes,
		Types:           types,
	}, nil
}
