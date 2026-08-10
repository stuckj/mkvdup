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

The key in use today is **ed25519**, not RSA. That choice has one consequence
worth knowing before rotating it: rpm gained EdDSA support in 4.16.0, so
EL8's rpm 4.14 cannot import an ed25519 key and cannot verify packages signed
with one. Measured on `almalinux:8` — `rpm --import` fails with
`key 1 import failed`, and a `gpgcheck=1` install then fails with
`GPG check FAILED`. RSA-4096 verifies everywhere including EL8. Generating a
new key means re-signing every published rpm (see below) and every user
re-importing it, so this is a decision to make deliberately rather than a
default to drift into.

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
4. Click "Run workflow"

**Dispatch from a branch, not a tag.** The commit to tag is resolved from the branch history,
and `sync-nix` pushes the Nix version bump back to that same ref, so a tag ref has nowhere to
put it. The workflow rejects a non-branch dispatch up front.

The commit to tag is not selectable. It is always the latest bookkeeping-free commit on the
branch the workflow was dispatched from — **not** necessarily `main`. There used to be an
optional commit-SHA input; it was removed in #212, having never been used, because it made
the Nix version bump unsatisfiable. The bump has to be committed somewhere the tag can reach,
and a commit on top of an arbitrary older SHA is not a fast-forward of the branch, so it
cannot be pushed there.

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

Current canary only. For every canary ever published:

```bash
echo "deb [signed-by=/usr/share/keyrings/mkvdup.gpg arch=amd64,arm64] https://github.com/stuckj/mkvdup/releases/download/apt-history-canary/ ./" | sudo tee /etc/apt/sources.list.d/mkvdup-canary-history.list
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

The Pages repository carries the current release only. Every version ever
published is in the archive repository, which is signed with the same key:

```bash
echo "deb [signed-by=/usr/share/keyrings/mkvdup.gpg arch=amd64,arm64] https://github.com/stuckj/mkvdup/releases/download/apt-history/ ./" | sudo tee /etc/apt/sources.list.d/mkvdup-history.list
sudo apt update && sudo apt install mkvdup=1.8.0
```

### YUM/DNF (RHEL/Fedora)

Indexes every version published; there is no separate archive repository.

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

### How the repositories are built

Package files are stored **once**, as assets on the per-version GitHub release.
The repositories hold only indexes, which `scripts/rebuild-package-repos.sh`
derives from those releases — so the release must exist before its packages can
be indexed, and a package whose release is deleted drops out of the index.

| Repository | Index lives in | Covers |
|------------|----------------|--------|
| `apt-history`, `apt-history-canary` | a release asset | every version |
| `gh-pages apt/` | Pages | current version only |
| `gh-pages yum/`, `yum-canary/` | Pages | every version |

APT needs two repositories and YUM one because of a format difference, not a
preference: RPM-MD takes an absolute per-package `xml:base`, so its index can
sit on Pages and point at the releases. APT resolves `Filename` against the
`sources.list` root and has no absolute form, so a Pages-hosted index can only
offer packages that are themselves on Pages. The archive repository is a flat
APT repo living beside the packages, reaching them with `../<tag>/<asset>`.

`publish-repo` runs the script on every release. To rebuild by hand — after
deleting a release, or if the indexes drift — dispatch **Rebuild Package
Repositories**. It is idempotent. Run it with `dry_run: true` first: that builds
everything, publishes nothing, and uploads the generated metadata as an artifact.
A real run needs `confirm: REBUILD`.

The rebuild refuses to shrink an index. Deleting a release does legitimately
drop its packages, but an incomplete read of the releases API looks the same and
would quietly drop versions that still exist — so the run fails and asks you to
confirm. Re-run it first; if the shrink really is intended, dispatch **Rebuild
Package Repositories** with the `allow_shrink` box ticked. A release workflow run
cannot override it, so after deliberately deleting a release, run the rebuild by
hand once before the next release.

Each run also re-downloads every package ever published (~1 GB today, growing
about 8 MB per release) because the indexes are derived from the packages
themselves. That is transient runner scratch, never committed, but it is what
the job's 60-minute bound has to accommodate as the archive grows.

Both writers share a `package-repositories` concurrency group, so a manual
rebuild and a release queue rather than interleave. Replacing the `apt-history`
assets is still not atomic — release assets cannot be swapped as a set — so a
client that fetches `InRelease` and `Packages.gz` across that window may need to
re-run `apt update`.

GitHub cancels a run that is still *pending* in a concurrency group when a newer
one joins it, even though the group sets `cancel-in-progress: false`. Release
three versions in quick succession and the middle `publish-repo` may show as
cancelled. A run cancelled while pending is harmless — each rebuild indexes every
release, so the last one to run covers the ones before it.

**A run that fails after it logs `publish APT history releases` is not
harmless.** The indexes and their signatures upload separately, so an
interruption between them leaves `apt-history` with a new `Packages` and an old
`InRelease`. Clients then get `Hash Sum mismatch`, which fails their whole
`apt update`, not just this source, and nothing repairs it on its own. Dispatch
**Rebuild Package Repositories** by hand as soon as you see it.

The rebuild reclaims space on the *published site*, which is what the 1 GB Pages
limit measures. It does not shrink the repository: the old package blobs stay in
`gh-pages` history, because the script commits on top of the branch rather than
replacing its history. That is deliberate — force-pushing an orphan would drop
commits from the benchmark and coverage workflows.

### Package signing

The two package formats need different things, because they check different
things:

| | What is signed | Verified by |
|---|---|---|
| **deb** | the repository's `Release` / `InRelease`, which covers each package through its hash | `apt update`, via `signed-by` |
| **rpm** | a signature *inside every package*, plus `repomd.xml.asc` | `gpgcheck=1` / `repo_gpgcheck=1` |

So a signed index is enough for apt and is not enough for dnf. rpms are signed
at build time by nfpm: the `build` job writes `secrets.GPG_PRIVATE_KEY` to a
file, points `RPM_SIGNING_KEY_FILE` at it, and passes the passphrase as
`NFPM_PASSPHRASE`.

nfpm signs only when that path resolves to a readable key, and **builds an
unsigned package with exit 0 when it does not** — a missing secret would ship a
release that no `gpgcheck=1` client can install. `scripts/check-rpm-signature.py`
runs straight after the build and fails the job if the signature is absent. A
wrong passphrase or an unreadable key file fails nfpm outright.

Verify a published package by hand with:

```bash
sudo rpm --import https://stuckj.github.io/mkvdup/yum/gpg-key.asc
rpm -Kv mkvdup-1.9.1-1.x86_64.rpm
# Header V4 EdDSA/SHA256 Signature, key ID cdb7b8f88afccbe3: OK   <- signed
# Header V4 EdDSA/SHA256 Signature, key ID cdb7b8f88afccbe3: NOKEY <- signed, key not imported
# digests OK (and no Signature line)                               <- NOT signed
```

Import the key first: without it a correctly signed package reports `NOKEY`,
which is easy to misread as unsigned.

### Re-signing already-published rpms

Every rpm published before signing existed is unsigned, which breaks
`gpgcheck=1` for exactly the old versions the archive repository exists to
serve. **Re-sign RPMs** (`resign-rpms.yml`) fixes them in place.

It runs in a Fedora container because `rpmsign` is not available on the Ubuntu
runner — and, worse, an earlier attempt there exited 0 having signed nothing.
For each published rpm it downloads the asset, signs it if it is not already
signed, and checks two things before anything is uploaded: that a signature is
now present, and that the main header and payload are **byte-identical** to the
original, so signing cannot quietly alter a package. Already-signed packages are
skipped, so re-running is cheap.

Nothing is uploaded unless `publish` is set *and* `confirm` is `RESIGN`. Run it
once without publishing first — the default — and read the counts.

Uploading replaces the only copy of each asset, and the checksums recorded in
the YUM repodata refer to the pre-signing bytes. **Dispatch "Rebuild Package
Repositories" immediately afterwards**, or every dnf install fails its checksum
check.

A full backfill is roughly 270 assets: about 500 MiB each way and a little over
500 API requests, against `GITHUB_TOKEN`'s budget of 1000 per hour per
repository. That fits in one hour but leaves little room, so avoid running a
release alongside it. The script refuses to start uploading if the remaining
budget is too small, and the `tags` input limits a run to named releases.

### Checking that installs actually work

**Verify RPM Installs** (`verify-rpm-install.yml`) installs from the live
repository across Fedora, Alma/Rocky 9 and Alma 10 exactly as README.md
documents, with `gpgcheck=1` and `repo_gpgcheck=1`, covering the current
version, a pinned older version and a canary. It is the only check that the
published instructions work end to end; the release workflow only proves a
signature exists, not that a real dnf accepts it.

EL8 is deliberately not in that matrix — see the note under
[Generate a GPG Key](#1-generate-a-gpg-key-if-you-dont-have-one).

### Migrating an existing client

The first rebuild removes `yum/packages/` and trims the Pages APT pool to the
current release. Clients hold cached metadata pointing at the old layout — dnf
keeps it for 48 hours by default — so until it expires an install can 404. To
refresh immediately:

```bash
sudo apt update                       # Debian/Ubuntu
sudo dnf clean metadata && sudo dnf makecache   # RHEL/Fedora
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
before `release`, so the tag can be created at the commit it produces.

The bump is committed on top of the **branch head**, which is not always the commit `prepare`
resolved: bookkeeping commits (`[skip ci]`, `[benchmark]` — benchmark baselines on `main`,
`vendorHash` refreshes on any branch) routinely land on top of it. That is fine, because those
commits do not change the built code — the same reason `prepare` passes over them when choosing
what to tag. So `sync-nix` syncs when the resolved commit is an ancestor of the head and everything
between the two is bookkeeping, and stands down otherwise: when the branch was rewritten
mid-release, or when a real commit landed while `build` was running. Both jobs read the pattern
from a single `BOOKKEEPING_COMMIT_RE` at the top of the workflow — if they ever disagreed about
which commits are bookkeeping, releases would silently skip the bump, which is exactly what #212
was.

Standing down emits a **warning** and a step summary naming the version the tag will actually
report. It is not a hard failure, because standing down is sometimes correct — but it must not be
quiet. `v1.9.0` shipped reporting `1.8.2` behind a green release whose only trace was a `::notice::`.

Unlike `nix-canary-hash.yml` it does *not* rebase and retry a rejected push, because rebasing would
move the bump onto commits that were never built. It fails instead, and the release should be
re-run.

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
- `Failed to create signatures: ... private key checksum failure` from nfpm means
  `GPG_PASSPHRASE` does not match `GPG_PRIVATE_KEY`
- "the rpm is not signed" from the build job means `RPM_SIGNING_KEY_FILE` was
  empty, so nfpm skipped signing and exited 0 — check the *Stage the rpm signing
  key* step ran
- `key 1 import failed` on a user's machine is not a key problem: their rpm
  predates 4.16 and cannot read ed25519 keys

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
