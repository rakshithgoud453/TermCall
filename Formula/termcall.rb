class Termcall < Formula
  desc "Direct, peer-to-peer encrypted terminal voice link in Go"
  homepage "https://github.com/rakshithgoud453/TermCall"
  url "https://github.com/rakshithgoud453/TermCall/archive/refs/tags/v0.2.1.tar.gz"
  sha256 "a52f11ce41d4a839a8153640405a097f2576b8243f90d88be76b86a279fa28cf"
  license "MIT"
  head "https://github.com/rakshithgoud453/TermCall.git", branch: "main"

  depends_on "go" => :build
  depends_on "pkg-config" => :build
  depends_on "opus"
  depends_on "opusfile"

  def install
    system "go", "build", "-ldflags", "-X github.com/rakshithgoud453/termcall/internal/cli.Version=#{version} -w -s", "-o", bin/"termcall", "./cmd/termcall"
  end

  test do
    assert_match "termcall version", shell_output("#{bin}/termcall version")
  end
end
