package source

import (
	"fmt"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/mmap"
	"golang.org/x/sys/unix"
)

// mmapRawReader wraps mmap.File to implement RawReader interface.
type mmapRawReader struct {
	mmapFile *mmap.File
}

func (r *mmapRawReader) ReadAt(buf []byte, offset int64) (int, error) {
	data := r.mmapFile.Slice(offset, len(buf))
	if data == nil {
		return 0, fmt.Errorf("offset out of range")
	}
	copy(buf, data)
	return len(data), nil
}

// Slice returns a zero-copy slice of the underlying mmap'd data.
func (r *mmapRawReader) Slice(offset int64, size int) []byte {
	return r.mmapFile.Slice(offset, size)
}

func (r *mmapRawReader) Len() int {
	return r.mmapFile.Len()
}

func (r *mmapRawReader) Close() error {
	return r.mmapFile.Close()
}

// indexRawFile processes a raw file (for non-DVD, non-Blu-ray formats).
// Processes the file in a single pass: computes checksum and indexes sync points
// together in chunks, releasing mmap pages as they're processed.
func (idx *Indexer) indexRawFile(fileIndex uint16, path string, size int64, progress func(int64)) (uint64, error) {
	mmapFile, err := mmap.Open(path)
	if err != nil {
		return 0, fmt.Errorf("mmap open: %w", err)
	}
	idx.index.RawReaders = append(idx.index.RawReaders, &mmapRawReader{mmapFile: mmapFile})

	mmapFile.Advise(unix.MADV_SEQUENTIAL)
	data := mmapFile.Data()

	return idx.indexRawFileData(fileIndex, mmapFile, data, size, progress)
}

// indexRawFileData is the core of indexRawFile operating on already-opened mmap data.
// Used as a fallback when M2TS packet structure cannot be detected.
func (idx *Indexer) indexRawFileData(fileIndex uint16, mmapFile *mmap.File, data []byte, size int64, progress func(int64)) (uint64, error) {
	hasher := xxhash.New()
	const chunkSize = 64 * 1024 * 1024
	const overlap = 3
	pageSize := unix.Getpagesize()
	checksumPos := 0

	for chunkStart := 0; chunkStart < len(data); {
		chunkEnd := chunkStart + chunkSize
		if chunkEnd > len(data) {
			chunkEnd = len(data)
		}

		chunk := data[chunkStart:chunkEnd]

		if chunkEnd > checksumPos {
			hasher.Write(data[checksumPos:chunkEnd])
			checksumPos = chunkEnd
		}

		videoOffsets := FindVideoNALStartsInRange(chunk, chunkStart)
		audioOffsets := FindAudioSyncPointsInRange(chunk, chunkStart)

		for _, offset := range videoOffsets {
			if offset+idx.windowSize > len(data) {
				continue
			}
			window := data[offset : offset+idx.windowSize]
			hash := xxhash.Sum64(window)
			idx.index.HashToLocations[hash] = append(idx.index.HashToLocations[hash], Location{
				FileIndex: fileIndex,
				Offset:    int64(offset),
			})
		}

		for _, offset := range audioOffsets {
			if offset+idx.windowSize > len(data) {
				continue
			}
			window := data[offset : offset+idx.windowSize]
			hash := xxhash.Sum64(window)
			idx.index.HashToLocations[hash] = append(idx.index.HashToLocations[hash], Location{
				FileIndex: fileIndex,
				Offset:    int64(offset),
			})
		}

		if progress != nil {
			progress(int64(chunkEnd))
		}

		releaseUpTo := (chunkStart / pageSize) * pageSize
		if releaseUpTo > 0 {
			unix.Madvise(data[:releaseUpTo], unix.MADV_DONTNEED)
		}

		if chunkEnd >= len(data) {
			break
		}
		chunkStart = chunkEnd - overlap
	}

	checksum := hasher.Sum64()
	mmapFile.Advise(unix.MADV_RANDOM)
	return checksum, nil
}
