package dedup

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/mmap"
	"github.com/stuckj/mkvdup/internal/source"
)

// Range map constants
const (
	RangeMapMagic = "RNGEMAPX" // 8 bytes
	// rangeMapCoarseStep is how many entries per coarse index slot.
	// Binary search the coarse index, then seek within a block.
	rangeMapCoarseStep = 1024
)

// RangeMapStreamHeader identifies a stream within the range map section.
type RangeMapStreamHeader struct {
	FileIndex   uint16 // Source file index (0-based)
	StreamType  uint8  // 0 = video, 1 = audio
	SubStreamID uint8  // For audio: sub-stream ID
	EntryCount  uint32 // Number of range entries for this stream
}

// rangeMapCoarseEntry is one slot in the coarse ESOffset index.
type rangeMapCoarseEntry struct {
	esOffset     int64 // ES offset at the start of this entry
	fileOffset   int64 // raw file offset of this entry
	entryIndex   int   // logical entry index
	entrySize    int   // payload size of this entry
	byteOff      int   // byte offset in compressed data for next decode
	rleRemaining int   // default entries remaining after this one in current RLE run
}

// rangeMapCursor tracks position during sequential access through a compressed range map.
type rangeMapCursor struct {
	esOff   int64
	fileOff int64
	size    int
	rleRem  int
	pos     int // position in compressed data
}

// StreamRangeMap provides random access to a stream's range map using
// compressed delta+varint+RLE encoded data and a coarse in-memory ESOffset index.
type StreamRangeMap struct {
	compressedData []byte // compressed range data (zero-copy slice from mmap)
	entryCount     int
	defaultGap     int64
	defaultSize    int
	coarse         []rangeMapCoarseEntry // coarse ESOffset index for binary search
	totalSize      int64                 // total ES size (sum of all entry sizes)

	// Sequential read cursor cache — avoids redundant binary search + seeking
	// for reads at or near the previous position (common in FUSE sequential reads).
	// Protected by cursorMu for concurrent FUSE read safety.
	cursorMu          sync.Mutex
	cachedCursor      rangeMapCursor
	cachedCursorValid bool
}

// TotalESSize returns the total size of the elementary stream.
func (sm *StreamRangeMap) TotalESSize() int64 {
	return sm.totalSize
}

// --- Varint / Zigzag helpers ---

func zigzagEncode(v int64) uint64 {
	return uint64((v << 1) ^ (v >> 63))
}

func zigzagDecode(v uint64) int64 {
	return int64(v>>1) ^ -int64(v&1)
}

// --- Compressed encoding ---

// findDefaultsSampleSize is the maximum number of entries to examine when
// determining the most common gap and size. For typical media streams the
// pattern is consistent throughout, so a small sample is sufficient and
// avoids O(N) map operations on streams with hundreds of millions of entries.
const findDefaultsSampleSize = 10000

// findDefaults finds the most common gap and size in a range sequence.
// Uses sampling for large inputs to avoid expensive map operations.
// Returns (0, 0) if ranges are too small or values don't fit in uint16.
func findDefaults(ranges []source.PESPayloadRange) (defaultGap int64, defaultSize int) {
	if len(ranges) < 2 {
		if len(ranges) == 1 {
			return 0, ranges[0].Size
		}
		return 0, 0
	}

	// Sample a prefix — patterns in PES streams are consistent throughout.
	sampleLen := len(ranges)
	if sampleLen > findDefaultsSampleSize {
		sampleLen = findDefaultsSampleSize
	}

	// Count gap frequencies (sample only)
	gapCounts := make(map[int64]int)
	for i := 1; i < sampleLen; i++ {
		prevEnd := ranges[i-1].FileOffset + int64(ranges[i-1].Size)
		gap := ranges[i].FileOffset - prevEnd
		gapCounts[gap]++
	}

	var bestGap int64
	bestGapCount := 0
	for gap, count := range gapCounts {
		if count > bestGapCount {
			bestGap = gap
			bestGapCount = count
		}
	}

	// Count size frequencies (sample only)
	sizeCounts := make(map[int]int)
	for i := 0; i < sampleLen; i++ {
		sizeCounts[ranges[i].Size]++
	}

	var bestSize int
	bestSizeCount := 0
	for size, count := range sizeCounts {
		if count > bestSizeCount {
			bestSize = size
			bestSizeCount = count
		}
	}

	// Clamp to uint16 range for on-disk storage; disable RLE if out of range
	if bestGap < 0 || bestGap > 65535 || bestSize > 65535 {
		return 0, 0
	}

	return bestGap, bestSize
}

// encodeCompressedRanges encodes PES payload ranges using delta+varint+RLE.
//
// Format:
//   - First entry: fileOffset (uvarint) + size (uvarint)
//   - Subsequent entries:
//   - 0x00 + count (uvarint): RLE run of count default entries
//   - (zigzag(delta)+1) (uvarint) + size (uvarint): explicit entry
//
// The +1 shift ensures explicit entries never start with 0x00.
func encodeCompressedRanges(ranges []source.PESPayloadRange, defaultGap int64, defaultSize int) []byte {
	if len(ranges) == 0 {
		return nil
	}

	// Use direct []byte append instead of bytes.Buffer to minimize overhead
	// in the hot loop. Initial capacity is generous for the header; the bulk
	// of the data compresses to very few bytes via RLE.
	out := make([]byte, 0, 256)
	var varintBuf [binary.MaxVarintLen64]byte

	// First entry: always explicit
	n := binary.PutUvarint(varintBuf[:], uint64(ranges[0].FileOffset))
	out = append(out, varintBuf[:n]...)
	n = binary.PutUvarint(varintBuf[:], uint64(ranges[0].Size))
	out = append(out, varintBuf[:n]...)

	// Subsequent entries: RLE or explicit
	rleCount := 0

	for i := 1; i < len(ranges); i++ {
		prevEnd := ranges[i-1].FileOffset + int64(ranges[i-1].Size)
		gap := ranges[i].FileOffset - prevEnd

		if gap == defaultGap && ranges[i].Size == defaultSize {
			rleCount++
		} else {
			// Flush pending RLE
			if rleCount > 0 {
				out = append(out, 0x00)
				n := binary.PutUvarint(varintBuf[:], uint64(rleCount))
				out = append(out, varintBuf[:n]...)
				rleCount = 0
			}

			predicted := prevEnd + defaultGap
			delta := ranges[i].FileOffset - predicted
			encoded := zigzagEncode(delta) + 1

			n := binary.PutUvarint(varintBuf[:], encoded)
			out = append(out, varintBuf[:n]...)
			n = binary.PutUvarint(varintBuf[:], uint64(ranges[i].Size))
			out = append(out, varintBuf[:n]...)
		}
	}

	// Flush final RLE
	if rleCount > 0 {
		out = append(out, 0x00)
		n := binary.PutUvarint(varintBuf[:], uint64(rleCount))
		out = append(out, varintBuf[:n]...)
	}

	return out
}

// --- Compressed decoding ---

// buildStreamRangeMap creates a StreamRangeMap from compressed data.
// It decodes the entire stream once to build a coarse ESOffset index.
func buildStreamRangeMap(compressedData []byte, entryCount int, defaultGap int64, defaultSize int) (*StreamRangeMap, error) {
	if entryCount == 0 {
		return &StreamRangeMap{entryCount: 0}, nil
	}

	sm := &StreamRangeMap{
		compressedData: compressedData,
		entryCount:     entryCount,
		defaultGap:     defaultGap,
		defaultSize:    defaultSize,
	}

	// Build coarse index by iterating through all entries
	coarseCount := (entryCount + rangeMapCoarseStep - 1) / rangeMapCoarseStep
	sm.coarse = make([]rangeMapCoarseEntry, 0, coarseCount)

	// Decode first entry
	pos := 0
	fo, n := binary.Uvarint(compressedData[pos:])
	if n <= 0 {
		return nil, fmt.Errorf("truncated first entry fileOffset")
	}
	pos += n
	sz, n := binary.Uvarint(compressedData[pos:])
	if n <= 0 {
		return nil, fmt.Errorf("truncated first entry size")
	}
	pos += n

	var esOff int64
	fileOff := int64(fo)
	entSize := int(sz)
	rleRem := 0

	// Record coarse entry for entry 0
	sm.coarse = append(sm.coarse, rangeMapCoarseEntry{
		esOffset: 0, fileOffset: fileOff, entryIndex: 0, entrySize: entSize,
		byteOff: pos, rleRemaining: 0,
	})

	// Iterate through entries 1..entryCount-1
	for i := 1; i < entryCount; i++ {
		prevEnd := fileOff + int64(entSize)
		esOff += int64(entSize)

		if rleRem > 0 {
			// Still in RLE run
			fileOff = prevEnd + defaultGap
			entSize = defaultSize
			rleRem--
		} else if pos < len(compressedData) && compressedData[pos] == 0x00 {
			// RLE token
			pos++
			count, n := binary.Uvarint(compressedData[pos:])
			if n <= 0 {
				return nil, fmt.Errorf("truncated RLE count at entry %d", i)
			}
			pos += n
			fileOff = prevEnd + defaultGap
			entSize = defaultSize
			rleRem = int(count) - 1
		} else if pos < len(compressedData) {
			// Explicit entry
			encoded, n := binary.Uvarint(compressedData[pos:])
			if n <= 0 {
				return nil, fmt.Errorf("truncated explicit delta at entry %d", i)
			}
			pos += n
			szv, n := binary.Uvarint(compressedData[pos:])
			if n <= 0 {
				return nil, fmt.Errorf("truncated explicit size at entry %d", i)
			}
			pos += n
			delta := zigzagDecode(encoded - 1)
			fileOff = prevEnd + defaultGap + delta
			entSize = int(szv)
			rleRem = 0
		} else {
			return nil, fmt.Errorf("unexpected end of compressed data at entry %d", i)
		}

		if i%rangeMapCoarseStep == 0 {
			sm.coarse = append(sm.coarse, rangeMapCoarseEntry{
				esOffset: esOff, fileOffset: fileOff, entryIndex: i, entrySize: entSize,
				byteOff: pos, rleRemaining: rleRem,
			})
		}
	}

	sm.totalSize = esOff + int64(entSize)

	return sm, nil
}

// advanceCursor moves the cursor forward by one entry.
func (sm *StreamRangeMap) advanceCursor(c *rangeMapCursor) error {
	prevEnd := c.fileOff + int64(c.size)
	c.esOff += int64(c.size)

	if c.rleRem > 0 {
		c.fileOff = prevEnd + sm.defaultGap
		c.size = sm.defaultSize
		c.rleRem--
		return nil
	}

	if c.pos >= len(sm.compressedData) {
		return fmt.Errorf("unexpected end of compressed data")
	}

	if sm.compressedData[c.pos] == 0x00 {
		c.pos++
		count, n := binary.Uvarint(sm.compressedData[c.pos:])
		if n <= 0 {
			return fmt.Errorf("truncated RLE count")
		}
		c.pos += n
		c.fileOff = prevEnd + sm.defaultGap
		c.size = sm.defaultSize
		c.rleRem = int(count) - 1
	} else {
		encoded, n := binary.Uvarint(sm.compressedData[c.pos:])
		if n <= 0 {
			return fmt.Errorf("truncated explicit delta")
		}
		c.pos += n
		szv, n := binary.Uvarint(sm.compressedData[c.pos:])
		if n <= 0 {
			return fmt.Errorf("truncated explicit size")
		}
		c.pos += n
		delta := zigzagDecode(encoded - 1)
		c.fileOff = prevEnd + sm.defaultGap + delta
		c.size = int(szv)
		c.rleRem = 0
	}

	return nil
}

// seekTo positions a cursor at the entry containing esOffset.
// Uses the cached cursor for O(1) sequential reads, falling back to
// coarse index binary search + seek for random access.
func (sm *StreamRangeMap) seekTo(esOffset int64) (rangeMapCursor, error) {
	// Fast path: check if cached cursor is at or before the target.
	// Lock to get a consistent snapshot of the cached cursor.
	sm.cursorMu.Lock()
	cachedValid := sm.cachedCursorValid
	var cc rangeMapCursor
	if cachedValid {
		cc = sm.cachedCursor // Copy while holding lock
	}
	sm.cursorMu.Unlock()

	if cachedValid {
		curEnd := cc.esOff + int64(cc.size)
		if esOffset >= cc.esOff && esOffset < curEnd {
			// Target is within the cached entry — use directly
			return cc, nil
		}
		if esOffset >= curEnd {
			// Target is ahead of cached cursor — try seeking forward.
			// Only use this path if the target is "close" (within ~2 coarse blocks).
			maxForwardSeek := int64(rangeMapCoarseStep*2) * int64(sm.defaultSize+1)
			if maxForwardSeek > 0 && esOffset-curEnd < maxForwardSeek {
				cur := cc
				for cur.esOff+int64(cur.size) <= esOffset {
					// RLE fast path: use arithmetic to skip directly to the target entry
					// instead of advancing one-by-one through the RLE run.
					if cur.rleRem > 0 && sm.defaultSize > 0 {
						afterCurrent := cur.esOff + int64(cur.size)
						maxRLEES := afterCurrent + int64(cur.rleRem)*int64(sm.defaultSize)
						if esOffset < maxRLEES {
							// k = entries to skip (1-based). k-1 in offset calc positions
							// relative to afterCurrent (the start of entry 1 in the run).
							k := int((esOffset-afterCurrent)/int64(sm.defaultSize)) + 1
							if k > cur.rleRem {
								k = cur.rleRem
							}
							stride := int64(sm.defaultSize) + sm.defaultGap
							cur.esOff = afterCurrent + int64(k-1)*int64(sm.defaultSize)
							cur.fileOff = cur.fileOff + int64(cur.size) + sm.defaultGap + int64(k-1)*stride
							cur.size = sm.defaultSize
							cur.rleRem -= k
							continue
						}
						k := cur.rleRem
						stride := int64(sm.defaultSize) + sm.defaultGap
						cur.esOff = afterCurrent + int64(k-1)*int64(sm.defaultSize)
						cur.fileOff = cur.fileOff + int64(cur.size) + sm.defaultGap + int64(k-1)*stride
						cur.size = sm.defaultSize
						cur.rleRem = 0
						continue
					}
					if err := sm.advanceCursor(&cur); err != nil {
						return rangeMapCursor{}, fmt.Errorf("seek to ES offset %d: %w", esOffset, err)
					}
				}
				return cur, nil
			}
		}
	}

	// Slow path: binary search the coarse index
	blockIdx := sort.Search(len(sm.coarse), func(i int) bool {
		return sm.coarse[i].esOffset > esOffset
	}) - 1
	if blockIdx < 0 {
		blockIdx = 0
	}

	ce := &sm.coarse[blockIdx]
	cur := rangeMapCursor{
		esOff:   ce.esOffset,
		fileOff: ce.fileOffset,
		size:    ce.entrySize,
		rleRem:  ce.rleRemaining,
		pos:     ce.byteOff,
	}

	for cur.esOff+int64(cur.size) <= esOffset {
		if cur.rleRem > 0 && sm.defaultSize > 0 {
			afterCurrent := cur.esOff + int64(cur.size)
			maxRLEES := afterCurrent + int64(cur.rleRem)*int64(sm.defaultSize)
			if esOffset < maxRLEES {
				k := int((esOffset-afterCurrent)/int64(sm.defaultSize)) + 1
				if k > cur.rleRem {
					k = cur.rleRem
				}
				stride := int64(sm.defaultSize) + sm.defaultGap
				cur.esOff = afterCurrent + int64(k-1)*int64(sm.defaultSize)
				cur.fileOff = cur.fileOff + int64(cur.size) + sm.defaultGap + int64(k-1)*stride
				cur.size = sm.defaultSize
				cur.rleRem -= k
				continue
			}
			k := cur.rleRem
			stride := int64(sm.defaultSize) + sm.defaultGap
			cur.esOff = afterCurrent + int64(k-1)*int64(sm.defaultSize)
			cur.fileOff = cur.fileOff + int64(cur.size) + sm.defaultGap + int64(k-1)*stride
			cur.size = sm.defaultSize
			cur.rleRem = 0
			continue
		}
		if err := sm.advanceCursor(&cur); err != nil {
			return rangeMapCursor{}, fmt.Errorf("seek to ES offset %d: %w", esOffset, err)
		}
	}

	return cur, nil
}

// ReadData reads ES data at the given offset, copying into a new buffer.
// Uses the coarse index for fast binary search, RLE arithmetic for fast seeking.
func (sm *StreamRangeMap) ReadData(sourceData []byte, sourceSize int64, esOffset int64, size int) ([]byte, error) {
	if sm.entryCount == 0 {
		return nil, fmt.Errorf("empty range map")
	}

	cur, err := sm.seekTo(esOffset)
	if err != nil {
		return nil, err
	}

	// Read data, potentially spanning multiple entries
	result := make([]byte, 0, size)
	remaining := size

	for remaining > 0 {
		offsetInEntry := esOffset - cur.esOff
		if offsetInEntry < 0 {
			return nil, fmt.Errorf("ES offset gap at ES %d", cur.esOff)
		}

		available := int64(cur.size) - offsetInEntry
		toRead := int64(remaining)
		if toRead > available {
			toRead = available
		}

		srcStart := cur.fileOff + offsetInEntry
		srcEnd := srcStart + toRead
		if srcEnd > sourceSize {
			return nil, fmt.Errorf("source read out of bounds: %d + %d > %d", srcStart, toRead, sourceSize)
		}
		result = append(result, sourceData[srcStart:srcEnd]...)

		remaining -= int(toRead)
		esOffset += toRead

		if remaining > 0 {
			// RLE batch path: batch-copy full entries using stride arithmetic.
			if cur.rleRem > 0 {
				cur.esOff += int64(cur.size)
				cur.fileOff += int64(cur.size) + sm.defaultGap
				cur.size = sm.defaultSize
				cur.rleRem--

				stride := int64(sm.defaultSize) + sm.defaultGap
				defSz := sm.defaultSize
				defSz64 := int64(defSz)

				// Calculate how many full entries we can batch-copy
				batchCount := remaining / defSz
				if maxRLE := cur.rleRem + 1; batchCount > maxRLE {
					batchCount = maxRLE
				}

				if batchCount > 0 {
					lastSrcEnd := cur.fileOff + int64(batchCount-1)*stride + defSz64
					if lastSrcEnd > sourceSize {
						return nil, fmt.Errorf("source read out of bounds: %d > %d",
							lastSrcEnd, sourceSize)
					}
					off := len(result)
					result = result[:off+batchCount*defSz]
					stridedCopy(
						result[off:off+batchCount*defSz],
						sourceData[cur.fileOff:lastSrcEnd],
						batchCount, defSz, int(stride),
					)
					copied := batchCount * defSz
					remaining -= copied
					esOffset += int64(copied)
					if batchCount > 1 {
						advance := batchCount - 1
						cur.esOff += int64(advance) * defSz64
						cur.fileOff += int64(advance) * stride
						cur.rleRem -= advance
					}
				}
				continue
			}

			if err := sm.advanceCursor(&cur); err != nil {
				return nil, fmt.Errorf("read spanning entries: %w", err)
			}
		}
	}

	// Update cached cursor for next sequential read
	sm.cursorMu.Lock()
	sm.cachedCursor = cur
	sm.cachedCursorValid = true
	sm.cursorMu.Unlock()

	return result, nil
}

// ReadDataInto reads ES data at the given offset directly into dest, avoiding allocation.
// Returns the number of bytes written. Uses cached cursor for sequential reads.
//
// The source parameter provides read access to the source file. If source
// implements MmapData, the mmap'd byte slice is used for zero-copy reads.
// Otherwise, source.ReadAt is used (pread path for network filesystems).
func (sm *StreamRangeMap) ReadDataInto(source mmap.SourceFile, esOffset int64, dest []byte) (int, error) {
	if sm.entryCount == 0 {
		return 0, fmt.Errorf("empty range map")
	}

	sourceSize := source.Size()

	// Resolve mmap data once for the zero-copy fast path.
	var sourceData []byte
	if md, ok := source.(mmap.MmapData); ok {
		sourceData = md.Data()
	}

	cur, err := sm.seekTo(esOffset)
	if err != nil {
		return 0, err
	}

	// Read data directly into dest, potentially spanning multiple entries
	written := 0
	remaining := len(dest)

	for remaining > 0 {
		offsetInEntry := esOffset - cur.esOff
		if offsetInEntry < 0 {
			return written, fmt.Errorf("ES offset gap at ES %d", cur.esOff)
		}

		available := int64(cur.size) - offsetInEntry
		toRead := int64(remaining)
		if toRead > available {
			toRead = available
		}

		srcStart := cur.fileOff + offsetInEntry
		srcEnd := srcStart + toRead
		if srcEnd > sourceSize {
			return written, fmt.Errorf("source read out of bounds: %d + %d > %d", srcStart, toRead, sourceSize)
		}
		if sourceData != nil {
			copy(dest[written:], sourceData[srcStart:srcEnd])
		} else {
			if n, err := source.ReadAt(dest[written:written+int(toRead)], srcStart); err != nil && !(n == int(toRead) && err == io.EOF) {
				return written, fmt.Errorf("pread at %d: %w", srcStart, err)
			}
		}

		written += int(toRead)
		remaining -= int(toRead)
		esOffset += toRead

		if remaining > 0 {
			// RLE batch path: when the next entries are in an RLE run,
			// batch-copy full entries using a single strided copy instead of
			// calling copy()/advanceCursor per entry.
			if cur.rleRem > 0 {
				// Advance to next RLE entry (equivalent to one advanceCursor)
				cur.esOff += int64(cur.size)
				cur.fileOff += int64(cur.size) + sm.defaultGap
				cur.size = sm.defaultSize
				cur.rleRem--

				stride := int64(sm.defaultSize) + sm.defaultGap
				defSz := sm.defaultSize
				defSz64 := int64(defSz)

				// Calculate how many full entries we can batch-copy
				batchCount := remaining / defSz
				if maxRLE := cur.rleRem + 1; batchCount > maxRLE {
					batchCount = maxRLE
				}

				if batchCount > 0 {
					// Bounds check the entire batch
					lastSrcEnd := cur.fileOff + int64(batchCount-1)*stride + defSz64
					if lastSrcEnd > sourceSize {
						return written, fmt.Errorf("source read out of bounds: %d > %d",
							lastSrcEnd, sourceSize)
					}
					if sourceData != nil {
						stridedCopy(
							dest[written:written+batchCount*defSz],
							sourceData[cur.fileOff:lastSrcEnd],
							batchCount, defSz, int(stride),
						)
					} else {
						// Pread path: read the contiguous source region into a
						// temp buffer, then strided-copy into dest.
						tmpSize := int(lastSrcEnd - cur.fileOff)
						tmp := make([]byte, tmpSize)
						if n, err := source.ReadAt(tmp, cur.fileOff); err != nil && !(n == tmpSize && err == io.EOF) {
							return written, fmt.Errorf("pread batch at %d: %w", cur.fileOff, err)
						}
						stridedCopy(
							dest[written:written+batchCount*defSz],
							tmp,
							batchCount, defSz, int(stride),
						)
					}
					copied := batchCount * defSz
					written += copied
					remaining -= copied
					esOffset += int64(copied)
					// Position cursor at the last copied entry
					if batchCount > 1 {
						advance := batchCount - 1
						cur.esOff += int64(advance) * defSz64
						cur.fileOff += int64(advance) * stride
						cur.rleRem -= advance
					}
				}
				continue
			}

			if err := sm.advanceCursor(&cur); err != nil {
				return written, fmt.Errorf("read spanning entries: %w", err)
			}
		}
	}

	// Update cached cursor for next sequential read
	sm.cursorMu.Lock()
	sm.cachedCursor = cur
	sm.cachedCursorValid = true
	sm.cursorMu.Unlock()

	return written, nil
}

// --- Deserialization (for Reader) ---

// SourceRangeMaps holds parsed range maps for one source file.
type SourceRangeMaps struct {
	FileIndex uint16
	VideoMap  *StreamRangeMap
	AudioMaps map[byte]*StreamRangeMap // keyed by sub-stream ID
}

// readRangeMapSection parses the range map section from mmap'd data.
// The data slice should point to the start of the range map section.
// Compressed data is zero-copy sliced from the input.
func readRangeMapSection(data []byte) ([]SourceRangeMaps, error) {
	if len(data) < 10 { // magic (8) + source count (2)
		return nil, fmt.Errorf("range map section too small: %d bytes", len(data))
	}

	// Verify magic
	if string(data[:8]) != RangeMapMagic {
		return nil, fmt.Errorf("invalid range map magic: %q", data[:8])
	}
	off := 8

	// Source count
	sourceCount := int(binary.LittleEndian.Uint16(data[off : off+2]))
	off += 2

	result := make([]SourceRangeMaps, 0, sourceCount)

	for s := 0; s < sourceCount; s++ {
		if off+3 > len(data) { // FileIndex (2) + StreamCount (1)
			return nil, fmt.Errorf("truncated range map at source %d", s)
		}

		fileIndex := binary.LittleEndian.Uint16(data[off : off+2])
		off += 2
		streamCount := int(data[off])
		off++

		src := SourceRangeMaps{
			FileIndex: fileIndex,
			AudioMaps: make(map[byte]*StreamRangeMap),
		}

		for st := 0; st < streamCount; st++ {
			if off+8 > len(data) { // StreamHeader size
				return nil, fmt.Errorf("truncated stream header at source %d stream %d", s, st)
			}

			// Parse stream header
			var hdr RangeMapStreamHeader
			_ = binary.LittleEndian.Uint16(data[off : off+2]) // per-stream FileIndex (already tracked per source)
			hdr.StreamType = data[off+2]
			hdr.SubStreamID = data[off+3]
			hdr.EntryCount = binary.LittleEndian.Uint32(data[off+4 : off+8])
			off += 8

			// Read compression parameters
			if off+8 > len(data) { // DefaultGap(2) + DefaultSize(2) + CompressedDataSize(4)
				return nil, fmt.Errorf("truncated compression params at source %d stream %d", s, st)
			}
			defGap := int64(binary.LittleEndian.Uint16(data[off : off+2]))
			off += 2
			defSize := int(binary.LittleEndian.Uint16(data[off : off+2]))
			off += 2
			compSize := int(binary.LittleEndian.Uint32(data[off : off+4]))
			off += 4

			if off+compSize > len(data) {
				return nil, fmt.Errorf("truncated compressed data at source %d stream %d: need %d bytes at offset %d, have %d total",
					s, st, compSize, off, len(data))
			}

			// Zero-copy slice into mmap'd data
			compData := data[off : off+compSize]
			off += compSize

			sm, err := buildStreamRangeMap(compData, int(hdr.EntryCount), defGap, defSize)
			if err != nil {
				return nil, fmt.Errorf("build range map for source %d stream %d: %w", s, st, err)
			}

			if hdr.StreamType == 0 {
				src.VideoMap = sm
			} else {
				src.AudioMaps[hdr.SubStreamID] = sm
			}
		}

		result = append(result, src)
	}

	return result, nil
}

// --- Serialization (for Writer) ---

// RangeMapData holds the range map data for all streams of one source file,
// ready for serialization into the dedup file.
type RangeMapData struct {
	FileIndex    uint16
	VideoRanges  []source.PESPayloadRange
	AudioStreams []AudioRangeData
}

// AudioRangeData holds range data for one audio sub-stream.
type AudioRangeData struct {
	SubStreamID byte
	Ranges      []source.PESPayloadRange
}

// encodeRangeMapSection encodes the entire range map section to a byte buffer.
// This is called before writing to determine the exact size and compute the checksum.
func encodeRangeMapSection(rangeMaps []RangeMapData) ([]byte, error) {
	var buf bytes.Buffer

	// Magic
	buf.Write([]byte(RangeMapMagic))

	// Source count
	var tmp [8]byte
	binary.LittleEndian.PutUint16(tmp[:2], uint16(len(rangeMaps)))
	buf.Write(tmp[:2])

	// Write each source's stream range maps
	for _, rm := range rangeMaps {
		// Count streams
		streamCount := uint8(0)
		if len(rm.VideoRanges) > 0 {
			streamCount++
		}
		streamCount += uint8(len(rm.AudioStreams))

		binary.LittleEndian.PutUint16(tmp[:2], rm.FileIndex)
		buf.Write(tmp[:2])
		buf.WriteByte(streamCount)

		// Video stream
		if len(rm.VideoRanges) > 0 {
			writeCompressedStream(&buf, rm.FileIndex, 0, 0, rm.VideoRanges)
		}

		// Audio streams
		for _, audio := range rm.AudioStreams {
			writeCompressedStream(&buf, rm.FileIndex, 1, audio.SubStreamID, audio.Ranges)
		}
	}

	return buf.Bytes(), nil
}

// writeCompressedStream writes one stream's compressed range data.
func writeCompressedStream(buf *bytes.Buffer, fileIndex uint16, streamType uint8, subStreamID byte, ranges []source.PESPayloadRange) {
	// Stream header: FileIndex(2) + StreamType(1) + SubStreamID(1) + EntryCount(4) = 8 bytes
	var hdrBuf [16]byte
	binary.LittleEndian.PutUint16(hdrBuf[0:2], fileIndex)
	hdrBuf[2] = streamType
	hdrBuf[3] = subStreamID
	binary.LittleEndian.PutUint32(hdrBuf[4:8], uint32(len(ranges)))
	buf.Write(hdrBuf[:8])

	// Find defaults
	defGap, defSize := findDefaults(ranges)

	// Compression parameters: DefaultGap(2) + DefaultSize(2) + CompressedDataSize(4) = 8 bytes
	binary.LittleEndian.PutUint16(hdrBuf[0:2], uint16(defGap))
	binary.LittleEndian.PutUint16(hdrBuf[2:4], uint16(defSize))

	// Encode compressed ranges
	compressed := encodeCompressedRanges(ranges, defGap, defSize)

	binary.LittleEndian.PutUint32(hdrBuf[4:8], uint32(len(compressed)))
	buf.Write(hdrBuf[:8])
	buf.Write(compressed)
}

// writeRangeMapSection writes a pre-encoded range map buffer and returns its checksum.
// Used by the writer to write the range map section to the dedup file.
func writeRangeMapSection(w io.Writer, rangeMapBuf []byte) (uint64, error) {
	hasher := xxhash.New()
	hasher.Write(rangeMapBuf)

	if _, err := w.Write(rangeMapBuf); err != nil {
		return 0, err
	}

	return hasher.Sum64(), nil
}
