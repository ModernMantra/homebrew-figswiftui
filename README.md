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

Just pass the file(s) or a directory as plain arguments, in any order —
figswiftui tells a `.css` from a screenshot by extension, and a directory
means batch mode. Nothing else is required: a bundle name is derived from
the content when you don't give one.

```bash
figswiftui onboarding.css                       # CSS only
figswiftui onboarding.css onboarding.png        # CSS + screenshot, either order
figswiftui onboarding.png                       # screenshot only
figswiftui ./screens                            # batch: every <basename>.css in the directory
```

Flags are only for the less common cases — an explicit output name, a
project to match colors against, or an ambiguous path (e.g. a `.txt` file
that's actually a CSS export):

```bash
figswiftui onboarding.css --name Customer-TICKET-123 --out ./output
figswiftui onboarding.css --match-project /path/to/XcodeProject
figswiftui ./screens --name Customer --allow-duplicates
```

Run `figswiftui --help` for the full flag reference.

## Input

- A plain-text Figma "copy as CSS" export (select a frame in Figma,
  right-click → Copy/Paste as → Copy as CSS). Must be plain text, not an
  RTF-wrapped paste.
- A PNG/JPEG screenshot of the same screen. Alone (no CSS), this yields a
  color/dimension skeleton — real layout/structure still needs a CSS export
  — but text is recognized via OCR (`brew install tesseract`; installed by
  default alongside figswiftui, skip it with `--without-tesseract` at
  install time) instead of a placeholder. Without tesseract on PATH,
  figswiftui detects that and falls back automatically.

## Batch mode

Point figswiftui at a directory and every `<basename>.css` inside it becomes
its own output bundle, paired with a same-basename `.png`/`.jpg`/`.jpeg`
screenshot when one exists alongside it. `--name` becomes an optional shared
prefix (`figswiftui ./screens --name Customer` → `Customer-<basename>`); one
failing screen is reported and skipped rather than aborting the whole batch.

**Duplicate detection**: a `.css` byte-identical to one already processed in
the same run (SHA-256) is skipped automatically, and so is a screenshot
that's a *near*-duplicate of one already processed (perceptual hashing —
catches a re-export or recompression even when the bytes differ, not just
exact copies). Both are reported with a warning naming which earlier file
they duplicate. Pass `--allow-duplicates` to generate every screen anyway.

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
