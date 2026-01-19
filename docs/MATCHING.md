# Matching Algorithms

*How MKV packets are matched to source media for deduplication.*

[Back to Architecture Overview](../DESIGN.md)

## Overview

The matching process involves four stages:
1. **Source Indexer** - Build a hash index of the source media
2. **MKV Parser** - Extract codec data locations from the MKV
3. **Matcher** - Find MKV packets in the source using hash lookups
4. **Verification** - Confirm byte-perfect reconstruction

## Source Indexer

### Source Type Detection

The source type is detected by scanning the directory structure:
- **DVD**: Contains `*.iso` file(s)
- **Blu-ray**: Contains `BDMV/STREAM/*.m2ts` files

### Codec Detection

Different media use different video codecs:

| Media | Video Codecs | Container |
|-------|-------------|-----------|
| DVD | MPEG-2 | VOB (MPEG-PS) |
| Blu-ray | H.264/AVC, MPEG-2, VC-1, HEVC | M2TS (MPEG-TS) |

### Start Code Patterns

| Codec | Start Code Pattern | Notes |
|-------|-------------------|-------|
| MPEG-2 | `00 00 01 xx` | xx = B3 (seq), 00 (pic), 01-AF (slice) |
| H.264/AVC | `00 00 01 xx` or `00 00 00 01 xx` | xx = NAL unit type |
| HEVC | `00 00 01 xx` or `00 00 00 01 xx` | xx = NAL unit type |
| VC-1 | `00 00 01 xx` | Different structure |

Since most video codecs use `00 00 01` as a start code prefix, we scan for this pattern and index at each occurrence.

### Audio Sync Patterns

| Codec | Sync Pattern | Notes |
|-------|-------------|-------|
| AC3 (Dolby Digital) | `0B 77` | 2-byte sync word |
| DTS | `7F FE 80 01` | 4-byte sync word |
| TrueHD | `F8 72 6F BA` | 4-byte sync word |
| MPEG Audio (MP2/MP3) | `FF Fx` | 11-bit sync, x varies |
| LPCM/PCM | (none) | Raw samples, no framing |

## ES-Aware Indexing (DVD)

**Problem:** DVDs use MPEG Program Stream (MPEG-PS) containers which wrap Elementary Stream (ES) data in PES packets. When ripping to MKV, tools extract the raw ES data, stripping all container framing. If we index raw file offsets in the ISO, the container bytes create misalignments.

**Solution:** Parse the MPEG-PS structure and build an index based on **ES offsets** (continuous byte positions within the elementary stream) rather than raw file offsets.

```
MPEG-PS file structure:
┌─────────────┬──────────────────────┬─────────────┬──────────────────────┬───
│ Pack Header │ PES Packet (video)   │ Pack Header │ PES Packet (video)   │...
│  (14 bytes) │ [header][payload]    │  (14 bytes) │ [header][payload]    │
└─────────────┴──────────────────────┴─────────────┴──────────────────────┴───
                      │                                     │
                      ▼                                     ▼
Extracted video ES:   [payload bytes...............][payload bytes...........]
ES offset:            0                             1234
File offset:          47                            1314
```

The parser maintains a mapping of ES offsets to file offsets for each PES payload range.

## Video user_data Filtering

**Problem:** MKV tools (like MakeMKV) strip `user_data` sections (start code `00 00 01 B2`) from video streams. These contain closed captions and other auxiliary data.

**Solution:** When building the video ES index, create "filtered ranges" that exclude user_data sections:

```
Raw video ES:          [video][user_data][video][user_data][video]
Filtered video ES:     [video]...........[video]...........[video]
                       (user_data sections excluded)
```

**Result:** Video matching improved from ~60% to ~98.6% after user_data filtering.

## DVD Audio: Private Stream 1

DVD audio is carried in MPEG-PS Private Stream 1 (stream ID 0xBD). Each PES packet contains a 4-byte header before the audio data:

```
┌────────────┬─────────────┬──────────────────┬─────────────────────────────┐
│ Sub-stream │ Frame count │ First access ptr │ Audio data (AC3/DTS/LPCM)   │
│   ID (1)   │    (1)      │      (2)         │                             │
└────────────┴─────────────┴──────────────────┴─────────────────────────────┘

Sub-stream IDs:
  0x80-0x87: AC3 (Dolby Digital)
  0x88-0x8F: DTS
  0xA0-0xA7: LPCM
  0x20-0x3F: Subpictures
```

MKV tools strip this 4-byte header. We must do the same when building filtered audio ranges.

## Per-Sub-Stream Audio Filtering

**Problem:** Private Stream 1 carries multiple audio tracks interleaved:

```
PES packets in file order:
[0x80 audio][0x81 audio][0x80 audio][0x82 audio][0x80 audio][0x81 audio]...
```

When matching an MKV audio track (which contains only ONE language), we'd hit data from other tracks and matching would fail.

**Solution:** Create separate filtered ES ranges for each sub-stream ID. When matching MKV audio, match against only the specific sub-stream.

**Result:** Audio matching improved from ~25% to ~99%+ after per-sub-stream filtering.

## Index Data Structure

The source index contains:
- **Hash table**: Maps 64-bit xxhash → list of (file index, offset) locations
- **Source directory**: Path to the source media
- **Source type**: DVD or Blu-ray
- **File list**: Relative path, size, and checksum for each source file
- **Window size**: Number of bytes used for hashing (default: 64 bytes)

## MKV Parser

Parse the MKV file to identify codec data packet boundaries.

### MKV Structure (Matroska/WebM)

```
EBML Header
Segment
├── SeekHead (index)
├── Info (metadata)
├── Tracks (codec definitions)
├── Chapters
├── Clusters
│   ├── Timestamp
│   ├── SimpleBlock (video/audio packet)
│   ├── BlockGroup
│   │   ├── Block
│   │   └── BlockDuration
│   └── ...
└── Cues (seek index)
```

The actual codec data is inside `SimpleBlock` and `Block` elements. Each block contains:
- Track number (1 byte, variable)
- Timestamp offset (2 bytes)
- Flags (1 byte for SimpleBlock)
- Frame data (the codec bytes we want to match)

**Output:** List of (mkv_offset, length, hash) for each frame.

## Matcher Algorithm

```
For each MKV packet (mkv_offset, length, hash):
    1. Look up hash in SourceIndex.HashToLocations
    2. If found:
       a. For each candidate location:
          - Verify full match (compare all `length` bytes)
          - If match:
            - EXPAND match boundaries
            - Record expanded region as SOURCE reference
            - Mark MKV range as matched
            - Break on first match
    3. If not found or no verified match:
       - Mark as DELTA (unique data)
       - Append data to delta buffer
```

### Index Coverage

The index covers the ENTIRE MKV file:

```
MKV byte range 0-1000:        EBML header → DELTA
MKV byte range 1000-1050:     Cluster header → DELTA
MKV byte range 1050-9000:     Video frame → SOURCE file=0, offset=463000
MKV byte range 9000-9020:     Block header → DELTA
MKV byte range 9020-11000:    Audio packet → SOURCE file=0, offset=464500
```

## Boundary Expansion

After finding a match via hash lookup, expand in both directions to maximize deduplication:

```
Source (VOB):     [container][video data...............][container]
                            ^
                       start code (indexed)

MKV:              [ebml hdr][video data...............][block hdr]
                            ^
                       matches here

Without expansion: Only match from start code to next start code
With expansion:    Match extends to include adjacent matching bytes
```

**Algorithm:**
1. **Expand backward**: Compare bytes before match until mismatch
2. **Expand forward**: Compare bytes after match until mismatch
3. **Return**: Updated start offsets and expanded length

**Constraints:**
- Don't expand into already-matched regions
- Limit expansion distance (e.g., max 64KB each direction)

**Benefits:**
- Maximizes deduplication ratio
- Reduces delta size
- Captures partial matches at packet boundaries
- Helps with LPCM audio (no sync patterns)

## Verification

**Mandatory verification after deduplication.**

1. Create dedup file
2. Mount via FUSE (temporary, internal mount)
3. Open virtual MKV through FUSE
4. Open original MKV
5. Compare byte-by-byte (or in large chunks with hash comparison)
6. If mismatch: report error with offset, delete dedup file, exit with failure
7. If match: report success, optionally delete original (with --delete-original)

## Performance Results

With all filtering implemented (ES-aware, user_data, per-sub-stream audio):

| Metric | Before Filtering | After Filtering |
|--------|-----------------|-----------------|
| Video byte match rate | ~60% | ~98.6% |
| Audio byte match rate | ~25% | ~99%+ |
| Overall byte match rate | ~50% | **98.4%** |
| Delta size (3.4GB MKV) | ~1.7GB | ~56MB |
| Storage savings | ~50% | **97.8%** |

The remaining ~1.6% unmatched data consists of:
- MKV container headers (EBML, cluster headers, block headers)
- Subtitle data (not in video/audio ES)
- Minor stream differences

## Related Documentation

- [File Format](FILE_FORMAT.md) - How matched entries are stored
- [CLI Commands](CLI.md) - Running the matching process
