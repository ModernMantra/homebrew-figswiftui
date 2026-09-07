# figswiftui

Generate a SwiftUI screen + Xcode `Assets.xcassets` from a Figma "copy as CSS"
export and/or a screenshot.

Fully offline and deterministic: no AI calls, no network calls at runtime.
It parses Figma's CSS export directly, reconstructs the real layout tree,
extracts and semantically names colors, converts decorative illustrations to
SVG, and emits idiomatic SwiftUI + a matching asset catalog.

## Install

```bash
brew tap ModernMantra/figswiftui
brew install figswiftui
```

## Usage

Single screen:

```bash
figswiftui --css onboarding.css --screenshot onboarding.png --name Customer-TICKET-123 --out ./output
```

Batch — a directory of `<basename>.css` files, each optionally paired with a
same-basename `.png`/`.jpg`/`.jpeg` screenshot:

```bash
figswiftui --batch ./screens --name Customer --out ./output
```

Match colors against an existing Xcode project's asset catalog by exact RGB
value, reusing its real color names instead of inventing generic ones:

```bash
figswiftui --css onboarding.css --name Customer-TICKET-123 --match-project /path/to/XcodeProject
```

Run `figswiftui --help` for the full flag reference.

## Input

- `--css` — a plain-text Figma "copy as CSS" export (select a frame in Figma,
  right-click → Copy/Paste as → Copy as CSS). Must be plain text, not an
  RTF-wrapped paste.
- `--screenshot` — a PNG/JPEG of the same screen. Alone (no `--css`), this
  yields a color/dimension skeleton — real layout/structure still needs a
  CSS export — but text is recognized via OCR (`brew install tesseract`;
  installed by default alongside figswiftui, skip it with
  `--without-tesseract` at install time) instead of a placeholder. Without
  tesseract on PATH, figswiftui detects that and falls back automatically.

In `--batch` mode, a `.css` byte-identical to one already processed, or a
screenshot that's a near-duplicate of one already processed, is skipped
with a warning — pass `--allow-duplicates` to generate it anyway.

## Known limitations

- Screenshot-only input still can't recover real VStack/HStack structure,
  spacing, or exact layout from pixels alone (that's what multimodal AI
  does, deliberately out of scope here) — OCR closes the text gap, not the
  structure gap.
- Componentized button labels (e.g. a "Skip"/"Next" component whose Figma
  layer name may not match its true localized/rendered text) get a
  `// TODO` comment flagging them for manual verification.
- The layout-tree reconstruction is tuned to Figma's standard auto-layout
  "copy as CSS" export convention, not arbitrary hand-authored CSS.

## License

MIT — see [LICENSE](LICENSE).
