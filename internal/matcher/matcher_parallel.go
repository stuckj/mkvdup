package matcher

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
)

// batchSize is the number of packets processed sequentially within each batch.
// Balances parallelism (enough batches for all workers) with locality quality
// (enough packets per batch for good intra-batch locality hints).
const batchSize = 64

// batchEdgeInfo records the locality state and edge miss status at the
// boundaries of a completed batch. Used for deterministic inter-batch
// edge sync after all batches complete.
type batchEdgeInfo struct {
	// firstTrack/lastTrack record which track the first/last packet belongs to.
	firstTrack uint64
	lastTrack  uint64
	// tailLocality is the locality state after the last packet in the batch.
	tailLocality packetLocality
	// headLocality is the locality state after the first packet in the batch.
	headLocality packetLocality
	// edgeMissHead is true if the first NAL of the first packet was unmatched
	// (failed both hash-based and locality-based matching).
	edgeMissHead bool
	// edgeMissTail is true if the last NAL of the last packet was unmatched
	// (failed both hash-based and locality-based matching).
	edgeMissTail bool
	// headPkt/tailPkt are the first/last packets for edge retry.
	headPkt mkv.Packet
	tailPkt mkv.Packet
	// headSyncOff/tailSyncOff are the sync offsets of the edge NALs.
	headSyncOff int
	tailSyncOff int
	// headNALSize/tailNALSize and exact flags for the edge NALs.
	headNALSize      int
	headNALSizeExact bool
	tailNALSize      int
	tailNALSizeExact bool
}

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

// matchParallel processes packets using deterministic batched workers.
// Packets must be pre-sorted by track number before calling this function.
// The batch boundaries are fixed, and each batch is processed sequentially
// by a single goroutine, making the output deterministic regardless of
// goroutine scheduling or CPU count.
func (m *Matcher) matchParallel(packets []mkv.Packet, progress ProgressFunc) int {
	totalPackets := len(packets)
	if totalPackets == 0 {
		return 0
	}

	numBatches := (totalPackets + batchSize - 1) / batchSize

	// Per-batch results — each batch appends to its own slice (no locking needed)
	batchResults := make([][]matchedRegion, numBatches)
	// Per-batch edge info for inter-batch sync
	batchEdges := make([]batchEdgeInfo, numBatches)

	var nextBatch atomic.Int64
	var matchedCount atomic.Int64
	var processedCount atomic.Int64

	var wg sync.WaitGroup
	for i := 0; i < m.numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				idx := int(nextBatch.Add(1) - 1)
				if idx >= numBatches {
					return
				}
				m.processBatch(packets, idx, batchResults, batchEdges, &matchedCount, &processedCount, progress, totalPackets)
			}
		}()
	}
	wg.Wait()

	// Inter-batch edge sync (deterministic sequential pass)
	m.syncBatchEdges(batchEdges, batchResults)

	// Merge all batch results in batch-index order (deterministic)
	for _, results := range batchResults {
		m.matchedRegions = append(m.matchedRegions, results...)
	}

	return int(matchedCount.Load())
}

// processBatch processes a single batch of packets sequentially.
// All locality state is batch-local. Results are appended to batchResults[batchIdx].
func (m *Matcher) processBatch(
	packets []mkv.Packet,
	batchIdx int,
	batchResults [][]matchedRegion,
	batchEdges []batchEdgeInfo,
	matchedCount *atomic.Int64,
	processedCount *atomic.Int64,
	progress ProgressFunc,
	totalPackets int,
) {
	start := batchIdx * batchSize
	end := start + batchSize
	if end > len(packets) {
		end = len(packets)
	}
	batchPackets := packets[start:end]

	// Batch-local state
	var results []matchedRegion
	var loc packetLocality // carried across packets within the batch

	edge := &batchEdges[batchIdx]
	edge.firstTrack = batchPackets[0].TrackNum
	edge.lastTrack = batchPackets[len(batchPackets)-1].TrackNum

	headEdgeRecorded := false
	for i, pkt := range batchPackets {
		// Reset locality when track changes within the batch
		if i > 0 && pkt.TrackNum != batchPackets[i-1].TrackNum {
			loc = packetLocality{}
		}

		matched, pktResults, edgeMiss := m.matchPacketBatch(pkt, loc)

		// Update batch-local locality from this packet's results
		if len(pktResults) > 0 {
			last := pktResults[len(pktResults)-1]
			matchLen := last.mkvEnd - last.mkvStart
			loc.valid = true
			loc.fileIdx = last.fileIndex
			loc.offset = last.srcOffset + matchLen/2
			loc.srcEnd = last.srcOffset + matchLen
			loc.mkvEnd = last.mkvEnd
		}

		results = append(results, pktResults...)

		if matched {
			matchedCount.Add(1)
		}
		count := processedCount.Add(1)
		if progress != nil && count%1000 == 0 {
			progress(int(count), totalPackets)
		}

		// Record edge info for first and last packets
		if i == 0 {
			edge.edgeMissHead = edgeMiss.firstNALMiss
			edge.headPkt = pkt
			edge.headSyncOff = edgeMiss.firstNALSyncOff
			edge.headNALSize = edgeMiss.firstNALSize
			edge.headNALSizeExact = edgeMiss.firstNALSizeExact
		}
		// Capture the first valid locality in the batch for edge sync.
		// If the first packet has no matches, we need locality from a
		// later packet so the previous batch's tail edge can be retried.
		if !headEdgeRecorded && loc.valid {
			edge.headLocality = loc
			headEdgeRecorded = true
		}
		if i == len(batchPackets)-1 {
			edge.tailLocality = loc
			edge.edgeMissTail = edgeMiss.lastNALMiss
			edge.tailPkt = pkt
			edge.tailSyncOff = edgeMiss.lastNALSyncOff
			edge.tailNALSize = edgeMiss.lastNALSize
			edge.tailNALSizeExact = edgeMiss.lastNALSizeExact
		}
	}

	batchResults[batchIdx] = results
}

// edgeMissInfo records whether the first/last NAL in a packet was unmatched
// (failed both hash-based and locality-based matching), for inter-batch edge sync.
type edgeMissInfo struct {
	firstNALMiss      bool
	firstNALSyncOff   int
	firstNALSize      int
	firstNALSizeExact bool
	lastNALMiss       bool
	lastNALSyncOff    int
	lastNALSize       int
	lastNALSizeExact  bool
}

// matchPacketBatch processes a single packet with batch-local state.
// Returns whether any NAL matched, the list of matched regions, and edge miss info.
func (m *Matcher) matchPacketBatch(pkt mkv.Packet, loc packetLocality) (bool, []matchedRegion, edgeMissInfo) {
	var results []matchedRegion
	var edgeMiss edgeMissInfo

	trackType := m.trackTypes[int(pkt.TrackNum)]
	isVideo := trackType == mkv.TrackTypeVideo

	if isVideo {
		m.diagVideoPacketsTotal.Add(1)
	}

	// Read packet data to find sync points (zero-copy slice access)
	readSize := pkt.Size
	if readSize < int64(m.windowSize) {
		return false, nil, edgeMiss
	}

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

	endOffset := pkt.Offset + readSize
	if endOffset > m.mkvSize {
		endOffset = m.mkvSize
	}
	data := m.mkvData[pkt.Offset:endOffset]
	if len(data) < m.windowSize {
		return false, nil, edgeMiss
	}

	// Find sync points within the packet data
	var syncPoints []int
	codecInfo := m.trackCodecs[int(pkt.TrackNum)]
	if isVideo {
		if codecInfo.nalLengthSize > 0 {
			syncPoints = source.FindAVCCNALStarts(data, codecInfo.nalLengthSize)
		} else {
			syncPoints = source.FindVideoNALStarts(data)
		}
	} else if trackType == mkv.TrackTypeSubtitle {
		syncPoints = source.FindPGSSyncPoints(data)
	} else if m.isPCMTrack[int(pkt.TrackNum)] {
		syncPoints = source.FindLPCMMatchSyncPoints(data)
	} else {
		syncPoints = source.FindAudioSyncPoints(data)
	}

	// Use the batch-local locality directly (passed in from caller)
	pktLoc := loc

	recordMatch := func(region *matchedRegion, nalSize int, nalType ...byte) {
		matchLen := region.mkvEnd - region.mkvStart
		results = append(results, *region)
		m.markChunksCovered(region.mkvStart, region.mkvEnd)
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

	anyMatched := false
	firstNALProcessed := false
	for i, syncOff := range syncPoints {
		if syncOff+m.windowSize > len(data) {
			if isVideo {
				m.diagVideoNALsTooSmall.Add(1)
				m.diagNALSizeUnmatched[0].Add(1)
			}
			continue
		}

		// Within a batch, intra-packet coverage skipping is deterministic
		// since the batch is processed sequentially by one goroutine.
		if m.isChunkCoveredParallel(pkt.Offset + int64(syncOff)) {
			continue
		}

		if isVideo {
			m.diagVideoNALsTotal.Add(1)
		}

		nalSize, nalSizeExact := computeNALSize(syncPoints, i, syncOff, len(data), isVideo, codecInfo.nalLengthSize)

		var nalType byte
		isAVC := isVideo && m.isAVCTrack[int(pkt.TrackNum)] && syncOff < len(data)
		if isAVC {
			nalType = data[syncOff] & 0x1F
			m.diagNALTypeTotal[nalType].Add(1)
		}

		var region *matchedRegion
		if isAVC {
			region = m.tryMatchFromOffsetParallel(pkt, int64(syncOff), data[syncOff:], isVideo, pktLoc, nalSize, nalSizeExact, nalType)
		} else {
			region = m.tryMatchFromOffsetParallel(pkt, int64(syncOff), data[syncOff:], isVideo, pktLoc, nalSize, nalSizeExact)
		}

		if region == nil && isVideo && m.sourceIndex.UsesESOffsets && nalSizeExact {
			region = m.tryLocalityMatch(pkt, syncOff, data[syncOff:], pktLoc, nalSize)
		}

		matched := region != nil
		if matched {
			if isAVC {
				recordMatch(region, nalSize, nalType)
			} else {
				recordMatch(region, nalSize)
			}
		} else if isVideo {
			m.diagNALSizeUnmatched[nalSizeBucket(nalSize)].Add(1)
		}

		// Track edge miss info
		if !firstNALProcessed {
			firstNALProcessed = true
			edgeMiss.firstNALMiss = !matched
			edgeMiss.firstNALSyncOff = syncOff
			edgeMiss.firstNALSize = nalSize
			edgeMiss.firstNALSizeExact = nalSizeExact
		}
		edgeMiss.lastNALMiss = !matched
		edgeMiss.lastNALSyncOff = syncOff
		edgeMiss.lastNALSize = nalSize
		edgeMiss.lastNALSizeExact = nalSizeExact

		if matched {
			anyMatched = true
			if m.isRangeCoveredParallel(pkt.Offset, pkt.Size) {
				break
			}
		}
	}

	// For Annex B video, scan rest of packet for additional sync points
	if isVideo && !useFullPacket && !m.isRangeCoveredParallel(pkt.Offset, pkt.Size) && pkt.Size > 4096 {
		fullEnd := pkt.Offset + pkt.Size
		if fullEnd > m.mkvSize {
			fullEnd = m.mkvSize
		}
		fullData := m.mkvData[pkt.Offset:fullEnd]
		moreSyncPoints := source.FindVideoNALStarts(fullData)
		for moreIdx, syncOff := range moreSyncPoints {
			if syncOff < int(readSize) {
				continue
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

	// Try from packet start if nothing matched
	if !anyMatched {
		region := m.tryMatchFromOffsetParallel(pkt, 0, data, isVideo, pktLoc, len(data), false)
		if region != nil {
			recordMatch(region, len(data))
			anyMatched = true
		}
	}

	return anyMatched, results, edgeMiss
}

// syncBatchEdges performs a deterministic sequential pass over batch boundaries
// to retry edge NALs that were unmatched using locality from adjacent batches.
// Edge sync adds regions to already-processed packets; it does not change
// packet-level match counts since those were determined during batch processing.
func (m *Matcher) syncBatchEdges(batchEdges []batchEdgeInfo, batchResults [][]matchedRegion) {
	for i := 0; i < len(batchEdges)-1; i++ {
		curr := &batchEdges[i]
		next := &batchEdges[i+1]

		// Only sync if adjacent batches share the same track at the boundary
		if curr.lastTrack != next.firstTrack {
			continue
		}

		// If the last NAL of batch i had a hash miss, retry with locality from batch i+1
		if curr.edgeMissTail && next.headLocality.valid && curr.tailNALSizeExact {
			pkt := curr.tailPkt
			if m.sourceIndex.UsesESOffsets {
				syncOff := curr.tailSyncOff
				endOffset := pkt.Offset + pkt.Size
				if endOffset > m.mkvSize {
					endOffset = m.mkvSize
				}
				data := m.mkvData[pkt.Offset:endOffset]
				if syncOff+m.windowSize <= len(data) {
					region := m.tryLocalityMatch(pkt, syncOff, data[syncOff:], next.headLocality, curr.tailNALSize)
					if region != nil {
						batchResults[i] = append(batchResults[i], *region)
						m.markChunksCovered(region.mkvStart, region.mkvEnd)
					}
				}
			}
		}

		// If the first NAL of batch i+1 was unmatched, retry with locality from batch i
		if next.edgeMissHead && curr.tailLocality.valid && next.headNALSizeExact {
			pkt := next.headPkt
			if m.sourceIndex.UsesESOffsets {
				syncOff := next.headSyncOff
				endOffset := pkt.Offset + pkt.Size
				if endOffset > m.mkvSize {
					endOffset = m.mkvSize
				}
				data := m.mkvData[pkt.Offset:endOffset]
				if syncOff+m.windowSize <= len(data) {
					region := m.tryLocalityMatch(pkt, syncOff, data[syncOff:], curr.tailLocality, next.headNALSize)
					if region != nil {
						batchResults[i+1] = append(batchResults[i+1], *region)
						m.markChunksCovered(region.mkvStart, region.mkvEnd)
					}
				}
			}
		}
	}
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
	triedVerify := false

	var triedIndices [localityNearbyCount]int
	triedCount := 0

	// Phase 1: Locality-aware search
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

	// Radiate outward from lo to collect the closest locations in the same file
	result := make([]int, 0, maxCount)
	left := lo - 1
	right := lo

	for len(result) < maxCount && (left >= 0 || right < n) {
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
			break
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
	startChunk := offset / coverageChunkSize
	endChunk := (offset + size - 1) / coverageChunkSize

	m.coverageMu.RLock()
	defer m.coverageMu.RUnlock()

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
	firstFullChunk := (start + coverageChunkSize - 1) / coverageChunkSize
	lastFullChunk := (end / coverageChunkSize) - 1

	if firstFullChunk > lastFullChunk {
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
