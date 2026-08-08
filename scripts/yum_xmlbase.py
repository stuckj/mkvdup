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
            return f'<location xml:base="{base}/{tag}/" href="{stored}"/>'

        raw, n = re.subn(r'<location href="([^"]+)"\s*/>', point_at_release, raw)
        if missing:
            sys.exit("no release holds: " + ", ".join(sorted(set(missing))))
        if not n:
            sys.exit("no <location> elements matched — createrepo_c output changed shape")

        body = raw.encode()
        gz = gzip.compress(body)
        open(os.path.join(dst, name), "wb").write(gz)
        data.find(f"{{{NS}}}checksum").text = hashlib.sha256(gz).hexdigest()
        for tagname, val in (("open-checksum", hashlib.sha256(body).hexdigest()),
                             ("size", len(gz)), ("open-size", len(body))):
            el = data.find(f"{{{NS}}}{tagname}")
            if el is not None:
                el.text = str(val)
        loc.set("href", f"repodata/{name}")
        print(f"    primary: {n} locations now carry xml:base")

    rev = root.find(f"{{{NS}}}revision")
    if rev is not None:
        rev.text = str(int(time.time()))
    tree.write(os.path.join(dst, "repomd.xml"), xml_declaration=True, encoding="UTF-8")


if __name__ == "__main__":
    main(*sys.argv[1:5])
