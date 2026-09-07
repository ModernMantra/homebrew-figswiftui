package main

import (
	"regexp"
	"strconv"
	"strings"
)

type NodeKind int

const (
	KindContainer NodeKind = iota
	KindText
	KindShape
	KindStatusBarChrome
	KindButton
	KindDotIndicatorGroup
	KindIllustrationGroup
)

type Node struct {
	Block    *Block
	Children []*Node
	Kind     NodeKind

	// Populated only for KindDotIndicatorGroup: the original sibling count
	// and which one looked "active" (its inner dot is larger or a
	// differently-colored fill than the rest), before the sibling list was
	// collapsed down to a single template child.
	DotCount       int
	DotActiveIndex int
	DotActiveColor string // normalized hex of the active dot's fill, if any

	// DisplayTextOverride, when set, is used as this node's rendered text
	// instead of deriving it from Block.Name — e.g. a button's real
	// OCR-recognized label (see applyOCRButtonLabels). Kept separate from
	// Block.Name, which stays the CSS component name and is still what
	// action-closure variable names (onNext/onPrevious/onSkip) derive
	// from — filling in real display text should never silently rename
	// onNext to onWeiter just because the rendered/localized text differs.
	DisplayTextOverride string
	OCRVerified         bool
}

type stackFrame struct {
	node        *Node
	expectedOrd int
	width       float64
	height      float64
	hasWidth    bool
	hasHeight   bool
}

// BuildTree reconstructs a layout tree from Figma's flat, order-annotated
// block sequence. Figma's "copy as CSS" export carries no explicit nesting
// braces — only a document-order sequence where each auto-layout child
// repeats its container-relative `order` (resetting to 0 per container).
// Order alone is ambiguous whenever two currently-open containers happen to
// expect the same next order value (ex: a single-child container and its own
// parent can both be "expecting order 1" at once) — a geometry containment
// check (a child's fixed px size can't exceed its real parent's) resolves
// that ambiguity by popping containers the new block plainly can't fit in.
func BuildTree(blocks []*Block) *Node {
	if len(blocks) == 0 {
		return nil
	}
	root := &Node{Block: blocks[0]}
	stack := []*stackFrame{newFrame(root)}

	for _, b := range blocks[1:] {
		w, hasW := parsePx(b.Props["width"])
		h, hasH := parsePx(b.Props["height"])

		if b.Order >= 0 {
			for len(stack) > 1 {
				top := stack[len(stack)-1]
				orderOK := top.expectedOrd == b.Order
				fitsOK := true
				if hasW && top.hasWidth && w > top.width+0.5 {
					fitsOK = false
				}
				if hasH && top.hasHeight && h > top.height+0.5 {
					fitsOK = false
				}
				if orderOK && fitsOK {
					break
				}
				stack = stack[:len(stack)-1]
			}
			top := stack[len(stack)-1]
			child := &Node{Block: b}
			top.node.Children = append(top.node.Children, child)
			top.expectedOrd = b.Order + 1
			if isFlexContainer(b) {
				stack = append(stack, newFrame(child))
			}
			continue
		}

		// No order marker: a plain absolutely-positioned (or otherwise
		// unordered) leaf. Attach under whatever container is currently
		// open — these are almost always decorative micro-shapes (status
		// bar glyphs, illustration internals) that get bucketed wholesale
		// by classify()/markIllustrationGroups() regardless of exact depth.
		top := stack[len(stack)-1]
		child := &Node{Block: b}
		top.node.Children = append(top.node.Children, child)
		if isFlexContainer(b) {
			stack = append(stack, newFrame(child))
		}
	}

	classify(root)
	markIllustrationGroups(root)
	return root
}

func newFrame(n *Node) *stackFrame {
	f := &stackFrame{node: n}
	if w, ok := parsePx(n.Block.Props["width"]); ok {
		f.width, f.hasWidth = w, true
	}
	if h, ok := parsePx(n.Block.Props["height"]); ok {
		f.height, f.hasHeight = h, true
	}
	return f
}

func isFlexContainer(b *Block) bool {
	return b.Props["display"] == "flex"
}

func parsePx(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if !strings.HasSuffix(s, "px") {
		return 0, false
	}
	n, err := strconv.ParseFloat(strings.TrimSuffix(s, "px"), 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

var (
	calcExprRe  = regexp.MustCompile(`calc\(([^)]*)\)`)
	calcTermRe  = regexp.MustCompile(`[+-][\d.]+(?:%|px(?:/[\d.]+)?)`)
	pxDivTermRe = regexp.MustCompile(`^([\d.]+)px/([\d.]+)$`)
	pxTermRe    = regexp.MustCompile(`^([\d.]+)px$`)
)

// resolveDimension resolves a Figma CSS length against dimension (the
// relevant parent width or height), handling a plain percentage ("50%") or
// a calc() expression. Figma commonly emits centering as
// `calc(50% - 600px/2)` — a percentage term plus one or more signed px
// terms (some divided, e.g. "600px/2" meaning half that length) — all of
// which must be summed together, not just the leading percentage, or a
// same-size-as-parent centered layer resolves to its parent's *middle*
// instead of (0,0). Returns false for a plain, non-percentage px value (the
// caller should fall back to parsePx for that case).
func resolveDimension(s string, dimension float64) (float64, bool) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "%") && !strings.Contains(s, "calc(") {
		if n, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64); err == nil {
			return n / 100 * dimension, true
		}
		return 0, false
	}

	m := calcExprRe.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	inner := strings.ReplaceAll(m[1], " ", "")
	if inner == "" {
		return 0, false
	}
	if inner[0] != '+' && inner[0] != '-' {
		inner = "+" + inner
	}
	terms := calcTermRe.FindAllString(inner, -1)
	if len(terms) == 0 {
		return 0, false
	}

	total := 0.0
	for _, t := range terms {
		sign := 1.0
		if t[0] == '-' {
			sign = -1.0
		}
		body := t[1:]
		switch {
		case strings.HasSuffix(body, "%"):
			pct, err := strconv.ParseFloat(strings.TrimSuffix(body, "%"), 64)
			if err != nil {
				return 0, false
			}
			total += sign * (pct / 100 * dimension)
		default:
			if dm := pxDivTermRe.FindStringSubmatch(body); dm != nil {
				num, _ := strconv.ParseFloat(dm[1], 64)
				den, _ := strconv.ParseFloat(dm[2], 64)
				if den == 0 {
					return 0, false
				}
				total += sign * (num / den)
			} else if pm := pxTermRe.FindStringSubmatch(body); pm != nil {
				num, _ := strconv.ParseFloat(pm[1], 64)
				total += sign * num
			} else {
				return 0, false
			}
		}
	}
	return total, true
}

var (
	statusBarNamePattern = regexp.MustCompile(`(?i)^(statusbar|status ?bar|battery|battery icon|capacity|home indicator|device bottombar|tabletlands.*|combined shape|9:41|100%|left|right)$`)
	buttonNamePattern    = regexp.MustCompile(`(?i)^(skip|next|previous|weiter|zurück|zurueck|überspringen|uberspringen|continue|done|fertig|los geht.?s?|get started|anmelden|passwort vergessen)$`)
	dotItemNamePattern   = regexp.MustCompile(`(?i)^onboarding progress dot$`)
)

// classify assigns each node a rendering role. Figma's raw export has no
// semantic markers, so this leans on naming/geometry conventions that this
// design system's screens consistently use (see plan step 3).
func classify(n *Node) {
	name := strings.TrimSpace(n.Block.Name)
	switch {
	case statusBarNamePattern.MatchString(name):
		n.Kind = KindStatusBarChrome
	// Checked before isFlexContainer: Figma sometimes puts `display: flex`
	// on a text layer too (to align its own content), but it's still text,
	// not a container of other layers.
	case n.Block.Props["font-family"] != "":
		n.Kind = KindText
	case isFlexContainer(n.Block):
		n.Kind = KindContainer
	default:
		n.Kind = KindShape
	}

	if buttonNamePattern.MatchString(name) {
		n.Kind = KindButton
	}

	// Collapse a run of repeated "Onboarding progress dot" siblings into one
	// dot-indicator group instead of emitting N hardcoded circles. Filtered
	// (not "all children match") because each dot item's own no-order fill
	// leaf ("dot") flat-attaches to this same container rather than nesting
	// under its rightful parent (see BuildTree's no-order attachment rule),
	// so the raw child list is a mix of "Onboarding progress dot" and "dot".
	// activeDotIndex uses that same raw (pre-filter) list, positionally
	// pairing each dot item with the fill-color leaf that follows it.
	if dots := filterByName(n.Children, dotItemNamePattern); len(dots) > 1 {
		n.Kind = KindDotIndicatorGroup
		n.DotCount = len(dots)
		n.DotActiveIndex, n.DotActiveColor = activeDotIndex(n.Children, dotItemNamePattern)
		n.Children = dots[:1]
	}

	for _, c := range n.Children {
		classify(c)
	}
}

// activeDotIndex finds which page-indicator dot looks "active": its fill
// color differs from the majority color shared by the other dots. It walks
// the raw, unfiltered, document-order child list (rather than each dot
// item's own Children) because a dot item's fill leaf attaches to the
// shared parent instead of nesting under its own item — see BuildTree's
// no-order attachment rule — so document order is the only reliable way to
// pair each dot item with the fill that immediately follows it.
func activeDotIndex(rawChildren []*Node, itemRe *regexp.Regexp) (int, string) {
	var colors []string
	inItem := false
	cur := ""
	flush := func() {
		if inItem {
			colors = append(colors, cur)
		}
	}
	for _, n := range rawChildren {
		if itemRe.MatchString(strings.TrimSpace(n.Block.Name)) {
			flush()
			inItem, cur = true, ""
			continue
		}
		if inItem {
			if bg := normalizeHex(n.Block.Props["background"]); bg != "" {
				cur = bg
			}
		}
	}
	flush()

	counts := map[string]int{}
	for _, c := range colors {
		counts[c]++
	}
	majority, majorityCount := "", -1
	for c, n := range counts {
		if n > majorityCount {
			majority, majorityCount = c, n
		}
	}
	for i, c := range colors {
		if c != "" && c != majority {
			return i, c
		}
	}
	return 0, majority
}

func filterByName(nodes []*Node, re *regexp.Regexp) []*Node {
	var out []*Node
	for _, n := range nodes {
		if re.MatchString(strings.TrimSpace(n.Block.Name)) {
			out = append(out, n)
		}
	}
	return out
}

// markIllustrationGroups finds containers whose entire subtree is shapes
// only (no text, buttons, or dot groups) and marks them as one opaque
// illustration unit, matching how the reference examples replace decorative
// Figma shape soup with a single generated image/SVG asset rather than a
// deeply nested SwiftUI view tree.
func markIllustrationGroups(n *Node) {
	for _, c := range n.Children {
		markIllustrationGroups(c)
	}
	if n.Kind != KindContainer {
		return
	}
	shapeCount, otherCount := 0, 0
	var count func(x *Node)
	count = func(x *Node) {
		switch x.Kind {
		case KindShape:
			shapeCount++
		case KindText, KindButton, KindDotIndicatorGroup:
			otherCount++
		case KindIllustrationGroup, KindStatusBarChrome:
			// Already resolved by an earlier (bottom-up) call: treat as an
			// opaque leaf so its internals aren't double-counted into this
			// ancestor's own tally.
		default: // still a plain KindContainer: keep descending
			for _, c := range x.Children {
				count(c)
			}
		}
	}
	count(n)
	if shapeCount >= 3 && otherCount == 0 {
		n.Kind = KindIllustrationGroup
	}
}
