// figswiftui generates a SwiftUI screen + Xcode Assets.xcassets from a
// Figma "copy as CSS" export and/or a screenshot. Fully offline/deterministic
// — no AI or network calls at runtime.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// boolFlagNames lists this program's boolean flags (those that don't
// consume a following value token, e.g. "-allow-duplicates" not
// "-allow-duplicates true") — reorderArgs needs to know these to correctly
// tell a flag's value apart from the next positional argument. Keep this in
// sync with any new flag.Bool(...) added to main().
var boolFlagNames = map[string]bool{"allow-duplicates": true}

// reorderArgs splits args into flags (with their values, in original
// relative order) and bare positional arguments, so that gluing
// flagArgs+positionals back together and handing that to flag.Parse always
// parses cleanly regardless of the order the user actually typed them in —
// Go's flag package otherwise stops parsing at the first non-flag argument.
func reorderArgs(args []string, boolFlags map[string]bool) (flagArgs, positionals []string) {
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			positionals = append(positionals, a)
			i++
			continue
		}
		flagArgs = append(flagArgs, a)
		name := strings.TrimLeft(a, "-") // e.g. "--allow-duplicates" -> "allow-duplicates"
		takesNoValue := strings.Contains(name, "=") || boolFlags[name]
		if !takesNoValue && i+1 < len(args) {
			flagArgs = append(flagArgs, args[i+1])
			i += 2
			continue
		}
		i++
	}
	return
}

func main() {
	cssPath := flag.String("css", "", "path to a Figma \"copy as CSS\" export (plain text .css)\n(usually unnecessary — just pass the file directly, e.g. `figswiftui onboarding.css`)")
	screenshotPath := flag.String("screenshot", "", "path to a PNG/JPEG screenshot of the screen\n(usually unnecessary — just pass the file directly)")
	name := flag.String("name", "", "output bundle folder name, e.g. Customer-TICKET-123\n(optional: derived from content if omitted; an optional shared prefix in batch mode)")
	screenName := flag.String("screen-name", "", "Swift struct/file name (derived from content if omitted;\nignored in batch mode, always derived per screen there)")
	out := flag.String("out", "output", "output root directory")
	matchProject := flag.String("match-project", "", "optional path to an existing Xcode project; extracted colors that\nexactly match an existing *.colorset are renamed to reuse it")
	batchDir := flag.String("batch", "", "process a directory of screen pairs\n(usually unnecessary — just pass the directory directly)")
	allowDuplicates := flag.Bool("allow-duplicates", false, "in batch mode, generate every screen even when its .css is\nbyte-identical to an earlier one, or its screenshot is a near-duplicate\n(default: skip duplicates and warn)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "figswiftui — generate a SwiftUI screen + Assets.xcassets from a Figma CSS\nexport and/or a screenshot. Fully offline: no AI, no network calls.\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n  figswiftui <file.css>\n  figswiftui <file.css> <screenshot.png>\n  figswiftui <screenshot.png>\n  figswiftui <directory>\n\n")
		fmt.Fprintf(os.Stderr, "Just pass the file(s) or a directory as plain arguments, in any order —\nfigswiftui tells a .css from a screenshot by extension, and a directory means\nbatch mode (each <basename>.css inside it, optionally paired with a\nsame-basename .png/.jpg/.jpeg, becomes its own output bundle). --name is\noptional everywhere; a sensible one is derived from the content when omitted.\n\nFlags (rarely needed — mainly for scripting or when a path's extension\ndoesn't match its real type):\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nNote: --css must be plain-text (a real Figma \"copy as CSS\" paste). Screenshot-only\ninput (no CSS) produces a color/dimension skeleton with real recognized text\n(via tesseract OCR, if installed — `brew install tesseract`) in place of a\nplaceholder; full VStack/HStack structure and spacing still needs a companion\nCSS export.\n\nIn batch mode, a .css byte-identical to one already processed, or a screenshot\nthat's a near-duplicate of one already processed, is skipped with a warning —\npass --allow-duplicates to generate it anyway.\n")
	}
	// Go's flag package stops parsing at the first non-flag argument, so
	// "figswiftui screens/ --out X" would silently misparse "--out" and "X"
	// as extra positional files instead of a flag. Reordering flags (with
	// their values) before positionals lets them appear in any order, the
	// way a user reaching for a plain "just pass the file" CLI would expect.
	flagArgs, positionals := reorderArgs(os.Args[1:], boolFlagNames)
	if err := flag.CommandLine.Parse(flagArgs); err != nil {
		os.Exit(2)
	}
	for _, arg := range positionals {
		info, statErr := os.Stat(arg)
		if statErr != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", arg, statErr)
			os.Exit(2)
		}
		switch {
		case info.IsDir():
			if *batchDir == "" {
				*batchDir = arg
			}
		case strings.EqualFold(filepath.Ext(arg), ".css"):
			if *cssPath == "" {
				*cssPath = arg
			}
		case isScreenshotExt(filepath.Ext(arg)):
			if *screenshotPath == "" {
				*screenshotPath = arg
			}
		default:
			fmt.Fprintf(os.Stderr, "error: %s: unrecognized file type (expected .css, .png/.jpg/.jpeg, or a directory)\n", arg)
			os.Exit(2)
		}
	}

	if *batchDir != "" {
		if *cssPath != "" || *screenshotPath != "" {
			fmt.Fprintln(os.Stderr, "error: a directory can't be combined with a .css or screenshot input")
			os.Exit(2)
		}
		if *screenName != "" {
			fmt.Fprintln(os.Stderr, "warning: --screen-name is ignored in batch mode (derived per screen)")
		}
		if err := runBatch(*batchDir, *name, *out, *matchProject, *allowDuplicates); err != nil {
			fatalf("%v", err)
		}
		return
	}

	if *cssPath == "" && *screenshotPath == "" {
		flag.Usage()
		os.Exit(2)
	}

	swiftPath, assetsPath, err := generateScreen(*cssPath, *screenshotPath, *name, *screenName, *out, *matchProject)
	if err != nil {
		fatalf("%v", err)
	}
	fmt.Printf("Wrote %s\n", swiftPath)
	fmt.Printf("Wrote %s\n", assetsPath)
}

var screenshotExts = []string{".png", ".jpg", ".jpeg"}

func isScreenshotExt(ext string) bool {
	ext = strings.ToLower(ext)
	for _, e := range screenshotExts {
		if ext == e {
			return true
		}
	}
	return false
}

// batchCandidate is one <basename>.css (+ optional screenshot) pair
// discovered by runBatch, before duplicate detection runs.
type batchCandidate struct {
	cssFile        string
	cssPath        string
	screenshotPath string
	bundleName     string
}

// runBatch scans dir (top-level only) for *.css files and, for each one,
// generates a screen bundle named "<namePrefix>-<basename>" (or just
// "<basename>" when namePrefix is empty), pairing it with a same-basename
// screenshot if one exists alongside it. One failing screen is reported and
// skipped rather than aborting the whole batch. Duplicate inputs — a .css
// byte-identical to one already seen, or a screenshot that's a
// near-duplicate of one already seen — are detected and skipped (with a
// warning) unless allowDuplicates is set.
func runBatch(dir, namePrefix, out, matchProject string, allowDuplicates bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading --batch directory: %w", err)
	}

	var candidates []batchCandidate
	for _, e := range entries {
		if e.IsDir() || strings.ToLower(filepath.Ext(e.Name())) != ".css" {
			continue
		}
		base := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		cssPath := filepath.Join(dir, e.Name())

		screenshotPath := ""
		for _, ext := range screenshotExts {
			candidate := filepath.Join(dir, base+ext)
			if _, err := os.Stat(candidate); err == nil {
				screenshotPath = candidate
				break
			}
		}

		bundleName := base
		if namePrefix != "" {
			bundleName = namePrefix + "-" + base
		}
		candidates = append(candidates, batchCandidate{e.Name(), cssPath, screenshotPath, bundleName})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].cssFile < candidates[j].cssFile })
	if len(candidates) == 0 {
		return fmt.Errorf("no .css files found in %s", dir)
	}

	okCount, failCount, skipCount := 0, 0, 0
	seenCSSHash := map[string]string{} // sha256 -> first filename
	var seenScreenshots []screenshotHashEntry

	for _, c := range candidates {
		if !allowDuplicates {
			if dupOf, isDup, err := detectDuplicate(c, seenCSSHash, seenScreenshots); err != nil {
				fmt.Fprintf(os.Stderr, "warning: %s: could not check for duplicates: %v\n", c.cssFile, err)
			} else if isDup {
				fmt.Printf("%s -> skipped (duplicate of %s; pass --allow-duplicates to generate anyway)\n", c.cssFile, dupOf)
				skipCount++
				continue
			}
		}
		if sha, err := fileSHA256(c.cssPath); err == nil {
			if _, exists := seenCSSHash[sha]; !exists {
				seenCSSHash[sha] = c.cssFile
			}
		}
		if c.screenshotPath != "" {
			if img, err := loadImageForHash(c.screenshotPath); err == nil {
				seenScreenshots = append(seenScreenshots, screenshotHashEntry{c.cssFile, imageDHash(img)})
			}
		}

		swiftPath, assetsPath, err := generateScreen(c.cssPath, c.screenshotPath, c.bundleName, "", out, matchProject)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", c.cssFile, err)
			failCount++
			continue
		}
		suffix := ""
		if c.screenshotPath != "" {
			suffix = " (+ " + filepath.Base(c.screenshotPath) + ")"
		}
		fmt.Printf("%s%s -> %s, %s\n", c.cssFile, suffix, swiftPath, assetsPath)
		okCount++
	}

	fmt.Printf("\n%d screen(s) generated, %d skipped as duplicates, %d failed\n", okCount, skipCount, failCount)
	if failCount > 0 {
		return fmt.Errorf("%d of %d screens failed", failCount, len(candidates))
	}
	return nil
}

// screenshotHashEntry records a previously-seen batch candidate's screenshot
// perceptual hash, keyed by its .css filename, for near-duplicate detection.
type screenshotHashEntry struct {
	name string
	hash uint64
}

// detectDuplicate reports whether c's .css is byte-identical to an earlier
// candidate's, or its screenshot is a near-duplicate (small dHash Hamming
// distance) of an earlier candidate's, and if so which one.
func detectDuplicate(c batchCandidate, seenCSSHash map[string]string, seenScreenshots []screenshotHashEntry) (dupOf string, isDup bool, err error) {
	sha, err := fileSHA256(c.cssPath)
	if err != nil {
		return "", false, err
	}
	if prior, ok := seenCSSHash[sha]; ok {
		return prior + " (identical .css)", true, nil
	}
	if c.screenshotPath == "" {
		return "", false, nil
	}
	img, err := loadImageForHash(c.screenshotPath)
	if err != nil {
		return "", false, err
	}
	h := imageDHash(img)
	for _, prior := range seenScreenshots {
		if hammingDistance(h, prior.hash) <= dHashThreshold {
			return prior.name + " (near-identical screenshot)", true, nil
		}
	}
	return "", false, nil
}

// generateScreen runs the full pipeline for one screen: parse/build the
// layout tree from cssPath and/or screenshotPath, collect and name colors,
// extract any illustration, generate the Swift source, and write the
// bundle (Screen/<name>.swift + Assets.xcassets) under out/bundleName.
// Returns the paths written on success.
func generateScreen(cssPath, screenshotPath, bundleName, screenNameOverride, out, matchProject string) (swiftPath, assetsPath string, err error) {
	var root *Node
	var screenshot *ScreenshotInfo

	if cssPath != "" {
		data, readErr := os.ReadFile(cssPath)
		if readErr != nil {
			return "", "", fmt.Errorf("reading --css: %w", readErr)
		}
		blocks := ParseBlocks(string(data))
		if len(blocks) == 0 {
			return "", "", fmt.Errorf("no layer blocks found in %s — is this a plain-text Figma \"copy as CSS\" export?", cssPath)
		}
		root = BuildTree(blocks)
	}

	if screenshotPath != "" {
		s, loadErr := LoadScreenshot(screenshotPath)
		if loadErr != nil {
			return "", "", fmt.Errorf("reading --screenshot: %w", loadErr)
		}
		screenshot = s
		top, bottom := statusBarRatios(root)
		if root == nil {
			top, bottom = 0.03, 0.02
		}
		screenshot.applyCrop(top, bottom)
	}

	if root == nil {
		root = skeletonFromScreenshot(screenshot)
	} else if screenshot != nil {
		// Both a CSS export and a screenshot were given: CSS still drives
		// structure/colors, but OCR on the screenshot recovers real text —
		// componentized button labels Figma's export only names generically
		// (e.g. "Skip"/"Next"), and template placeholder copy that may be
		// stale relative to the real, localized instance shown on screen.
		if ocrLines, ocrErr := recognizeTextFromBytes(screenshot.CroppedPNG); ocrErr == nil {
			applyOCRText(root, ocrLines)
		} else {
			fmt.Fprintf(os.Stderr, "warning: OCR unavailable for text verification (%v)\n", ocrErr)
		}
	}

	// Small icon-like shapes get real icon treatment: an SF Symbol when the
	// Figma layer name matches a recognizable keyword (no screenshot
	// needed), or — failing that, when a screenshot is available — a
	// direct ImageMagick crop of the real rendered pixels at that shape's
	// position, closer to pixel-perfect than a mechanical reconstruction.
	applySystemImages(root, screenshotPath)

	resolvedScreenName := screenNameOverride
	if resolvedScreenName == "" {
		resolvedScreenName = deriveScreenName(root)
	}
	if bundleName == "" {
		bundleName = resolvedScreenName
	}

	colors := CollectColors(root)
	if matchProject != "" {
		matcher, matchErr := loadProjectColors(matchProject)
		if matchErr != nil {
			fmt.Fprintf(os.Stderr, "warning: --match-project: %v\n", matchErr)
		} else {
			applyProjectColorNames(colors, matcher)
		}
	}

	illustrations := map[string]string{}
	if illustration := findIllustrationGroup(root); illustration != nil {
		imgName := sanitizeIdentifier(illustration.Block.Name, true) + "Illustration"
		illustrations[imgName] = RenderIllustrationSVG(illustration)
	}
	pngIcons := map[string][]byte{}
	collectCroppedIcons(root, pngIcons)

	swiftSrc := GenerateSwift(root, resolvedScreenName, colors)

	bundleRoot := filepath.Join(out, bundleName)
	screenDir := filepath.Join(bundleRoot, "Screen")
	if mkErr := os.MkdirAll(screenDir, 0o755); mkErr != nil {
		return "", "", fmt.Errorf("creating output directory: %w", mkErr)
	}
	swiftPath = filepath.Join(screenDir, resolvedScreenName+".swift")
	if writeErr := os.WriteFile(swiftPath, []byte(swiftSrc), 0o644); writeErr != nil {
		return "", "", fmt.Errorf("writing Swift file: %w", writeErr)
	}

	var screenshotPNG []byte
	if screenshot != nil {
		screenshotPNG = screenshot.CroppedPNG
	}
	screenshotAssetName := resolvedScreenName + "Reference"

	if assetErr := WriteAssetsCatalog(bundleRoot, colors, illustrations, pngIcons, screenshotPNG, screenshotAssetName); assetErr != nil {
		return "", "", fmt.Errorf("writing Assets.xcassets: %w", assetErr)
	}
	assetsPath = filepath.Join(bundleRoot, "Assets.xcassets")

	return swiftPath, assetsPath, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}

func collectCroppedIcons(n *Node, out map[string][]byte) {
	if n.Kind == KindSystemImage && len(n.CroppedImageData) > 0 && n.CroppedAssetName != "" {
		out[n.CroppedAssetName] = n.CroppedImageData
	}
	for _, c := range n.Children {
		collectCroppedIcons(c, out)
	}
}

func findIllustrationGroup(n *Node) *Node {
	if n.Kind == KindIllustrationGroup {
		return n
	}
	for _, c := range n.Children {
		if g := findIllustrationGroup(c); g != nil {
			return g
		}
	}
	return nil
}

// skeletonFromScreenshot builds a placeholder tree for screenshot-only
// input (no companion CSS). Real layout/structure understanding still needs
// a CSS export — no amount of image analysis recovers VStack/HStack
// nesting or spacing from pixels alone — but real *text* is recoverable via
// OCR (tesseract, if installed) instead of a "TODO" placeholder. Lines are
// ordered top-to-bottom by their detected position; each line's own
// detected height stands in for font-size so heading-vs-body text still
// gets a plausible relative size via the same swiftFont() bucketing CSS
// input uses.
func skeletonFromScreenshot(s *ScreenshotInfo) *Node {
	root := &Node{
		Block: &Block{Name: "Screen", Props: map[string]string{
			"display": "flex", "flex-direction": "column",
		}},
		Kind: KindContainer,
	}

	var ocrLines []OCRLine
	var ocrErr error
	if s != nil && len(s.CroppedPNG) > 0 {
		ocrLines, ocrErr = recognizeTextFromBytes(s.CroppedPNG)
	}

	switch {
	case len(ocrLines) > 0:
		// A line's raw OCR pixel height doesn't mean anything on its own —
		// it depends entirely on the screenshot's resolution/scale, which
		// is why using it directly as a "font-size" produced nonsense (body
		// copy reading as a bold title). Scaling each line relative to the
		// *median* detected line height — a reasonable proxy for "typical
		// body text" in any screenshot — and mapping that ratio onto a
		// plausible 16px body baseline gives swiftFont()'s existing
		// px-threshold bucketing something calibrated to work with.
		medianH := medianLineHeight(ocrLines)
		for _, line := range ocrLines {
			fontSize := 16.0
			if medianH > 0 {
				fontSize = float64(line.H) / medianH * 16
			}
			weight := "400"
			if fontSize >= 22 {
				weight = "700" // notably larger than typical body text reads as a heading
			}
			root.Children = append(root.Children, &Node{
				Block: &Block{Name: line.Text, Props: map[string]string{
					"font-family": "System",
					"font-size":   fmt.Sprintf("%.0fpx", fontSize),
					"font-weight": weight,
				}},
				Kind: KindText,
			})
		}
	default:
		name := "TODO: extract heading text from the screenshot"
		if ocrErr != nil {
			fmt.Fprintf(os.Stderr, "warning: OCR unavailable (%v) — using a placeholder instead of real text\n", ocrErr)
		}
		root.Children = append(root.Children, &Node{
			Block: &Block{Name: name, Props: map[string]string{
				"font-family": "System", "font-size": "24px", "font-weight": "700",
			}},
			Kind: KindText,
		})
	}

	if s != nil && len(s.DominantColors) > 0 {
		root.Block.Props["background"] = s.DominantColors[0]
	}
	return root
}

func medianLineHeight(lines []OCRLine) float64 {
	if len(lines) == 0 {
		return 0
	}
	heights := make([]int, len(lines))
	for i, l := range lines {
		heights[i] = l.H
	}
	sort.Ints(heights)
	mid := len(heights) / 2
	if len(heights)%2 == 0 {
		return float64(heights[mid-1]+heights[mid]) / 2
	}
	return float64(heights[mid])
}

// deriveScreenName picks a PascalCase struct/file name from the first
// meaningful text node's content, falling back to the root layer's own name.
func deriveScreenName(root *Node) string {
	var found string
	var walk func(n *Node)
	walk = func(n *Node) {
		if found != "" || n.Kind == KindStatusBarChrome {
			return
		}
		if n.Kind == KindText {
			if t := textOf(n); t != "" && t != "TODO" {
				found = t
				return
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)

	base := found
	if base == "" {
		base = root.Block.Name
	}
	words := strings.Fields(nonAlnumRe.ReplaceAllString(base, " "))
	if len(words) > 4 {
		words = words[:4]
	}
	var name strings.Builder
	for _, w := range words {
		name.WriteString(strings.ToUpper(w[:1]) + strings.ToLower(w[1:]))
	}
	if name.Len() == 0 {
		return "GeneratedScreen"
	}
	return name.String() + "Screen"
}
