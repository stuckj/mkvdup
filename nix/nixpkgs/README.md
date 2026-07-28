# nixpkgs submission

`package.nix` in this directory is the **reference copy** of the mkvdup derivation as it is meant
to exist in [nixpkgs](https://github.com/NixOS/nixpkgs) at `pkgs/by-name/mk/mkvdup/package.nix`.

Nothing in this repo builds it — `flake.nix` and `default.nix` build from the working tree instead,
against whatever ref you point them at. This file exists so the upstream derivation is reviewable
alongside the code, and so a re-submission or a manual version bump has a correct starting point.

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

The commit message must be `mkvdup: init at <version>`, nixpkgs' convention for a new package,
and must carry the `Assisted-by:` trailer described above. Note this differs from this repo's own
convention — a `Co-authored-by:` trailer does **not** satisfy the nixpkgs policy.

### PR description

The policy requires the disclosure to appear in the pull request **separately** from the commit
trailer, so the description needs its own note. Something like:

```markdown
mkvdup deduplicates MKV files against their source media (DVD ISOs, Blu-ray backups), storing an
MKV as an index into the source plus whatever bytes are unique to it, and exposing the
reconstructed files through FUSE.

- Upstream: https://github.com/stuckj/mkvdup
- Pure Go, no cgo. Builds and tests run in `checkPhase` on Linux and Darwin.
- Runtime dependency on fuse3 for `fusermount3`; a `mount.fuse.mkvdup` helper is installed so the
  filesystem can be mounted from fstab.

I am the upstream author and am adding myself to `maintainers/maintainer-list.nix` in this PR.

Built and tested locally with `nix-build -A mkvdup` and `nixpkgs-review`.

Assistance disclosure, per the Automation/AI policy: the derivation was drafted with Claude Code
(model Claude Opus 5). I have reviewed it, understand it, and verified it builds and behaves
correctly; the same disclosure appears as an `Assisted-by:` trailer on the commit.
```

Adjust the last paragraph to match what actually happened before posting — the policy is about
accurate disclosure, not a fixed wording, and you are the one accountable for the claim.

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

## Things a reviewer may raise

- **`meta.platforms` is Linux and Darwin**, matching what upstream builds and tests. It has to be
  set explicitly: left unset it defaults to `go.meta.platforms`, which also contains
  `i686-freebsd`, `x86_64-freebsd`, `aarch64-freebsd`, `wasm32-wasi` and `wasm64-wasi` — none of
  which make sense for a FUSE filesystem that ships a bash `mount.fuse.*` helper.

  In practice this resolves to the Linux doubles plus `aarch64-darwin` only: nixpkgs has dropped
  `x86_64-darwin`, so `lib.platforms.darwin` no longer includes it. Intel macOS users get mkvdup
  from Homebrew or the release tarballs instead — nothing to do about it here.

  The Go test suite runs in `checkPhase` and is only exercised on Linux in this repo's CI, so a
  Darwin failure in ofborg is possible. Narrow `platforms` if that happens, rather than dropping
  the tests.
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
