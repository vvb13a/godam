package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/image/draw"
	gofont "golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	_ "golang.org/x/image/webp"
)

// PreviewGenerator renders a flattened, scaled JPEG preview for an asset,
// shelling out to pdftoppm/ffmpeg for documents and video when available and
// falling back to a neutral placeholder otherwise.
type PreviewGenerator interface {
	Generate(ctx context.Context, mimeType string, r io.Reader) (io.Reader, error)
	GetFallback(mimeType string) io.Reader
}

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

	// 4. Audio Waveform (transparent PNG so a dark-mode card can tint it)
	if strings.HasPrefix(mimeType, "audio/") {
		if reader, err := g.extractAudioWaveform(ctx, r); err == nil {
			return reader, nil
		}
		return g.pngFallback(), nil
	}

	// 5. Font Specimen (draw sample text with the font)
	if isFontMime(mimeType) {
		if reader, err := g.generateFontPreview(r); err == nil {
			return reader, nil
		}
		return g.GetFallback(mimeType), nil
	}

	// 6. Non-visual assets (Zip, Text) -> Fallback
	return g.GetFallback(mimeType), nil
}

// generateFontPreview renders a simple specimen (sample text plus the family
// name) into the standard JPEG preview canvas.
func (g *DefaultPreviewGenerator) generateFontPreview(r io.Reader) (io.Reader, error) {
	data, err := io.ReadAll(io.LimitReader(r, 16<<20))
	if err != nil {
		return nil, err
	}
	font, err := sfnt.Parse(data)
	if err != nil {
		return nil, err
	}
	size := float64(g.maxDimension) / 6
	face, err := opentype.NewFace(font, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: gofont.HintingFull,
	})
	if err != nil {
		return nil, err
	}
	defer face.Close()

	width, height := g.maxDimension, g.maxDimension
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)

	drawer := &gofont.Drawer{Dst: img, Src: image.Black, Face: face}
	drawer.Dot = fixed.P(width/12, height/2)
	drawer.DrawString("Aa Bb Cc")
	if family, err := font.Name(nil, sfnt.NameIDFamily); err == nil && family != "" {
		small, err := opentype.NewFace(font, &opentype.FaceOptions{Size: size / 2, DPI: 72, Hinting: gofont.HintingFull})
		if err == nil {
			defer small.Close()
			sub := &gofont.Drawer{Dst: img, Src: image.Black, Face: small}
			sub.Dot = fixed.P(width/12, height/2+int(size))
			sub.DrawString(truncate(family, 24))
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: g.quality}); err != nil {
		return nil, err
	}
	return &buf, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
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

// tempInput streams a reader into a temporary file and returns its path plus a
// cleanup func. ffmpeg/pdftoppm need a seekable input: containers like MP4/M4A
// keep their index (moov) at the end, which cannot be read from a pipe.
func tempInput(r io.Reader) (string, func(), error) {
	f, err := os.CreateTemp("", "godam-media-*")
	if err != nil {
		return "", nil, err
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", nil, err
	}
	return f.Name(), func() { _ = os.Remove(f.Name()) }, nil
}

func (g *DefaultPreviewGenerator) extractVideoFrame(ctx context.Context, r io.Reader) (io.Reader, error) {
	path, cleanup, err := tempInput(r)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	for _, timestamp := range []string{"1", "0"} {
		cmd := exec.CommandContext(ctx, "ffmpeg",
			"-ss", timestamp,
			"-i", path,
			"-frames:v", "1",
			"-vf", fmt.Sprintf("scale=w=min(%d\\,iw):h=-1", g.maxDimension),
			"-f", "image2pipe",
			"-c:v", "mjpeg",
			"pipe:1",
		)
		out, err := cmd.Output()
		if err == nil && len(out) > 0 {
			return bytes.NewReader(out), nil
		}
	}
	return nil, fmt.Errorf("no video frame extracted")
}

// extractAudioWaveform renders a waveform image via ffmpeg's showwavespic on a
// 2:1 canvas (matching the mosaic landscape cell), then normalizes its vertical
// extent so the wave fills a consistent fraction of the height.
func (g *DefaultPreviewGenerator) extractAudioWaveform(ctx context.Context, r io.Reader) (io.Reader, error) {
	path, cleanup, err := tempInput(r)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	width := g.maxDimension
	height := width / 2
	filter := fmt.Sprintf("showwavespic=s=%dx%d:colors=0xf59e0b", width, height)
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", path,
		"-filter_complex", filter,
		"-frames:v", "1",
		"-f", "image2pipe",
		"-c:v", "png",
		"pipe:1",
	)
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return nil, fmt.Errorf("waveform extraction failed: %w", err)
	}
	src, _, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		return nil, err
	}
	return normalizeWaveform(src, 0.85)
}

// normalizeWaveform scales the drawn wave vertically so its peak reaches
// `fill` of the canvas height, then centers it, so quiet and loud tracks look
// consistent without ever touching the edges.
func normalizeWaveform(src image.Image, fill float64) (io.Reader, error) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	minY, maxY := h, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if _, _, _, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA(); a > 0 {
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
				break
			}
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if maxY >= minY {
		extent := maxY - minY + 1
		target := float64(h) * fill
		scale := target / float64(extent)
		centerY := float64(minY+maxY+1) / 2
		// Scale the whole (transparent) source and shift it so the wave's
		// vertical centre lands on the canvas centre; the overflow is clipped.
		dstH := int(float64(h) * scale)
		dstTop := int(float64(h)/2 - centerY*scale)
		draw.BiLinear.Scale(dst, image.Rect(0, dstTop, w, dstTop+dstH), src, b, draw.Over, nil)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, err
	}
	return &buf, nil
}

// previewExtension is the generated preview's file extension for a MIME type.
// Audio waveforms are transparent PNGs (so a dark-mode card can tint them);
// everything else is a flattened JPEG.
func previewExtension(mimeType string) string {
	if strings.HasPrefix(mimeType, "audio/") {
		return ".png"
	}
	return ".jpg"
}

// pngFallback is a plain light placeholder PNG used for audio when the waveform
// cannot be generated.
func (g *DefaultPreviewGenerator) pngFallback() io.Reader {
	img := image.NewRGBA(image.Rect(0, 0, g.maxDimension, g.maxDimension/2))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 241, G: 245, B: 249, A: 255}}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return &buf
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
