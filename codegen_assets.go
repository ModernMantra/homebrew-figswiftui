package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// WriteAssetsCatalog writes an Assets.xcassets folder matching the exact
// Contents.json shapes Xcode itself produces (schemas taken verbatim from
// the hand-built reference examples in output/, not guessed).
func WriteAssetsCatalog(root string, colors []ColorDef, illustrations map[string]string, pngIcons map[string][]byte, screenshotPNG []byte, screenshotAssetName string) error {
	catalog := filepath.Join(root, "Assets.xcassets")
	if err := os.MkdirAll(catalog, 0o755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(catalog, "Contents.json"), map[string]any{
		"info": map[string]any{"author": "xcode", "version": 1},
	}); err != nil {
		return err
	}

	for _, c := range colors {
		if err := writeColorSet(catalog, c); err != nil {
			return err
		}
	}
	for name, svg := range illustrations {
		if err := writeVectorImageSet(catalog, name, svg); err != nil {
			return err
		}
	}
	for name, png := range pngIcons {
		if err := writePNGImageSet(catalog, name, png); err != nil {
			return err
		}
	}
	if len(screenshotPNG) > 0 {
		if err := writePNGImageSet(catalog, screenshotAssetName, screenshotPNG); err != nil {
			return err
		}
	}
	return nil
}

func writeColorSet(catalog string, c ColorDef) error {
	dir := filepath.Join(catalog, c.Name+".colorset")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	hex := strings.TrimPrefix(c.Hex, "#")
	if len(hex) != 6 {
		hex = "000000"
	}
	doc := map[string]any{
		"colors": []map[string]any{{
			"color": map[string]any{
				"color-space": "srgb",
				"components": map[string]any{
					"alpha": "1.000",
					"red":   "0x" + strings.ToUpper(hex[0:2]),
					"green": "0x" + strings.ToUpper(hex[2:4]),
					"blue":  "0x" + strings.ToUpper(hex[4:6]),
				},
			},
			"idiom": "universal",
		}},
		"info": map[string]any{"author": "xcode", "version": 1},
	}
	return writeJSON(filepath.Join(dir, "Contents.json"), doc)
}

func writeVectorImageSet(catalog, name, svg string) error {
	dir := filepath.Join(catalog, name+".imageset")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, name+".svg"), []byte(svg), 0o644); err != nil {
		return err
	}
	doc := map[string]any{
		"images":     []map[string]any{{"filename": name + ".svg", "idiom": "universal"}},
		"info":       map[string]any{"author": "xcode", "version": 1},
		"properties": map[string]any{"preserves-vector-representation": true},
	}
	return writeJSON(filepath.Join(dir, "Contents.json"), doc)
}

func writePNGImageSet(catalog, name string, png []byte) error {
	dir := filepath.Join(catalog, name+".imageset")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, name+".png"), png, 0o644); err != nil {
		return err
	}
	doc := map[string]any{
		"images": []map[string]any{{"filename": name + ".png", "idiom": "universal"}},
		"info":   map[string]any{"author": "xcode", "version": 1},
	}
	return writeJSON(filepath.Join(dir, "Contents.json"), doc)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
