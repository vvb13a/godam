package service

import (
	"image"
	"image/color"
	"image/draw"
	"testing"
)

func TestNormalizeWaveformFillsAndCenters(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 100))
	// A thin bar near the vertical centre (extent 4).
	draw.Draw(src, image.Rect(20, 48, 80, 52), &image.Uniform{C: color.RGBA{R: 79, G: 70, B: 229, A: 255}}, image.Point{}, draw.Src)

	out, err := normalizeWaveform(src, 0.8)
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := image.Decode(out)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	minY, maxY := b.Dy(), -1
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a > 32768 {
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
	extent := maxY - minY + 1
	if extent < 78 || extent > 82 {
		t.Fatalf("extent = %d, want ~80", extent)
	}
	center := float64(minY+maxY) / 2
	if center < 45 || center > 55 {
		t.Fatalf("center = %.1f, want ~50", center)
	}
}
