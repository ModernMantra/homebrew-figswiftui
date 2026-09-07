class Figswiftui < Formula
  desc "Generate a SwiftUI screen + Assets.xcassets from a Figma CSS export or a screenshot (offline, no AI)"
  homepage "https://github.com/ModernMantra/homebrew-figswiftui"
  url "https://github.com/ModernMantra/homebrew-figswiftui/archive/refs/tags/v0.8.1.tar.gz"
  sha256 "995d0a34d0b3e0d6bee8a5f3c8b142a24b1c91a3f31bafc8571afb0409703e01"
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
