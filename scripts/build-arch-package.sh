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
# rather than preventing one. Re-running the job recovers a transient failure;
# a code fix does not, since the job checks out the commit the release was cut
# from -- that needs a new canary.
#
# Requires makepkg and go. archlinux:base-devel supplies makepkg but not go --
# its depends array has no Go toolchain -- so install it as well. makepkg refuses
# to run as root, so this creates an unprivileged user when it is running as one.
set -euo pipefail

VERSION="${VERSION:?VERSION must be set}"      # 1.9.2 or 1.9.2-canary.1
TAG="${TAG:?TAG must be set}"                  # v1.9.2
IS_CANARY="${IS_CANARY:-false}"
REPO="${GITHUB_REPOSITORY:-stuckj/mkvdup}"
PACKAGER="${PACKAGER:-stuckj <stuckj@users.noreply.github.com>}"
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

# $WORK is caller-supplied and about to be deleted, and RELEASING.md invites
# running this by hand -- so the argument is maintainer-typed. Same guard as
# publish-pacman-repo.sh: absolute path, never a home or working directory, and
# never an existing directory this script did not create.
WORK=$(readlink -m -- "$WORK")
case "$WORK" in
  /|"$HOME"|"$PWD") echo "::error::refusing to use $WORK as a work directory"; exit 1 ;;
esac
if [ -e "$WORK" ] && [ ! -e "$WORK/.archbuild-scratch" ]; then
  echo "::error::$WORK already exists and was not created by this script; refusing to delete it"
  exit 1
fi
rm -rf -- "$WORK"; mkdir -p "$WORK"; touch "$WORK/.archbuild-scratch"

echo "== source archive checksum"
# Taken from the archive GitHub actually serves, because that is the one an AUR
# user's makepkg will verify. Generating our own tarball here and hashing that
# would produce a checksum nobody else can reproduce.
# Substituted into the PKGBUILD verbatim, so the checksum below is provably taken
# from the URL the published package points at. Deriving it in both places let
# them disagree on a fork or after a rename, and the seeding hid that from CI.
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
    -e "s|@SRC_URL@|${SRC_URL}|g" \
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
    # PACKAGER through the environment: load_makepkg_config restores it after
    # sourcing the config, so it wins without having to write a config at all.
    # Left unset, makepkg stamps "Unknown Packager" into .PKGINFO, which is what
    # pacman -Si then shows -- unlike the maintainer nfpm.yaml gives deb and rpm.
    sudo -u builder bash -c "cd '$WORK' && PACKAGER='$PACKAGER' $*"
  else
    ( cd "$WORK" && PACKAGER="$PACKAGER" eval "$*" )
  fi
}

run_makepkg "makepkg --nodeps --noconfirm --cleanbuild"
run_makepkg "makepkg --printsrcinfo > .SRCINFO"

ls -la "$WORK"/*.pkg.tar.zst "$WORK/.SRCINFO"
