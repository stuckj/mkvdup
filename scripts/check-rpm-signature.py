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

Verifying the signature is *correct* is deliberately out of scope — that needs
the public key and rpm's own header canonicalisation, and it is what the distro
matrix in RELEASING.md covers.

Usage: check-rpm-signature.py <package.rpm> [<package.rpm> ...]
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
               22: "EdDSA", 25: "Ed25519"}
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


def describe(body):
    """Describe a v4 signature packet body as 'EdDSA/SHA256'."""
    if len(body) < 4:
        return "malformed"
    version = body[0]
    if version != 4:
        return f"v{version} signature"
    pub, hsh = body[2], body[3]
    return f"{PUBKEY_ALGO.get(pub, pub)}/{HASH_ALGO.get(hsh, hsh)}"


def inspect(path):
    """Return (signed, [description]) for one rpm."""
    with open(path, "rb") as fh:
        blob = fh.read()
    found = []
    for tag, raw in sorted(signature_header(blob).items()):
        if tag not in SIG_TAGS:
            continue
        for ptag, body in openpgp_packets(raw):
            if ptag == 2:                      # signature packet
                found.append(f"{SIG_TAGS[tag]}={describe(body)}")
    return bool(found), found


def main(argv):
    if not argv:
        print(__doc__.strip().splitlines()[-1], file=sys.stderr)
        return 2
    unsigned = []
    for path in argv:
        try:
            signed, found = inspect(path)
        except (OSError, ValueError, struct.error) as exc:
            print(f"UNREADABLE  {path}: {exc}")
            unsigned.append(path)
            continue
        if signed:
            print(f"signed      {path}  [{', '.join(found)}]")
        else:
            print(f"UNSIGNED    {path}")
            unsigned.append(path)
    if unsigned:
        print(f"\n{len(unsigned)} of {len(argv)} package(s) carry no signature",
              file=sys.stderr)
        return 1
    print(f"\nall {len(argv)} package(s) signed")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
