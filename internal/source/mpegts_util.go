package source

// mergeAdjacentRanges merges consecutive PESPayloadRange entries that are
// contiguous in both file offset and ES offset.
func mergeAdjacentRanges(ranges []PESPayloadRange) []PESPayloadRange {
	if len(ranges) <= 1 {
		return ranges
	}
	merged := make([]PESPayloadRange, 0, len(ranges)/2)
	merged = append(merged, ranges[0])
	for i := 1; i < len(ranges); i++ {
		last := &merged[len(merged)-1]
		r := ranges[i]
		if r.FileOffset == last.FileOffset+int64(last.Size) &&
			r.ESOffset == last.ESOffset+int64(last.Size) {
			last.Size += r.Size
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}
