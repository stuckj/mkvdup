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
				binary.LittleEndian.PutUint16(r.Source[:], 1)
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
				binary.LittleEndian.PutUint16(r.Source[:], 0) // delta
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
				binary.LittleEndian.PutUint16(r.Source[:], 2)
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
			name: "large source index (>256)",
			raw: func() RawEntry {
				var r RawEntry
				binary.LittleEndian.PutUint64(r.MkvOffset[:], 1000)
				binary.LittleEndian.PutUint64(r.Length[:], 500)
				binary.LittleEndian.PutUint16(r.Source[:], 1000) // > 256 files
				binary.LittleEndian.PutUint64(r.SourceOffset[:], 2000)
				r.ESFlags = 1
				r.AudioSubStreamID = 0
				return r
			}(),
			expected: Entry{
				MkvOffset:        1000,
				Length:           500,
				Source:           1000,
				SourceOffset:     2000,
				IsVideo:          true,
				AudioSubStreamID: 0,
			},
		},
		{
			name: "max values",
			raw: func() RawEntry {
				var r RawEntry
				binary.LittleEndian.PutUint64(r.MkvOffset[:], 0x7FFFFFFFFFFFFFFF) // max int64
				binary.LittleEndian.PutUint64(r.Length[:], 0x100000000)           // 4GB
				binary.LittleEndian.PutUint16(r.Source[:], 65535)                 // max uint16
				binary.LittleEndian.PutUint64(r.SourceOffset[:], 0x7FFFFFFFFFFFFFFF)
				r.ESFlags = 1
				r.AudioSubStreamID = 255
				return r
			}(),
			expected: Entry{
				MkvOffset:        0x7FFFFFFFFFFFFFFF,
				Length:           0x100000000,
				Source:           65535,
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

func TestRawEntrySizeIs28Bytes(t *testing.T) {
	// Verify the RawEntry struct is exactly 28 bytes (matches EntrySize constant)
	// Layout: MkvOffset(8) + Length(8) + Source(2) + SourceOffset(8) + ESFlags(1) + AudioSubStreamID(1) = 28
	var r RawEntry
	expectedSize := 8 + 8 + 2 + 8 + 1 + 1
	if expectedSize != EntrySize {
		t.Errorf("Expected size calculation %d != EntrySize constant %d", expectedSize, EntrySize)
	}

	// Verify the struct layout matches by writing to a byte slice
	buf := make([]byte, EntrySize)
	binary.LittleEndian.PutUint64(buf[0:8], 1234)   // MkvOffset
	binary.LittleEndian.PutUint64(buf[8:16], 5678)  // Length
	binary.LittleEndian.PutUint16(buf[16:18], 999)  // Source (uint16)
	binary.LittleEndian.PutUint64(buf[18:26], 9999) // SourceOffset
	buf[26] = 1                                     // ESFlags
	buf[27] = 0x80                                  // AudioSubStreamID

	// Copy to RawEntry and verify
	copy(r.MkvOffset[:], buf[0:8])
	copy(r.Length[:], buf[8:16])
	copy(r.Source[:], buf[16:18])
	copy(r.SourceOffset[:], buf[18:26])
	r.ESFlags = buf[26]
	r.AudioSubStreamID = buf[27]

	entry := r.ToEntry()
	if entry.MkvOffset != 1234 {
		t.Errorf("MkvOffset = %d, want 1234", entry.MkvOffset)
	}
	if entry.Length != 5678 {
		t.Errorf("Length = %d, want 5678", entry.Length)
	}
	if entry.Source != 999 {
		t.Errorf("Source = %d, want 999", entry.Source)
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

func TestRawEntryLPCMFlags(t *testing.T) {
	tests := []struct {
		name             string
		esFlags          uint8
		wantIsLPCM       bool
		wantQuantization byte
		wantChannels     byte
	}{
		{
			name:       "no LPCM (video only)",
			esFlags:    0x01, // bit 0 = IsVideo
			wantIsLPCM: false,
		},
		{
			name:       "no flags",
			esFlags:    0x00,
			wantIsLPCM: false,
		},
		{
			name:             "LPCM 16-bit stereo",
			esFlags:          0x02 | (0 << 2) | (1 << 4), // IsLPCM | quant=0(16bit) | channels=1(stereo)
			wantIsLPCM:       true,
			wantQuantization: 0,
			wantChannels:     1,
		},
		{
			name:             "LPCM 20-bit 6ch",
			esFlags:          0x02 | (1 << 2) | (5 << 4), // IsLPCM | quant=1(20bit) | channels=5(6ch)
			wantIsLPCM:       true,
			wantQuantization: 1,
			wantChannels:     5,
		},
		{
			name:             "LPCM 24-bit mono",
			esFlags:          0x02 | (2 << 2) | (0 << 4), // IsLPCM | quant=2(24bit) | channels=0(mono)
			wantIsLPCM:       true,
			wantQuantization: 2,
			wantChannels:     0,
		},
		{
			name:             "LPCM with IsVideo (both set)",
			esFlags:          0x03 | (0 << 2) | (1 << 4), // IsVideo + IsLPCM
			wantIsLPCM:       true,
			wantQuantization: 0,
			wantChannels:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r RawEntry
			binary.LittleEndian.PutUint64(r.MkvOffset[:], 100)
			binary.LittleEndian.PutUint64(r.Length[:], 200)
			binary.LittleEndian.PutUint16(r.Source[:], 1)
			binary.LittleEndian.PutUint64(r.SourceOffset[:], 300)
			r.ESFlags = tt.esFlags
			r.AudioSubStreamID = 0xA0

			entry := r.ToEntry()
			if entry.IsLPCM != tt.wantIsLPCM {
				t.Errorf("IsLPCM = %v, want %v", entry.IsLPCM, tt.wantIsLPCM)
			}
			if tt.wantIsLPCM {
				if entry.LPCMQuantization != tt.wantQuantization {
					t.Errorf("LPCMQuantization = %d, want %d", entry.LPCMQuantization, tt.wantQuantization)
				}
				if entry.LPCMChannels != tt.wantChannels {
					t.Errorf("LPCMChannels = %d, want %d", entry.LPCMChannels, tt.wantChannels)
				}
			}
		})
	}
}

func TestLPCMESFlagsRoundTrip(t *testing.T) {
	// Verify that encoding LPCM params into ESFlags and decoding them back
	// produces the original values.
	for quant := byte(0); quant <= 2; quant++ {
		for ch := byte(0); ch <= 7; ch++ {
			var esFlags uint8
			esFlags |= 2 // IsLPCM
			esFlags |= (quant & 0x03) << 2
			esFlags |= (ch & 0x07) << 4

			var r RawEntry
			r.ESFlags = esFlags
			entry := r.ToEntry()

			if !entry.IsLPCM {
				t.Errorf("IsLPCM should be true for quant=%d, ch=%d", quant, ch)
			}
			if entry.LPCMQuantization != quant {
				t.Errorf("quant=%d, ch=%d: LPCMQuantization = %d, want %d", quant, ch, entry.LPCMQuantization, quant)
			}
			if entry.LPCMChannels != ch {
				t.Errorf("quant=%d, ch=%d: LPCMChannels = %d, want %d", quant, ch, entry.LPCMChannels, ch)
			}
		}
	}
}
