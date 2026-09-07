package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var genericLayerNameRe = regexp.MustCompile(`(?i)^(rectangle\s*\d*|ellipse\s*\d*|vector|frame\s*\d*|group\s*\d*|combined shape|placeholder|divier|divider|\d+%?)$`)

var nonAlnumRe = regexp.MustCompile(`[^a-zA-Z0-9]+`)

type buttonInfo struct {
	varName string
	label   string
}

type swiftGen struct {
	buttons        []buttonInfo
	colorByHex     map[string]string
	dotCount       int
	dotActiveIndex int
	hasDots        bool
}

// GenerateSwift walks the reconstructed layout tree and emits a SwiftUI
// View source file matching the structural conventions of the hand-built
// reference examples (VStack/HStack skeleton, semantic Color("Name")
// references, injectable on<Action> closures, trailing #Preview).
func GenerateSwift(root *Node, screenName string, colors []ColorDef) string {
	g := &swiftGen{colorByHex: map[string]string{}}
	for _, c := range colors {
		g.colorByHex[c.Hex] = c.Name
	}
	g.collectButtons(root)
	g.findDots(root)

	spacing := 16.0
	if gp, ok := parsePx(root.Block.Props["gap"]); ok && gp > 0 {
		spacing = gp
	}

	var body strings.Builder
	for _, child := range root.Children {
		g.writeNode(&body, child, 3)
	}

	var out strings.Builder
	out.WriteString("import SwiftUI\n\n")
	fmt.Fprintf(&out, "struct %s: View {\n", screenName)
	if g.hasDots {
		fmt.Fprintf(&out, "    var pageIndex: Int = %d\n", g.dotActiveIndex)
		fmt.Fprintf(&out, "    var totalPages: Int = %d\n", g.dotCount)
	}
	for _, b := range g.buttons {
		fmt.Fprintf(&out, "    var %s: () -> Void = {}\n", b.varName)
	}
	out.WriteString("\n    var body: some View {\n")
	fmt.Fprintf(&out, "        VStack(spacing: %s) {\n", trimNum(spacing))
	out.WriteString(body.String())
	out.WriteString("        }\n")
	out.WriteString("        .padding(24)\n")
	out.WriteString("        .frame(maxWidth: .infinity, maxHeight: .infinity)\n")
	if bgName := g.colorName(root.Block.Props["background"]); bgName != "" {
		fmt.Fprintf(&out, "        .background(Color(\"%s\"))\n", bgName)
	}
	out.WriteString("    }\n")
	out.WriteString("}\n\n")
	fmt.Fprintf(&out, "#Preview {\n    %s()\n}\n", screenName)
	return out.String()
}

func (g *swiftGen) collectButtons(n *Node) {
	if n.Kind == KindButton {
		g.buttons = append(g.buttons, buttonInfo{
			varName: "on" + sanitizeIdentifier(n.Block.Name, true),
			label:   textOf(n),
		})
	}
	for _, c := range n.Children {
		g.collectButtons(c)
	}
}

func (g *swiftGen) findDots(n *Node) {
	if n.Kind == KindDotIndicatorGroup && !g.hasDots {
		g.hasDots = true
		g.dotCount = n.DotCount
		g.dotActiveIndex = n.DotActiveIndex
	}
	for _, c := range n.Children {
		g.findDots(c)
	}
}

// buttonLabelColor finds the label color for a KindButton node. writeNode
// never recurses into a button's children (it renders as a single flat
// Button(...)), but the color is still present on a descendant block's own
// Props (typically a generically-named "Placeholder" text child) even
// though that descendant is never itself walked.
func (g *swiftGen) buttonLabelColor(n *Node) string {
	if c := g.colorName(n.Block.Props["color"]); c != "" {
		return c
	}
	for _, c := range n.Children {
		if found := g.buttonLabelColor(c); found != "" {
			return found
		}
	}
	return ""
}

func (g *swiftGen) colorName(hex string) string {
	n := normalizeHex(hex)
	if n == "" {
		return ""
	}
	return g.colorByHex[n]
}

func (g *swiftGen) writeNode(b *strings.Builder, n *Node, depth int) {
	ind := strings.Repeat("    ", depth)

	switch n.Kind {
	case KindStatusBarChrome:
		return

	case KindText:
		txt := textOf(n)
		fmt.Fprintf(b, "%sText(%s)\n", ind, swiftStringLiteral(txt))
		if fs, ok := parsePx(n.Block.Props["font-size"]); ok {
			fmt.Fprintf(b, "%s    .font(%s)\n", ind, swiftFont(fs, n.Block.Props["font-weight"]))
		}
		if col := g.colorName(n.Block.Props["color"]); col != "" {
			fmt.Fprintf(b, "%s    .foregroundStyle(Color(\"%s\"))\n", ind, col)
		}

	case KindButton:
		varName := "on" + sanitizeIdentifier(n.Block.Name, true)
		fmt.Fprintf(b, "%s// TODO(figswiftui): confirm this label matches the rendered/localized text\n", ind)
		fmt.Fprintf(b, "%sButton(%s, action: %s)\n", ind, swiftStringLiteral(textOf(n)), varName)
		if labelColor := g.buttonLabelColor(n); labelColor != "" {
			fmt.Fprintf(b, "%s    .foregroundStyle(Color(\"%s\"))\n", ind, labelColor)
		}
		// A plain text-link button (e.g. "Skip") has no fill of its own —
		// giving it button-chrome padding/frame/background/clipShape it was
		// never designed with would visibly misrender it as a boxed button.
		bgColorName := g.colorName(n.Block.Props["background"])
		if bgColorName != "" {
			if pad := paddingModifier(n); pad != "" {
				fmt.Fprintf(b, "%s    %s\n", ind, pad)
			} else {
				fmt.Fprintf(b, "%s    .padding(.horizontal, 14)\n", ind)
				fmt.Fprintf(b, "%s    .padding(.vertical, 10)\n", ind)
			}
			if w, ok := parsePx(n.Block.Props["width"]); ok {
				fmt.Fprintf(b, "%s    .frame(width: %s)\n", ind, trimNum(w))
			}
			fmt.Fprintf(b, "%s    .background(Color(\"%s\"))\n", ind, bgColorName)
			radius := 8.0
			if br, ok := parsePx(n.Block.Props["border-radius"]); ok {
				radius = br
			}
			shape := fmt.Sprintf("RoundedRectangle(cornerRadius: %s)", trimNum(radius))
			if borderColor := g.colorName(borderHex(n.Block.Props["border"])); borderColor != "" {
				fmt.Fprintf(b, "%s    .overlay(%s.stroke(Color(\"%s\")))\n", ind, shape, borderColor)
			}
			fmt.Fprintf(b, "%s    .clipShape(%s)\n", ind, shape)
		}

	case KindDotIndicatorGroup:
		fmt.Fprintf(b, "%sHStack(spacing: 8) {\n", ind)
		fmt.Fprintf(b, "%s    ForEach(0..<totalPages, id: \\.self) { index in\n", ind)
		fmt.Fprintf(b, "%s        Circle()\n", ind)
		fmt.Fprintf(b, "%s            .fill(index == pageIndex ? Color.accentColor : Color(.systemGray4))\n", ind)
		fmt.Fprintf(b, "%s            .frame(width: 12, height: 12)\n", ind)
		fmt.Fprintf(b, "%s    }\n", ind)
		fmt.Fprintf(b, "%s}\n", ind)

	case KindIllustrationGroup:
		imgName := sanitizeIdentifier(n.Block.Name, true) + "Illustration"
		fmt.Fprintf(b, "%sImage(\"%s\")\n", ind, imgName)
		fmt.Fprintf(b, "%s    .resizable()\n", ind)
		fmt.Fprintf(b, "%s    .aspectRatio(contentMode: .fit)\n", ind)

	case KindShape:
		if op, ok := parseOpacity(n.Block.Props["opacity"]); ok && op == 0 {
			// An invisible (opacity: 0) fill is a layout spacer in disguise,
			// not a color to render.
			fmt.Fprintf(b, "%sSpacer(minLength: 0)\n", ind)
		} else if bg := g.colorName(n.Block.Props["background"]); bg != "" {
			fmt.Fprintf(b, "%sColor(\"%s\")\n", ind, bg)
		}

	default: // KindContainer
		stack, spacingAttr := containerStack(n)
		fmt.Fprintf(b, "%s%s%s {\n", ind, stack, spacingAttr)
		for _, c := range n.Children {
			g.writeNode(b, c, depth+1)
		}
		fmt.Fprintf(b, "%s}\n", ind)
		if pad := paddingModifier(n); pad != "" {
			fmt.Fprintf(b, "%s%s\n", ind, pad)
		}
	}
}

func textOf(n *Node) string {
	name := strings.TrimSpace(n.Block.Name)
	if genericLayerNameRe.MatchString(name) {
		return "TODO"
	}
	return name
}

// borderHex extracts the color from a CSS border shorthand, e.g.
// "1px solid #EDEDED" -> "#EDEDED".
func borderHex(border string) string {
	for _, field := range strings.Fields(border) {
		if hexColorRe.MatchString(field) {
			return field
		}
	}
	return ""
}

func parseOpacity(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

func swiftFont(px float64, weight string) string {
	base := ".caption"
	switch {
	case px >= 24:
		base = ".title2"
	case px >= 20:
		base = ".title3"
	case px >= 17:
		base = ".headline"
	case px >= 15:
		base = ".subheadline"
	case px >= 13:
		base = ".footnote"
	}
	if weight == "700" || strings.EqualFold(weight, "bold") {
		return base + ".bold()"
	}
	return base
}

func containerStack(n *Node) (string, string) {
	stack := "VStack"
	if n.Block.Props["flex-direction"] == "row" {
		stack = "HStack"
	}
	spacing := "(spacing: 0)"
	if gapPx, ok := parsePx(n.Block.Props["gap"]); ok && gapPx > 0 {
		spacing = fmt.Sprintf("(spacing: %s)", trimNum(gapPx))
	}
	return stack, spacing
}

func paddingModifier(n *Node) string {
	pad := n.Block.Props["padding"]
	if pad == "" {
		return ""
	}
	var maxPx float64
	found := false
	for _, v := range strings.Fields(pad) {
		if f, ok := parsePx(v); ok {
			found = true
			if f > maxPx {
				maxPx = f
			}
		}
	}
	if !found || maxPx == 0 {
		return ""
	}
	return fmt.Sprintf(".padding(%s)", trimNum(maxPx))
}

func sanitizeIdentifier(s string, upperFirst bool) string {
	parts := nonAlnumRe.Split(s, -1)
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	out := b.String()
	if out == "" {
		out = "Item"
	}
	if !upperFirst {
		out = strings.ToLower(out[:1]) + out[1:]
	}
	return out
}

func swiftStringLiteral(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
