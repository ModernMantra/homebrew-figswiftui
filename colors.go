package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ColorDef struct {
	Name string
	Hex  string // "#RRGGBB", always uppercase
}

var hexColorRe = regexp.MustCompile(`(?i)^#([0-9a-f]{6}|[0-9a-f]{3})$`)

func normalizeHex(v string) string {
	v = strings.TrimSpace(v)
	if !hexColorRe.MatchString(v) {
		return ""
	}
	if len(v) == 4 { // #RGB -> #RRGGBB
		v = "#" + string(v[1]) + string(v[1]) + string(v[2]) + string(v[2]) + string(v[3]) + string(v[3])
	}
	return strings.ToUpper(v)
}

// CollectColors walks the tree and gathers every distinct background/text
// color, assigning each a semantic name. Figma's raw CSS export carries no
// token names — those in the hand-built reference examples were assigned
// against an existing design system this tool has no access to by default
// (see --match-project) — so names are inferred from how a color is used.
// No color is ever silently dropped: unclassifiable colors fall back to a
// literal Color<HEX> name.
func CollectColors(root *Node) []ColorDef {
	seen := map[string]string{} // hex -> name
	var order []string
	textRank := 0

	// A color used as a button's fill *and* as the active page-indicator
	// dot's fill is almost certainly the app's real accent/brand color.
	// Naming it "AccentColor" (rather than e.g. "ButtonPrimaryBackground")
	// matters beyond labeling: SwiftUI treats a colorset literally named
	// "AccentColor" as the app-wide tint, so generated code that reaches
	// for the system's own Color.accentColor (the page-indicator dots, for
	// instance) resolves to the real brand color instead of silently
	// falling back to the system default blue.
	accentCandidates := map[string]bool{}
	var collectAccent func(n *Node)
	collectAccent = func(n *Node) {
		if n.Kind == KindDotIndicatorGroup && n.DotActiveColor != "" {
			accentCandidates[n.DotActiveColor] = true
		}
		for _, c := range n.Children {
			collectAccent(c)
		}
	}
	collectAccent(root)

	assign := func(hex, role string) {
		hex = normalizeHex(hex)
		if hex == "" {
			return
		}
		if _, ok := seen[hex]; ok {
			return
		}
		seen[hex] = role
		order = append(order, hex)
	}

	var walk func(n *Node, depth int)
	walk = func(n *Node, depth int) {
		// Status-bar chrome is dropped entirely, and an illustration's
		// internal shape colors are already baked as literal hex fills into
		// its generated SVG — harvesting them here too would let a
		// coincidentally-matching decorative color claim a semantic role
		// (Background, AccentColor, ...) ahead of the real button/text/root
		// usage that comes later in document order.
		if n.Kind == KindStatusBarChrome || n.Kind == KindIllustrationGroup {
			return
		}
		if bg := n.Block.Props["background"]; bg != "" {
			role := "Background"
			switch {
			case n.Kind == KindButton && accentCandidates[normalizeHex(bg)]:
				role = "AccentColor"
			case n.Kind == KindButton:
				role = "ButtonPrimaryBackground"
			case depth == 0:
				role = "Background"
			case strings.Contains(strings.ToLower(n.Block.Name), "dialog"):
				role = "DialogBackground"
			}
			assign(bg, role)
		}
		if n.Kind == KindButton {
			if bc := borderHex(n.Block.Props["border"]); bc != "" {
				assign(bc, "ButtonSecondaryBorder")
			}
			// A button's own label color (e.g. a plain text-link button
			// like "Skip") lives directly on the button block itself, not
			// on a KindText child — codegen's buttonLabelColor() falls back
			// to a child's color only when the button node has none of its
			// own, so that same direct color must be registered here too,
			// or the lookup silently misses and the label defaults to
			// whatever tint SwiftUI gives an unstyled Button (the app's
			// accent color).
			if c := n.Block.Props["color"]; c != "" {
				assign(c, "ButtonSecondaryLabel")
			}
		}
		if n.Kind == KindText {
			if c := n.Block.Props["color"]; c != "" {
				role := "TextPrimary"
				switch textRank {
				case 0:
					role = "TextPrimary"
				case 1:
					role = "TextSecondary"
				default:
					role = "TextTertiary"
				}
				textRank++
				assign(c, role)
			}
		}
		for _, c := range n.Children {
			walk(c, depth+1)
		}
	}
	walk(root, 0)

	used := map[string]bool{}
	defs := make([]ColorDef, 0, len(order))
	for _, hex := range order {
		name := seen[hex]
		if used[name] {
			name = "Color" + strings.TrimPrefix(hex, "#")
		}
		used[name] = true
		defs = append(defs, ColorDef{Name: name, Hex: hex})
	}
	return defs
}

// projectColorMatcher looks up existing colorset names in a real Xcode
// project by exact RGB match, so generated code can reuse e.g. "ContentPrimary"
// instead of inventing "TextPrimary" when the target project already defines it.
type projectColorMatcher struct {
	byHex map[string]string
}

func loadProjectColors(root string) (*projectColorMatcher, error) {
	m := &projectColorMatcher{byHex: map[string]string{}}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Base(path) != "Contents.json" {
			return nil
		}
		if !strings.HasSuffix(filepath.Dir(path), ".colorset") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var doc struct {
			Colors []struct {
				Color struct {
					Components struct {
						Red   string `json:"red"`
						Green string `json:"green"`
						Blue  string `json:"blue"`
					} `json:"components"`
				} `json:"color"`
			} `json:"colors"`
		}
		if err := json.Unmarshal(data, &doc); err != nil || len(doc.Colors) == 0 {
			return nil
		}
		comp := doc.Colors[0].Color.Components
		hex := hexFromComponents(comp.Red, comp.Green, comp.Blue)
		if hex == "" {
			return nil
		}
		name := strings.TrimSuffix(filepath.Base(filepath.Dir(path)), ".colorset")
		m.byHex[hex] = name
		return nil
	})
	return m, err
}

func hexFromComponents(r, g, b string) string {
	rr := strings.TrimPrefix(r, "0x")
	gg := strings.TrimPrefix(g, "0x")
	bb := strings.TrimPrefix(b, "0x")
	if len(rr) != 2 || len(gg) != 2 || len(bb) != 2 {
		return ""
	}
	return strings.ToUpper("#" + rr + gg + bb)
}

func applyProjectColorNames(colors []ColorDef, matcher *projectColorMatcher) {
	for i := range colors {
		if name, ok := matcher.byHex[colors[i].Hex]; ok {
			colors[i].Name = name
		}
	}
}
