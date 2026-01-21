# Dedup File Format (.mkvdup)

*Binary specification for the unified dedup file format.*

[Back to Architecture Overview](../DESIGN.md)

## Overview

The `.mkvdup` file is a single binary file containing both the index and delta data needed to reconstruct an MKV file from its source media.

## Version History

| Version | Description |
|---------|-------------|
| 2 (current) | Raw file offsets stored directly. UsesESOffsets is always 0. |
| 1 (deprecated) | Used ES (elementary stream) offsets for DVD sources. No longer supported; files must be recreated. |

## Design Principles

1. **No repeated strings**: Filenames stored once, referenced by index
2. **Binary encoding**: All numeric values as binary (little-endian), not text
3. **Relative paths**: Source file paths relative to `source_dir` from FUSE config
4. **Compact index entries**: Use smallest types that fit the data

## File Structure

```
┌────────────────────────────────────────────────────────┐
│  Header (fixed size: 60 bytes)                         │
├────────────────────────────────────────────────────────┤
│  Magic: "MKVDUP01" (8 bytes)                           │
│  Version: uint32 (4 bytes)                             │
│  Flags: uint32 (4 bytes)  [reserved for future use]    │
│  OriginalSize: int64 (8 bytes)                         │
│  OriginalChecksum: uint64 (8 bytes)                    │
│  SourceType: uint8 (1 byte)  [0=DVD, 1=Blu-ray]        │
│  UsesESOffsets: uint8 (1 byte)  [always 0 in v2]       │
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
│                                                        │
│  Note: Path is relative to source_dir in FUSE config   │
│  Example: "VIDEO_TS/VTS_09_1.VOB" not full path        │
├────────────────────────────────────────────────────────┤
│  Index Entries Section (fixed 27 bytes per entry)      │
├────────────────────────────────────────────────────────┤
│  For each entry (sorted by MkvOffset, contiguous):     │
│    MkvOffset: int64 (8 bytes)                          │
│    Length: int64 (8 bytes)                             │
│    Source: uint8 (1 byte) [0=DELTA, 1+=file idx+1]     │
│    SourceOffset: int64 (8 bytes)                       │
│    IsVideo: uint8 (1 byte)  [for ES-based sources]     │
│    AudioSubStreamID: uint8 (1 byte)  [for ES audio]    │
│                                                        │
│  Entry size: 27 bytes                                  │
│  1M entries = 27 MB index overhead                     │
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

## Source Reference Encoding

For index entries:
- `Source = 0`: Data is in delta section at `SourceOffset`
- `Source = 1`: Data is in source file 0 at `SourceOffset`
- `Source = 2`: Data is in source file 1 at `SourceOffset`
- etc.

In version 2, `SourceOffset` is always a raw byte offset into the source file.
Entries that would span multiple non-contiguous regions in the source file
(due to container header removal) are split into multiple entries during creation.

## Storage Efficiency

**Current entry size: 27 bytes**
- MkvOffset: 8 bytes
- Length: 8 bytes
- Source: 1 byte
- SourceOffset: 8 bytes
- IsVideo: 1 byte
- AudioSubStreamID: 1 byte

**Estimated index size for typical video:**
- ~1-2 million packets → 27-54 MB index
- Acceptable given ~3 GB space savings

**Future optimization (if needed): Varint encoding**
- Use variable-length integer encoding (protobuf-style)
- Small offsets/lengths use fewer bytes
- Could reduce index by 40-60%
- Adds complexity, defer unless index size is problematic

**Future optimization (if needed): Delta encoding**
- Store MkvOffset as delta from previous entry
- Most deltas are small (packet sizes)
- Combined with varint, significant savings
- Complicates random access

## Delta Contents

The delta section contains ONLY data that couldn't be matched to the source:
- **MKV container overhead** (EBML headers, cluster metadata, block headers)
- **Closed caption user data** (if present in video stream)
- **Any unmatched codec data** (rare, if matching works correctly)

Audio and video codec data should NOT end up in delta if the matching algorithm works correctly. The delta should be almost entirely container overhead.

**Expected delta size (tested with a DVD ISO):**

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
