package matcher

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
)

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
	// Determine if this is video or audio
	trackType := m.trackTypes[int(pkt.TrackNum)]
	isVideo := trackType == mkv.TrackTypeVideo

	if isVideo {
		m.diagVideoPacketsTotal.Add(1)
	}

	// Check if this region is already covered by a matched region
	// Note: This is a relaxed check - we may miss some coverage due to race conditions,
	// but that's okay since we merge overlapping regions at the end anyway
	if m.isRangeCoveredParallel(pkt.Offset, pkt.Size) {
		if isVideo {
			m.diagVideoPacketsCoverage.Add(1)
		}
		return true
	}

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
	codecInfo := m.trackCodecs[int(pkt.TrackNum)]
	if isVideo {
		if codecInfo.nalLengthSize > 0 {
			// AVCC/HVCC format: parse length-prefixed NAL units
			syncPoints = source.FindAVCCNALStarts(data, codecInfo.nalLengthSize)
		} else {
			// Annex B format: find NAL starts after 00 00 01
			syncPoints = source.FindVideoNALStarts(data)
		}
	} else if trackType == mkv.TrackTypeSubtitle {
		syncPoints = source.FindPGSSyncPoints(data)
	} else if m.isPCMTrack[int(pkt.TrackNum)] {
		syncPoints = source.FindLPCMMatchSyncPoints(data)
	} else {
		syncPoints = source.FindAudioSyncPoints(data)
	}

	// Look up per-track locality hint once per packet (not per sync point).
	hint := m.trackHints[pkt.TrackNum]

	// For AVCC/HVCC video, each NAL unit has different framing bytes than the
	// source (length prefix vs start code), so expansion stops at NAL boundaries.
	// We must match each NAL individually to cover the full packet.
	// For Annex B video (MPEG-2), expansion can cross start code boundaries
	// when the source data matches. However, shared structures like sequence
	// headers match many source locations with short expansions. We must
	// continue trying other sync points (e.g., slice headers) to find better
	// matches that cover the full packet.
	anyMatched := false
	for i, syncOff := range syncPoints {
		if syncOff+m.windowSize > len(data) {
			if isVideo {
				m.diagVideoNALsTooSmall.Add(1)
				m.diagNALSizeUnmatched[0].Add(1) // <64B bucket
			}
			continue
		}

		// Skip sync points whose chunk is already covered — the source data
		// for this region has already been verified byte-for-byte by a prior match.
		if m.isChunkCoveredParallel(pkt.Offset + int64(syncOff)) {
			continue
		}

		if isVideo {
			m.diagVideoNALsTotal.Add(1)
		}

		// Compute NAL size from distance to next sync point (minus length prefix).
		// For AVCC, consecutive sync points are separated by the length prefix of the
		// next NAL. For Annex B and non-video, use remaining data length.
		nalSize := len(data) - syncOff
		if isVideo && codecInfo.nalLengthSize > 0 && i+1 < len(syncPoints) {
			nalSize = syncPoints[i+1] - codecInfo.nalLengthSize - syncOff
		}

		// Track NAL type for video diagnostics (H.264 only —
		// HEVC uses different NAL type encoding, MPEG-2 uses start code types)
		var matched bool
		if isVideo && m.isAVCTrack[int(pkt.TrackNum)] && syncOff < len(data) {
			nalType := data[syncOff] & 0x1F
			m.diagNALTypeTotal[nalType].Add(1)
			matched = m.tryMatchFromOffsetParallel(pkt, int64(syncOff), data[syncOff:], isVideo, hint, nalSize, nalType)
		} else {
			matched = m.tryMatchFromOffsetParallel(pkt, int64(syncOff), data[syncOff:], isVideo, hint, nalSize)
		}

		if matched {
			anyMatched = true
			// Early return once the packet is fully covered
			if m.isRangeCoveredParallel(pkt.Offset, pkt.Size) {
				return true
			}
		}
	}

	// For Annex B video, if the first 4096 bytes didn't give full coverage,
	// scan the rest of the packet for additional sync points. This handles
	// cases where only shared structures (sequence headers) appear early
	// but unique slice data further in the packet would match.
	if isVideo && !useFullPacket && !m.isRangeCoveredParallel(pkt.Offset, pkt.Size) && pkt.Size > 4096 {
		fullEnd := pkt.Offset + pkt.Size
		if fullEnd > m.mkvSize {
			fullEnd = m.mkvSize
		}
		fullData := m.mkvData[pkt.Offset:fullEnd]
		moreSyncPoints := source.FindVideoNALStarts(fullData)
		for _, syncOff := range moreSyncPoints {
			if syncOff < int(readSize) {
				continue // Already tried in the first pass
			}
			if syncOff+m.windowSize > len(fullData) {
				continue
			}
			// Skip sync points whose chunk is already covered
			if m.isChunkCoveredParallel(pkt.Offset + int64(syncOff)) {
				continue
			}
			if m.tryMatchFromOffsetParallel(pkt, int64(syncOff), fullData[syncOff:], isVideo, hint, len(fullData)-syncOff) {
				anyMatched = true
				if m.isRangeCoveredParallel(pkt.Offset, pkt.Size) {
					return true
				}
			}
		}
	}

	// Also try from packet start (in case it's already aligned)
	if !anyMatched {
		if m.tryMatchFromOffsetParallel(pkt, 0, data, isVideo, hint, len(data)) {
			anyMatched = true
		}
	}

	return anyMatched
}

// tryMatchFromOffsetParallel is a thread-safe version of tryMatchFromOffset.
// Uses two-phase locality-aware matching:
//   - Phase 1: If a locality hint exists, try the closest locations first.
//     If any produces a match >= localityGoodMatchThreshold, accept immediately.
//   - Phase 2: Fall back to trying all remaining locations (handles scene changes,
//     chapter boundaries, multi-file sources).
func (m *Matcher) tryMatchFromOffsetParallel(pkt mkv.Packet, offsetInPacket int64, data []byte, isVideo bool, hint *trackLocalityHint, nalSize int, nalType ...byte) bool {
	if len(data) < m.windowSize {
		return false
	}

	window := data[:m.windowSize]
	hash := xxhash.Sum64(window)

	// Look up in source index (read-only, thread-safe)
	locations := m.sourceIndex.Lookup(hash)
	if len(locations) == 0 {
		if isVideo {
			m.diagVideoNALsHashNotFound.Add(1)
			m.diagNALSizeUnmatched[nalSizeBucket(nalSize)].Add(1)
			if len(nalType) > 0 {
				m.diagNALTypeNotFound[nalType[0]].Add(1)
			}
			// Capture first 20 examples
			if len(nalType) > 0 {
				m.diagExamplesMu.Lock()
				if m.diagExamplesCount < 20 {
					m.diagExamplesCount++
					example := fmt.Sprintf("  NAL type=%d, pktOff=%d, syncOff=%d, nalSize=%d, hash=%016x, first8bytes=%02x",
						nalType[0], pkt.Offset, offsetInPacket, nalSize, hash, data[:min(8, len(data))])
					m.diagExamplesOutput = append(m.diagExamplesOutput, example)
				}
				m.diagExamplesMu.Unlock()
			}
		}
		return false
	}

	var bestMatch *matchedRegion
	bestMatchLen := int64(0)
	triedVerify := false // whether any tryVerifyAndExpand was called

	// Track which location indices were tried in Phase 1 (small fixed-size array)
	var triedIndices [localityNearbyCount]int
	triedCount := 0

	// Phase 1: Locality-aware search — try nearby locations first (per-track hint)
	if hint != nil && hint.valid.Load() && len(locations) > 1 {
		hintFile := uint16(hint.fileIndex.Load())
		hintOffset := hint.offset.Load()

		nearby := nearbyLocationIndices(locations, hintFile, hintOffset, localityNearbyCount)
		for _, idx := range nearby {
			triedIndices[triedCount] = idx
			triedCount++
			loc := locations[idx]

			if m.sourceIndex.UsesESOffsets && loc.IsVideo != isVideo {
				if isVideo {
					m.diagVideoNALsSkippedIsVideo.Add(1)
				}
				continue
			}

			triedVerify = true
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
			// Skip indices already tried in Phase 1 (linear scan of small array)
			alreadyTried := false
			for t := 0; t < triedCount; t++ {
				if triedIndices[t] == i {
					alreadyTried = true
					break
				}
			}
			if alreadyTried {
				continue
			}
			if m.sourceIndex.UsesESOffsets && loc.IsVideo != isVideo {
				if isVideo {
					m.diagVideoNALsSkippedIsVideo.Add(1)
				}
				continue
			}

			triedVerify = true
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
		// Update per-track locality hint with midpoint of matched source region
		if hint != nil {
			hint.fileIndex.Store(uint32(bestMatch.fileIndex))
			hint.offset.Store(bestMatch.srcOffset + bestMatchLen/2)
			hint.valid.Store(true)
		}
		if isVideo {
			m.diagVideoNALsMatched.Add(1)
			m.diagVideoNALsMatchedBytes.Add(bestMatchLen)
			m.diagNALSizeMatched[nalSizeBucket(nalSize)].Add(1)
			if len(nalType) > 0 {
				m.diagNALTypeMatched[nalType[0]].Add(1)
			}
		}
		return true
	}

	if isVideo {
		m.diagNALSizeUnmatched[nalSizeBucket(nalSize)].Add(1)
		if triedVerify {
			m.diagVideoNALsVerifyFailed.Add(1)
		} else {
			m.diagVideoNALsAllSkipped.Add(1)
		}
	}
	return false
}

// nearbyLocationIndices returns up to N indices into locations that are closest
// to hintOffset within the same file as hintFileIndex. Locations must be pre-sorted
// by (FileIndex, Offset) via SortLocationsByOffset. Returns an empty slice if no
// locations are in the target file.
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

// isRangeCoveredParallel checks if a range is likely covered using a coverage bitmap.
// This is an O(1) check using chunk-level granularity. It may have false positives
// (multiple regions covering different chunks) but that's acceptable since we merge
// overlapping regions at the end anyway.
func (m *Matcher) isRangeCoveredParallel(offset, size int64) bool {
	// Calculate chunk range
	startChunk := offset / coverageChunkSize
	endChunk := (offset + size - 1) / coverageChunkSize

	m.coverageMu.RLock()
	defer m.coverageMu.RUnlock()

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

// isChunkCoveredParallel checks if the chunk containing absOffset is already covered.
// This is used to skip sync points that fall within already-matched regions,
// avoiding redundant hash lookups and source reads.
func (m *Matcher) isChunkCoveredParallel(absOffset int64) bool {
	chunk := absOffset / coverageChunkSize
	wordIdx := chunk / 64
	bitIdx := uint(chunk % 64)

	m.coverageMu.RLock()
	defer m.coverageMu.RUnlock()

	if wordIdx >= int64(len(m.coveredChunks)) {
		return false
	}
	return m.coveredChunks[wordIdx]&(1<<bitIdx) != 0
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
