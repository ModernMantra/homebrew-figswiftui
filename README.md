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
  only yields a low-fidelity color/dimension skeleton with placeholder text
  — there's no OCR, so real text and layout extraction needs a CSS export.

## Known limitations

- Screenshot-only input can't recover real structure or text (no AI/OCR by
  design).
- Componentized button labels (e.g. a "Skip"/"Next" component whose Figma
  layer name may not match its true localized/rendered text) get a
  `// TODO` comment flagging them for manual verification.
- The layout-tree reconstruction is tuned to Figma's standard auto-layout
  "copy as CSS" export convention, not arbitrary hand-authored CSS.

## License

MIT — see [LICENSE](LICENSE).
