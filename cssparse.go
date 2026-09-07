package main

import (
	"regexp"
	"strconv"
	"strings"
)

// Block is one parsed Figma "copy as CSS" layer entry: a `/* Layer Name */`
// comment followed by its `key: value;` declarations, in document order.
type Block struct {
	Name  string
	Props map[string]string
	Order int // parsed from an "order" declaration; -1 if absent
	Index int // 0-based position in document order
}

var commentLineRe = regexp.MustCompile(`^/\*\s*(.*?)\s*\*/$`)

// figmaSubHeaders are comment lines Figma's CSS export always uses to label
// a sub-group of declarations *within* the current layer (its auto-layout
// properties, or its flex-child properties) rather than a new layer. Kept
// as a backstop alongside the blank-line rule below, since these two exact
// phrases are fixed strings Figma itself emits.
var figmaSubHeaders = map[string]bool{
	"Auto layout":        true,
	"Inside auto layout": true,
}

// ParseBlocks splits a Figma "copy as CSS" export into ordered layer blocks.
// The export is a flat, ordered sequence of comment+declaration groups with
// no real nesting braces; BuildTree reconstructs the tree from this list.
//
// Figma also inserts inline, mid-property comments that are *not* new
// layers: sub-headers ("Auto layout", "Inside auto layout"), text/effect
// style attributions ("Menu/Menu item", "Button Shadow"), and computed-value
// asides ("identical to box height, or 150%"). A genuine new layer is
// reliably preceded by two or more blank lines in Figma's own formatting;
// these inline asides have zero or one. That blank-line count is the
// primary signal here — the fixed-phrase check is only a backstop for the
// two sub-header phrases, in case some export has different spacing around
// them.
func ParseBlocks(css string) []*Block {
	var blocks []*Block
	var current *Block
	blankRun := 0

	for _, raw := range strings.Split(css, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			blankRun++
			continue
		}
		precedingBlanks := blankRun
		blankRun = 0

		if m := commentLineRe.FindStringSubmatch(line); m != nil {
			name := m[1]
			startsNewLayer := current == nil || precedingBlanks >= 2
			if figmaSubHeaders[name] {
				startsNewLayer = false
			}
			if !startsNewLayer {
				continue
			}
			current = &Block{Name: name, Props: map[string]string{}, Order: -1, Index: len(blocks)}
			blocks = append(blocks, current)
			continue
		}
		if current == nil {
			continue
		}
		line = strings.TrimSuffix(line, ";")
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key == "" || val == "" {
			continue
		}
		current.Props[key] = val
		if key == "order" {
			if n, err := strconv.Atoi(val); err == nil {
				current.Order = n
			}
		}
	}
	return blocks
}
