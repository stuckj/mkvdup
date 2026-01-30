package main

import (
	"testing"
)

func TestParseUint32(t *testing.T) {
	tests := []struct {
		input   string
		want    uint32
		wantErr bool
	}{
		{"0", 0, false},
		{"1", 1, false},
		{"1000", 1000, false},
		{"4294967295", 4294967295, false},
		{"4294967296", 0, true},
		{"-1", 0, true},
		{"abc", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseUint32(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseUint32(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseUint32(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseOctalMode(t *testing.T) {
	tests := []struct {
		input   string
		want    uint32
		wantErr bool
	}{
		{"0644", 0644, false},
		{"0755", 0755, false},
		{"777", 0777, false},
		{"0444", 0444, false},
		{"0", 0, false},
		{"0000", 0, false},
		{"7777", 07777, false},
		{"8", 0, true},
		{"abc", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseOctalMode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseOctalMode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseOctalMode(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
