package main

import (
	"crypto/sha256"
	"encoding/hex"
	"image"
	"math/bits"
	"os"
)

// fileSHA256 returns the hex-encoded SHA-256 of a file's raw bytes, used to
// detect byte-identical duplicate CSS inputs in --batch mode.
func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// imageDHash computes a 64-bit difference hash of an image: shrink to a 9x8
// grayscale grid and record, for each row, whether each pixel is brighter
// than the one to its right. Near-identical images (the same screen
// exported twice, a re-compressed copy, a crop that's off by a few pixels)
// end up with hashes only a few bits apart, so duplicate screenshots can be
// caught even when they aren't byte-identical. No external library needed —
// this is the same pixel-sampling approach already used for dominant-color
// extraction in screenshot.go.
func imageDHash(img image.Image) uint64 {
	const w, h = 9, 8
	b := img.Bounds()
	gray := make([][]int, h)
	for y := 0; y < h; y++ {
		gray[y] = make([]int, w)
		for x := 0; x < w; x++ {
			sx := b.Min.X + x*b.Dx()/w
			sy := b.Min.Y + y*b.Dy()/h
			r, g, bl, _ := img.At(sx, sy).RGBA()
			gray[y][x] = int(r+g+bl) / 3
		}
	}
	var hash uint64
	bit := uint(0)
	for y := 0; y < h; y++ {
		for x := 0; x < w-1; x++ {
			if gray[y][x] > gray[y][x+1] {
				hash |= 1 << bit
			}
			bit++
		}
	}
	return hash
}

func hammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// dHashThreshold is the maximum Hamming distance (out of 64 bits) between
// two dHashes for their images to be considered near-duplicates. A
// same-screen re-export (recompression, a few stray anti-aliased pixels,
// a minor crop) typically lands around 5-10 bits apart; genuinely
// different screens are usually 20+ bits apart.
const dHashThreshold = 10
