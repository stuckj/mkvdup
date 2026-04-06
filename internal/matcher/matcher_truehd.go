package matcher

import (
	"fmt"
	"sort"

	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
)

// fillTrueHDGaps fills unmatched gaps in TrueHD tracks by comparing MKV data
// with source ES data between existing matched regions.
//
// MKV A_TRUEHD packets contain pure TrueHD data (AC3 stripped by the muxer).
// The source parser independently strips AC3 from the Blu-ray interleaved
// stream, but may split at slightly different boundaries. This creates small
// "extra" byte regions in the MKV that aren't in the source ES, breaking
// expansion chains from sync-point matches and leaving ~46% unmatched.
//
// The gap-fill works from existing matched regions (anchors) and fills the
// gaps between them using a greedy forward comparison that skips over extra
// MKV bytes to resynchronize with the source ES.
func (m *Matcher) fillTrueHDGaps(packets []mkv.Packet) {
	// Group packets by TrueHD track
	trackPackets := make(map[int][]mkv.Packet)
	for _, pkt := range packets {
		trackNum := int(pkt.TrackNum)
		if m.isTrueHDTrack[trackNum] {
			trackPackets[trackNum] = append(trackPackets[trackNum], pkt)
		}
	}

	if len(trackPackets) == 0 {
		return
	}

	for trackNum, pkts := range trackPackets {
		sort.Slice(pkts, func(i, j int) bool {
			return pkts[i].Offset < pkts[j].Offset
		})

		m.fillTrueHDTrackGaps(trackNum, pkts)
	}
}

// fillTrueHDTrackGaps fills gaps for a single TrueHD track by finding
// existing matched regions on this track and filling the gaps between them.
func (m *Matcher) fillTrueHDTrackGaps(trackNum int, pkts []mkv.Packet) {
	if len(pkts) == 0 {
		return
	}

	// Binary search to find which packet contains a given MKV offset.
	findPacketIdx := func(mkvOffset int64) int {
		lo, hi := 0, len(pkts)
		for lo < hi {
			mid := lo + (hi-lo)/2
			if pkts[mid].Offset+pkts[mid].Size <= mkvOffset {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		if lo < len(pkts) && mkvOffset >= pkts[lo].Offset && mkvOffset < pkts[lo].Offset+pkts[lo].Size {
			return lo
		}
		return -1
	}

	// findFirstPacketAt returns the index of the first packet whose end is > mkvOffset.
	findFirstPacketAt := func(mkvOffset int64) int {
		lo, hi := 0, len(pkts)
		for lo < hi {
			mid := lo + (hi-lo)/2
			if pkts[mid].Offset+pkts[mid].Size <= mkvOffset {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		return lo
	}

	// Collect matched regions that fall within this track's packets.
	var trackRegions []matchedRegion
	for _, r := range m.matchedRegions {
		if findPacketIdx(r.mkvStart) >= 0 {
			trackRegions = append(trackRegions, r)
		}
	}

	if len(trackRegions) < 2 {
		if m.verboseWriter != nil {
			fmt.Fprintf(m.verboseWriter, "\n[TrueHD gap-fill] track %d: only %d matched regions, need ≥2 for gap-fill\n",
				trackNum, len(trackRegions))
		}
		return
	}

	// Sort by mkvStart for sequential gap processing.
	sort.Slice(trackRegions, func(i, j int) bool {
		return trackRegions[i].mkvStart < trackRegions[j].mkvStart
	})

	if m.verboseWriter != nil {
		fmt.Fprintf(m.verboseWriter, "\n[TrueHD gap-fill] track %d: %d matched regions, fileIndex=%d, subStreamID=0x%02X\n",
			trackNum, len(trackRegions), trackRegions[0].fileIndex, trackRegions[0].audioSubStreamID)
	}

	// Fill gaps between adjacent matched regions.
	var newRegions []matchedRegion
	var totalFilledBytes, totalGapBytes, gapsFilled, gapsSkipped int64

	for i := 0; i < len(trackRegions)-1; i++ {
		prev := trackRegions[i]
		next := trackRegions[i+1]

		// Verify both regions use the same source
		if prev.fileIndex != next.fileIndex || prev.audioSubStreamID != next.audioSubStreamID {
			continue
		}

		gapMKVStart := prev.mkvEnd
		gapMKVEnd := next.mkvStart
		if gapMKVEnd <= gapMKVStart {
			continue
		}

		// Source ES gap: from end of prev's source range to start of next's source range
		prevSrcEnd := prev.srcOffset + (prev.mkvEnd - prev.mkvStart)
		nextSrcStart := next.srcOffset
		srcGapSize := nextSrcStart - prevSrcEnd
		// srcGapSize <= 0 means overlapping or backwards source offsets (invalid gap);
		// srcGapSize < 16 means too small to produce a meaningful match run.
		if srcGapSize <= 0 || srcGapSize < 16 {
			gapsSkipped++
			continue
		}

		// Collect TrueHD packet segments within the gap.
		// Only compare bytes within actual TrueHD packets, skipping
		// interleaved video/audio data from other tracks.
		startPkt := findFirstPacketAt(gapMKVStart)
		var segments []mkvSegment

		for p := startPkt; p < len(pkts) && pkts[p].Offset < gapMKVEnd; p++ {
			pkt := pkts[p]
			segStart := max(pkt.Offset, gapMKVStart)
			segEnd := min(pkt.Offset+pkt.Size, gapMKVEnd)
			if segEnd > m.mkvSize {
				segEnd = m.mkvSize
			}
			if segStart < segEnd {
				segments = append(segments, mkvSegment{segStart, segEnd})
				totalGapBytes += segEnd - segStart
			}
		}

		if len(segments) == 0 {
			gapsSkipped++
			continue
		}

		regions := m.fillTrueHDGapSegments(segments, prevSrcEnd, srcGapSize, prev.fileIndex, prev.audioSubStreamID)
		if len(regions) > 0 {
			newRegions = append(newRegions, regions...)
			gapsFilled++
			for _, r := range regions {
				totalFilledBytes += r.mkvEnd - r.mkvStart
			}
		}
	}

	// Add all new regions
	if len(newRegions) > 0 {
		m.matchedRegions = append(m.matchedRegions, newRegions...)
		for i := range newRegions {
			m.markChunksCovered(newRegions[i].mkvStart, newRegions[i].mkvEnd)
		}
	}

	if m.verboseWriter != nil {
		fmt.Fprintf(m.verboseWriter, "[TrueHD gap-fill] track %d: filled %d gaps (%d bytes, %.2f MB), %d gaps skipped, total TrueHD gap bytes=%d (%.2f MB)\n",
			trackNum, gapsFilled, totalFilledBytes, float64(totalFilledBytes)/(1024*1024),
			gapsSkipped, totalGapBytes, float64(totalGapBytes)/(1024*1024))
	}
}

// mkvSegment describes a contiguous range of MKV data to compare.
type mkvSegment struct{ start, end int64 }

// fillTrueHDGapSegments fills a gap between two matched regions using greedy
// forward comparison across multiple MKV segments (TrueHD packet portions).
//
// The MKV may contain extra bytes (from AC3 splitting differences) that aren't
// in the source ES. When a mismatch occurs, the algorithm advances the MKV
// position by one byte while keeping the source position fixed, then retries.
// Matching runs of ≥16 bytes are recorded as new matched regions.
func (m *Matcher) fillTrueHDGapSegments(
	segments []mkvSegment,
	srcStart, srcSize int64,
	fileIndex uint16, subStreamID byte,
) []matchedRegion {
	if srcSize <= 0 {
		return nil
	}

	// Read source ES data for the gap
	loc := source.Location{
		FileIndex:        fileIndex,
		Offset:           srcStart,
		IsVideo:          false,
		AudioSubStreamID: subStreamID,
	}
	srcData, err := m.sourceIndex.ReadESDataAt(loc, int(srcSize))
	if err != nil || len(srcData) == 0 {
		return nil
	}

	const minRunLen = 16
	var regions []matchedRegion
	srcIdx := 0
	runMKVStart := int64(-1)
	runSrcStart := -1

	// Walk each MKV segment (TrueHD packet data only)
	for _, seg := range segments {
		if srcIdx >= len(srcData) {
			break
		}
		if seg.end > m.mkvSize {
			continue
		}
		mkvData := m.mkvData[seg.start:seg.end]

		for mkvOff := 0; mkvOff < len(mkvData) && srcIdx < len(srcData); {
			mkvAbsPos := seg.start + int64(mkvOff)

			if mkvData[mkvOff] == srcData[srcIdx] {
				if runMKVStart < 0 {
					runMKVStart = mkvAbsPos
					runSrcStart = srcIdx
				}
				mkvOff++
				srcIdx++
			} else {
				// Flush any pending run
				if runMKVStart >= 0 {
					runLen := mkvAbsPos - runMKVStart
					if runLen >= minRunLen {
						regions = append(regions, matchedRegion{
							mkvStart:         runMKVStart,
							mkvEnd:           mkvAbsPos,
							fileIndex:        fileIndex,
							srcOffset:        srcStart + int64(runSrcStart),
							isVideo:          false,
							audioSubStreamID: subStreamID,
						})
					}
					runMKVStart = -1
					runSrcStart = -1
				}
				// Skip forward in MKV (extra byte not in source ES)
				mkvOff++
			}
		}

		// At segment boundary: flush any pending run since the next segment
		// starts at a different MKV offset (non-TrueHD data between packets).
		if runMKVStart >= 0 {
			runEnd := seg.end
			runLen := runEnd - runMKVStart
			if runLen >= minRunLen {
				regions = append(regions, matchedRegion{
					mkvStart:         runMKVStart,
					mkvEnd:           runEnd,
					fileIndex:        fileIndex,
					srcOffset:        srcStart + int64(runSrcStart),
					isVideo:          false,
					audioSubStreamID: subStreamID,
				})
			}
			runMKVStart = -1
			runSrcStart = -1
		}
	}

	return regions
}
