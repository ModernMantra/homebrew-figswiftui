class Figswiftui < Formula
  desc "Generate a SwiftUI screen + Assets.xcassets from a Figma CSS export or a screenshot (offline, no AI)"
  homepage "https://github.com/ModernMantra/homebrew-figswiftui"
  url "https://github.com/ModernMantra/homebrew-figswiftui/archive/refs/tags/v0.2.0.tar.gz"
  sha256 "PLACEHOLDER_FILLED_IN_AFTER_TAG_PUSH"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", "-o", bin/"figswiftui", "."
  end

  test do
    output = shell_output("#{bin}/figswiftui --help 2>&1")
    assert_match "figswiftui", output
  end
end
