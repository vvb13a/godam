package ports

import (
	"context"

	"github.com/vvb13a/godam/internal/domain"
)

type TagRepository interface {
	GetOrCreate(ctx context.Context, name string) (*domain.Tag, error)
	List(ctx context.Context) ([]domain.Tag, error)
	Delete(ctx context.Context, id string) error
}

type CollectionRepository interface {
	Create(ctx context.Context, col *domain.Collection) error
	GetByID(ctx context.Context, id string) (*domain.Collection, error)
	List(ctx context.Context) ([]domain.Collection, error)
	Update(ctx context.Context, col *domain.Collection) error
	Delete(ctx context.Context, id string) error
}

type AssetRepository interface {
	Save(ctx context.Context, asset *domain.Asset) error
	GetByID(ctx context.Context, id string) (*domain.Asset, error)
	List(ctx context.Context, collectionID *string) ([]domain.Asset, error)
	AddToCollection(ctx context.Context, assetID, collectionID string) error
	RemoveFromCollection(ctx context.Context, assetID, collectionID string) error
	AddTag(ctx context.Context, assetID, tagID string) error
	RemoveTag(ctx context.Context, assetID, tagID string) error
	GetTagsForAsset(ctx context.Context, assetID string) ([]domain.Tag, error)
	Delete(ctx context.Context, id string) error
}
