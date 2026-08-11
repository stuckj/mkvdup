#!/usr/bin/env bash
# Add an OpenPGP signature to published rpms that do not have one.
#
# Packages built before rpm signing existed carry no signature, so `gpgcheck=1`
# — which every documented dnf install uses — rejects them. Signing only new
# releases would leave the archive repository unusable for exactly the old
# versions it exists to serve, so the published assets are re-signed in place.
#
# Signing rewrites only the signature header: the main header and the payload
# come through byte-identical, and this script proves that for every package
# rather than assuming it. A package that is already signed is left
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
# Every key id that counts as "ours" when deciding whether a package is already
# correctly signed. gpg signs with a signing subkey when the key has one, so the
# id on the signature is not necessarily the primary's; resign-rpms.yml passes
# the primary and every subkey. Falls back to the primary alone.
KEY_IDS="${GPG_KEY_IDS:-$KEY}"
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
# Re-sign even packages already signed by this key. Rarely wanted: the skip is
# what makes a run resumable, so with FORCE=1 every run redoes the same first
# releases and a backfill larger than one write budget never reaches the end.
FORCE="${FORCE:-0}"
# Waits before each upload attempt. A 403 that lands on the upload half of a
# replacement has already deleted the asset, so waiting recovers the package
# where giving up loses it — hence a ladder long enough to outlast a
# secondary-limit block. It stops at 1800: the five steps total 51 minutes per
# release, so even two pathological releases stay inside the job's timeout,
# where adding an hour-long step would not. Overridable for tests.
UPLOAD_BACKOFF="${UPLOAD_BACKOFF:-0 60 300 900 1800}"
# Each release costs four writes (a delete and an upload for each of its two
# rpms), so staying under 80 a minute needs at least three seconds between
# them; five leaves room for the release lookup.
UPLOAD_PACE="${UPLOAD_PACE:-5}"
# Seconds this run may spend before it stops taking on another release. The
# write budget bounds API calls, not time: a release that exhausts the ladder
# costs 20 writes and 51 minutes, so a handful of them would run past the job's
# timeout -- and a job killed by its timeout can die between a delete and its
# upload, which is the one state where a package exists nowhere. Stopping short
# on the clock turns that into the same clean resume the write budget produces.
TIME_BUDGET="${TIME_BUDGET:-10800}"

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
# file. The macros go in ~/.rpmmacros, so an existing one is moved aside; the
# trap restores it and removes the passphrase however this exits.
PASSFILE="$WORK/.passphrase"
RPMMACROS="$HOME/.rpmmacros"
SAVED_MACROS="$WORK/.rpmmacros.saved"
# rpm >= 4.18 reads $XDG_CONFIG_HOME/rpm/macros in preference to ~/.rpmmacros
# and falls back only when that directory does not exist. If it ever does, the
# macros below are ignored, gpg is left with no passphrase and no pinentry, and
# every package fails to sign for a reason nothing here would report. Refuse
# rather than sign nothing quietly.
XDG_RPM="${XDG_CONFIG_HOME:-$HOME/.config}/rpm"
if [ -d "$XDG_RPM" ]; then
  die "'$XDG_RPM' exists, so rpm would read its macros in preference to $RPMMACROS"
fi
cleanup() {
  rm -f "$PASSFILE"
  if [ -e "$SAVED_MACROS" ]; then
    mv -f "$SAVED_MACROS" "$RPMMACROS"
  else
    rm -f "$RPMMACROS"
  fi
}
# Save before arming the trap: armed first, an early exit here would delete an
# existing ~/.rpmmacros that had not been copied anywhere yet. Not when a saved
# copy already exists, and not when the file in place is one of ours: after a
# run that died without its trap either would mean adopting this script's own
# generated macros as the operator's, and the trap would then install them
# permanently — pointing at a passphrase file that no longer exists.
MACRO_SENTINEL="# generated by resign-release-rpms.sh — safe to delete"
if [ -e "$RPMMACROS" ] && [ ! -e "$SAVED_MACROS" ] \
   && ! grep -qF "$MACRO_SENTINEL" "$RPMMACROS"; then
  cp -p "$RPMMACROS" "$SAVED_MACROS"
fi
trap cleanup EXIT
install -m 600 /dev/null "$PASSFILE"
printf '%s' "${GPG_PASSPHRASE:-}" > "$PASSFILE"
# Only the passphrase is added; rpm's own signing command is left alone. It has
# to be, because its shape is version-specific: rpm 6 made %__gpg_sign_cmd
# parametric (%{1} plaintext on stdin, %{2} signature), stopped repeating gpg as
# argv[0], dropped __plaintext_filename, and reads the identity from
# %_openpgp_sign_id rather than %_gpg_name. Replacing it works on one generation
# and fails on the other -- measured: an override signs on rpm 4.20.1 and dies
# with "/usr/bin/gpg exec failed (2)" on rpm 6.0.2, while the form below signs on
# both. Both generations interpolate %_gpg_sign_cmd_extra_args.
#
# Both identity macros are set because which one is read depends on the version;
# the unused one is inert. SHA-256 matches what nfpm produces for new packages.
cat > "$RPMMACROS" <<EOF
$MACRO_SENTINEL
%_gpg_name $KEY
%_openpgp_sign_id $KEY
%_gpg_digest_algo sha256
%_gpg_sign_cmd_extra_args --batch --pinentry-mode loopback --passphrase-file $PASSFILE
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
  # A typo in a tag would otherwise silently narrow the run to nothing. A tag
  # whose rpms this tool deleted has no assets left to match, so the signed
  # copies on disk count as a match too -- otherwise the recovery instruction
  # ("pass those tags to ONLY_TAGS") is rejected by this very check.
  { cut -f1 assets.tsv
    # Only directories that actually hold a package: every enumerated tag gets
    # an empty signed/<tag> created for it, so their mere existence would let
    # any previously seen tag past this check.
    [ -d signed ] && find signed -mindepth 2 -name '*.rpm' -printf '%h\n' \
      | sed 's|^signed/||'
  } | LC_ALL=C sort -u > matched.txt
  if ! LC_ALL=C comm -23 wanted.txt matched.txt | grep -q .; then
    echo "  limited to: $ONLY_TAGS"
  else
    die "no rpm assets found for tag(s): $(LC_ALL=C comm -23 wanted.txt matched.txt | tr '\n' ' ')"
  fi
fi

total=$(wc -l < assets.tsv)
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

# Nothing to work on at all. Orphans count: a tag whose rpms this tool deleted
# has no assets left to enumerate, and restoring it is exactly the job.
if [ "$total" -eq 0 ] && [ ! -s orphans.txt ]; then
  die "no rpm assets found — refusing to report success over an empty set"
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
     && python3 "$SCRIPTS/check-rpm-signature.py" --key "$KEY_IDS" "$src" >/dev/null 2>&1; then
    skipped=$((skipped + 1))
    continue
  fi

  cp -- "$src" "$dst"
  # --resign, not --addsign: rpm 6 refuses to add a second header signature
  # ("already contains a legacy signature") and only deletes the existing one
  # under --resign, which is what re-signing after a key rotation needs. rpm 4
  # treats the two as identical. Measured on both: --addsign re-signs fine on
  # 4.20.1 and exits 1 on 6.0.2, --resign works on both, contents unchanged.
  # </dev/null: this loop reads assets.tsv on stdin, and rpm 6 hands the
  # plaintext to gpg on *its* stdin, so anything that reads from ours would
  # swallow the rest of the work list.
  if ! rpmsign --resign "$dst" >"sign.log" 2>&1 </dev/null; then
    failed=$((failed + 1))
    { echo "$tag/$name: rpmsign failed"; sed 's/^/    /' sign.log; } >> failures.txt
    continue
  fi

  # rpmsign exits 0 when it has changed nothing: it skips a package that already
  # carries an identical signature, and it cannot distinguish that from a signing
  # backend that produced none. Check the bytes rather than the exit status.
  # --key as well: on a re-sign the source already carried a signature, so
  # "there is one" proves nothing. It has to be by a key this run holds.
  if ! python3 "$SCRIPTS/check-rpm-signature.py" --key "$KEY_IDS" "$dst" >/dev/null 2>&1; then
    failed=$((failed + 1))
    echo "$tag/$name: rpmsign exited 0 but the package is not signed by $KEY_IDS" >> failures.txt
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
# Every asset must have been accounted for. Without this, anything that ends the
# loop early — a command that consumes the work list on stdin, say — reports a
# tidy summary over a fraction of the packages and exits 0.
if [ $((signed + skipped + failed)) -ne "$total" ]; then
  die "only $((signed + skipped + failed)) of $total asset(s) were processed — refusing to report on a partial pass"
fi

# Built before the failure gate below: a dry run that died on one package is
# exactly when the ones that did sign are worth looking at. A handful only —
# all of them would be ~590 MiB.
rm -rf sample && mkdir -p sample
head -4 to-upload.txt | cut -f3 | while IFS= read -r f; do cp -- "$f" sample/; done
if [ "$failed" -gt 0 ]; then
  sed 's/^/    /' failures.txt
  die "$failed package(s) could not be signed — nothing has been uploaded"
fi

queued=$((signed + orphans))
# Written before any early exit so the caller always has a count to read.
echo 0 > uploaded-count.txt
if [ "$queued" -eq 0 ]; then
  say "nothing to do"
  echo "::warning::Every published rpm is already signed, so nothing was replaced. If an earlier backfill's rebuild did not finish, the published repodata still records pre-signing checksums — dispatch 'Rebuild Package Repositories'."
  exit 0
fi

if [ "$PUBLISH" != 1 ]; then
  say "dry run — nothing uploaded"
  echo "  $signed re-signed package(s) are under $WORK/signed"
  echo "  a sample of $(find sample -type f | wc -l) is under $WORK/sample"
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
# comm needs both inputs byte-sorted; the upload order does not have to be.
# Newest release first, because a full backfill takes more than one run and the
# version `dnf install mkvdup` resolves to should be fixed by the first.
cat orphan-tags.txt > upload-tags.txt
LC_ALL=C comm -23 other-tags.txt orphan-tags.txt | sort -rV >> upload-tags.txt
orphan_tags=$(wc -l < orphan-tags.txt)
tags_total=$(wc -l < upload-tags.txt)
echo "  $queued package(s) across $tags_total release(s)"
echo "  write budget for this run: $WRITE_BUDGET content-generating request(s)"

budget() { gh api rate_limit -q '.resources.core.remaining' 2>/dev/null || echo unknown; }

uploaded=0
tags_done=0
writes=0
stopped_at=""
stopped_reason=budget
# The longest a single release can spend waiting out rate-limit blocks.
WORST_CASE_BACKOFF=0
for _d in $UPLOAD_BACKOFF; do WORST_CASE_BACKOFF=$((WORST_CASE_BACKOFF + _d)); done

while read -r tag; do
  mapfile -t paths < <(awk -F'\t' -v t="$tag" '$1 == t {print $3}' to-upload.txt)
  # A delete and an upload per asset, all content-generating.
  writes_needed=$((2 * ${#paths[@]}))
  if [ $((writes + writes_needed)) -gt "$WRITE_BUDGET" ]; then stopped_at="$tag"; break; fi
  # Room for this release to exhaust the whole retry ladder and still finish
  # inside the budget, so the run is never killed mid-replacement.
  if [ $((SECONDS + WORST_CASE_BACKOFF)) -gt "$TIME_BUDGET" ]; then
    stopped_at="$tag"; stopped_reason=clock; break
  fi

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
    if gh release upload "$tag" "${paths[@]}" --clobber --repo "$REPO" \
         >upload.log 2>&1 </dev/null; then
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

if [ -z "$stopped_at" ] && [ "$tags_done" -ne "$tags_total" ]; then
  die "only $tags_done of $tags_total release(s) were uploaded without the budget stopping the run — refusing to report success"
fi

say "confirm every replacement is present"
# One more enumeration, not one request per asset: it costs a handful of
# requests and catches an upload that reported success but did not land.
#
# Sizes, not just names. --clobber replaces in place, so the name is there
# whether or not the replacement happened — checking names alone would pass over
# an asset that was never touched. The comparison is against the exact size of
# the signed copy on disk, so it holds however the signature changed the size.
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
  echo "::warning::Replaced $uploaded of $queued package(s); stopped before $stopped_at (${stopped_reason})."
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
