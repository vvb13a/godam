package tui

import (
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// RenderPreview scales images using Catmull-Rom bicubic interpolation for maximum sharpness
func RenderPreview(r io.Reader, targetCols, maxRows int) (string, error) {
	// 1. Fast decode from 600px JPEG preview
	srcImg, err := jpeg.Decode(r)
	if err != nil {
		var decodeErr error
		srcImg, _, decodeErr = image.Decode(r)
		if decodeErr != nil {
			return "", decodeErr
		}
	}

	bounds := srcImg.Bounds()
	imgW := bounds.Dx()
	imgH := bounds.Dy()

	if imgW == 0 || imgH == 0 {
		return "", fmt.Errorf("invalid image dimensions")
	}

	// 2. Width-first aspect ratio calculation
	targetW := targetCols
	targetPixelH := (targetW * imgH) / imgW
	targetRows := (targetPixelH + 1) / 2

	// Height clamping for tall portrait images
	if maxRows > 0 && targetRows > maxRows {
		targetRows = maxRows
		targetPixelH = targetRows * 2
		targetW = (targetPixelH * imgW) / imgH
	}

	if targetW < 1 {
		targetW = 1
	}
	if targetPixelH < 2 {
		targetPixelH = 2
	}
	if targetPixelH%2 != 0 {
		targetPixelH++
	}

	// 3. Ultra-fast Catmull-Rom Resampling (Edge-preserving bicubic)
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetPixelH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), srcImg, bounds, draw.Over, nil)

	// 4. Direct Byte Slice Traversal (sub-millisecond execution)
	var sb strings.Builder
	sb.Grow(targetW * targetPixelH * 26)

	stride := dst.Stride

	for y := 0; y < targetPixelH; y += 2 {
		topRowOffset := y * stride
		botRowOffset := (y + 1) * stride

		for x := 0; x < targetW; x++ {
			topIdx := topRowOffset + (x * 4)
			botIdx := botRowOffset + (x * 4)

			// Top pixel (Foreground)
			topR := dst.Pix[topIdx]
			topG := dst.Pix[topIdx+1]
			topB := dst.Pix[topIdx+2]

			// Bottom pixel (Background)
			botR := dst.Pix[botIdx]
			botG := dst.Pix[botIdx+1]
			botB := dst.Pix[botIdx+2]

			// Dynamic contrast boost for terminal readability
			topR, topG, topB = sharpenColor(topR, topG, topB)
			botR, botG, botB = sharpenColor(botR, botG, botB)

			// \x1b[38;2;R;G;Bm (Top) \x1b[48;2;R;G;Bm (Bottom) ▀ \x1b[0m
			sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀\x1b[0m", topR, topG, topB, botR, botG, botB))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// sharpenColor applies an S-curve contrast boost so edges stand out against terminal backgrounds
func sharpenColor(r, g, b uint8) (uint8, uint8, uint8) {
	return boostChannel(r), boostChannel(g), boostChannel(b)
}

func boostChannel(v uint8) uint8 {
	f := float64(v) / 255.0
	// Gentle S-Curve contrast: f' = 3f^2 - 2f^3
	f = f * f * (3.0 - 2.0*f)
	res := int(f * 255.0)
	if res > 255 {
		return 255
	}
	if res < 0 {
		return 0
	}
	return uint8(res)
}
