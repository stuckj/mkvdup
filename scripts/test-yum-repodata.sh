#!/usr/bin/env bash
# Run yum-repodata.sh over a published rpm with whatever createrepo_c is
# installed, and check the output is a repository a client can use: gzip
# primary, every location carrying the release xml:base, and repomd.xml
# checksums matching the files beside it. If dnf is present it must also read
# the result.
#
# Usage: test-yum-repodata.sh <work-directory> [release-tag]
set -euo pipefail

SCRIPTS="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="${GITHUB_REPOSITORY:-stuckj/mkvdup}"
BASE="https://github.com/${REPO}/releases/download"
WORK="${1:?usage: $(basename "$0") <work-directory> [release-tag]}"
TAG="${2:-v1.9.3}"
ASSET="mkvdup-${TAG#v}-1.x86_64.rpm"

fail() { echo "FAIL: $*" >&2; exit 1; }

echo "createrepo_c $(createrepo_c --version | head -n1)"
rm -rf "$WORK"
mkdir -p "$WORK/rpms"
curl -fsSL --retry 5 --retry-all-errors --retry-delay 10 -o "$WORK/rpms/$ASSET" "$BASE/$TAG/$ASSET"
echo "$TAG/$ASSET" > "$WORK/assetmap.txt"

bash "$SCRIPTS/yum-repodata.sh" "$WORK/rpms" "$WORK/out/repodata" "$WORK/assetmap.txt" "$BASE"

python3 - "$WORK/out/repodata" "$BASE/$TAG/" <<'EOF'
import gzip, hashlib, os, sys
import xml.etree.ElementTree as ET

repodata, want_base = sys.argv[1:3]
NS = "{http://linux.duke.edu/metadata/repo}"
root = ET.parse(os.path.join(repodata, "repomd.xml")).getroot()
types = set()
for data in root.findall(f"{NS}data"):
    t = data.get("type")
    types.add(t)
    path = os.path.join(repodata, os.path.basename(data.find(f"{NS}location").get("href")))
    blob = open(path, "rb").read()
    ck = data.find(f"{NS}checksum")
    if hashlib.new(ck.get("type"), blob).hexdigest() != ck.text:
        sys.exit(f"FAIL: {t}: checksum in repomd.xml does not match {path}")
    if t == "primary":
        if not path.endswith(".gz"):
            sys.exit(f"FAIL: primary is not gzip: {path}")
        xml = gzip.decompress(blob).decode()
        if f'xml:base="{want_base}"' not in xml:
            sys.exit(f"FAIL: primary has no location with xml:base {want_base}")
if "primary" not in types:
    sys.exit("FAIL: repomd.xml lists no primary")
print(f"repodata ok: {', '.join(sorted(types))}")
EOF

if command -v dnf >/dev/null; then
  got=$(dnf -q repoquery --repofrompath="t,file://$WORK/out" --repo=t \
          --setopt=t.gpgcheck=0 --qf '%{location}\n' mkvdup)
  echo "dnf sees: $got"
  [[ "$got" == "$BASE/$TAG/"* ]] || fail "dnf location '$got' is not under $BASE/$TAG/"
fi
echo PASS
