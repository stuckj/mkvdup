#!/usr/bin/env python3
"""Point a createrepo_c repodata at the release assets that hold the packages.

RPM-MD allows an absolute location per package via xml:base, so the repodata can
be served from GitHub Pages while the rpms themselves stay in the per-version
GitHub releases. Only primary.xml carries locations; the other metadata files are
copied through and repomd.xml is re-emitted so every checksum matches what is
actually served.

GitHub rewrites '~' to '.' in release asset names, so a package built as
1.8.2~canary.2 is stored as 1.8.2.canary.2 and the href must use the stored form.

Usage: yum_xmlbase.py <in-repodata> <out-repodata> <assetmap> <base-url>
  assetmap lines are "<release-tag>/<asset-name>".
"""
import gzip
import hashlib
import os
import re
import shutil
import sys
import time
import xml.etree.ElementTree as ET
from xml.sax.saxutils import quoteattr

NS = "http://linux.duke.edu/metadata/repo"


def main(src, dst, mapfile, base):
    tag_of = {}
    for line in open(mapfile):
        line = line.strip()
        if line:
            tag, name = line.split("/", 1)
            tag_of[name] = tag

    os.makedirs(dst, exist_ok=True)
    ET.register_namespace("", NS)
    tree = ET.parse(os.path.join(src, "repomd.xml"))
    root = tree.getroot()
    missing = []

    for data in root.findall(f"{{{NS}}}data"):
        loc = data.find(f"{{{NS}}}location")
        name = os.path.basename(loc.get("href"))
        srcf = os.path.join(src, name)

        if data.get("type") != "primary":
            shutil.copy2(srcf, os.path.join(dst, name))
            loc.set("href", f"repodata/{name}")
            continue

        raw = gzip.decompress(open(srcf, "rb").read()).decode()

        def point_at_release(m):
            href = os.path.basename(m.group(1))
            stored = href.replace("~", ".")
            tag = tag_of.get(stored) or tag_of.get(href)
            if not tag:
                missing.append(href)
                return m.group(0)
            # quoteattr supplies the quotes; a tag or asset name containing & or
            # " would otherwise produce XML that fails the whole repository.
            b = quoteattr(f"{base}/{tag}/")
            return f"<location xml:base={b} href={quoteattr(stored)}/>"

        raw, n = re.subn(r'<location href="([^"]+)"\s*/>', point_at_release, raw)
        if missing:
            sys.exit("no release holds: " + ", ".join(sorted(set(missing))))
        if not n:
            sys.exit("no <location> elements matched — createrepo_c output changed shape")

        # createrepo_c stamps each package's filesystem mtime into
        # <time file="…">, and the packages are re-downloaded on every run, so
        # the metadata would otherwise differ every rebuild even with an
        # unchanged package set — a fresh gh-pages blob each time and a pointless
        # re-download for every client. Only --update consults this field, and
        # this pipeline always rebuilds from scratch.
        raw = re.sub(r'(<time file=")\d+(")', r"\g<1>0\g<2>", raw)

        body = raw.encode()
        # mtime=0 likewise: gzip's default only became deterministic in Python
        # 3.14 and the runner is on 3.12.
        gz = gzip.compress(body, mtime=0)
        # createrepo_c prefixes the filename with the digest of the file it
        # wrote (--unique-md-filenames), so that changed metadata gets a changed
        # URL. Keeping its name for our rewritten bytes would break that: a cache
        # holding the old URL would serve content repomd.xml no longer describes.
        name = f"{hashlib.sha256(gz).hexdigest()}-primary.xml.gz"
        open(os.path.join(dst, name), "wb").write(gz)
        # Set type= alongside the digest. Inheriting whatever createrepo_c wrote
        # would mislabel the value if a future default emitted sha512, and every
        # dnf client would reject the repository as a checksum mismatch.
        ck = data.find(f"{{{NS}}}checksum")
        ck.text = hashlib.sha256(gz).hexdigest()
        ck.set("type", "sha256")
        for tagname, val in (("open-checksum", hashlib.sha256(body).hexdigest()),
                             ("size", len(gz)), ("open-size", len(body))):
            el = data.find(f"{{{NS}}}{tagname}")
            if el is not None:
                el.text = str(val)
                if tagname == "open-checksum":
                    el.set("type", "sha256")
        loc.set("href", f"repodata/{name}")
        print(f"    primary: {n} locations now carry xml:base")

    rev = root.find(f"{{{NS}}}revision")
    if rev is not None:
        rev.text = str(int(time.time()))
    tree.write(os.path.join(dst, "repomd.xml"), xml_declaration=True, encoding="UTF-8")


if __name__ == "__main__":
    main(*sys.argv[1:5])
