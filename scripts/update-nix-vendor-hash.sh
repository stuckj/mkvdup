#!/usr/bin/env bash
#
# Recompute the canary vendorHash in flake.nix and default.nix.
#
# buildGoModule pins the hash of the fetched Go module set, so any change to
# go.mod/go.sum invalidates it and `nix build .#mkvdup-canary` starts failing.
# This is run from two places:
#
#   - the release workflow, after the version strings are bumped
#   - the nix-canary-hash workflow, whenever go.mod/go.sum lands on main
#
# Writes the hash actually in use to stdout; progress goes to stderr. Exits
# non-zero if the hash could not be determined.
#
# Requires: nix (with flakes enabled), git.

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

nix_files=(flake.nix default.nix)

# Flakes only see files git knows about. A caller may have just rewritten the
# version strings, so stage the Nix files to be sure nix builds what's on disk.
git add "${nix_files[@]}"

read_hash() {
  sed -n 's/.*vendorHash = "\(sha256-[^"]*\)".*/\1/p' "$1" | head -1
}

current=$(read_hash flake.nix)
if [[ -z "$current" ]]; then
  echo "error: no vendorHash found in flake.nix" >&2
  exit 1
fi

if nix build .#mkvdup-canary --no-link >/dev/null 2>&1; then
  echo "vendorHash is already current: ${current}" >&2
  echo "$current"
  exit 0
fi

# The build failed. Nix reports the correct hash in the mismatch error, but it
# may also have failed for an unrelated reason — only treat it as a hash
# refresh if a mismatch is actually what we got.
build_output=$(nix build .#mkvdup-canary --no-link 2>&1 || true)

if ! grep -q "hash mismatch in fixed-output derivation" <<<"$build_output"; then
  echo "error: nix build failed for a reason other than a vendorHash mismatch:" >&2
  echo "$build_output" >&2
  exit 1
fi

new_hash=$(awk '/got:/ { print $2; exit }' <<<"$build_output")
if [[ -z "$new_hash" ]]; then
  echo "error: could not parse the expected hash out of the build output:" >&2
  echo "$build_output" >&2
  exit 1
fi

for f in "${nix_files[@]}"; do
  sed -i "s|vendorHash = \"sha256-[^\"]*\";|vendorHash = \"${new_hash}\";|" "$f"
done
git add "${nix_files[@]}"

# Prove the new hash is right rather than trusting the parse.
if ! nix build .#mkvdup-canary --no-link >/dev/null 2>&1; then
  echo "error: build still fails after writing vendorHash ${new_hash}" >&2
  exit 1
fi

echo "vendorHash updated: ${current} -> ${new_hash}" >&2
echo "$new_hash"
