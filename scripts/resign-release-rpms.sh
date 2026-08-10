#!/usr/bin/env bash
# Add an OpenPGP signature to published rpms that do not have one.
#
# Packages built before rpm signing existed carry no signature, so `gpgcheck=1`
# — which every documented dnf install uses — rejects them. Signing only new
# releases would leave the archive repository unusable for exactly the old
# versions it exists to serve, so the published assets are re-signed in place.
#
# `rpm --addsign` rewrites only the signature header: the main header and the
# payload come through byte-identical, and this script proves that for every
# package rather than assuming it. A package that is already signed is left
# alone, so re-running is cheap and safe.
#
# Nothing is uploaded unless PUBLISH=1. The default run downloads, signs and
# verifies, then reports — the published releases are untouched.
#
# Replacing an asset destroys the original, so the checksums recorded in the YUM
# repodata stop matching. Dispatch "Rebuild Package Repositories" afterwards.
#
# Requires: gh (authenticated), curl, python3, rpm and rpmsign (so: a Fedora or
# EL container, not the Ubuntu runner — see the note in RELEASING.md), and gpg
# with the signing key imported and GPG_KEY_ID set.
#
# Usage: resign-release-rpms.sh <work-dir>
set -euo pipefail

SCRIPTS="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="${GITHUB_REPOSITORY:-stuckj/mkvdup}"
KEY="${GPG_KEY_ID:?GPG_KEY_ID must be set}"
export GH_TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
# Uploading requires PUBLISH=1 explicitly: the upload overwrites the only copy
# of each package that exists.
PUBLISH="${PUBLISH:-0}"
[ "$PUBLISH" = 1 ] || PUBLISH=0
# Space-separated release tags to limit the run to. Empty means every release.
ONLY_TAGS="${ONLY_TAGS:-}"

say() { printf '\n== %s\n' "$1"; }
die() { echo "FATAL: $*" >&2; exit 1; }

[ $# -eq 1 ] || die "usage: $(basename "$0") <work-dir>"
WORK="$(realpath -m -- "$1")" || die "cannot resolve '$1'"
[ -d "$(dirname "$WORK")" ] || die "cannot resolve '$1' — its parent directory does not exist"
MARKER=.resign-marker
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
mkdir -p "$WORK"/{orig,signed}
cd "$WORK"

command -v rpmsign >/dev/null || die "rpmsign is not installed (need the rpm-sign package)"

say "configure rpm signing"
# rpm shells out to gpg, which cannot prompt here, so the passphrase is fed from
# a file. It is written inside the work directory, which the caller removes.
PASSFILE="$WORK/.passphrase"
install -m 600 /dev/null "$PASSFILE"
printf '%s' "${GPG_PASSPHRASE:-}" > "$PASSFILE"
# SHA-256 to match what nfpm produces for newly built packages; rpm's own
# default has been SHA-1 in older versions, which modern crypto policies reject.
cat > "$HOME/.rpmmacros" <<EOF
%_signature gpg
%_gpg_name $KEY
%_gpg_digest_algo sha256
%__gpg_sign_cmd %{__gpg} gpg --batch --no-verbose --no-armor --pinentry-mode loopback --passphrase-file $PASSFILE --digest-algo sha256 --no-secmem-warning -u "%{_gpg_name}" -sbo %{__signature_filename} %{__plaintext_filename}
EOF
echo "  key $KEY, sha256 digests"

say "enumerate published rpms"
# browser_download_url is fetched with curl and costs no API request, unlike
# `gh release download` which spends one per asset.
gh api "repos/$REPO/releases" --paginate \
  -q '.[] | select(.draft == false) | select(.tag_name | startswith("v"))
      | .tag_name as $t | .assets[] | select(.state == "uploaded")
      | "\($t)\t\(.name)\t\(.browser_download_url)"' \
  | awk -F'\t' '$2 ~ /\.rpm$/' | sort -u > assets.tsv

if [ -n "$ONLY_TAGS" ]; then
  # Word splitting is what turns the space-separated input into patterns.
  # shellcheck disable=SC2086
  printf '%s\n' $ONLY_TAGS > wanted.txt
  awk -F'\t' 'NR==FNR{w[$1];next} $1 in w' wanted.txt assets.tsv > filtered.tsv
  mv filtered.tsv assets.tsv
  echo "  limited to: $ONLY_TAGS"
fi

total=$(wc -l < assets.tsv)
[ "$total" -gt 0 ] || die "no rpm assets found — refusing to report success over an empty set"
echo "  $total rpm asset(s) across $(cut -f1 assets.tsv | sort -u | wc -l) release(s)"

say "download, sign and verify"
signed=0; skipped=0; failed=0
: > to-upload.txt
: > failures.txt

while IFS=$'\t' read -r tag name url; do
  mkdir -p "orig/$tag" "signed/$tag"
  src="orig/$tag/$name"
  dst="signed/$tag/$name"

  curl -fsSL --retry 3 --retry-delay 2 \
       --connect-timeout 30 --speed-limit 1024 --speed-time 60 \
       -o "$src" "$url" || die "could not download $tag/$name"

  # Already signed: nothing to do. Keeps a re-run from churning assets that are
  # fine, and lets this run alongside releases that are signed at build time.
  if python3 "$SCRIPTS/check-rpm-signature.py" "$src" >/dev/null 2>&1; then
    skipped=$((skipped + 1))
    continue
  fi

  cp -- "$src" "$dst"
  if ! rpmsign --addsign "$dst" >"sign.log" 2>&1; then
    failed=$((failed + 1))
    { echo "$tag/$name: rpmsign failed"; sed 's/^/    /' sign.log; } >> failures.txt
    continue
  fi

  # rpmsign reports success even where it has changed nothing — that is how the
  # first attempt at this on an Ubuntu runner produced 270 unsigned packages and
  # a green build. Check the bytes.
  if ! python3 "$SCRIPTS/check-rpm-signature.py" "$dst" >/dev/null 2>&1; then
    failed=$((failed + 1))
    echo "$tag/$name: rpmsign exited 0 but the package is still unsigned" >> failures.txt
    continue
  fi

  # The signature header is the only part allowed to differ. Anything else means
  # the package contents changed, and this is the last moment it can be caught:
  # the upload overwrites the original.
  if ! python3 - "$src" "$dst" <<'PY'
import struct, sys

def body_offset(blob):
    """Offset of the main header: past the lead and the signature header,
    whose data store is padded to an 8-byte boundary."""
    count, size = struct.unpack(">II", blob[104:112])
    end = 112 + 16 * count + size
    return end + (-end % 8)

a = open(sys.argv[1], "rb").read()
b = open(sys.argv[2], "rb").read()
oa, ob = body_offset(a), body_offset(b)
if b[ob:ob + 3] != b"\x8e\xad\xe8":
    sys.exit("signed package has no main header where one is expected")
sys.exit(0 if a[oa:] == b[ob:] else "main header or payload changed")
PY
  then
    failed=$((failed + 1))
    echo "$tag/$name: content changed during signing" >> failures.txt
    continue
  fi

  printf '%s\t%s\t%s\n' "$tag" "$name" "$dst" >> to-upload.txt
  signed=$((signed + 1))
done < assets.tsv

echo "  signed $signed, already signed $skipped, failed $failed"
if [ "$failed" -gt 0 ]; then
  sed 's/^/    /' failures.txt
  die "$failed package(s) could not be signed — nothing has been uploaded"
fi

if [ "$signed" -eq 0 ]; then
  say "nothing to do"
  echo "  every published rpm already carries a signature"
  exit 0
fi

if [ "$PUBLISH" != 1 ]; then
  say "dry run — nothing uploaded"
  echo "  $signed re-signed package(s) are under $WORK/signed"
  echo "  re-run with PUBLISH=1 to replace the published assets"
  exit 0
fi

say "upload"
# Each replacement is a delete plus an upload. GITHUB_TOKEN allows 1000 requests
# per hour per repository, so a full backfill sits inside one budget but leaves
# little room to share the hour with anything else.
remaining=$(gh api rate_limit -q '.resources.core.remaining' 2>/dev/null || echo unknown)
need=$((signed * 2 + 10))
echo "  $need request(s) needed, $remaining remaining"
case "$remaining" in
  ''|*[!0-9]*) echo "  could not read the rate limit; continuing" ;;
  *) [ "$remaining" -ge "$need" ] || die "not enough API budget left this hour" ;;
esac

uploaded=0
while IFS=$'\t' read -r tag name path; do
  gh release upload "$tag" "$path" --clobber --repo "$REPO" \
    || die "failed to upload $name to $tag — $uploaded asset(s) already replaced, re-run to finish"
  uploaded=$((uploaded + 1))
  if [ $((uploaded % 25)) -eq 0 ]; then echo "  $uploaded/$signed"; fi
done < to-upload.txt

say "done"
echo "  replaced $uploaded published rpm(s)"
echo "  the YUM repodata still records the old checksums —"
echo "  dispatch 'Rebuild Package Repositories' now"
