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
	"github.com/stuckj/mkvdup/internal/source"
	"golang.org/x/exp/mmap"
)

const (
	// MaxExpansionBytes is the maximum number of bytes to expand a match in each direction.
	// Set high to allow matching entire video keyframes which can be several MB.
	MaxExpansionBytes = 16 * 1024 * 1024 // 16MB
)

// Entry represents a region in the MKV file and where its data comes from.
type Entry struct {
	MkvOffset        int64 // Start offset in the MKV file
	Length           int64 // Length of this region
	Source           uint8 // 0 = delta, 1+ = source file index + 1
	SourceOffset     int64 // Offset in source file (or ES offset for ES-based sources)
	IsVideo          bool  // For ES-based sources: whether this is video or audio data
	AudioSubStreamID byte  // For ES-based audio: sub-stream ID (0x80-0x87=AC3, etc.)
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
	mkvMmap        *mmap.ReaderAt
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
	// Memory-map the MKV file
	info, err := os.Stat(mkvPath)
	if err != nil {
		return nil, fmt.Errorf("stat MKV: %w", err)
	}
	m.mkvSize = info.Size()

	m.mkvMmap, err = mmap.Open(mkvPath)
	if err != nil {
		return nil, fmt.Errorf("mmap MKV: %w", err)
	}

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
	if m.numWorkers > 1 {
		result.MatchedPackets = m.matchParallel(packets, progress)
	} else {
		// Single-threaded fallback
		for i, pkt := range packets {
			if progress != nil && i%1000 == 0 {
				progress(i, len(packets))
			}

			matched := m.matchPacket(pkt)
			if matched {
				result.MatchedPackets++
			}
		}
	}

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

	// Read packet data to find sync points
	readSize := pkt.Size
	if readSize > 4096 {
		readSize = 4096 // Only need to check beginning for sync points
	}
	if readSize < int64(m.windowSize) {
		return false
	}

	data := make([]byte, readSize)
	n, err := m.mkvMmap.ReadAt(data, pkt.Offset)
	if err != nil || n < m.windowSize {
		return false
	}

	// Find sync points within the packet data
	var syncPoints []int
	if isVideo {
		syncPoints = source.FindVideoStartCodes(data[:n])
	} else {
		syncPoints = source.FindAudioSyncPoints(data[:n])
	}

	// Try sync points first
	for _, syncOff := range syncPoints {
		if syncOff+m.windowSize > n {
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

// matchPacket attempts to match a single packet to the source.
func (m *Matcher) matchPacket(pkt mkv.Packet) bool {
	// Check if this region is already covered by a matched region
	if m.isRangeCovered(pkt.Offset, pkt.Size) {
		return true
	}

	// Determine if this is video or audio
	trackType := m.trackTypes[int(pkt.TrackNum)]
	isVideo := trackType == mkv.TrackTypeVideo

	// Read packet data to find sync points
	readSize := pkt.Size
	if readSize > 4096 {
		readSize = 4096 // Only need to check beginning for sync points
	}
	if readSize < int64(m.windowSize) {
		return false
	}

	data := make([]byte, readSize)
	n, err := m.mkvMmap.ReadAt(data, pkt.Offset)
	if err != nil || n < m.windowSize {
		return false
	}

	// Find sync points within the packet data
	var syncPoints []int
	if isVideo {
		syncPoints = source.FindVideoStartCodes(data[:n])
	} else {
		syncPoints = source.FindAudioSyncPoints(data[:n])
	}

	// Try sync points first
	for _, syncOff := range syncPoints {
		if syncOff+m.windowSize > n {
			continue
		}
		if m.tryMatchFromOffset(pkt, int64(syncOff), data[syncOff:], isVideo) {
			return true
		}
	}

	// Also try from packet start (in case it's already aligned)
	if m.tryMatchFromOffset(pkt, 0, data, isVideo) {
		return true
	}

	return false
}

// tryMatchFromOffset tries to match starting from a specific offset within a packet.
func (m *Matcher) tryMatchFromOffset(pkt mkv.Packet, offsetInPacket int64, data []byte, isVideo bool) bool {
	if len(data) < m.windowSize {
		return false
	}

	window := data[:m.windowSize]
	hash := xxhash.Sum64(window)

	// Look up in source index
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
		m.matchedRegions = append(m.matchedRegions, *bestMatch)
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

	mkvBuf := make([]byte, verifyLen)
	if _, err := m.mkvMmap.ReadAt(mkvBuf, mkvSyncOffset); err != nil {
		return nil
	}

	// Read source data - use ES reader for ES-based indexes, raw reader otherwise
	var srcBuf []byte
	var err error
	if m.sourceIndex.UsesESOffsets {
		srcBuf, err = m.sourceIndex.ReadESDataAt(loc, int(verifyLen))
	} else {
		// For raw indexes, read directly from the file
		srcBuf, err = m.sourceIndex.ReadRawDataAt(loc, int(verifyLen))
	}
	if err != nil || len(srcBuf) < int(verifyLen) {
		return nil
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

// verifyAndExpandFromSync verifies a match starting from a sync point within a packet.
// Deprecated: Use tryVerifyAndExpand instead for best-match selection.
func (m *Matcher) verifyAndExpandFromSync(pkt mkv.Packet, loc source.Location, offsetInPacket int64, isVideo bool) bool {
	region := m.tryVerifyAndExpand(pkt, loc, offsetInPacket, isVideo)
	if region != nil {
		m.matchedRegions = append(m.matchedRegions, *region)
		return true
	}
	return false
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

	// Expand backward
	backwardExpanded := int64(0)
	for mkvStart > 0 && srcStart > 0 && backwardExpanded < MaxExpansionBytes {
		var mkvByte [1]byte
		if _, err := m.mkvMmap.ReadAt(mkvByte[:], mkvStart-1); err != nil {
			break
		}

		var srcByte []byte
		var err error
		if m.sourceIndex.UsesESOffsets {
			readLoc := source.Location{
				FileIndex:        loc.FileIndex,
				Offset:           srcStart - 1,
				IsVideo:          loc.IsVideo,
				AudioSubStreamID: loc.AudioSubStreamID,
			}
			srcByte, err = m.sourceIndex.ReadESDataAt(readLoc, 1)
		} else {
			srcByte, err = m.sourceIndex.ReadRawDataAt(source.Location{FileIndex: loc.FileIndex, Offset: srcStart - 1}, 1)
		}
		if err != nil || len(srcByte) == 0 {
			break
		}

		if mkvByte[0] != srcByte[0] {
			break
		}

		mkvStart--
		srcStart--
		length++
		backwardExpanded++
	}

	// Expand forward
	mkvEnd := mkvOffset + initialLen
	srcEnd := loc.Offset + initialLen
	forwardExpanded := int64(0)
	for mkvEnd < m.mkvSize && srcEnd < srcSize && forwardExpanded < MaxExpansionBytes {
		var mkvByte [1]byte
		if _, err := m.mkvMmap.ReadAt(mkvByte[:], mkvEnd); err != nil {
			break
		}

		var srcByte []byte
		var err error
		if m.sourceIndex.UsesESOffsets {
			readLoc := source.Location{
				FileIndex:        loc.FileIndex,
				Offset:           srcEnd,
				IsVideo:          loc.IsVideo,
				AudioSubStreamID: loc.AudioSubStreamID,
			}
			srcByte, err = m.sourceIndex.ReadESDataAt(readLoc, 1)
		} else {
			srcByte, err = m.sourceIndex.ReadRawDataAt(source.Location{FileIndex: loc.FileIndex, Offset: srcEnd}, 1)
		}
		if err != nil || len(srcByte) == 0 {
			break
		}

		if mkvByte[0] != srcByte[0] {
			break
		}

		mkvEnd++
		srcEnd++
		length++
		forwardExpanded++
	}

	return mkvStart, srcStart, length
}

// isRangeCovered checks if a byte range is already fully covered by matched regions.
func (m *Matcher) isRangeCovered(offset, size int64) bool {
	end := offset + size
	for _, r := range m.matchedRegions {
		if r.mkvStart <= offset && r.mkvEnd >= end {
			return true
		}
	}
	return false
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
				Source:           uint8(inRegion.fileIndex + 1),
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

			// Read delta data
			data := make([]byte, gapLen)
			if _, err := m.mkvMmap.ReadAt(data, pos); err == nil {
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
