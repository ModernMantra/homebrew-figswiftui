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

func main() {
	cssPath := flag.String("css", "", "path to a Figma \"copy as CSS\" export (plain text .css)")
	screenshotPath := flag.String("screenshot", "", "path to a PNG/JPEG screenshot of the screen")
	name := flag.String("name", "", "output bundle folder name, e.g. Customer-TICKET-123\n(required in single-file mode; an optional shared prefix in --batch mode)")
	screenName := flag.String("screen-name", "", "Swift struct/file name (derived from content if omitted;\nignored in --batch mode, always derived per screen there)")
	out := flag.String("out", "output", "output root directory")
	matchProject := flag.String("match-project", "", "optional path to an existing Xcode project; extracted colors that\nexactly match an existing *.colorset are renamed to reuse it")
	batchDir := flag.String("batch", "", "process a directory of screen pairs instead of a single --css/--screenshot:\neach <basename>.css (optionally with a same-basename .png/.jpg/.jpeg\nscreenshot alongside it) becomes its own output bundle")
	allowDuplicates := flag.Bool("allow-duplicates", false, "in --batch mode, generate every screen even when its .css is\nbyte-identical to an earlier one, or its screenshot is a near-duplicate\n(default: skip duplicates and warn)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "figswiftui — generate a SwiftUI screen + Assets.xcassets from a Figma CSS\nexport and/or a screenshot. Fully offline: no AI, no network calls.\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n  figswiftui --css <file> [--screenshot <file>] --name <bundle-name> [flags]\n  figswiftui --screenshot <file> --name <bundle-name> [flags]\n  figswiftui --batch <dir> [--name <prefix>] [flags]\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nNote: --css must be plain-text (a real Figma \"copy as CSS\" paste). Screenshot-only\ninput (no --css) produces a lower-fidelity color/dimension skeleton with placeholder\ntext — full structure and text extraction needs a companion CSS export (no OCR is\nperformed).\n\nIn --batch mode, a .css byte-identical to one already processed, or a screenshot\nthat's a near-duplicate of one already processed, is skipped with a warning —\npass --allow-duplicates to generate it anyway.\n")
	}
	flag.Parse()

	if *batchDir != "" {
		if *cssPath != "" || *screenshotPath != "" {
			fmt.Fprintln(os.Stderr, "error: --batch cannot be combined with --css/--screenshot")
			os.Exit(2)
		}
		if *screenName != "" {
			fmt.Fprintln(os.Stderr, "warning: --screen-name is ignored in --batch mode (derived per screen)")
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
	if *name == "" {
		fmt.Fprintln(os.Stderr, "error: --name is required")
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
	}

	resolvedScreenName := screenNameOverride
	if resolvedScreenName == "" {
		resolvedScreenName = deriveScreenName(root)
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

	if assetErr := WriteAssetsCatalog(bundleRoot, colors, illustrations, screenshotPNG, screenshotAssetName); assetErr != nil {
		return "", "", fmt.Errorf("writing Assets.xcassets: %w", assetErr)
	}
	assetsPath = filepath.Join(bundleRoot, "Assets.xcassets")

	return swiftPath, assetsPath, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
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

// skeletonFromScreenshot builds a minimal placeholder tree for
// screenshot-only input (no companion CSS). No OCR/vision is performed, so
// this intentionally cannot recover real structure or text — it surfaces a
// clearly-marked TODO rather than silently emitting wrong content.
func skeletonFromScreenshot(s *ScreenshotInfo) *Node {
	root := &Node{
		Block: &Block{Name: "Screen", Props: map[string]string{
			"display": "flex", "flex-direction": "column",
		}},
		Kind: KindContainer,
	}
	title := &Node{
		Block: &Block{Name: "TODO: extract heading text from the screenshot", Props: map[string]string{
			"font-family": "System", "font-size": "24px", "font-weight": "700",
		}},
		Kind: KindText,
	}
	root.Children = append(root.Children, title)
	if s != nil && len(s.DominantColors) > 0 {
		root.Block.Props["background"] = s.DominantColors[0]
	}
	return root
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
