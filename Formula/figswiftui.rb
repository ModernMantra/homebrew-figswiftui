class Figswiftui < Formula
  desc "Generate a SwiftUI screen + Assets.xcassets from a Figma CSS export or a screenshot (offline, no AI)"
  homepage "https://github.com/ModernMantra/homebrew-figswiftui"
  url "https://github.com/ModernMantra/homebrew-figswiftui/archive/refs/tags/v0.7.0.tar.gz"
  sha256 "2ba4a6e6ac3011a41ecad35cdd212de58182e3846c0e5120ca7d5ff3d0dcd566"
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
