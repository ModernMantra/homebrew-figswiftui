class Figswiftui < Formula
  desc "Generate a SwiftUI screen + Assets.xcassets from a Figma CSS export or a screenshot (offline, no AI)"
  homepage "https://github.com/ModernMantra/homebrew-figswiftui"
  url "https://github.com/ModernMantra/homebrew-figswiftui/archive/refs/tags/v0.3.0.tar.gz"
  sha256 "9003baf0fdd1f303083cdd46ff95d6ef02ba79dc1c624016275938d2b0b4fb77"
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
