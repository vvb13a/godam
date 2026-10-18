package service

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestExtractImageColorsDistribution(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	red := color.RGBA{R: 200, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			if y < 60 {
				img.Set(x, y, red)
			} else {
				img.Set(x, y, blue)
			}
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	colors, err := extractImageColors(data.Bytes())
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(colors) != 2 {
		t.Fatalf("colors = %+v, want 2", colors)
	}
	if colors[0].Hex != "#c80000" || colors[0].Bucket != "red" || colors[0].Percentage != 60 {
		t.Fatalf("first color = %+v", colors[0])
	}
	if colors[1].Hex != "#0000ff" || colors[1].Bucket != "blue" || colors[1].Percentage != 40 {
		t.Fatalf("second color = %+v", colors[1])
	}
}

func TestExtractImageColorsIgnoresTransparentPixels(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	green := color.RGBA{G: 180, A: 255}
	for y := 0; y < 100; y++ {
		for x := 0; x < 50; x++ {
			img.Set(x, y, green)
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	colors, err := extractImageColors(data.Bytes())
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(colors) != 1 || colors[0].Bucket != "green" || colors[0].Percentage != 100 {
		t.Fatalf("colors = %+v", colors)
	}
}

func TestColorBucket(t *testing.T) {
	tests := []struct {
		r, g, b uint8
		want    string
	}{
		{0, 0, 0, "black"},
		{255, 255, 255, "white"},
		{128, 128, 128, "gray"},
		{200, 0, 0, "red"},
		{220, 90, 20, "orange"},
		{240, 220, 20, "yellow"},
		{20, 160, 40, "green"},
		{20, 160, 150, "green"},
		{20, 80, 220, "blue"},
		{140, 40, 220, "purple"},
		{220, 40, 160, "purple"},
		{100, 55, 20, "brown"},
		{248, 246, 250, "white"},
		{234, 231, 242, "white"},
		{224, 218, 232, "white"},
		{174, 139, 135, "brown"},
		{197, 165, 166, "pink"},
		{114, 83, 86, "brown"},
		{53, 28, 32, "purple"},
		{118, 74, 29, "brown"},
		{147, 107, 67, "brown"},
		{132, 90, 47, "brown"},
		{171, 136, 101, "brown"},
		{35, 44, 54, "black"},
		{8, 18, 31, "black"},
		{89, 80, 74, "gray"},
		{235, 219, 183, "yellow"},
		{203, 200, 174, "yellow"},
	}
	for _, test := range tests {
		if got := colorBucket(test.r, test.g, test.b); got != test.want {
			t.Errorf("colorBucket(%d,%d,%d) = %q, want %q", test.r, test.g, test.b, got, test.want)
		}
	}
}
