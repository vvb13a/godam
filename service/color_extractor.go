package service

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"sort"
	"strconv"

	"github.com/vvb13a/godam/domain"
	_ "golang.org/x/image/webp"
)

const (
	colorPaletteSize   = 10
	maxColorPalette    = 10
	maxColorSamples    = 250000
	minColorPercentage = 0.5
)

type weightedColor struct {
	r     float64
	g     float64
	b     float64
	count int
}

type colorBin struct {
	count int
	r     int64
	g     int64
	b     int64
}

func extractImageColors(data []byte) ([]domain.AssetColor, error) {
	return extractImageColorsReader(bytes.NewReader(data))
}

func extractImageColorsReader(reader io.Reader) ([]domain.AssetColor, error) {
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}
	return paletteFromImage(img), nil
}

func paletteFromImage(img image.Image) []domain.AssetColor {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil
	}

	step := 1
	for (width/step)*(height/step) > maxColorSamples {
		step++
	}

	histogram := make(map[uint32]*colorBin)
	total := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y += step {
		for x := bounds.Min.X; x < bounds.Max.X; x += step {
			pixel := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if pixel.A < 128 {
				continue
			}
			rr := pixel.R
			gg := pixel.G
			bb := pixel.B
			key := uint32(rr>>4)<<8 | uint32(gg>>4)<<4 | uint32(bb>>4)
			bin := histogram[key]
			if bin == nil {
				bin = &colorBin{}
				histogram[key] = bin
			}
			bin.count++
			bin.r += int64(rr)
			bin.g += int64(gg)
			bin.b += int64(bb)
			total++
		}
	}
	if total == 0 {
		return nil
	}

	samples := make([]weightedColor, 0, len(histogram))
	for _, bin := range histogram {
		if bin.count == 0 {
			continue
		}
		samples = append(samples, weightedColor{
			r:     float64(bin.r) / float64(bin.count),
			g:     float64(bin.g) / float64(bin.count),
			b:     float64(bin.b) / float64(bin.count),
			count: bin.count,
		})
	}
	sort.Slice(samples, func(i, j int) bool {
		if samples[i].count != samples[j].count {
			return samples[i].count > samples[j].count
		}
		return weightedColorLess(samples[i], samples[j])
	})

	k := colorPaletteSize
	if len(samples) < k {
		k = len(samples)
	}
	centroids := initialCentroids(samples, k)
	assignments := make([]int, len(samples))
	for i := range assignments {
		assignments[i] = -1
	}

	for iteration := 0; iteration < 12; iteration++ {
		changed := false
		for i, sample := range samples {
			best := nearestCentroid(sample, centroids)
			if assignments[i] != best {
				assignments[i] = best
				changed = true
			}
		}
		if !changed && iteration > 0 {
			break
		}
		sums := make([]weightedColor, len(centroids))
		for i, sample := range samples {
			centroid := &sums[assignments[i]]
			centroid.r += sample.r * float64(sample.count)
			centroid.g += sample.g * float64(sample.count)
			centroid.b += sample.b * float64(sample.count)
			centroid.count += sample.count
		}
		for i := range centroids {
			if sums[i].count == 0 {
				continue
			}
			centroids[i] = weightedColor{
				r:     sums[i].r / float64(sums[i].count),
				g:     sums[i].g / float64(sums[i].count),
				b:     sums[i].b / float64(sums[i].count),
				count: sums[i].count,
			}
		}
	}

	clusters := make([]weightedColor, len(centroids))
	for _, sample := range samples {
		best := nearestCentroid(sample, centroids)
		cluster := &clusters[best]
		cluster.r += sample.r * float64(sample.count)
		cluster.g += sample.g * float64(sample.count)
		cluster.b += sample.b * float64(sample.count)
		cluster.count += sample.count
	}
	merged := make([]weightedColor, 0, len(clusters))
	for _, cluster := range clusters {
		if cluster.count == 0 {
			continue
		}
		cluster.r /= float64(cluster.count)
		cluster.g /= float64(cluster.count)
		cluster.b /= float64(cluster.count)
		mergeColor(&merged, cluster)
	}

	sort.Slice(merged, func(i, j int) bool {
		if merged[i].count != merged[j].count {
			return merged[i].count > merged[j].count
		}
		return weightedColorLess(merged[i], merged[j])
	})
	if len(merged) > maxColorPalette {
		merged = merged[:maxColorPalette]
	}

	colors := make([]domain.AssetColor, 0, len(merged))
	for _, cluster := range merged {
		percentage := float64(cluster.count) * 100 / float64(total)
		if percentage < minColorPercentage {
			continue
		}
		r := uint8(math.Round(cluster.r))
		g := uint8(math.Round(cluster.g))
		b := uint8(math.Round(cluster.b))
		colors = append(colors, domain.AssetColor{
			Hex:        fmt.Sprintf("#%02x%02x%02x", r, g, b),
			Bucket:     colorBucket(r, g, b),
			Percentage: math.Round(percentage*10) / 10,
		})
	}
	return colors
}

func initialCentroids(samples []weightedColor, k int) []weightedColor {
	centroids := []weightedColor{samples[0]}
	for len(centroids) < k {
		bestIndex := -1
		bestScore := -1.0
		for i, sample := range samples {
			minDistance := math.MaxFloat64
			for _, centroid := range centroids {
				distance := colorDistanceSquared(sample, centroid)
				if distance < minDistance {
					minDistance = distance
				}
			}
			score := minDistance * float64(sample.count)
			if score > bestScore {
				bestScore = score
				bestIndex = i
			}
		}
		if bestIndex < 0 || bestScore == 0 {
			break
		}
		centroids = append(centroids, samples[bestIndex])
	}
	return centroids
}

func nearestCentroid(sample weightedColor, centroids []weightedColor) int {
	best := 0
	bestDistance := math.MaxFloat64
	for i, centroid := range centroids {
		distance := colorDistanceSquared(sample, centroid)
		if distance < bestDistance {
			bestDistance = distance
			best = i
		}
	}
	return best
}

func mergeColor(merged *[]weightedColor, color weightedColor) {
	for i := range *merged {
		existing := &(*merged)[i]
		if colorDistanceSquared(*existing, color) > 18*18 {
			continue
		}
		total := existing.count + color.count
		existing.r = (existing.r*float64(existing.count) + color.r*float64(color.count)) / float64(total)
		existing.g = (existing.g*float64(existing.count) + color.g*float64(color.count)) / float64(total)
		existing.b = (existing.b*float64(existing.count) + color.b*float64(color.count)) / float64(total)
		existing.count = total
		return
	}
	*merged = append(*merged, color)
}

func colorDistanceSquared(a, b weightedColor) float64 {
	dr := a.r - b.r
	dg := a.g - b.g
	db := a.b - b.b
	return dr*dr + dg*dg + db*db
}

func weightedColorLess(a, b weightedColor) bool {
	if a.r != b.r {
		return a.r < b.r
	}
	if a.g != b.g {
		return a.g < b.g
	}
	return a.b < b.b
}

func bucketForHex(value string) (string, bool) {
	if len(value) != 7 || value[0] != '#' {
		return "", false
	}
	number, err := strconv.ParseUint(value[1:], 16, 24)
	if err != nil {
		return "", false
	}
	return colorBucket(uint8(number>>16), uint8(number>>8), uint8(number)), true
}

func colorBucket(r, g, b uint8) string {
	index := int(r>>3) + 32*int(g>>3) + 32*32*int(b>>3)
	return basicColorTermNames[basicColorTermTable[index]]
}
