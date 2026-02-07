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

## ES-Aware Indexing (Blu-ray)

**Problem:** Blu-rays use MPEG Transport Stream (MPEG-TS) containers in M2TS files. M2TS files consist of fixed-size 192-byte packets: a 4-byte timecode prefix plus a 188-byte TS packet (which itself has a 4-byte header, leaving up to 184 bytes for payload). Each TS packet carries a small fragment of a PES packet, identified by a 13-bit PID. PES packets span multiple TS packets and contain the actual codec data. When ripping to MKV, tools extract the raw ES data, stripping all TS and PES framing. If we index raw file offsets in the M2TS file, the 8-byte headers (4-byte timecode + 4-byte TS header, interleaved every 192 bytes) cause misalignments — expansion fails at every packet boundary.

**Solution:** Parse the MPEG-TS structure (PAT → PMT → PES packets) and build an index based on continuous ES offsets, the same approach used for DVDs.

```
M2TS file structure (192-byte packets):
┌───────────┬──────────────────────────────────────────────────────────────┐
│ Timecode  │ TS Packet (188 bytes)                                        │
│ (4 bytes) │ [sync 0x47][PID][flags][adaptation?][PES payload ≤184 bytes] │
└───────────┴──────────────────────────────────────────────────────────────┘
             └─ 4-byte TS header ─┘

PES packets span multiple TS packets:
┌──────────────────────┐  ┌──────────────────────┐  ┌──────────────────────┐
│ TS pkt (PUSI=1)      │  │ TS pkt (continuation)│  │ TS pkt (continuation)│
│ [PES header][payload]│  │ [payload............]│  │ [payload............]│
└──────────────────────┘  └──────────────────────┘  └──────────────────────┘
         │                         │                         │
         ▼                         ▼                         ▼
ES:      [payload bytes....][payload bytes.........][payload bytes.........]
ES off:  0                  ~167                    ~351
```

### Key Differences from DVD (MPEG-PS)

| Feature | DVD (MPEG-PS) | Blu-ray (MPEG-TS) |
|---------|--------------|-------------------|
| Container | MPEG Program Stream | MPEG Transport Stream |
| Packet size | Variable | Fixed 192 bytes (M2TS) or 188 bytes (TS) |
| Stream ID | Stream ID byte in PES header | PID (13-bit) per TS packet |
| Audio framing | Private Stream 1 with 4-byte sub-headers | Separate PID per audio track, no sub-headers |
| Audio sub-streams | Sub-stream IDs (0x80-0x87 = AC3, etc.) | PIDs mapped to sequential byte IDs (0, 1, 2...) |
| user_data filtering | Required (MPEG-2 video) | Not needed for H.264/H.265 (only for MPEG-2) |

### PAT/PMT Parsing

The parser identifies streams via MPEG-TS Program Specific Information:
1. **PAT** (PID 0): Maps program numbers to PMT PIDs
2. **PMT**: Lists elementary stream PIDs and their stream types (e.g., 0x1B = H.264, 0x81 = AC3)

### Audio PID Mapping

Unlike DVDs where audio is multiplexed in Private Stream 1 with sub-stream IDs, Blu-ray audio tracks have individual PIDs. The parser assigns sequential byte sub-stream IDs (0, 1, 2, ...) to audio PIDs in PMT order, maintaining compatibility with the `Location.AudioSubStreamID` field used throughout the codebase.

## Blu-ray TrueHD+AC3 Stream Splitting

**Problem:** On Blu-ray discs, TrueHD audio streams (PMT stream type 0x83) embed an AC3 compatibility core interleaved in the same PID. The raw PES payload data looks like:

```
PES payload: [AC3 frame][TrueHD frame(s)][AC3 frame][TrueHD frame(s)]...
```

MakeMKV (and other ripping tools) split these into separate MKV tracks: one for TrueHD-only data, one for the AC3 core. If we index them as a single combined sub-stream, the interleaved AC3+TrueHD bytes don't match either MKV track.

**Solution:** After parsing all PES payloads, detect combined TrueHD+AC3 streams by scanning the first 16KB of ES data for both AC3 sync words (`0B 77`) and TrueHD major sync words (`F8 72 6F BA`). When both are found, split the ranges by walking through the payload and parsing AC3 frame headers to determine frame boundaries:

1. **AC3 frame detection**: When `0B 77` is found, read byte 4 for `fscod` (2-bit sample rate code) and `frmsizecod` (6-bit frame size code). The frame size is deterministic from these values (ATSC A/52 Table 5.18).
2. **Range assignment**: AC3 frame bytes go to the new AC3 sub-stream; all other bytes go to the TrueHD sub-stream.
3. **Cross-range tracking**: AC3 frames may span TS payload chunks. The `ac3Remaining` counter tracks bytes still belonging to the current AC3 frame across range boundaries.
4. **Range merging**: After splitting, merge adjacent ranges that are contiguous in both file offset and ES offset to reduce range count.

The original sub-stream ID keeps the TrueHD-only ranges; a new sub-stream ID is assigned for the AC3 core.

**Impact:** On a 40GB Blu-ray (MI7), audio delta dropped from 1.85 GB (42% of total delta) to near-zero after splitting. The matcher can now find TrueHD data in the TrueHD MKV track and AC3 data in the AC3 MKV track.

## Video user_data Filtering

**Problem:** MKV remuxing tools typically strip `user_data` sections (start code `00 00 01 B2`) from video streams. These contain closed captions and other auxiliary data.

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

After finding a match via hash lookup, expand in both directions to maximize deduplication. For both DVD and Blu-ray sources with ES-aware indexing, expansion reads continuous ES bytes through the ES reader interface, transparently skipping container framing (MPEG-PS pack headers or MPEG-TS packet headers).

```
Source (VOB/M2TS): [container][video data...............][container]
                              ^
                         start code (indexed)

MKV:               [ebml hdr][video data...............][block hdr]
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

*Results from Big Buck Bunny test data (see [testdata/README.md](../testdata/README.md) and #27).*

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
