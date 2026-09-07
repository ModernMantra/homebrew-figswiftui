class Figswiftui < Formula
  desc "Generate a SwiftUI screen + Assets.xcassets from a Figma CSS export or a screenshot (offline, no AI)"
  homepage "https://github.com/ModernMantra/homebrew-figswiftui"
  url "https://github.com/ModernMantra/homebrew-figswiftui/archive/refs/tags/v0.4.0.tar.gz"
  sha256 "fea239765c6262245a6ba178a6a4b4ca8e3911c64c0d1bb647930fe0b4def016"
  license "MIT"

  depends_on "go" => :build
  depends_on "tesseract" => :recommended

  def install
    system "go", "build", "-o", bin/"figswiftui", "."
  end

  def caveats
    <<~EOS
      tesseract is used only for --screenshot input with no companion --css:
      it recognizes real text instead of a "TODO" placeholder. Everything
      else (the primary --css-driven path, --batch, --match-project) works
      the same with or without it — figswiftui detects its absence and
      falls back automatically.
    EOS
  end

  test do
    output = shell_output("#{bin}/figswiftui --help 2>&1")
    assert_match "figswiftui", output
  end
end
