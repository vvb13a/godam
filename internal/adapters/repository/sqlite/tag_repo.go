package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/vvb13a/godam/internal/domain"
)

type TagRepo struct {
	db *sql.DB
}

func NewTagRepo(db *sql.DB) *TagRepo {
	return &TagRepo{db: db}
}

func (r *TagRepo) GetOrCreate(ctx context.Context, name string) (*domain.Tag, error) {
	var tag domain.Tag
	query := `SELECT id, name, created_at FROM tags WHERE name = ?`
	err := r.db.QueryRowContext(ctx, query, name).Scan(&tag.ID, &tag.Name, &tag.CreatedAt)
	if err == nil {
		return &tag, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// Create if not exists
	tag = domain.Tag{
		ID:        uuid.New().String(),
		Name:      name,
		CreatedAt: time.Now().UTC(),
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO tags (id, name, created_at) VALUES (?, ?, ?)`, tag.ID, tag.Name, tag.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

func (r *TagRepo) List(ctx context.Context) ([]domain.Tag, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, created_at FROM tags ORDER BY name ASC`)
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

func (r *TagRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tags WHERE id = ?`, id)
	return err
}
