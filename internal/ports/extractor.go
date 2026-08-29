package ports

import (
	"context"
	"io"

	"github.com/vvb13a/godam/internal/domain"
)

type MetadataExtractor interface {
	Extract(ctx context.Context, mimeType string, r io.ReaderAt, size int64) (domain.Metadata, error)
}

type FormatExtractor interface {
	Supports(mimeType string) bool
	Extract(ctx context.Context, r io.ReaderAt, size int64) (domain.Metadata, error)
}
