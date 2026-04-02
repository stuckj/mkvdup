package matcher

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
)

// computeNALSize computes the NAL/sync-unit or frame size from the sync point layout.
// For AVCC, consecutive sync points are separated by the length prefix of the
// next NAL. For Annex B video, sync points correspond to NAL/sync-unit boundaries
// (e.g. slice headers, sequence headers), not necessarily whole decoded frames;
// for audio and subtitles, consecutive sync points typically delimit frame boundaries.
// Returns (nalSize, exact). exact is true only when derived from a known next
// sync point; when false, nalSize is just the remaining data in the (possibly
// truncated) buffer and must not be used for short-circuit decisions.
func computeNALSize(syncPoints []int, i, syncOff, dataLen int, isVideo bool, nalLengthSize int) (int, bool) {
	nalSize := dataLen - syncOff
	if i+1 < len(syncPoints) {
		if isVideo && nalLengthSize > 0 {
			return syncPoints[i+1] - nalLengthSize - syncOff, true
		}
		return syncPoints[i+1] - syncOff, true
	}
	return nalSize, false
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

	// Read cross-packet hint once at packet start (mutex-protected, consistent snapshot).
	crossHint := m.trackHints[pkt.TrackNum]
	var pktLoc packetLocality
	crossHint.mu.Lock()
	if crossHint.valid {
		pktLoc.valid = true
		pktLoc.fileIdx = crossHint.fileIdx
		pktLoc.offset = crossHint.offset
		pktLoc.srcEnd = crossHint.srcEnd
		pktLoc.mkvEnd = crossHint.mkvEnd
	}
	crossHint.mu.Unlock()

	// recordMatch handles bookkeeping for a successful match (hash-based
	// or locality-based): adds the region, marks coverage, updates state.
	recordMatch := func(region *matchedRegion, nalSize int, nalType ...byte) {
		matchLen := region.mkvEnd - region.mkvStart
		m.regionsMu.Lock()
		m.matchedRegions = append(m.matchedRegions, *region)
		m.regionsMu.Unlock()
		m.markChunksCovered(region.mkvStart, region.mkvEnd)
		// Update per-packet locality (deterministic, goroutine-local)
		pktLoc.valid = true
		pktLoc.fileIdx = region.fileIndex
		pktLoc.offset = region.srcOffset + matchLen/2
		pktLoc.srcEnd = region.srcOffset + matchLen
		pktLoc.mkvEnd = region.mkvEnd
		if isVideo {
			m.diagVideoNALsMatched.Add(1)
			m.diagVideoNALsMatchedBytes.Add(matchLen)
			m.diagNALSizeMatched[nalSizeBucket(nalSize)].Add(1)
			if len(nalType) > 0 {
				m.diagNALTypeMatched[nalType[0]].Add(1)
			}
		}
	}

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

		// Compute NAL/frame size from distance to next sync point.
		nalSize, nalSizeExact := computeNALSize(syncPoints, i, syncOff, len(data), isVideo, codecInfo.nalLengthSize)

		// H.264 NAL type diagnostics (other codecs use different type encodings)
		var nalType byte
		isAVC := isVideo && m.isAVCTrack[int(pkt.TrackNum)] && syncOff < len(data)
		if isAVC {
			nalType = data[syncOff] & 0x1F
			m.diagNALTypeTotal[nalType].Add(1)
		}

		// Hash-based matching (all codecs)
		var region *matchedRegion
		if isAVC {
			region = m.tryMatchFromOffsetParallel(pkt, int64(syncOff), data[syncOff:], isVideo, pktLoc, nalSize, nalSizeExact, nalType)
		} else {
			region = m.tryMatchFromOffsetParallel(pkt, int64(syncOff), data[syncOff:], isVideo, pktLoc, nalSize, nalSizeExact)
		}

		// Locality-based recovery for unmatched video NALs (all video codecs)
		if region == nil && isVideo && m.sourceIndex.UsesESOffsets && nalSizeExact {
			region = m.tryLocalityMatch(pkt, syncOff, data[syncOff:], pktLoc, nalSize)
		}

		if region != nil {
			if isAVC {
				recordMatch(region, nalSize, nalType)
			} else {
				recordMatch(region, nalSize)
			}
		} else if isVideo {
			m.diagNALSizeUnmatched[nalSizeBucket(nalSize)].Add(1)
		}

		if region != nil {
			anyMatched = true
			if m.isRangeCoveredParallel(pkt.Offset, pkt.Size) {
				break
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
		for moreIdx, syncOff := range moreSyncPoints {
			if syncOff < int(readSize) {
				continue // Already tried in the first pass
			}
			if syncOff+m.windowSize > len(fullData) {
				continue
			}
			if m.isChunkCoveredParallel(pkt.Offset + int64(syncOff)) {
				continue
			}
			moreNALSize, moreNALSizeExact := computeNALSize(moreSyncPoints, moreIdx, syncOff, len(fullData), isVideo, codecInfo.nalLengthSize)
			region := m.tryMatchFromOffsetParallel(pkt, int64(syncOff), fullData[syncOff:], isVideo, pktLoc, moreNALSize, moreNALSizeExact)
			if region != nil {
				recordMatch(region, moreNALSize)
				anyMatched = true
				if m.isRangeCoveredParallel(pkt.Offset, pkt.Size) {
					break
				}
			}
		}
	}

	// Also try from packet start (in case it's already aligned)
	if !anyMatched {
		region := m.tryMatchFromOffsetParallel(pkt, 0, data, isVideo, pktLoc, len(data), false)
		if region != nil {
			recordMatch(region, len(data))
			anyMatched = true
		}
	}

	// Write back cross-packet hint (mutex-protected, consistent snapshot)
	if pktLoc.valid {
		crossHint.mu.Lock()
		crossHint.valid = true
		crossHint.fileIdx = pktLoc.fileIdx
		crossHint.offset = pktLoc.offset
		crossHint.srcEnd = pktLoc.srcEnd
		crossHint.mkvEnd = pktLoc.mkvEnd
		crossHint.mu.Unlock()
	}

	return anyMatched
}

// tryMatchFromOffsetParallel attempts hash-based matching for a NAL at the given
// offset. Returns the matched region or nil. The caller handles bookkeeping
// (adding to matchedRegions, marking coverage, updating locality state).
//
// Uses two-phase locality-aware matching:
//   - Phase 1: If packet locality exists, try the closest hash locations first.
//   - Phase 2: Fall back to trying all remaining locations.
func (m *Matcher) tryMatchFromOffsetParallel(pkt mkv.Packet, offsetInPacket int64, data []byte, isVideo bool, loc packetLocality, nalSize int, nalSizeExact bool, nalType ...byte) *matchedRegion {
	if len(data) < m.windowSize {
		return nil
	}

	m.diagTotalSyncPoints.Add(1)

	window := data[:m.windowSize]
	hash := xxhash.Sum64(window)

	// Look up in source index (read-only, thread-safe)
	locations := m.sourceIndex.Lookup(hash)
	if len(locations) == 0 {
		if isVideo {
			m.diagVideoNALsHashNotFound.Add(1)
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
		return nil
	}

	var bestMatch *matchedRegion
	bestMatchLen := int64(0)
	triedVerify := false // whether any tryVerifyAndExpand was called

	// Track which location indices were tried in Phase 1 (small fixed-size array)
	var triedIndices [localityNearbyCount]int
	triedCount := 0

	// Phase 1: Locality-aware search — try nearby locations first (per-packet locality)
	if loc.valid && len(locations) > 1 {
		nearby := nearbyLocationIndices(locations, loc.fileIdx, loc.offset, localityNearbyCount)
		for _, idx := range nearby {
			triedIndices[triedCount] = idx
			triedCount++
			l := locations[idx]

			if m.sourceIndex.UsesESOffsets && l.IsVideo != isVideo {
				if isVideo {
					m.diagVideoNALsSkippedIsVideo.Add(1)
				}
				continue
			}

			triedVerify = true
			region := m.tryVerifyAndExpand(pkt, l, offsetInPacket, isVideo)
			if region != nil {
				matchLen := region.mkvEnd - region.mkvStart
				if matchLen > bestMatchLen {
					bestMatch = region
					bestMatchLen = matchLen
				}
				if bestMatchLen >= localityGoodMatchThreshold || (nalSizeExact && nalSize >= m.windowSize && bestMatchLen >= int64(nalSize)) {
					break
				}
			}
		}
	}

	// Phase 2: Full search of remaining locations
	phase2Skipped := bestMatchLen >= localityGoodMatchThreshold || (nalSizeExact && nalSize >= m.windowSize && bestMatchLen >= int64(nalSize))
	if phase2Skipped {
		m.diagPhase1Skips.Add(1)
	}
	if !phase2Skipped {
		m.diagPhase2Fallbacks.Add(1)
		verifyAttempts := 0
		for i, l := range locations {
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
			if m.sourceIndex.UsesESOffsets && l.IsVideo != isVideo {
				if isVideo {
					m.diagVideoNALsSkippedIsVideo.Add(1)
				}
				continue
			}

			triedVerify = true
			verifyAttempts++
			m.diagPhase2Locations.Add(1)
			region := m.tryVerifyAndExpand(pkt, l, offsetInPacket, isVideo)
			if region != nil {
				matchLen := region.mkvEnd - region.mkvStart
				if matchLen > bestMatchLen {
					bestMatch = region
					bestMatchLen = matchLen
				}
				if bestMatchLen >= localityGoodMatchThreshold || (nalSizeExact && nalSize >= m.windowSize && bestMatchLen >= int64(nalSize)) {
					m.diagPhase2EarlyExits.Add(1)
					break
				}
			}

			if verifyAttempts >= phase2MaxVerifyAttempts {
				m.diagPhase2Capped.Add(1)
				break
			}
		}
	}

	if bestMatch != nil {
		return bestMatch
	}

	if isVideo {
		if triedVerify {
			m.diagVideoNALsVerifyFailed.Add(1)
		} else {
			m.diagVideoNALsAllSkipped.Add(1)
		}
	}
	return nil
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
