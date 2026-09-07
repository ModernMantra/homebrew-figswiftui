package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// OCRLine is one line of text recognized in a screenshot, with its
// pixel bounding box (relative to the image passed to tesseract).
type OCRLine struct {
	Text       string
	X, Y, W, H int
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
	out, err := exec.Command("tesseract", imagePath, "stdout", "--psm", "11", "tsv").Output()
	if err != nil {
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
// per-line entries, keyed by (block, paragraph, line), with each line's
// bounding box being the union of its words' boxes and its confidence the
// average of its words' confidences.
func parseTesseractTSV(tsv string) []OCRLine {
	type key struct{ block, par, line int }
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
		lineNum, _ := strconv.Atoi(fields[4])
		left, _ := strconv.Atoi(fields[6])
		top, _ := strconv.Atoi(fields[7])
		width, _ := strconv.Atoi(fields[8])
		height, _ := strconv.Atoi(fields[9])

		k := key{blockNum, parNum, lineNum}
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
