package ports

import (
	"context"
	"errors"
	"fmt"
	"io"
)

var ErrDriverNotFound = errors.New("storage driver not found")

type StorageDriver interface {
	Save(ctx context.Context, key string, r io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

type StorageRegistry struct {
	drivers       map[string]StorageDriver
	defaultDriver string
}

func NewStorageRegistry(defaultDriver string) *StorageRegistry {
	return &StorageRegistry{
		drivers:       make(map[string]StorageDriver),
		defaultDriver: defaultDriver,
	}
}

func (r *StorageRegistry) Register(name string, driver StorageDriver) {
	r.drivers[name] = driver
}

func (r *StorageRegistry) Get(name string) (StorageDriver, error) {
	if name == "" {
		name = r.defaultDriver
	}
	driver, ok := r.drivers[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrDriverNotFound, name)
	}
	return driver, nil
}
