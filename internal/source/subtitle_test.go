package source

import (
	"testing"
)

func TestFindPGSSyncPoints(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []int
	}{
		{
			name: "empty",
			data: nil,
			want: nil,
		},
		{
			name: "too short",
			data: []byte{0x16, 0x00},
			want: nil,
		},
		{
			name: "single PCS segment",
			// PCS: type=0x16, size=11
			data: append([]byte{0x16, 0x00, 0x0B}, make([]byte, 11)...),
			want: []int{0},
		},
		{
			name: "PCS + END",
			// PCS: type=0x16, size=5, then END: type=0x80, size=0
			data: append(append([]byte{0x16, 0x00, 0x05}, make([]byte, 5)...), 0x80, 0x00, 0x00),
			want: []int{0, 8},
		},
		{
			name: "all segment types",
			// PCS(0x16, size=2) + WDS(0x17, size=2) + PDS(0x14, size=2) + ODS(0x15, size=2) + END(0x80, size=0)
			data: []byte{
				0x16, 0x00, 0x02, 0xAA, 0xBB, // PCS
				0x17, 0x00, 0x02, 0xCC, 0xDD, // WDS
				0x14, 0x00, 0x02, 0xEE, 0xFF, // PDS
				0x15, 0x00, 0x02, 0x11, 0x22, // ODS
				0x80, 0x00, 0x00, // END
			},
			want: []int{0, 5, 10, 15, 20},
		},
		{
			name: "invalid segment type stops parsing",
			data: []byte{
				0x16, 0x00, 0x02, 0xAA, 0xBB, // PCS
				0x99, 0x00, 0x02, 0xCC, 0xDD, // invalid type
			},
			want: []int{0},
		},
		{
			name: "truncated segment data",
			// PCS with size=100 but only 5 bytes of data available
			data: []byte{0x16, 0x00, 0x64, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE},
			want: []int{0},
		},
		{
			name: "large ODS segment",
			// ODS with large size (1000 bytes) followed by END
			data: func() []byte {
				d := make([]byte, 3+1000+3)
				d[0] = 0x15        // ODS type
				d[1] = 0x03        // size high byte
				d[2] = 0xE8        // size low byte (0x03E8 = 1000)
				d[3+1000] = 0x80   // END type
				d[3+1000+1] = 0x00 // size high
				d[3+1000+2] = 0x00 // size low
				return d
			}(),
			want: []int{0, 1003},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindPGSSyncPoints(tt.data)
			if len(got) != len(tt.want) {
				t.Fatalf("FindPGSSyncPoints() returned %d sync points, want %d: got %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("sync point [%d] = %d, want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestIsValidPGSSegmentType(t *testing.T) {
	valid := []byte{0x14, 0x15, 0x16, 0x17, 0x80}
	for _, b := range valid {
		if !isValidPGSSegmentType(b) {
			t.Errorf("isValidPGSSegmentType(0x%02X) = false, want true", b)
		}
	}

	invalid := []byte{0x00, 0x13, 0x18, 0x7F, 0x81, 0x90, 0xFF}
	for _, b := range invalid {
		if isValidPGSSegmentType(b) {
			t.Errorf("isValidPGSSegmentType(0x%02X) = true, want false", b)
		}
	}
}
