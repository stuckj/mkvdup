package source

import "testing"

func TestParseTrueHDAULength(t *testing.T) {
	tests := []struct {
		name   string
		header []byte
		want   int
	}{
		{
			name:   "typical TrueHD AU (960 words = 1920 bytes)",
			header: []byte{0x03, 0xC0}, // 0x3C0 = 960, * 2 = 1920
			want:   1920,
		},
		{
			name:   "minimum AU (1 word = 2 bytes)",
			header: []byte{0x00, 0x01},
			want:   2,
		},
		{
			name:   "max 12-bit value (4095 words = 8190 bytes)",
			header: []byte{0x0F, 0xFF},
			want:   8190,
		},
		{
			name:   "upper nibble masked out",
			header: []byte{0xF3, 0xC0}, // upper 4 bits set, should be masked to 0x3C0
			want:   1920,
		},
		{
			name:   "zero length",
			header: []byte{0x00, 0x00},
			want:   0,
		},
		{
			name:   "too short",
			header: []byte{0x03},
			want:   0,
		},
		{
			name:   "empty",
			header: []byte{},
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTrueHDAULength(tt.header)
			if got != tt.want {
				t.Errorf("ParseTrueHDAULength(%x) = %d, want %d", tt.header, got, tt.want)
			}
		})
	}
}
