class Nod < Formula
  desc "Offline-first, decentralized terminal-based P2P messaging app"
  homepage "https://github.com/T9ner/nod"
  url "https://github.com/T9ner/nod/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "0000000000000000000000000000000000000000000000000000000000000000" # Replace with actual SHA256 of release archive
  license "MIT"

  head "https://github.com/T9ner/nod.git", branch: "main"

  depends_on "go" => :build

  def install
    # Build and install Nod client
    system "go", "build", "-ldflags", "-s -w", "-o", bin/"Nod", "./cmd/Nod"
    
    # Build and install Nod relay server
    system "go", "build", "-ldflags", "-s -w", "-o", bin/"Nod-relay", "./cmd/Nod-relay"
  end

  test do
    # Simple check that the app runs and exits (it starts a TUI, so we check usage or output)
    # Since it expects a TTY for TUI, we check that it compiled successfully.
    assert_predicate bin/"Nod", :exist?
  end
end
