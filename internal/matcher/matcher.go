// Package matcher provides the core deduplication logic for matching MKV packets to source files.
package matcher

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/mmap"
	"github.com/stuckj/mkvdup/internal/source"
)

const (
	// MaxExpansionBytes is the maximum number of bytes to expand a match in each direction.
	// Set high to allow matching entire video keyframes which can be several MB.
	MaxExpansionBytes = 16 * 1024 * 1024 // 16MB

	// localityNearbyCount is the max number of nearby locations to try in Phase 1
	// of locality-aware matching before falling back to a full search.
	localityNearbyCount = 8

	// localityGoodMatchThreshold is the minimum match length (bytes) to accept
	// from a nearby location without trying all remaining locations.
	// At 4KB (64x the 64-byte window), a false positive is vanishingly unlikely.
	localityGoodMatchThreshold = 4096
)

// detectNALLengthSize determines the NAL unit length field size from an MKV track's
// codec ID and codec private data. Returns 0 for Annex B (start code) formats,
// or the length field size (1, 2, or 4) for AVCC/HVCC formats.
func detectNALLengthSize(codecID string, codecPrivate []byte) int {
	switch codecID {
	case "V_MPEG4/ISO/AVC":
		// AVCC format: CodecPrivate is AVCDecoderConfigurationRecord
		// Byte 4 bits 0-1 = NAL length size - 1
		if len(codecPrivate) >= 7 && codecPrivate[0] == 1 {
			return int(codecPrivate[4]&0x03) + 1
		}
		return 4 // Default for AVC if CodecPrivate is missing or malformed
	case "V_MPEGH/ISO/HEVC":
		// HVCC format: CodecPrivate is HEVCDecoderConfigurationRecord
		// Byte 0 = configurationVersion (must be 1)
		// Byte 21 bits 6-7 = reserved (must be 111111)
		// Byte 21 bits 0-1 = NAL length size - 1
		if len(codecPrivate) >= 23 && codecPrivate[0] == 1 {
			b := codecPrivate[21]
			// Upper 6 bits must be all 1s per ISO/IEC 23008-2
			if b&0xFC == 0xFC {
				size := int(b&0x03) + 1
				// Valid NAL length sizes are 1, 2, or 4 bytes
				if size == 1 || size == 2 || size == 4 {
					return size
				}
			}
		}
		return 4 // Default for HEVC if CodecPrivate is missing or malformed
	default:
		return 0 // Annex B format (MPEG-2, etc.)
	}
}

// NALLengthSizeForTrack returns the NAL length size for a track, suitable for
// use by external callers like ExtractProbeHashes. Returns 0 for Annex B.
func NALLengthSizeForTrack(codecID string, codecPrivate []byte) int {
	return detectNALLengthSize(codecID, codecPrivate)
}

// Entry represents a region in the MKV file and where its data comes from.
type Entry struct {
	MkvOffset        int64  // Start offset in the MKV file
	Length           int64  // Length of this region
	Source           uint16 // 0 = delta, 1+ = source file index + 1 (supports up to 65535 files)
	SourceOffset     int64  // Offset in source file (or ES offset for ES-based sources)
	IsVideo          bool   // For ES-based sources: whether this is video or audio data
	AudioSubStreamID byte   // For ES-based audio: sub-stream ID (0x80-0x87=AC3, etc.)
}

// Result contains the results of the matching process.
type Result struct {
	Entries        []Entry      // All entries covering the entire MKV file
	DeltaData      []byte       // Concatenated unique data (for small deltas / tests)
	DeltaFile      *DeltaWriter // File-backed delta data (for large files)
	MatchedBytes   int64        // Total bytes matched to source
	UnmatchedBytes int64        // Total bytes in delta
	MatchedPackets int          // Number of packets that matched
	TotalPackets   int          // Total number of packets processed
}

// DeltaSize returns the total size of delta data.
func (r *Result) DeltaSize() int64 {
	if r.DeltaFile != nil {
		return r.DeltaFile.Size()
	}
	return int64(len(r.DeltaData))
}

// Close cleans up resources held by the result (temp files).
func (r *Result) Close() {
	if r.DeltaFile != nil {
		r.DeltaFile.Close()
		r.DeltaFile = nil
	}
}

// DeltaWriter writes delta data to a temp file to avoid heap accumulation.
type DeltaWriter struct {
	file     *os.File
	buffered *bufio.Writer
	size     int64
}

// NewDeltaWriter creates a DeltaWriter backed by a temp file.
func NewDeltaWriter() (*DeltaWriter, error) {
	f, err := os.CreateTemp("", "mkvdup-delta-*")
	if err != nil {
		return nil, fmt.Errorf("create delta temp file: %w", err)
	}
	return &DeltaWriter{
		file:     f,
		buffered: bufio.NewWriterSize(f, 256*1024),
	}, nil
}

// Write appends data to the delta file.
func (dw *DeltaWriter) Write(data []byte) error {
	n, err := dw.buffered.Write(data)
	dw.size += int64(n)
	return err
}

// Flush ensures all buffered data is written to disk.
func (dw *DeltaWriter) Flush() error {
	return dw.buffered.Flush()
}

// Size returns the total bytes written.
func (dw *DeltaWriter) Size() int64 {
	return dw.size
}

// File returns the underlying file for reading. Must call Flush() first.
func (dw *DeltaWriter) File() *os.File {
	return dw.file
}

// Close removes the temp file.
func (dw *DeltaWriter) Close() {
	if dw.file != nil {
		name := dw.file.Name()
		dw.file.Close()
		os.Remove(name)
		dw.file = nil
	}
}

// matchedRegion tracks a region that was matched to a source.
type matchedRegion struct {
	mkvStart         int64
	mkvEnd           int64
	fileIndex        uint16
	srcOffset        int64 // File offset or ES offset depending on source type
	isVideo          bool  // For ES-based sources
	audioSubStreamID byte  // For audio in MPEG-PS
}

// Matcher performs the deduplication matching.
// coverageChunkSize is the granularity for coverage tracking.
// Smaller values give more accurate coverage checks but use more memory.
const coverageChunkSize = 4096 // 4KB chunks

// trackCodecInfo stores per-track codec information for format-aware matching.
type trackCodecInfo struct {
	trackType     int
	nalLengthSize int // 0 = Annex B (start codes), 1/2/4 = AVCC/HVCC (length-prefixed NAL units)
}

type Matcher struct {
	sourceIndex    *source.Index
	mkvMmap        *mmap.File
	mkvData        []byte // Zero-copy mmap'd MKV data
	mkvSize        int64
	windowSize     int
	matchedRegions []matchedRegion
	regionsMu      sync.Mutex             // Protects matchedRegions for concurrent access
	trackTypes     map[int]int            // Map from track number to track type
	trackCodecs    map[int]trackCodecInfo // Map from track number to codec info
	numWorkers     int                    // Number of worker goroutines for parallel matching
	verbose        bool                   // Enable diagnostic output
	isAVCTrack     map[int]bool           // Per-track: whether this track uses H.264 NAL types
	// Coverage bitmap for O(1) coverage checks. Each bit represents a chunk.
	// A chunk is marked covered when a matched region fully contains it.
	coveredChunks []uint64 // Bitmap: bit i = chunk i is covered
	coverageMu    sync.RWMutex

	// Locality hint: shared across workers. Workers process near-sequential
	// packets so hints naturally stay roughly current. Stale values just mean
	// one wasted nearby search before falling back to the full location list.
	lastMatchFileIndex atomic.Uint32
	lastMatchOffset    atomic.Int64
	lastMatchValid     atomic.Bool

	// Diagnostic counters for investigating match failures
	diagVideoPacketsTotal       atomic.Int64 // Total video packets processed
	diagVideoPacketsCoverage    atomic.Int64 // Video packets skipped (coverage check)
	diagVideoNALsTotal          atomic.Int64 // Total video NAL sync points tried
	diagVideoNALsTooSmall       atomic.Int64 // NALs where window didn't fit
	diagVideoNALsHashNotFound   atomic.Int64 // NALs where hash wasn't in index
	diagVideoNALsVerifyFailed   atomic.Int64 // NALs where hash found but all verifications failed
	diagVideoNALsAllSkipped     atomic.Int64 // NALs where hash found but all locations skipped (e.g. isVideo mismatch)
	diagVideoNALsMatched        atomic.Int64 // NALs successfully matched
	diagVideoNALsMatchedBytes   atomic.Int64 // Total bytes from matched video NALs
	diagVideoNALsSkippedIsVideo atomic.Int64 // Locations skipped due to isVideo mismatch
	// Per-NAL-type diagnostics (H.264 NAL type = first byte & 0x1F)
	diagNALTypeNotFound [32]atomic.Int64 // hash not found, by NAL type
	diagNALTypeMatched  [32]atomic.Int64 // matched, by NAL type
	diagNALTypeTotal    [32]atomic.Int64 // total attempted, by NAL type

	// NAL size bucket diagnostics (video only)
	// Buckets: 0=<64, 1=64-127, 2=128-1023, 3=1K-32K, 4=32K+
	diagNALSizeMatched   [5]atomic.Int64
	diagNALSizeUnmatched [5]atomic.Int64

	// First few hash-not-found examples for debugging
	diagExamplesMu     sync.Mutex
	diagExamplesCount  int
	diagExamplesOutput []string
}

// nalSizeBucket returns the bucket index for a NAL size.
// Buckets: 0=<64, 1=64-127, 2=128-1023, 3=1K-32K, 4=32K+
func nalSizeBucket(size int) int {
	switch {
	case size < 64:
		return 0
	case size < 128:
		return 1
	case size < 1024:
		return 2
	case size < 32768:
		return 3
	default:
		return 4
	}
}

// NewMatcher creates a new Matcher with the given source index.
func NewMatcher(sourceIndex *source.Index) (*Matcher, error) {
	numWorkers := runtime.NumCPU() / 2
	if numWorkers < 1 {
		numWorkers = 1
	}
	return &Matcher{
		sourceIndex: sourceIndex,
		windowSize:  sourceIndex.WindowSize,
		trackTypes:  make(map[int]int),
		trackCodecs: make(map[int]trackCodecInfo),
		isAVCTrack:  make(map[int]bool),
		numWorkers:  numWorkers,
	}, nil
}

// SetVerbose enables or disables diagnostic output during matching.
func (m *Matcher) SetVerbose(v bool) {
	m.verbose = v
}

// SetNumWorkers sets the number of worker goroutines for parallel matching.
func (m *Matcher) SetNumWorkers(n int) {
	if n < 1 {
		n = 1
	}
	m.numWorkers = n
}

// Close releases resources.
func (m *Matcher) Close() error {
	if m.mkvMmap != nil {
		m.mkvMmap.Close()
	}
	return nil
}

// ProgressFunc is called to report matching progress.
type ProgressFunc func(processedPackets, totalPackets int)

// Match processes an MKV file and matches packets to the source.
func (m *Matcher) Match(mkvPath string, packets []mkv.Packet, tracks []mkv.Track, progress ProgressFunc) (*Result, error) {
	// Memory-map the MKV file for zero-copy access
	info, err := os.Stat(mkvPath)
	if err != nil {
		return nil, fmt.Errorf("stat MKV: %w", err)
	}
	m.mkvSize = info.Size()

	m.mkvMmap, err = mmap.Open(mkvPath)
	if err != nil {
		return nil, fmt.Errorf("mmap MKV: %w", err)
	}
	m.mkvData = m.mkvMmap.Data() // Store reference for zero-copy access

	// Reset per-run state in case Match() is called multiple times
	m.trackTypes = make(map[int]int)
	m.trackCodecs = make(map[int]trackCodecInfo)
	m.isAVCTrack = make(map[int]bool)
	m.diagVideoPacketsTotal.Store(0)
	m.diagVideoPacketsCoverage.Store(0)
	m.diagVideoNALsTotal.Store(0)
	m.diagVideoNALsTooSmall.Store(0)
	m.diagVideoNALsHashNotFound.Store(0)
	m.diagVideoNALsVerifyFailed.Store(0)
	m.diagVideoNALsAllSkipped.Store(0)
	m.diagVideoNALsMatched.Store(0)
	m.diagVideoNALsMatchedBytes.Store(0)
	m.diagVideoNALsSkippedIsVideo.Store(0)
	for i := range m.diagNALTypeNotFound {
		m.diagNALTypeNotFound[i].Store(0)
		m.diagNALTypeMatched[i].Store(0)
		m.diagNALTypeTotal[i].Store(0)
	}
	for i := range m.diagNALSizeMatched {
		m.diagNALSizeMatched[i].Store(0)
		m.diagNALSizeUnmatched[i].Store(0)
	}
	m.diagExamplesMu.Lock()
	m.diagExamplesCount = 0
	m.diagExamplesOutput = nil
	m.diagExamplesMu.Unlock()

	// Reset locality hint so matches from a previous MKV do not bias this run
	m.lastMatchFileIndex.Store(0)
	m.lastMatchOffset.Store(0)
	m.lastMatchValid.Store(false)

	// Build track type and codec info maps
	for _, t := range tracks {
		m.trackTypes[int(t.Number)] = t.Type
		nlSize := detectNALLengthSize(t.CodecID, t.CodecPrivate)
		m.trackCodecs[int(t.Number)] = trackCodecInfo{
			trackType:     t.Type,
			nalLengthSize: nlSize,
		}
		if t.Type == mkv.TrackTypeVideo && strings.HasPrefix(t.CodecID, "V_MPEG4/ISO/AVC") {
			m.isAVCTrack[int(t.Number)] = true
		}
	}

	// Reset matched regions with pre-allocated capacity
	// Most packets will match, so estimate capacity as number of packets
	m.matchedRegions = make([]matchedRegion, 0, len(packets))

	// Initialize coverage bitmap
	// Each uint64 holds 64 chunk bits, so we need (numChunks + 63) / 64 uint64s
	numChunks := (m.mkvSize + coverageChunkSize - 1) / coverageChunkSize
	m.coveredChunks = make([]uint64, (numChunks+63)/64)

	// Pre-sort source locations by offset to enable binary search for
	// locality-aware matching. One-time cost before concurrent access.
	m.sourceIndex.SortLocationsByOffset()

	// Set appropriate madvise hints for matching access patterns.
	m.sourceIndex.AdviseForMatching()

	result := &Result{
		TotalPackets: len(packets),
	}

	// Use parallel processing with worker pool
	result.MatchedPackets = m.matchParallel(packets, progress)

	if progress != nil {
		progress(len(packets), len(packets))
	}

	// Print diagnostic summary (verbose only)
	if m.verbose {
		fmt.Fprintf(os.Stderr, "\n=== Video Matching Diagnostics ===\n")
		fmt.Fprintf(os.Stderr, "Video packets total:        %d\n", m.diagVideoPacketsTotal.Load())
		fmt.Fprintf(os.Stderr, "Video packets skip-covered: %d\n", m.diagVideoPacketsCoverage.Load())
		fmt.Fprintf(os.Stderr, "Video NALs total:           %d\n", m.diagVideoNALsTotal.Load())
		fmt.Fprintf(os.Stderr, "Video NALs too small:       %d\n", m.diagVideoNALsTooSmall.Load())
		fmt.Fprintf(os.Stderr, "Video NALs hash not found:  %d\n", m.diagVideoNALsHashNotFound.Load())
		fmt.Fprintf(os.Stderr, "Video NALs verify failed:   %d\n", m.diagVideoNALsVerifyFailed.Load())
		fmt.Fprintf(os.Stderr, "Video NALs all skipped:     %d\n", m.diagVideoNALsAllSkipped.Load())
		fmt.Fprintf(os.Stderr, "Video NALs matched:         %d\n", m.diagVideoNALsMatched.Load())
		fmt.Fprintf(os.Stderr, "Video NALs matched bytes:   %d (%.2f MB)\n",
			m.diagVideoNALsMatchedBytes.Load(), float64(m.diagVideoNALsMatchedBytes.Load())/(1024*1024))
		fmt.Fprintf(os.Stderr, "Video NALs isVideo skips:   %d\n", m.diagVideoNALsSkippedIsVideo.Load())
		if len(m.isAVCTrack) > 0 {
			fmt.Fprintf(os.Stderr, "\nPer-NAL-type breakdown (H.264, type: total / matched / not_found / miss%%):\n")
			nalTypeNames := map[byte]string{
				1: "non-IDR slice", 2: "slice A", 3: "slice B", 4: "slice C",
				5: "IDR slice", 6: "SEI", 7: "SPS", 8: "PPS", 9: "AUD", 12: "filler",
			}
			for i := 0; i < 32; i++ {
				total := m.diagNALTypeTotal[i].Load()
				if total == 0 {
					continue
				}
				matched := m.diagNALTypeMatched[i].Load()
				notFound := m.diagNALTypeNotFound[i].Load()
				name := nalTypeNames[byte(i)]
				if name == "" {
					name = "other"
				}
				fmt.Fprintf(os.Stderr, "  type %2d (%14s): %8d / %8d / %8d (%.1f%% miss)\n",
					i, name, total, matched, notFound, float64(notFound)/float64(total)*100)
			}
		}
		// NAL size bucket breakdown
		nalSizeBucketNames := [5]string{"<64B", "64-127B", "128B-1KB", "1KB-32KB", "32KB+"}
		fmt.Fprintf(os.Stderr, "\nVideo NAL size distribution (matched / unmatched):\n")
		for i := 0; i < 5; i++ {
			matched := m.diagNALSizeMatched[i].Load()
			unmatched := m.diagNALSizeUnmatched[i].Load()
			if matched > 0 || unmatched > 0 {
				fmt.Fprintf(os.Stderr, "  %9s: %8d matched, %8d unmatched\n",
					nalSizeBucketNames[i], matched, unmatched)
			}
		}

		fmt.Fprintf(os.Stderr, "\nFirst hash-not-found examples:\n")
		for _, ex := range m.diagExamplesOutput {
			fmt.Fprintf(os.Stderr, "%s\n", ex)
		}
		fmt.Fprintf(os.Stderr, "=================================\n")
	}

	// Merge overlapping regions and build final entries
	m.mergeRegions()
	var buildErr error
	result.Entries, result.DeltaFile, buildErr = m.buildEntries()
	if buildErr != nil {
		return nil, fmt.Errorf("build entries: %w", buildErr)
	}

	// Calculate statistics
	for _, e := range result.Entries {
		if e.Source == 0 {
			result.UnmatchedBytes += e.Length
		} else {
			result.MatchedBytes += e.Length
		}
	}

	return result, nil
}

// ProbeHash represents a hash computed from a sync point in packet data.
type ProbeHash struct {
	Hash    uint64
	IsVideo bool
}

// ExtractProbeHashes extracts probe hashes from packet data using sync point detection.
// This is the same algorithm used by the matcher to find matching points.
// The data should be the first few KB of a packet (typically up to 4096 bytes).
// windowSize should match the source index window size (typically 64 bytes).
// nalLengthSize is 0 for Annex B video, or 1/2/4 for AVCC/HVCC video.
// Returns nil if no valid hashes could be extracted.
func ExtractProbeHashes(data []byte, isVideo bool, windowSize int, nalLengthSize int) []ProbeHash {
	if len(data) < windowSize {
		return nil
	}

	var hashes []ProbeHash

	// Find sync points within the packet data
	var syncPoints []int
	if isVideo {
		if nalLengthSize > 0 {
			syncPoints = source.FindAVCCNALStarts(data, nalLengthSize)
		} else {
			syncPoints = source.FindVideoNALStarts(data)
		}
	} else {
		syncPoints = source.FindAudioSyncPoints(data)
	}

	// Hash from sync points
	for _, syncOff := range syncPoints {
		if syncOff+windowSize > len(data) {
			continue
		}
		hash := xxhash.Sum64(data[syncOff : syncOff+windowSize])
		hashes = append(hashes, ProbeHash{
			Hash:    hash,
			IsVideo: isVideo,
		})
	}

	// If no sync points found, try from data start
	if len(hashes) == 0 {
		hash := xxhash.Sum64(data[:windowSize])
		hashes = append(hashes, ProbeHash{
			Hash:    hash,
			IsVideo: isVideo,
		})
	}

	return hashes
}

// mergeRegions merges overlapping matched regions.
// Regions from the same source with consistent offset mappings are merged into one.
// Overlapping regions from different sources (or inconsistent offsets) are clipped:
// the earlier region keeps its full range, the later region is trimmed to start
// after the earlier one ends.
func (m *Matcher) mergeRegions() {
	if len(m.matchedRegions) == 0 {
		return
	}

	// Sort by start offset
	sort.Slice(m.matchedRegions, func(i, j int) bool {
		return m.matchedRegions[i].mkvStart < m.matchedRegions[j].mkvStart
	})

	// Merge overlapping regions
	// Pre-allocate with capacity since merged will be at most len(matchedRegions)
	merged := make([]matchedRegion, 1, len(m.matchedRegions))
	merged[0] = m.matchedRegions[0]
	for i := 1; i < len(m.matchedRegions); i++ {
		curr := m.matchedRegions[i]
		last := &merged[len(merged)-1]

		if curr.mkvStart >= last.mkvEnd {
			// No overlap - add new region
			merged = append(merged, curr)
			continue
		}

		// Regions overlap. Check if they're from the same source with consistent
		// offset mapping, meaning the overlapping bytes map to the same source bytes.
		expectedSrcOffset := last.srcOffset + (curr.mkvStart - last.mkvStart)
		sameMapping := curr.fileIndex == last.fileIndex &&
			curr.srcOffset == expectedSrcOffset &&
			curr.isVideo == last.isVideo &&
			curr.audioSubStreamID == last.audioSubStreamID

		if sameMapping {
			// Same source, consistent mapping - safe to extend since both regions
			// were independently verified and the combined range maps correctly.
			if curr.mkvEnd > last.mkvEnd {
				last.mkvEnd = curr.mkvEnd
			}
		} else if curr.mkvEnd > last.mkvEnd {
			// Different source or inconsistent mapping. The earlier region (last)
			// keeps priority. Clip curr to start where last ends.
			overlap := last.mkvEnd - curr.mkvStart
			curr.mkvStart = last.mkvEnd
			curr.srcOffset += overlap
			// After clipping, curr may have zero or negative length if the overlap
			// equals or exceeds the original region size. Only keep valid regions.
			if curr.mkvStart < curr.mkvEnd {
				merged = append(merged, curr)
			}
		}
		// If curr is fully contained in last, drop it (nothing to add).
	}

	m.matchedRegions = merged
}

// buildEntries creates the final entry list and streams delta data to a temp file.
func (m *Matcher) buildEntries() ([]Entry, *DeltaWriter, error) {
	entries := make([]Entry, 0, len(m.matchedRegions)*2+1)

	deltaWriter, err := NewDeltaWriter()
	if err != nil {
		return nil, nil, err
	}

	deltaOffset := int64(0)
	pos := int64(0)
	regionIdx := 0

	for pos < m.mkvSize {
		var inRegion *matchedRegion
		if regionIdx < len(m.matchedRegions) && m.matchedRegions[regionIdx].mkvStart <= pos {
			inRegion = &m.matchedRegions[regionIdx]
		}

		if inRegion != nil && pos >= inRegion.mkvStart && pos < inRegion.mkvEnd {
			offsetInRegion := pos - inRegion.mkvStart
			regionLen := inRegion.mkvEnd - pos

			entries = append(entries, Entry{
				MkvOffset:        pos,
				Length:           regionLen,
				Source:           uint16(inRegion.fileIndex + 1),
				SourceOffset:     inRegion.srcOffset + offsetInRegion,
				IsVideo:          inRegion.isVideo,
				AudioSubStreamID: inRegion.audioSubStreamID,
			})

			pos = inRegion.mkvEnd
			regionIdx++
		} else {
			gapEnd := m.mkvSize
			if regionIdx < len(m.matchedRegions) {
				gapEnd = m.matchedRegions[regionIdx].mkvStart
			}
			gapLen := gapEnd - pos

			if gapEnd <= m.mkvSize {
				entries = append(entries, Entry{
					MkvOffset:    pos,
					Length:       gapLen,
					Source:       0,
					SourceOffset: deltaOffset,
				})
				// Write gap data directly from mmap to temp file
				if err := deltaWriter.Write(m.mkvData[pos:gapEnd]); err != nil {
					deltaWriter.Close()
					return nil, nil, fmt.Errorf("write delta: %w", err)
				}
				deltaOffset += gapLen
			}

			pos = gapEnd
		}
	}

	if err := deltaWriter.Flush(); err != nil {
		deltaWriter.Close()
		return nil, nil, fmt.Errorf("flush delta: %w", err)
	}

	return entries, deltaWriter, nil
}
