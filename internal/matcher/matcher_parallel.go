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
	// headLocality is the first valid locality observed in the batch.
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

// batchRange tracks the start/end packet indices for a batch.
type batchRange struct {
	start int // inclusive index into packets slice
	end   int // exclusive index into packets slice
}

// matchParallel processes packets using deterministic batched workers.
// Packets must be pre-sorted by track number before calling this function.
// Batches never cross track boundaries — each batch contains packets from
// exactly one track. Within a track, packets are split into chunks of up
// to batchSize. This makes intra-batch locality fully deterministic.
func (m *Matcher) matchParallel(packets []mkv.Packet, progress ProgressFunc) int {
	totalPackets := len(packets)
	if totalPackets == 0 {
		return 0
	}

	// Build track-aware batches: split at track boundaries, then chunk
	// each track's packets into batches of up to batchSize.
	var batches []batchRange
	trackStart := 0
	for i := 1; i <= totalPackets; i++ {
		if i == totalPackets || packets[i].TrackNum != packets[trackStart].TrackNum {
			for j := trackStart; j < i; j += batchSize {
				end := j + batchSize
				if end > i {
					end = i
				}
				batches = append(batches, batchRange{start: j, end: end})
			}
			trackStart = i
		}
	}
	numBatches := len(batches)

	// Per-batch results — each batch appends to its own slice (no locking needed)
	batchResults := make([][]matchedRegion, numBatches)
	// Per-batch edge info for inter-batch sync
	batchEdges := make([]batchEdgeInfo, numBatches)
	// Per-packet match status — true if any NAL in the packet matched
	packetMatched := make([]bool, totalPackets)

	var nextBatch atomic.Int64
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
				m.processBatch(packets, batches[idx], idx, batchResults, batchEdges, packetMatched, &processedCount, progress, totalPackets)
			}
		}()
	}
	wg.Wait()

	// Inter-batch edge sync (deterministic sequential pass)
	m.syncBatchEdges(batches, batchEdges, batchResults, packetMatched)

	// Merge all batch results in batch-index order (deterministic)
	// and count matched packets from the per-packet booleans.
	matchedCount := 0
	for i, results := range batchResults {
		m.matchedRegions = append(m.matchedRegions, results...)
		for j := batches[i].start; j < batches[i].end; j++ {
			if packetMatched[j] {
				matchedCount++
			}
		}
	}

	return matchedCount
}

// processBatch processes a single batch of packets sequentially.
// Each batch contains packets from exactly one track (guaranteed by
// track-aware batch construction). Results are appended to batchResults[batchIdx].
func (m *Matcher) processBatch(
	packets []mkv.Packet,
	br batchRange,
	batchIdx int,
	batchResults [][]matchedRegion,
	batchEdges []batchEdgeInfo,
	packetMatched []bool,
	processedCount *atomic.Int64,
	progress ProgressFunc,
	totalPackets int,
) {
	batchPackets := packets[br.start:br.end]

	// Batch-local coverage bitmap for deterministic intra-batch skipping.
	// Only reads from this bitmap (not the global one) to avoid cross-batch
	// interference from concurrent same-track batches.
	localCov := newLocalCoverage(batchPackets)

	// Batch-local state
	var results []matchedRegion
	var loc packetLocality // carried across packets within the batch

	edge := &batchEdges[batchIdx]
	edge.firstTrack = batchPackets[0].TrackNum
	edge.lastTrack = batchPackets[0].TrackNum // single-track batch

	headEdgeRecorded := false
	for i, pkt := range batchPackets {
		matched, pktResults, edgeMiss := m.matchPacketBatch(pkt, loc, &localCov)

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

		packetMatched[br.start+i] = matched
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
// Coverage reads use the batch-local bitmap (localCov) for determinism;
// coverage writes go to both localCov and the global bitmap (for TrueHD gap-fill).
// Returns whether any NAL matched, the list of matched regions, and edge miss info.
func (m *Matcher) matchPacketBatch(pkt mkv.Packet, loc packetLocality, localCov *localCoverage) (bool, []matchedRegion, edgeMissInfo) {
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
		localCov.markCovered(region.mkvStart, region.mkvEnd)
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

		// Skip sync points already covered by this batch's own matches.
		// Reads from the batch-local bitmap to avoid cross-batch interference
		// from concurrent same-track batches, ensuring deterministic skipping.
		if localCov.isChunkCovered(pkt.Offset + int64(syncOff)) {
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
			if localCov.isRangeCovered(pkt.Offset, pkt.Size) {
				break
			}
		}
	}

	// For Annex B video, scan rest of packet for additional sync points
	if isVideo && !useFullPacket && !localCov.isRangeCovered(pkt.Offset, pkt.Size) && pkt.Size > 4096 {
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
			if localCov.isChunkCovered(pkt.Offset + int64(syncOff)) {
				continue
			}
			moreNALSize, moreNALSizeExact := computeNALSize(moreSyncPoints, moreIdx, syncOff, len(fullData), isVideo, codecInfo.nalLengthSize)
			region := m.tryMatchFromOffsetParallel(pkt, int64(syncOff), fullData[syncOff:], isVideo, pktLoc, moreNALSize, moreNALSizeExact)

			matched := region != nil
			edgeMiss.lastNALMiss = !matched
			edgeMiss.lastNALSyncOff = syncOff
			edgeMiss.lastNALSize = moreNALSize
			edgeMiss.lastNALSizeExact = moreNALSizeExact

			if region != nil {
				recordMatch(region, moreNALSize)
				anyMatched = true
				if localCov.isRangeCovered(pkt.Offset, pkt.Size) {
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
// Only runs for video tracks (tryLocalityMatch is video-specific).
// If a recovery succeeds, packetMatched is updated for the affected packet.
func (m *Matcher) syncBatchEdges(batches []batchRange, batchEdges []batchEdgeInfo, batchResults [][]matchedRegion, packetMatched []bool) {
	for i := 0; i < len(batchEdges)-1; i++ {
		curr := &batchEdges[i]
		next := &batchEdges[i+1]

		// Only sync adjacent batches on the same track
		if curr.lastTrack != next.firstTrack {
			continue
		}

		// tryLocalityMatch is video-specific (uses ES offsets with IsVideo: true)
		if m.trackTypes[int(curr.lastTrack)] != mkv.TrackTypeVideo {
			continue
		}

		if !m.sourceIndex.UsesESOffsets {
			continue
		}

		// If the last NAL of batch i was unmatched, retry with locality from batch i+1
		if curr.edgeMissTail && next.headLocality.valid && curr.tailNALSizeExact {
			pkt := curr.tailPkt
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
					packetMatched[batches[i].end-1] = true
				}
			}
		}

		// If the first NAL of batch i+1 was unmatched, retry with locality from batch i
		if next.edgeMissHead && curr.tailLocality.valid && next.headNALSizeExact {
			pkt := next.headPkt
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
					packetMatched[batches[i+1].start] = true
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

// localCoverage is a sparse batch-local coverage bitmap for deterministic
// intra-batch coverage skipping without cross-batch interference.
// Uses a map from absolute chunk index to dense bitmap index, so memory is
// proportional to actual packet data (not the MKV offset span, which can be
// large for sparse tracks like subtitles).
type localCoverage struct {
	bits     []uint64       // dense bitmap indexed by position
	chunkMap map[int64]int  // absolute chunk index -> dense index
}

// newLocalCoverage creates a sparse coverage bitmap covering only the chunks
// that the given packets actually occupy.
func newLocalCoverage(packets []mkv.Packet) localCoverage {
	if len(packets) == 0 {
		return localCoverage{}
	}

	// Collect all chunk indices touched by the packets
	chunkMap := make(map[int64]int)
	idx := 0
	for _, pkt := range packets {
		if pkt.Size <= 0 {
			continue
		}
		startChunk := pkt.Offset / coverageChunkSize
		endChunk := (pkt.Offset + pkt.Size - 1) / coverageChunkSize
		for chunk := startChunk; chunk <= endChunk; chunk++ {
			if _, exists := chunkMap[chunk]; !exists {
				chunkMap[chunk] = idx
				idx++
			}
		}
	}

	return localCoverage{
		bits:     make([]uint64, (idx+63)/64),
		chunkMap: chunkMap,
	}
}

// markCovered marks chunks fully contained within [start, end) as covered.
func (lc *localCoverage) markCovered(start, end int64) {
	firstFullChunk := (start + coverageChunkSize - 1) / coverageChunkSize
	lastFullChunk := (end / coverageChunkSize) - 1
	for chunk := firstFullChunk; chunk <= lastFullChunk; chunk++ {
		if idx, ok := lc.chunkMap[chunk]; ok {
			lc.bits[idx/64] |= 1 << uint(idx%64)
		}
	}
}

// isChunkCovered checks if the chunk containing absOffset is covered.
func (lc *localCoverage) isChunkCovered(absOffset int64) bool {
	idx, ok := lc.chunkMap[absOffset/coverageChunkSize]
	if !ok {
		return false
	}
	return lc.bits[idx/64]&(1<<uint(idx%64)) != 0
}

// isRangeCovered checks if all chunks in [offset, offset+size) are covered.
func (lc *localCoverage) isRangeCovered(offset, size int64) bool {
	startChunk := offset / coverageChunkSize
	endChunk := (offset + size - 1) / coverageChunkSize
	for chunk := startChunk; chunk <= endChunk; chunk++ {
		idx, ok := lc.chunkMap[chunk]
		if !ok {
			return false
		}
		if lc.bits[idx/64]&(1<<uint(idx%64)) == 0 {
			return false
		}
	}
	return true
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
