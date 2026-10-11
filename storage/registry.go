package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
)

var ErrDriverNotFound = errors.New("storage driver not found")

// Driver stores and retrieves asset blobs under opaque keys. Implementations
// exist for the local filesystem and SFTP.
type Driver interface {
	Save(ctx context.Context, key string, r io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// Registry resolves a driver by name. An empty name resolves to the default
// driver.
type Registry struct {
	drivers       map[string]Driver
	defaultDriver string
}

func NewRegistry(defaultDriver string) *Registry {
	return &Registry{
		drivers:       make(map[string]Driver),
		defaultDriver: defaultDriver,
	}
}

func (r *Registry) Register(name string, driver Driver) {
	r.drivers[name] = driver
}

func (r *Registry) Get(name string) (Driver, error) {
	if name == "" {
		name = r.defaultDriver
	}
	driver, ok := r.drivers[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrDriverNotFound, name)
	}
	return driver, nil
}
