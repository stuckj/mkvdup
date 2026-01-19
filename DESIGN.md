# MKV-ISO Deduplication System Design

*This is a living design document. Update it as the implementation evolves.*

## Overview

This system deduplicates MKV files ripped from DVDs or Blu-rays against their source media. Since the underlying codec data (video frames, audio packets) is identical between the MKV and source (just at different offsets with different container framing), we can store only the unique MKV data plus an index mapping MKV offsets to source offsets.

**Goal:** Store a 3.4GB MKV as ~50-55MB by referencing the source media for shared codec data.

## Implementation Status

### Completed Features

| Feature | Status | Notes |
|---------|--------|-------|
| **Source Indexer** | ✅ Complete | DVD (MPEG-PS), ES-aware indexing |
| **MKV Parser** | ✅ Complete | EBML parser, track detection |
| **Matcher** | ✅ Complete | Multi-threaded, boundary expansion |
| **Dedup File Format** | ✅ Complete | Binary format with checksums |
| **Dedup Reader** | ✅ Complete | Lazy loading with sync.Once |
| **Dedup Writer** | ✅ Complete | Full file generation |
| **FUSE Filesystem** | ✅ Complete | Basic mount, read operations |
| **Verification** | ✅ Complete | Byte-by-byte comparison |
| **ES-aware Indexing** | ✅ Complete | MPEG-PS video/audio ES extraction |
| **Video user_data Filtering** | ✅ Complete | Excludes CC data for matching |
| **Per-Sub-Stream Audio** | ✅ Complete | Separate audio track matching |
| **Lazy Entry Loading** | ✅ Complete | Fast mount via sync.Once |
| **Verbose CLI Flag** | ✅ Complete | -v/--verbose for debug output |

### CLI Commands

| Command | Status | Notes |
|---------|--------|-------|
| `create` | ✅ Complete | Creates .mkvdup + .mkvdup.yaml |
| `mount` | ✅ Complete | Mounts config files via FUSE |
| `info` | ✅ Complete | Shows dedup file information |
| `verify` | ✅ Complete | Verifies reconstruction |
| `parse-mkv` | ✅ Complete | Debug: parse MKV structure |
| `index-source` | ✅ Complete | Debug: index source directory |
| `match` | ✅ Complete | Debug: match packets |
| `extract` | ❌ Not implemented | Rebuild original MKV |
| `reload` | ❌ Not implemented | Send SIGHUP to daemon |
| `check` | ❌ Not implemented | Full integrity check |
| `probe` | ❌ Not implemented | Quick MKV-to-source match test |

### Planned Features (Not Yet Implemented)

| Feature | Section | Notes |
|---------|---------|-------|
| Master config with includes | Phase 6 | Currently uses individual .yaml files |
| Hot reload (SIGHUP) | Phase 6 | Config file watching |
| Permissions store | Phase 6 | chmod/chown support |
| inotify event emission | Phase 6 | File change notifications |
| Source file watching | Error Handling | inotify on source files |
| Periodic health checks | Error Handling | Background verification |
| Blu-ray support | Source Indexer | M2TS parsing not implemented |
| Progress meters | Phase 7 | Fancy progress bars |
| Warning threshold | Phase 7 | Low dedup ratio warning |
| Quick probe command | Phase 7 | Fast MKV-to-source match test |

### Future Enhancements

#### Optimized Entry Loading (Optional)

The current lazy loading implementation with zero-copy mmap provides good performance:
- Mount is near-instant (lazy loading defers entry parsing)
- First file access: ~23s for 563MB file (includes loading 40K entries + reading from 7.6GB source ISO)
- Subsequent reads: ~3s for full file, 9ms for 1MB partial read

For very large files or seek-heavy workloads, two optional optimizations could further improve performance:

1. **Partial entry loading**: Implement a range-based index structure (e.g., B-tree over MKV offsets) to load only entries needed for each read. This would help applications that seek frequently or extract metadata without reading the entire file.

2. **Direct memory mapping to structures**: Use unsafe pointers to interpret the mmap'd region as entry structures directly. This eliminates parsing overhead but requires careful alignment handling.

#### Raw Offset Storage (Performance)

Currently, entries for ES-indexed sources (DVDs) store ES offsets, requiring ES-to-raw offset translation during FUSE reads. This translation involves looking up the appropriate PES payload range for each read.

Storing raw file offsets instead would enable direct zero-copy reads without translation overhead:
- During `create`, convert ES offsets to raw offsets before writing entries
- During `mount`/`read`, access source data directly via raw offset + mmap slice
- This would eliminate the need to maintain `ESReader` and payload range lookups at read time

Trade-off: Slightly larger dedup files (raw offsets may require additional metadata), but significantly faster FUSE access for ES-indexed sources.

### Zero-Copy Memory Mapping

All file access in the system uses true zero-copy memory mapping via `unix.Mmap` from `golang.org/x/sys/unix`. This is implemented in the `internal/mmap` package which provides:

```go
type File struct {
    data []byte  // Direct slice into mmap'd memory
    size int64
}

func Open(path string) (*File, error)     // Memory-map a file
func (m *File) Data() []byte              // Get full data slice (zero-copy)
func (m *File) Slice(offset, size) []byte // Get sub-slice (zero-copy)
func (m *File) Advise(advice int) error   // Hint kernel about access patterns
func (m *File) Close() error              // Unmap the file
```

**Key benefits:**
- **No data copying**: Slices point directly into kernel page cache
- **Efficient memory usage**: Pages are demand-loaded and can be evicted under memory pressure
- **Fast random access**: No syscall overhead for reads within mapped region

**ISO detection optimization**: DVD/Blu-ray detection reads only ~18KB (primary volume descriptor + root directory) instead of loading the entire ISO. This reduced memory usage from 40GB+ to ~640MB for a 7.6GB DVD.

**Usage throughout the codebase:**
- Source indexer: Memory-maps ISO files for zero-copy parsing
- MPEG-PS parser: Direct slice access to PES payloads
- MKV parser: Zero-copy access to EBML elements
- Matcher: Zero-copy byte comparisons
- Dedup reader: Zero-copy reconstruction from source files

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
│  │  │   - name: "Video2.mkv"                              │  │   │
│  │  │     dedup_file: "/data/dedup/video2.mkvdup"        │  │   │
│  │  │     source_dir: "/data/media/Video2_BD"            │  │   │
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
│  │                         │                                 │   │
│  │                         ▼                                 │   │
│  │              ┌─────────────────────┐                     │   │
│  │              │  Return data        │                     │   │
│  │              └─────────────────────┘                     │   │
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

## Phase 1: Source Indexer

Build a hash index of the source directory for fast lookup of byte sequences.

### Source Directory Detection

```go
type SourceType int

const (
    SourceTypeDVD     SourceType = iota  // Contains .iso file
    SourceTypeBluray                      // Contains BDMV/STREAM/*.m2ts
)

func DetectSourceType(dir string) (SourceType, error) {
    // Check for ISO file
    isos, _ := filepath.Glob(filepath.Join(dir, "*.iso"))
    if len(isos) > 0 {
        return SourceTypeDVD, nil
    }

    // Check for Blu-ray structure
    m2ts, _ := filepath.Glob(filepath.Join(dir, "BDMV", "STREAM", "*.m2ts"))
    if len(m2ts) > 0 {
        return SourceTypeBluray, nil
    }

    return 0, errors.New("unknown source type")
}
```

### Indexing Strategy

**Codec detection:**

Different media use different video codecs with different framing:

| Media | Possible Video Codecs | Container |
|-------|----------------------|-----------|
| DVD | MPEG-2 | VOB (MPEG-PS) |
| Blu-ray | H.264/AVC, MPEG-2, VC-1, HEVC | M2TS (MPEG-TS) |

**Start code patterns by codec:**

| Codec | Start Code Pattern | Notes |
|-------|-------------------|-------|
| MPEG-2 | `00 00 01 xx` | xx = B3 (seq), 00 (pic), 01-AF (slice) |
| H.264/AVC | `00 00 01 xx` or `00 00 00 01 xx` | xx = NAL unit type |
| HEVC | `00 00 01 xx` or `00 00 00 01 xx` | xx = NAL unit type |
| VC-1 | `00 00 01 xx` | Different structure than above |

**Pragmatic approach:**

Since most video codecs use `00 00 01` as a start code prefix, we can:

1. **For DVD (VOB files):**
   - Scan for `00 00 01` byte sequences
   - Index at each occurrence

2. **For Blu-ray (M2TS files):**
   - Parse MPEG-TS packets (192 bytes: 4-byte header + 188-byte payload)
   - Within payload, scan for `00 00 01` sequences
   - Index at each occurrence

3. **Codec-specific refinement (optional):**
   - Detect codec from container metadata (IFO for DVD, CLIPINF for Blu-ray)
   - Apply codec-specific filtering (e.g., only index certain NAL types for H.264)

**Video implementation:**

```go
// Common start code used by MPEG-2, H.264, HEVC, VC-1
var startCodePrefix = []byte{0x00, 0x00, 0x01}

func findVideoStartCodes(data []byte) []int {
    var offsets []int
    for i := 0; i <= len(data)-3; i++ {
        if data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x01 {
            offsets = append(offsets, i)
        }
    }
    return offsets
}
```

This approach works across all common video codecs without needing to detect the specific codec in advance.

### ES-Aware Indexing for DVDs (MPEG-PS)

**Problem:** DVDs use MPEG Program Stream (MPEG-PS) containers which wrap Elementary Stream (ES) data
in PES packets. When ripping to MKV, tools extract the raw ES data, stripping all container framing.
If we index raw file offsets in the ISO, the container bytes create misalignments and matches fail.

**Solution:** For DVDs, we parse the MPEG-PS structure and build an index based on **ES offsets**
(continuous byte positions within the elementary stream) rather than raw file offsets.

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

The ES is continuous - no container headers interrupting the codec data.
```

**Implementation:**

```go
type MPEGPSParser struct {
    reader      *mmap.ReaderAt
    videoRanges []PESPayloadRange  // Maps ES offsets to file offsets
    audioRanges []PESPayloadRange
}

type PESPayloadRange struct {
    FileOffset int64  // Where the payload is in the actual file
    Size       int    // Size of this payload chunk
    ESOffset   int64  // Logical position in the continuous ES
}

// ReadESData reads continuous ES data, handling the fragmented file storage
func (p *MPEGPSParser) ReadESData(esOffset int64, size int, isVideo bool) ([]byte, error)
```

**Index entry storage:**

For DVD sources, `Location.Offset` stores ES offsets, not file offsets. The `Index.UsesESOffsets`
flag indicates this mode is active.

### Video user_data Filtering

**Problem:** MKV tools (like MakeMKV) strip `user_data` sections (start code `00 00 01 B2`) from
video streams. These sections contain closed captions and other auxiliary data. If we index the
raw video ES including user_data, those sections create misalignments when matching against MKV.

**Solution:** When building the video ES index, we create "filtered ranges" that exclude user_data
sections. This produces an ES that matches what MKV tools output.

```
Raw video ES:          [video][user_data][video][user_data][video]
                          │        │        │        │        │
Filtered video ES:     [video]...........[video]...........[video]
                       (user_data sections excluded from filtered ES offsets)
```

**Implementation in `mpegps.go`:**

```go
func (p *MPEGPSParser) buildFilteredVideoRanges() error {
    for _, rawRange := range p.videoRanges {
        data := make([]byte, rawRange.Size)
        p.reader.ReadAt(data, rawRange.FileOffset)

        // Scan for user_data sections (00 00 01 B2) and exclude them
        i := 0
        rangeStart := 0
        for i < len(data)-3 {
            if data[i] == 0x00 && data[i+1] == 0x00 &&
               data[i+2] == 0x01 && data[i+3] == 0xB2 {
                // Emit range before user_data
                if i > rangeStart {
                    filteredRanges = append(filteredRanges, PESPayloadRange{...})
                }
                // Skip to next start code
                i += 4
                for i < len(data)-3 {
                    if data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x01 {
                        break
                    }
                    i++
                }
                rangeStart = i
            } else {
                i++
            }
        }
        // Emit remaining data
        ...
    }
}
```

**Result:** Video matching improved from ~60% to ~98.6% after user_data filtering.

### Audio Indexing Strategy

Audio packets also need to be indexed for deduplication. Audio typically achieves ~100% match rate
(unlike video which may have CC differences).

**Audio sync patterns by codec:**

| Codec | Sync Pattern | Notes |
|-------|-------------|-------|
| AC3 (Dolby Digital) | `0B 77` | 2-byte sync word |
| E-AC3 (Dolby Digital Plus) | `0B 77` | Same as AC3 |
| DTS | `7F FE 80 01` | 4-byte sync word |
| DTS-HD | `7F FE 80 01` | Same core sync |
| MPEG Audio (MP2/MP3) | `FF Fx` | 11-bit sync (0x7FF), x varies |
| AAC (ADTS) | `FF Fx` | Similar to MPEG, x = 0xF0-0xF1 |
| LPCM/PCM | (none) | Raw samples, no framing |
| TrueHD | `F8 72 6F BA` | 4-byte sync word |
| FLAC | `FF F8` | Frame sync pattern |

**Audio implementation:**

```go
// Audio sync patterns
var (
    syncAC3    = []byte{0x0B, 0x77}
    syncDTS    = []byte{0x7F, 0xFE, 0x80, 0x01}
    syncTrueHD = []byte{0xF8, 0x72, 0x6F, 0xBA}
)

func findAudioSyncPoints(data []byte) []int {
    var offsets []int

    for i := 0; i <= len(data)-4; i++ {
        // AC3/E-AC3
        if data[i] == 0x0B && data[i+1] == 0x77 {
            offsets = append(offsets, i)
            continue
        }

        // DTS / DTS-HD
        if i <= len(data)-4 &&
           data[i] == 0x7F && data[i+1] == 0xFE &&
           data[i+2] == 0x80 && data[i+3] == 0x01 {
            offsets = append(offsets, i)
            continue
        }

        // TrueHD
        if i <= len(data)-4 &&
           data[i] == 0xF8 && data[i+1] == 0x72 &&
           data[i+2] == 0x6F && data[i+3] == 0xBA {
            offsets = append(offsets, i)
            continue
        }

        // MPEG Audio / AAC ADTS (0xFF followed by 0xF0-0xFF with bit constraints)
        if data[i] == 0xFF && (data[i+1]&0xF0) == 0xF0 {
            offsets = append(offsets, i)
            continue
        }
    }

    return offsets
}
```

**Combined indexing:**

The source indexer scans for both video and audio sync points:

```go
func indexSourceFile(data []byte) []IndexEntry {
    var entries []IndexEntry

    // Find all video start codes
    for _, offset := range findVideoStartCodes(data) {
        entries = append(entries, IndexEntry{
            Offset: offset,
            Hash:   computeHash(data, offset),
            Type:   TypeVideo,
        })
    }

    // Find all audio sync points
    for _, offset := range findAudioSyncPoints(data) {
        entries = append(entries, IndexEntry{
            Offset: offset,
            Hash:   computeHash(data, offset),
            Type:   TypeAudio,
        })
    }

    // Sort by offset for efficient lookup
    sort.Slice(entries, func(i, j int) bool {
        return entries[i].Offset < entries[j].Offset
    })

    return entries
}
```

**LPCM/PCM handling:**

LPCM has no sync patterns - it's raw sample data. For LPCM tracks:
- Index at regular intervals (e.g., every 2048 or 4096 bytes)
- Or skip LPCM indexing and rely on adjacent packet boundary expansion

### DVD Audio: Private Stream 1 Header Stripping

**Problem:** DVD audio is carried in MPEG-PS Private Stream 1 (stream ID 0xBD). Each PES packet
contains a 4-byte header before the actual audio data:

```
Private Stream 1 payload structure:
┌────────────┬─────────────┬──────────────────┬─────────────────────────────┐
│ Sub-stream │ Frame count │ First access ptr │ Audio data (AC3/DTS/LPCM)   │
│   ID (1)   │    (1)      │      (2)         │                             │
└────────────┴─────────────┴──────────────────┴─────────────────────────────┘
     0x80        0x02           0x00 0x01         0x0B 0x77 ... (AC3 sync)

Sub-stream IDs:
  0x80-0x87: AC3 (Dolby Digital)
  0x88-0x8F: DTS
  0xA0-0xA7: LPCM
  0x20-0x3F: Subpictures (DVD subtitles)
```

MKV tools strip this 4-byte header, storing only the raw audio codec data. We must do the same
when building filtered audio ranges.

### DVD Audio: Per-Sub-Stream Filtering

**Problem:** Private Stream 1 carries multiple audio tracks interleaved together. A DVD might have:
- Sub-stream 0x80: English AC3 (5.1)
- Sub-stream 0x81: Spanish AC3 (2.0)
- Sub-stream 0x82: French AC3 (5.1)

These are interleaved in the MPEG-PS file:

```
PES packets in file order:
[0x80 audio][0x81 audio][0x80 audio][0x82 audio][0x80 audio][0x81 audio]...

If combined into single ES:
[English][Spanish][English][French][English][Spanish]...
```

When matching an MKV audio track (which contains only ONE language), we'd hit data from other
tracks and matching would fail. This is analogous to the user_data problem with video.

**Solution:** Create separate filtered ES ranges for each sub-stream ID. When matching MKV audio,
we match against only the specific sub-stream that corresponds to that MKV track.

```go
type MPEGPSParser struct {
    // ...
    // Filtered audio ranges per sub-stream ID - separates interleaved audio tracks
    filteredAudioBySubStream map[byte][]PESPayloadRange
    // Sub-stream IDs in order of appearance
    audioSubStreams []byte
}

func (p *MPEGPSParser) buildFilteredAudioRanges() error {
    rangesBySubStream := make(map[byte][]PESPayloadRange)
    esOffsetBySubStream := make(map[byte]int64)

    for _, rawRange := range p.audioRanges {
        // Read sub-stream ID (first byte of payload)
        header := make([]byte, 1)
        p.reader.ReadAt(header, rawRange.FileOffset)
        subStreamID := header[0]

        // Check if this is AC3, DTS, or LPCM (skip subpictures)
        isAudio := (subStreamID >= 0x80 && subStreamID <= 0x87) ||  // AC3
                   (subStreamID >= 0x88 && subStreamID <= 0x8F) ||  // DTS
                   (subStreamID >= 0xA0 && subStreamID <= 0xA7)     // LPCM

        if isAudio && rawRange.Size > 4 {
            esOffset := esOffsetBySubStream[subStreamID]
            rangesBySubStream[subStreamID] = append(rangesBySubStream[subStreamID],
                PESPayloadRange{
                    FileOffset: rawRange.FileOffset + 4,  // Skip 4-byte header
                    Size:       rawRange.Size - 4,
                    ESOffset:   esOffset,
                })
            esOffsetBySubStream[subStreamID] += int64(rawRange.Size - 4)
        }
    }

    p.filteredAudioBySubStream = rangesBySubStream
    return nil
}
```

**Index entry storage:**

The `Location` struct includes `AudioSubStreamID` to track which sub-stream a hash belongs to:

```go
type Location struct {
    FileIndex        uint16
    Offset           int64  // ES offset within the sub-stream
    IsVideo          bool
    AudioSubStreamID byte   // For audio: 0x80, 0x81, etc.
}
```

**Matching:** When matching audio, the matcher filters candidate locations to only those with
the same sub-stream ID. This prevents false matches against different audio tracks.

**Result:** Audio matching improved from ~25% to ~99%+ after per-sub-stream filtering.
Combined with video filtering, overall byte matching improved from ~50% to **98.4%**.

### Data Structure

```go
type SourceIndex struct {
    // Map from hash to list of locations
    HashToLocations map[uint64][]SourceLocation

    // Source directory path (stored as relative reference)
    SourceDir string

    // Type of source
    SourceType SourceType

    // List of files in source (for multi-file sources like Blu-ray)
    Files []SourceFile

    // Window size used for hashing
    WindowSize int
}

type SourceLocation struct {
    FileIndex  uint16  // Index into Files array
    Offset     int64   // Offset within that file
}

type SourceFile struct {
    RelativePath string  // Relative to source dir
    Size         int64
    Checksum     uint64  // xxhash of file for integrity
}
```

### Algorithm

```
1. Detect source type (DVD or Blu-ray)
2. Enumerate media files:
   - DVD: Single ISO or VOB files
   - Blu-ray: All .m2ts files in BDMV/STREAM/
3. For each file:
   a. Memory-map the file
   b. Scan for codec packet boundaries (start codes or TS sync)
   c. At each boundary:
      - Read WindowSize bytes
      - Compute xxhash
      - Store: HashToLocations[hash] = append(locs, {fileIdx, offset})
4. Cache index for reuse (optional)
```

## Phase 2: MKV Parser

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
│   ├── SimpleBlock
│   ├── BlockGroup
│   │   ├── Block
│   │   └── BlockDuration
│   └── ...
├── Clusters...
└── Cues (seek index)
```

### Key Insight

The actual codec data is inside `SimpleBlock` and `Block` elements. Each block contains:
- Track number (1 byte, variable)
- Timestamp offset (2 bytes)
- Flags (1 byte for SimpleBlock)
- Frame data (the actual codec bytes we want to match)

### Algorithm

```
1. Parse EBML header
2. Find Segment element
3. Iterate through Clusters:
   a. For each SimpleBlock/Block:
      - Record: MKV file offset of frame data
      - Record: Length of frame data
      - Compute: Hash of first N bytes of frame data
4. Output: List of (mkv_offset, length, hash)
```

### Library Options

- Use existing Go EBML library: `github.com/ebml-go/ebml`
- Or parse manually (EBML is relatively simple)

## Phase 3: Matcher

Match MKV packets to source locations.

### Algorithm

```
For each MKV packet (mkv_offset, length, hash):
    1. Look up hash in SourceIndex.HashToLocations
    2. If found:
       a. For each candidate location:
          - Verify full match (compare all `length` bytes)
          - If match: record as SOURCE reference
          - Break on first match
    3. If not found or no verified match:
       - Mark as DELTA (unique data)
       - Append data to delta buffer
```

### Handling Non-Packet Data

MKV container overhead (EBML headers, cluster headers) won't match anything in the source. These go into the delta automatically.

The index needs to cover the ENTIRE MKV file:

```
MKV byte range 0-1000:        EBML header → DELTA
MKV byte range 1000-1050:     Cluster header → DELTA
MKV byte range 1050-9000:     Video frame → SOURCE file=0, offset=463000
MKV byte range 9000-9020:     Block header → DELTA
MKV byte range 9020-11000:    Audio packet → SOURCE file=0, offset=464500
...
```

### Boundary Expansion (Match Maximization)

After finding a match via hash lookup, the matched region should be **expanded** in both directions
to maximize deduplication. The hash-indexed position is just a starting point - the actual matching
data often extends beyond the sync point boundaries.

**Why this matters:**

```
Source (VOB):     [container][video data...............][container][audio data....]
                            ^                          ^
                       start code                  start code
                       (indexed)                   (indexed)

MKV:              [ebml hdr][video data...............][block hdr][audio data....]
                            ^                          ^
                       matches here                matches here

Without expansion: Only match from start code to next start code
With expansion:    Match extends to include any adjacent matching bytes
```

**Algorithm:**

```go
// After finding initial match at (mkvOffset, sourceOffset) with length matchLen:

func expandMatch(mkvData, sourceData []byte, mkvOffset, sourceOffset, initialLen int64) (newMkvStart, newSourceStart, newLen int64) {
    // Start with the verified match
    newMkvStart = mkvOffset
    newSourceStart = sourceOffset
    newLen = initialLen

    // Expand backward: check bytes before the match
    for newMkvStart > 0 && newSourceStart > 0 {
        mkvByte := mkvData[newMkvStart-1]
        srcByte := sourceData[newSourceStart-1]
        if mkvByte != srcByte {
            break
        }
        newMkvStart--
        newSourceStart--
        newLen++
    }

    // Expand forward: check bytes after the match
    mkvEnd := mkvOffset + initialLen
    srcEnd := sourceOffset + initialLen
    for mkvEnd < int64(len(mkvData)) && srcEnd < int64(len(sourceData)) {
        mkvByte := mkvData[mkvEnd]
        srcByte := sourceData[srcEnd]
        if mkvByte != srcByte {
            break
        }
        mkvEnd++
        srcEnd++
        newLen++
    }

    return newMkvStart, newSourceStart, newLen
}
```

**Example:**

```
Initial match found via hash:
  MKV offset 1050, Source offset 463000, Length 7950 (video packet)

After boundary expansion:
  MKV offset 1048, Source offset 462998, Length 7954
  (Expanded 2 bytes backward, 2 bytes forward)
```

**Benefits:**
- Maximizes deduplication ratio
- Reduces delta size
- Captures partial matches at packet boundaries
- Helps with LPCM audio (no sync patterns) - can expand from adjacent matches

**Constraints:**
- Don't expand into already-matched regions (track matched ranges)
- Limit expansion distance to prevent runaway (e.g., max 64KB expansion each direction)
- Expansion is done after initial match verification, not during hash lookup

**Updated matching algorithm:**

```
For each MKV packet (mkv_offset, length, hash):
    1. Look up hash in SourceIndex.HashToLocations
    2. If found:
       a. For each candidate location:
          - Verify full match (compare all `length` bytes)
          - If match:
            - EXPAND match boundaries (new step)
            - Record expanded region as SOURCE reference
            - Mark MKV range as matched (for overlap prevention)
            - Break on first match
    3. If not found or no verified match:
       - Mark as DELTA (unique data)
       - Append data to delta buffer
```

## Phase 4: Unified Dedup File Format (.mkvdup)

**Single file containing both index and delta data.**

### Design Principles

1. **No repeated strings**: Filenames stored once, referenced by index
2. **Binary encoding**: All numeric values as binary (little-endian), not text
3. **Relative paths**: Source file paths relative to `source_dir` from FUSE config
4. **Compact index entries**: Use smallest types that fit the data

### File Structure

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
│  UsesESOffsets: uint8 (1 byte)  [1=ES offsets mode]    │
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

### Source Reference Encoding

For index entries:
- `Source = 0`: Data is in delta section at `SourceOffset`
- `Source = 1`: Data is in source file 0 at `SourceOffset`
- `Source = 2`: Data is in source file 1 at `SourceOffset`
- etc.

### Storage Efficiency Notes

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

### Delta Contents

The delta section contains ONLY data that couldn't be matched to the source:
- **MKV container overhead** (EBML headers, cluster metadata, block headers)
- **Closed caption user data** (if present in video stream)
- **Any unmatched codec data** (rare, if matching works correctly)

**Important:** Audio and video codec data should NOT end up in delta if the matching algorithm
works correctly. The delta should be almost entirely container overhead.

**Expected delta size (tested with a DVD ISO):**

For a typical video with ~8,000-12,000 clusters and ~200,000 blocks:
- Cluster headers: ~120-180 KB
- Block headers: ~1 MB
- EBML header: ~5 KB
- **Total delta: ~1.2-1.5 MB** (if matching is perfect)

If delta is significantly larger (e.g., >10 MB), this indicates unmatched codec data,
which suggests a problem with the matching algorithm or wrong source files.

**Note:** Delta compression was considered but rejected. Testing showed only 2-3:1 compression
ratio on container headers, saving ~500 KB on a typical video. The complexity (decompression
on reads, memory buffering) is not worth such minimal savings.

## Phase 5: Verification

**Mandatory verification after deduplication.**

### Algorithm

```
1. Create dedup file
2. Mount via FUSE (temporary, internal mount)
3. Open virtual MKV through FUSE
4. Open original MKV
5. Compare byte-by-byte (or in large chunks with hash comparison)
6. If mismatch:
   a. Report error with offset
   b. Delete dedup file
   c. Exit with failure
7. If match:
   a. Report success
   b. Optionally delete original MKV (with --delete-original flag)
```

### Implementation

```go
func Verify(dedupFile, originalMKV string) error {
    // Create temporary mount point
    tmpMount, err := os.MkdirTemp("", "mkvdup-verify-")
    if err != nil {
        return err
    }
    defer os.RemoveAll(tmpMount)

    // Mount FUSE filesystem with single file
    fs := NewDedupFS(dedupFile, sourceDir)
    go fs.Mount(tmpMount)
    defer fs.Unmount()

    // Wait for mount
    time.Sleep(100 * time.Millisecond)

    // Compare files
    virtualPath := filepath.Join(tmpMount, "verify.mkv")
    return compareFiles(virtualPath, originalMKV)
}

func compareFiles(a, b string) error {
    const chunkSize = 1024 * 1024 // 1MB chunks

    fa, _ := os.Open(a)
    fb, _ := os.Open(b)
    defer fa.Close()
    defer fb.Close()

    bufA := make([]byte, chunkSize)
    bufB := make([]byte, chunkSize)
    offset := int64(0)

    for {
        nA, errA := fa.Read(bufA)
        nB, errB := fb.Read(bufB)

        if nA != nB || !bytes.Equal(bufA[:nA], bufB[:nB]) {
            return fmt.Errorf("mismatch at offset %d", offset)
        }

        if errA == io.EOF && errB == io.EOF {
            return nil // Success
        }
        if errA != nil || errB != nil {
            return fmt.Errorf("read error: %v / %v", errA, errB)
        }

        offset += int64(nA)
    }
}
```

## Phase 6: FUSE Configuration File

### Format (YAML)

### Per-Mapping Config Files

Each dedup operation creates TWO files:
1. `.mkvdup` - The dedup data file (index + delta)
2. `.mkvdup.yaml` - Config file for this mapping

**Individual mapping config (video1.mkvdup.yaml):**

```yaml
# Auto-generated by mkvdup create
name: "Video1.mkv"
dedup_file: "/data/dedup/video1.mkvdup"
source_dir: "/data/sources/Video1_DVD"
```

### Master FUSE Config with Includes

**Main config (/etc/mkvdup/mount.yaml):**

```yaml
mountpoint: /mnt/media

# Include individual mapping configs
includes:
  - "/data/dedup/video1.mkvdup.yaml"
  - "/data/dedup/video2.mkvdup.yaml"
  - "/data/dedup/*.mkvdup.yaml"  # Glob patterns supported

# Can also define inline (optional)
virtual_files:
  - name: "Videos/Collection1/Video3.mkv"
    dedup_file: "/data/dedup/video3.mkvdup"
    source_dir: "/data/sources/Collection1_Bluray"
```

### Hot Reload Support

The FUSE daemon supports live config reload without restart:

1. **Signal-based reload:**
   ```bash
   # Send SIGHUP to reload config
   kill -HUP $(pidof mkvdup)
   ```

2. **File-watch reload:**
   - Daemon watches config file and include directories
   - Automatically reloads when changes detected
   - Uses inotify for efficient monitoring

3. **On reload:**
   - New virtual files become immediately available
   - Removed virtual files become unavailable (active readers continue until close)
   - Modified mappings: existing readers use old mapping until close

### Linux File Change Notifications

The FUSE filesystem emits standard inotify events:

```go
// When virtual files are added/removed via config reload
func (fs *DedupFS) notifyFileAdded(path string) {
    // Emit IN_CREATE event
    fs.inotify.Emit(path, unix.IN_CREATE)
}

func (fs *DedupFS) notifyFileRemoved(path string) {
    // Emit IN_DELETE event
    fs.inotify.Emit(path, unix.IN_DELETE)
}
```

Applications watching the mountpoint (e.g., media servers like Jellyfin/Plex)
will automatically detect changes.

### Data Structures

```go
type MasterConfig struct {
    Mountpoint   string        `yaml:"mountpoint"`
    Includes     []string      `yaml:"includes"`      // Paths or globs
    VirtualFiles []VirtualFile `yaml:"virtual_files"` // Inline definitions
}

type VirtualFile struct {
    Name      string `yaml:"name"`       // Virtual path under mountpoint
    DedupFile string `yaml:"dedup_file"` // Path to .mkvdup file
    SourceDir string `yaml:"source_dir"` // Path to source directory
}

type DedupFS struct {
    mu           sync.RWMutex
    config       *MasterConfig
    configPath   string
    files        map[string]*VirtualFileState
    inotifyFd    int                    // For emitting events
    watcher      *fsnotify.Watcher      // For watching config changes
}

func (fs *DedupFS) ReloadConfig() error {
    fs.mu.Lock()
    defer fs.mu.Unlock()

    newConfig, err := LoadMasterConfig(fs.configPath)
    if err != nil {
        return err
    }

    // Diff old vs new
    added, removed := diffConfigs(fs.config, newConfig)

    // Remove old files (mark as removed, actual cleanup on last close)
    for _, vf := range removed {
        if state, ok := fs.files[vf.Name]; ok {
            state.markedForRemoval = true
            fs.notifyFileRemoved(vf.Name)
        }
    }

    // Add new files
    for _, vf := range added {
        fs.files[vf.Name] = NewVirtualFileState(vf)
        fs.notifyFileAdded(vf.Name)
    }

    fs.config = newConfig
    return nil
}
```

### FUSE Directory Structure

The FUSE filesystem presents a virtual directory tree:

```
/mnt/media/                          (mountpoint)
└── Videos/
    ├── Video1.mkv                   (virtual file)
    ├── Video2.mkv                   (virtual file)
    └── Collection1/
        ├── Video3.mkv               (virtual file)
        ├── Video4.mkv               (virtual file)
        └── Video5.mkv               (virtual file)
```

### Filesystem Metadata (Permissions, Ownership, Size)

**File size:**
- Returned from `OriginalSize` in the .mkvdup header (O(1) lookup)
- No computation needed at runtime

**Permissions and ownership:**

Virtual files support `chmod` and `chown` operations. Metadata is stored in a separate
permissions file to keep .mkvdup files immutable.

**Permissions file location:**
- Default: `~/.config/mkvdup/permissions.yaml`
- Configurable via `--permissions-file` or in mount config

**Permissions file format:**
```yaml
# Auto-managed by mkvdup daemon (also human-editable)
# Changes are picked up on SIGHUP reload

files:
  "Videos/Video1.mkv":
    uid: 1000
    gid: 1001
    mode: 0640
  "Videos/Video2.mkv":
    mode: 0444  # read-only, inherits uid/gid from daemon defaults
```

**Default behavior (when file not in permissions.yaml):**
1. Use daemon defaults from config/command line if specified
2. Otherwise use `root:root` (uid=0, gid=0) with mode `0644`

**CLI/config options for defaults:**
```bash
# Mount with custom default ownership
mkvdup mount --config mount.yaml --default-uid 1000 --default-gid 1000 --default-mode 0644

# Or in mount config yaml
defaults:
  uid: 1000
  gid: 1000
  mode: 0644
  permissions_file: /var/lib/mkvdup/permissions.yaml
```

**Implementation:**

```go
type PermissionsStore struct {
    mu       sync.RWMutex
    path     string
    files    map[string]*FilePermissions  // virtual path -> permissions
    defaults DefaultPermissions           // non-nil defaults
}

// DefaultPermissions holds the fallback values (always set)
type DefaultPermissions struct {
    Uid  uint32
    Gid  uint32
    Mode uint32
}

// FilePermissions holds per-file overrides (nil = use default)
type FilePermissions struct {
    Uid  *uint32 `yaml:"uid,omitempty"`
    Gid  *uint32 `yaml:"gid,omitempty"`
    Mode *uint32 `yaml:"mode,omitempty"`
}

func (ps *PermissionsStore) Get(path string) (uid, gid, mode uint32) {
    ps.mu.RLock()
    defer ps.mu.RUnlock()

    // Start with defaults
    uid = ps.defaults.Uid
    gid = ps.defaults.Gid
    mode = ps.defaults.Mode

    // Override with file-specific values if present
    if fp := ps.files[path]; fp != nil {
        if fp.Uid != nil { uid = *fp.Uid }
        if fp.Gid != nil { gid = *fp.Gid }
        if fp.Mode != nil { mode = *fp.Mode }
    }

    return uid, gid, mode
}

func (ps *PermissionsStore) SetOwner(path string, uid, gid uint32) error {
    ps.mu.Lock()
    defer ps.mu.Unlock()

    if ps.files[path] == nil {
        ps.files[path] = &FilePermissions{}
    }
    ps.files[path].Uid = &uid
    ps.files[path].Gid = &gid

    return ps.save()
}

func (ps *PermissionsStore) SetMode(path string, mode uint32) error {
    ps.mu.Lock()
    defer ps.mu.Unlock()

    if ps.files[path] == nil {
        ps.files[path] = &FilePermissions{}
    }
    ps.files[path].Mode = &mode

    return ps.save()
}

func (ps *PermissionsStore) Reload() error {
    ps.mu.Lock()
    defer ps.mu.Unlock()
    return ps.load()
}
```

**FUSE operations:**

```go
func (f *VirtualFile) Getattr(out *fuse.Attr) fuse.Status {
    // File size from header (O(1))
    out.Size = uint64(f.state.header.OriginalSize)

    // Permissions from store
    out.Uid, out.Gid, out.Mode = f.fs.permissions.Get(f.path)
    out.Mode |= fuse.S_IFREG  // Mark as regular file

    // Timestamps (use dedup file's mtime)
    out.Mtime = f.state.dedupMtime
    out.Atime = f.state.dedupMtime
    out.Ctime = f.state.dedupMtime

    return fuse.OK
}

func (f *VirtualFile) Chown(uid, gid uint32) fuse.Status {
    if err := f.fs.permissions.SetOwner(f.path, uid, gid); err != nil {
        log.Errorf("Failed to set owner for %s: %v", f.path, err)
        return fuse.EIO
    }
    return fuse.OK
}

func (f *VirtualFile) Chmod(mode uint32) fuse.Status {
    if err := f.fs.permissions.SetMode(f.path, mode); err != nil {
        log.Errorf("Failed to set mode for %s: %v", f.path, err)
        return fuse.EIO
    }
    return fuse.OK
}
```

**Permissions cleanup:**

When virtual files are removed from the config, their permissions entries should be cleaned up.
Cleanup runs on:
1. **Initial mount** - removes stale entries from permissions file
2. **SIGHUP reload** - removes entries for files just removed from config

```go
func (ps *PermissionsStore) Cleanup(validPaths map[string]bool) (removed []string, err error) {
    ps.mu.Lock()
    defer ps.mu.Unlock()

    for path := range ps.files {
        if !validPaths[path] {
            delete(ps.files, path)
            removed = append(removed, path)
        }
    }

    if len(removed) > 0 {
        log.Infof("Cleaned up permissions for %d removed virtual files", len(removed))
        return removed, ps.save()
    }
    return nil, nil
}
```

**SIGHUP reload behavior:**

On SIGHUP, the daemon reloads:
1. Main mount config (virtual file mappings)
2. Permissions file (ownership/mode changes)
3. Cleans up permissions for removed virtual files
4. Triggers source file re-validation if source_watch is enabled

```go
func (fs *DedupFS) handleSIGHUP() {
    log.Info("Received SIGHUP, reloading configuration...")

    // Reload main config
    if err := fs.ReloadConfig(); err != nil {
        log.Errorf("Failed to reload config: %v", err)
    }

    // Reload permissions
    if err := fs.permissions.Reload(); err != nil {
        log.Errorf("Failed to reload permissions: %v", err)
    }

    // Cleanup permissions for files no longer in config
    validPaths := make(map[string]bool)
    for path := range fs.files {
        validPaths[path] = true
    }
    if _, err := fs.permissions.Cleanup(validPaths); err != nil {
        log.Errorf("Failed to cleanup permissions: %v", err)
    }

    log.Info("Configuration reload complete")
}
```

## Phase 7: CLI Tool

### Global Options (Implemented)

```bash
# Enable verbose/debug output for any command
mkvdup -v <command> [args...]
mkvdup --verbose <command> [args...]

# Examples:
mkvdup -v create video.mkv /source/dir
mkvdup -v mount /mnt/media config1.yaml config2.yaml
mkvdup -v verify video.mkvdup /source/dir video.mkv
```

**Verbose mode enables:**
- FUSE operation logging (Open, Read, Lookup, Readdir)
- Detailed verification output with byte comparisons
- Debug information for troubleshooting

### Commands

```bash
# Create dedup file from MKV + source directory
# Automatically detects DVD ISO or Blu-ray structure
# Verifies after creation
# Outputs: video.mkvdup (data) + video.mkvdup.yaml (config)
mkvdup create \
    --mkv /path/to/video.mkv \
    --source /path/to/source_dir \
    --output /path/to/video.mkvdup \
    --name "Videos/video.mkv"  # Virtual path in FUSE mount

# Create with automatic deletion of original after verification
mkvdup create \
    --mkv /path/to/video.mkv \
    --source /path/to/source_dir \
    --output /path/to/video.mkvdup \
    --name "Videos/video.mkv" \
    --delete-original

# Custom warning threshold (default 75%)
mkvdup create \
    --mkv /path/to/video.mkv \
    --source /path/to/source_dir \
    --output /path/to/video.mkvdup \
    --warn-threshold 80  # Warn if space savings < 80%

# Verify existing dedup file against original
mkvdup verify \
    --dedup /path/to/video.mkvdup \
    --source /path/to/source_dir \
    --original /path/to/video.mkv

# Mount virtual filesystem from config file
mkvdup mount --config /path/to/config.yaml

# Mount with auto-reload on config changes
mkvdup mount --config /path/to/config.yaml --watch

# Reload running daemon's config
mkvdup reload  # Sends SIGHUP to running daemon

# Show info about dedup file
mkvdup info --dedup /path/to/video.mkvdup

# Rebuild/extract original MKV from dedup + source
mkvdup extract \
    --dedup /path/to/video.mkvdup \
    --source /path/to/source_dir \
    --output /path/to/restored.mkv

# Quick probe: test if MKV likely matches a source (fast, <1 min)
# Useful for multi-disc sets to find which ISO matches which MKV
mkvdup probe /path/to/video.mkv /path/to/source1 /path/to/source2 ...

# Example output:
#   Probing video.mkv against 3 sources...
#   Sampling 20 packets from MKV...
#
#   Results:
#     /data/disc1  18/20 matches (90%) ← likely match
#     /data/disc2   2/20 matches (10%)
#     /data/disc3   0/20 matches (0%)
```

### Statistics Output

The `create` command outputs detailed statistics with progress meters and time estimates for each phase:

```
$ mkvdup create --mkv video.mkv --source /data/DVD --output video.mkvdup --name "video.mkv"

Phase 1/6: Analyzing source directory...
  Source type: DVD
  Source files: 4
  Total source size: 4.51 GB
  ✓ Complete (0.2s)

Phase 2/6: Parsing MKV file...
  [████████████████████████████████████████] 100%  3.42 GB / 3.42 GB  ETA: 00:00:00
  MKV size: 3.42 GB
  Video packets: 1,247,832
  Audio packets: 892,156
  Total packets: 2,139,988
  ✓ Complete (4.8s)

Phase 3/6: Building source index...
  [████████████████████████████████████████] 100%  4.51 GB / 4.51 GB  ETA: 00:00:00
  Indexed 2,847,291 start codes
  ✓ Complete (12.4s)

Phase 4/6: Matching packets...
  [████████████████████░░░░░░░░░░░░░░░░░░░░]  52%  1.1M / 2.1M packets  ETA: 00:00:14
  (progress updates in place)
  [████████████████████████████████████████] 100%  2.1M / 2.1M packets  ETA: 00:00:00
  ✓ Complete (28.7s)

Phase 5/6: Writing dedup file...
  [████████████████████████████████████████] 100%  52.7 MB written  ETA: 00:00:00
  ✓ Complete (0.2s)

Phase 6/6: Verifying reconstruction...
  [████████████████████████████████████████] 100%  3.42 GB / 3.42 GB  ETA: 00:00:00
  ✓ Verification passed: reconstructed MKV matches original
  ✓ Complete (5.2s)

═══════════════════════════════════════════════════════════════

Deduplication Results:
  ┌─────────────────────────────────────────────────────────┐
  │  Original MKV size:          3.42 GB                    │
  │  Matched data (from source): 3.418 GB (99.95%)          │
  │  Unique data (delta):        1.5 MB   (0.04%)           │
  │  Index overhead:             51.2 MB  (1.5%)            │
  │  ─────────────────────────────────────────────────────  │
  │  Dedup file size:            52.7 MB                    │
  │  Space savings:              3.37 GB  (98.5%)           │
  └─────────────────────────────────────────────────────────┘

  Packet Statistics:
    Video matched:    1,247,832 / 1,247,832  (100.0%)
    Audio matched:      892,156 /   892,156  (100.0%)
    Container overhead: 12,419 clusters + 2.1M block headers (delta)

Output files:
  Dedup file:  /path/to/video.mkvdup (52.7 MB)
  Config file: /path/to/video.mkvdup.yaml

Total time: 51.5s
  Phase 1 (Source analysis):   0.2s  ( 0.4%)
  Phase 2 (MKV parsing):       4.8s  ( 9.3%)
  Phase 3 (Index building):   12.4s  (24.1%)
  Phase 4 (Packet matching):  28.7s  (55.7%)
  Phase 5 (File writing):      0.2s  ( 0.4%)
  Phase 6 (Verification):      5.2s  (10.1%)
```

### Progress Meter Details

Each phase shows:
- **Phase number**: Current phase out of total (e.g., "Phase 3/6")
- **Progress bar**: Visual representation of completion percentage
- **Percentage**: Numeric completion percentage
- **Counts/Sizes**: Processed vs total (bytes, packets, or items)
- **ETA**: Estimated time remaining in HH:MM:SS format
- **Completion time**: Time taken for each phase after completion

The ETA is calculated using:
```go
func calculateETA(processed, total int64, elapsed time.Duration) time.Duration {
    if processed == 0 {
        return 0
    }
    rate := float64(processed) / elapsed.Seconds()
    remaining := float64(total - processed)
    return time.Duration(remaining/rate) * time.Second
}
```

Progress updates use carriage return (`\r`) to update in place, avoiding log spam.

### Warning Threshold

If space savings fall below threshold (default 75%), show warning but still create the file:

```
$ mkvdup create --mkv video.mkv --source /data/wrong_source --output video.mkvdup

...

Deduplication Results:
  ┌─────────────────────────────────────────────────────────┐
  │  Original MKV size:          3.42 GB                    │
  │  Matched data (from source): 1.02 GB  (29.8%)           │
  │  Unique data (delta):        2.40 GB  (70.2%)           │
  │  Index overhead:             51.2 MB  (1.5%)            │
  │  ─────────────────────────────────────────────────────  │
  │  Dedup file size:            2.45 GB                    │
  │  Space savings:              970 MB   (28.4%)           │
  └─────────────────────────────────────────────────────────┘

  ⚠️  WARNING: Space savings (28.4%) below threshold (75%)
      This may indicate:
      - Wrong source directory (MKV not from this disc)
      - Source files modified after ripping
      - Transcoded MKV (not lossless remux)

      Use --warn-threshold to adjust the threshold, or
      --quiet to suppress this warning.

Proceeding with file creation despite low space savings...
```

Note: The warning threshold does NOT prevent file generation. It only warns the user
that deduplication may not be effective. The user can delete or regenerate files as needed.

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success (includes success with low space savings warning) |
| 1 | General error |
| 2 | Verification failed |
| 3 | Source directory not found or invalid |
| 4 | MKV file not found or invalid |

### Quick Probe Command

The `probe` command provides a fast way to identify which source directory an MKV file
likely came from. This is particularly useful for multi-disc DVD/Blu-ray sets where
you have multiple ISOs and need to match each MKV to its source.

**Use case:** You have 5 ISOs from a multi-disc set and 20 MKV files. Rather than
trying each combination with full dedup (which takes minutes per attempt), probe
can test all combinations in under a minute and show which ISOs likely match which MKVs.

**Algorithm:**

```
1. Parse MKV file (quick scan, not full parse)
2. Sample N packets from different positions:
   - 5 from first 10% of file
   - 10 from middle 80%
   - 5 from last 10%
3. For each source directory:
   a. Build source index (or use cached index)
   b. Look up each sampled packet hash
   c. Count matches
4. Report match percentages, sorted by likelihood
```

**Speed optimizations:**
- Only parse enough MKV structure to locate sample packets
- Sample ~20 packets total (not millions)
- Reuse source index across multiple MKV probes
- Target: <30 seconds per MKV against multiple sources

**Output interpretation:**
- 80-100% match: Very likely the correct source
- 40-80% match: Possible match (may be partial content or different encode settings)
- <40% match: Unlikely to be the source

**Note:** Probe finds the best *candidate* - the actual `create` command does the
definitive matching. A high probe score doesn't guarantee perfect dedup, but a low
score almost certainly means wrong source.

## Implementation Order

### Step 1: Core Libraries
1. Source type detection (DVD vs Blu-ray)
2. Source indexer (hash table builder)
3. EBML/MKV parser
4. Matcher algorithm

### Step 2: File I/O
5. Dedup file writer (unified format)
6. Dedup file reader

### Step 3: Verification
7. FUSE mount (minimal, for verification)
8. Byte-by-byte verification

### Step 4: Full FUSE
9. Config file parser
10. Multi-file FUSE mount with directory structure

### Step 5: Polish
11. Progress reporting
12. Parallel processing
13. Error handling and recovery
14. Integrity checks (source file checksums)

## Development Guidelines

Follow standard Go best practices throughout development:

### Code Style
- Run `gofmt` or `goimports` on all code before committing
- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Use `golint` and `go vet` to catch common issues
- Keep functions focused and reasonably sized
- Use meaningful variable and function names

### Testing
- Write tests alongside implementation (test-driven when practical)
- Use table-driven tests for cases with multiple inputs
- Aim for high test coverage on critical paths (matching, file format, FUSE reads)
- Use `go test -race` to detect data races in concurrent code
- Integration tests should use temporary directories and clean up after themselves

### Error Handling
- Return errors rather than panicking (except for truly unrecoverable situations)
- Wrap errors with context using `fmt.Errorf("context: %w", err)`
- Log errors at appropriate levels (debug, info, warn, error)

### Documentation
- Document exported functions and types with godoc comments
- Keep comments current when code changes
- Complex algorithms should have explanatory comments

### Dependencies
- Prefer standard library when sufficient
- Vet third-party dependencies for maintenance status and security
- Use Go modules for dependency management
- Pin dependency versions in go.mod

### Performance
- Profile before optimizing (`go test -bench`, `pprof`)
- Avoid premature optimization
- Memory-map large files rather than reading into memory
- Use sync.Pool for frequently allocated buffers

## Technical Decisions

### Deduplication Performance Summary

With all filtering implemented (ES-aware indexing, user_data filtering, per-sub-stream audio):

| Metric | Before Filtering | After Filtering |
|--------|-----------------|-----------------|
| Video byte match rate | ~60% | ~98.6% |
| Audio byte match rate | ~25% | ~99%+ |
| Overall byte match rate | ~50% | **98.4%** |
| Delta size (3.4GB MKV) | ~1.7GB | ~56MB |
| Storage savings | ~50% | **97.8%** |

The remaining ~1.6% unmatched data consists of:
- MKV container headers (EBML, cluster headers, block headers)
- Subtitle data (not present in video/audio ES)
- Minor stream differences

### Hash Function
- **xxhash** (extremely fast, good distribution)
- 64-bit hash sufficient (collision probability negligible)

### Window Size for Hashing
- 64 bytes default (sufficient uniqueness for codec sync points)
- Trade-off: larger = fewer false positives, but more memory and potential boundary issues

### Index Granularity
- Per-sync-point (millions of entries) for maximum dedup
- Index is stored in the dedup file, not in memory during normal operation

### Memory Usage at Runtime

**Goal:** Keep memory footprint low even with many virtual files configured.

**Strategy: Lazy loading with reference counting**

Files are NOT memory-mapped at startup. Instead:

1. **On file open (FUSE Open):**
   - Increment reference count for that virtual file
   - If first open: memory-map the dedup file and source files
   - Track open file handles

2. **On file close (FUSE Release):**
   - Decrement reference count
   - If reference count reaches 0: unmap all files for that virtual file

3. **Data structures:**

```go
type VirtualFileState struct {
    mu          sync.RWMutex
    refCount    int32
    dedupMmap   *mmap.ReaderAt  // nil when not in use
    sourceMmaps []*mmap.ReaderAt // nil when not in use
    index       *DedupIndex      // parsed from dedupMmap, nil when not in use
}

func (v *VirtualFileState) Open() error {
    v.mu.Lock()
    defer v.mu.Unlock()

    v.refCount++
    if v.refCount == 1 {
        // First open - load everything
        return v.load()
    }
    return nil
}

func (v *VirtualFileState) Release() {
    v.mu.Lock()
    defer v.mu.Unlock()

    v.refCount--
    if v.refCount == 0 {
        // Last close - unload everything
        v.unload()
    }
}

func (v *VirtualFileState) load() error {
    var err error
    v.dedupMmap, err = mmap.Open(v.dedupPath)
    if err != nil {
        return err
    }

    // Parse index from mmap'd dedup file
    v.index, err = ParseDedupIndex(v.dedupMmap)
    if err != nil {
        v.dedupMmap.Close()
        return err
    }

    // Memory-map source files
    for _, sf := range v.index.SourceFiles {
        m, err := mmap.Open(filepath.Join(v.sourceDir, sf.RelativePath))
        if err != nil {
            v.unload()
            return err
        }
        v.sourceMmaps = append(v.sourceMmaps, m)
    }

    return nil
}

func (v *VirtualFileState) unload() {
    if v.dedupMmap != nil {
        v.dedupMmap.Close()
        v.dedupMmap = nil
    }
    for _, m := range v.sourceMmaps {
        if m != nil {
            m.Close()
        }
    }
    v.sourceMmaps = nil
    v.index = nil
}
```

**Result:**
- At startup: Only config parsed, no files mapped
- During playback: Only active files are mapped
- After playback: Files unmapped, memory returned to OS
- 100 configured videos = ~0 memory if none are playing

**Optional enhancement:** Add a configurable grace period before unmapping (e.g., keep mapped for 30 seconds after close in case user seeks back). This avoids repeated map/unmap for quick seeks.

### Dedup Reader Lazy Loading (Implemented)

For fast FUSE mount initialization, the dedup Reader supports two-phase loading:

**Phase 1 (On Mount):** `NewReaderLazy()` - Parses only the header (60 bytes)
- Gets original file size immediately (needed for `stat()`)
- Index entries NOT parsed yet (~750K entries = ~7 seconds avoided)
- Source files NOT memory-mapped yet

**Phase 2 (On First Read):** `loadEntries()` via `sync.Once`
- Thread-safe lazy loading on first `ReadAt()` call
- Parses all index entries
- Loads source files

```go
type Reader struct {
    // ... other fields ...
    header       *Header
    entries      []Entry
    entriesOnce  sync.Once   // Ensures entries loaded exactly once
    entriesErr   error       // Captures any error from lazy loading
}

func NewReaderLazy(dedupPath, sourceDir string) (*Reader, error) {
    // Only reads header - entries NOT parsed
    return parseHeaderOnly(dedupPath, sourceDir)
}

func (r *Reader) ReadAt(buf []byte, offset int64) (int, error) {
    // Load entries on first access (thread-safe)
    if err := r.loadEntries(); err != nil {
        return 0, fmt.Errorf("load entries: %w", err)
    }
    // ... read logic ...
}

func (r *Reader) loadEntries() error {
    r.entriesOnce.Do(func() {
        r.entriesErr = r.parseEntriesAndSources()
    })
    return r.entriesErr
}
```

**Result:**
- Mount time: ~0 seconds (was ~7+ seconds with 750K entries)
- First read: ~7 seconds (entry parsing deferred)
- Subsequent reads: Instant (entries already loaded)

This enables mounting thousands of dedup files without waiting for each one to parse its full index.

## Performance: Multi-threading

### FUSE Multi-threaded Operation

go-fuse supports multi-threaded operation for concurrent request handling:

```yaml
# In mount config
performance:
  threads: 0              # 0 = auto (NumCPU), or specify count
  read_ahead_kb: 128      # Kernel read-ahead buffer size
  max_background: 12      # Max background FUSE requests
  congestion_threshold: 9 # When to start throttling
```

**Benefits of multi-threading:**

| Scenario | Single-threaded | Multi-threaded |
|----------|-----------------|----------------|
| One video playing | OK | Same |
| Multiple videos playing | Serialized reads, stuttering | Parallel reads, smooth |
| Player read-ahead | Blocks other reads | Parallel prefetch |
| Multiple users | Poor | Good |
| Checksum verification | Blocks playback | Background, no impact |

**Implementation:**

```go
func NewDedupFS(config *MasterConfig) *DedupFS {
    fs := &DedupFS{
        config: config,
        files:  make(map[string]*VirtualFileState),
    }

    // Determine thread count
    threads := config.Performance.Threads
    if threads == 0 {
        threads = runtime.NumCPU()
    }

    return fs
}

func (fs *DedupFS) Mount(mountpoint string) error {
    opts := &fuse.MountOptions{
        AllowOther:        true,  // Allow other users to access
        MaxBackground:     fs.config.Performance.MaxBackground,
        CongestionThreshold: fs.config.Performance.CongestionThreshold,
        Name:              "mkvdup",
    }

    // Create server with specified thread count
    server, err := fuse.NewServer(fs, mountpoint, opts)
    if err != nil {
        return err
    }

    // Enable multi-threaded operation
    server.SetDebug(false)

    // Serve with multiple threads
    go server.Serve()

    return nil
}
```

**Thread safety requirements:**

```go
type VirtualFileState struct {
    mu sync.RWMutex  // Protects all fields below

    // Read-only after load (no lock needed for reads)
    index       *DedupIndex
    dedupMmap   *mmap.ReaderAt
    sourceMmaps []*mmap.ReaderAt

    // Mutable (needs lock)
    refCount    int32
    loadError   error
    lastAccess  time.Time
}

func (v *VirtualFileState) Read(dest []byte, offset int64) (int, error) {
    v.mu.RLock()
    defer v.mu.RUnlock()

    // Memory-mapped reads are inherently thread-safe
    // Index binary search is read-only, thread-safe
    // No writes during normal operation

    return v.readLocked(dest, offset)
}
```

**Tuning guidance:**

| Workload | Recommended threads | Notes |
|----------|---------------------|-------|
| Single user, local playback | 2-4 | Low overhead |
| Media server (Jellyfin/Plex) | NumCPU | Multiple streams |
| NAS with many users | NumCPU * 2 | I/O bound, more threads help |
| Low-power device | 1-2 | Reduce CPU usage |

## Error Handling

### Error Categories

| Category | Detection | Impact | Recovery |
|----------|-----------|--------|----------|
| Dedup file missing | File open fails | Virtual file unavailable | Return ENOENT |
| Dedup file corrupt | Checksum mismatch | Virtual file unavailable | Return EIO, log error |
| Source file missing | File open fails | Virtual file unavailable | Return EIO |
| Source file wrong size | Size check on open | Virtual file unavailable | Return EIO |
| Source file corrupt | Checksum (optional) | Bad data returned | Warn, return data anyway |
| Read beyond EOF | Offset check | Partial/no data | Return short read |
| Config file invalid | Parse error | Mount fails / reload fails | Log error, keep old config |

### Integrity Checking Strategy

**Fast checks (always performed):**

```go
func (v *VirtualFileState) load() error {
    // 1. Check dedup file exists and is readable
    dedupInfo, err := os.Stat(v.dedupPath)
    if err != nil {
        return fmt.Errorf("dedup file not found: %w", err)
    }

    // 2. Open and verify dedup file header
    v.dedupMmap, err = mmap.Open(v.dedupPath)
    if err != nil {
        return fmt.Errorf("cannot open dedup file: %w", err)
    }

    // 3. Verify magic number
    if !bytes.Equal(v.dedupMmap[:8], []byte("MKVDUP01")) {
        return errors.New("invalid dedup file: bad magic")
    }

    // 4. Parse header, verify file size matches expected
    header := parseHeader(v.dedupMmap)
    expectedSize := header.DeltaOffset + header.DeltaSize + 24 // +footer
    if dedupInfo.Size() != expectedSize {
        return fmt.Errorf("dedup file truncated: expected %d, got %d",
            expectedSize, dedupInfo.Size())
    }

    // 5. Verify index checksum (fast, ~50MB for large file)
    indexStart := 57 + sourceFileSectionSize
    indexEnd := header.DeltaOffset
    indexData := v.dedupMmap[indexStart:indexEnd]
    if xxhash.Sum64(indexData) != footer.IndexChecksum {
        return errors.New("dedup file corrupt: index checksum mismatch")
    }

    // 6. Check each source file exists and has correct size
    for i, sf := range header.SourceFiles {
        path := filepath.Join(v.sourceDir, sf.RelativePath)
        info, err := os.Stat(path)
        if err != nil {
            return fmt.Errorf("source file missing: %s: %w", sf.RelativePath, err)
        }
        if info.Size() != sf.FileSize {
            return fmt.Errorf("source file size mismatch: %s: expected %d, got %d",
                sf.RelativePath, sf.FileSize, info.Size())
        }
    }

    return nil
}
```

**Slow checks (optional, on-demand via CLI):**

```bash
# Full integrity check including source file checksums
mkvdup check --dedup /path/to/video.mkvdup --source /path/to/source

Checking dedup file integrity...
  ✓ Header valid
  ✓ Index checksum valid
  ✓ Delta checksum valid

Checking source files...
  Verifying VIDEO_TS/VTS_09_1.VOB (1.0 GB)...
    [████████████████████████████████████████] 100%
    ✓ Checksum valid
  Verifying VIDEO_TS/VTS_09_2.VOB (1.0 GB)...
    [████████████████████████████████████████] 100%
    ✓ Checksum valid
  ...

All checks passed.
```

### FUSE Error Responses

```go
func (f *VirtualFile) Read(dest []byte, offset int64) (fuse.ReadResult, fuse.Status) {
    // Ensure file is loaded
    if err := f.state.ensureLoaded(); err != nil {
        log.Errorf("Failed to load %s: %v", f.name, err)
        return nil, fuse.EIO
    }

    // Find index entries for this read
    entries, err := f.state.index.FindEntries(offset, len(dest))
    if err != nil {
        log.Errorf("Index lookup failed for %s at offset %d: %v", f.name, offset, err)
        return nil, fuse.EIO
    }

    // Read and stitch data
    var bytesRead int
    for _, entry := range entries {
        var data []byte
        var err error

        if entry.Source == 0 {
            // Read from delta section
            data, err = f.readDelta(entry)
        } else {
            // Read from source file
            data, err = f.readSource(entry)
        }

        if err != nil {
            log.Errorf("Read failed for %s: %v", f.name, err)
            // Return partial data if we have some, otherwise EIO
            if bytesRead > 0 {
                return fuse.ReadResultData(dest[:bytesRead]), fuse.OK
            }
            return nil, fuse.EIO
        }

        copy(dest[bytesRead:], data)
        bytesRead += len(data)
    }

    return fuse.ReadResultData(dest[:bytesRead]), fuse.OK
}
```

### Graceful Degradation

When errors occur for a specific virtual file, other files remain accessible:

```go
type VirtualFileState struct {
    // ... existing fields ...

    // Error state
    loadError     error     // Non-nil if file failed to load
    loadErrorTime time.Time // When error occurred
    errorCount    int       // Number of consecutive errors
}

func (fs *DedupFS) Getattr(name string, ...) fuse.Status {
    state := fs.files[name]
    if state == nil {
        return fuse.ENOENT
    }

    // If file has persistent error, report it as inaccessible
    if state.loadError != nil {
        // Retry after cooldown period
        if time.Since(state.loadErrorTime) > 5*time.Minute {
            state.loadError = nil
        } else {
            return fuse.EIO
        }
    }

    return fuse.OK
}
```

### Logging and Monitoring

```go
// Structured logging for errors
type ErrorEvent struct {
    Timestamp  time.Time `json:"timestamp"`
    Level      string    `json:"level"`
    File       string    `json:"file,omitempty"`
    SourceFile string    `json:"source_file,omitempty"`
    Error      string    `json:"error"`
    Offset     int64     `json:"offset,omitempty"`
}

// Log levels
// - ERROR: File unavailable, data loss possible
// - WARN:  Degraded operation, checksum mismatch in non-critical path
// - INFO:  Successful recovery, retry succeeded
```

### User-Facing Error Messages

**On mount failure:**
```
$ mkvdup mount --config /etc/mkvdup/mount.yaml

Error loading virtual file "Videos/Video1.mkv":
  Source file missing: /data/sources/Video1_DVD/VIDEO_TS/VTS_09_1.VOB

  Possible causes:
  - Source directory moved or renamed
  - External drive not mounted
  - Files deleted

  This file will be unavailable. Other files will still be mounted.
  Fix the issue and run 'mkvdup reload' to retry.

Mounted 47/48 virtual files at /mnt/media
```

**On read error (in system log):**
```
mkvdup[1234]: ERROR file="Videos/Video1.mkv" error="source file read failed" \
    source="VIDEO_TS/VTS_09_1.VOB" offset=463519577 errno=EIO
```

### Periodic Health Checks

Optional background health monitoring:

```yaml
# In mount config
health_check:
  enabled: true
  interval: 1h           # Check every hour
  check_source_sizes: true
  check_source_checksums: false  # Too slow for routine checks
  on_error: warn         # warn, disable_file, or unmount
```

### Source File Change Watching

Optional inotify-based monitoring of source files:

```yaml
# In mount config
source_watch:
  enabled: true
  on_change: checksum    # checksum, disable_file, or warn
  checksum_threads: 2    # Parallel checksum workers for background verification
```

**Implementation:**

```go
type SourceWatcher struct {
    watcher     *fsnotify.Watcher
    fs          *DedupFS
    checksumCh  chan checksumJob
    workerCount int

    // Deduplication: map source path -> list of virtual files using it
    mu                sync.RWMutex
    sourceToVirtuals  map[string][]*VirtualFile  // source file -> virtual files
    watchedDirs       map[string]bool            // directories already watched
    pendingChecksums  map[string]bool            // source files with pending checksum jobs
}

func (sw *SourceWatcher) Start() error {
    sw.sourceToVirtuals = make(map[string][]*VirtualFile)
    sw.watchedDirs = make(map[string]bool)
    sw.pendingChecksums = make(map[string]bool)

    // Start checksum worker pool
    for i := 0; i < sw.workerCount; i++ {
        go sw.checksumWorker()
    }

    // Build reverse mapping: source file -> virtual files
    // This also deduplicates watches (each source dir watched only once)
    for _, vf := range sw.fs.config.VirtualFiles {
        sw.registerVirtualFile(vf)
    }

    log.Infof("Source watcher: watching %d unique directories for %d virtual files",
        len(sw.watchedDirs), len(sw.fs.config.VirtualFiles))

    // Handle events
    go func() {
        for {
            select {
            case event := <-sw.watcher.Events:
                if event.Op&(fsnotify.Write|fsnotify.Remove|fsnotify.Rename) != 0 {
                    sw.handleSourceChange(event.Name)
                }
            case err := <-sw.watcher.Errors:
                log.Errorf("Source watcher error: %v", err)
            }
        }
    }()

    return nil
}

// registerVirtualFile adds a virtual file to the watch system, deduplicating watches
func (sw *SourceWatcher) registerVirtualFile(vf *VirtualFile) {
    sw.mu.Lock()
    defer sw.mu.Unlock()

    sourceDir := vf.SourceDir

    // Add to reverse mapping for each source file this virtual file uses
    for _, sf := range vf.SourceFiles {
        fullPath := filepath.Join(sourceDir, sf.RelativePath)
        sw.sourceToVirtuals[fullPath] = append(sw.sourceToVirtuals[fullPath], vf)
    }

    // Only add watch if we haven't already watched this directory
    if !sw.watchedDirs[sourceDir] {
        sw.watchedDirs[sourceDir] = true

        // Add recursive watch on source directory
        filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
            if err != nil {
                return nil // Skip inaccessible paths
            }
            if info.IsDir() && !sw.watchedDirs[path] {
                sw.watchedDirs[path] = true
                sw.watcher.Add(path)
            }
            return nil
        })

        log.Debugf("Added watch on source directory: %s", sourceDir)
    }
}

func (sw *SourceWatcher) handleSourceChange(path string) {
    sw.mu.RLock()
    affected := sw.sourceToVirtuals[path]
    sw.mu.RUnlock()

    if len(affected) == 0 {
        return // Not a tracked source file
    }

    // Log once per source file, listing all affected virtual files
    affectedNames := make([]string, len(affected))
    for i, vf := range affected {
        affectedNames[i] = vf.Name
    }
    log.Warnf("Source file changed: %s (affects %d virtual files: %v)",
        path, len(affected), affectedNames)

    switch sw.fs.config.SourceWatch.OnChange {
    case "warn":
        // Just log the warning (already done above)

    case "disable_file":
        // Mark all affected virtual files as unavailable
        for _, vf := range affected {
            sw.fs.disableVirtualFile(vf.Name, "source file modified")
        }

    case "checksum":
        // Queue ONE checksum job per source file (not per virtual file)
        sw.mu.Lock()
        alreadyPending := sw.pendingChecksums[path]
        if !alreadyPending {
            sw.pendingChecksums[path] = true
        }
        sw.mu.Unlock()

        if !alreadyPending {
            sw.checksumCh <- checksumJob{
                sourcePath:      path,
                affectedVirtuals: affected,
            }
        }
    }
}

type checksumJob struct {
    sourcePath       string
    affectedVirtuals []*VirtualFile
}

func (sw *SourceWatcher) checksumWorker() {
    for job := range sw.checksumCh {
        log.Infof("Verifying checksum for %s (affects %d virtual files)",
            job.sourcePath, len(job.affectedVirtuals))

        // Checksum the source file ONCE
        valid, err := verifySourceFileChecksum(job.sourcePath, job.affectedVirtuals[0])

        // Clear pending flag
        sw.mu.Lock()
        delete(sw.pendingChecksums, job.sourcePath)
        sw.mu.Unlock()

        if err != nil || !valid {
            log.Errorf("Checksum mismatch for %s: %v", job.sourcePath, err)
            // Disable ALL virtual files that depend on this source
            for _, vf := range job.affectedVirtuals {
                sw.fs.disableVirtualFile(vf.Name, "checksum mismatch after modification")
                sw.fs.notifyFileRemoved(vf.Name)
            }
        } else {
            log.Infof("Checksum verified OK for %s", job.sourcePath)
        }
    }
}
```

**Deduplication benefits:**

| Scenario | Without dedup | With dedup |
|----------|---------------|------------|
| 10 episodes from same ISO | 10 watches on ISO | 1 watch on ISO |
| Source file changes | 10 checksum jobs queued | 1 checksum job queued |
| inotify watch count | O(virtual files) | O(unique source dirs) |

**Caveats:**
- inotify has per-user watch limits (default ~8192, tunable via `/proc/sys/fs/inotify/max_user_watches`)
- Doesn't work on network filesystems (NFS/SMB) - fall back to periodic polling
- Large ISOs are single files, so only one watch needed per ISO
- When config is reloaded, the watcher must update its mappings accordingly

## Testing Strategy

### Unit Tests

1. **Parser tests** - EBML/MKV parser, source indexer, hash functions
2. **Index tests** - Binary search, entry lookup, boundary conditions
3. **Permissions store tests** - Get/Set/Cleanup operations
4. **Boundary expansion tests:**
   - `TestExpandMatch_ForwardOnly` - expansion only extends forward
   - `TestExpandMatch_BackwardOnly` - expansion only extends backward
   - `TestExpandMatch_BothDirections` - expansion in both directions
   - `TestExpandMatch_AtFileBoundary` - doesn't expand past file start/end
   - `TestExpandMatch_MaxLimit` - respects maximum expansion limit
5. **Sync pattern detection tests:**
   - `TestFindVideoStartCodes` - detects MPEG-2, H.264, HEVC patterns
   - `TestFindAudioSyncPoints_AC3` - detects 0B 77 sync
   - `TestFindAudioSyncPoints_DTS` - detects 7F FE 80 01 sync
   - `TestFindAudioSyncPoints_TrueHD` - detects F8 72 6F BA sync
   - `TestFindAudioSyncPoints_MPEG` - detects FF Fx patterns
   - `TestFindSyncPoints_Overlapping` - handles adjacent sync points
6. **Config parsing tests:**
   - `TestLoadMasterConfig_Basic` - simple config loads
   - `TestLoadMasterConfig_WithIncludes` - include directives work
   - `TestLoadMasterConfig_GlobPatterns` - glob patterns expand correctly
   - `TestLoadMasterConfig_InvalidYAML` - graceful error handling
   - `TestLoadMasterConfig_MissingFile` - missing include file handling
7. **Dedup file format tests:**
   - `TestDedupHeader_Parse` - header parsing
   - `TestDedupHeader_Write` - header serialization
   - `TestDedupFooter_Checksum` - checksum validation
   - `TestDedupFile_Roundtrip` - write then read produces identical data

### Integration Tests

1. **End-to-end:** Create dedup → verify → compare SHA256 of reconstructed file
2. **FUSE playback:** Mount, play in VLC/mpv, verify no artifacts or stuttering
3. **Stress test:** Multiple concurrent reads, random seeks across file
4. **Concurrency tests:**
   - `TestConcurrentReads_SingleFile` - multiple goroutines reading same file
   - `TestConcurrentReads_MultipleFiles` - parallel reads across different files
   - `TestConcurrentOpenClose` - rapid open/close cycles
5. **Lazy loading tests:**
   - `TestLazyLoading_NoMmapAtStartup` - no source files mmap'd until opened
   - `TestLazyLoading_MmapOnOpen` - source files mmap'd when file opened
   - `TestLazyLoading_UnmapOnClose` - source files unmmap'd when refcount hits 0
   - `TestLazyLoading_GracePeriod` - optional grace period before unmap
6. **Graceful degradation tests:**
   - `TestGracefulDegradation_SingleFileError` - one file error doesn't affect others
   - `TestGracefulDegradation_ErrorRecovery` - file becomes accessible after fix
   - `TestGracefulDegradation_ErrorCooldown` - retry after cooldown period
7. **Health check tests:**
   - `TestHealthCheck_DetectsSourceSizeChange` - detects truncated source
   - `TestHealthCheck_DetectsMissingSource` - detects deleted source file
   - `TestHealthCheck_IntervalRespected` - checks run at configured interval
8. **inotify event emission tests:**
   - `TestInotify_CreateOnAdd` - IN_CREATE when file added via SIGHUP
   - `TestInotify_DeleteOnRemove` - IN_DELETE when file removed via SIGHUP
9. **Source watch with config reload:**
   - `TestSourceWatch_ConfigReload_AddFile` - new file's source added to watch
   - `TestSourceWatch_ConfigReload_RemoveFile` - removed file's exclusive sources unwatched
   - `TestSourceWatch_ConfigReload_SharedSource` - shared source stays watched

### SIGHUP Reload Tests (Required)

```go
func TestSIGHUP_ConfigReload(t *testing.T) {
    // Setup: mount with initial config containing 2 virtual files
    // Action: modify config to add a 3rd file, send SIGHUP
    // Verify: new file appears in mount, old files still accessible
}

func TestSIGHUP_ConfigRemoveFile(t *testing.T) {
    // Setup: mount with 3 virtual files
    // Action: modify config to remove 1 file, send SIGHUP
    // Verify: removed file no longer accessible, others still work
    // Verify: permissions for removed file cleaned up
}

func TestSIGHUP_PermissionsReload(t *testing.T) {
    // Setup: mount, chmod a file to 0600
    // Action: manually edit permissions.yaml to change mode to 0644, send SIGHUP
    // Verify: stat() returns new mode 0644
}

func TestSIGHUP_PermissionsCleanup(t *testing.T) {
    // Setup: mount with 2 files, chmod both
    // Action: remove one file from config, send SIGHUP
    // Verify: permissions.yaml no longer contains entry for removed file
}

func TestMount_PermissionsCleanup(t *testing.T) {
    // Setup: permissions.yaml has entries for files A, B, C
    //        config only has files A, B (C was removed previously)
    // Action: mount the filesystem
    // Verify: permissions.yaml cleaned up, C's entry removed
}

func TestSIGHUP_DuringActiveRead(t *testing.T) {
    // Setup: mount, start reading a large file
    // Action: send SIGHUP mid-read
    // Verify: read completes successfully, no corruption
}
```

### Source File Watch Tests (Required)

```go
func TestSourceWatch_FileModified(t *testing.T) {
    // Setup: mount with source watching enabled
    // Action: modify a byte in the source ISO
    // Verify: inotify event detected
    // Verify: appropriate action taken (warn/disable/checksum based on config)
}

func TestSourceWatch_FileDeleted(t *testing.T) {
    // Setup: mount with source watching enabled
    // Action: delete (or rename) source file
    // Verify: virtual file becomes inaccessible with EIO
    // Verify: error logged
}

func TestSourceWatch_SharedSource(t *testing.T) {
    // Setup: mount with 3 virtual files sharing same ISO source
    // Action: modify the ISO
    // Verify: only ONE inotify watch exists for the ISO
    // Verify: all 3 virtual files affected
    // Verify: only ONE checksum job queued (not 3)
}

func TestSourceWatch_ChecksumVerification(t *testing.T) {
    // Setup: mount with on_change: checksum
    // Action: touch source file (modify mtime but not content)
    // Verify: checksum passes, file remains accessible
}

func TestSourceWatch_ChecksumFailure(t *testing.T) {
    // Setup: mount with on_change: checksum
    // Action: actually modify source file content
    // Verify: checksum fails
    // Verify: all dependent virtual files disabled
    // Verify: inotify DELETE events emitted for virtual files
}
```

### Edge Case Tests

1. **Empty files** - 0-byte MKV (should fail gracefully)
2. **Tiny files** - MKV smaller than signature size
3. **Corrupted sources** - Truncated ISO, bad sectors
4. **Missing sources** - Source deleted after dedup created
5. **Permissions edge cases** - chown as non-root, chmod to 0000
6. **Deduplication quality validation:**
   - `TestDeltaContainsOnlyContainerOverhead` - verify delta has no codec data
   - `TestAudioMatching_HighMatchRate` - audio tracks achieve >99% match rate
   - `TestVideoMatching_HighMatchRate` - video tracks achieve >99% match rate
7. **Source watch edge cases:**
   - `TestSourceWatch_RapidChanges` - debouncing when file modified multiple times quickly
   - `TestSourceWatch_NetworkFS` - graceful fallback when inotify unsupported
8. **Boundary conditions:**
   - `TestRead_AtExactFileEnd` - read ending exactly at EOF
   - `TestRead_PastFileEnd` - read starting past EOF returns empty
   - `TestRead_SpanningMultipleEntries` - single read crossing many index entries
   - `TestRead_ExactlyOneEntry` - read matches single index entry exactly

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| MKV parser bugs | Use well-tested library, extensive test suite |
| Hash collisions | Verify full match after hash lookup |
| Source files change | Store checksums, verify before use |
| FUSE performance | Memory-map files, optimize read path |
| Large index | Store in file, memory-map for access |
| Source dir moved | Config uses paths, user must update |

## Future Enhancements

1. **Network sources**
   - Mount ISO/Blu-ray from NAS via NFS/SMB
   - Stream source data over network

2. **Incremental updates**
   - If MKV is re-ripped with different settings
   - Reuse existing matches, only reprocess changed parts

3. **Source index caching**
   - Save source index to disk
   - Reuse when creating multiple dedup files from same source

4. **Web UI**
   - Browse virtual files
   - Monitor FUSE mount status
   - Create new dedup files
