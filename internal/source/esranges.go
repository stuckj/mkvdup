package source

import "fmt"

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

// readByteWithHint reads a single byte from a set of PES payload ranges using a hint
// for O(1) sequential access. Returns the byte, the range index for the next hint,
// and success status. Pass rangeHint=-1 to force binary search.
func readByteWithHint(data []byte, dataSize int64, ranges []PESPayloadRange, esOffset int64, rangeHint int) (byte, int, bool) {
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
				return data[fileOffset], rangeHint, true
			}
		}
		// Check next range (common case when crossing boundaries forward)
		if rangeHint+1 < len(ranges) {
			r = ranges[rangeHint+1]
			if esOffset >= r.ESOffset && esOffset < r.ESOffset+int64(r.Size) {
				offsetInPayload := esOffset - r.ESOffset
				fileOffset := r.FileOffset + offsetInPayload
				if fileOffset >= 0 && fileOffset < dataSize {
					return data[fileOffset], rangeHint + 1, true
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
					return data[fileOffset], rangeHint - 1, true
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
		return data[fileOffset], rangeIdx, true
	}

	return 0, -1, false
}

// readFromRanges reads data from PES payload ranges starting at the given ES offset.
// Returns a zero-copy slice when data fits in a single range (common case),
// only copies when data spans multiple ranges.
func readFromRanges(data []byte, dataSize int64, ranges []PESPayloadRange, esOffset int64, size int) ([]byte, error) {
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
		return data[fileOffset:endOffset], nil
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

		result = append(result, data[fileOffset:endOffset]...)
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
