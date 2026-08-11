#!/usr/bin/env bash
# Rebuild every published package repository from the release assets.
#
# The packages themselves live only in the per-version GitHub releases. This
# script derives all repository metadata from those releases: running it twice
# produces the same repositories, and running it after a release adds that
# release. Nothing reads gh-pages to decide what exists, which is why packages
# whose release was deleted simply drop out.
#
# Same content, not identical bytes — each Release carries a fresh Date, so a
# re-run still records a commit. Git stores the unchanged package blobs once, so
# a repeat rebuild costs kilobytes; only a real release adds package weight.
#
# Everything is built before anything is published, so a failure while building
# leaves the live repositories untouched. Publishing itself is several separate
# mutations — two uploads per channel, then the gh-pages push — and dying between
# them can still leave one repository ahead of another. Re-running fixes it,
# because the result depends only on the releases.
#
# Produces:
#   release apt-history[-canary]  flat APT repo, full history.
#                                 Filename: ../<tag>/<asset> reaches the sibling
#                                 release; apt follows it, GitHub normalises it.
#   gh-pages apt/                 the current version only, kept at the existing
#                                 URL so configured machines keep working. APT
#                                 resolves Filename against the sources.list root,
#                                 so a Pages-hosted index can only serve packages
#                                 that are themselves on Pages.
#   gh-pages yum/, yum-canary/    repodata only, full history. RPM-MD takes an
#                                 absolute per-package xml:base, so the index
#                                 stays put and the rpms come from the releases.
#
# Requires: gh (authenticated), curl, python3, dpkg-scanpackages, createrepo_c,
# realpath, and gpg with the signing key imported and GPG_KEY_ID set.
set -euo pipefail

SCRIPTS="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="${GITHUB_REPOSITORY:-stuckj/mkvdup}"
# RELEASE_BASE overrides where release assets are read from and pointed at, so
# the publish path can be exercised against a local server. Same purpose as
# PAGES_REMOTE below.
BASE="${RELEASE_BASE:-https://github.com/${REPO}/releases/download}"
# Exported so repo-index.sh honours an override rather than re-deriving a default.
export PAGES_URL="${PAGES_URL:-https://stuckj.github.io/mkvdup}"
KEY="${GPG_KEY_ID:?GPG_KEY_ID must be set}"
export GH_TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
# Publishing requires DRY_RUN=0 explicitly. Defaulting the other way would mean
# a maintainer inspecting the script locally — or typing the obvious DRY_RUN=true
# — rewrites the live repositories.
DRY_RUN="${DRY_RUN:-1}"
[ "$DRY_RUN" = 0 ] || DRY_RUN=1
# Paths on gh-pages this script replaces. Everything else on the branch is left
# alone, so benchmarks, coverage, and anything added later survive. The canary
# paths are only listed when the canary channel was actually rebuilt — removing
# them while producing no replacement would delete the live canary repositories.
owned_paths() {
  printf '%s\n' apt/dists/stable apt/pool/main apt/gpg-key.asc yum index.html gpg-key.asc
  if [ "${DO_CANARY:-0}" = 1 ]; then
    printf '%s\n' apt/dists/canary apt/pool/canary yum-canary
  fi
}

say() { printf '\n== %s\n' "$1"; }
die() { echo "FATAL: $*" >&2; exit 1; }
# The signing key is passphrase-protected, so every gpg call feeds it on fd 0.
gpg_do() { printf '%s' "${GPG_PASSPHRASE:-}" | gpg --batch --yes --armor --passphrase-fd 0 \
                                                   --pinentry-mode loopback "$@"; }
detach() { gpg_do --detach-sign -o "$1" "$2"; }
sign() {  # sign <detached-out> <clear-out> <file>
  gpg_do --detach-sign -o "$1" "$3"
  gpg_do --clearsign -o "$2" "$3"
}

# The work tree is wiped, and later steps cd into it and then refer back to it,
# so resolve it to an absolute path and refuse anything that is not clearly a
# scratch directory.
[ $# -ge 1 ] || die "usage: $(basename "$0") <work-directory>   (a scratch dir outside any checkout)"
# The wipe below is destructive and runs before the dry-run gate, so validate the
# fully resolved path, not the argument: `rebuild-package-repos.sh docs` from a
# checkout resolves to a real directory of tracked files.
#
# realpath -m resolves "." and ".." without requiring the path to exist. A
# dirname/basename split does not, so "$HOME/." slipped past the guards below;
# and under uutils coreutils, which Ubuntu 25.10 ships by default, basename of
# "$HOME/./" is "claude", which would have retargeted the wipe at a directory the
# operator never named. Nothing is created before the guards run.
WORK="$(realpath -m -- "$1")" || die "cannot resolve '$1'"
[ -d "$(dirname "$WORK")" ] || die "cannot resolve '$1' — its parent directory does not exist"
MARKER=.repobuild-marker
case "$WORK" in
  / | "${HOME:-/nonexistent}") die "refusing to use '$WORK' as a work directory" ;;
esac
# Ask about the parent, which is guaranteed to exist — $WORK may not yet, and
# `git -C` on a missing directory just fails, which would skip the check.
if git -C "$(dirname "$WORK")" rev-parse --show-toplevel >/dev/null 2>&1; then
  die "refusing to use '$WORK': it is inside a git working tree"
fi
if [ -n "$(ls -A "$WORK" 2>/dev/null)" ] && [ ! -e "$WORK/$MARKER" ]; then
  die "refusing to wipe non-empty '$WORK': this script did not create it"
fi
mkdir -p "$WORK"
rm -rf "${WORK:?}"/* "${WORK:?}"/.[!.]* 2>/dev/null || true
touch "$WORK/$MARKER"
mkdir -p "$WORK"/{pkgs,apt/stable,apt/canary,pages}
cd "$WORK"

say "enumerate release assets"
# Only v* tags hold packages; apt-history* are this script's own output. Drafts
# are excluded: the API returns them to a token with push access, but their
# download URLs 404 for everyone else, so indexing one publishes a broken entry.
enumerate() {
  gh api "repos/$REPO/releases" --paginate \
    -q '.[] | select(.draft == false) | select(.tag_name | startswith("v"))
        | .tag_name as $t | .assets[] | select(.state == "uploaded")
        | "\($t)/\(.name)\t\(.size)\t\(.browser_download_url)"' \
    | awk -F'\t' '$1 ~ /\.(deb|rpm)$/' | sort -u > assetsizes.txt
  cut -f1 assetsizes.txt > assetmap.txt
}
enumerate
# The releases API is eventually consistent, and this job runs moments after the
# release was created. Wait for the release to be listed *completely*: assets are
# uploaded one at a time, so the last one is the likeliest to lag a replica, and
# a listing missing just that one would sail past a "is the tag there?" check and
# then fail the completeness check further down — aborting a release whose tag,
# GitHub release, Homebrew formula and Nix bump have already shipped.
tag_complete() {  # tag_complete <tag>
  local pat
  for pat in '_amd64\.deb$' '_arm64\.deb$' '\.x86_64\.rpm$' '\.aarch64\.rpm$'; do
    grep -q "^${1}/.*${pat}" assetmap.txt || return 1
  done
  return 0
}
if [ -n "${EXPECT_TAG:-}" ]; then
  for attempt in 1 2 3 4 5; do
    tag_complete "$EXPECT_TAG" && break
    echo "  ${EXPECT_TAG} not fully listed yet (attempt $attempt) — re-reading in ${ENUM_RETRY_DELAY:-10}s"
    sleep "${ENUM_RETRY_DELAY:-10}"
    enumerate
  done
fi
[ -s assetmap.txt ] || die "no package assets found — refusing to publish an empty repository"
# One asset name must map to one release. The APT index resolves a name to the
# first matching tag and the YUM rewrite to the last, so a duplicate across two
# tags could point a package at a release holding different bytes — a permanent
# hash mismatch for every client.
dupes=$(cut -d/ -f2- assetmap.txt | sort | uniq -d)
[ -z "$dupes" ] || die "asset name(s) present under more than one release: $(echo "$dupes" | tr '\n' ' ')"
echo "  $(wc -l < assetmap.txt) package assets across $(cut -d/ -f1 assetmap.txt | sort -u | wc -l) releases"

say "download packages"
# Fetched over the asset download URL rather than `gh release download`, which
# spends one REST request per asset. At 540 assets that is most of the 1000/hour
# GITHUB_TOKEN budget shared with the rest of the release, so two runs in an hour
# — a release plus a manual rebuild, or two canaries — would start failing.
# No `|| true`: a silently dropped release would be published as an index that no
# longer offers that version.
while IFS=$'\t' read -r path want url; do
  out="pkgs/$path"
  mkdir -p "$(dirname "$out")"
  # Bounded: curl's retry does not fire on a transfer that merely stalls, and
  # this job holds the package-repositories concurrency group — a hang would
  # queue every later release behind it until the workflow timeout.
  curl -fsSL --retry 3 --retry-delay 2 \
       --connect-timeout 30 --speed-limit 1024 --speed-time 60 \
       -o "$out" "$url" \
    || die "could not download $path"
done < assetsizes.txt

# Size-check every asset: an interrupted transfer leaves a short file that would
# otherwise be indexed with the hash of the truncated bytes — a permanent
# mismatch for every client.
missing=0; short=0
while IFS=$'\t' read -r path want url; do
  f="pkgs/$path"
  if [ ! -f "$f" ]; then echo "  MISSING $path" >&2; missing=$((missing+1)); continue; fi
  got=$(stat -c%s "$f")
  [ "$got" = "$want" ] || { echo "  SHORT $path: $got != $want" >&2; short=$((short+1)); }
done < assetsizes.txt
[ "$missing" = 0 ] || die "$missing asset(s) failed to download"
[ "$short" = 0 ] || die "$short asset(s) truncated"
echo "  $(wc -l < assetmap.txt) files, all sizes match the release metadata"

# Hardlink rather than copy: this is ~1 GB of packages and both trees live in
# the same work directory.
say "stage by channel"
mkdir -p stage/{deb-stable,deb-canary,rpm-stable,rpm-canary}
stage_one() { ln -f "$1" "$2" 2>/dev/null || cp -f "$1" "$2"; }
while IFS=/ read -r tag name; do
  src="pkgs/$tag/$name"
  [ -f "$src" ] || die "staging lost $tag/$name"
  # Match the package name, not the substring "canary": a version string that
  # merely contains it (1.9.2-canary1, which the release workflow classifies as
  # stable) would otherwise file stable packages into the canary channel and
  # publish the release to the wrong repository without failing.
  case "$name" in
    mkvdup-canary_*.deb) stage_one "$src" "stage/deb-canary/$name" ;;
    mkvdup_*.deb)        stage_one "$src" "stage/deb-stable/$name" ;;
    mkvdup-canary-*.rpm) stage_one "$src" "stage/rpm-canary/$name" ;;
    mkvdup-*.rpm)        stage_one "$src" "stage/rpm-stable/$name" ;;
    *)                   die "unclassified asset $name" ;;
  esac
done < assetmap.txt
for d in stage/*; do echo "  $(basename "$d"): $(find "$d" -type f | wc -l)"; done

newest() {  # newest <stagedir> <prefix> -> highest Debian version, or non-zero
  local best="" v f
  for f in "$1"/"$2"_*.deb; do
    [ -f "$f" ] || return 1
    # set -e does not apply inside a command substitution, so check explicitly.
    v=$(dpkg-deb -f "$f" Version 2>/dev/null) || return 1
    [ -n "$v" ] || return 1
    if [ -z "$best" ] || dpkg --compare-versions "$v" gt "$best"; then best="$v"; fi
  done
  [ -n "$best" ] || return 1
  printf '%s' "$best"
}

# Whether a channel can be published, decided before anything is built. Every
# check is explicit rather than relying on errexit, because this is called from a
# condition — where bash disables errexit for the whole function body.
channel_ready() {  # channel_ready <debdir> <rpmdir> <prefix>
  local v vd a
  # Say which condition failed: this gates the whole run, and "incomplete" alone
  # leaves an operator unable to tell a transient API race from a real gap.
  [ "$(find "$1" -name '*.deb' | wc -l)" -gt 0 ] || { echo "  no .deb packages in $1" >&2; return 1; }
  [ "$(find "$2" -name '*.rpm' | wc -l)" -gt 0 ] || { echo "  no .rpm packages in $2" >&2; return 1; }
  v=$(newest "$1" "$3") || { echo "  cannot determine the newest version in $1" >&2; return 1; }
  vd="${v//\~/.}"
  # Both architectures must exist for the version about to be advertised,
  # otherwise the missing one gets a signed but empty index.
  for a in amd64 arm64; do
    [ -f "$1/${3}_${vd}_${a}.deb" ] || [ -f "$1/${3}_${v}_${a}.deb" ] || {
      echo "  newest version $v has no $a package in $1" >&2; return 1; }
  done
  # Same for the rpms. Checking only that the directory is non-empty would let a
  # half-uploaded pair through: the yum repo would then offer the new version to
  # one architecture and the previous one to the other, with the run green.
  for a in x86_64 aarch64; do
    compgen -G "$2/${3}-${vd}-*.${a}.rpm" >/dev/null \
      || compgen -G "$2/${3}-${v}-*.${a}.rpm" >/dev/null || {
      echo "  newest version $v has no $a rpm in $2" >&2; return 1; }
  done
  return 0
}

channel_ready stage/deb-stable stage/rpm-stable mkvdup \
  || die "the stable channel is incomplete — refusing to rebuild"
# Canary is optional. Retiring it, or one malformed canary release, must not stop
# a stable release from reaching users, so the channel is checked up front and
# skipped as a whole rather than failing the run part-way through.
DO_CANARY=1
if ! channel_ready stage/deb-canary stage/rpm-canary mkvdup-canary; then
  # Skipping a channel leaves its published index exactly as it is, which is the
  # right answer when the index still describes the packages on the releases. It
  # is the wrong answer when a run has just rewritten those packages' bytes: the
  # index would keep advertising pre-signing checksums and dnf would refuse
  # every one. resign-rpms.yml sets this so that case fails instead.
  if [ "${REQUIRE_ALL_CHANNELS:-0}" = 1 ]; then
    die "the canary channel is incomplete, and this rebuild must not leave any
       channel's published index behind — its checksums would no longer match
       the packages on the releases"
  fi
  DO_CANARY=0
  msg="Canary channel incomplete — its repositories were left exactly as they are, so canary
users stay on the last good version until a complete canary release is published."
  echo "::warning::${msg}"
  # A warning annotation on a green run is easy to miss, and the channel stays
  # frozen until someone notices.
  [ -n "${GITHUB_STEP_SUMMARY:-}" ] && printf '### ⚠️ Canary skipped\n\n%s\n' "$msg" >> "$GITHUB_STEP_SUMMARY"
fi

say "build flat APT history repos (metadata only)"
build_apt_history() {  # build_apt_history <channel> <stagedir> <label>
  # Separate `local` statements: bash expands every argument of a single `local`
  # before performing any of its assignments, so `out="apt/$ch"` alongside
  # `ch="$1"` would read the caller's ch, not this one's.
  local ch="$1" dir="$2" label="$3"
  local out="apt/$ch"
  local line n tag
  ( cd "$dir" && dpkg-scanpackages -m . 2>/dev/null ) | sed 's|^Filename: \./|Filename: |' > "$out/.raw"
  : > "$out/Packages"
  while IFS= read -r line; do
    if [[ $line == Filename:\ * ]]; then
      n="${line#Filename: }"
      # `|| true`: without it pipefail aborts the whole script on grep's no-match
      # exit, so the diagnostic below would never be reached.
      tag=$(grep -m1 -F "/$n" "$WORK/assetmap.txt" | cut -d/ -f1 || true)
      [ -n "$tag" ] || die "no release holds $n"
      printf 'Filename: ../%s/%s\n' "$tag" "$n" >> "$out/Packages"
    else
      printf '%s\n' "$line" >> "$out/Packages"
    fi
  done < "$out/.raw"
  rm -f "$out/.raw"
  # dpkg-scanpackages emits stanzas in readdir order, which is not stable across
  # runs even when the staged set is identical. Sorting them makes the index
  # byte-reproducible, so an unchanged package set produces an unchanged asset
  # instead of a fresh blob on every rebuild.
  python3 - "$out/Packages" <<'PY'
import sys
path = sys.argv[1]
stanzas = [s.strip("\n") for s in open(path).read().split("\n\n") if s.strip()]
stanzas.sort()
with open(path, "w") as fh:
    for s in stanzas:
        fh.write(s + "\n\n")
PY
  # dpkg-scanpackages exits 0 on an empty directory, so check the result.
  local entries
  entries=$(grep -c '^Package:' "$out/Packages" || true)
  [ "$entries" -gt 0 ] || die "$ch index is empty — refusing to publish it"
  ( cd "$out"
    gzip -nkf Packages   # -n: no mtime, so identical input gives identical bytes
    { echo "Origin: $label"; echo "Label: $label"; echo "Codename: ./"
      echo "Architectures: amd64 arm64"; echo "Description: $label (full version history)"
      echo "Date: $(date -Ru)"; echo "SHA256:"
      for f in Packages Packages.gz; do
        echo " $(sha256sum "$f" | awk '{print $1}') $(stat -c%s "$f") $f"
      done
    } > Release
    sign Release.gpg InRelease Release )
  echo "  $ch: $entries entries, index $(stat -c%s "$out/Packages.gz")B"
}
# Deleting a release legitimately drops its packages from the index, but a
# partial read of the releases API looks identical and would quietly republish
# without versions that still exist — including re-pinning the Pages suite to an
# older "current" release. Every other suspicious input here fails loudly; this
# is the one that would not.
#
# A whole release vanishing takes its rpms with its debs, so the APT count
# covers that case. Individual rpm assets disappearing while their debs remain
# would not move it at all, which is why check_no_shrink_yum exists as well —
# resign-release-rpms.sh replaces rpms one at a time and can leave exactly that
# asymmetry behind if it fails partway.
check_no_shrink() {  # check_no_shrink <channel> <release-tag>
  local ch="$1" tag="$2" prev=0 now http
  now=$(grep -c '^Package:' "apt/$ch/Packages")
  # Read the status, not curl's exit code: `curl -f` returns 22 for every HTTP
  # error alike, so keying on it would treat a 503 as "nothing published yet"
  # and disable this guard during exactly the GitHub incident that also makes
  # the releases listing come back partial.
  http=$(curl -sSL --retry 3 --retry-delay 2 --connect-timeout 30 --max-time 120 \
              -o prev-Packages -w '%{http_code}' "$BASE/$tag/Packages" 2>/dev/null || true)
  [ -n "$http" ] || http=000
  case "$http" in
    200) prev=$(grep -c '^Package:' prev-Packages || true) ;;
    404) prev=0 ;;   # nothing published yet, so nothing to shrink
    *)   die "cannot read the published $ch index to compare against (HTTP $http)" ;;
  esac
  rm -f prev-Packages
  if [ "$prev" -gt "$now" ] && [ "${ALLOW_SHRINK:-0}" != 1 ]; then
    die "the $ch index would shrink from $prev to $now entries. If a release was
       deliberately deleted, re-run with ALLOW_SHRINK=1; otherwise the releases
       API returned an incomplete view and re-running should fix it."
  fi
  echo "  $ch: $now entries (currently published: $prev)"
}

build_apt_history stable stage/deb-stable mkvdup
# The YUM equivalent, counting packages rather than Packages stanzas. The
# published primary.xml is the comparison point because it is what clients
# actually resolve; a count taken from the staged rpms alone could only ever
# agree with itself.
check_no_shrink_yum() {  # check_no_shrink_yum <pages-subdir> <stage-dir>
  local out="$1" dir="$2" now prev=0 http href declared
  now=$(find "$dir" -name '*.rpm' | wc -l)
  http=$(curl -sSL --retry 3 --retry-delay 2 --connect-timeout 30 --max-time 120 \
              -o prev-repomd.xml -w '%{http_code}' \
              "$PAGES_URL/$out/repodata/repomd.xml" 2>/dev/null || true)
  [ -n "$http" ] || http=000
  case "$http" in
    200)
      # || true: a no-match makes the pipeline fail under errexit, which would
      # abort here with no output instead of reaching the die below.
      href=$(grep -oE 'repodata/[a-f0-9]+-primary\.xml\.gz' prev-repomd.xml | head -1 || true)
      [ -n "$href" ] || die "the published $out repomd.xml names no primary.xml"
      curl -fsSL --retry 3 --retry-delay 2 --connect-timeout 30 --max-time 300 \
           -o prev-primary.gz "$PAGES_URL/$out/$href" \
        || die "cannot fetch the published $out primary.xml to compare against"
      # Decompress to a file and check the status. Piping into grep hides it:
      # a truncated 200 makes gzip fail inside a process substitution, grep
      # prints 0, and the comparison below would silently be skipped on exactly
      # the sort of incident this guard exists for.
      gzip -dc prev-primary.gz > prev-primary.xml \
        || die "the published $out primary.xml did not decompress — refusing to
       compare against a partial fetch"
      # Counted from the same hrefs the comparison uses, and cross-checked
      # against the packages="N" the index declares. Counting with a separate
      # pattern let a published index whose markup differed report prev=0,
      # which skips the comparison and looks exactly like "nothing published".
      # Basenames: the published index carries bare names under xml:base, but
      # an older layout prefixed them with a directory, and comparing those
      # against find's %f would call every package missing.
      grep -oE 'href="[^"]+\.rpm"' prev-primary.xml \
        | sed 's/^href="//; s/"$//; s|.*/||' | LC_ALL=C sort -u \
        > "$out-prev-names.txt" || true
      prev=$(wc -l < "$out-prev-names.txt")
      declared=$(grep -oE 'packages="[0-9]+"' prev-primary.xml | head -1 \
                 | grep -oE '[0-9]+' || echo "")
      if [ -n "$declared" ] && [ "$declared" != "$prev" ]; then
        die "the published $out index declares $declared packages but $prev
       locations could be read from it — refusing to compare against a listing
       this script cannot parse"
      fi ;;
    404) prev=0 ;;   # nothing published yet, so nothing to shrink
    *)   die "cannot read the published $out repodata to compare against (HTTP $http)" ;;
  esac
  # Compare the sets, not just the totals: a rebuild that adds as many packages
  # as an earlier failure dropped nets out, and a count would call that
  # unchanged. Comparing names also lets the failure say which package went.
  : > "$out-gone.txt"
  if [ "$prev" -gt 0 ]; then
    # LC_ALL=C throughout: en_US collation ignores the '-' and '.' in package
    # names, so sort's order and comm's byte comparison disagree and comm warns
    # "not in sorted order" and can miss entries.
    find "$dir" -name '*.rpm' -printf '%f\n' | LC_ALL=C sort -u > "$out-now-names.txt"
    LC_ALL=C comm -23 "$out-prev-names.txt" "$out-now-names.txt" > "$out-gone.txt"
  fi
  rm -f prev-repomd.xml prev-primary.gz prev-primary.xml
  if [ -s "$out-gone.txt" ] && [ "${ALLOW_SHRINK:-0}" != 1 ]; then
    sed 's/^/    /' "$out-gone.txt" >&2
    die "$(wc -l < "$out-gone.txt") package(s) listed above are published in $out
       but are no longer among the release assets. If a release was deliberately
       deleted, re-run with ALLOW_SHRINK=1; otherwise assets are missing — check
       whether a re-signing run failed partway."
  fi
  echo "  $out: $now packages (currently published: $prev)"
}

check_no_shrink stable apt-history
if [ "$DO_CANARY" = 1 ]; then
  build_apt_history canary stage/deb-canary mkvdup-canary
  check_no_shrink canary apt-history-canary
fi
gpg --armor --export "$KEY" > apt/gpg-key.asc
# gpg exits 0 and writes nothing if the key id does not resolve, which would
# replace the key every documented install curls with an empty file.
[ -s apt/gpg-key.asc ] || die "exported public key is empty — is GPG_KEY_ID ($KEY) right?"

say "build gh-pages APT (current version only)"
mkdir -p pages/apt/pool/{main,canary}
# label is per channel: the replaced job set Origin/Label to mkvdup-canary for
# the canary suite, and an apt pin on o=/l= would break if both said mkvdup.
for spec in "stable:deb-stable:mkvdup:main:mkvdup" "canary:deb-canary:mkvdup-canary:canary:mkvdup-canary"; do
  IFS=: read -r ch dir prefix pool label <<<"$spec"
  [ "$ch" = canary ] && [ "$DO_CANARY" = 0 ] && continue
  v=$(newest "stage/$dir" "$prefix") || die "cannot determine the newest $ch version"
  cp "stage/$dir/${prefix}_${v//\~/.}"_*.deb "pages/apt/pool/$pool/" 2>/dev/null \
    || cp "stage/$dir/${prefix}_${v}"_*.deb "pages/apt/pool/$pool/"
  mkdir -p "pages/apt/dists/$ch/main"/binary-{amd64,arm64}
  ( cd pages/apt
    for a in amd64 arm64; do
      dpkg-scanpackages --arch "$a" "pool/$pool/" 2>/dev/null > "dists/$ch/main/binary-$a/Packages"
      # dpkg-scanpackages exits 0 and writes nothing when no package matches the
      # architecture. Signing that would advertise an empty index, and the arch
      # would report "Unable to locate package" with the run still green.
      grep -q '^Package:' "dists/$ch/main/binary-$a/Packages" \
        || die "$ch/$a index is empty — no $a package for the current version"
      gzip -nkf "dists/$ch/main/binary-$a/Packages"
    done
    cd "dists/$ch"
    { echo "Origin: $label"; echo "Label: $label"; echo "Suite: $ch"; echo "Codename: $ch"
      echo "Architectures: amd64 arm64"; echo "Components: main"
      echo "Description: $label (current version; see the apt-history release for older versions)"
      echo "Date: $(date -Ru)"; echo "SHA256:"
      for f in main/binary-amd64/Packages main/binary-amd64/Packages.gz \
               main/binary-arm64/Packages main/binary-arm64/Packages.gz; do
        echo " $(sha256sum "$f" | awk '{print $1}') $(stat -c%s "$f") $f"
      done
    } > Release
    sign Release.gpg InRelease Release )
  echo "  $ch: pinned at $v"
done

say "build gh-pages YUM repodata (full history, packages via xml:base)"
for spec in "yum:rpm-stable" "yum-canary:rpm-canary"; do
  IFS=: read -r out dir <<<"$spec"
  [ "$out" = yum-canary ] && [ "$DO_CANARY" = 0 ] && continue
  # --no-database: only primary.xml carries package locations, so the sqlite
  # copies cannot be redirected to the releases. Shipping them would advertise
  # packages/ paths that no longer exist to any client that prefers sqlite.
  createrepo_c --quiet --no-database "stage/$dir"
  mkdir -p "pages/$out"
  python3 "$SCRIPTS/yum_xmlbase.py" "stage/$dir/repodata" "pages/$out/repodata" assetmap.txt "$BASE"
  detach "pages/$out/repodata/repomd.xml.asc" "pages/$out/repodata/repomd.xml"
  check_no_shrink_yum "$out" "stage/$dir"
  echo "  $out: $(find "stage/$dir" -name '*.rpm' | wc -l) packages indexed, 0 hosted"
done

cp apt/gpg-key.asc pages/gpg-key.asc
# yum-canary is absent when the canary channel was skipped.
for d in apt yum yum-canary; do
  [ -d "pages/$d" ] && cp pages/gpg-key.asc "pages/$d/"
done

# The release being published must actually appear in what was built. The
# replaced job fed the freshly built packages straight in, so this was
# structurally guaranteed; now the index comes from the releases API, and a view
# that does not yet list the new release would publish silently without it.
# Confirm the release being published actually reached both indexes. Match on the
# release tag, not the version: nfpm normalises the version it is handed (0.8
# becomes 0.8.0, and any semver prerelease takes a tilde, so 1.9.2-rc1 becomes
# 1.9.2~rc1), and re-deriving those rules here would reject valid releases. The
# tag is what names the directory the packages are fetched from, so it is both
# stable and the thing that has to be right.
if [ -n "${EXPECT_TAG:-}" ]; then
  grep -qF "${EXPECT_TAG}/" assetmap.txt \
    || die "no assets for ${EXPECT_TAG} — the releases API may not list the new release yet"
  # Decide the channel from the packages the release actually produced, not from
  # the tag text. Staging classifies by package name and the release workflow by
  # `*-canary.*`; a looser tag match here would disagree with both — a version
  # like 1.9.2-canary1 builds stable packages under a tag containing "canary",
  # and looking for them in the canary index kills the run before anything at all
  # is published, including the correctly built stable index.
  if grep -qF "${EXPECT_TAG}/mkvdup-canary" assetmap.txt; then
    aptidx=apt/canary/Packages;  yumdir=pages/yum-canary
  else
    aptidx=apt/stable/Packages;  yumdir=pages/yum
  fi
  [ -f "$aptidx" ] || die "expected ${EXPECT_TAG} in ${aptidx}, but that channel was not rebuilt"
  grep -qF "Filename: ../${EXPECT_TAG}/" "$aptidx" \
    || die "${EXPECT_TAG} is not in the APT index ${aptidx}"
  # The rpm half is checked too: deb and rpm assets can finish uploading at
  # different times, and indexing one without the other is silent.
  # Redirect rather than pipe: `grep -q` exits on the first match, which SIGPIPEs
  # gzip, and pipefail then reports the whole pipeline as failed — a false
  # negative whenever the match is not near the end of the stream.
  grep -qF "/${EXPECT_TAG}/" < <(gzip -dc "$yumdir"/repodata/*primary.xml.gz) \
    || die "${EXPECT_TAG} is not in the YUM repodata under ${yumdir}"
  echo "  confirmed ${EXPECT_TAG} is in both the APT and YUM indexes"
fi
bash "$SCRIPTS/repo-index.sh" > pages/index.html
echo "  generated tree: $(du -sh pages | cut -f1), $(find pages -type f | wc -l) files"

if [ "$DRY_RUN" = 1 ]; then
  say "dry run — publishing nothing"
  echo "  APT history and gh-pages tree left in $WORK"
  exit 0
fi

# ---- everything below publishes; nothing above has touched the live repos ----

say "publish APT history releases"
for ch in stable canary; do
  tag=apt-history; [ "$ch" = canary ] && tag=apt-history-canary
  [ "$ch" = canary ] && [ "$DO_CANARY" = 0 ] && continue
  gh release view "$tag" --repo "$REPO" >/dev/null 2>&1 || \
    gh release create "$tag" --repo "$REPO" --prerelease --title "APT repository ($ch, full history)" \
      --notes "Flat APT repository metadata for the ${ch} channel. Packages resolve into the
per-version releases; this tag holds only the index. Not a software release —
marked pre-release so it never shows as latest.

    deb [signed-by=/usr/share/keyrings/mkvdup.gpg] ${BASE}/${tag}/ ./" >/dev/null
  # --clobber deletes each asset before re-uploading, so the set cannot be
  # swapped atomically: during this window a client can see a 404 or an
  # index that disagrees with the signature, and has to re-run apt update.
  # Either order leaves such a window; indexes first only makes it shorter,
  # because the signature files are small and land quickly after them.
  gh release upload "$tag" --repo "$REPO" --clobber \
    "apt/$ch"/{Packages,Packages.gz} apt/gpg-key.asc >/dev/null \
    || die "could not upload indexes to $tag"
  gh release upload "$tag" --repo "$REPO" --clobber \
    "apt/$ch"/{Release,Release.gpg,InRelease} >/dev/null \
    || die "could not upload signatures to $tag"
  echo "  published -> $tag"
done

say "publish gh-pages"
# Commit on top of the branch rather than force-pushing an orphan: benchmark and
# coverage runs push here too, and replacing history would silently revert a
# commit that landed since the clone. Only the paths this script owns are
# replaced.
URL="${PAGES_REMOTE:-https://x-access-token:${GH_TOKEN}@github.com/${REPO}.git}"

# Deliberately not a function called from `if`: bash disables errexit for the
# whole body of a function invoked in a condition, so a failed cp here would
# carry on and push a tree missing whatever had not been copied yet. Only the
# push itself — the one failure that is expected and retryable — is tested.
stage_ghp() {
  rm -rf ghp
  local rc=0
  git ls-remote --exit-code --heads "$URL" gh-pages >/dev/null || rc=$?
  # --exit-code reports 2 for "no such ref"; anything else is a real failure
  # (network, auth, rate limit) and must not be reported as a missing branch.
  case "$rc" in
    0) git clone --depth 1 --branch gh-pages --quiet "$URL" ghp ;;
    2) echo "  gh-pages does not exist yet — creating it"
       mkdir -p ghp
       git -C ghp init -q -b gh-pages ;;
    *) die "cannot reach the repository to check for gh-pages (git ls-remote exit $rc)" ;;
  esac
  local p
  while read -r p; do rm -rf "ghp/${p:?}"; done < <(owned_paths)
  cp -r pages/. ghp/
  git -C ghp config user.name "github-actions[bot]"
  git -C ghp config user.email "41898282+github-actions[bot]@users.noreply.github.com"
  git -C ghp add -A
}

pushed=0
for attempt in 1 2 3; do
  stage_ghp
  if git -C ghp diff --staged --quiet; then
    echo "  no change to gh-pages"
    pushed=1; break
  fi
  # Name the release when there is one, so the branch history still records
  # which release produced a given state of the site.
  git -C ghp commit -qm "Rebuild package repositories${EXPECT_TAG:+ for ${EXPECT_TAG}}"
  if git -C ghp push -q "$URL" gh-pages; then
    echo "  gh-pages updated (attempt $attempt)"
    pushed=1; break
  fi
  echo "  push rejected — another writer landed first, retrying from a fresh clone"
done
[ "$pushed" = 1 ] || die "could not push gh-pages after 3 attempts"
