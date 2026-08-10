#!/usr/bin/env python3
"""Fail unless every named rpm carries an OpenPGP signature over its header.

`gpgcheck=1` verifies a signature stored *inside* the package, so an unsigned
rpm cannot be installed from a repository that asks for one. nfpm signs only
when rpm.signature.key_file resolves to a readable key and reports success
either way, so a missing or empty key produces an unsigned package and a green
build. This asserts the signature is actually there.

It parses the rpm directly rather than shelling out to `rpm -K` because the
release runners are Ubuntu and have no rpm, and because `rpm -K` conflates
"unsigned" with "signed by a key I do not have" in its exit status.

With --key it also requires the signature to be by that key, reported from the
signature's Issuer subpacket. The backfill needs that distinction: after a key
rotation every package is still signed, by the retired key, and a
presence-only test would report nothing left to do.

Verifying the signature is *cryptographically valid* is deliberately out of
scope — that needs the public key and rpm's own header canonicalisation, and it
is what the distro matrix in RELEASING.md covers.

Usage: check-rpm-signature.py [--key <keyid>] <package.rpm> [<package.rpm> ...]
"""

import struct
import sys

LEAD_SIZE = 96
HEADER_MAGIC = b"\x8e\xad\xe8"

# Signature-header tags that hold an OpenPGP signature packet. RPM's names are
# historical: RSAHEADER and DSAHEADER hold a signature over the header alone,
# PGP and GPG one over header+payload, and any tag may carry any algorithm.
SIG_TAGS = {267: "DSAHEADER", 268: "RSAHEADER", 1002: "PGP", 1005: "GPG"}

PUBKEY_ALGO = {1: "RSA", 3: "RSA-sign-only", 17: "DSA", 19: "ECDSA",
               22: "EdDSA", 25: "X25519", 27: "Ed25519"}
HASH_ALGO = {1: "MD5", 2: "SHA1", 8: "SHA256", 9: "SHA384", 10: "SHA512"}


def signature_header(blob):
    """Return {tag: bytes} for the rpm's signature header.

    The lead is a fixed 96 bytes, immediately followed by the signature header:
    3-byte magic, version, 4 reserved bytes, entry count, and data-store size.
    """
    if len(blob) < LEAD_SIZE + 16:
        raise ValueError("file is too short to be an rpm")
    off = LEAD_SIZE
    if blob[off:off + 3] != HEADER_MAGIC:
        raise ValueError("no rpm header magic at offset 96")
    count, store_size = struct.unpack(">II", blob[off + 8:off + 16])
    index = off + 16
    store = index + 16 * count
    if store + store_size > len(blob):
        raise ValueError("signature header runs past end of file")
    entries = {}
    for i in range(count):
        tag, _type, offset, length = struct.unpack(
            ">iiii", blob[index + 16 * i:index + 16 * i + 16]
        )
        # Offsets are signed on the wire and are not otherwise constrained, so a
        # malformed file could otherwise address bytes outside the data store.
        if offset < 0 or length < 0 or offset + length > store_size:
            raise ValueError(f"tag {tag} points outside the signature data store")
        entries[tag] = blob[store + offset:store + offset + length]
    return entries


def openpgp_packets(blob):
    """Yield (tag, body) for each OpenPGP packet in blob."""
    i = 0
    while i < len(blob):
        c = blob[i]
        if not c & 0x80:                       # not a packet header; give up
            return
        if c & 0x40:                           # RFC 4880 new format
            tag = c & 0x3F
            i += 1
            first = blob[i]
            if first < 192:
                length, i = first, i + 1
            elif first < 224:
                length = ((first - 192) << 8) + blob[i + 1] + 192
                i += 2
            else:
                length = struct.unpack(">I", blob[i + 1:i + 5])[0]
                i += 5
        else:                                  # old format
            tag = (c & 0x3C) >> 2
            size = {0: 1, 1: 2, 2: 4}.get(c & 0x03)
            i += 1
            if size is None:                   # indeterminate: rest of blob
                yield tag, blob[i:]
                return
            length = int.from_bytes(blob[i:i + size], "big")
            i += size
        yield tag, blob[i:i + length]
        i += length


def subpackets(area):
    """Yield (type, data) for each subpacket in a v4 subpacket area."""
    i = 0
    while i < len(area):
        first = area[i]
        if first < 192:
            length, i = first, i + 1
        elif first < 255:
            length = ((first - 192) << 8) + area[i + 1] + 192
            i += 2
        else:
            length = struct.unpack(">I", area[i + 1:i + 5])[0]
            i += 5
        if length == 0:
            return
        yield area[i], area[i + 1:i + length]     # length counts the type octet
        i += length


def issuer(body):
    """Long key ID of a v4 signature's issuer, lowercase hex, or None.

    Issuer (subpacket 16) carries the 8-byte key ID directly. Issuer
    Fingerprint (33) carries a version octet then the fingerprint, whose last
    eight bytes are the key ID for v4 keys. gpg emits both; either will do.
    """
    hashed_len = struct.unpack(">H", body[4:6])[0]
    pos = 6 + hashed_len
    unhashed_len = struct.unpack(">H", body[pos:pos + 2])[0]
    areas = (body[6:6 + hashed_len], body[pos + 2:pos + 2 + unhashed_len])
    fingerprint = None
    for area in areas:
        for kind, data in subpackets(area):
            if kind == 16 and len(data) == 8:
                return data.hex()
            if kind == 33 and len(data) >= 21:
                fingerprint = data[1:].hex()
    return fingerprint[-16:] if fingerprint else None


def describe(body):
    """Describe a v4 signature packet body as 'EdDSA/SHA256 by 4599d3032f3cd570'."""
    if len(body) < 6:
        return "malformed", None
    version = body[0]
    if version != 4:
        return f"v{version} signature", None
    pub, hsh = body[2], body[3]
    try:
        key = issuer(body)
    except (IndexError, struct.error):
        key = None
    text = f"{PUBKEY_ALGO.get(pub, pub)}/{HASH_ALGO.get(hsh, hsh)}"
    return (f"{text} by {key}" if key else text), key


def inspect(path):
    """Return (descriptions, {key ids}) for one rpm."""
    with open(path, "rb") as fh:
        blob = fh.read()
    found, keys = [], set()
    for tag, raw in sorted(signature_header(blob).items()):
        if tag not in SIG_TAGS:
            continue
        for ptag, body in openpgp_packets(raw):
            if ptag == 2:                      # signature packet
                text, key = describe(body)
                found.append(f"{SIG_TAGS[tag]}={text}")
                if key:
                    keys.add(key)
    return found, keys


def main(argv):
    # --key <hex>: require the signature to be by that key. Without it any
    # signature counts. The backfill needs the distinction: after a key
    # rotation every package is still signed, by the retired key, and a
    # presence-only test would report nothing left to do.
    want = None
    if len(argv) >= 2 and argv[0] == "--key":
        want, argv = argv[1].lower()[-16:], argv[2:]
    if not argv:
        print(__doc__.strip().splitlines()[-1], file=sys.stderr)
        return 2
    unsigned = []
    for path in argv:
        try:
            found, keys = inspect(path)
        except (OSError, ValueError, IndexError, struct.error) as exc:
            print(f"UNREADABLE  {path}: {exc}")
            unsigned.append(path)
            continue
        if found and want and want not in keys:
            print(f"WRONG KEY   {path}  [{', '.join(found)}] — wanted {want}")
            unsigned.append(path)
        elif found:
            print(f"signed      {path}  [{', '.join(found)}]")
        else:
            print(f"UNSIGNED    {path}")
            unsigned.append(path)
    if unsigned:
        what = f"are not signed by {want}" if want else "carry no signature"
        print(f"\n{len(unsigned)} of {len(argv)} package(s) {what}", file=sys.stderr)
        return 1
    print(f"\nall {len(argv)} package(s) signed" + (f" by {want}" if want else ""))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
