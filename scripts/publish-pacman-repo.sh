#!/usr/bin/env bash
# Rebuild the pacman repositories from the release assets.
#
# A pacman repository has to be one flat directory: libalpm validates %FILENAME%
# and rejects anything containing a '/', so a package cannot be addressed in a
# sibling location the way APT's Filename can. Database and packages therefore
# live together, and a GitHub release is exactly one flat namespace -- so each
# channel/architecture gets a release of its own to be that directory:
#
#   pacman-x86_64          Server = .../releases/download/pacman-$arch
#   pacman-aarch64
#   pacman-canary-x86_64   Server = .../releases/download/pacman-canary-$arch
#   pacman-canary-aarch64
#
# Only the current version is served. A pacman database holds one entry per
# package name, so an older .pkg.tar.zst in the repository is not installable
# through pacman at all -- it would be the same dead weight the APT pool
# accumulated. Every version stays on its own v* release regardless.
#
# Like rebuild-package-repos.sh, this reads the releases rather than any branch,
# so it is idempotent, it repairs drift, and a package whose release was deleted
# drops out. It picks the highest version present rather than trusting a caller,
# which is what keeps a re-released older version from regressing a channel.
#
# Requires: gh (authenticated), repo-add and vercmp from pacman >= 6.1, and gpg
# with the signing key imported. Run it inside archlinux:base -- Ubuntu ships
# pacman 6.0.2, whose repo-add has no --include-sigs.
set -euo pipefail

REPO="${GITHUB_REPOSITORY:-stuckj/mkvdup}"
WORK="${1:-$PWD/.pacmanbuild}"
KEY="${GPG_KEY_ID:?GPG_KEY_ID must be set}"
export GH_TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
# DRY_RUN=1 builds every repository and publishes nothing, so a run can be
# inspected before it replaces what users are installing from.
DRY_RUN="${DRY_RUN:-0}"

say() { printf '\n== %s\n' "$1"; }
gpg_sign() {  # detached *binary* signature: repo-add --include-sigs refuses an armoured one
  printf '%s' "${GPG_PASSPHRASE:-}" | gpg --batch --yes --passphrase-fd 0 \
    --pinentry-mode loopback --local-user "$KEY" --detach-sign --no-armor -o "$1.sig" "$1"
}

rm -rf "$WORK"; mkdir -p "$WORK"; cd "$WORK"

say "enumerate Arch packages across the v* releases"
# apt-history*/pacman-* are output of this script and its sibling, never sources.
# shellcheck disable=SC2016  # $t is jq's variable, not the shell's
gh api "repos/$REPO/releases" --paginate \
  -q '.[] | select(.tag_name | startswith("v")) | .tag_name as $t | .assets[] | "\($t)/\(.name)"' \
  | grep -E '\.pkg\.tar\.zst$' | sort -u > assetmap.txt || true
echo "  $(wc -l < assetmap.txt) package assets"

build_channel() {  # build_channel <pkgname> <arch> <tag> <dbname>
  local pkgname=$1 arch=$2 tag=$3 db=$4
  local dir="$WORK/$tag" best="" best_line="" ver line name

  # Every asset for this package name and architecture, newest wins. Matching on
  # the full "<pkgname>-" prefix keeps mkvdup-bin from also claiming
  # mkvdup-canary-bin, whose name merely contains it.
  while IFS= read -r line; do
    name=${line#*/}
    case "$name" in
      "$pkgname"-*"-$arch.pkg.tar.zst") ;;
      *) continue ;;
    esac
    ver=${name#"$pkgname"-}
    ver=${ver%"-$arch.pkg.tar.zst"}
    if [ -z "$best" ] || [ "$(vercmp "$ver" "$best")" -gt 0 ]; then
      best=$ver; best_line=$line
    fi
  done < assetmap.txt

  if [ -z "$best" ]; then
    echo "  $tag: no $pkgname package on any release yet, skipping"
    return 0
  fi

  mkdir -p "$dir"
  gh release download "${best_line%%/*}" --repo "$REPO" --dir "$dir" \
     --pattern "${best_line#*/}" --clobber >/dev/null
  gpg_sign "$dir/${best_line#*/}"

  ( cd "$dir"
    repo-add --include-sigs "$db.db.tar.gz" "${best_line#*/}" >/dev/null
    for ext in db files; do
      gpg_sign "$db.$ext.tar.gz"
      # repo-add leaves $db.db and $db.files as symlinks. A release asset is a
      # plain file upload, so publish copies -- and re-sign nothing, the copy is
      # byte-identical to what was just signed.
      rm -f "$db.$ext" "$db.$ext.sig"
      cp "$db.$ext.tar.gz" "$db.$ext"
      cp "$db.$ext.tar.gz.sig" "$db.$ext.sig"
      rm -f "$db.$ext.tar.gz.old"*
    done )
  # What the database says it serves has to be what is uploaded beside it: a
  # signed database naming an absent file gives clients a 404 they cannot tell
  # from an attack.
  local referenced
  referenced=$(bsdtar -xOf "$dir/$db.db.tar.gz" '*/desc' | awk '/^%FILENAME%$/ {getline; print}')
  [ -n "$referenced" ] || { echo "::error::$tag: database lists no package files"; return 1; }
  while read -r want; do
    [ -f "$dir/$want" ] || { echo "::error::$tag: database references $want, which was not built"; return 1; }
  done <<<"$referenced"

  echo "  $tag: $pkgname $best (from ${best_line%%/*})"
  printf '%s\n' "$best" > "$dir/.version"
}

say "build repositories"
build_channel mkvdup-bin        x86_64  pacman-x86_64         mkvdup
build_channel mkvdup-bin        aarch64 pacman-aarch64        mkvdup
build_channel mkvdup-canary-bin x86_64  pacman-canary-x86_64  mkvdup-canary
build_channel mkvdup-canary-bin aarch64 pacman-canary-aarch64 mkvdup-canary

say "publish"
for dir in "$WORK"/pacman-*; do
  [ -d "$dir" ] || continue
  tag=$(basename "$dir")
  if [ "$DRY_RUN" = 1 ]; then
    echo "  DRY RUN: would publish $(find "$dir" -maxdepth 1 -type f | wc -l) assets -> $tag"
    continue
  fi

  case "$tag" in
    pacman-canary-*) section=mkvdup-canary ;;
    *)               section=mkvdup ;;
  esac
  # pacman-canary-x86_64 -> pacman-canary; pacman-x86_64 -> pacman. The Server
  # line users copy keeps $arch literal, for pacman to substitute.
  family=${tag%-*}
  gh release view "$tag" --repo "$REPO" >/dev/null 2>&1 || \
    gh release create "$tag" --repo "$REPO" --prerelease \
      --title "pacman repository (${tag#pacman-})" \
      --notes "Rolling pacman repository. Holds the current version only; every
version stays on its own v* release. Not a software release -- marked
pre-release so it never shows as latest.

    [${section}]
    SigLevel = Required
    Server = https://github.com/${REPO}/releases/download/${family}-\$arch" >/dev/null

  # Upload first, then remove what is no longer part of the repository, so the
  # window where the database and its packages disagree never opens.
  gh release upload "$tag" --repo "$REPO" --clobber "$dir"/* >/dev/null
  keep=$(cd "$dir" && ls)
  gh release view "$tag" --repo "$REPO" --json assets -q '.assets[].name' | while read -r have; do
    grep -qxF "$have" <<<"$keep" && continue
    echo "  $tag: removing superseded $have"
    gh release delete-asset "$tag" "$have" --repo "$REPO" --yes
  done
  echo "  published -> $tag ($(cat "$dir/.version" 2>/dev/null || echo '?'))"
done
