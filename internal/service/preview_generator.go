package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"os/exec"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

type DefaultPreviewGenerator struct {
	maxDimension int
	quality      int
}

func NewPreviewGenerator(maxDim, quality int) *DefaultPreviewGenerator {
	if maxDim <= 0 {
		maxDim = 600
	}
	if quality <= 0 {
		quality = 85
	}
	return &DefaultPreviewGenerator{
		maxDimension: maxDim,
		quality:      quality,
	}
}

// Generate creates a flattened, scaled JPEG preview
func (g *DefaultPreviewGenerator) Generate(ctx context.Context, mimeType string, r io.Reader) (io.Reader, error) {
	// 1. Standard Images (JPEG, PNG, WebP, GIF)
	if strings.HasPrefix(mimeType, "image/") {
		src, _, err := image.Decode(r)
		if err != nil {
			return nil, fmt.Errorf("image decode failed: %w", err)
		}
		return g.processAndEncodeJPEG(src)
	}

	// 2. PDF First Page (via pdftoppm if available)
	if mimeType == "application/pdf" {
		ppmReader, err := g.extractPDFPage(ctx, r)
		if err == nil {
			src, _, err := image.Decode(ppmReader)
			if err == nil {
				return g.processAndEncodeJPEG(src)
			}
		}
		// If pdftoppm is not installed, return the Document fallback
		return g.GetFallback(mimeType), nil
	}

	// 3. Video First Frame (via ffmpeg if available)
	if strings.HasPrefix(mimeType, "video/") {
		frameReader, err := g.extractVideoFrame(ctx, r)
		if err == nil {
			src, _, err := image.Decode(frameReader)
			if err == nil {
				return g.processAndEncodeJPEG(src)
			}
		}
		return g.GetFallback(mimeType), nil
	}

	// 4. Non-visual assets (Audio, Zip, Text) -> Fallback
	return g.GetFallback(mimeType), nil
}

// processAndEncodeJPEG downscales and flattens alpha onto a white background
func (g *DefaultPreviewGenerator) processAndEncodeJPEG(src image.Image) (io.Reader, error) {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	targetW, targetH := srcW, srcH

	// Calculate bounds preserving aspect ratio
	if srcW > g.maxDimension || srcH > g.maxDimension {
		if srcW > srcH {
			targetW = g.maxDimension
			targetH = (g.maxDimension * srcH) / srcW
		} else {
			targetH = g.maxDimension
			targetW = (g.maxDimension * srcW) / srcH
		}
	}

	if targetW < 1 {
		targetW = 1
	}
	if targetH < 1 {
		targetH = 1
	}

	// 1. Create canvas with pure white background (prevents black background on transparent PNGs/WebPs)
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)

	// 2. High-quality Bilinear Downscaling
	draw.BiLinear.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)

	// 3. Encode to JPEG
	buf := new(bytes.Buffer)
	err := jpeg.Encode(buf, dst, &jpeg.Options{Quality: g.quality})
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func (g *DefaultPreviewGenerator) extractPDFPage(ctx context.Context, r io.Reader) (io.Reader, error) {
	cmd := exec.CommandContext(ctx, "pdftoppm", "-png", "-f", "1", "-l", "1", "-scale-to", fmt.Sprintf("%d", g.maxDimension), "-")
	cmd.Stdin = r
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return &out, nil
}

func (g *DefaultPreviewGenerator) extractVideoFrame(ctx context.Context, r io.Reader) (io.Reader, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg", "-ss", "00:00:01", "-i", "pipe:0", "-vframes", "1", "-vf", fmt.Sprintf("scale='min(%d,iw)':-1", g.maxDimension), "-f", "image2", "pipe:1")
	cmd.Stdin = r
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetFallback generates a clean placeholder image for audio, zip, code, etc.
func (g *DefaultPreviewGenerator) GetFallback(mimeType string) io.Reader {
	// Create a simple 400x300 canvas with neutral gray background
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	bgColor := color.RGBA{R: 240, G: 242, B: 245, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80})
	return &buf
}
