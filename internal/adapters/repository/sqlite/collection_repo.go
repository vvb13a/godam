package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/vvb13a/godam/internal/domain"
)

type CollectionRepo struct {
	db *sql.DB
}

func NewCollectionRepo(db *sql.DB) *CollectionRepo {
	return &CollectionRepo{db: db}
}

func (r *CollectionRepo) Create(ctx context.Context, col *domain.Collection) error {
	query := `INSERT INTO collections (id, name, description, created_at, updated_at)
	          VALUES (?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, col.ID, col.Name, col.Description, col.CreatedAt, col.UpdatedAt)
	return err
}

func (r *CollectionRepo) GetByID(ctx context.Context, id string) (*domain.Collection, error) {
	query := `SELECT id, name, description, created_at, updated_at FROM collections WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)

	var c domain.Collection
	var desc sql.NullString

	if err := row.Scan(&c.ID, &c.Name, &desc, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	if desc.Valid {
		c.Description = desc.String
	}
	return &c, nil
}

func (r *CollectionRepo) List(ctx context.Context) ([]domain.Collection, error) {
	query := `SELECT id, name, description, created_at, updated_at FROM collections ORDER BY name ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var collections []domain.Collection
	for rows.Next() {
		var c domain.Collection
		var desc sql.NullString
		if err := rows.Scan(&c.ID, &c.Name, &desc, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		if desc.Valid {
			c.Description = desc.String
		}
		collections = append(collections, c)
	}
	return collections, nil
}

func (r *CollectionRepo) Update(ctx context.Context, col *domain.Collection) error {
	col.UpdatedAt = time.Now().UTC()
	query := `UPDATE collections SET name = ?, description = ?, updated_at = ? WHERE id = ?`

	res, err := r.db.ExecContext(ctx, query, col.Name, col.Description, col.UpdatedAt, col.ID)
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

func (r *CollectionRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM collections WHERE id = ?`
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
