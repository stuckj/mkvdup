# Releasing mkvdup

This document describes how to create a new release of mkvdup.

## Prerequisites

Before your first release, you need to set up GPG signing:

### 1. Generate a GPG Key (if you don't have one)

```bash
gpg --full-generate-key
# Choose: RSA and RSA, 4096 bits, no expiration
# Use your GitHub email address
```

### 2. Export and Add Secrets to GitHub

```bash
# Export private key (keep this secure!)
gpg --armor --export-secret-keys YOUR_KEY_ID > private-key.asc

# Get your passphrase ready
```

Add these secrets to your GitHub repository (Settings → Secrets and variables → Actions):

- `GPG_PRIVATE_KEY`: Contents of `private-key.asc`
- `GPG_PASSPHRASE`: Your GPG key passphrase

### 3. Enable GitHub Pages

1. Go to repository Settings → Pages
2. Set Source to "Deploy from a branch"
3. Select `gh-pages` branch (will be created on first release)

## Creating a Release

### Option 1: Tag-based Release (Recommended)

Create and push a version tag:

```bash
# Ensure you're on main and up to date
git checkout main
git pull

# Create an annotated tag
git tag -a v1.0.0 -m "Release v1.0.0"

# Push the tag
git push origin v1.0.0
```

This will automatically:
1. Build packages for amd64 and arm64
2. Create a GitHub release with the packages attached
3. Update the APT and YUM repositories on GitHub Pages

### Option 2: Manual Workflow Dispatch

For testing or building without a release:

1. Go to Actions → "Build and Release Packages"
2. Click "Run workflow"
3. Enter the version number (without `v` prefix)
4. Click "Run workflow"

This builds packages and updates repositories but doesn't create a GitHub release.

## Version Numbering

Follow semantic versioning (semver):

- `MAJOR.MINOR.PATCH` (e.g., `1.2.3`)
- MAJOR: Breaking changes
- MINOR: New features, backwards compatible
- PATCH: Bug fixes, backwards compatible

## Canary Releases

Canary releases provide early access to new features and are installed as a
separate `mkvdup-canary` package that coexists with the stable `mkvdup`.

### Canary Version Format

Use the format `MAJOR.MINOR.PATCH-canary.N` where N is an incrementing number:
- `1.2.0-canary.1`
- `1.2.0-canary.2`

### Creating a Canary Release

Use the same workflow dispatch as stable releases, but with a canary version:

1. Go to Actions > "Build and Release Packages"
2. Click "Run workflow"
3. Enter the version (e.g., `1.2.0-canary.1`)
4. Click "Run workflow"

The workflow automatically detects the `-canary.` suffix and:
- Builds the binary as `mkvdup-canary`
- Packages as `mkvdup-canary` (installs to `/usr/bin/mkvdup-canary`)
- Creates a pre-release on GitHub
- Publishes to the canary APT/YUM repositories (separate from stable)

### Canary Package Repositories

#### APT (Debian/Ubuntu) - Canary

```bash
curl -fsSL https://stuckj.github.io/mkvdup/gpg-key.asc | sudo gpg --dearmor -o /usr/share/keyrings/mkvdup.gpg
echo "deb [signed-by=/usr/share/keyrings/mkvdup.gpg arch=amd64,arm64] https://stuckj.github.io/mkvdup/apt canary main" | sudo tee /etc/apt/sources.list.d/mkvdup-canary.list
sudo apt update
sudo apt install mkvdup-canary
```

#### YUM/DNF (RHEL/Fedora) - Canary

```bash
sudo tee /etc/yum.repos.d/mkvdup-canary.repo << 'EOF'
[mkvdup-canary]
name=mkvdup-canary
baseurl=https://stuckj.github.io/mkvdup/yum-canary
enabled=1
gpgcheck=1
gpgkey=https://stuckj.github.io/mkvdup/yum-canary/gpg-key.asc
EOF

sudo dnf install mkvdup-canary
```

### Local Testing (Canary)

```bash
go build -o mkvdup-canary ./cmd/mkvdup
PACKAGE_NAME=mkvdup-canary VERSION=1.0.0-canary.1 GOARCH=amd64 nfpm package --packager deb
PACKAGE_NAME=mkvdup-canary VERSION=1.0.0-canary.1 GOARCH=amd64 nfpm package --packager rpm
```

## What Gets Built

Each release produces:

| Package | Architecture | Format |
|---------|--------------|--------|
| mkvdup_VERSION_amd64.deb | x86_64 | Debian/Ubuntu |
| mkvdup_VERSION_arm64.deb | ARM64 | Debian/Ubuntu |
| mkvdup-VERSION.x86_64.rpm | x86_64 | RHEL/Fedora |
| mkvdup-VERSION.aarch64.rpm | ARM64 | RHEL/Fedora |

## Package Repositories

After a release, packages are available from:

### APT (Debian/Ubuntu)

```bash
curl -fsSL https://stuckj.github.io/mkvdup/gpg-key.asc | sudo gpg --dearmor -o /usr/share/keyrings/mkvdup.gpg
echo "deb [signed-by=/usr/share/keyrings/mkvdup.gpg arch=amd64,arm64] https://stuckj.github.io/mkvdup/apt stable main" | sudo tee /etc/apt/sources.list.d/mkvdup.list
sudo apt update
sudo apt install mkvdup
```

### YUM/DNF (RHEL/Fedora)

```bash
sudo tee /etc/yum.repos.d/mkvdup.repo << 'EOF'
[mkvdup]
name=mkvdup
baseurl=https://stuckj.github.io/mkvdup/yum
enabled=1
gpgcheck=1
gpgkey=https://stuckj.github.io/mkvdup/yum/gpg-key.asc
EOF

sudo dnf install mkvdup
```

## Troubleshooting

### Build Failures

- Check the Actions tab for detailed logs
- Ensure Go version in workflow matches go.mod
- Verify nfpm.yaml syntax

### GPG Signing Errors

- Verify `GPG_PRIVATE_KEY` secret contains the full armored key
- Verify `GPG_PASSPHRASE` is correct
- Check that the key hasn't expired

### Repository Not Updating

- Ensure GitHub Pages is enabled
- Check that `gh-pages` branch exists
- Verify the workflow has `pages: write` permission

## Local Testing

To build packages locally:

```bash
# Install nfpm
go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest

# Build the binary
go build -o mkvdup ./cmd/mkvdup

# Build packages
PACKAGE_NAME=mkvdup VERSION=1.0.0 GOARCH=amd64 nfpm package --packager deb
PACKAGE_NAME=mkvdup VERSION=1.0.0 GOARCH=amd64 nfpm package --packager rpm
```
