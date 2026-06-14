class Termtalk < Formula
  desc "Offline-first, decentralized terminal-based P2P messaging app"
  homepage "https://github.com/T9ner/termtk"
  url "https://github.com/T9ner/termtk/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "0000000000000000000000000000000000000000000000000000000000000000" # Replace with actual SHA256 of release archive
  license "MIT"

  depends_on "go" => :build

  def install
    # Build the main client binary
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/termtalk"
    
    # Optionally build and install the relay binary too
    # system "go", "build", *std_go_args(ldflags: "-s -w", output: bin/"termtalk-relay"), "./cmd/termtalk-relay"
  end

  test do
    # Simple check that the app runs and exits (it starts a TUI, so we check usage or output)
    # Since it expects a TTY for TUI, we check that it compiled successfully.
    assert_predicate bin/"termtalk", :exist?
  end
end
