# typed: false
# frozen_string_literal: true

# Homebrew formula for mkvdup-canary (pre-release)
# This file is auto-updated by the release workflow.
# To install: brew tap stuckj/mkvdup https://github.com/stuckj/mkvdup && brew install mkvdup-canary
#
# The canary version can be installed alongside the stable version.

class MkvdupCanary < Formula
  desc "Storage deduplication tool for MKV files (canary/pre-release)"
  homepage "https://github.com/stuckj/mkvdup"
  license "MIT"
  version "0.1.0-canary.2"

  on_macos do
    on_arm do
      url "https://github.com/stuckj/mkvdup/releases/download/v0.1.0-canary.2/mkvdup-canary_darwin_arm64.tar.gz"
      sha256 "720b5818b451a058d22e78527739fc118eb687a1438c5f2d1d65b412f9d39dd6"
    end
    on_intel do
      url "https://github.com/stuckj/mkvdup/releases/download/v0.1.0-canary.2/mkvdup-canary_darwin_amd64.tar.gz"
      sha256 "2c2f87294efe8a27c16bf52a5daea0b893a2fd6bf684e2c1688c37364c97474a"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/stuckj/mkvdup/releases/download/v0.1.0-canary.2/mkvdup-canary_linux_arm64.tar.gz"
      sha256 "f99297ad9c4a69d2e439b630e67fd1fc108980af767dcd0250107d7a0051a314"
    end
    on_intel do
      url "https://github.com/stuckj/mkvdup/releases/download/v0.1.0-canary.2/mkvdup-canary_linux_amd64.tar.gz"
      sha256 "a54d5978656c64a07ac095d00519e42fefc1471dacb1fb7634e653cdaf1c24cc"
    end
  end

  # Build from source when formula hasn't been updated yet
  head "https://github.com/stuckj/mkvdup.git", branch: "main"

  depends_on "go" => :build if build.head?

  def install
    if build.head?
      system "go", "build", *std_go_args(output: bin/"mkvdup-canary", ldflags: "-s -w"), "./cmd/mkvdup"
      man1.install "docs/mkvdup.1" => "mkvdup-canary.1"
      doc.install "README.md", "DESIGN.md", "LICENSE"
      doc.install Dir["docs/*.md"]
      bash_completion.install "scripts/mkvdup-completion.bash" => "mkvdup-canary"
      zsh_completion.install "scripts/mkvdup-completion.zsh" => "_mkvdup-canary"
      fish_completion.install "scripts/mkvdup.fish" => "mkvdup-canary.fish"
    else
      bin.install "mkvdup-canary"
      man1.install "mkvdup-canary.1"
      doc.install "README.md", "DESIGN.md", "LICENSE"
      doc.install Dir["docs/*"]
      bash_completion.install "mkvdup-completion.bash" => "mkvdup-canary"
      zsh_completion.install "mkvdup-completion.zsh" => "_mkvdup-canary"
      fish_completion.install "mkvdup.fish" => "mkvdup-canary.fish"
    end
  end

  def caveats
    <<~EOS
      This is the canary (pre-release) version of mkvdup.
      The binary is installed as 'mkvdup-canary' to allow side-by-side
      installation with the stable 'mkvdup' package.
    EOS
  end

  test do
    if build.head?
      assert_match "mkvdup", shell_output("#{bin}/mkvdup-canary --help")
    else
      assert_match version.to_s, shell_output("#{bin}/mkvdup-canary --version")
    end
  end
end
