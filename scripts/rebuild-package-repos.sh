#!/usr/bin/env bash
# Rebuild every published package repository from the release assets.
#
# The packages themselves live only in the per-version GitHub releases. This
# script derives all repository metadata from those releases, so it is
# idempotent: running it twice produces the same repositories, and running it
# after a release adds that release. Nothing reads gh-pages to decide what
# exists, which is why packages whose release was deleted simply drop out.
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
# Requires: gh (authenticated), dpkg-scanpackages, createrepo_c, gpg with the
# signing key imported, and GPG_KEY_ID set.
set -euo pipefail

SCRIPTS="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="${GITHUB_REPOSITORY:-stuckj/mkvdup}"
BASE="https://github.com/${REPO}/releases/download"
PAGES_URL="${PAGES_URL:-https://stuckj.github.io/mkvdup}"
WORK="${1:-$PWD/.repobuild}"
KEY="${GPG_KEY_ID:?GPG_KEY_ID must be set}"
export GH_TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
# DRY_RUN=1 builds everything and publishes nothing, so a run can be inspected
# before it replaces the live repositories.
DRY_RUN="${DRY_RUN:-0}"

say() { printf '\n== %s\n' "$1"; }
# The signing key is passphrase-protected, so every gpg call feeds it on fd 0.
gpg_do() { printf '%s' "${GPG_PASSPHRASE:-}" | gpg --batch --yes --armor --passphrase-fd 0 \
                                                   --pinentry-mode loopback "$@"; }
detach() { gpg_do --detach-sign -o "$1" "$2"; }
sign() {  # sign <detached-out> <clear-out> <file>
  gpg_do --detach-sign -o "$1" "$3"
  gpg_do --clearsign -o "$2" "$3"
}

rm -rf "$WORK"; mkdir -p "$WORK"/{pkgs,apt/stable,apt/canary,pages}
cd "$WORK"

say "enumerate release assets"
# Only v* tags hold packages; apt-history* are this script's own output.
gh api "repos/$REPO/releases" --paginate \
  -q '.[] | select(.tag_name | startswith("v")) | .tag_name as $t | .assets[] | "\($t)/\(.name)"' \
  | grep -E '\.(deb|rpm)$' | sort -u > assetmap.txt
echo "  $(wc -l < assetmap.txt) package assets across $(cut -d/ -f1 assetmap.txt | sort -u | wc -l) releases"

say "download packages"
cut -d/ -f1 assetmap.txt | sort -u | while read -r tag; do
  mkdir -p "pkgs/$tag"
  gh release download "$tag" --repo "$REPO" --dir "pkgs/$tag" \
     --pattern '*.deb' --pattern '*.rpm' --clobber >/dev/null 2>&1 || true
done
echo "  $(find pkgs -type f \( -name '*.deb' -o -name '*.rpm' \) | wc -l) files"

# Stage each package under the name GitHub actually stored, so metadata hrefs
# match the asset URL. GitHub rewrites '~' to '.', which Debian versioning needs
# for canary ordering; the version inside the package is untouched.
say "stage under stored names"
mkdir -p stage/{deb-stable,deb-canary,rpm-stable,rpm-canary}
while IFS=/ read -r tag name; do
  src="pkgs/$tag/$name"
  [ -f "$src" ] || src="pkgs/$tag/$(printf '%s' "$name" | sed 's/\([0-9]\)\.canary/\1~canary/')"
  [ -f "$src" ] || { echo "  MISSING $tag/$name" >&2; continue; }
  case "$name" in
    *canary*.deb) cp -f "$src" "stage/deb-canary/$name" ;;
    *.deb)        cp -f "$src" "stage/deb-stable/$name" ;;
    *canary*.rpm) cp -f "$src" "stage/rpm-canary/$name" ;;
    *.rpm)        cp -f "$src" "stage/rpm-stable/$name" ;;
  esac
done < assetmap.txt
for d in stage/*; do echo "  $(basename "$d"): $(find "$d" -type f | wc -l)"; done

say "flat APT history repos (metadata only)"
build_apt_history() {  # build_apt_history <channel> <stagedir> <label>
  local ch="$1" dir="$2" label="$3" line n tag out="apt/$ch"
  ( cd "$dir" && dpkg-scanpackages -m . 2>/dev/null ) | sed 's|^Filename: \./|Filename: |' > "$out/.raw"
  : > "$out/Packages"
  while IFS= read -r line; do
    if [[ $line == Filename:\ * ]]; then
      n="${line#Filename: }"
      tag=$(grep -m1 -F "/$n" "$WORK/assetmap.txt" | cut -d/ -f1)
      [ -n "$tag" ] || { echo "FATAL: no release holds $n" >&2; exit 1; }
      printf 'Filename: ../%s/%s\n' "$tag" "$n" >> "$out/Packages"
    else
      printf '%s\n' "$line" >> "$out/Packages"
    fi
  done < "$out/.raw"
  rm -f "$out/.raw"
  ( cd "$out"
    gzip -kf Packages
    { echo "Origin: $label"; echo "Label: $label"; echo "Codename: ./"
      echo "Architectures: amd64 arm64"; echo "Description: $label (full version history)"
      echo "Date: $(date -Ru)"; echo "SHA256:"
      for f in Packages Packages.gz; do
        echo " $(sha256sum "$f" | awk '{print $1}') $(stat -c%s "$f") $f"
      done
    } > Release
    sign Release.gpg InRelease Release )
  echo "  $ch: $(grep -c '^Package:' "$out/Packages") entries, index $(stat -c%s "$out/Packages.gz")B"
}
build_apt_history stable stage/deb-stable mkvdup
build_apt_history canary stage/deb-canary mkvdup-canary
gpg --armor --export "$KEY" > apt/gpg-key.asc

for ch in stable canary; do
  tag=apt-history; [ "$ch" = canary ] && tag=apt-history-canary
  if [ "$DRY_RUN" = 1 ]; then echo "  DRY RUN: would publish -> $tag"; continue; fi
  gh release view "$tag" --repo "$REPO" >/dev/null 2>&1 || \
    gh release create "$tag" --repo "$REPO" --prerelease --title "APT repository ($ch, full history)" \
      --notes "Flat APT repository metadata for the ${ch} channel. Packages resolve into the
per-version releases; this tag holds only the index. Not a software release —
marked pre-release so it never shows as latest.

    deb [signed-by=/usr/share/keyrings/mkvdup.gpg] ${BASE}/${tag}/ ./" >/dev/null
  gh release upload "$tag" --repo "$REPO" --clobber \
    "apt/$ch"/{Packages,Packages.gz,Release,Release.gpg,InRelease} apt/gpg-key.asc >/dev/null
  echo "  published -> $tag"
done

say "gh-pages: APT current-version-only"
newest() {  # newest <stagedir> <prefix> -> highest Debian version present
  local best="" v
  for f in "$1"/"$2"_*.deb; do
    v=$(dpkg-deb -f "$f" Version)
    if [ -z "$best" ] || dpkg --compare-versions "$v" gt "$best"; then best="$v"; fi
  done
  printf '%s' "$best"
}
mkdir -p pages/apt/pool/{main,canary}
for spec in "stable:deb-stable:mkvdup:main" "canary:deb-canary:mkvdup-canary:canary"; do
  IFS=: read -r ch dir prefix pool <<<"$spec"
  v=$(newest "stage/$dir" "$prefix")
  cp "stage/$dir/${prefix}_${v//\~/.}"_*.deb "pages/apt/pool/$pool/" 2>/dev/null \
    || cp "stage/$dir/${prefix}_${v}"_*.deb "pages/apt/pool/$pool/"
  mkdir -p "pages/apt/dists/$ch/main"/binary-{amd64,arm64}
  ( cd pages/apt
    for a in amd64 arm64; do
      dpkg-scanpackages --arch "$a" "pool/$pool/" 2>/dev/null > "dists/$ch/main/binary-$a/Packages"
      gzip -kf "dists/$ch/main/binary-$a/Packages"
    done
    cd "dists/$ch"
    { echo "Origin: mkvdup"; echo "Label: mkvdup"; echo "Suite: $ch"; echo "Codename: $ch"
      echo "Architectures: amd64 arm64"; echo "Components: main"
      echo "Description: mkvdup packages (current version; see the apt-history release for older versions)"
      echo "Date: $(date -Ru)"; echo "SHA256:"
      for f in main/binary-amd64/Packages main/binary-amd64/Packages.gz \
               main/binary-arm64/Packages main/binary-arm64/Packages.gz; do
        echo " $(sha256sum "$f" | awk '{print $1}') $(stat -c%s "$f") $f"
      done
    } > Release
    sign Release.gpg InRelease Release )
  echo "  $ch: pinned at $v"
done

say "gh-pages: YUM repodata (full history, packages via xml:base)"
for spec in "yum:rpm-stable" "yum-canary:rpm-canary"; do
  IFS=: read -r out dir <<<"$spec"
  createrepo_c --quiet "stage/$dir"
  mkdir -p "pages/$out"
  python3 "$SCRIPTS/yum_xmlbase.py" "stage/$dir/repodata" "pages/$out/repodata" assetmap.txt "$BASE"
  detach "pages/$out/repodata/repomd.xml.asc" "pages/$out/repodata/repomd.xml"
  echo "  $out: $(find "stage/$dir" -name '*.rpm' | wc -l) packages indexed, 0 hosted"
done

gpg --armor --export "$KEY" > pages/gpg-key.asc
for d in apt yum yum-canary; do cp pages/gpg-key.asc "pages/$d/"; done
bash "$SCRIPTS/repo-index.sh" > pages/index.html

say "publish gh-pages"
# Carry over anything this script does not own (benchmarks, coverage).
git clone --depth 1 --branch gh-pages --quiet "https://x-access-token:${GH_TOKEN}@github.com/${REPO}.git" old
for keep in benchmarks coverage; do
  [ -d "old/$keep" ] && cp -r "old/$keep" pages/
done
rm -rf old
echo "  gh-pages tree: $(du -sh pages | cut -f1), $(find pages -type f | wc -l) files (was ~1022 MiB, 590 files)"
if [ "$DRY_RUN" = 1 ]; then
  echo "  DRY RUN: not pushing. Tree left in $WORK/pages"
  exit 0
fi
( cd pages
  git init -q -b gh-pages
  git config user.name "github-actions[bot]"
  git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
  git add -A
  git commit -qm "Rebuild package repositories from release assets"
  git push -q --force "https://x-access-token:${GH_TOKEN}@github.com/${REPO}.git" gh-pages )
echo "  pushed gh-pages"
