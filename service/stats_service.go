package service

import (
	"context"
	"fmt"

	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/store"
	"gorm.io/gorm"
)

// StatsService aggregates library-wide numbers for the dashboard.
type StatsService struct {
	db *gorm.DB
}

func NewStatsService(db *gorm.DB) *StatsService {
	return &StatsService{db: db}
}

func (s *StatsService) Get(ctx context.Context) (*domain.Stats, error) {
	db := s.db.WithContext(ctx)

	var assets, collections, tags int64
	if err := db.Model(&store.Asset{}).Count(&assets).Error; err != nil {
		return nil, fmt.Errorf("count assets: %w", err)
	}
	if err := db.Model(&store.Collection{}).Count(&collections).Error; err != nil {
		return nil, fmt.Errorf("count collections: %w", err)
	}
	if err := db.Model(&store.Tag{}).Count(&tags).Error; err != nil {
		return nil, fmt.Errorf("count tags: %w", err)
	}

	var uncategorized int64
	if err := db.Model(&store.Asset{}).
		Where("id NOT IN (SELECT asset_id FROM asset_collections)").
		Count(&uncategorized).Error; err != nil {
		return nil, fmt.Errorf("count uncategorized: %w", err)
	}

	var untagged int64
	if err := db.Model(&store.Asset{}).
		Where("id NOT IN (SELECT asset_id FROM asset_tags)").
		Count(&untagged).Error; err != nil {
		return nil, fmt.Errorf("count untagged: %w", err)
	}

	var storageBytes int64
	row := db.Model(&store.Asset{}).
		Select("COALESCE(SUM(byte_size), 0)").Row()
	if err := row.Scan(&storageBytes); err != nil {
		return nil, fmt.Errorf("sum storage: %w", err)
	}

	var rows []struct {
		MimeType string
		Count    int64
		Bytes    int64
	}
	if err := db.Model(&store.Asset{}).
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

	return &domain.Stats{
		Assets:        int(assets),
		Collections:   int(collections),
		Tags:          int(tags),
		Uncategorized: int(uncategorized),
		Untagged:      int(untagged),
		StorageBytes:  storageBytes,
		Types:         types,
	}, nil
}
