package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// OCRLine is one line of text recognized in a screenshot, with its
// pixel bounding box (relative to the image passed to tesseract).
type OCRLine struct {
	Text       string
	X, Y, W, H int
}

// applyOCRText matches OCR-recognized text to every text-bearing node in
// the CSS-derived tree (buttons and general text), in document order,
// against OCR paragraphs in reading order. OCR reflects what's actually
// rendered on screen — which a Figma CSS export often can't give reliably:
// a componentized button only exposes its component name ("Skip"/"Next"),
// and a template's placeholder copy ("Welcome to your Modular App!") can be
// stale relative to a real, localized instance. So OCR is preferred
// whenever a plausible match is found, not just used to patch obvious
// gaps.
//
// There's no real layout-position resolver behind this — CSS flow-layout
// nodes don't carry absolute coordinates to match against OCR pixel
// positions — so this is order correlation, not geometry. Buttons are
// matched first, restricted to short (<=2 word) candidates — a heading
// that happens to be short shouldn't be mistaken for a button label, and
// word count alone reliably separates the two in practice (a height cutoff
// was tried too, but a handful of OCR lines is too small a sample for
// "median line height" to mean much, and it excluded genuine short button
// text more often than it excluded anything wrongly matched); general text
// nodes then take whatever candidates remain, in order. Every match is
// marked OCRVerified so the generated comment stays honest about it being
// a best-effort correlation, not a guarantee.
func applyOCRText(root *Node, ocrLines []OCRLine) {
	var nodes []*Node
	var collect func(n *Node)
	collect = func(n *Node) {
		// Status-bar chrome is never rendered at all (see codegen's own
		// KindStatusBarChrome handling) — a node nested under it, like a
		// status-bar clock's own text, must not compete for a text-node's
		// OCR slot. A button's own children (e.g. a componentized
		// "Placeholder" label) are also skipped here: the button itself
		// already gets matched in pass 1, and writeNode never renders a
		// button's children as independent Text elements, so matching one
		// would just waste a candidate a real text node could have used.
		if n.Kind == KindStatusBarChrome || n.Kind == KindButton {
			if n.Kind == KindButton {
				nodes = append(nodes, n)
			}
			return
		}
		if n.Kind == KindText {
			nodes = append(nodes, n)
		}
		for _, c := range n.Children {
			collect(c)
		}
	}
	collect(root)
	if len(nodes) == 0 {
		return
	}

	candidates := append([]OCRLine(nil), ocrLines...)
	sort.Slice(candidates, func(i, j int) bool {
		const rowHeight = 20 // px; groups same-row text despite small OCR Y jitter
		ri, rj := candidates[i].Y/rowHeight, candidates[j].Y/rowHeight
		if ri != rj {
			return ri < rj
		}
		return candidates[i].X < candidates[j].X
	})
	used := make([]bool, len(candidates))

	// Pass 1: buttons get the pickiest match — short, non-oversized text
	// only — since a paragraph or heading should never end up as a
	// button's label just because it was next in reading order.
	for _, n := range nodes {
		if n.Kind != KindButton {
			continue
		}
		for i, c := range candidates {
			if used[i] {
				continue
			}
			wc := len(strings.Fields(c.Text))
			if wc < 1 || wc > 2 {
				continue
			}
			n.DisplayTextOverride = c.Text
			n.OCRVerified = true
			used[i] = true
			break
		}
	}

	// Pass 2: general text nodes, in document order, take the next
	// unclaimed candidate in reading order — whatever length it is, since
	// headings and body paragraphs vary too widely to filter by size.
	ci := 0
	for _, n := range nodes {
		if n.Kind != KindText {
			continue
		}
		for ci < len(candidates) && used[ci] {
			ci++
		}
		if ci >= len(candidates) {
			break
		}
		n.DisplayTextOverride = candidates[ci].Text
		n.OCRVerified = true
		used[ci] = true
		ci++
	}
}

// tesseractAvailable reports whether the tesseract CLI is on PATH. OCR is
// an optional enhancement, not a hard dependency: screenshot-only input
// still works without it, just with placeholder text instead of real
// recognized strings.
func tesseractAvailable() bool {
	_, err := exec.LookPath("tesseract")
	return err == nil
}

// recognizeText shells out to the tesseract CLI (must be installed
// separately, e.g. `brew install tesseract`) and returns one OCRLine per
// recognized line of text, ordered top-to-bottom as tesseract emits them.
// PSM 11 ("sparse text") suits UI screenshots — scattered text separated
// by large blank/graphical regions, not a uniform block of prose.
func recognizeText(imagePath string) ([]OCRLine, error) {
	if !tesseractAvailable() {
		return nil, fmt.Errorf("tesseract not found on PATH — install it with `brew install tesseract` for real text extraction from screenshot-only input")
	}
	cmd := exec.Command("tesseract", imagePath, "stdout", "--psm", "11", "tsv")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("running tesseract: %s", msg)
		}
		return nil, fmt.Errorf("running tesseract: %w", err)
	}
	return parseTesseractTSV(string(out)), nil
}

// recognizeTextFromBytes runs OCR against in-memory PNG bytes (the cropped
// screenshot, with OS status-bar/home-indicator chrome already removed, so
// tesseract doesn't recognize stray "9:41"/"100%" as real content) by
// staging them to a temp file, since the tesseract CLI needs a real path.
func recognizeTextFromBytes(pngData []byte) ([]OCRLine, error) {
	tmp, err := os.CreateTemp("", "figswiftui-ocr-*.png")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(pngData); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	return recognizeText(tmp.Name())
}

// minLineConfidence is the minimum *average* per-line OCR confidence
// (0-100, tesseract's own uncertainty score) to keep a line at all. A UI
// screenshot's real, cleanly-rendered text reliably scores in the 80s-90s;
// a busy screenshot's photos and icons still tempt tesseract into "reading"
// them as text (PSM 11 searches hard for text everywhere), but those reads
// score much lower. This is the primary noise filter — leaning on
// tesseract's own confidence signal is more reliable than guessing at
// patterns for "text that looks like an icon misread".
const minLineConfidence = 60

// parseTesseractTSV groups tesseract's word-level TSV rows (level 5) into
// per-*paragraph* entries, keyed by (block, paragraph) — merging multiple
// wrapped visual lines of the same paragraph into one text blob, so a
// multi-line CSS text node (body copy that wraps across 2-3 lines) can
// match one OCR entry instead of several fragments. A short, visually
// isolated label (a button, a single-line heading) is virtually always its
// own paragraph already, so this doesn't fragment those. Each entry's
// bounding box is the union of its words' boxes and its confidence the
// average of its words' confidences.
func parseTesseractTSV(tsv string) []OCRLine {
	type key struct{ block, par int }
	type acc struct {
		words                  []string
		confSum                float64
		confCount              int
		minX, minY, maxX, maxY int
		started                bool
	}
	lines := map[key]*acc{}
	var order []key

	scanner := bufio.NewScanner(strings.NewReader(tsv))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	headerSkipped := false
	for scanner.Scan() {
		if !headerSkipped {
			headerSkipped = true
			continue
		}
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) < 12 {
			continue
		}
		level, _ := strconv.Atoi(fields[0])
		if level != 5 { // word-level rows only
			continue
		}
		conf, _ := strconv.ParseFloat(fields[10], 64)
		text := strings.TrimSpace(fields[11])
		if text == "" || conf < 0 {
			continue
		}
		blockNum, _ := strconv.Atoi(fields[2])
		parNum, _ := strconv.Atoi(fields[3])
		left, _ := strconv.Atoi(fields[6])
		top, _ := strconv.Atoi(fields[7])
		width, _ := strconv.Atoi(fields[8])
		height, _ := strconv.Atoi(fields[9])

		k := key{blockNum, parNum}
		a, ok := lines[k]
		if !ok {
			a = &acc{}
			lines[k] = a
			order = append(order, k)
		}
		a.words = append(a.words, text)
		a.confSum += conf
		a.confCount++
		right, bottom := left+width, top+height
		if !a.started {
			a.minX, a.minY, a.maxX, a.maxY, a.started = left, top, right, bottom, true
		} else {
			if left < a.minX {
				a.minX = left
			}
			if top < a.minY {
				a.minY = top
			}
			if right > a.maxX {
				a.maxX = right
			}
			if bottom > a.maxY {
				a.maxY = bottom
			}
		}
	}

	result := make([]OCRLine, 0, len(order))
	for _, k := range order {
		a := lines[k]
		if a.confCount == 0 || a.confSum/float64(a.confCount) < minLineConfidence {
			continue
		}
		result = append(result, OCRLine{
			Text: strings.Join(a.words, " "),
			X:    a.minX, Y: a.minY,
			W: a.maxX - a.minX, H: a.maxY - a.minY,
		})
	}
	return result
}
