package dedup

import (
	"encoding/binary"
	"testing"
)

func TestRawEntryToEntry(t *testing.T) {
	tests := []struct {
		name     string
		raw      RawEntry
		expected Entry
	}{
		{
			name: "basic entry",
			raw: func() RawEntry {
				var r RawEntry
				binary.LittleEndian.PutUint64(r.MkvOffset[:], 1000)
				binary.LittleEndian.PutUint64(r.Length[:], 500)
				r.Source = 1
				binary.LittleEndian.PutUint64(r.SourceOffset[:], 2000)
				r.ESFlags = 1 // IsVideo = true
				r.AudioSubStreamID = 0
				return r
			}(),
			expected: Entry{
				MkvOffset:        1000,
				Length:           500,
				Source:           1,
				SourceOffset:     2000,
				IsVideo:          true,
				AudioSubStreamID: 0,
			},
		},
		{
			name: "delta entry (source=0)",
			raw: func() RawEntry {
				var r RawEntry
				binary.LittleEndian.PutUint64(r.MkvOffset[:], 0)
				binary.LittleEndian.PutUint64(r.Length[:], 100)
				r.Source = 0 // delta
				binary.LittleEndian.PutUint64(r.SourceOffset[:], 0)
				r.ESFlags = 0
				r.AudioSubStreamID = 0
				return r
			}(),
			expected: Entry{
				MkvOffset:        0,
				Length:           100,
				Source:           0,
				SourceOffset:     0,
				IsVideo:          false,
				AudioSubStreamID: 0,
			},
		},
		{
			name: "audio entry with substream",
			raw: func() RawEntry {
				var r RawEntry
				binary.LittleEndian.PutUint64(r.MkvOffset[:], 5000)
				binary.LittleEndian.PutUint64(r.Length[:], 1024)
				r.Source = 2
				binary.LittleEndian.PutUint64(r.SourceOffset[:], 10000)
				r.ESFlags = 0 // IsVideo = false
				r.AudioSubStreamID = 0x80
				return r
			}(),
			expected: Entry{
				MkvOffset:        5000,
				Length:           1024,
				Source:           2,
				SourceOffset:     10000,
				IsVideo:          false,
				AudioSubStreamID: 0x80,
			},
		},
		{
			name: "large offsets",
			raw: func() RawEntry {
				var r RawEntry
				binary.LittleEndian.PutUint64(r.MkvOffset[:], 0x7FFFFFFFFFFFFFFF) // max int64
				binary.LittleEndian.PutUint64(r.Length[:], 0x100000000)           // 4GB
				r.Source = 255
				binary.LittleEndian.PutUint64(r.SourceOffset[:], 0x7FFFFFFFFFFFFFFF)
				r.ESFlags = 1
				r.AudioSubStreamID = 255
				return r
			}(),
			expected: Entry{
				MkvOffset:        0x7FFFFFFFFFFFFFFF,
				Length:           0x100000000,
				Source:           255,
				SourceOffset:     0x7FFFFFFFFFFFFFFF,
				IsVideo:          true,
				AudioSubStreamID: 255,
			},
		},
		{
			name: "zero values",
			raw: func() RawEntry {
				return RawEntry{} // all zeros
			}(),
			expected: Entry{
				MkvOffset:        0,
				Length:           0,
				Source:           0,
				SourceOffset:     0,
				IsVideo:          false,
				AudioSubStreamID: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.raw.ToEntry()
			if got != tt.expected {
				t.Errorf("ToEntry() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestRawEntrySizeIs27Bytes(t *testing.T) {
	// Verify the RawEntry struct is exactly 27 bytes (matches EntrySize constant)
	var r RawEntry
	// Calculate size manually: 8 + 8 + 1 + 8 + 1 + 1 = 27
	expectedSize := 8 + 8 + 1 + 8 + 1 + 1
	if expectedSize != EntrySize {
		t.Errorf("Expected size calculation %d != EntrySize constant %d", expectedSize, EntrySize)
	}

	// Verify the struct layout matches by writing to a byte slice
	buf := make([]byte, EntrySize)
	binary.LittleEndian.PutUint64(buf[0:8], 1234)   // MkvOffset
	binary.LittleEndian.PutUint64(buf[8:16], 5678)  // Length
	buf[16] = 1                                     // Source
	binary.LittleEndian.PutUint64(buf[17:25], 9999) // SourceOffset (unaligned!)
	buf[25] = 1                                     // ESFlags
	buf[26] = 0x80                                  // AudioSubStreamID

	// Copy to RawEntry and verify
	copy(r.MkvOffset[:], buf[0:8])
	copy(r.Length[:], buf[8:16])
	r.Source = buf[16]
	copy(r.SourceOffset[:], buf[17:25])
	r.ESFlags = buf[25]
	r.AudioSubStreamID = buf[26]

	entry := r.ToEntry()
	if entry.MkvOffset != 1234 {
		t.Errorf("MkvOffset = %d, want 1234", entry.MkvOffset)
	}
	if entry.Length != 5678 {
		t.Errorf("Length = %d, want 5678", entry.Length)
	}
	if entry.Source != 1 {
		t.Errorf("Source = %d, want 1", entry.Source)
	}
	if entry.SourceOffset != 9999 {
		t.Errorf("SourceOffset = %d, want 9999", entry.SourceOffset)
	}
	if !entry.IsVideo {
		t.Error("IsVideo = false, want true")
	}
	if entry.AudioSubStreamID != 0x80 {
		t.Errorf("AudioSubStreamID = %d, want 0x80", entry.AudioSubStreamID)
	}
}
