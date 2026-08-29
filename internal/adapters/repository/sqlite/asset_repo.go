package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/vvb13a/godam/internal/domain"
	_ "modernc.org/sqlite"
)

type AssetRepo struct {
	db *sql.DB
}

func NewAssetRepo(db *sql.DB) *AssetRepo {
	return &AssetRepo{db: db}
}

func (r *AssetRepo) Save(ctx context.Context, a *domain.Asset) error {
	query := `INSERT INTO assets (
		id, filename, storage_driver, storage_key, mime_type, byte_size, 
		width, height, duration_sec, page_count, raw_metadata, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		a.ID, a.Filename, a.StorageDriver, a.StorageKey, a.MimeType, a.ByteSize,
		a.Metadata.Width, a.Metadata.Height, a.Metadata.DurationSec, a.Metadata.PageCount,
		a.Metadata.RawJSON, a.CreatedAt,
	)
	return err
}

func (r *AssetRepo) GetByID(ctx context.Context, id string) (*domain.Asset, error) {
	query := `SELECT id, filename, storage_driver, storage_key, mime_type, byte_size, 
	                 width, height, duration_sec, page_count, raw_metadata, created_at 
	          FROM assets WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)

	var a domain.Asset
	var width, height, pageCount sql.NullInt64
	var duration sql.NullFloat64
	var rawMeta sql.NullString

	if err := row.Scan(
		&a.ID, &a.Filename, &a.StorageDriver, &a.StorageKey, &a.MimeType, &a.ByteSize,
		&width, &height, &duration, &pageCount, &rawMeta, &a.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	if width.Valid {
		w := int(width.Int64)
		a.Metadata.Width = &w
	}
	if height.Valid {
		h := int(height.Int64)
		a.Metadata.Height = &h
	}
	if pageCount.Valid {
		p := int(pageCount.Int64)
		a.Metadata.PageCount = &p
	}
	if duration.Valid {
		a.Metadata.DurationSec = &duration.Float64
	}
	if rawMeta.Valid {
		a.Metadata.RawJSON = rawMeta.String
	}

	return &a, nil
}

func (r *AssetRepo) List(ctx context.Context, collectionID *string) ([]domain.Asset, error) {
	var rows *sql.Rows
	var err error

	if collectionID != nil {
		query := `
			SELECT a.id, a.filename, a.storage_driver, a.storage_key, a.mime_type, a.byte_size, 
			       a.width, a.height, a.duration_sec, a.page_count, a.raw_metadata, a.created_at
			FROM assets a
			INNER JOIN asset_collections ac ON a.id = ac.asset_id
			WHERE ac.collection_id = ?
			ORDER BY a.created_at DESC`
		rows, err = r.db.QueryContext(ctx, query, *collectionID)
	} else {
		query := `SELECT id, filename, storage_driver, storage_key, mime_type, byte_size, 
		                 width, height, duration_sec, page_count, raw_metadata, created_at 
		          FROM assets ORDER BY created_at DESC`
		rows, err = r.db.QueryContext(ctx, query)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assets []domain.Asset
	for rows.Next() {
		var a domain.Asset
		var width, height, pageCount sql.NullInt64
		var duration sql.NullFloat64
		var rawMeta sql.NullString

		if err := rows.Scan(
			&a.ID, &a.Filename, &a.StorageDriver, &a.StorageKey, &a.MimeType, &a.ByteSize,
			&width, &height, &duration, &pageCount, &rawMeta, &a.CreatedAt,
		); err != nil {
			return nil, err
		}

		if width.Valid {
			w := int(width.Int64)
			a.Metadata.Width = &w
		}
		if height.Valid {
			h := int(height.Int64)
			a.Metadata.Height = &h
		}
		if pageCount.Valid {
			p := int(pageCount.Int64)
			a.Metadata.PageCount = &p
		}
		if duration.Valid {
			a.Metadata.DurationSec = &duration.Float64
		}
		if rawMeta.Valid {
			a.Metadata.RawJSON = rawMeta.String
		}

		assets = append(assets, a)
	}
	return assets, nil
}

func (r *AssetRepo) AddToCollection(ctx context.Context, assetID, collectionID string) error {
	query := `INSERT OR IGNORE INTO asset_collections (asset_id, collection_id) VALUES (?, ?)`
	_, err := r.db.ExecContext(ctx, query, assetID, collectionID)
	return err
}

func (r *AssetRepo) RemoveFromCollection(ctx context.Context, assetID, collectionID string) error {
	query := `DELETE FROM asset_collections WHERE asset_id = ? AND collection_id = ?`
	_, err := r.db.ExecContext(ctx, query, assetID, collectionID)
	return err
}

func (r *AssetRepo) AddTag(ctx context.Context, assetID, tagID string) error {
	query := `INSERT OR IGNORE INTO asset_tags (asset_id, tag_id) VALUES (?, ?)`
	_, err := r.db.ExecContext(ctx, query, assetID, tagID)
	return err
}

func (r *AssetRepo) RemoveTag(ctx context.Context, assetID, tagID string) error {
	query := `DELETE FROM asset_tags WHERE asset_id = ? AND tag_id = ?`
	_, err := r.db.ExecContext(ctx, query, assetID, tagID)
	return err
}

func (r *AssetRepo) GetTagsForAsset(ctx context.Context, assetID string) ([]domain.Tag, error) {
	query := `
		SELECT t.id, t.name, t.created_at
		FROM tags t
		INNER JOIN asset_tags at ON t.id = at.tag_id
		WHERE at.asset_id = ?
		ORDER BY t.name ASC`
	rows, err := r.db.QueryContext(ctx, query, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []domain.Tag
	for rows.Next() {
		var t domain.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, nil
}

func (r *AssetRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM assets WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}
