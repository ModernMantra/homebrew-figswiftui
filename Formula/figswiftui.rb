class Figswiftui < Formula
  desc "Generate a SwiftUI screen + Assets.xcassets from a Figma CSS export or a screenshot (offline, no AI)"
  homepage "https://github.com/ModernMantra/homebrew-figswiftui"
  url "https://github.com/ModernMantra/homebrew-figswiftui/archive/refs/tags/v0.5.1.tar.gz"
  sha256 "e2d1885c0098a2b6b52d4428c56a351a3b4c3fbc40aa6cf92256e05069509180"
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
