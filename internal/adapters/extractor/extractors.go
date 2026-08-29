package extractor

import (
	"context"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/vvb13a/godam/internal/domain"
	"github.com/vvb13a/godam/internal/ports"
	_ "golang.org/x/image/webp" // WebP decoder support
)

// CompositeExtractor aggregates multiple format-specific extractors
type CompositeExtractor struct {
	extractors []ports.FormatExtractor
}

func NewCompositeExtractor() *CompositeExtractor {
	return &CompositeExtractor{
		extractors: []ports.FormatExtractor{
			&ImageExtractor{},
			&PDFExtractor{},
		},
	}
}

func (c *CompositeExtractor) Extract(ctx context.Context, mimeType string, r io.ReaderAt, size int64) (domain.Metadata, error) {
	for _, ext := range c.extractors {
		if ext.Supports(mimeType) {
			return ext.Extract(ctx, r, size)
		}
	}
	return domain.Metadata{}, nil
}

// -------------------------------------------------------------
// 1. Image Extractor (JPEG, PNG, GIF, WebP)
// -------------------------------------------------------------
type ImageExtractor struct{}

func (e *ImageExtractor) Supports(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}

func (e *ImageExtractor) Extract(_ context.Context, r io.ReaderAt, size int64) (domain.Metadata, error) {
	meta := domain.Metadata{}
	readSeeker := io.NewSectionReader(r, 0, size)
	cfg, _, err := image.DecodeConfig(readSeeker)
	if err == nil && cfg.Width > 0 && cfg.Height > 0 {
		meta.Width = &cfg.Width
		meta.Height = &cfg.Height
	}
	return meta, nil
}

// -------------------------------------------------------------
// 2. Pure Go PDF Extractor (Extracts Page Count)
// -------------------------------------------------------------
type PDFExtractor struct{}

func (e *PDFExtractor) Supports(mimeType string) bool {
	return mimeType == "application/pdf"
}

var pdfPagesRegex = regexp.MustCompile(`/Type\s*/Pages[^>]*?/Count\s+(\d+)`)
var pdfPageRegex = regexp.MustCompile(`/Type\s*/Page\b`)

func (e *PDFExtractor) Extract(_ context.Context, r io.ReaderAt, size int64) (domain.Metadata, error) {
	meta := domain.Metadata{}
	buf := make([]byte, size)
	if _, err := r.ReadAt(buf, 0); err != nil && err != io.EOF {
		return meta, err
	}

	// 1. Attempt reading /Pages /Count
	matches := pdfPagesRegex.FindAllSubmatch(buf, -1)
	if len(matches) > 0 {
		// The last /Pages /Count match in a PDF is usually the root catalog count
		lastMatch := matches[len(matches)-1]
		if count, err := strconv.Atoi(string(lastMatch[1])); err == nil && count > 0 {
			meta.PageCount = &count
			return meta, nil
		}
	}

	// 2. Fallback: Count /Type /Page objects
	pageMatches := pdfPageRegex.FindAll(buf, -1)
	if len(pageMatches) > 0 {
		count := len(pageMatches)
		meta.PageCount = &count
	}

	return meta, nil
}
