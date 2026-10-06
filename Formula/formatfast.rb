# Bootstrap formula. Install with --HEAD until the first stable release.
# Replace with the formatfast.rb attached to the published GitHub Release.
class Formatfast < Formula
  desc "Fast, lossless JSON formatter and multiline log viewer"
  homepage "https://github.com/leadfact/format-fast"
  head "https://github.com/leadfact/format-fast.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args, "./cmd/formatfast"
  end

  test do
    assert_match "formatfast", shell_output("#{bin}/formatfast --version")
    assert_equal "{\"amount\":1.2300,\"id\":9007199254740993}\n",
                 shell_output("#{bin}/formatfast --compact '{\"amount\":1.2300,\"id\":9007199254740993}'")
  end
end
