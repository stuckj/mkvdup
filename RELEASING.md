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

Releases are created via the GitHub Actions workflow:

1. Go to Actions → "Build and Release Packages"
2. Click "Run workflow"
3. Enter the version number (without `v` prefix, e.g., `1.0.0`). The workflow rejects a
   leading `v` and derives the tag itself.
4. Optionally specify a commit SHA. The default is the latest non-benchmark commit on the
   branch the workflow was dispatched from — **not** necessarily `main`.
5. Click "Run workflow"

The workflow will:

1. Resolve the version and commit, and verify the tag does not already exist on the remote
2. Build packages for amd64 and arm64
3. Create the GitHub release **and the tag together**, pointing at the resolved commit
4. Update the APT and YUM repositories on GitHub Pages, and the Homebrew and Nix files

**Tag timing:** the tag is deliberately created by the release step, not up front. Nothing
user-visible — tag, release, packages, formula and Nix bumps — is created until every build
has succeeded. An earlier version pushed the tag first, so a failed build left a dangling tag
pointing at a release that never happened *and* blocked retrying the same version, because of
the existence check in step 1.

**Note:** Do not manually create tags before running the workflow. It creates the tag itself
and will fail if the tag already exists. If a run fails after the tag was created (only
possible on versions of the workflow predating the change above), delete the tag before
retrying that same version.

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

### Nix

There is no separate canary channel to publish to, unlike the APT/YUM repositories above. The flake
builds `src = ./.`, so the flake reference *is* the selector and users point it at whatever ref they
want. See [Installation → NixOS / Nix](README.md#nixos--nix) for the user-facing commands.

The flake exposes two packages, differing only in the installed command name so a canary can sit
alongside a stable install:

| Output | Command | Intended ref |
|--------|---------|--------------|
| `#mkvdup` (also `default`) | `mkvdup` | a release tag |
| `#mkvdup-canary` | `mkvdup-canary` | a development branch, or a canary tag |

Both outputs are available from **`v1.8.2`** onward, the first release cut after this landed. Older
tags are limited by what the flake contained at the time: `v1.8.1` has only `#mkvdup-canary`, and
tags at or before `v1.8.0` have no flake at all.

## Nix Maintenance

Both halves are meant to stay current on their own. What to watch for:

### Stable (nixpkgs)

- **The derivation lives upstream, and only upstream**, at `pkgs/by-name/mk/mkvdup/package.nix` in
  [nixpkgs](https://github.com/NixOS/nixpkgs). This repo deliberately keeps no copy of it — see
  [nix/nixpkgs/README.md](nix/nixpkgs/README.md) for why, and for the notes worth keeping.
- After a release, the [`r-ryantm`](https://github.com/r-ryantm) bot usually opens a version-bump
  PR against nixpkgs within a few days. **Approve it** — that is the normal update path. As a
  listed maintainer you are auto-requested for review.
- Changes that alter how mkvdup is *built* — not just its version — always need a hand-written
  nixpkgs PR. The bot only bumps `version`, `hash`, and `vendorHash`. Edit the file in a nixpkgs
  checkout; there is nothing here to keep in sync.

### Canary (in-repo flake)

The canary side is fully automated — there is no manual step in the normal course of events.
`flake.nix` and `default.nix` pin a `vendorHash` over the Go module set, which goes stale whenever
`go.mod`/`go.sum` changes. Two workflows keep it fresh, both calling
`scripts/update-nix-vendor-hash.sh`:

| Workflow | Trigger | What it does |
|----------|---------|--------------|
| `nix-canary-hash.yml` | Push to **any branch** touching `go.mod`/`go.sum`, or manual dispatch | Refreshes `vendorHash` and pushes it back to that same branch |
| `release.yml` (`sync-nix` job) | Every release | Writes `version` and a refreshed `vendorHash` onto **the commit being released**, then the tag is created at the result |

The first one runs on every branch, not just `main`, and that matters: canaries are cut from
development branches, and a dependency bump is simultaneously the case where you most want a canary
and the thing that invalidates the hash. Refreshing on the branch is what keeps
`nix profile install github:stuckj/mkvdup/<branch>#mkvdup-canary` working with no manual step.

`sync-nix` writes to the released ref rather than to `main`, which is what lets a canary tag report
its own version. It runs after `build` — so nothing is pushed until every build has passed — and
before `release`, so the tag can be created at the commit it produces. It stands down without
touching anything when the release was dispatched from a tag, or when a `commit` input points
somewhere other than the branch head; in both cases a bump would land on a commit that was never
requested or built. Unlike `nix-canary-hash.yml` it does *not* rebase and retry a rejected push,
because rebasing would move the bump onto commits that were never built. It fails instead, and the
release should be re-run.

Merging such a branch into `main` carries the refreshed hash along with the `go.sum` that produced
it, so `main` normally needs nothing of its own — the hash is a function of `go.mod`/`go.sum`, not
of the source tree. The exception is a merge that combines dependency changes from two branches:
the resulting `go.sum` is a union that neither branch pinned, and `main`'s own run of the workflow
catches that.

The nixpkgs package is never affected either way — it pins an immutable release tag.

To refresh the hash by hand (needs `nix` with flakes enabled):

```bash
./scripts/update-nix-vendor-hash.sh   # updates both files, verifies the result
nix flake check
```

If the script reports a build failure that isn't a hash mismatch, the canary is genuinely broken
and needs a code fix — it deliberately refuses to rewrite the hash in that case.

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
