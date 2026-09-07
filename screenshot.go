package main

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"os"
	"sort"
)

type ScreenshotInfo struct {
	Data           []byte // original file bytes
	CroppedPNG     []byte // set by applyCrop
	Format         string
	Width          int
	Height         int
	DominantColors []string // hex, most frequent first
}

func LoadScreenshot(path string) (*ScreenshotInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode screenshot: %w", err)
	}
	b := img.Bounds()
	info := &ScreenshotInfo{Data: data, Format: format, Width: b.Dx(), Height: b.Dy()}
	info.DominantColors = dominantColors(img, 5)
	info.CroppedPNG = data
	return info, nil
}

// applyCrop removes the OS status-bar band from the top and the home-
// indicator band from the bottom before the screenshot is used as a
// reference asset (SwiftUI views don't draw their own status bar chrome).
// Ratios are 0..1 of total height; when unknown (no companion CSS), a small
// fixed default tuned for a typical iPad/iPhone screenshot is used.
func (s *ScreenshotInfo) applyCrop(topRatio, bottomRatio float64) {
	img, _, err := image.Decode(bytes.NewReader(s.Data))
	if err != nil {
		return
	}
	cropped := cropChrome(img, topRatio, bottomRatio)
	var buf bytes.Buffer
	if err := png.Encode(&buf, cropped); err != nil {
		return
	}
	s.CroppedPNG = buf.Bytes()
}

func cropChrome(img image.Image, topRatio, bottomRatio float64) image.Image {
	b := img.Bounds()
	top := b.Min.Y + int(float64(b.Dy())*topRatio)
	bottom := b.Max.Y - int(float64(b.Dy())*bottomRatio)
	if bottom <= top {
		return img
	}
	rect := image.Rect(b.Min.X, top, b.Max.X, bottom)
	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	sub, ok := img.(subImager)
	if !ok {
		return img
	}
	return sub.SubImage(rect)
}

// statusBarRatios derives the top/bottom chrome-crop ratios from a
// companion CSS tree, using the parsed status-bar block's own declared
// height against the root frame's total height.
func statusBarRatios(root *Node) (top, bottom float64) {
	if root == nil {
		return 0, 0
	}
	totalH, ok := parsePx(root.Block.Props["height"])
	if !ok || totalH == 0 {
		return 0, 0
	}
	var find func(n *Node) *Node
	find = func(n *Node) *Node {
		if n.Kind == KindStatusBarChrome {
			return n
		}
		for _, c := range n.Children {
			if f := find(c); f != nil {
				return f
			}
		}
		return nil
	}
	if sb := find(root); sb != nil {
		if h, ok := parsePx(sb.Block.Props["height"]); ok {
			top = h / totalH
		}
	}
	return top, 0
}

// dominantColors buckets a downsampled grid of pixels into a coarse RGB
// histogram and returns the top-N bucket colors as hex, most frequent
// first. No AI/vision involved — a plain color histogram.
func dominantColors(img image.Image, topN int) []string {
	b := img.Bounds()
	const grid = 40
	stepX := maxInt(1, b.Dx()/grid)
	stepY := maxInt(1, b.Dy()/grid)

	counts := map[[3]uint8]int{}
	for y := b.Min.Y; y < b.Max.Y; y += stepY {
		for x := b.Min.X; x < b.Max.X; x += stepX {
			r, g, bl, a := img.At(x, y).RGBA()
			if a < 0x8000 {
				continue
			}
			key := [3]uint8{uint8(r >> 8 &^ 0x0F), uint8(g >> 8 &^ 0x0F), uint8(bl >> 8 &^ 0x0F)}
			counts[key]++
		}
	}
	type kv struct {
		k [3]uint8
		n int
	}
	list := make([]kv, 0, len(counts))
	for k, n := range counts {
		list = append(list, kv{k, n})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].n > list[j].n })

	out := make([]string, 0, topN)
	for i := 0; i < len(list) && i < topN; i++ {
		k := list[i].k
		out = append(out, fmt.Sprintf("#%02X%02X%02X", k[0], k[1], k[2]))
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
