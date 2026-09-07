package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// maxSystemImageSize caps how large a shape can be and still be considered
// for icon-style treatment (SF Symbol or a cropped-screenshot fallback) — a
// large decorative background shape that happens to share a keyword with a
// real icon (e.g. a "Star Decoration" backdrop blob) shouldn't be
// reclassified just because of its name.
const maxSystemImageSize = 60.0

// systemImageKeywords maps common icon-name keywords in Figma layer names
// to their closest SF Symbol equivalent. A small, named icon-like shape
// renders as a real system symbol — scales cleanly, matches the OS's own
// icon style, needs no exported asset at all — instead of a mechanically
// reconstructed shape approximation.
var systemImageKeywords = []struct {
	pattern *regexp.Regexp
	symbol  string
}{
	{regexp.MustCompile(`(?i)chevron.*left|arrow.*left|\bback\b`), "chevron.left"},
	{regexp.MustCompile(`(?i)chevron.*right|arrow.*right|\bforward\b`), "chevron.right"},
	{regexp.MustCompile(`(?i)\bsearch\b`), "magnifyingglass"},
	{regexp.MustCompile(`(?i)profile|avatar|\bperson\b|account`), "person.crop.circle"},
	{regexp.MustCompile(`(?i)bookmark|\bsave\b`), "bookmark"},
	{regexp.MustCompile(`(?i)headphone|\baudio\b|\blisten\b`), "headphones"},
	{regexp.MustCompile(`(?i)\bplay\b`), "play.fill"},
	{regexp.MustCompile(`(?i)\bpause\b`), "pause.fill"},
	{regexp.MustCompile(`(?i)\bclose\b|dismiss|xmark`), "xmark"},
	{regexp.MustCompile(`(?i)\bmenu\b|hamburger`), "line.3.horizontal"},
	{regexp.MustCompile(`(?i)settings|\bgear\b|\bcog\b`), "gearshape"},
	{regexp.MustCompile(`(?i)\bstar\b|favorite|rating`), "star"},
	{regexp.MustCompile(`(?i)\bheart\b|\blike\b`), "heart"},
	{regexp.MustCompile(`(?i)checkmark|\bcheck\b|\bdone\b|success`), "checkmark"},
	{regexp.MustCompile(`(?i)calendar|\bdate\b`), "calendar"},
	{regexp.MustCompile(`(?i)\bshare\b`), "square.and.arrow.up"},
	{regexp.MustCompile(`(?i)\bmore\b|ellipsis|\bdots\b`), "ellipsis"},
	{regexp.MustCompile(`(?i)\bgrid\b`), "square.grid.2x2"},
	{regexp.MustCompile(`(?i)\bhome\b`), "house"},
	{regexp.MustCompile(`(?i)trash|delete|remove`), "trash"},
	{regexp.MustCompile(`(?i)download`), "arrow.down.circle"},
	{regexp.MustCompile(`(?i)upload`), "arrow.up.circle"},
	{regexp.MustCompile(`(?i)notification|\bbell\b|\balert\b`), "bell"},
	{regexp.MustCompile(`(?i)\block\b|secure|password`), "lock"},
	{regexp.MustCompile(`(?i)\bmail\b|email|envelope`), "envelope"},
	{regexp.MustCompile(`(?i)\bphone\b|\bcall\b`), "phone"},
	{regexp.MustCompile(`(?i)camera|\bphoto\b`), "camera"},
	{regexp.MustCompile(`(?i)\bvideo\b|film|movie`), "video"},
	{regexp.MustCompile(`(?i)location|\bpin\b|\bmap\b`), "mappin.and.ellipse"},
	{regexp.MustCompile(`(?i)\bcart\b|basket|\bshop\b`), "cart"},
	{regexp.MustCompile(`(?i)\bfilter\b`), "line.3.horizontal.decrease.circle"},
	{regexp.MustCompile(`(?i)refresh|reload|\bsync\b`), "arrow.clockwise"},
	{regexp.MustCompile(`(?i)\binfo\b`), "info.circle"},
	{regexp.MustCompile(`(?i)warning|caution`), "exclamationmark.triangle"},
	{regexp.MustCompile(`(?i)\bplus\b|\badd\b`), "plus"},
	{regexp.MustCompile(`(?i)\bminus\b|subtract`), "minus"},
	{regexp.MustCompile(`(?i)\bedit\b|pencil`), "pencil"},
	{regexp.MustCompile(`(?i)\beye\b`), "eye"},
	{regexp.MustCompile(`(?i)\blink\b`), "link"},
	{regexp.MustCompile(`(?i)\bfolder\b`), "folder"},
	{regexp.MustCompile(`(?i)document|\bfile\b`), "doc"},
}

var wordSeparatorRe = regexp.MustCompile(`[_\-./]`)

func matchSystemImage(name string) (string, bool) {
	// Icon asset names are almost always snake_case, kebab-case, or a
	// path-like slug ("icon/search", "ic_bookmark_24") rather than plain
	// space-separated words — but "_"/"-" count as word characters in Go's
	// regexp, so a \b boundary never actually falls between "search" and
	// "_icon". Normalizing separators to spaces first lets every pattern's
	// \b keyword boundaries work the way they read.
	normalized := wordSeparatorRe.ReplaceAllString(name, " ")
	for _, m := range systemImageKeywords {
		if m.pattern.MatchString(normalized) {
			return m.symbol, true
		}
	}
	return "", false
}

// applySystemImages walks the tree for small (<=maxSystemImageSize) shape
// nodes not already absorbed into an illustration group and gives each one
// real icon treatment instead of a flat color rect: an SF Symbol when its
// Figma layer name matches a recognizable icon keyword (works for any
// input, no screenshot needed), or — when no keyword matches but a
// companion screenshot is available and the shape's own position:absolute
// left/top/width/height resolve to a real pixel region — a direct crop of
// that region from the actual screenshot via ImageMagick. That crop is the
// "use system images if not available" fallback in its literal sense: real
// rendered pixels instead of a mechanical shape reconstruction, closer to
// pixel-perfect than this tool can get by redrawing.
func applySystemImages(root *Node, screenshotPath string) {
	var walk func(n *Node)
	walk = func(n *Node) {
		if n.Kind == KindShape {
			tryApplySystemImage(n, root, screenshotPath)
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
}

func tryApplySystemImage(n, root *Node, screenshotPath string) {
	w, wOk := parsePx(n.Block.Props["width"])
	h, hOk := parsePx(n.Block.Props["height"])
	if !wOk || !hOk || w > maxSystemImageSize || h > maxSystemImageSize {
		return
	}

	if symbol, ok := matchSystemImage(strings.TrimSpace(n.Block.Name)); ok {
		n.Kind = KindSystemImage
		n.SystemImageName = symbol
		return
	}

	if screenshotPath == "" || !imageMagickAvailable() {
		return
	}
	x, y, cw, ch, ok := resolveAbsolutePosition(n, root)
	if !ok {
		return
	}
	data, err := cropRegionViaImageMagick(screenshotPath, int(x), int(y), int(cw), int(ch))
	if err != nil {
		return
	}
	n.Kind = KindSystemImage
	n.CroppedImageData = data
	n.CroppedAssetName = sanitizeIdentifier(n.Block.Name, true) + "Icon"
}

// resolveAbsolutePosition returns a shape's approximate absolute pixel
// position, resolved against the tree's root dimensions. Not a real
// layout-position resolver — CSS flow-layout nodes don't carry absolute
// coordinates anywhere in this tool — so this only applies to a node
// that's itself position:absolute with resolvable left/top/width/height;
// everything else reports ok=false and is left as a plain colored shape.
func resolveAbsolutePosition(n, root *Node) (x, y, w, h float64, ok bool) {
	if n.Block.Props["position"] != "absolute" {
		return 0, 0, 0, 0, false
	}
	rootW, wOk := parsePx(root.Block.Props["width"])
	rootH, hOk := parsePx(root.Block.Props["height"])
	if !wOk || !hOk {
		return 0, 0, 0, 0, false
	}
	x, y, w, h = resolveBox(n.Block.Props, rootW, rootH)
	if w <= 0 || h <= 0 {
		return 0, 0, 0, 0, false
	}
	return x, y, w, h, true
}

func imageMagickAvailable() bool {
	_, err := exec.LookPath("magick")
	return err == nil
}

// cropRegionViaImageMagick shells out to the `magick` CLI (ImageMagick;
// install with `brew install imagemagick`) to crop a precise pixel region
// out of the original screenshot file. Cropping is the kind of heavyweight
// pixel-manipulation job better delegated to a real, battle-tested
// image-processing tool than hand-rolled with Go's stdlib image package.
func cropRegionViaImageMagick(screenshotPath string, x, y, w, h int) ([]byte, error) {
	if !imageMagickAvailable() {
		return nil, fmt.Errorf("ImageMagick (`magick`) not found on PATH — install it with `brew install imagemagick` for pixel-cropped icon fallback")
	}
	geometry := fmt.Sprintf("%dx%d+%d+%d", w, h, x, y)
	cmd := exec.Command("magick", screenshotPath, "-crop", geometry, "+repage", "PNG24:-")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("cropping region via ImageMagick: %s", msg)
		}
		return nil, fmt.Errorf("cropping region via ImageMagick: %w", err)
	}
	return out, nil
}
