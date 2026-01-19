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
| DVD | Single `.iso` file | VOB (MPEG-PS) |
| Blu-ray | Directory with BDMV structure | M2TS (MPEG-TS) |

Both source types are referenced via a **source directory** which contains either:
- A single ISO file (DVD)
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

### Dedup Reader Lazy Loading
- Mount time: ~0 seconds (header only)
- First read: parses entries via `sync.Once`
- Subsequent reads: instant

## Performance Results

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
