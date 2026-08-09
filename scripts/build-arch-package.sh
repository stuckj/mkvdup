#!/usr/bin/env bash
# Generate the PKGBUILD for a release and build it.
#
# Produces, in $WORK:
#   PKGBUILD .SRCINFO        what update-aur pushes to the AUR
#   *.pkg.tar.zst            what the release and the pacman repository carry
#
# The package builds from source, like every Go package in Arch's own
# repositories -- a prebuilt one would have to be named mkvdup-bin and could
# never be promoted out of the AUR, since Arch builds what it ships.
#
# That is why this runs after the release exists: source=() points at the tag's
# archive, so the tag has to be there before the checksum can be taken or
# makepkg can fetch it. An Arch failure therefore lands on a published release
# rather than preventing one. Re-running the job is the recovery; nothing else
# has to be undone.
#
# Requires: makepkg and go, i.e. archlinux:base-devel. makepkg refuses to run as
# root, so this creates an unprivileged user when it is running as one.
set -euo pipefail

VERSION="${VERSION:?VERSION must be set}"      # 1.9.2 or 1.9.2-canary.1
TAG="${TAG:?TAG must be set}"                  # v1.9.2
IS_CANARY="${IS_CANARY:-false}"
REPO="${GITHUB_REPOSITORY:-stuckj/mkvdup}"
WORK="${1:-$PWD/.archbuild}"
TEMPLATE="${TEMPLATE:-$PWD/packaging/arch/PKGBUILD.in}"

# A pacman pkgver cannot contain a hyphen -- that is the pkgver/pkgrel separator
# -- so 1.9.2-canary.1 is packaged as 1.9.2_canary.1. Canaries are their own
# package, as they are for deb and rpm, so that version never has to sort against
# a stable one.
PKGVER="${VERSION//-/_}"
if [ "$IS_CANARY" = true ]; then
  PKGNAME=mkvdup-canary
  PKGDESC="Storage deduplication tool for MKV files (canary/pre-release)"
else
  PKGNAME=mkvdup
  PKGDESC="Storage deduplication tool for MKV files and their source media"
fi

rm -rf "$WORK"; mkdir -p "$WORK"

echo "== source archive checksum"
# Taken from the archive GitHub actually serves, because that is the one an AUR
# user's makepkg will verify. Generating our own tarball here and hashing that
# would produce a checksum nobody else can reproduce.
SRC_URL="https://github.com/${REPO}/archive/${TAG}.tar.gz"
curl -fsSL --retry 3 --retry-delay 5 --retry-all-errors -o "$WORK/src.tar.gz" "$SRC_URL"
SHA256_SRC=$(sha256sum "$WORK/src.tar.gz" | awk '{print $1}')
echo "   $SRC_URL"
echo "   $SHA256_SRC"

echo "== generate PKGBUILD"
sed -e "s|@PKGNAME@|${PKGNAME}|g" \
    -e "s|@PKGVER@|${PKGVER}|g" \
    -e "s|@PKGDESC@|${PKGDESC}|g" \
    -e "s|@TAG@|${TAG}|g" \
    -e "s|@VERSION@|${VERSION}|g" \
    -e "s|@SHA256_SRC@|${SHA256_SRC}|g" \
    "$TEMPLATE" > "$WORK/PKGBUILD"

# Deliberately broader than the list substituted above, so a @commit@ added to
# the template without a matching -e here trips it too. The two exceptions belong
# to docs/mkvdup.1 rather than to this generator: package() expands them at build
# time, so they are supposed to survive into the published PKGBUILD.
if grep -n '@[A-Za-z0-9_]*@' "$WORK/PKGBUILD" \
   | grep -v '@PACKAGE_NAME@\|@PACKAGE_NAME_UPPER@'; then
  echo "::error::PKGBUILD still contains unsubstituted placeholders"
  exit 1
fi
cat "$WORK/PKGBUILD"

echo "== build"
# Seeded under the name the PKGBUILD's `::` rename gives it, so makepkg verifies
# the archive already downloaded rather than fetching it a second time. The
# checksum is still enforced against it.
cp "$WORK/src.tar.gz" "$WORK/${PKGNAME}-${PKGVER}.tar.gz"
rm -f "$WORK/src.tar.gz"

run_makepkg() {
  if [ "$(id -u)" -eq 0 ]; then
    id builder >/dev/null 2>&1 || useradd --create-home builder
    chown -R builder:builder "$WORK"
    sudo -u builder --preserve-env=GOFLAGS,GOPATH,GOCACHE bash -c "cd '$WORK' && $*"
  else
    ( cd "$WORK" && eval "$*" )
  fi
}

run_makepkg "makepkg --nodeps --noconfirm --cleanbuild"
run_makepkg "makepkg --printsrcinfo > .SRCINFO"

ls -la "$WORK"/*.pkg.tar.zst "$WORK/.SRCINFO"
