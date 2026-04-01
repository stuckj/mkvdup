package matcher

import (
	"github.com/stuckj/mkvdup/internal/bitshift"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
)

// maxDivergenceOffset is the maximum byte offset from the NAL start where
// divergence (bit-shift) can begin. H.264 slice headers typically diverge
// within the first 5-10 bytes; 20 provides margin for unusual encodings.
const maxDivergenceOffset = 20

// bitShiftVerifyLen is the number of bytes used for initial shift verification
// before committing to a full-NAL check.
const bitShiftVerifyLen = 64

// alignSearchRange is the number of byte offsets to try in each direction
// when aligning the predicted source offset to the actual NAL header position.
// The prediction can be off by a few bytes due to AVCC vs Annex B framing
// differences (length prefix size vs start code size).
const alignSearchRange = 3

// minAlignBytes is the minimum number of pre-divergence bytes that must match
// to confirm alignment between MKV and source NAL data. Set to 4 to avoid
// false positives — H.264 NALs have predictable first 2 bytes (NAL type +
// first slice header byte), so 2 bytes is insufficient across 7 candidates.
const minAlignBytes = 4

// tryBitShiftMatch attempts to recover a NAL that failed hash-based matching
// by detecting a bit-shift between source and MKV data. This handles H.264
// slice headers where extraction tools modified VLC-coded fields.
//
// The approach uses the per-track locality hint to predict where this NAL
// should be in the source, then compares pre-divergence bytes at nearby
// offsets to align. Once aligned, it detects the divergence point and tries
// all 7 possible shift amounts.
//
// Returns a matchedRegion with bitShift and divergenceOffset set, or nil
// if no bit-shift match was found.
func (m *Matcher) tryBitShiftMatch(
	pkt mkv.Packet,
	syncOff int,
	mkvNALData []byte,
	hint *trackLocalityHint,
	nalSize int,
) *matchedRegion {
	m.diagBitShiftAttempts.Add(1)

	// Need a valid locality hint with lastSrcEnd set
	if hint == nil || !hint.valid.Load() {
		return nil
	}
	lastSrcEnd := hint.lastSrcEnd.Load()
	lastMkvEnd := hint.lastMkvEnd.Load()
	if lastSrcEnd <= 0 || lastMkvEnd <= 0 {
		return nil
	}

	// NAL must be at least large enough to have a header + some shifted data
	if nalSize < bitShiftVerifyLen || len(mkvNALData) < nalSize {
		return nil
	}

	// Predict approximate source ES offset. The MKV and source NALs are
	// packed sequentially on the same track, so the offset delta between
	// consecutive NALs is similar (differing only by framing: AVCC length
	// prefix vs Annex B start code, typically ±1 byte).
	currentMkvOff := pkt.Offset + int64(syncOff)
	predictedSrcOff := lastSrcEnd + (currentMkvOff - lastMkvEnd)
	if predictedSrcOff < 0 {
		return nil
	}

	hintFileIndex := uint16(hint.fileIndex.Load())

	// Try to align the predicted offset to the actual NAL header position
	// by comparing pre-divergence bytes. The prediction can be off by a few
	// bytes due to AVCC (4-byte length prefix) vs Annex B (3-4 byte start code)
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
		return nil
	}

	// Read source NAL data at the aligned offset.
	// We need nalSize+1 bytes: nalSize for comparison + 1 for the shift transform.
	srcReadSize := nalSize + 1
	srcLoc := source.Location{
		FileIndex: hintFileIndex,
		Offset:    srcNALOffset,
		IsVideo:   true,
	}
	srcData, err := m.sourceIndex.ReadESDataAt(srcLoc, srcReadSize)
	if err != nil || len(srcData) < srcReadSize {
		return nil
	}

	// Find divergence point: compare MKV vs source byte-by-byte.
	// The NAL header byte and possibly a few slice header bytes match exactly.
	divergeAt := -1
	for i := 0; i < nalSize && i < len(srcData); i++ {
		if mkvNALData[i] != srcData[i] {
			divergeAt = i
			break
		}
	}

	// If no divergence found, this is actually a normal match (shouldn't happen
	// since hash lookup already failed, but handle gracefully).
	if divergeAt < 0 {
		return nil
	}

	// If divergence is at byte 0, the NAL header doesn't match — wrong source location.
	// If divergence is too far in, this isn't a header modification.
	if divergeAt == 0 || divergeAt > maxDivergenceOffset {
		return nil
	}

	// Try each shift amount 1-7. The relationship after divergence is:
	// mkv[j] = (src[j] << shift) | (src[j+1] >> (8 - shift))
	verifyLen := bitShiftVerifyLen
	if verifyLen > nalSize-divergeAt {
		verifyLen = nalSize - divergeAt
	}
	// Need verifyLen+1 source bytes for the verify (Verify reads src[j+1])
	if divergeAt+verifyLen+1 > len(srcData) {
		verifyLen = len(srcData) - divergeAt - 1
	}
	if verifyLen < 8 {
		return nil // Too few bytes to reliably verify
	}

	var foundShift uint8
	for shift := uint8(1); shift <= 7; shift++ {
		if bitshift.Verify(srcData[divergeAt:divergeAt+verifyLen+1], shift, mkvNALData[divergeAt:divergeAt+verifyLen]) {
			foundShift = shift
			break
		}
	}

	if foundShift == 0 {
		return nil
	}

	// Full NAL verification with the detected shift
	fullVerifyLen := nalSize - divergeAt
	if divergeAt+fullVerifyLen+1 > len(srcData) {
		fullVerifyLen = len(srcData) - divergeAt - 1
	}
	if fullVerifyLen > verifyLen {
		if !bitshift.Verify(srcData[divergeAt:divergeAt+fullVerifyLen+1], foundShift, mkvNALData[divergeAt:divergeAt+fullVerifyLen]) {
			return nil
		}
	}

	// Success! Record as a bit-shifted match.
	mkvStart := pkt.Offset + int64(syncOff)
	mkvEnd := mkvStart + int64(nalSize)

	m.diagBitShiftMatched.Add(1)
	m.diagBitShiftMatchedBytes.Add(int64(nalSize))
	m.diagBitShiftByAmount[foundShift].Add(1)

	return &matchedRegion{
		mkvStart:         mkvStart,
		mkvEnd:           mkvEnd,
		fileIndex:        hintFileIndex,
		srcOffset:        srcNALOffset,
		isVideo:          true,
		audioSubStreamID: 0,
		isLPCM:           false,
		bitShift:         foundShift,
		divergenceOffset: int64(divergeAt),
	}
}
