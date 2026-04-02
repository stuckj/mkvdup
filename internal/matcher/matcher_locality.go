package matcher

import (
	"fmt"

	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
)

// localityVerifyLen is the minimum number of bytes needed for a reliable
// locality-based match. NALs smaller than this are skipped.
const localityVerifyLen = 64

// alignSearchRange is the number of byte offsets to try in each direction
// when aligning the predicted source offset to the actual NAL header position.
// The prediction can be off by a few bytes due to AVCC vs Annex B framing
// differences (length prefix size vs start code size).
const alignSearchRange = 3

// minAlignBytes is the minimum number of leading bytes that must match
// to confirm alignment between MKV and source NAL data. Set to 4 to avoid
// false positives — H.264 NALs have predictable first 2 bytes (NAL type +
// first slice header byte), so 2 bytes is insufficient across 7 candidates.
const minAlignBytes = 4

// tryLocalityMatch attempts to recover a NAL that failed hash-based matching
// by using the per-track locality hint to predict the source location. This
// recovers NALs that the indexer missed during source indexing — the bytes
// exist in the source but were never hashed into the index.
//
// The approach compares leading bytes at nearby offsets to align the
// prediction, then verifies the full NAL matches byte-for-byte.
//
// Returns a normal matchedRegion, or nil if no match found.
func (m *Matcher) tryLocalityMatch(
	pkt mkv.Packet,
	syncOff int,
	mkvNALData []byte,
	loc packetLocality,
	nalSize int,
) *matchedRegion {
	// Only attempt if we have valid per-packet locality and a large enough NAL
	if !loc.valid || loc.srcEnd <= 0 || loc.mkvEnd <= 0 {
		return nil
	}
	if nalSize < localityVerifyLen || len(mkvNALData) < nalSize {
		return nil
	}

	// Predict approximate source ES offset. Within a single MKV packet,
	// NALs are packed sequentially, so the MKV offset delta closely
	// matches the source ES offset delta (differing only by framing:
	// AVCC length prefix vs Annex B start code, typically ±1 byte).
	// Across packets, MKV offsets include container overhead (cluster/block
	// headers, other tracks' data) that doesn't exist in the source ES,
	// making the prediction unreliable. Skip if the gap is too large.
	currentMkvOff := pkt.Offset + int64(syncOff)
	mkvDelta := currentMkvOff - loc.mkvEnd
	if mkvDelta < 0 || mkvDelta > int64(nalSize)*2 {
		return nil
	}
	predictedSrcOff := loc.srcEnd + mkvDelta
	if predictedSrcOff < 0 {
		return nil
	}

	// Count actual IO-probing attempts (after all early-exit guards)
	m.diagLocalityAttempts.Add(1)
	debugN := m.diagLocalityAttempts.Load()
	debug := m.verboseWriter != nil && debugN <= 10

	hintFileIndex := loc.fileIdx

	if debug {
		fmt.Fprintf(m.verboseWriter, "[locality#%d] mkvOff=%d nalSize=%d nalHdr=%02x predictedSrc=%d fileIdx=%d\n",
			debugN, currentMkvOff, nalSize, mkvNALData[0], predictedSrcOff, hintFileIndex)
	}

	// Try to align the predicted offset to the actual NAL header position
	// by comparing leading bytes. The prediction can be off by a few bytes
	// due to AVCC (4-byte length prefix) vs Annex B (3-4 byte start code)
	// framing differences. We try offsets around the prediction and look for
	// the first position where the initial bytes match the MKV NAL data.
	srcNALOffset := int64(-1)
	for delta := -alignSearchRange; delta <= alignSearchRange; delta++ {
		candidateOff := predictedSrcOff + int64(delta)
		if candidateOff < 0 {
			continue
		}

		loc := source.Location{
			FileIndex: hintFileIndex,
			Offset:    candidateOff,
			IsVideo:   true,
		}
		probe, err := m.sourceIndex.ReadESDataAt(loc, minAlignBytes)
		if err != nil || len(probe) < minAlignBytes {
			continue
		}

		// Check if the first minAlignBytes bytes match the MKV NAL data
		match := true
		for i := 0; i < minAlignBytes; i++ {
			if probe[i] != mkvNALData[i] {
				match = false
				break
			}
		}
		if match {
			srcNALOffset = candidateOff
			break
		}
	}

	if srcNALOffset < 0 {
		if debug {
			fmt.Fprintf(m.verboseWriter, "[locality#%d] alignment failed\n", debugN)
		}
		return nil
	}

	// Read source NAL data at the aligned offset and verify full match.
	srcLoc := source.Location{
		FileIndex: hintFileIndex,
		Offset:    srcNALOffset,
		IsVideo:   true,
	}
	srcData, err := m.sourceIndex.ReadESDataAt(srcLoc, nalSize)
	if err != nil || len(srcData) < nalSize {
		if debug {
			fmt.Fprintf(m.verboseWriter, "[locality#%d] source read failed: err=%v len=%d need=%d\n", debugN, err, len(srcData), nalSize)
		}
		return nil
	}

	// Verify every byte matches
	for i := 0; i < nalSize; i++ {
		if mkvNALData[i] != srcData[i] {
			if debug {
				fmt.Fprintf(m.verboseWriter, "[locality#%d] mismatch at byte %d: src=%02x mkv=%02x\n", debugN, i, srcData[i], mkvNALData[i])
			}
			return nil
		}
	}

	// Success — exact match found via locality prediction.
	if debug {
		fmt.Fprintf(m.verboseWriter, "[locality#%d] exact match at srcOff=%d\n", debugN, srcNALOffset)
	}

	mkvStart := pkt.Offset + int64(syncOff)
	mkvEnd := mkvStart + int64(nalSize)

	m.diagLocalityMatched.Add(1)
	m.diagLocalityMatchedBytes.Add(int64(nalSize))

	return &matchedRegion{
		mkvStart:  mkvStart,
		mkvEnd:    mkvEnd,
		fileIndex: hintFileIndex,
		srcOffset: srcNALOffset,
		isVideo:   true,
	}
}
