# mkvdup in nixpkgs

The mkvdup derivation lives in [nixpkgs](https://github.com/NixOS/nixpkgs) at
`pkgs/by-name/mk/mkvdup/package.nix`. **That is the only copy.** This directory holds notes, not
code.

Submitted as [NixOS/nixpkgs#547640](https://github.com/NixOS/nixpkgs/pull/547640). Until that
merges, the branch behind it is the working copy; edit it in a nixpkgs checkout, not here.

Do not reintroduce a copy of `package.nix` in this repo. There was one, and it was deleted: nothing
built it, no workflow or script bumped it, and it silently fell three releases behind `main` while
looking authoritative. `r-ryantm` bumps the upstream file and would never have touched a copy here,
so the two could only ever drift apart. If you need to see the derivation, read it in nixpkgs.

## Maintenance

- `r-ryantm` opens version-bump PRs after each release; you are auto-requested as the listed
  maintainer. Approving those is the normal update path.
- Changes to *how* mkvdup is built — new files in `postInstall`, a new `subPackages` entry, a
  licence change — need a hand-written nixpkgs PR. The bot only bumps `version`, `hash` and
  `vendorHash`.
- Hydra builds binaries once merged, so users get `nix profile install nixpkgs#mkvdup` with no
  local compile. See [RELEASING.md](../../RELEASING.md) for the user-facing install docs.

## Attribution rules for any nixpkgs PR

nixpkgs' [Automation/AI policy](https://github.com/NixOS/nixpkgs/blob/master/CONTRIBUTING.md#automationai-policy)
requires a **responsible person in the loop** who reviews the contribution, understands it, and can
answer reviewer questions — so the PR must come from the maintainer's own account, the same handle
in `maintainers/maintainer-list.nix`, never a bot or assistant account.

Two disclosures are required, and they are separate:

- Commits produced with LLM assistance carry an `Assisted-by:` trailer naming the tool and primary
  model. A `Co-authored-by:` trailer does **not** satisfy the policy — note this differs from this
  repo's own commit convention.
- The PR description discloses the assistance in its own right, not just via the trailer.

## What CI enforces

Learned the hard way on #547640 — each of these failed there first:

- **`__structuredAttrs = true;` is mandatory for new packages.** `nixpkgs-vet` fails with
  [NPV-166](https://github.com/NixOS/nixpkgs-vet/wiki/NPV-166) without it. `nix-build` will not
  catch this: the package builds identically either way, so it only surfaces once the PR is open
  unless you run the vet yourself.
- **No merge commits.** The commit lint rejects a `Merge branch …` subject for not matching the
  `type: subject` convention, and separately reports that merging is discouraged. If the branch
  falls behind, rebase onto the base branch — do not use GitHub's "Update branch" button.
- **`nixfmt` formatting.**
- The commit subject must be `mkvdup: <version>` (or `mkvdup: init at <version>` for a new
  package), and nothing needs adding to `all-packages.nix` — the `pkgs/by-name/<shard>/<name>/`
  layout is auto-discovered.

## Verifying a nixpkgs change locally

From a nixpkgs checkout with the change applied:

```bash
nix-build -A mkvdup && ./result/bin/mkvdup --version

# Expected contents of ./result
#   bin/mkvdup
#   bin/mount.fuse.mkvdup
#   share/man/man1/mkvdup.1.gz                     # no @PACKAGE_NAME@ placeholders left
#   share/bash-completion/completions/mkvdup.bash
#   share/zsh/site-functions/_mkvdup
#   share/fish/vendor_completions.d/mkvdup.fish

nix run nixpkgs#nixfmt-rfc-style -- pkgs/by-name/mk/mkvdup/package.nix
nix run nixpkgs#nixpkgs-review -- pr <number>      # must be run from inside a nixpkgs checkout
```

`nixpkgs-vet` takes a **base checkout path**, not a revision, which its help does not make obvious:

```bash
git worktree add --detach /tmp/vet-base HEAD~1
nix shell nixpkgs#nixpkgs-vet -c nixpkgs-vet --base /tmp/vet-base .
```

To confirm the `src.hash` for a tag:

```bash
nix run nixpkgs#nix-prefetch-github -- stuckj mkvdup --rev v<version>
```

## Decisions a reviewer may question

- **`meta.platforms` is Linux and Darwin**, set explicitly. Left unset it defaults to
  `go.meta.platforms`, which also contains `i686-freebsd`, `x86_64-freebsd`, `aarch64-freebsd`,
  `wasm32-wasi` and `wasm64-wasi` — none of which make sense for a FUSE filesystem shipping a bash
  `mount.fuse.*` helper.

  In practice this resolves to the Linux doubles plus `aarch64-darwin` only: nixpkgs has dropped
  `x86_64-darwin`, so `lib.platforms.darwin` no longer includes it. Intel macOS users get mkvdup
  from Homebrew or the release tarballs instead.

  The Go test suite runs in `checkPhase` but is only exercised on Linux in this repo's CI, so a
  Darwin failure in ofborg is possible. Narrow `platforms` if that happens, rather than dropping
  the tests.
- **The maintainer entry includes an email address.** That field is optional in nixpkgs; drop the
  `email` line if you would rather not have it in a heavily-scraped repository.
