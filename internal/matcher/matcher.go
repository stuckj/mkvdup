// Package matcher provides the core deduplication logic for matching MKV packets to source files.
package matcher

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"sort"
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
	// Coverage bitmap for O(1) coverage checks. Each bit represents a chunk.
	// A chunk is marked covered when a matched region fully contains it.
	coveredChunks []uint64 // Bitmap: bit i = chunk i is covered
	coverageMu    sync.Mutex

	// Locality hint: shared across workers. Workers process near-sequential
	// packets so hints naturally stay roughly current. Stale values just mean
	// one wasted nearby search before falling back to the full location list.
	lastMatchFileIndex atomic.Uint32
	lastMatchOffset    atomic.Int64
	lastMatchValid     atomic.Bool
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
		numWorkers:  numWorkers,
	}, nil
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

	// Build track type and codec info maps
	for _, t := range tracks {
		m.trackTypes[int(t.Number)] = t.Type
		m.trackCodecs[int(t.Number)] = trackCodecInfo{
			trackType:     t.Type,
			nalLengthSize: detectNALLengthSize(t.CodecID, t.CodecPrivate),
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

	result := &Result{
		TotalPackets: len(packets),
	}

	// Use parallel processing with worker pool
	result.MatchedPackets = m.matchParallel(packets, progress)

	if progress != nil {
		progress(len(packets), len(packets))
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

// matchParallel processes packets in parallel using a worker pool.
func (m *Matcher) matchParallel(packets []mkv.Packet, progress ProgressFunc) int {
	var processedCount atomic.Int64
	var matchedCount atomic.Int64
	totalPackets := len(packets)

	// Create work channel
	workChan := make(chan mkv.Packet, m.numWorkers*2)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < m.numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pkt := range workChan {
				matched := m.matchPacketParallel(pkt)
				if matched {
					matchedCount.Add(1)
				}
				count := processedCount.Add(1)
				if progress != nil && count%1000 == 0 {
					progress(int(count), totalPackets)
				}
			}
		}()
	}

	// Send work to workers
	for _, pkt := range packets {
		workChan <- pkt
	}
	close(workChan)

	// Wait for all workers to finish
	wg.Wait()

	return int(matchedCount.Load())
}

// matchPacketParallel is the thread-safe version of matchPacket.
func (m *Matcher) matchPacketParallel(pkt mkv.Packet) bool {
	// Check if this region is already covered by a matched region
	// Note: This is a relaxed check - we may miss some coverage due to race conditions,
	// but that's okay since we merge overlapping regions at the end anyway
	if m.isRangeCoveredParallel(pkt.Offset, pkt.Size) {
		return true
	}

	// Determine if this is video or audio
	trackType := m.trackTypes[int(pkt.TrackNum)]
	isVideo := trackType == mkv.TrackTypeVideo

	// Read packet data to find sync points (zero-copy slice access)
	readSize := pkt.Size
	if readSize < int64(m.windowSize) {
		return false
	}

	// For AVCC/HVCC video, use the full packet data. AVCC parsing is O(num_NALs)
	// not O(packet_size) — it reads 4-byte length fields and jumps, touching only
	// ~20 bytes for a typical frame with 5 NALs. Without this, large frames with
	// multiple slice NALs (common in 1080p Blu-ray) only match the first slice
	// since subsequent slices start past the truncated window.
	// For audio and Annex B video (linear scan), cap at 4096 to avoid waste.
	var useFullPacket bool
	if isVideo {
		codecInfo := m.trackCodecs[int(pkt.TrackNum)]
		if codecInfo.nalLengthSize > 0 {
			useFullPacket = true
		}
	}
	if !useFullPacket && readSize > 4096 {
		readSize = 4096
	}

	// Zero-copy: slice directly into mmap'd data
	endOffset := pkt.Offset + readSize
	if endOffset > m.mkvSize {
		endOffset = m.mkvSize
	}
	data := m.mkvData[pkt.Offset:endOffset]
	if len(data) < m.windowSize {
		return false
	}

	// Find sync points within the packet data
	var syncPoints []int
	if isVideo {
		codecInfo := m.trackCodecs[int(pkt.TrackNum)]
		if codecInfo.nalLengthSize > 0 {
			// AVCC/HVCC format: parse length-prefixed NAL units
			syncPoints = source.FindAVCCNALStarts(data, codecInfo.nalLengthSize)
		} else {
			// Annex B format: find NAL starts after 00 00 01
			syncPoints = source.FindVideoNALStarts(data)
		}
	} else {
		syncPoints = source.FindAudioSyncPoints(data)
	}

	// For AVCC/HVCC video, each NAL unit has different framing bytes than the
	// source (length prefix vs start code), so expansion stops at NAL boundaries.
	// We must match each NAL individually to cover the full packet.
	// For Annex B video (nalLengthSize == 0), one match is sufficient since
	// expansion works correctly across start code boundaries.
	isAVCC := isVideo && m.trackCodecs[int(pkt.TrackNum)].nalLengthSize > 0
	anyMatched := false
	for _, syncOff := range syncPoints {
		if syncOff+m.windowSize > len(data) {
			continue
		}
		if m.tryMatchFromOffsetParallel(pkt, int64(syncOff), data[syncOff:], isVideo) {
			anyMatched = true
			// Early return for Annex B video (expansion covers full packet)
			// or AVCC when full packet is now covered
			if !isAVCC || m.isRangeCoveredParallel(pkt.Offset, pkt.Size) {
				return true
			}
		}
	}

	// Also try from packet start (in case it's already aligned)
	if !anyMatched {
		if m.tryMatchFromOffsetParallel(pkt, 0, data, isVideo) {
			anyMatched = true
		}
	}

	return anyMatched
}

// isRangeCoveredParallel checks if a range is likely covered using a coverage bitmap.
// This is an O(1) check using chunk-level granularity. It may have false positives
// (multiple regions covering different chunks) but that's acceptable since we merge
// overlapping regions at the end anyway.
func (m *Matcher) isRangeCoveredParallel(offset, size int64) bool {
	// Calculate chunk range
	startChunk := offset / coverageChunkSize
	endChunk := (offset + size - 1) / coverageChunkSize

	m.coverageMu.Lock()
	defer m.coverageMu.Unlock()

	// Check if all chunks in the range are covered
	for chunk := startChunk; chunk <= endChunk; chunk++ {
		wordIdx := chunk / 64
		bitIdx := uint(chunk % 64)
		if wordIdx >= int64(len(m.coveredChunks)) {
			return false
		}
		if m.coveredChunks[wordIdx]&(1<<bitIdx) == 0 {
			return false
		}
	}
	return true
}

// markChunksCovered marks the chunks fully contained within a region as covered.
func (m *Matcher) markChunksCovered(start, end int64) {
	// Only mark chunks that are fully contained within the region
	// First chunk that starts at or after 'start' and is fully contained
	firstFullChunk := (start + coverageChunkSize - 1) / coverageChunkSize
	// Last chunk that ends before 'end'
	lastFullChunk := (end / coverageChunkSize) - 1

	if firstFullChunk > lastFullChunk {
		// Region doesn't fully contain any chunks
		return
	}

	m.coverageMu.Lock()
	defer m.coverageMu.Unlock()

	for chunk := firstFullChunk; chunk <= lastFullChunk; chunk++ {
		wordIdx := chunk / 64
		bitIdx := uint(chunk % 64)
		if wordIdx < int64(len(m.coveredChunks)) {
			m.coveredChunks[wordIdx] |= 1 << bitIdx
		}
	}
}

// nearbyLocationIndices returns up to N indices into locations that are closest
// to hintOffset within the same file as hintFileIndex. Locations must be pre-sorted
// by (FileIndex, Offset) via SortLocationsByOffset. Returns nil if no locations
// are in the target file.
func nearbyLocationIndices(locations []source.Location, hintFileIndex uint16, hintOffset int64, maxCount int) []int {
	n := len(locations)
	if n == 0 {
		return nil
	}

	// Binary search for the insertion point of (hintFileIndex, hintOffset)
	lo, hi := 0, n
	for lo < hi {
		mid := lo + (hi-lo)/2
		loc := locations[mid]
		if loc.FileIndex < hintFileIndex || (loc.FileIndex == hintFileIndex && loc.Offset < hintOffset) {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	// lo is now the index of the first location >= (hintFileIndex, hintOffset)

	// Radiate outward from lo to collect the closest locations in the same file
	result := make([]int, 0, maxCount)
	left := lo - 1
	right := lo

	for len(result) < maxCount && (left >= 0 || right < n) {
		// Pick the closer of left and right candidates
		useLeft := false
		useRight := false

		leftOK := left >= 0 && locations[left].FileIndex == hintFileIndex
		rightOK := right < n && locations[right].FileIndex == hintFileIndex

		if leftOK && rightOK {
			leftDist := hintOffset - locations[left].Offset
			rightDist := locations[right].Offset - hintOffset
			if leftDist < 0 {
				leftDist = -leftDist
			}
			if rightDist < 0 {
				rightDist = -rightDist
			}
			if leftDist <= rightDist {
				useLeft = true
			} else {
				useRight = true
			}
		} else if leftOK {
			useLeft = true
		} else if rightOK {
			useRight = true
		} else {
			break // No more candidates in the target file
		}

		if useLeft {
			result = append(result, left)
			left--
		} else if useRight {
			result = append(result, right)
			right++
		}
	}

	return result
}

// tryMatchFromOffsetParallel is a thread-safe version of tryMatchFromOffset.
// Uses two-phase locality-aware matching:
//   - Phase 1: If a locality hint exists, try the closest locations first.
//     If any produces a match >= localityGoodMatchThreshold, accept immediately.
//   - Phase 2: Fall back to trying all remaining locations (handles scene changes,
//     chapter boundaries, multi-file sources).
func (m *Matcher) tryMatchFromOffsetParallel(pkt mkv.Packet, offsetInPacket int64, data []byte, isVideo bool) bool {
	if len(data) < m.windowSize {
		return false
	}

	window := data[:m.windowSize]
	hash := xxhash.Sum64(window)

	// Look up in source index (read-only, thread-safe)
	locations := m.sourceIndex.Lookup(hash)
	if len(locations) == 0 {
		return false
	}

	var bestMatch *matchedRegion
	bestMatchLen := int64(0)

	// Track which location indices were already tried in Phase 1
	tried := make([]bool, len(locations))

	// Phase 1: Locality-aware search — try nearby locations first
	if m.lastMatchValid.Load() && len(locations) > 1 {
		hintFile := uint16(m.lastMatchFileIndex.Load())
		hintOffset := m.lastMatchOffset.Load()

		nearby := nearbyLocationIndices(locations, hintFile, hintOffset, localityNearbyCount)
		for _, idx := range nearby {
			tried[idx] = true
			loc := locations[idx]

			if m.sourceIndex.UsesESOffsets && loc.IsVideo != isVideo {
				continue
			}

			region := m.tryVerifyAndExpand(pkt, loc, offsetInPacket, isVideo)
			if region != nil {
				matchLen := region.mkvEnd - region.mkvStart
				if matchLen > bestMatchLen {
					bestMatch = region
					bestMatchLen = matchLen
				}
				// Short-circuit: a match this large is almost certainly the best
				if bestMatchLen >= localityGoodMatchThreshold {
					break
				}
			}
		}
	}

	// Phase 2: Full search of remaining locations (skip if Phase 1 found a good match)
	if bestMatchLen < localityGoodMatchThreshold {
		for i, loc := range locations {
			if tried[i] {
				continue
			}
			if m.sourceIndex.UsesESOffsets && loc.IsVideo != isVideo {
				continue
			}

			region := m.tryVerifyAndExpand(pkt, loc, offsetInPacket, isVideo)
			if region != nil {
				matchLen := region.mkvEnd - region.mkvStart
				if matchLen > bestMatchLen {
					bestMatch = region
					bestMatchLen = matchLen
				}
			}
		}
	}

	if bestMatch != nil {
		m.regionsMu.Lock()
		m.matchedRegions = append(m.matchedRegions, *bestMatch)
		m.regionsMu.Unlock()
		// Mark chunks as covered for fast coverage checks
		m.markChunksCovered(bestMatch.mkvStart, bestMatch.mkvEnd)
		// Update locality hint with midpoint of matched source region
		m.lastMatchFileIndex.Store(uint32(bestMatch.fileIndex))
		m.lastMatchOffset.Store(bestMatch.srcOffset + bestMatchLen/2)
		m.lastMatchValid.Store(true)
		return true
	}

	return false
}

// tryVerifyAndExpand attempts to verify and expand a match, returning the matched region or nil.
func (m *Matcher) tryVerifyAndExpand(pkt mkv.Packet, loc source.Location, offsetInPacket int64, isVideo bool) *matchedRegion {
	// The MKV offset where this sync point is
	mkvSyncOffset := pkt.Offset + offsetInPacket

	// Verify the initial match (at least windowSize bytes)
	verifyLen := int64(m.windowSize)
	remainingInPacket := pkt.Size - offsetInPacket
	if verifyLen > remainingInPacket {
		verifyLen = remainingInPacket
	}

	// Zero-copy: slice directly into mmap'd data
	endOffset := mkvSyncOffset + verifyLen
	if endOffset > m.mkvSize {
		return nil
	}
	mkvBuf := m.mkvData[mkvSyncOffset:endOffset]

	// Read source data - use ES reader for ES-based indexes, raw slice for zero-copy
	var srcBuf []byte
	var err error
	if m.sourceIndex.UsesESOffsets {
		srcBuf, err = m.sourceIndex.ReadESDataAt(loc, int(verifyLen))
		if err != nil || len(srcBuf) < int(verifyLen) {
			return nil
		}
	} else {
		// For raw indexes, use zero-copy slice
		srcBuf = m.sourceIndex.RawSlice(loc, int(verifyLen))
		if srcBuf == nil || len(srcBuf) < int(verifyLen) {
			return nil
		}
	}

	// Check if bytes match
	for i := range mkvBuf {
		if mkvBuf[i] != srcBuf[i] {
			return nil
		}
	}

	// Expand the match from the sync point
	mkvStart, srcStart, matchLen := m.expandMatch(
		mkvSyncOffset, loc, verifyLen,
	)

	return &matchedRegion{
		mkvStart:         mkvStart,
		mkvEnd:           mkvStart + matchLen,
		fileIndex:        loc.FileIndex,
		srcOffset:        srcStart,
		isVideo:          isVideo,
		audioSubStreamID: loc.AudioSubStreamID,
	}
}

// expandChunkSize is the number of bytes to read at once during match expansion.
// Larger chunks reduce page faults when expanding across mmap'd source files.
const expandChunkSize = 4096

// expandMatch expands a verified match in both directions.
func (m *Matcher) expandMatch(mkvOffset int64, loc source.Location, initialLen int64) (mkvStart, srcStart, length int64) {
	mkvStart = mkvOffset
	srcStart = loc.Offset
	length = initialLen

	// Get source size for bounds checking
	var srcSize int64
	if m.sourceIndex.UsesESOffsets && int(loc.FileIndex) < len(m.sourceIndex.ESReaders) {
		if loc.IsVideo {
			srcSize = m.sourceIndex.ESReaders[loc.FileIndex].TotalESSize(true)
		} else {
			srcSize = m.sourceIndex.ESReaders[loc.FileIndex].AudioSubStreamESSize(loc.AudioSubStreamID)
		}
	} else {
		if int(loc.FileIndex) < len(m.sourceIndex.Files) {
			srcSize = m.sourceIndex.Files[loc.FileIndex].Size
		}
	}

	if m.sourceIndex.UsesESOffsets {
		m.expandMatchES(mkvOffset, loc, srcSize, &mkvStart, &srcStart, &length)
	} else {
		m.expandMatchRaw(mkvOffset, loc, srcSize, &mkvStart, &srcStart, &length)
	}

	return mkvStart, srcStart, length
}

// expandMatchES expands a match using byte-by-byte ES reads with range hints.
// This is optimized for DVD MPEG-PS sources where ES data is non-contiguous.
func (m *Matcher) expandMatchES(mkvOffset int64, loc source.Location, srcSize int64, mkvStart, srcStart, length *int64) {
	// Expand backward
	backwardHint := -1
	backwardExpanded := int64(0)
	for *mkvStart > 0 && *srcStart > 0 && backwardExpanded < MaxExpansionBytes {
		mkvByte := m.mkvData[*mkvStart-1]
		readLoc := source.Location{
			FileIndex:        loc.FileIndex,
			Offset:           *srcStart - 1,
			IsVideo:          loc.IsVideo,
			AudioSubStreamID: loc.AudioSubStreamID,
		}
		srcByteVal, hint, ok := m.sourceIndex.ReadESByteWithHint(readLoc, backwardHint)
		backwardHint = hint
		if !ok || mkvByte != srcByteVal {
			break
		}
		*mkvStart--
		*srcStart--
		*length++
		backwardExpanded++
	}

	// Expand forward
	forwardHint := -1
	mkvEnd := *mkvStart + *length
	srcEnd := *srcStart + *length
	forwardExpanded := int64(0)
	for mkvEnd < m.mkvSize && srcEnd < srcSize && forwardExpanded < MaxExpansionBytes {
		mkvByte := m.mkvData[mkvEnd]
		readLoc := source.Location{
			FileIndex:        loc.FileIndex,
			Offset:           srcEnd,
			IsVideo:          loc.IsVideo,
			AudioSubStreamID: loc.AudioSubStreamID,
		}
		srcByteVal, hint, ok := m.sourceIndex.ReadESByteWithHint(readLoc, forwardHint)
		forwardHint = hint
		if !ok || mkvByte != srcByteVal {
			break
		}
		mkvEnd++
		srcEnd++
		*length++
		forwardExpanded++
	}
}

// expandMatchRaw expands a match using chunked reads from raw mmap'd source files.
// Reads 4KB chunks at a time to reduce page faults compared to byte-by-byte access.
func (m *Matcher) expandMatchRaw(mkvOffset int64, loc source.Location, srcSize int64, mkvStart, srcStart, length *int64) {
	// Expand backward in chunks
	backwardExpanded := int64(0)
	for *mkvStart > 0 && *srcStart > 0 && backwardExpanded < MaxExpansionBytes {
		// Determine chunk size
		chunkLen := int64(expandChunkSize)
		if chunkLen > *srcStart {
			chunkLen = *srcStart
		}
		if chunkLen > *mkvStart {
			chunkLen = *mkvStart
		}
		if chunkLen > MaxExpansionBytes-backwardExpanded {
			chunkLen = MaxExpansionBytes - backwardExpanded
		}
		if chunkLen <= 0 {
			break
		}

		srcChunk := m.sourceIndex.RawSlice(source.Location{
			FileIndex: loc.FileIndex,
			Offset:    *srcStart - chunkLen,
		}, int(chunkLen))
		if len(srcChunk) == 0 {
			break
		}

		// Compare backwards through the chunk
		mkvChunkStart := *mkvStart - int64(len(srcChunk))
		matched := int64(0)
		for i := len(srcChunk) - 1; i >= 0; i-- {
			if srcChunk[i] != m.mkvData[mkvChunkStart+int64(i)] {
				break
			}
			matched++
		}

		if matched == 0 {
			break
		}

		*mkvStart -= matched
		*srcStart -= matched
		*length += matched
		backwardExpanded += matched

		if matched < int64(len(srcChunk)) {
			break
		}
	}

	// Expand forward in chunks
	mkvEnd := *mkvStart + *length
	srcEnd := *srcStart + *length
	forwardExpanded := int64(0)
	for mkvEnd < m.mkvSize && srcEnd < srcSize && forwardExpanded < MaxExpansionBytes {
		chunkLen := int64(expandChunkSize)
		if chunkLen > srcSize-srcEnd {
			chunkLen = srcSize - srcEnd
		}
		if chunkLen > m.mkvSize-mkvEnd {
			chunkLen = m.mkvSize - mkvEnd
		}
		if chunkLen > MaxExpansionBytes-forwardExpanded {
			chunkLen = MaxExpansionBytes - forwardExpanded
		}
		if chunkLen <= 0 {
			break
		}

		srcChunk := m.sourceIndex.RawSlice(source.Location{
			FileIndex: loc.FileIndex,
			Offset:    srcEnd,
		}, int(chunkLen))
		if len(srcChunk) == 0 {
			break
		}

		// Compare forward through the chunk
		matched := int64(0)
		for i := 0; i < len(srcChunk); i++ {
			if srcChunk[i] != m.mkvData[mkvEnd+int64(i)] {
				break
			}
			matched++
		}

		if matched == 0 {
			break
		}

		mkvEnd += matched
		srcEnd += matched
		*length += matched
		forwardExpanded += matched

		if matched < int64(len(srcChunk)) {
			break
		}
	}
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
