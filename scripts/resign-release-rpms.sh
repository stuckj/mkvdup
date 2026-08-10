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
# Requires: gh (authenticated), curl, python3, rpmsign built with gpg signing
# support — the Fedora container resign-rpms.yml runs in provides it — and gpg
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
# GitHub allows "no more than 80 content-generating requests per minute and no
# more than 500 content-generating requests per hour"
# (docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api,
# read 2026-08-10). Replacing an asset is a delete plus an upload, so a full
# backfill needs more writes than one hour allows and *must* stop partway. It
# stops on its own terms rather than on a 403 landing between a delete and its
# upload; the next run resumes, because signed packages are skipped.
WRITE_BUDGET="${WRITE_BUDGET:-450}"
# Re-signing skips packages that already carry any signature. After a key
# rotation that is every one of them, so FORCE=1 re-signs regardless.
FORCE="${FORCE:-0}"
# Waits before each upload attempt, and the pause between releases. GitHub's
# secondary limit on content-generating requests is per minute and answers 403
# for at least that long, so the retries have to outlast it. Overridable so a
# test does not have to sit through the real thing.
UPLOAD_BACKOFF="${UPLOAD_BACKOFF:-0 60 300 900}"
UPLOAD_PACE="${UPLOAD_PACE:-2}"

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
# signed/ survives: after a failed upload it holds the only copy of a package
# that has already been deleted from its release, and the obvious response to a
# failed run is to re-run it with the same work directory.
# .rpmmacros.saved survives too: a run killed with SIGKILL leaves the operator's
# original only there, and wiping it before re-saving would replace it with this
# script's own generated file.
find "$WORK" -mindepth 1 -maxdepth 1 \
     ! -name signed ! -name "$MARKER" ! -name .rpmmacros.saved \
     -exec rm -rf {} + 2>/dev/null || true
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
# all-assets.tsv is the unfiltered view, kept because orphan detection below
# must ask "is this package on its release?" of every release, not just the
# ones this run was asked to touch.
enumerate all-assets.tsv
cp all-assets.tsv assets.tsv

if [ -n "$ONLY_TAGS" ]; then
  # Tags are matched exactly, not as patterns. -f keeps the shell from expanding
  # anything glob-like in the input against the work directory.
  set -f
  # shellcheck disable=SC2086
  printf '%s\n' $ONLY_TAGS | LC_ALL=C sort -u > wanted.txt
  set +f
  awk -F'\t' 'NR==FNR{w[$1];next} $1 in w' wanted.txt assets.tsv > filtered.tsv
  mv filtered.tsv assets.tsv
  # A typo in a tag would otherwise silently narrow the run to nothing.
  cut -f1 assets.tsv | LC_ALL=C sort -u > matched.txt
  if ! LC_ALL=C comm -23 wanted.txt matched.txt | grep -q .; then
    echo "  limited to: $ONLY_TAGS"
  else
    die "no rpm assets found for tag(s): $(LC_ALL=C comm -23 wanted.txt matched.txt | tr '\n' ' ')"
  fi
fi

total=$(wc -l < assets.tsv)
[ "$total" -gt 0 ] || die "no rpm assets found — refusing to report success over an empty set"
echo "  $total rpm asset(s) across $(cut -f1 assets.tsv | sort -u | wc -l) release(s)"

# A signed copy left by an earlier run whose release no longer advertises it is
# an asset this tool deleted and did not put back. Enumeration cannot see it —
# it is gone from the release — so without this a re-run would sign what remains
# and report success while the package is still missing.
#
# Judged against the *unfiltered* enumeration. Against the ONLY_TAGS-filtered
# one every signed copy from another release looks orphaned, which would turn a
# two-tag run into a replacement of everything on disk.
: > orphans.txt
if [ -d signed ]; then
  while IFS= read -r path; do
    tag=$(basename "$(dirname "$path")")
    name=$(basename "$path")
    if [ -n "$ONLY_TAGS" ] && ! grep -qxF "$tag" wanted.txt; then continue; fi
    awk -F'\t' -v t="$tag" -v n="$name" '$1 == t && $2 == n {found=1} END {exit !found}' \
      all-assets.tsv || printf '%s\t%s\t%s\n' "$tag" "$name" "$path" >> orphans.txt
  done < <(find signed -type f -name '*.rpm')
fi
if [ -s orphans.txt ]; then
  echo "  $(wc -l < orphans.txt) package(s) are missing from their release but"
  echo "  present here from an earlier run:"
  cut -f1,2 orphans.txt | sed 's|^|    |;s|\t|/|'
  if [ "$PUBLISH" != 1 ]; then
    die "re-run with PUBLISH=1 to put them back"
  fi
  echo "  they will be uploaded first"
fi

say "download, sign and verify"
signed=0; skipped=0; failed=0
orphans=$(wc -l < orphans.txt)
# Orphans are already signed and already queued; they only need putting back.
cp orphans.txt to-upload.txt
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

  # Already signed by *this* key: nothing to do. Keeps a re-run from churning
  # assets that are fine, and lets a run that stopped early resume. Asking whose
  # signature it is, rather than whether there is one, is what makes a key
  # rotation resumable too: after a rotation every package still carries the
  # retired key's signature, and a presence-only test would report nothing left
  # to do while none of them verify.
  if [ "$FORCE" != 1 ] \
     && python3 "$SCRIPTS/check-rpm-signature.py" --key "$KEY" "$src" >/dev/null 2>&1; then
    skipped=$((skipped + 1))
    continue
  fi

  cp -- "$src" "$dst"
  if ! rpmsign --addsign "$dst" >"sign.log" 2>&1; then
    failed=$((failed + 1))
    { echo "$tag/$name: rpmsign failed"; sed 's/^/    /' sign.log; } >> failures.txt
    continue
  fi

  # rpmsign exits 0 when it has changed nothing: it skips a package that already
  # carries an identical signature, and it cannot distinguish that from a signing
  # backend that produced none. Check the bytes rather than the exit status.
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

queued=$((signed + orphans))
if [ "$queued" -eq 0 ]; then
  say "nothing to do"
  echo "  every published rpm already carries a signature"
  echo "  (if the repositories were left mid-backfill, rebuild them anyway —"
  echo "   the repodata records whatever checksums were current when it ran)"
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
# Orphans first. They are the only packages that are *currently missing* from
# their release; everything else is merely still unsigned, which is the state
# the repository has been in all along. If the budget runs out, it should run
# out having restored those.
cut -f1 orphans.txt | LC_ALL=C sort -u > orphan-tags.txt
cut -f1 to-upload.txt | LC_ALL=C sort -u > other-tags.txt
cat orphan-tags.txt > upload-tags.txt
LC_ALL=C comm -23 other-tags.txt orphan-tags.txt >> upload-tags.txt
orphan_tags=$(wc -l < orphan-tags.txt)
tags_total=$(wc -l < upload-tags.txt)
echo "  $queued package(s) across $tags_total release(s)"
echo "  write budget for this run: $WRITE_BUDGET content-generating request(s)"

budget() { gh api rate_limit -q '.resources.core.remaining' 2>/dev/null || echo unknown; }

uploaded=0
tags_done=0
writes=0
stopped_at=""

while read -r tag; do
  mapfile -t paths < <(awk -F'\t' -v t="$tag" '$1 == t {print $3}' to-upload.txt)
  # A delete and an upload per asset, all content-generating.
  writes_needed=$((2 * ${#paths[@]}))
  if [ $((writes + writes_needed)) -gt "$WRITE_BUDGET" ]; then stopped_at="$tag"; break; fi

  # The hourly core budget is the looser of the two and rarely binds, but a run
  # sharing its hour with a release could still hit it.
  need=$((1 + writes_needed + BUDGET_RESERVE))
  remaining=$(budget)
  case "$remaining" in
    ''|*[!0-9]*) : ;;   # unreadable: proceed rather than stall the backfill
    *) if [ "$remaining" -lt "$need" ]; then stopped_at="$tag"; break; fi ;;
  esac

  echo "  [$((tags_done + 1))/$tags_total] $tag"
  ok=0
  attempts=0
  # Backoff, not immediate retries: the limit that actually bites here is the
  # secondary one, which caps content-generating requests per minute, and a
  # backfill is two writes per asset. Retrying within seconds just spends the
  # attempts against a block that is still in force.
  # shellcheck disable=SC2086
  for delay in $UPLOAD_BACKOFF; do
    if [ "$delay" != 0 ]; then
      echo "      waiting ${delay}s before retrying $tag"
      sleep "$delay"
    fi
    # --clobber deletes the existing asset first. On a retry the delete has
    # usually already happened, which --clobber also tolerates.
    attempts=$((attempts + 1))
    if gh release upload "$tag" "${paths[@]}" --clobber --repo "$REPO" >upload.log 2>&1; then
      ok=1
      break
    fi
    sed 's/^/      /' upload.log
  done
  if [ "$ok" != 1 ]; then
    die "could not replace the assets of $tag — its rpms are probably now MISSING from that release. Recover them from $WORK/signed/$tag (the workflow keeps this as the resigned-recovery artifact) and upload them to $tag before rebuilding the repositories."
  fi

  uploaded=$((uploaded + ${#paths[@]}))
  # Charge every attempt, not just the successful one: a retry re-issues the
  # deletes and uploads, and those count against the same limit the budget
  # exists to respect.
  writes=$((writes + writes_needed * attempts))
  tags_done=$((tags_done + 1))
  echo "$uploaded" > uploaded-count.txt
  # Stay under the secondary limit on content-generating requests, which is far
  # tighter than the hourly budget the check above reads.
  if [ "$UPLOAD_PACE" != 0 ]; then sleep "$UPLOAD_PACE"; fi
done < upload-tags.txt

say "confirm every replacement is present"
# One more enumeration, not one request per asset: it costs a handful of
# requests and catches an upload that reported success but did not land.
#
# Sizes, not just names. --clobber replaces in place, so the name is there
# whether or not the replacement happened — checking names alone would pass over
# an asset that was never touched. A signed rpm is always larger than the
# unsigned original, so the published size is what distinguishes them.
# Only the releases actually reached; the rest were never meant to change.
head -n "$tags_done" upload-tags.txt > uploaded-tags.txt

# The releases API is eventually consistent — an asset uploaded seconds ago can
# be absent or still 'uploading' in the next listing. Reporting that as a lost
# package would fail the run and, worse, skip the repository rebuild that the
# assets already replaced now need. Re-read a few times before believing it.
missing=0
for attempt in 1 2 3 4 5; do
  enumerate after.tsv
  missing=0
  : > mismatches.txt
  while IFS=$'\t' read -r tag name path; do
    grep -qxF "$tag" uploaded-tags.txt || continue
    want=$(stat -c%s "$path")
    got=$(awk -F'\t' -v t="$tag" -v n="$name" '$1 == t && $2 == n {print $3; exit}' after.tsv)
    if [ -z "$got" ]; then
      echo "  MISSING $tag/$name" >> mismatches.txt
      missing=$((missing + 1))
    elif [ "$got" != "$want" ]; then
      echo "  WRONG SIZE $tag/$name: published $got, signed copy is $want" >> mismatches.txt
      missing=$((missing + 1))
    fi
  done < to-upload.txt
  [ "$missing" -eq 0 ] && break
  if [ "$attempt" != 5 ]; then
    echo "  $missing not visible yet; re-reading (attempt $attempt)"
    sleep "${VERIFY_RETRY_DELAY:-10}"
  fi
done
if [ "$missing" -ne 0 ]; then
  cat mismatches.txt
  die "$missing replaced asset(s) are missing or do not match on their release. The signed copies are under $WORK/signed — put them back, then rebuild the package repositories."
fi
echo "  all $uploaded replacement(s) present at the expected size"

if [ -n "$stopped_at" ]; then
  # Not a failure: every release it started, it finished. Exiting non-zero here
  # would make the designed outcome indistinguishable from the one where a
  # package is missing, and would suppress the repository rebuild that the
  # assets already replaced now need.
  say "stopped early to stay inside the API budget"
  echo "::warning::Replaced $uploaded of $queued package(s); stopped before $stopped_at."
  if [ "$tags_done" -lt "$orphan_tags" ]; then
    die "stopped before restoring every package that was missing from its release — $((orphan_tags - tags_done)) release(s) still short. Raise WRITE_BUDGET or pass those tags to ONLY_TAGS and run again now."
  fi
  echo "  Nothing is missing. GitHub allows 500 content-generating requests an"
  echo "  hour and each replacement costs two, so a full backfill takes more than"
  echo "  one run by design. Re-run in an hour: already-signed packages are"
  echo "  skipped, so it carries on from $stopped_at."
  exit 0
fi

say "done"
echo "  replaced $uploaded published rpm(s)"
echo "  the YUM repodata still records the old checksums —"
echo "  rebuild the package repositories now"
