package source

import (
	"bytes"
	"fmt"
)

// binarySearchRanges performs binary search on PES payload ranges to find the one
// containing the given ES offset. Returns the index, or -1 if not found.
func binarySearchRanges(ranges []PESPayloadRange, esOffset int64) int {
	if len(ranges) == 0 {
		return -1
	}

	low, high := 0, len(ranges)-1
	for low <= high {
		mid := (low + high) / 2
		r := ranges[mid]
		if esOffset < r.ESOffset {
			high = mid - 1
		} else if esOffset >= r.ESOffset+int64(r.Size) {
			low = mid + 1
		} else {
			return mid
		}
	}
	return -1
}

// readByteAt reads a single byte from data or multiRegion at the given file offset.
func readByteAt(data []byte, mr *multiRegionData, fileOffset int64) byte {
	if mr != nil {
		return mr.ByteAt(fileOffset)
	}
	return data[fileOffset]
}

// readByteWithHint reads a single byte from a set of PES payload ranges using a hint
// for O(1) sequential access. Returns the byte, the range index for the next hint,
// and success status. Pass rangeHint=-1 to force binary search.
// When mr is non-nil, byte reads use the multi-region data instead of data.
func readByteWithHint(data []byte, mr *multiRegionData, dataSize int64, ranges []PESPayloadRange, esOffset int64, rangeHint int) (byte, int, bool) {
	if len(ranges) == 0 {
		return 0, -1, false
	}

	// Fast path: check if hint is still valid (O(1) check)
	if rangeHint >= 0 && rangeHint < len(ranges) {
		r := ranges[rangeHint]
		if esOffset >= r.ESOffset && esOffset < r.ESOffset+int64(r.Size) {
			offsetInPayload := esOffset - r.ESOffset
			fileOffset := r.FileOffset + offsetInPayload
			if fileOffset >= 0 && fileOffset < dataSize {
				return readByteAt(data, mr, fileOffset), rangeHint, true
			}
		}
		// Check next range (common case when crossing boundaries forward)
		if rangeHint+1 < len(ranges) {
			r = ranges[rangeHint+1]
			if esOffset >= r.ESOffset && esOffset < r.ESOffset+int64(r.Size) {
				offsetInPayload := esOffset - r.ESOffset
				fileOffset := r.FileOffset + offsetInPayload
				if fileOffset >= 0 && fileOffset < dataSize {
					return readByteAt(data, mr, fileOffset), rangeHint + 1, true
				}
			}
		}
		// Check previous range (common case when crossing boundaries backward)
		if rangeHint-1 >= 0 {
			r = ranges[rangeHint-1]
			if esOffset >= r.ESOffset && esOffset < r.ESOffset+int64(r.Size) {
				offsetInPayload := esOffset - r.ESOffset
				fileOffset := r.FileOffset + offsetInPayload
				if fileOffset >= 0 && fileOffset < dataSize {
					return readByteAt(data, mr, fileOffset), rangeHint - 1, true
				}
			}
		}
	}

	// Slow path: binary search
	rangeIdx := binarySearchRanges(ranges, esOffset)
	if rangeIdx < 0 {
		return 0, -1, false
	}

	r := ranges[rangeIdx]
	offsetInPayload := esOffset - r.ESOffset
	fileOffset := r.FileOffset + offsetInPayload
	if fileOffset >= 0 && fileOffset < dataSize {
		return readByteAt(data, mr, fileOffset), rangeIdx, true
	}

	return 0, -1, false
}

// readSliceAt reads a byte slice from data or multiRegion at the given file offset range.
func readSliceAt(data []byte, mr *multiRegionData, fileOffset, endOffset int64) []byte {
	if mr != nil {
		return mr.Slice(fileOffset, endOffset)
	}
	return data[fileOffset:endOffset]
}

// readFromRanges reads data from PES payload ranges starting at the given ES offset.
// Returns a zero-copy slice when data fits in a single range (common case),
// only copies when data spans multiple ranges.
// When mr is non-nil, data reads use the multi-region data instead of data.
func readFromRanges(data []byte, mr *multiRegionData, dataSize int64, ranges []PESPayloadRange, esOffset int64, size int) ([]byte, error) {
	if len(ranges) == 0 {
		return nil, fmt.Errorf("no ranges available")
	}

	// Use binary search to find starting range
	rangeIdx := binarySearchRanges(ranges, esOffset)
	if rangeIdx < 0 {
		rangeIdx = 0
		for rangeIdx < len(ranges) && esOffset >= ranges[rangeIdx].ESOffset+int64(ranges[rangeIdx].Size) {
			rangeIdx++
		}
	}

	if rangeIdx >= len(ranges) {
		return nil, fmt.Errorf("ES offset %d not found in ranges", esOffset)
	}

	r := ranges[rangeIdx]
	if esOffset < r.ESOffset || esOffset >= r.ESOffset+int64(r.Size) {
		return nil, fmt.Errorf("ES offset %d not in range [%d, %d)", esOffset, r.ESOffset, r.ESOffset+int64(r.Size))
	}

	offsetInPayload := esOffset - r.ESOffset
	availableInRange := int64(r.Size) - offsetInPayload

	// Fast path: data fits entirely within this single range (zero-copy)
	if int64(size) <= availableInRange {
		fileOffset := r.FileOffset + offsetInPayload
		endOffset := fileOffset + int64(size)
		if endOffset > dataSize {
			return nil, fmt.Errorf("file offset out of range")
		}
		return readSliceAt(data, mr, fileOffset, endOffset), nil
	}

	// Slow path: data spans multiple ranges — must copy
	result := make([]byte, 0, size)
	remaining := size

	for remaining > 0 && rangeIdx < len(ranges) {
		r := ranges[rangeIdx]

		if esOffset < r.ESOffset {
			break
		}

		if esOffset >= r.ESOffset+int64(r.Size) {
			rangeIdx++
			continue
		}

		offsetInPayload := esOffset - r.ESOffset
		availableInRange := int64(r.Size) - offsetInPayload
		toRead := remaining
		if int64(toRead) > availableInRange {
			toRead = int(availableInRange)
		}

		fileOffset := r.FileOffset + offsetInPayload
		endOffset := fileOffset + int64(toRead)
		if endOffset > dataSize {
			if len(result) > 0 {
				return result, nil
			}
			return nil, fmt.Errorf("failed to read ES data: offset out of range")
		}

		result = append(result, readSliceAt(data, mr, fileOffset, endOffset)...)
		esOffset += int64(toRead)
		remaining -= toRead
		rangeIdx++
	}

	return result, nil
}

// rawRangesFromPESRanges enumerates raw file ranges for a given ES region.
func rawRangesFromPESRanges(ranges []PESPayloadRange, esOffset int64, size int) ([]RawRange, error) {
	if len(ranges) == 0 {
		return nil, fmt.Errorf("no ranges available")
	}

	// Use binary search to find starting range
	rangeIdx := binarySearchRanges(ranges, esOffset)
	if rangeIdx < 0 {
		rangeIdx = 0
		for rangeIdx < len(ranges) && esOffset >= ranges[rangeIdx].ESOffset+int64(ranges[rangeIdx].Size) {
			rangeIdx++
		}
	}

	if rangeIdx >= len(ranges) {
		return nil, fmt.Errorf("ES offset %d not found in ranges", esOffset)
	}

	r := ranges[rangeIdx]
	if esOffset < r.ESOffset || esOffset >= r.ESOffset+int64(r.Size) {
		return nil, fmt.Errorf("ES offset %d not in range [%d, %d)", esOffset, r.ESOffset, r.ESOffset+int64(r.Size))
	}

	var result []RawRange
	remaining := size

	for remaining > 0 && rangeIdx < len(ranges) {
		r := ranges[rangeIdx]

		if esOffset < r.ESOffset {
			break
		}

		if esOffset >= r.ESOffset+int64(r.Size) {
			rangeIdx++
			continue
		}

		offsetInPayload := esOffset - r.ESOffset
		availableInRange := int64(r.Size) - offsetInPayload
		toTake := remaining
		if int64(toTake) > availableInRange {
			toTake = int(availableInRange)
		}

		fileOffset := r.FileOffset + offsetInPayload
		result = append(result, RawRange{
			FileOffset: fileOffset,
			Size:       toTake,
		})

		esOffset += int64(toTake)
		remaining -= toTake
		rangeIdx++
	}

	if remaining > 0 {
		return nil, fmt.Errorf("could not map entire ES region: %d bytes remaining", remaining)
	}

	return result, nil
}

// totalESSizeFromRanges returns the total ES size from a range list.
func totalESSizeFromRanges(ranges []PESPayloadRange) int64 {
	if len(ranges) == 0 {
		return 0
	}
	last := ranges[len(ranges)-1]
	return last.ESOffset + int64(last.Size)
}

// Common error helpers for ESReader implementations.
var errAudioUsesSubStream = fmt.Errorf("audio uses per-sub-stream methods, use ReadAudioSubStreamData")

func errSubStreamNotFound(id byte) error {
	return fmt.Errorf("audio sub-stream 0x%02X not found", id)
}

// buildFilteredVideoRangesFromData creates filtered video ranges (excluding user_data
// sections) from the given raw video ranges. This is the shared implementation used
// by both MPEGPSParser and cellSegmentAdapter.
func buildFilteredVideoRangesFromData(data []byte, dataSize int64, videoRanges []PESPayloadRange) []PESPayloadRange {
	if len(videoRanges) == 0 {
		return nil
	}

	filteredRanges := make([]PESPayloadRange, 0, len(videoRanges))
	var filteredESOffset int64

	for _, rawRange := range videoRanges {
		endOffset := rawRange.FileOffset + int64(rawRange.Size)
		if endOffset > dataSize {
			continue
		}
		rangeData := data[rawRange.FileOffset:endOffset]

		// Scan for user_data (00 00 01 B2) sections within this PES payload
		i := 2
		rangeStart := 0
		for i < len(rangeData)-1 {
			idx := bytes.IndexByte(rangeData[i:], 0x01)
			if idx < 0 {
				break
			}
			pos := i + idx

			if pos >= 2 && pos < len(rangeData)-1 &&
				rangeData[pos-1] == 0x00 && rangeData[pos-2] == 0x00 && rangeData[pos+1] == UserDataStartCode {
				startCodePos := pos - 2
				if startCodePos > rangeStart {
					filteredRanges = append(filteredRanges, PESPayloadRange{
						FileOffset: rawRange.FileOffset + int64(rangeStart),
						Size:       startCodePos - rangeStart,
						ESOffset:   filteredESOffset,
					})
					filteredESOffset += int64(startCodePos - rangeStart)
				}

				i = pos + 2
				for i < len(rangeData)-1 {
					idx := bytes.IndexByte(rangeData[i:], 0x01)
					if idx < 0 {
						i = len(rangeData)
						break
					}
					nextPos := i + idx
					if nextPos >= 2 && rangeData[nextPos-1] == 0x00 && rangeData[nextPos-2] == 0x00 {
						i = nextPos - 2
						break
					}
					i = nextPos + 1
				}
				rangeStart = i
			} else {
				i = pos + 1
			}
		}

		if rangeStart < len(rangeData) {
			filteredRanges = append(filteredRanges, PESPayloadRange{
				FileOffset: rawRange.FileOffset + int64(rangeStart),
				Size:       len(rangeData) - rangeStart,
				ESOffset:   filteredESOffset,
			})
			filteredESOffset += int64(len(rangeData) - rangeStart)
		}
	}

	return filteredRanges
}

// filteredAudioResult holds the output of buildFilteredAudioRangesFromData.
type filteredAudioResult struct {
	RangesBySubStream map[byte][]PESPayloadRange
	SubStreams        []byte        // sub-stream IDs in order of appearance
	LPCMSubStreams    map[byte]bool // which sub-streams are 16-bit LPCM
	LPCMInfo          map[byte]LPCMFrameHeader
}

// buildFilteredAudioRangesFromData creates per-sub-stream filtered audio ranges.
// This is the shared implementation used by both MPEGPSParser and cellSegmentAdapter.
//
// For Private Stream 1 (0xBD), strips the 4-byte PS header (sub-stream ID, frame count,
// first access unit pointer). For LPCM, strips an additional 3-byte frame header.
// For MPEG-1 audio streams (0xC0-0xDF), the payload is raw MP2 data with no header.
//
// When existingLPCM is non-nil, LPCM sub-stream detection is skipped and the provided
// metadata is used instead (for cell segment adapters that share the parser's LPCM info).
func buildFilteredAudioRangesFromData(
	data []byte, dataSize int64,
	audioRanges []PESPayloadRange, audioRangeStreamIDs []byte,
	existingLPCM map[byte]bool,
) filteredAudioResult {
	result := filteredAudioResult{
		RangesBySubStream: make(map[byte][]PESPayloadRange),
		LPCMSubStreams:    make(map[byte]bool),
		LPCMInfo:          make(map[byte]LPCMFrameHeader),
	}

	if len(audioRanges) == 0 {
		return result
	}

	// Copy existing LPCM metadata if provided
	for id, v := range existingLPCM {
		result.LPCMSubStreams[id] = v
	}

	esOffsetBySubStream := make(map[byte]int64)
	seenSubStreams := make(map[byte]bool)

	for i, rawRange := range audioRanges {
		if rawRange.FileOffset >= dataSize {
			continue
		}

		pesStreamID := audioRangeStreamIDs[i]

		// MPEG-1 audio streams (0xC0-0xDF): payload is raw MP2 data, no sub-stream header
		if pesStreamID >= 0xC0 && pesStreamID <= 0xDF {
			if rawRange.Size <= 0 {
				continue
			}
			if !seenSubStreams[pesStreamID] {
				seenSubStreams[pesStreamID] = true
				result.SubStreams = append(result.SubStreams, pesStreamID)
			}
			esOffset := esOffsetBySubStream[pesStreamID]
			result.RangesBySubStream[pesStreamID] = append(result.RangesBySubStream[pesStreamID], PESPayloadRange{
				FileOffset: rawRange.FileOffset,
				Size:       rawRange.Size,
				ESOffset:   esOffset,
			})
			esOffsetBySubStream[pesStreamID] += int64(rawRange.Size)
			continue
		}

		// Private Stream 1 (0xBD): has sub-stream header
		if rawRange.Size < 4 {
			continue
		}

		subStreamID := data[rawRange.FileOffset]

		isAC3 := subStreamID >= 0x80 && subStreamID <= 0x87
		isDTS := subStreamID >= 0x88 && subStreamID <= 0x8F
		isLPCM := subStreamID >= 0xA0 && subStreamID <= 0xA7

		if isAC3 || isDTS || isLPCM {
			if !seenSubStreams[subStreamID] {
				seenSubStreams[subStreamID] = true
				result.SubStreams = append(result.SubStreams, subStreamID)
			}

			if isLPCM {
				if rawRange.Size > LPCMTotalHeaderSize {
					// Parse LPCM header on first packet (only when not using existing metadata)
					if existingLPCM == nil {
						if _, ok := result.LPCMInfo[subStreamID]; !ok {
							headerEnd := rawRange.FileOffset + 4 + LPCMHeaderSize
							if headerEnd > dataSize {
								continue
							}
							headerData := data[rawRange.FileOffset+4 : headerEnd]
							info := ParseLPCMFrameHeader(headerData)
							result.LPCMInfo[subStreamID] = info
							if IsLPCM16Bit(info.Quantization) {
								result.LPCMSubStreams[subStreamID] = true
							}
						}
					}
					esOffset := esOffsetBySubStream[subStreamID]
					result.RangesBySubStream[subStreamID] = append(result.RangesBySubStream[subStreamID], PESPayloadRange{
						FileOffset: rawRange.FileOffset + LPCMTotalHeaderSize,
						Size:       rawRange.Size - LPCMTotalHeaderSize,
						ESOffset:   esOffset,
					})
					esOffsetBySubStream[subStreamID] += int64(rawRange.Size - LPCMTotalHeaderSize)
				}
			} else {
				if rawRange.Size > 4 {
					esOffset := esOffsetBySubStream[subStreamID]
					result.RangesBySubStream[subStreamID] = append(result.RangesBySubStream[subStreamID], PESPayloadRange{
						FileOffset: rawRange.FileOffset + 4,
						Size:       rawRange.Size - 4,
						ESOffset:   esOffset,
					})
					esOffsetBySubStream[subStreamID] += int64(rawRange.Size - 4)
				}
			}
		}
	}

	return result
}

// readLPCMSubStreamData reads LPCM audio data with 16-bit byte-swap transform.
// Handles alignment: if esOffset is odd, reads from the pair-aligned offset,
// swaps, and returns only the requested portion.
func readLPCMSubStreamData(data []byte, dataSize int64, ranges []PESPayloadRange, esOffset int64, size int) ([]byte, error) {
	alignedOffset := esOffset
	trimFront := 0
	if esOffset%2 == 1 {
		alignedOffset = esOffset - 1
		trimFront = 1
	}
	alignedSize := size + trimFront
	trimBack := 0
	if alignedSize%2 == 1 {
		alignedSize++
		trimBack = 1
	}

	raw, err := readFromRanges(data, nil, dataSize, ranges, alignedOffset, alignedSize)
	if err != nil {
		if trimBack > 0 {
			alignedSize--
			trimBack = 0
			raw, err = readFromRanges(data, nil, dataSize, ranges, alignedOffset, alignedSize)
		}
		if err != nil {
			return nil, err
		}
	}

	// readFromRanges may return a zero-copy mmap slice, so clone first
	result := make([]byte, len(raw))
	copy(result, raw)
	TransformLPCM16BE(result)

	start := trimFront
	end := start + size
	if end > len(result) {
		end = len(result)
	}
	return result[start:end], nil
}
