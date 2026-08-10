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
# alone, so re-running is cheap — and so a run that stops early resumes where
# it left off.
#
# Nothing is uploaded unless PUBLISH=1. The default run downloads, signs and
# verifies, then reports — the published releases are untouched.
#
# Replacing an asset is a delete followed by an upload; the GitHub API has no
# atomic replace. Between the two the package does not exist, and if the upload
# never lands it is gone for good — the release asset is the only copy. So the
# upload budget is checked before each release rather than once at the start,
# and a run that cannot afford the next release stops with every release it did
# start finished. Uploads are retried, and a final pass re-reads the releases to
# confirm every replacement is actually present.
#
# Replacing an asset also invalidates the checksums in the YUM repodata, so the
# repositories must be rebuilt afterwards. resign-rpms.yml does that in a
# following job; a manual run must dispatch "Rebuild Package Repositories".
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
# Space-separated release tags to limit the run to, matched exactly. Empty means
# every release.
ONLY_TAGS="${ONLY_TAGS:-}"
# Requests to keep in reserve so a stop-short never lands between a delete and
# its upload.
BUDGET_RESERVE="${BUDGET_RESERVE:-40}"

say() { printf '\n== %s\n' "$1"; }
die() { echo "FATAL: $*" >&2; exit 1; }

[ $# -eq 1 ] || die "usage: $(basename "$0") <work-dir>"
WORK="$(realpath -m -- "$1")" || die "cannot resolve '$1'"
[ -d "$(dirname "$WORK")" ] || die "cannot resolve '$1' — its parent directory does not exist"
MARKER=.resign-marker
case "$WORK" in
  / | "${HOME:-/nonexistent}") die "refusing to use '$WORK' as a work directory" ;;
esac
# The signing macro interpolates this path into a command line rpm re-splits on
# whitespace, so a space here would break signing in a confusing way.
case "$WORK" in
  *[[:space:]]*) die "refusing to use '$WORK': the path contains whitespace" ;;
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

for tool in rpmsign curl python3 gh; do
  command -v "$tool" >/dev/null || die "$tool is not installed"
done

say "configure rpm signing"
# rpm shells out to gpg, which cannot prompt here, so the passphrase goes in a
# file. rpm only reads macros from $HOME/.rpmmacros, so an existing one is moved
# aside; the trap restores it and removes the passphrase however this exits.
PASSFILE="$WORK/.passphrase"
RPMMACROS="$HOME/.rpmmacros"
SAVED_MACROS="$WORK/.rpmmacros.saved"
cleanup() {
  rm -f "$PASSFILE"
  if [ -e "$SAVED_MACROS" ]; then
    mv -f "$SAVED_MACROS" "$RPMMACROS"
  else
    rm -f "$RPMMACROS"
  fi
}
# Save before arming the trap: armed first, an early exit here would delete an
# existing ~/.rpmmacros that had not been copied anywhere yet.
if [ -e "$RPMMACROS" ]; then cp -p "$RPMMACROS" "$SAVED_MACROS"; fi
trap cleanup EXIT
install -m 600 /dev/null "$PASSFILE"
printf '%s' "${GPG_PASSPHRASE:-}" > "$PASSFILE"
# SHA-256 to match what nfpm produces for newly built packages; rpm's own
# default has been SHA-1 in older versions, which modern crypto policies reject.
cat > "$RPMMACROS" <<EOF
%_signature gpg
%_gpg_name $KEY
%_gpg_digest_algo sha256
%__gpg_sign_cmd %{__gpg} gpg --batch --no-verbose --no-armor --pinentry-mode loopback --passphrase-file $PASSFILE --digest-algo sha256 --no-secmem-warning -u "%{_gpg_name}" -sbo %{__signature_filename} %{__plaintext_filename}
EOF
echo "  key $KEY, sha256 digests"

# browser_download_url is fetched with curl and costs no API request, unlike
# `gh release download` which spends one per asset.
enumerate() {  # enumerate <output-file>
  gh api "repos/$REPO/releases" --paginate \
    -q '.[] | select(.draft == false) | select(.tag_name | startswith("v"))
        | .tag_name as $t | .assets[] | select(.state == "uploaded")
        | "\($t)\t\(.name)\t\(.size)\t\(.browser_download_url)"' \
    | awk -F'\t' '$2 ~ /\.rpm$/' | sort -u > "$1"
}

say "enumerate published rpms"
enumerate assets.tsv

if [ -n "$ONLY_TAGS" ]; then
  # Tags are matched exactly, not as patterns. -f keeps the shell from expanding
  # anything glob-like in the input against the work directory.
  set -f
  # shellcheck disable=SC2086
  printf '%s\n' $ONLY_TAGS | sort -u > wanted.txt
  set +f
  awk -F'\t' 'NR==FNR{w[$1];next} $1 in w' wanted.txt assets.tsv > filtered.tsv
  mv filtered.tsv assets.tsv
  # A typo in a tag would otherwise silently narrow the run to nothing.
  cut -f1 assets.tsv | sort -u > matched.txt
  if ! comm -23 wanted.txt matched.txt | grep -q .; then
    echo "  limited to: $ONLY_TAGS"
  else
    die "no rpm assets found for tag(s): $(comm -23 wanted.txt matched.txt | tr '\n' ' ')"
  fi
fi

total=$(wc -l < assets.tsv)
[ "$total" -gt 0 ] || die "no rpm assets found — refusing to report success over an empty set"
echo "  $total rpm asset(s) across $(cut -f1 assets.tsv | sort -u | wc -l) release(s)"

say "download, sign and verify"
signed=0; skipped=0; failed=0
: > to-upload.txt
: > failures.txt

while IFS=$'\t' read -r tag name size url; do
  mkdir -p "orig/$tag" "signed/$tag"
  src="orig/$tag/$name"
  dst="signed/$tag/$name"

  curl -fsSL --retry 3 --retry-delay 2 \
       --connect-timeout 30 --speed-limit 1024 --speed-time 60 \
       -o "$src" "$url" || die "could not download $tag/$name"

  # A short read would otherwise be signed and uploaded over the intact original.
  got=$(stat -c%s "$src")
  [ "$got" = "$size" ] || die "$tag/$name downloaded as $got bytes, expected $size"

  # Already signed: nothing to do. Keeps a re-run from churning assets that are
  # fine, and lets a run that stopped early resume.
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
  # first attempt at this on an Ubuntu runner produced unsigned packages and a
  # green build. Check the bytes.
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
# Assets are replaced a release at a time: one release lookup covers all of its
# assets, and stopping between releases never leaves a half-replaced one.
cut -f1 to-upload.txt | sort -u > upload-tags.txt
tags_total=$(wc -l < upload-tags.txt)
echo "  $signed package(s) across $tags_total release(s)"

budget() { gh api rate_limit -q '.resources.core.remaining' 2>/dev/null || echo unknown; }

uploaded=0
tags_done=0
stopped_at=""
echo 0 > uploaded-count.txt

while read -r tag; do
  mapfile -t paths < <(awk -F'\t' -v t="$tag" '$1 == t {print $3}' to-upload.txt)
  # One release lookup, then a delete and an upload for each asset.
  need=$((1 + 2 * ${#paths[@]} + BUDGET_RESERVE))
  remaining=$(budget)
  case "$remaining" in
    ''|*[!0-9]*) : ;;   # unreadable: proceed rather than stall the backfill
    *) if [ "$remaining" -lt "$need" ]; then stopped_at="$tag"; break; fi ;;
  esac

  ok=0
  for attempt in 1 2 3; do
    # --clobber deletes the existing asset first. On a retry the delete has
    # usually already happened, which --clobber also tolerates.
    if gh release upload "$tag" "${paths[@]}" --clobber --repo "$REPO" >upload.log 2>&1; then
      ok=1
      break
    fi
    echo "  attempt $attempt failed for $tag"
    sed 's/^/      /' upload.log
  done
  if [ "$ok" != 1 ]; then
    die "could not replace the assets of $tag after 3 attempts — its rpms may now be MISSING from that release; the signed copies are under $WORK/signed/$tag"
  fi

  uploaded=$((uploaded + ${#paths[@]}))
  tags_done=$((tags_done + 1))
  echo "$uploaded" > uploaded-count.txt
  if [ $((tags_done % 20)) -eq 0 ]; then echo "  $tags_done/$tags_total releases"; fi
done < upload-tags.txt

say "confirm every replacement is present"
# One more enumeration, not one request per asset: it costs a handful of
# requests and catches an upload that reported success but did not land.
enumerate after.tsv
missing=0
while IFS=$'\t' read -r tag name path; do
  if ! awk -F'\t' -v t="$tag" -v n="$name" '$1 == t && $2 == n {found=1} END {exit !found}' after.tsv; then
    echo "  MISSING $tag/$name"
    missing=$((missing + 1))
  fi
done < to-upload.txt
[ "$missing" -eq 0 ] || die "$missing replaced asset(s) are not present on their release — the signed copies are under $WORK/signed"
echo "  all $uploaded replacement(s) present"

if [ -n "$stopped_at" ]; then
  say "stopped early to stay inside the API budget"
  echo "  replaced $uploaded of $signed package(s); stopped before $stopped_at"
  echo "  the repositories still need rebuilding for what did change"
  die "run again once the hourly budget resets to finish the rest"
fi

say "done"
echo "  replaced $uploaded published rpm(s)"
echo "  the YUM repodata still records the old checksums —"
echo "  rebuild the package repositories now"
