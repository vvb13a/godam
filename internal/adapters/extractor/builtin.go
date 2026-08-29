package extractor

import (
	"context"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"strings"

	"github.com/vvb13a/godam/internal/domain"
)

type BuiltinExtractor struct{}

func NewBuiltinExtractor() *BuiltinExtractor {
	return &BuiltinExtractor{}
}

func (e *BuiltinExtractor) Extract(_ context.Context, mimeType string, r io.ReaderAt, size int64) (domain.Metadata, error) {
	meta := domain.Metadata{}

	// If image, decode config for width & height
	if strings.HasPrefix(mimeType, "image/") {
		readSeeker := io.NewSectionReader(r, 0, size)
		cfg, _, err := image.DecodeConfig(readSeeker)
		if err == nil {
			meta.Width = &cfg.Width
			meta.Height = &cfg.Height
		}
	}

	return meta, nil
}
