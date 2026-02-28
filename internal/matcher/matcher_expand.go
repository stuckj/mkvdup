package matcher

import (
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
)

// expandChunkSize is the number of bytes to read at once during match expansion.
// Larger chunks reduce page faults when expanding across mmap'd source files.
const expandChunkSize = 4096

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

	region := &matchedRegion{
		mkvStart:         mkvStart,
		mkvEnd:           mkvStart + matchLen,
		fileIndex:        loc.FileIndex,
		srcOffset:        srcStart,
		isVideo:          isVideo,
		audioSubStreamID: loc.AudioSubStreamID,
		isLPCM:           source.IsLPCMSubStreamID(loc.AudioSubStreamID),
	}

	return region
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
