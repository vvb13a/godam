package ports

import (
	"context"
	"io"
)

type PreviewGenerator interface {
	Generate(ctx context.Context, mimeType string, r io.Reader) (io.Reader, error)
	GetFallback(mimeType string) io.Reader
}
