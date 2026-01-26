// Package matcher provides the core deduplication logic for matching MKV packets to source files.
package matcher

import (
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
)

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
	Entries        []Entry // All entries covering the entire MKV file
	DeltaData      []byte  // Concatenated unique data (delta)
	MatchedBytes   int64   // Total bytes matched to source
	UnmatchedBytes int64   // Total bytes in delta
	MatchedPackets int     // Number of packets that matched
	TotalPackets   int     // Total number of packets processed
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
type Matcher struct {
	sourceIndex    *source.Index
	mkvMmap        *mmap.File
	mkvData        []byte // Zero-copy mmap'd MKV data
	mkvSize        int64
	windowSize     int
	matchedRegions []matchedRegion
	regionsMu      sync.Mutex  // Protects matchedRegions for concurrent access
	trackTypes     map[int]int // Map from track number to track type
	numWorkers     int         // Number of worker goroutines for parallel matching
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

	// Build track type map
	for _, t := range tracks {
		m.trackTypes[int(t.Number)] = t.Type
	}

	// Reset matched regions
	m.matchedRegions = nil

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
	result.Entries, result.DeltaData = m.buildEntries()

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
// Returns nil if no valid hashes could be extracted.
func ExtractProbeHashes(data []byte, isVideo bool, windowSize int) []ProbeHash {
	if len(data) < windowSize {
		return nil
	}

	var hashes []ProbeHash

	// Find sync points within the packet data
	var syncPoints []int
	if isVideo {
		syncPoints = source.FindVideoStartCodes(data)
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
	if readSize > 4096 {
		readSize = 4096 // Only need to check beginning for sync points
	}
	if readSize < int64(m.windowSize) {
		return false
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
		syncPoints = source.FindVideoStartCodes(data)
	} else {
		syncPoints = source.FindAudioSyncPoints(data)
	}

	// Try sync points first
	for _, syncOff := range syncPoints {
		if syncOff+m.windowSize > len(data) {
			continue
		}
		if m.tryMatchFromOffsetParallel(pkt, int64(syncOff), data[syncOff:], isVideo) {
			return true
		}
	}

	// Also try from packet start (in case it's already aligned)
	if m.tryMatchFromOffsetParallel(pkt, 0, data, isVideo) {
		return true
	}

	return false
}

// isRangeCoveredParallel is a thread-safe version of isRangeCovered.
func (m *Matcher) isRangeCoveredParallel(offset, size int64) bool {
	end := offset + size
	m.regionsMu.Lock()
	defer m.regionsMu.Unlock()
	for _, r := range m.matchedRegions {
		if r.mkvStart <= offset && r.mkvEnd >= end {
			return true
		}
	}
	return false
}

// tryMatchFromOffsetParallel is a thread-safe version of tryMatchFromOffset.
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

	// Try all locations and find the best match (longest expansion)
	var bestMatch *matchedRegion
	bestMatchLen := int64(0)

	for _, loc := range locations {
		// Skip locations that don't match the stream type (video vs audio)
		// This is important for ES-based indexes where video and audio
		// have separate offset spaces
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

	if bestMatch != nil {
		m.regionsMu.Lock()
		m.matchedRegions = append(m.matchedRegions, *bestMatch)
		m.regionsMu.Unlock()
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
		// For raw files, use file size
		if int(loc.FileIndex) < len(m.sourceIndex.Files) {
			srcSize = m.sourceIndex.Files[loc.FileIndex].Size
		}
	}

	// Expand backward (zero-copy single-byte reads from mmap'd data)
	// Track range hint across reads to avoid repeated binary searches
	backwardHint := -1
	backwardExpanded := int64(0)
	for mkvStart > 0 && srcStart > 0 && backwardExpanded < MaxExpansionBytes {
		// Zero-copy: direct byte access
		mkvByte := m.mkvData[mkvStart-1]

		var srcByteVal byte
		var ok bool
		if m.sourceIndex.UsesESOffsets {
			readLoc := source.Location{
				FileIndex:        loc.FileIndex,
				Offset:           srcStart - 1,
				IsVideo:          loc.IsVideo,
				AudioSubStreamID: loc.AudioSubStreamID,
			}
			srcByteVal, backwardHint, ok = m.sourceIndex.ReadESByteWithHint(readLoc, backwardHint)
			if !ok {
				break
			}
		} else {
			// Zero-copy for raw indexes
			srcByte := m.sourceIndex.RawSlice(source.Location{FileIndex: loc.FileIndex, Offset: srcStart - 1}, 1)
			if len(srcByte) == 0 {
				break
			}
			srcByteVal = srcByte[0]
		}

		if mkvByte != srcByteVal {
			break
		}

		mkvStart--
		srcStart--
		length++
		backwardExpanded++
	}

	// Expand forward (zero-copy single-byte reads from mmap'd data)
	// Track range hint across reads to avoid repeated binary searches
	forwardHint := -1
	mkvEnd := mkvOffset + initialLen
	srcEnd := loc.Offset + initialLen
	forwardExpanded := int64(0)
	for mkvEnd < m.mkvSize && srcEnd < srcSize && forwardExpanded < MaxExpansionBytes {
		// Zero-copy: direct byte access
		mkvByte := m.mkvData[mkvEnd]

		var srcByteVal byte
		var ok bool
		if m.sourceIndex.UsesESOffsets {
			readLoc := source.Location{
				FileIndex:        loc.FileIndex,
				Offset:           srcEnd,
				IsVideo:          loc.IsVideo,
				AudioSubStreamID: loc.AudioSubStreamID,
			}
			srcByteVal, forwardHint, ok = m.sourceIndex.ReadESByteWithHint(readLoc, forwardHint)
			if !ok {
				break
			}
		} else {
			// Zero-copy for raw indexes
			srcByte := m.sourceIndex.RawSlice(source.Location{FileIndex: loc.FileIndex, Offset: srcEnd}, 1)
			if len(srcByte) == 0 {
				break
			}
			srcByteVal = srcByte[0]
		}

		if mkvByte != srcByteVal {
			break
		}

		mkvEnd++
		srcEnd++
		length++
		forwardExpanded++
	}

	return mkvStart, srcStart, length
}

// mergeRegions merges overlapping matched regions.
func (m *Matcher) mergeRegions() {
	if len(m.matchedRegions) == 0 {
		return
	}

	// Sort by start offset
	sort.Slice(m.matchedRegions, func(i, j int) bool {
		return m.matchedRegions[i].mkvStart < m.matchedRegions[j].mkvStart
	})

	// Merge overlapping regions
	merged := []matchedRegion{m.matchedRegions[0]}
	for i := 1; i < len(m.matchedRegions); i++ {
		curr := m.matchedRegions[i]
		last := &merged[len(merged)-1]

		// Check for overlap
		if curr.mkvStart <= last.mkvEnd {
			// Overlapping - extend if needed (keep the longer one)
			if curr.mkvEnd > last.mkvEnd {
				// Current extends beyond last - need to decide which source to use
				// For simplicity, keep the one that covers more
				if (curr.mkvEnd - curr.mkvStart) > (last.mkvEnd - last.mkvStart) {
					*last = curr
				} else {
					last.mkvEnd = curr.mkvEnd
				}
			}
		} else {
			// No overlap - add new region
			merged = append(merged, curr)
		}
	}

	m.matchedRegions = merged
}

// buildEntries creates the final entry list and delta data.
func (m *Matcher) buildEntries() ([]Entry, []byte) {
	var entries []Entry
	var deltaData []byte
	deltaOffset := int64(0)

	// Start from beginning of file
	pos := int64(0)
	regionIdx := 0

	for pos < m.mkvSize {
		// Check if we're in a matched region
		var inRegion *matchedRegion
		if regionIdx < len(m.matchedRegions) && m.matchedRegions[regionIdx].mkvStart <= pos {
			inRegion = &m.matchedRegions[regionIdx]
		}

		if inRegion != nil && pos >= inRegion.mkvStart && pos < inRegion.mkvEnd {
			// We're in a matched region
			// Adjust source offset for our position within the region
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
			// We're in a gap (unmatched data)
			gapEnd := m.mkvSize
			if regionIdx < len(m.matchedRegions) {
				gapEnd = m.matchedRegions[regionIdx].mkvStart
			}
			gapLen := gapEnd - pos

			// Zero-copy: slice directly into mmap'd data for reading,
			// but we need to copy it to deltaData since that's returned to caller
			if gapEnd <= m.mkvSize {
				data := m.mkvData[pos:gapEnd]
				entries = append(entries, Entry{
					MkvOffset:    pos,
					Length:       gapLen,
					Source:       0,
					SourceOffset: deltaOffset,
				})
				deltaData = append(deltaData, data...)
				deltaOffset += gapLen
			}

			pos = gapEnd
		}
	}

	return entries, deltaData
}
