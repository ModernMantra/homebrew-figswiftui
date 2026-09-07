class Figswiftui < Formula
  desc "Generate a SwiftUI screen + Assets.xcassets from a Figma CSS export or a screenshot (offline, no AI)"
  homepage "https://github.com/ModernMantra/homebrew-figswiftui"
  url "https://github.com/ModernMantra/homebrew-figswiftui/archive/refs/tags/v0.8.0.tar.gz"
  sha256 "1b01dfd154270bae361ce9f6bb480c9ebcd29d12172238c288f016082762016a"
  license "MIT"

  depends_on "go" => :build
  depends_on "tesseract" => :recommended
  depends_on "imagemagick" => :recommended

  def install
    system "go", "build", "-o", bin/"figswiftui", "."
  end

  def caveats
    <<~EOS
      tesseract is used for --screenshot input: it recognizes real text
      instead of a "TODO" placeholder (screenshot-only), or fills in real
      button labels and refines heading/body copy (CSS + screenshot
      together). imagemagick is used only when a small icon-like shape has
      no good SF Symbol match: it crops the real pixels for that shape
      directly out of the screenshot instead of a mechanical reconstruction.
      Everything else (the primary --css-driven path, --batch,
      --match-project) works the same with or without either — figswiftui
      detects their absence and falls back automatically.
    EOS
  end

  test do
    output = shell_output("#{bin}/figswiftui --help 2>&1")
    assert_match "figswiftui", output
  end
end
