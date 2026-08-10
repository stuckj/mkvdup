# Releasing mkvdup

This document describes how to create a new release of mkvdup.

## Prerequisites

Before your first release, set up GPG signing and — if you want the Arch packages to reach the
AUR — an AUR account:

### 1. Generate a GPG Key (if you don't have one)

```bash
gpg --full-generate-key
# Use your GitHub email address, no expiration
```

The key in use today is **ed25519**, not RSA. That choice has one consequence
worth knowing before rotating it: rpm gained EdDSA support in 4.16.0, so
EL8's rpm 4.14 cannot import an ed25519 key and cannot verify packages signed
with one. Measured on `almalinux:8` — `rpm --import` fails with
`key 1 import failed`, and a `gpgcheck=1` install then fails with
`GPG check FAILED`. RSA-4096 verifies everywhere including EL8. Generating a
new key means re-signing every published rpm and every user re-importing it, so
this is a decision to make deliberately rather than a default to drift into.

To re-sign after a rotation, dispatch **Re-sign RPMs** as usual — it compares
each package's signature against `GPG_KEY_ID` and re-signs anything carrying the
old key, so it resumes across runs exactly as a first backfill does. (`force`
exists to re-sign packages that are already correct, which is rarely wanted: it
disables the skip that makes a run resumable, so every run redoes the same first
releases and the backfill never reaches the end.)

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

### 4. Set Up AUR Publishing

Only the AUR half of the Arch release needs this. The pacman repository is signed with the same
GPG key as APT and YUM and needs nothing extra.

Do it **before** the first Arch release. A release without it still succeeds, but the README
and the published landing page both tell users to run `yay -S mkvdup` unconditionally, and
that finds nothing until the first push to the AUR has happened.

1. Create an account at [aur.archlinux.org](https://aur.archlinux.org) and add an SSH public
   key to it under My Account.
2. Add the matching private key as the `AUR_SSH_PRIVATE_KEY` secret.
3. Check that `mkvdup` and `mkvdup-canary` are unclaimed on the AUR. A name nobody owns clones as
   an empty repository and is created by the first push, which the workflow does on its own —
   but a name someone else owns will refuse the push.

Without the secret the `update-aur` job logs a warning and succeeds without publishing, so
releases still work — they just do not reach the AUR.

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
2. Build packages for amd64 and arm64 — deb, rpm and tarballs in `build`
3. Create the GitHub release **and the tag together**, pointing at the resolved commit
4. Build the Arch package in `build-arch`, which compiles from the tag's source archive and so
   has to run after the tag exists
5. Update the APT and YUM repositories on GitHub Pages, the pacman repository in its own
   release, and the Homebrew and AUR files

Two things that list does not convey. The Nix version bump is not in that order at all:
`sync-nix` runs *before* the release, so the tag it creates already contains it, which is the
ordering #212 established. And steps 4 and 5 are not serial — `publish-repo` and
`update-homebrew` need only the release, so they run alongside `build-arch`; only the pacman
repository and the AUR wait for it.

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
- Publishes to the canary APT/YUM/pacman repositories (separate from stable)
- Publishes `mkvdup-canary` to the AUR (separate package from `mkvdup`)

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

Requires rpm 4.16 or newer, as the stable repository does — see the
[note on the signing key](#1-generate-a-gpg-key-if-you-dont-have-one).

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

#### Arch Linux - Canary

```bash
curl -fsSL https://stuckj.github.io/mkvdup/gpg-key.asc | sudo pacman-key --add -
sudo pacman-key --lsign-key 3AABF4C834FFE7E08D91A9BACDB7B8F88AFCCBE3

sudo tee -a /etc/pacman.conf << 'EOF'

[mkvdup-canary]
SigLevel = Required
Server = https://github.com/stuckj/mkvdup/releases/download/pacman-canary-$arch
EOF

sudo pacman -Syu mkvdup-canary
```

Or from the AUR: `yay -S mkvdup-canary`.

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
| mkvdup-VERSION-1-x86_64.pkg.tar.zst | x86_64 | Arch Linux |

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

Requires **rpm 4.16 or newer** (Fedora, RHEL/Alma/Rocky 9 and 10) — see the
[note on the signing key](#1-generate-a-gpg-key-if-you-dont-have-one).

This `.repo` snippet is published in **six** places — stable and canary, in
`README.md`, in this file, and in the landing page `scripts/repo-index.sh`
generates. All six carry the caveat, and they change together. Two further
copies live in `.github/workflows/verify-rpm-install.yml`, which installs from
them; those need no caveat but do need the same `baseurl`.

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

### Arch Linux

```bash
curl -fsSL https://stuckj.github.io/mkvdup/gpg-key.asc | sudo pacman-key --add -
sudo pacman-key --lsign-key 3AABF4C834FFE7E08D91A9BACDB7B8F88AFCCBE3

sudo tee -a /etc/pacman.conf << 'EOF'

[mkvdup]
SigLevel = Required
Server = https://github.com/stuckj/mkvdup/releases/download/pacman-$arch
EOF

sudo pacman -Syu mkvdup
```

Or from the AUR: `yay -S mkvdup`.

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

All three writers share a `package-repositories` concurrency group, so a manual
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
# Header V4 EdDSA/SHA256 Signature, key ID 8afccbe3: OK     <- signed
# Header V4 EdDSA/SHA256 Signature, key ID 8afccbe3: NOKEY  <- signed, key not imported
# digests OK, with no Signature line at all                 <- NOT signed
```

Import the key first: without it a correctly signed package reports `NOKEY`,
which is easy to misread as unsigned.

That wording is rpm 4's (EL9, EL10). rpm 6 — Fedora 43 and newer — prints
`... OpenPGP ...` and, once the key is imported, gives the full fingerprint
rather than the short key id. The distinction that matters is the same either
way: a `Signature`/`OpenPGP` line at all means signed, `digests OK` alone means
not.

### Re-signing already-published rpms

Every rpm published before signing existed is unsigned, which breaks
`gpgcheck=1` for exactly the old versions the archive repository exists to
serve. **Re-sign RPMs** (`resign-rpms.yml`) fixes them in place.

It runs in a Fedora container because signing needs `rpmsign` built with gpg
support. It adds only `%_gpg_sign_cmd_extra_args` and leaves rpm's own signing
command alone, because that command's shape is version-specific: rpm 6 made it
parametric and stopped repeating `gpg` as `argv[0]`; it also reads the identity
from `%_openpgp_sign_id`, which defaults to `%_gpg_name`, so the script sets
both. Measured on both generations —
replacing the command signs on rpm 4.20.1 and fails on rpm 6.0.2 with
`/usr/bin/gpg exec failed (2)`, while the extra-args form signs on both, leaving
the package byte-identical apart from a 128-byte signature.

It signs with `--resign` rather than `--addsign` for the same reason: rpm 6
refuses to add a second header signature and deletes the existing one only
under `--resign`, which is what re-signing after a key rotation needs. rpm 4
treats the two as identical (measured: `--addsign` on an already-signed package
exits 0 on rpm 4.20.1 and 1 on rpm 6.0.2; `--resign` works on both). For each published rpm it downloads the asset, checks the download is
the size the release says it is, signs it if it is not already signed, and
checks two more things before anything is uploaded: that a signature is now
present, and that the main header and payload are **byte-identical** to the
original, so signing cannot quietly alter a package. `rpmsign` exits 0 when it
has changed nothing, so none of these rely on its exit status. Already-signed
packages are skipped, so re-running is cheap — and a run that stops partway
resumes.

Nothing is uploaded unless `publish` is set *and* `confirm` is `RESIGN`. Run it
once without publishing first — the default — and read the counts.

Replacing an asset is a **delete followed by an upload** — the GitHub API has
no atomic replace — and the release asset is the only copy of that package. So:

- Uploads are retried, and a final pass re-reads every release to confirm each
  replacement is actually present.
- The API budget is checked **before each release**, not once at the start, and
  a run that cannot afford the next one stops with every release it started
  finished. Because already-signed packages are skipped, re-running later simply
  resumes.
- If a publish run fails or is cancelled, the signed packages are uploaded as
  the `resigned-recovery` artifact — the container is otherwise destroyed
  holding the only remaining copy of anything already deleted.

**A full backfill does not fit in one run, by design.** GitHub allows "no more
than 80 content-generating requests per minute and no more than 500
content-generating requests per hour". Replacing an asset is a delete plus an
upload, so the ~278 published rpms need ~556 writes — more than one hour allows.

Rather than run until a 403 lands between a delete and its upload, the script
tracks the writes it has issued and stops cleanly at `WRITE_BUDGET` (450),
finishing whatever release it was on. That run exits **0** with a warning, the
repositories are rebuilt for what did change, and re-running an hour later
carries on: already-signed packages are skipped. Expect **two runs** for the
first full backfill.

It also paces itself for the per-minute half of that limit: a release costs
four writes (a delete and an upload for each of its two rpms), so the five
seconds between them keeps the rate near 48 a minute. A failed upload backs off
0/60/300/900 seconds.

The looser `GITHUB_TOKEN` budget of 1000 core requests per hour is checked too
— roughly 690 for a whole backfill — but it is not usually the binding one.
Still, do not run a release alongside this.

The `tags` input limits a run to named releases, matched exactly; an unknown tag
fails the run rather than narrowing it silently.

Every replaced asset gets a new sha256 while the published repodata still
records the old one, so the workflow runs `rebuild-package-repos.sh` in a
following job — after a successful run, a stop-short, **and a failure**. A
failure partway has already replaced some assets, and skipping the rebuild would
leave dnf refusing them on a checksum mismatch: worse than the unsigned state.
Rebuilding a repository that has genuinely lost a package is what
`check_no_shrink_yum` prevents; it compares the published package set against
the release assets and refuses, naming what went missing.

The one case that does *not* rebuild automatically is a **cancelled** run, which
may be mid-upload. Dispatch **Rebuild Package Repositories** by hand afterwards.
A manual invocation of the script must likewise dispatch the rebuild itself.

**Recovering a package that was deleted but never replaced.** The workflow gets
a fresh work directory each dispatch, so the script's own orphan detection —
which spots a signed copy on disk whose release no longer lists it — only fires
if you give it that directory back. Download the `resigned-recovery` artifact,
unpack it over an empty work directory (it carries `signed/`, the manifests and
the `.resign-marker` the script needs to adopt the directory), and re-run with
`publish` set; outstanding packages are uploaded before anything else — though
only once the signing pass over the rest has finished without error, since
nothing is uploaded from a run that failed to sign something. Or just
`gh release upload <tag> <file>` them by hand. `check_no_shrink_yum` will refuse
to rebuild the repositories until they are back either way.

### Checking that installs actually work

**Verify RPM Installs** (`verify-rpm-install.yml`) installs from the live
repository across Fedora, Alma/Rocky 9 and Alma 10 using the `.repo` snippet
README.md publishes — `gpgcheck=1`, no `repo_gpgcheck` — covering the current
version, a pinned older version and a canary. It then asserts each installed
package records a signing key ID, so the check cannot pass on an unsigned
package if `gpgcheck` ever stops being enforced. A final step re-runs the
install with `repo_gpgcheck=1`, labelled as stricter than anything documented.

It is the only check that the published instructions work end to end; the
release workflow only proves a signature exists on the package it just built,
not that a real dnf accepts what is on the server.

**It is dispatch-only.** Nothing runs it after a release or on a schedule, so it
does not by itself prevent another #220 — it makes the check one dispatch away.
Adding a `schedule:` trigger would close that gap.

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

## Arch Maintenance

Arch is served two ways, mirroring how the other platforms are split: the AUR is the analogue
of the Homebrew tap (a recipe in an external repo) and the pacman repository is the analogue of
the APT and YUM repositories (signed packages we host). Both are generated from one template,
[`packaging/arch/PKGBUILD.in`](packaging/arch/PKGBUILD.in), and both are fully automated. Edit
the template, never a published copy.

### Where the pacman repository lives

Not on GitHub Pages — in releases of its own, one per channel:

| Release | `Server` line users add |
|---------|-------------------------|
| `pacman-x86_64` | `.../releases/download/pacman-$arch` |
| `pacman-canary-x86_64` | `.../releases/download/pacman-canary-$arch` |

**x86_64 only.** Arch is an x86_64 distribution and publishes no aarch64 container, so there is
nothing to build an aarch64 package in. The PKGBUILD still declares `aarch64`, so Arch Linux ARM
users install from the AUR and build it natively — which is how ALARM users install everything.
The `$arch` in the `Server` line is left in place so the layout does not have to change if that
ever becomes possible.

That shape is forced, not chosen. libalpm validates `%FILENAME%` and rejects any value
containing a `/` (`_alpm_validate_filename` in `be_sync.c`), so a database entry cannot point at
a package anywhere but beside the database itself. APT's `Filename` has no such restriction,
which is why an APT index hosted on Pages can reach packages in a sibling release with a
relative `../<tag>/<asset>` and a pacman database cannot. A GitHub release is one flat
namespace, so it can be the directory a pacman repository requires; Pages would have to hold the
packages too, which is the thing that filled it up.

One consequence worth knowing: pacman derives the URL of a package's detached signature from
the URL it *ended up* at after redirects, but only when that URL's last path segment contains
`.db` or `.pkg`. GitHub redirects release assets to an opaque blob path with the filename only
in the query string, so the test fails and pacman falls back to the original URL — which is what
makes `<asset>.sig` resolve. Were the CDN path ever to end in the asset name instead, pacman
would append `.sig` to the whole signed URL, query string included, and the request would be
rejected as a bad signature rather than resolving to anything.

Because the whole channel is release assets, a release also never has to wait on the Pages build,
and nothing about Arch counts against the Pages size limit.

The template's header is copied verbatim into what the AUR publishes, so it is written for
someone reading it there. Nothing in the template may use an at-delimited word outside the
substituted set — including in a comment — because the workflow rejects a generated PKGBUILD
that still matches one.

Three things about the naming are load-bearing:

- **The AUR package builds from source, and is called `mkvdup` rather than `mkvdup-bin`.**
  That is what makes it eligible to be adopted into Arch's own repositories: Arch builds what
  it ships, so a prebuilt package has nothing for a Package Maintainer to promote. The AUR
  submission guidelines *require* the `-bin` suffix for prebuilt deliverables when the sources
  are available, so shipping a binary there would also have forced the name that closes that
  door. Using the same name in the hosted repository means pacman treats it as an upgrade of an
  already-installed AUR build rather than a conflict to resolve by hand.
- **`pkgver` replaces `-` with `_`.** A pacman `pkgver` cannot contain a hyphen — that is the
  `pkgver`/`pkgrel` separator — so `1.2.0-canary.1` is packaged as `1.2.0_canary.1`. Canaries
  are their own package, as they are for deb and rpm, so that version never has to sort against
  a stable one.

  **Only `-canary.` gets that separation, so use no other pre-release suffix.** The version
  regex would accept `1.9.2-beta.1`, and it would build as stable `mkvdup` `1.9.2_beta.1`.
  pacman has no equivalent of Debian's `~`, so `vercmp` sorts that *above* `1.9.2` — APT would
  decline the beta as an upgrade while pacman pushed it to every stable user on their next
  `pacman -Syu`. Cut pre-releases as canaries.
- **The pacman database file is named after the `[section]` users add to `pacman.conf`**
  (`mkvdup.db` in the `pacman-*` releases, `mkvdup-canary.db` in the `pacman-canary-*` ones).
  Renaming either — or renaming a release — breaks every configured client, so both have to
  change in lockstep with the documented instructions.

The mount helper installs to `/usr/bin/mount.fuse.mkvdup` rather than the `/usr/sbin` the deb
and rpm use: Arch symlinks `/usr/sbin` to `/usr/bin` and packages may not write there.

`build-arch` runs *after* the release, unlike every other build. It has to: `source=()` points at
the tag's source archive, so the tag must exist before the checksum can be taken or makepkg can
fetch it. The cost is that an Arch failure lands on a published release rather than preventing
one. Re-running the job recovers a transient failure, but not a code fix: it checks out the
commit the release was cut from, so a fix needs a new canary rather than a re-run. `update-aur`
then downloads that same URL again and compares it against the sum in the PKGBUILD before
pushing, so a source archive that does not resolve stops the AUR publish rather than reaching
users who would build from it.

Signatures are the same GPG key as APT and YUM, but detached *binary* rather than ASCII-armoured:
`repo-add --include-sigs` refuses to record an armoured package signature in the database. pacman
hands signatures to gpgme and would accept either.

The pacman repository keeps only the current version — its database holds a single entry per
package name, so an older file is not installable through pacman anyway, and keeping it would be
the dead weight the APT pool accumulated. Every version stays on its own `v*` release regardless;
the `pacman-*` releases hold a second copy of the current one and nothing else.

[`scripts/publish-pacman-repo.sh`](scripts/publish-pacman-repo.sh) rebuilds them from the
release assets. It reads the releases
rather than any branch, so it is idempotent, it repairs drift, and a package whose release was
deleted drops out. It selects the highest version present with `vercmp` rather than trusting the
release that triggered it, which is what keeps a backport release from regressing the channel —
the same protection `repo-add --prevent-downgrade` gives, but from the data rather than a flag.
Run it by hand with **Actions → Rebuild pacman Repositories**; `DRY_RUN=1` builds everything and
publishes nothing.

### Local Testing (Arch)

Needs an Arch system or an `archlinux:base-devel` container, plus `go` and `namcap`
(`pacman -S go namcap`) — `base-devel` supplies makepkg but no Go toolchain. **makepkg refuses to run as root**, which is what that container gives you — create
an unprivileged user first, as the workflow does, and give it access to the checkout:

```bash
useradd --create-home builder
chgrp -R builder . && chmod -R g+rX .
su builder   # keeps the working directory, unlike `su -`
```

Run the same script the workflow runs. It generates the PKGBUILD from the template, downloads
the tag's source archive to take its checksum, and builds it:

```bash
VERSION=1.9.1 TAG=v1.9.1 ./scripts/build-arch-package.sh /tmp/archbuild
namcap /tmp/archbuild/PKGBUILD /tmp/archbuild/*.pkg.tar.zst
```

Set `IS_CANARY=true` to build the canary package instead. Any released tag works — the package
builds from source, so it does not depend on what a particular release happened to attach.

Substituting the placeholders by hand is not worth doing: the set they come from is defined in
that script, and a hand-written `sed` drifts from it silently. The script refuses to overwrite a
directory it did not create, so point it somewhere disposable.

`.SRCINFO` is not needed to build, only to publish: the AUR rejects a push whose tree lacks one,
or whose `.SRCINFO` names a `pkgbase` other than the repository being pushed to. It does not
check the `.SRCINFO` against the PKGBUILD, so a stale one is accepted and is what the AUR page
and helpers then show — regenerate it whenever the PKGBUILD changes. The workflow does.

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
- `UNSIGNED  ./mkvdup-…rpm` and `1 of 1 package(s) carry no signature` from the
  build job means `RPM_SIGNING_KEY_FILE` was empty, so nfpm skipped signing and
  exited 0 — check the *Stage the rpm signing key* step ran
- `key 1 import failed` on a user's machine is not a key problem: their rpm
  predates 4.16 and cannot read ed25519 keys

### Repository Not Updating

- Ensure GitHub Pages is enabled
- Check that `gh-pages` branch exists
- Verify the workflow has `pages: write` permission

### AUR Publish Skipped or Failing

- A "AUR_SSH_PRIVATE_KEY secret not configured" warning means the secret is unset; the rest of
  the release is unaffected. See [Prerequisites](#4-set-up-aur-publishing).
- `Permission denied (publickey)` means the private key in the secret does not match a public
  key on the AUR account.
- A rejected push on a package name that clones empty means someone else owns the name.
- A failure in "Verify the source archive the PKGBUILD points at" means the tag's source
  archive did not resolve, or did not match the checksum recorded when it was built. The
  release itself is complete at that point; only the AUR is behind.
- "is older than the ... already on the AUR" means a backport release declined to move the AUR
  backwards, the same way the pacman channel declines. Every other channel has the release.

### pacman Reporting a Corrupted Database

- `pacman -Sy` calling the database invalid or corrupted usually means the `.db` asset is not a
  plain file. `repo-add` leaves `.db` and `.files` as symlinks to the tarballs, which is why
  `publish-pacman-repo.sh` replaces them with copies before uploading.
- A 404 on the database means the `pacman-*` release does not exist yet. It is created by the
  first run of the publish script, which needs at least one `v*` release carrying Arch packages.
- A signature error after `pacman-key --add` means the key was never locally signed;
  `SigLevel = Required` implies `TrustedOnly`, so `pacman-key --lsign-key` is not optional.

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
