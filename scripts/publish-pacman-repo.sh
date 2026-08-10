#!/usr/bin/env bash
# Rebuild the pacman repositories from the release assets.
#
# A pacman repository has to be one flat directory: libalpm validates %FILENAME%
# and rejects anything containing a '/', so a package cannot be addressed in a
# sibling location the way APT's Filename can. Database and packages therefore
# live together, and a GitHub release is exactly one flat namespace -- so each
# channel gets a release of its own to be that directory:
#
#   pacman-x86_64          Server = .../releases/download/pacman-$arch
#   pacman-canary-x86_64   Server = .../releases/download/pacman-canary-$arch
#
# x86_64 only: Arch is an x86_64 distribution and publishes no aarch64 container
# to build one in. The PKGBUILD still declares aarch64 for Arch Linux ARM users,
# who build it from the AUR themselves.
#
# Only the current version is served. A pacman database holds one entry per
# package name, so an older .pkg.tar.zst in the repository is not installable
# through pacman at all. Every version stays on its own v* release regardless.
#
# This reads the releases rather than any branch, so it is idempotent and it
# repairs drift. It picks the highest version present rather than trusting a
# caller, which is what keeps a re-released older version from regressing a
# channel. Deleting a release therefore falls back to the next version still
# published -- but a channel whose packages are *all* gone keeps serving what it
# last published, because there is nothing left to rebuild from.
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
# Set by the release workflow so this can tell "the channel already has something
# newer" from "the release reached it". A standalone rebuild has no particular
# version to expect and leaves both empty.
RELEASED_VERSION="${RELEASED_VERSION:-}"
RELEASED_CHANNEL="${RELEASED_CHANNEL:-}"
# Same transformation build-arch applies: a pacman pkgver cannot contain a hyphen.
RELEASED_PKGVER="${RELEASED_VERSION//-/_}"

say() { printf '\n== %s\n' "$1"; }

# The one path where a release deliberately does not reach this channel. A plain
# log line on an otherwise green release is invisible -- that is how v1.9.0
# shipped reporting 1.8.2 (#212) -- so say it as a warning and in the summary,
# the way sync-nix does for its own stand-down.
stand_down() {  # stand_down <tag> <message>
  echo "::warning::$2"
  if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
    { echo "### ⚠️ pacman repository not updated ($1)"
      echo
      echo "$2"
      echo
      echo "Every other channel that reported success carries this release."
    } >> "$GITHUB_STEP_SUMMARY"
  fi
}
gpg_sign() {  # detached *binary* signature: repo-add --include-sigs refuses an armoured one
  printf '%s' "${GPG_PASSPHRASE:-}" | gpg --batch --yes --passphrase-fd 0 \
    --pinentry-mode loopback --local-user "$KEY" --detach-sign --no-armor -o "$1.sig" "$1"
}

# $WORK is caller-supplied and about to be deleted, so refuse anything that is
# not a private scratch directory: a stray "." or a repository root would take
# real work with it. Resolved to an absolute path first, because the checks below
# are meaningless against a relative one.
WORK=$(readlink -m -- "$WORK")
case "$WORK" in
  /|"$HOME"|"$PWD") echo "::error::refusing to use $WORK as a work directory"; exit 1 ;;
esac
if [ -e "$WORK" ] && [ ! -e "$WORK/.pacmanbuild-scratch" ]; then
  echo "::error::$WORK already exists and was not created by this script; refusing to delete it"
  exit 1
fi
rm -rf -- "$WORK"; mkdir -p "$WORK"; touch "$WORK/.pacmanbuild-scratch"; cd "$WORK"

say "enumerate Arch packages across the v* releases"
# apt-history*/pacman-* are output of this script and its sibling, never sources.
# shellcheck disable=SC2016  # $t is jq's variable, not the shell's
#
# Fetched on its own line rather than piped straight into grep. grep exits 1 when
# a repository has no Arch packages yet, which is a legitimate state and has to be
# tolerated -- but tolerating it in a pipeline would tolerate a failed gh api call
# with it, and this script's response to an empty list is to publish nothing and
# exit 0. A rate-limited or 5xx enumeration would then leave every channel serving
# its previous version behind a green release.
gh api "repos/$REPO/releases" --paginate \
  -q '.[] | select(.tag_name | startswith("v")) | .tag_name as $t | .assets[] | "\($t)/\(.name)"' \
  > releaseassets.txt
grep -E '\.pkg\.tar\.zst$' releaseassets.txt | sort -u > assetmap.txt || true
echo "  $(wc -l < assetmap.txt) package assets"

build_channel() {  # build_channel <pkgname> <arch> <tag> <dbname>
  local pkgname=$1 arch=$2 tag=$3 db=$4
  local dir="$WORK/$tag" best="" best_line="" ver line name

  # Every asset for this package name and architecture, newest wins.
  #
  # The glob alone is not enough: "mkvdup-*-x86_64.pkg.tar.zst" also matches
  # mkvdup-canary's files, because one package name prefixes the other. What
  # separates them is that the remainder has to be exactly <pkgver>-<pkgrel>, and
  # a pkgver cannot contain a hyphen -- so anything with a second one belongs to a
  # longer package name.
  while IFS= read -r line; do
    name=${line#*/}
    case "$name" in
      "$pkgname"-*"-$arch.pkg.tar.zst") ;;
      *) continue ;;
    esac
    ver=${name#"$pkgname"-}
    ver=${ver%"-$arch.pkg.tar.zst"}
    case "$ver" in
      *-*-*) continue ;;   # two hyphens or more: a longer package name
      *-*)   ;;            # exactly one: <pkgver>-<pkgrel>
      *)     continue ;;   # none: not a package filename at all
    esac
    if [ -z "$best" ] || [ "$(vercmp "$ver" "$best")" -gt 0 ]; then
      best=$ver; best_line=$line
    fi
  done < assetmap.txt

  # Only the channel this release belongs to is expected to move; the other one
  # legitimately stays where it was.
  local channel=stable
  case "$pkgname" in *-canary) channel=canary ;; esac
  local released=0
  [ -n "$RELEASED_PKGVER" ] && [ "$channel" = "$RELEASED_CHANNEL" ] && released=1

  if [ -z "$best" ]; then
    if [ "$released" = 1 ]; then
      stand_down "$tag" "No $pkgname package for $arch is attached to any release, including this one, so the pacman channel keeps whatever it last published."
    else
      echo "  $tag: no $pkgname package on any release yet, skipping"
    fi
    return 0
  fi

  # ${best%-*} drops the pkgrel: best is <pkgver>-<pkgrel>, the released version
  # is just <pkgver>. Which side is higher decides which of two different things
  # happened, so compare rather than merely testing for inequality -- reporting a
  # missing upload as "you released something older" would send the maintainer
  # looking in the wrong place entirely.
  if [ "$released" = 1 ] && [ "${best%-*}" != "$RELEASED_PKGVER" ]; then
    if [ "$(vercmp "$RELEASED_PKGVER" "${best%-*}")" -lt 0 ]; then
      stand_down "$tag" "${RELEASED_PKGVER} is older than the ${best} this channel already serves, so the pacman channel keeps its current version."
    else
      stand_down "$tag" "${RELEASED_PKGVER} is newer than the ${best} this channel serves, but no ${pkgname} asset for it was found on any release -- so this release did not reach the pacman channel."
    fi
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
build_channel mkvdup        x86_64 pacman-x86_64        mkvdup
build_channel mkvdup-canary x86_64 pacman-canary-x86_64 mkvdup-canary

say "publish"
for dir in "$WORK"/pacman-*; do
  [ -d "$dir" ] || continue
  tag=$(basename "$dir")
  if [ "$DRY_RUN" = 1 ]; then
    # Counted with the glob that actually uploads, so this does not include the
    # dotfile the keep-list and the upload both exclude.
    # shellcheck disable=SC2012  # names here are ours, not arbitrary
    echo "  DRY RUN: would publish $(ls "$dir" | wc -l) assets -> $tag"
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
  # database never names a package that has already been deleted. It does not
  # close every window: --clobber replaces an asset by deleting and re-uploading
  # it, so a client syncing during that instant can still see a missing file.
  gh release upload "$tag" --repo "$REPO" --clobber "$dir"/* >/dev/null
  keep=$(cd "$dir" && ls)
  gh release view "$tag" --repo "$REPO" --json assets -q '.assets[].name' | while read -r have; do
    grep -qxF "$have" <<<"$keep" && continue
    echo "  $tag: removing superseded $have"
    gh release delete-asset "$tag" "$have" --repo "$REPO" --yes
  done
  echo "  published -> $tag ($(cat "$dir/.version" 2>/dev/null || echo '?'))"
done
