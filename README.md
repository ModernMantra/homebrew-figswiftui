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

**Giving both together** does more than attach a reference image: CSS still
drives all the structure and colors, but every piece of text — not just
buttons — is matched against the screenshot's OCR output in reading order
and preferred when a plausible match is found, since OCR reflects what's
actually rendered (a componentized button's Figma layer only names the
*component*, e.g. "Skip"/"Next", and a template's placeholder copy can be
stale relative to a real, localized instance). It's a best-effort
correlation, not a guaranteed-correct one — there's no real layout-position
resolver behind it, just reading order — so a button's filled-in label
still gets a `// TODO` comment naming it as OCR-derived.

**Small icon-like shapes** (<=60px) get real icon treatment instead of a
flat color rect: an SF Symbol when the Figma layer name matches a
recognizable keyword ("search_icon" → `magnifyingglass`, works without a
screenshot), or — when no keyword matches but a screenshot is available —
a direct crop of the real pixels at that shape's position (via
`brew install imagemagick`; installed by default alongside figswiftui,
skip it with `--without-imagemagick`), which is the "use a real icon
instead of guessing" fallback in its literal sense: actual rendered pixels,
not a reconstruction. Without imagemagick, or when the shape's position
can't be resolved, it stays a plain colored shape.

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
- OCR-filled text (buttons and general text alike) is a best-effort reading-
  order correlation, not a guaranteed-correct match — always flagged with a
  `// TODO` comment for manual verification.
- The icon fallback (SF Symbol or a cropped screenshot region) only applies
  to small (<=60px), individually-classified shapes — a decorative
  illustration subtree is still mechanically translated to SVG as a whole,
  and the crop fallback specifically only works for a shape that's itself
  `position: absolute` with a resolvable box; there's no general
  layout-position resolver behind either path.
- The layout-tree reconstruction is tuned to Figma's standard auto-layout
  "copy as CSS" export convention, not arbitrary hand-authored CSS.

## License

MIT — see [LICENSE](LICENSE).
