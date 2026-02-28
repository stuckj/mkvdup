# MKV-ISO Deduplication System Design

## Overview

This system deduplicates MKV files ripped from DVDs or Blu-rays against their source media. Since the underlying codec data (video frames, audio packets) is identical between the MKV and source (just at different offsets with different container framing), we can store only the unique MKV data plus an index mapping MKV offsets to source offsets.

**Goal:** Store a 3.4GB MKV as ~50-55MB by referencing the source media for shared codec data.

## Documentation

| Document | Description |
|----------|-------------|
| [docs/MATCHING.md](docs/MATCHING.md) | Source indexing, MKV parsing, packet matching, verification |
| [docs/FILE_FORMAT.md](docs/FILE_FORMAT.md) | Binary specification for .mkvdup files |
| [docs/FUSE.md](docs/FUSE.md) | FUSE filesystem configuration and operation |
| [docs/CLI.md](docs/CLI.md) | Command-line interface reference |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Development guidelines and testing |

## Supported Source Media

| Type | Structure | Container Format |
|------|-----------|------------------|
| DVD | Single `.iso` file (ISO 9660) | VOB (MPEG-PS) |
| Blu-ray | Directory with BDMV structure | M2TS (MPEG-TS) |
| Blu-ray ISO | Single `.iso` file (UDF) | M2TS (MPEG-TS) |

All source types are referenced via a **source directory** which contains either:
- A single ISO file — DVD (ISO 9660) or Blu-ray (UDF)
- A Blu-ray backup directory structure (BDMV/STREAM/*.m2ts)

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Deduplication Phase                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────┐     ┌─────────────┐     ┌──────────────────────┐   │
│  │   MKV   │────▶│  MKV Parser │────▶│  Packet List         │   │
│  │  File   │     │             │     │  (offset, len, hash) │   │
│  └─────────┘     └─────────────┘     └──────────┬───────────┘   │
│                                                  │               │
│  ┌─────────┐     ┌─────────────┐     ┌──────────▼───────────┐   │
│  │ Source  │────▶│   Source    │────▶│  Hash Table          │   │
│  │   Dir   │     │   Indexer   │     │  hash -> [file,off]  │   │
│  │(ISO/BD) │     │             │     │                      │   │
│  └─────────┘     └─────────────┘     └──────────┬───────────┘   │
│                                                  │               │
│                                      ┌──────────▼───────────┐   │
│                                      │     Matcher          │   │
│                                      │  Find MKV packets    │   │
│                                      │  in source           │   │
│                                      └──────────┬───────────┘   │
│                                                  │               │
│                                                  ▼               │
│                                      ┌──────────────────────┐   │
│                                      │  Dedup File          │   │
│                                      │  (.mkvdup)           │   │
│                                      │  [index + delta]     │   │
│                                      └──────────────────────┘   │
│                                                  │               │
│                                                  ▼               │
│                                      ┌──────────────────────┐   │
│                                      │  Verification        │   │
│                                      │  Mount & compare     │   │
│                                      │  to original MKV     │   │
│                                      └──────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                      Reconstruction Phase                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  FUSE Config File (YAML/JSON)                            │   │
│  │  ┌────────────────────────────────────────────────────┐  │   │
│  │  │ virtual_files:                                     │  │   │
│  │  │   - name: "Video1.mkv"                              │  │   │
│  │  │     dedup_file: "/data/dedup/video1.mkvdup"        │  │   │
│  │  │     source_dir: "/data/media/Video1_DVD"           │  │   │
│  │  └────────────────────────────────────────────────────┘  │   │
│  └──────────────────────────────────────────────────────────┘   │
│                              │                                   │
│                              ▼                                   │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                      FUSE Daemon                          │   │
│  │                                                           │   │
│  │   read("/mnt/dedup/Video1.mkv", offset, len)              │   │
│  │                         │                                 │   │
│  │                         ▼                                 │   │
│  │   ┌─────────────────────────────────────────────────┐    │   │
│  │   │  Lookup in dedup file index                     │    │   │
│  │   │  Stitch from: source_dir files + embedded delta │    │   │
│  │   └─────────────────────────────────────────────────┘    │   │
│  └──────────────────────────────────────────────────────────┘   │
│                              │                                   │
│                              ▼                                   │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  /mnt/dedup/                                              │   │
│  │  ├── Video1.mkv        (virtual, 3.4GB apparent)         │   │
│  │  └── Video2.mkv        (virtual, 25GB apparent)          │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### Source Indexer Pipeline

```
┌─────────────────────────────────────────────────────────────────────┐
│                      Source Indexer Pipeline                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  Source Dir ──▶ Detect Type                                         │
│                    │                                                │
│        ┌───────────┼───────────┐                                    │
│        ▼           ▼           ▼                                    │
│   ┌─────────┐ ┌─────────┐ ┌─────────┐                              │
│   │   DVD   │ │ Blu-ray │ │ BD ISO  │                               │
│   │  (ISO)  │ │  (dir)  │ │ (UDF)   │                               │
│   └────┬────┘ └────┬────┘ └────┬────┘                               │
│        │           │           │                                    │
│        ▼           ▼           ▼                                    │
│   ┌─────────┐ ┌──────────────────┐                                  │
│   │ MPEG-PS │ │ MPEG-TS Parser   │                                  │
│   │  Parse  │ │                  │                                  │
│   │ (VOB)   │ │  PAT/PMT ─▶ PID │                                  │
│   └────┬────┘ │  Filtering       │                                  │
│        │      └────────┬─────────┘                                  │
│        │               │                                            │
│        │      ┌────────▼─────────┐                                  │
│        │      │ Audio Processing │                                  │
│        │      │ ┌──────────────┐ │                                  │
│        │      │ │TrueHD+AC3   │ │                                  │
│        │      │ │ Splitter     │ │                                  │
│        │      │ └──────────────┘ │                                  │
│        │      │ ┌──────────────┐ │                                  │
│        │      │ │DTS-HD Core   │ │                                  │
│        │      │ │ Extractor    │ │                                  │
│        │      │ └──────────────┘ │                                  │
│        │      └────────┬─────────┘                                  │
│        │               │                                            │
│        ▼               ▼                                            │
│   ┌─────────────────────────┐                                       │
│   │   ES Data Indexer       │                                       │
│   │                         │                                       │
│   │ For each stream:        │                                       │
│   │  Find sync points       │                                       │
│   │  Hash 64-byte windows   │                                       │
│   │  Build hash ─▶ location │                                       │
│   └────────────┬────────────┘                                       │
│                │                                                    │
│                ▼                                                    │
│   ┌─────────────────────────┐                                       │
│   │  Index (hash table)     │                                       │
│   │  hash ─▶ [file, offset] │                                       │
│   └─────────────────────────┘                                       │
└─────────────────────────────────────────────────────────────────────┘
```

### Matcher Pipeline

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Matcher Pipeline                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  MKV ──▶ Parse tracks ──▶ For each track:                          │
│                                                                     │
│  ┌───────────────────────────────────────────────┐                  │
│  │ Track Processing                              │                  │
│  │                                               │                  │
│  │  Detect NAL framing (AVCC vs Annex B)         │                  │
│  │             │                                 │                  │
│  │             ▼                                 │                  │
│  │  Find sync points in MKV track data           │                  │
│  │             │                                 │                  │
│  │             ▼                                 │                  │
│  │  Hash windows ──▶ Lookup in source index      │                  │
│  │             │                                 │                  │
│  │             ▼                                 │                  │
│  │  Expand matches byte-by-byte                  │                  │
│  │  (bidirectional, respects NAL boundaries)     │                  │
│  │             │                                 │                  │
│  │             ▼                                 │                  │
│  │  Coverage bitmap (4KB granularity)            │                  │
│  │  for parallel duplicate detection             │                  │
│  └───────────────────────────────────────────────┘                  │
│                    │                                                │
│                    ▼                                                │
│  ┌───────────────────────────────────────────────┐                  │
│  │ Post-processing                               │                  │
│  │                                               │                  │
│  │  Merge overlapping regions                    │                  │
│  │             │                                 │                  │
│  │             ▼                                 │                  │
│  │  Build entries (MKV offset ─▶ source offset)  │                  │
│  │             │                                 │                  │
│  │             ▼                                 │                  │
│  │  Write delta (unmatched MKV bytes)            │                  │
│  └───────────────────────────────────────────────┘                  │
│                    │                                                │
│                    ▼                                                │
│  ┌───────────────────────────────────────────────┐                  │
│  │ Result                                        │                  │
│  │  Entries[] + Delta file + Range maps           │                  │
│  └───────────────────────────────────────────────┘                  │
└─────────────────────────────────────────────────────────────────────┘
```

## Zero-Copy Memory Mapping

All file access uses true zero-copy memory mapping via `unix.Mmap` from `golang.org/x/sys/unix`. The `internal/mmap` package provides:

- **No data copying**: Slices point directly into kernel page cache
- **Efficient memory usage**: Pages are demand-loaded and can be evicted under memory pressure
- **Fast random access**: No syscall overhead for reads within mapped region

**ISO detection optimization**: DVD/Blu-ray detection reads only ~18KB (primary volume descriptor + root directory) instead of loading the entire ISO.

## Technical Decisions

### Hash Function
- **xxhash** (extremely fast, good distribution)
- 64-bit hash sufficient (collision probability negligible)

### Window Size
- 64 bytes default (sufficient uniqueness for codec sync points)

### Index Granularity
- Per-sync-point (millions of entries) for maximum dedup
- Index stored in dedup file, not in memory during normal operation

### Dedup Reader Direct Mmap Access
- Mount time: ~0 seconds (header only)
- Entry access: Direct from mmap (no parsing into `[]Entry`)
- Memory usage: Near-zero (kernel-managed mmap pages)
- Sequential reads: O(1) via last-entry cache
- Random seeks: O(log N) binary search + single entry parse

### Entry Access Optimization
- Entries accessed directly from mmap'd file (no allocation)
- `RawEntry` packed struct (28 bytes) matches on-disk format exactly
- Uses byte arrays with explicit little-endian decoding for portability
- Single-entry cache eliminates repeated parsing for sequential access

### File Format Versions
- **v1 (deprecated)**: Stored ES (elementary stream) offsets for DVD sources
- **v2 (deprecated)**: Used uint8 for Source field (max 256 files)
- **v3 (current, DVD)**: Uses uint16 for Source field (max 65535 files), raw file offsets. Entries that span multiple PES payload ranges are split during create.
- **v4 (current, Blu-ray)**: Adds embedded range map section mapping ES offsets to raw M2TS file offsets. Uses compressed delta+varint+RLE encoding for >1000:1 compression of the highly regular M2TS packet structure. Footer extended to 32 bytes with range map checksum. See [FILE_FORMAT.md](docs/FILE_FORMAT.md#range-map-format-version-4) for details.

## Performance Results

*Results from Big Buck Bunny test data (see [testdata/README.md](testdata/README.md) and #27 for reproducible test setup).*

| Metric | Value |
|--------|-------|
| Video byte match rate | ~98.6% |
| Audio byte match rate | ~99%+ |
| Overall byte match rate | 98.4% |
| Storage savings | 97.8% |

For a 3.4GB MKV:
- Delta size: ~56MB (container overhead only)
- Index overhead: ~27-54MB
- Total dedup file: ~50-55MB

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| MKV parser bugs | Use well-tested library, extensive test suite |
| Hash collisions | Verify full match after hash lookup |
| Source files change | Store checksums, verify before use |
| FUSE performance | Memory-map files, optimize read path |
| Large index | Store in file, memory-map for access |
| Source dir moved | Config uses paths, user must update |
