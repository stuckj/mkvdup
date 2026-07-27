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
# Only edits the two files; staging and committing is left to the caller.
#
# Requires: nix (with flakes enabled).

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

nix_files=(flake.nix default.nix)

read_hash() {
  sed -n 's/.*vendorHash = "\(sha256-[^"]*\)".*/\1/p' "$1" | head -1
}

# Portable in-place edit: BSD/macOS sed reads the argument after -i as a backup
# extension, so `sed -i <expr> <file>` fails there. This script is documented as
# hand-runnable, so don't depend on GNU sed. Write back with `cat` rather than
# `mv` so the file keeps its original mode and inode — mktemp creates 0600.
# mktemp gets an explicit template too: the bare form is a GNU extension that
# BSD/macOS mktemp rejects.
write_hash() {
  local file=$1 hash=$2 tmp
  tmp=$(mktemp "${TMPDIR:-/tmp}/mkvdup-vendor-hash.XXXXXX")
  sed "s|vendorHash = \"sha256-[^\"]*\";|vendorHash = \"${hash}\";|" "$file" >"$tmp"
  cat "$tmp" >"$file"
  rm -f "$tmp"
}

# Nothing ever builds default.nix — not this script, which builds the flake, and
# not CI. Its hash matching flake.nix's is therefore the only thing keeping
# `nix-build` working for non-flake users, so check it explicitly rather than
# inferring it from a successful flake build. The two always carry the same
# value: same go.mod/go.sum, and every writer below sets both.
sync_default() {
  local flake_hash=$1 default_hash
  default_hash=$(read_hash default.nix)
  if [[ -z "$default_hash" ]]; then
    echo "error: no vendorHash found in default.nix" >&2
    exit 1
  fi
  if [[ "$default_hash" != "$flake_hash" ]]; then
    write_hash default.nix "$flake_hash"
    echo "default.nix was out of sync (${default_hash}); set to ${flake_hash}" >&2
  fi
}

current=$(read_hash flake.nix)
if [[ -z "$current" ]]; then
  echo "error: no vendorHash found in flake.nix" >&2
  exit 1
fi

if nix build .#mkvdup-canary --no-link >/dev/null 2>&1; then
  sync_default "$current"
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
  write_hash "$f" "$new_hash"
done

# Prove the new hash is right rather than trusting the parse.
if ! nix build .#mkvdup-canary --no-link >/dev/null 2>&1; then
  echo "error: build still fails after writing vendorHash ${new_hash}" >&2
  exit 1
fi

echo "vendorHash updated: ${current} -> ${new_hash}" >&2
echo "$new_hash"
