#!/usr/bin/env bash
# Emit the landing page for the package repositories on stdout.
set -euo pipefail
REPO="${GITHUB_REPOSITORY:-stuckj/mkvdup}"
BASE="https://github.com/${REPO}/releases/download"
PAGES="${PAGES_URL:-https://stuckj.github.io/mkvdup}"

cat <<EOF
<!DOCTYPE html>
<html>
<head>
  <title>mkvdup - Package Repository</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
    pre { background: #f4f4f4; padding: 15px; border-radius: 5px; overflow-x: auto; }
    code { background: #f4f4f4; padding: 2px 6px; border-radius: 3px; }
    h1 { border-bottom: 2px solid #333; padding-bottom: 10px; }
    h2 { margin-top: 30px; }
    .note { background: #f0f6ff; border-left: 4px solid #4a7dbd; padding: 10px 15px; margin: 15px 0; }
  </style>
</head>
<body>
  <h1>mkvdup</h1>
  <p>MKV deduplication tool using FUSE. Store MKV files as small dedup files that reference original DVD/Blu-ray sources.</p>

  <h2>macOS / Linux (Homebrew)</h2>
  <pre>
brew tap stuckj/mkvdup
brew install mkvdup</pre>

  <h2>Debian/Ubuntu (APT)</h2>
  <pre>
# Add the GPG key
curl -fsSL ${PAGES}/gpg-key.asc | sudo gpg --dearmor -o /usr/share/keyrings/mkvdup.gpg

# Add the repository
echo "deb [signed-by=/usr/share/keyrings/mkvdup.gpg arch=amd64,arm64] ${PAGES}/apt stable main" | sudo tee /etc/apt/sources.list.d/mkvdup.list

# Install
sudo apt update
sudo apt install mkvdup</pre>

  <div class="note">
    <p>This repository carries the <strong>current release only</strong>. To install or pin an
    older version, use the archive repository below instead — it indexes every version ever
    published. Both are signed with the same key, and both may be enabled at once.</p>
  </div>

  <h3>Debian/Ubuntu (APT) — full version history</h3>
  <pre>
echo "deb [signed-by=/usr/share/keyrings/mkvdup.gpg arch=amd64,arm64] ${BASE}/apt-history/ ./" | sudo tee /etc/apt/sources.list.d/mkvdup-history.list

sudo apt update
apt list -a mkvdup                 # every published version
sudo apt install mkvdup=1.8.0      # pin one</pre>

  <h2>RHEL/CentOS/Fedora (YUM/DNF)</h2>
  <p>Indexes every version published; no separate archive repository is needed.</p>
  <pre>
# Add the repository
sudo tee /etc/yum.repos.d/mkvdup.repo &lt;&lt; 'REPO'
[mkvdup]
name=mkvdup
baseurl=${PAGES}/yum
enabled=1
gpgcheck=1
gpgkey=${PAGES}/yum/gpg-key.asc
REPO

# Install
sudo dnf install mkvdup
sudo dnf install mkvdup-1.8.0      # or pin an older version</pre>

  <h2>Canary Channel (Pre-release)</h2>
  <p>The canary channel provides early access to new features. It installs as
  <code>mkvdup-canary</code> and can be installed alongside the stable version.</p>

  <h3>macOS / Linux (Homebrew) - Canary</h3>
  <pre>
brew tap stuckj/mkvdup
brew install mkvdup-canary</pre>

  <h3>Debian/Ubuntu (APT) - Canary</h3>
  <pre>
# Add the GPG key (same key as stable)
curl -fsSL ${PAGES}/gpg-key.asc | sudo gpg --dearmor -o /usr/share/keyrings/mkvdup.gpg

# Current canary only
echo "deb [signed-by=/usr/share/keyrings/mkvdup.gpg arch=amd64,arm64] ${PAGES}/apt canary main" | sudo tee /etc/apt/sources.list.d/mkvdup-canary.list

# ...or every canary ever published
echo "deb [signed-by=/usr/share/keyrings/mkvdup.gpg arch=amd64,arm64] ${BASE}/apt-history-canary/ ./" | sudo tee /etc/apt/sources.list.d/mkvdup-canary-history.list

sudo apt update
sudo apt install mkvdup-canary</pre>

  <h3>RHEL/CentOS/Fedora (YUM/DNF) - Canary</h3>
  <pre>
# Add the canary repository
sudo tee /etc/yum.repos.d/mkvdup-canary.repo &lt;&lt; 'REPO'
[mkvdup-canary]
name=mkvdup-canary
baseurl=${PAGES}/yum-canary
enabled=1
gpgcheck=1
gpgkey=${PAGES}/yum-canary/gpg-key.asc
REPO

# Install
sudo dnf install mkvdup-canary</pre>

  <h2>Where the packages live</h2>
  <p>Package files are served from the per-version
  <a href="https://github.com/${REPO}/releases">GitHub releases</a>; these repositories carry
  only the indexes. The APT archive reaches them with a relative
  <code>Filename</code>, and the YUM metadata with a per-package <code>xml:base</code>.</p>
</body>
</html>
EOF
