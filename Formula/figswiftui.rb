class Figswiftui < Formula
  desc "Generate a SwiftUI screen + Assets.xcassets from a Figma CSS export or a screenshot (offline, no AI)"
  homepage "https://github.com/ModernMantra/homebrew-figswiftui"
  url "https://github.com/ModernMantra/homebrew-figswiftui/archive/refs/tags/v0.6.0.tar.gz"
  sha256 "28fcbe6adfa8622feab23a58d417c6a140e3ed85f04db8808cdb2f37857ec840"
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
