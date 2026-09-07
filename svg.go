package main

import (
	"fmt"
	"strconv"
	"strings"
)

// RenderIllustrationSVG mechanically translates a shape-only subtree into an
// SVG: Rectangle-named nodes become <rect>, Ellipse-named become <ellipse>,
// positions resolve from the CSS's left/right/top/bottom (percentages or
// px) against the group's own bounding box. This is the deterministic
// counterpart to a hand-drawn illustration asset — mechanical shape
// translation, not artistic judgement.
func RenderIllustrationSVG(group *Node) string {
	w, ok := parsePx(group.Block.Props["width"])
	if !ok || w == 0 {
		w = 600
	}
	h, ok := parsePx(group.Block.Props["height"])
	if !ok || h == 0 {
		h = 400
	}

	var b strings.Builder
	fmt.Fprintf(&b, "<svg viewBox=\"0 0 %s %s\" xmlns=\"http://www.w3.org/2000/svg\">\n", trimNum(w), trimNum(h))

	var walk func(n *Node)
	walk = func(n *Node) {
		if n.Kind == KindShape {
			writeShape(&b, n, w, h)
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, c := range group.Children {
		walk(c)
	}
	b.WriteString("</svg>\n")
	return b.String()
}

func writeShape(b *strings.Builder, n *Node, parentW, parentH float64) {
	x, y, sw, sh := resolveBox(n.Block.Props, parentW, parentH)
	fill := normalizeHex(n.Block.Props["background"])
	if fill == "" {
		fill = "#E3E3E3"
	}

	name := strings.ToLower(n.Block.Name)
	isEllipse := strings.Contains(name, "ellipse") || strings.Contains(name, "circle")

	if isEllipse {
		rx, ry := sw/2, sh/2
		fmt.Fprintf(b, "  <ellipse cx=\"%s\" cy=\"%s\" rx=\"%s\" ry=\"%s\" fill=\"%s\"",
			trimNum(x+rx), trimNum(y+ry), trimNum(rx), trimNum(ry), fill)
	} else {
		rxAttr := ""
		if br, ok := parsePx(n.Block.Props["border-radius"]); ok {
			rxAttr = fmt.Sprintf(" rx=\"%s\"", trimNum(br))
		}
		fmt.Fprintf(b, "  <rect x=\"%s\" y=\"%s\" width=\"%s\" height=\"%s\" fill=\"%s\"%s",
			trimNum(x), trimNum(y), trimNum(sw), trimNum(sh), fill, rxAttr)
	}
	if op := n.Block.Props["opacity"]; op != "" {
		fmt.Fprintf(b, " opacity=\"%s\"", op)
	}
	b.WriteString(" />\n")
}

// resolveBox turns Figma's left/right/top/bottom (percent, calc(), or px)
// declarations into an absolute x/y/w/h box within the parent's own bounds.
func resolveBox(props map[string]string, parentW, parentH float64) (x, y, w, h float64) {
	if pw, ok := parsePx(props["width"]); ok {
		w = pw
	}
	if ph, ok := parsePx(props["height"]); ok {
		h = ph
	}
	if lp, ok := parsePercent(props["left"]); ok {
		x = lp / 100 * parentW
	} else if lx, ok := parsePx(props["left"]); ok {
		x = lx
	}
	if tp, ok := parsePercent(props["top"]); ok {
		y = tp / 100 * parentH
	} else if ty, ok := parsePx(props["top"]); ok {
		y = ty
	}
	if w == 0 {
		if rp, ok := parsePercent(props["right"]); ok {
			w = parentW - x - (rp / 100 * parentW)
		}
	}
	if h == 0 {
		if bp, ok := parsePercent(props["bottom"]); ok {
			h = parentH - y - (bp / 100 * parentH)
		}
	}
	if w <= 0 {
		w = parentW * 0.1
	}
	if h <= 0 {
		h = parentH * 0.1
	}
	return
}

func trimNum(f float64) string {
	s := strconv.FormatFloat(f, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}
