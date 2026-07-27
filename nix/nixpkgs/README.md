# nixpkgs submission

`package.nix` in this directory is the **reference copy** of the mkvdup derivation as it is meant
to exist in [nixpkgs](https://github.com/NixOS/nixpkgs) at `pkgs/by-name/mk/mkvdup/package.nix`.

Nothing in this repo builds it — `flake.nix` and `default.nix` build the *canary* from the working
tree instead. This file exists so the upstream derivation is reviewable alongside the code, and so
a re-submission or a manual version bump has a correct starting point.

It pins an immutable release tag, so it is never affected by dependency drift on `main`.

## Who submits, and why it isn't automated

nixpkgs' [Automation/AI policy](https://github.com/NixOS/nixpkgs/blob/master/CONTRIBUTING.md#automationai-policy)
requires a **responsible person in the loop** who reviews a contribution and is accountable for it
*before* it is submitted, and who can answer reviewer questions directly. The PR must therefore be
opened from the maintainer's own GitHub account — the same handle recorded in
`maintainers/maintainer-list.nix` — not from a bot or assistant account.

Two disclosure rules apply on top of that:

- Commits produced with LLM assistance **must** carry an `Assisted-by:` trailer naming the tool and
  the primary model. A `Co-authored-by:` trailer does **not** satisfy the policy. (Note that this
  differs from this repo's own commit convention.)
- The PR description must disclose such assistance **separately** from the commit trailer.

## Submitting

```bash
# 1. Fork NixOS/nixpkgs to your own account, then clone your fork
git clone --filter=blob:none https://github.com/<you>/nixpkgs.git
cd nixpkgs
git checkout -b mkvdup-init

# 2. Apply the prepared commit, or copy the file in by hand
git am /path/to/mkvdup-nixpkgs-init.patch
#   equivalently:
#     mkdir -p pkgs/by-name/mk/mkvdup
#     cp <mkvdup>/nix/nixpkgs/package.nix pkgs/by-name/mk/mkvdup/package.nix
#     # then add your maintainer entry to maintainers/maintainer-list.nix

# 3. Verify (see below), then push and open the PR
git push -u origin mkvdup-init
```

Nothing needs to be added to `all-packages.nix` — the `pkgs/by-name/<shard>/<name>/` layout is
auto-discovered, and `mk` is the correct shard for `mkvdup`.

## Verifying before opening the PR

```bash
# Builds the package, runs the Go test suite in checkPhase
nix-build -A mkvdup
./result/bin/mkvdup --version                      # expect the packaged version

# Expected contents of ./result
#   bin/mkvdup
#   bin/mount.fuse.mkvdup
#   share/man/man1/mkvdup.1.gz                     # no @PACKAGE_NAME@ placeholders left
#   share/bash-completion/completions/mkvdup.bash
#   share/zsh/site-functions/_mkvdup
#   share/fish/vendor_completions.d/mkvdup.fish

# Formatting is enforced by CI
nix run nixpkgs#nixfmt-rfc-style -- pkgs/by-name/mk/mkvdup/package.nix

# Structural check for by-name additions
./ci/nixpkgs-vet.sh master

# Full review build (optional but the most thorough)
nix run nixpkgs#nixpkgs-review -- rev HEAD
```

The recorded `src.hash` covers an immutable tag, so it should never need refreshing. To confirm:

```bash
nix run nixpkgs#nix-prefetch-github -- stuckj mkvdup --rev v<version>
```

## Two things a reviewer may raise

- **`meta.platforms` is unset**, so it defaults to every platform Go supports, including Darwin.
  mkvdup does ship macOS binaries, so claiming Darwin is honest — but the Go test suite that runs
  in `checkPhase` is only exercised on Linux in this repo's CI. If ofborg's Darwin build fails,
  the fix is a one-liner: add `platforms = lib.platforms.linux;` to `meta`.
- **The maintainer entry includes an email address.** That field is optional in nixpkgs; drop the
  `email` line if you'd rather not have it in a heavily-scraped repository.

## After it merges

- Hydra builds binaries, so users get `nix profile install nixpkgs#mkvdup` with no local compile.
- The [`r-ryantm`](https://github.com/r-ryantm) bot opens version-bump PRs automatically after
  each release, and you are auto-requested for review as the listed maintainer. Approving those is
  the normal update path — see the Nix Maintenance section of [RELEASING.md](../../RELEASING.md).
- Changes to *how* mkvdup is built (new files in `postInstall`, new `subPackages`, a license
  change) still need a hand-written PR. Mirror any such edit back into this file so the two copies
  don't drift.
