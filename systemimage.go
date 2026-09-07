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
// Every keyword is \b-bounded — deliberately, even ones that look safe to
// leave loose. A real screen surfaced why: "profile" (no boundary) matched
// inside "profilemenu_onboarding", "profilemenu_faq", and
// "profilemenu_topbar_back" — a namespace prefix shared by nearly every
// layer on a "Profilemenu" screen, none of them an actual profile/avatar
// icon. Professional design systems name layers with exactly this kind of
// compound/namespaced convention, so an unbounded keyword is a real,
// recurring false-positive risk, not just a theoretical one.
var systemImageKeywords = []struct {
	pattern *regexp.Regexp
	symbol  string
}{
	{regexp.MustCompile(`(?i)\bchevron\b.*\bleft\b|\barrow\b.*\bleft\b|\bback\b`), "chevron.left"},
	{regexp.MustCompile(`(?i)\bchevron\b.*\bright\b|\barrow\b.*\bright\b|\bforward\b`), "chevron.right"},
	{regexp.MustCompile(`(?i)\bsearch\b`), "magnifyingglass"},
	{regexp.MustCompile(`(?i)\bprofile\b|\bavatar\b|\bperson\b|\baccount\b`), "person.crop.circle"},
	{regexp.MustCompile(`(?i)\bbookmark\b|\bsave\b`), "bookmark"},
	{regexp.MustCompile(`(?i)\bheadphone\b|\baudio\b|\blisten\b`), "headphones"},
	{regexp.MustCompile(`(?i)\bplay\b`), "play.fill"},
	{regexp.MustCompile(`(?i)\bpause\b`), "pause.fill"},
	{regexp.MustCompile(`(?i)\bclose\b|\bdismiss\b|\bxmark\b`), "xmark"},
	{regexp.MustCompile(`(?i)\bmenu\b|\bhamburger\b`), "line.3.horizontal"},
	{regexp.MustCompile(`(?i)\bsettings\b|\bgear\b|\bcog\b`), "gearshape"},
	{regexp.MustCompile(`(?i)\bstar\b|\bfavorite\b|\brating\b`), "star"},
	{regexp.MustCompile(`(?i)\bheart\b|\blike\b`), "heart"},
	{regexp.MustCompile(`(?i)\bcheckmark\b|\bcheck\b|\bdone\b|\bsuccess\b`), "checkmark"},
	{regexp.MustCompile(`(?i)\bcalendar\b|\bdate\b`), "calendar"},
	{regexp.MustCompile(`(?i)\bshare\b`), "square.and.arrow.up"},
	{regexp.MustCompile(`(?i)\bmore\b|\bellipsis\b|\bdots\b`), "ellipsis"},
	{regexp.MustCompile(`(?i)\bgrid\b`), "square.grid.2x2"},
	{regexp.MustCompile(`(?i)\bhome\b`), "house"},
	{regexp.MustCompile(`(?i)\btrash\b|\bdelete\b|\bremove\b`), "trash"},
	{regexp.MustCompile(`(?i)\bdownload\b`), "arrow.down.circle"},
	{regexp.MustCompile(`(?i)\bupload\b`), "arrow.up.circle"},
	{regexp.MustCompile(`(?i)\bnotification\b|\bbell\b|\balert\b`), "bell"},
	{regexp.MustCompile(`(?i)\block\b|\bsecure\b|\bpassword\b`), "lock"},
	{regexp.MustCompile(`(?i)\bmail\b|\bemail\b|\benvelope\b`), "envelope"},
	{regexp.MustCompile(`(?i)\bphone\b|\bcall\b`), "phone"},
	{regexp.MustCompile(`(?i)\bcamera\b|\bphoto\b`), "camera"},
	{regexp.MustCompile(`(?i)\bvideo\b|\bfilm\b|\bmovie\b`), "video"},
	{regexp.MustCompile(`(?i)\blocation\b|\bpin\b|\bmap\b`), "mappin.and.ellipse"},
	{regexp.MustCompile(`(?i)\bcart\b|\bbasket\b|\bshop\b`), "cart"},
	{regexp.MustCompile(`(?i)\bfilter\b`), "line.3.horizontal.decrease.circle"},
	{regexp.MustCompile(`(?i)\brefresh\b|\breload\b|\bsync\b`), "arrow.clockwise"},
	{regexp.MustCompile(`(?i)\binfo\b`), "info.circle"},
	{regexp.MustCompile(`(?i)\bwarning\b|\bcaution\b`), "exclamationmark.triangle"},
	{regexp.MustCompile(`(?i)\bplus\b|\badd\b`), "plus"},
	{regexp.MustCompile(`(?i)\bminus\b|\bsubtract\b`), "minus"},
	{regexp.MustCompile(`(?i)\bedit\b|\bpencil\b`), "pencil"},
	{regexp.MustCompile(`(?i)\beye\b`), "eye"},
	{regexp.MustCompile(`(?i)\blink\b`), "link"},
	{regexp.MustCompile(`(?i)\bfolder\b`), "folder"},
	{regexp.MustCompile(`(?i)\bdocument\b|\bfile\b`), "doc"},
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
		// Status-bar chrome is never rendered at all (see codegen's own
		// KindStatusBarChrome handling) — classifying and cropping an icon
		// inside it (a battery/wifi/signal glyph) would just waste an
		// ImageMagick call and leave an unreferenced asset in the catalog.
		if n.Kind == KindStatusBarChrome {
			return
		}
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
