class Figswiftui < Formula
  desc "Generate a SwiftUI screen + Assets.xcassets from a Figma CSS export or a screenshot (offline, no AI)"
  homepage "https://github.com/ModernMantra/homebrew-figswiftui"
  url "https://github.com/ModernMantra/homebrew-figswiftui/archive/refs/tags/v0.2.0.tar.gz"
  sha256 "58bbccfd7dce9f07e5ef2d332c63268943ba9c84e6296423d5c073f7c3933c76"
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
