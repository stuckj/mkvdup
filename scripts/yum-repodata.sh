#!/usr/bin/env bash
# Index a directory of rpms with createrepo_c and rewrite the result so each
# package location points at the release that holds it (see yum_xmlbase.py).
#
# Usage: yum-repodata.sh <rpm-dir> <out-repodata> <assetmap> <base-url>
#
# Called by rebuild-package-repos.sh, and by the package-scripts workflow under
# several createrepo_c versions.
set -euo pipefail

SCRIPTS="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[ $# -eq 4 ] || { echo "usage: $(basename "$0") <rpm-dir> <out-repodata> <assetmap> <base-url>" >&2; exit 2; }

# --no-database: only primary.xml carries package locations, so the sqlite
# copies cannot be redirected to the releases. Shipping them would advertise
# packages/ paths that no longer exist to any client that prefers sqlite.
# --general-compress-type gz: yum_xmlbase.py reads and rewrites gzip.
# createrepo_c 1.x defaults to zstd, 0.x to gzip.
createrepo_c --quiet --no-database --general-compress-type gz "$1"
python3 "$SCRIPTS/yum_xmlbase.py" "$1/repodata" "$2" "$3" "$4"
