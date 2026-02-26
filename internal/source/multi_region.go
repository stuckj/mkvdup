package source

import (
	"sort"
	"sync/atomic"
)

// multiRegionData provides a virtual contiguous view over multiple
// non-contiguous byte slices from a memory-mapped ISO. Used for
// multi-extent UDF files where M2TS data is split across
// non-contiguous ISO regions.
type multiRegionData struct {
	regions   []multiRegion
	totalSize int64
	lastIdx   atomic.Int32 // cached region index for fast sequential access
}

type multiRegion struct {
	data         []byte
	logicalStart int64 // cumulative offset in the virtual contiguous view
}

// newMultiRegionData creates a multiRegionData from ISO physical extents.
// Each extent becomes a region backed by a sub-slice of isoData (zero-copy).
func newMultiRegionData(extents []isoPhysicalRange, isoData []byte) *multiRegionData {
	mr := &multiRegionData{
		regions: make([]multiRegion, len(extents)),
	}
	logicalOff := int64(0)
	isoLen := int64(len(isoData))
	for i, ext := range extents {
		end := ext.ISOOffset + ext.Length
		if ext.ISOOffset < 0 || end > isoLen {
			// Clamp to ISO bounds (corrupted/malformed UDF metadata)
			start := ext.ISOOffset
			if start < 0 {
				start = 0
			}
			if end > isoLen {
				end = isoLen
			}
			if start >= end {
				mr.regions[i] = multiRegion{logicalStart: logicalOff}
				continue
			}
			mr.regions[i] = multiRegion{
				data:         isoData[start:end],
				logicalStart: logicalOff,
			}
			logicalOff += end - start
			continue
		}
		mr.regions[i] = multiRegion{
			data:         isoData[ext.ISOOffset:end],
			logicalStart: logicalOff,
		}
		logicalOff += ext.Length
	}
	mr.totalSize = logicalOff
	return mr
}

// Len returns the total logical size across all regions.
func (m *multiRegionData) Len() int64 { return m.totalSize }

// regionFor returns the index of the region containing the given logical offset.
// Returns len(m.regions) if the offset is beyond all regions.
func (m *multiRegionData) regionFor(off int64) int {
	// Fast path: check cached index
	cached := int(m.lastIdx.Load())
	if cached < len(m.regions) {
		r := m.regions[cached]
		if off >= r.logicalStart && off < r.logicalStart+int64(len(r.data)) {
			return cached
		}
	}
	// Binary search
	idx := sort.Search(len(m.regions), func(i int) bool {
		return m.regions[i].logicalStart+int64(len(m.regions[i].data)) > off
	})
	if idx < len(m.regions) {
		m.lastIdx.Store(int32(idx))
	}
	return idx
}

// ByteAt returns the byte at the given logical offset.
// Returns 0 if the offset is out of bounds.
func (m *multiRegionData) ByteAt(off int64) byte {
	if off < 0 || off >= m.totalSize {
		return 0
	}
	idx := m.regionFor(off)
	if idx >= len(m.regions) {
		return 0
	}
	r := m.regions[idx]
	return r.data[off-r.logicalStart]
}

// Slice returns a byte slice for the given logical offset range [off, end).
// Returns a zero-copy sub-slice when the range falls within one region.
// Copies into a new buffer when the range straddles a region boundary.
func (m *multiRegionData) Slice(off, end int64) []byte {
	if off < 0 || end < 0 || off >= end {
		return nil
	}
	idx := m.regionFor(off)
	if idx >= len(m.regions) {
		return nil
	}
	r := m.regions[idx]
	regionOff := off - r.logicalStart
	regionEnd := end - r.logicalStart
	if regionEnd <= int64(len(r.data)) {
		// Fast path: entirely within one region (zero-copy)
		return r.data[regionOff:regionEnd]
	}
	// Slow path: straddles region boundary — copy
	size := int(end - off)
	buf := make([]byte, size)
	copied := copy(buf, r.data[regionOff:])
	for i := idx + 1; i < len(m.regions) && copied < size; i++ {
		r := m.regions[i]
		n := copy(buf[copied:], r.data)
		copied += n
	}
	return buf
}
