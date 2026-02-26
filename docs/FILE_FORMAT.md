# Dedup File Format (.mkvdup)

*Binary specification for the unified dedup file format.*

[Back to Architecture Overview](../DESIGN.md)

## Overview

The `.mkvdup` file is a single binary file containing both the index and delta data needed to reconstruct an MKV file from its source media.

## Version History

| Version | Description |
|---------|-------------|
| 8 (current) | V6 + per-source-file Used byte. On-disk layout otherwise identical to V6. |
| 7 (current) | V5 + per-source-file Used byte. On-disk layout otherwise identical to V5. |
| 6 | V4 + embedded creator version string after the header. On-disk layout otherwise identical to V4. |
| 5 | V3 + embedded creator version string after the header. On-disk layout otherwise identical to V3. |
| 4 | Adds embedded range map section for Blu-ray M2TS sources. Index entries use ES offsets; the range map translates ES offsets to raw file offsets at read time. Footer extended to 32 bytes with range map checksum. |
| 3 | Source field expanded to uint16 (supports >256 source files). Entry size: 28 bytes. Index entries use raw file offsets directly. |
| 2 (deprecated) | Raw file offsets stored directly. Source field was uint8 (max 256 files). No longer supported; files must be recreated. |
| 1 (deprecated) | Used ES (elementary stream) offsets for DVD sources. No longer supported; files must be recreated. |

The writer produces V7 (DVD) or V8 (Blu-ray) files. V3-V6 files are supported for reading. V5-V8 add a creator version string (uint16 length + UTF-8 string) immediately after the 60-byte header, shifting all subsequent sections by `2 + len(version_string)` bytes. V7/V8 additionally add a Used byte (uint8) per source file record, indicating whether the file is referenced by any index entry.

## Design Principles

1. **No repeated strings**: Filenames stored once, referenced by index
2. **Binary encoding**: All numeric values as binary (little-endian), not text
3. **Relative paths**: Source file paths relative to `source_dir` from FUSE config
4. **Compact index entries**: Use smallest types that fit the data

## File Structure (Version 3 — DVD)

```
┌────────────────────────────────────────────────────────┐
│  Header (fixed size: 60 bytes)                         │
├────────────────────────────────────────────────────────┤
│  Magic: "MKVDUP01" (8 bytes)                           │
│  Version: uint32 (4 bytes) = 3                         │
│  Flags: uint32 (4 bytes)  [reserved for future use]    │
│  OriginalSize: int64 (8 bytes)                         │
│  OriginalChecksum: uint64 (8 bytes)                    │
│  SourceType: uint8 (1 byte)  [0=DVD, 1=Blu-ray]       │
│  UsesESOffsets: uint8 (1 byte)  [always 0 in v3]       │
│  SourceFileCount: uint16 (2 bytes)                     │
│  EntryCount: uint64 (8 bytes)                          │
│  DeltaOffset: int64 (8 bytes)                          │
│  DeltaSize: int64 (8 bytes)                            │
├────────────────────────────────────────────────────────┤
│  Source Files Section (variable size)                  │
├────────────────────────────────────────────────────────┤
│  For each source file:                                 │
│    PathLen: uint16 (2 bytes)                           │
│    Path: []byte (PathLen bytes, UTF-8, relative)       │
│    FileSize: int64 (8 bytes)                           │
│    FileChecksum: uint64 (8 bytes)                      │
│    Used: uint8 (1 byte, V7/V8 only; 1=used, 0=unused) │
│                                                        │
│  Note: Path is relative to source_dir in FUSE config   │
│  Example: "VIDEO_TS/VTS_09_1.VOB"                      │
├────────────────────────────────────────────────────────┤
│  Index Entries Section (fixed 28 bytes per entry)      │
├────────────────────────────────────────────────────────┤
│  For each entry (sorted by MkvOffset, contiguous):     │
│    MkvOffset: int64 (8 bytes)                          │
│    Length: int64 (8 bytes)                              │
│    Source: uint16 (2 bytes) [0=DELTA, 1+=file idx+1]   │
│    SourceOffset: int64 (8 bytes) [raw file offset]     │
│    IsVideo: uint8 (1 byte)                             │
│    AudioSubStreamID: uint8 (1 byte) [audio or sub]     │
│                                                        │
│  Entry size: 28 bytes                                  │
│  1M entries = 28 MB index overhead                     │
├────────────────────────────────────────────────────────┤
│  Delta Section (variable size)                         │
├────────────────────────────────────────────────────────┤
│  [raw unique MKV bytes concatenated]                   │
│  No framing - offsets from index entries               │
├────────────────────────────────────────────────────────┤
│  Footer (24 bytes)                                     │
├────────────────────────────────────────────────────────┤
│  IndexChecksum: uint64 (8 bytes, xxhash of entries)    │
│  DeltaChecksum: uint64 (8 bytes, xxhash of delta)      │
│  Magic: "MKVDUP01" (8 bytes, for reverse scanning)     │
└────────────────────────────────────────────────────────┘
```

## File Structure (Version 4 — Blu-ray)

Version 4 adds a **range map section** between the delta section and footer. This section maps ES (elementary stream) offsets to raw file offsets, allowing the reader to reconstruct data from M2TS source files at read time.

```
┌────────────────────────────────────────────────────────┐
│  Header (fixed size: 60 bytes)                         │
├────────────────────────────────────────────────────────┤
│  (same fields as V3)                                   │
│  Version: uint32 (4 bytes) = 4                         │
│  SourceType: uint8 = 1 (Blu-ray)                       │
│  UsesESOffsets: uint8 = 1                              │
├────────────────────────────────────────────────────────┤
│  Source Files Section (variable size)                  │
├────────────────────────────────────────────────────────┤
│  (same format as V3; V8 includes Used byte per file)   │
│  Example: "BDMV/STREAM/00705.m2ts"                     │
├────────────────────────────────────────────────────────┤
│  Index Entries Section (fixed 28 bytes per entry)      │
├────────────────────────────────────────────────────────┤
│  (same format as V3, but SourceOffset = ES offset)     │
│  For source entries: SourceOffset is a continuous       │
│  byte position within the elementary stream, not a     │
│  raw file offset. The range map section provides the   │
│  mapping from ES offsets to raw M2TS file offsets.     │
├────────────────────────────────────────────────────────┤
│  Delta Section (variable size)                         │
├────────────────────────────────────────────────────────┤
│  (same format as V3)                                   │
├────────────────────────────────────────────────────────┤
│  Range Map Section (variable size)                     │
├────────────────────────────────────────────────────────┤
│  (see Range Map Format below)                          │
├────────────────────────────────────────────────────────┤
│  Footer (32 bytes — 8 bytes larger than V3)            │
├────────────────────────────────────────────────────────┤
│  IndexChecksum: uint64 (8 bytes, xxhash of entries)    │
│  DeltaChecksum: uint64 (8 bytes, xxhash of delta)      │
│  RangeMapChecksum: uint64 (8 bytes, xxhash of ranges)  │
│  Magic: "MKVDUP01" (8 bytes, for reverse scanning)     │
└────────────────────────────────────────────────────────┘
```

## Source Reference Encoding

For index entries:
- `Source = 0`: Data is in delta section at `SourceOffset`
- `Source = 1`: Data is in source file 0 at `SourceOffset`
- `Source = 2`: Data is in source file 1 at `SourceOffset`
- etc.

In version 3, `SourceOffset` is always a raw byte offset into the source file.
Entries that would span multiple non-contiguous regions in the source file
(due to container header removal) are split into multiple entries during creation.

In version 4, `SourceOffset` is an ES (elementary stream) offset — a continuous
byte position within the decoded stream. The range map section provides the
mapping from ES offsets to raw M2TS file offsets, allowing the reader to
extract payloads from the correct positions in the source file.

## Range Map Format (Version 4)

The range map section encodes the mapping from ES offsets to raw file offsets
for each stream (video and audio) of each source file. It uses compressed
delta+varint+RLE encoding to achieve high compression ratios on the highly
regular M2TS packet structure.

Only streams (video, audio sub-streams) that are actually referenced by
matched index entries are included. For multi-region Blu-ray ISOs that may
contain 100+ M2TS files, this avoids storing range maps for unrelated
regions and keeps the section compact.

### Section Layout

```
┌────────────────────────────────────────────────────────┐
│  Magic: "RNGEMAPX" (8 bytes)                           │
│  SourceCount: uint16 (2 bytes)                         │
├────────────────────────────────────────────────────────┤
│  For each source file:                                 │
│    FileIndex: uint16 (2 bytes)                         │
│    StreamCount: uint8 (1 byte)                         │
│                                                        │
│    For each stream:                                    │
│      Stream Header (8 bytes):                          │
│        FileIndex: uint16                               │
│        StreamType: uint8 (0=video, 1=audio)            │
│        SubStreamID: uint8                              │
│        EntryCount: uint32                              │
│                                                        │
│      Compression Parameters (8 bytes):                 │
│        DefaultGap: uint16                              │
│        DefaultSize: uint16                             │
│        CompressedDataSize: uint32                      │
│                                                        │
│      Compressed Range Data (CompressedDataSize bytes)  │
└────────────────────────────────────────────────────────┘
```

### Compressed Range Encoding

#### Background: M2TS Packet Structure

An M2TS file consists of 192-byte transport stream packets:
```
┌─────────────────────────────────────────────────────────────┐
│ TS Packet (192 bytes)                                        │
├──────────────┬──────────────────────────────────────────────┤
│ Header (4B)  │ Adaptation + Payload (188 bytes max)         │
│ + timecode   │ └── PES payload portion (≤184 bytes)         │
│ (4B)         │                                               │
└──────────────┴──────────────────────────────────────────────┘
    offset 0       offset 8                              offset 192
```

For a given stream (video or audio), the range map records where each PES
payload starts in the file and how long it is. Most payloads are exactly 184
bytes, spaced 192 bytes apart (one per packet). This regularity enables
extreme compression.

#### What We Encode

Each range map entry is a (file_offset, payload_size) pair:
```
Entry 0:  file_offset=1000,  size=184   ← first packet's payload
Entry 1:  file_offset=1192,  size=184   ← +192 bytes later (gap=8, then 184 payload)
Entry 2:  file_offset=1384,  size=184   ← +192 bytes later
Entry 3:  file_offset=1576,  size=184   ← +192 bytes later
...
Entry N:  file_offset=X,     size=120   ← partial payload at end of PES
```

The "gap" between entries is the space from the end of one payload to the
start of the next (typically 8 bytes: 4-byte TS header + 4-byte timecode).

#### Compression Scheme

Default values (determined by sampling):
- `defaultGap`: Most common gap between entries (typically 8)
- `defaultSize`: Most common payload size (typically 184)

Encoding rules:
1. **First entry**: Absolute file offset (uvarint) + size (uvarint)
2. **RLE run**: When consecutive entries all have the default gap and size,
   encode as: `0x00` + run_count (uvarint)
3. **Explicit entry**: When gap or size differs from default, encode as:
   `zigzag(gap_delta) + 1` (uvarint) + size (uvarint)

The zigzag encoding converts signed deltas to unsigned varints (0→0, -1→1,
1→2, -2→3, 2→4, etc.). Adding 1 ensures the value is never 0x00 (reserved
for RLE marker).

#### Concrete Example

Given entries at offsets 1000, 1192, 1384, 1576, 1768 (all size=184):
```
Encoded bytes:
  [uvarint: 1000]    ← first entry file offset
  [uvarint: 184]     ← first entry size
  [0x00]             ← RLE marker
  [uvarint: 4]       ← run of 4 more entries with default gap/size
```

This encodes 5 entries in ~5 bytes instead of 80 bytes (5 × 16 bytes raw).

For a 35 GB M2TS file with ~190 million payload entries, nearly all follow
the regular pattern. The compressed range data is typically 10-20 KB—a
compression ratio exceeding 1000:1.

### Read-Time Range Map Usage

At read time, the range map is decoded into a `StreamRangeMap` with a coarse
in-memory index (one entry per 1024 compressed entries) for fast binary search.
Sequential reads use a cached cursor for O(1) access. RLE runs support
arithmetic skip for O(1) seeking within a run.

For each FUSE read:
1. Look up the index entry to get the ES offset and stream type
2. Seek to the correct position in the range map (cached cursor or binary search)
3. Batch-copy payloads from the mmap'd source file using stride arithmetic

## Storage Efficiency

**Current entry size: 28 bytes**
- MkvOffset: 8 bytes
- Length: 8 bytes
- Source: 2 bytes (uint16, supports up to 65535 source files)
- SourceOffset: 8 bytes
- IsVideo: 1 byte
- AudioSubStreamID: 1 byte (also used for subtitle sub-streams)

**Estimated index size for typical video:**
- DVD: ~1-2 million packets → 28-56 MB index
- Blu-ray: ~1-2 million packets → 28-56 MB index (similar to DVD because
  entries that span multiple PES payloads are merged during creation)

**Range map overhead (V4 only):**
- Video range maps: typically <100 KB due to regular M2TS packet structure
  (192-byte stride compresses ~1000:1 via RLE)
- Audio range maps: can be tens of MB due to irregular payload sizes
  (e.g., DTS core frames extracted from DTS-HD streams have variable sizes
  that break RLE compression)
- Only streams referenced by matched entries are included, so unrelated
  M2TS regions in multi-region Blu-ray ISOs are excluded

## Delta Contents

The delta section contains ONLY data that couldn't be matched to the source:
- **MKV container overhead** (EBML headers, cluster metadata, block headers)
- **Closed caption user data** (if present in video stream)
- **Any unmatched codec data** (rare, if matching works correctly)

Audio and video codec data should NOT end up in delta if the matching algorithm works correctly. The delta should be almost entirely container overhead.

**Expected delta size:**

For a typical video with ~8,000-12,000 clusters and ~200,000 blocks:
- Cluster headers: ~120-180 KB
- Block headers: ~1 MB
- EBML header: ~5 KB
- **Total delta: ~1.2-1.5 MB** (if matching is perfect)

If delta is significantly larger (e.g., >10 MB), this indicates unmatched codec data, which suggests a problem with the matching algorithm or wrong source files.

**Note:** Delta compression was considered but rejected. Testing showed only 2-3:1 compression ratio on container headers, saving ~500 KB on a typical video. The complexity (decompression on reads, memory buffering) is not worth such minimal savings.

## Related Documentation

- [Matching Algorithms](MATCHING.md) - How packets are matched to create entries
- [CLI Commands](CLI.md) - Creating and inspecting dedup files
