package matcher

import (
	"fmt"
	"sort"
)

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
				IsLPCM:           inRegion.isLPCM,
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
