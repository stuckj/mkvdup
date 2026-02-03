# typed: false
# frozen_string_literal: true

# Homebrew formula for mkvdup
# This file is auto-updated by the release workflow.
# To install: brew tap stuckj/mkvdup https://github.com/stuckj/mkvdup && brew install mkvdup

class Mkvdup < Formula
  desc "Storage deduplication tool for MKV files and their source media"
  homepage "https://github.com/stuckj/mkvdup"
  license "MIT"
  version "0.0.0"

  on_macos do
    on_arm do
      url "https://github.com/stuckj/mkvdup/releases/download/v0.0.0/mkvdup_darwin_arm64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
    on_intel do
      url "https://github.com/stuckj/mkvdup/releases/download/v0.0.0/mkvdup_darwin_amd64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/stuckj/mkvdup/releases/download/v0.0.0/mkvdup_linux_arm64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
    on_intel do
      url "https://github.com/stuckj/mkvdup/releases/download/v0.0.0/mkvdup_linux_amd64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
  end

  def install
    bin.install "mkvdup"
    man1.install "mkvdup.1"
    doc.install "README.md", "DESIGN.md", "LICENSE"
    doc.install Dir["docs/*"]
    bash_completion.install "mkvdup-completion.bash" => "mkvdup"
    zsh_completion.install "mkvdup-completion.zsh" => "_mkvdup"
    fish_completion.install "mkvdup.fish"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/mkvdup --version")
  end
end
