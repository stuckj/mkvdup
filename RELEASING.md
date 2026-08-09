# Releasing mkvdup

This document describes how to create a new release of mkvdup.

## Prerequisites

Before your first release, set up GPG signing and — if you want the Arch packages to reach the
AUR — an AUR account:

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
one — re-running the job is the whole recovery, and nothing else has to be undone. `update-aur`
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
