package matcher

import (
	"bufio"
	"fmt"
	"os"
)

// Entry represents a region in the MKV file and where its data comes from.
type Entry struct {
	MkvOffset        int64  // Start offset in the MKV file
	Length           int64  // Length of this region
	Source           uint16 // 0 = delta, 1+ = source file index + 1 (supports up to 65535 files)
	SourceOffset     int64  // Offset in source file (or ES offset for ES-based sources)
	IsVideo          bool   // For ES-based sources: whether this is video or audio data
	AudioSubStreamID byte   // For ES-based audio: sub-stream ID (0x80-0x87=AC3, etc.)
	IsLPCM           bool   // True if this is LPCM audio requiring inverse transform on read
	LPCMQuantization byte   // LPCM quantization code (0=16-bit, 1=20-bit, 2=24-bit)
	LPCMChannels     byte   // LPCM channel count minus 1 (0=mono, 1=stereo, ...)
}

// Result contains the results of the matching process.
type Result struct {
	Entries        []Entry      // All entries covering the entire MKV file
	DeltaData      []byte       // Concatenated unique data (for small deltas / tests)
	DeltaFile      *DeltaWriter // File-backed delta data (for large files)
	MatchedBytes   int64        // Total bytes matched to source
	UnmatchedBytes int64        // Total bytes in delta
	MatchedPackets int          // Number of packets that matched
	TotalPackets   int          // Total number of packets processed
}

// DeltaSize returns the total size of delta data.
func (r *Result) DeltaSize() int64 {
	if r.DeltaFile != nil {
		return r.DeltaFile.Size()
	}
	return int64(len(r.DeltaData))
}

// Close cleans up resources held by the result (temp files).
func (r *Result) Close() {
	if r.DeltaFile != nil {
		r.DeltaFile.Close()
		r.DeltaFile = nil
	}
}

// DeltaWriter writes delta data to a temp file to avoid heap accumulation.
type DeltaWriter struct {
	file     *os.File
	buffered *bufio.Writer
	size     int64
}

// NewDeltaWriter creates a DeltaWriter backed by a temp file.
func NewDeltaWriter() (*DeltaWriter, error) {
	f, err := os.CreateTemp("", "mkvdup-delta-*")
	if err != nil {
		return nil, fmt.Errorf("create delta temp file: %w", err)
	}
	return &DeltaWriter{
		file:     f,
		buffered: bufio.NewWriterSize(f, 256*1024),
	}, nil
}

// Write appends data to the delta file.
func (dw *DeltaWriter) Write(data []byte) error {
	n, err := dw.buffered.Write(data)
	dw.size += int64(n)
	return err
}

// Flush ensures all buffered data is written to disk.
func (dw *DeltaWriter) Flush() error {
	return dw.buffered.Flush()
}

// Size returns the total bytes written.
func (dw *DeltaWriter) Size() int64 {
	return dw.size
}

// File returns the underlying file for reading. Must call Flush() first.
func (dw *DeltaWriter) File() *os.File {
	return dw.file
}

// Close removes the temp file.
func (dw *DeltaWriter) Close() {
	if dw.file != nil {
		name := dw.file.Name()
		dw.file.Close()
		os.Remove(name)
		dw.file = nil
	}
}
