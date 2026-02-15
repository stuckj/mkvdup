package main

import (
	"io"
	"os"
	"strings"
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

func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { os.Stdout = oldStdout }()
	os.Stdout = w
	f()
	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestPrintVersion(t *testing.T) {
	output := captureStdout(t, func() {
		printVersion()
	})
	if !strings.Contains(output, "mkvdup version") {
		t.Errorf("printVersion() output = %q, want it to contain %q", output, "mkvdup version")
	}
}

func TestPrintUsage(t *testing.T) {
	output := captureStdout(t, func() {
		printUsage()
	})
	for _, want := range []string{"mkvdup", "create", "probe", "mount", "info", "verify", "extract", "reload"} {
		if !strings.Contains(output, want) {
			t.Errorf("printUsage() output missing %q", want)
		}
	}
}

func TestPrintCommandUsage(t *testing.T) {
	tests := []struct {
		cmd      string
		contains []string
	}{
		{"create", []string{"mkv-file", "source-dir"}},
		{"batch-create", []string{"manifest.yaml", "--warn-threshold", "--skip-codec-mismatch", "source_dir"}},
		{"probe", []string{"mkv-file", "source-dir"}},
		{"mount", []string{"mountpoint", "--allow-other"}},
		{"info", []string{"dedup-file"}},
		{"verify", []string{"dedup-file", "original-mkv"}},
		{"extract", []string{"dedup-file", "source-dir", "output-mkv"}},
		{"parse-mkv", []string{"mkv-file"}},
		{"index-source", []string{"source-dir"}},
		{"match", []string{"mkv-file", "source-dir"}},
		{"reload", []string{"pid-file", "SIGHUP"}},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			output := captureStdout(t, func() {
				printCommandUsage(tt.cmd)
			})
			for _, want := range tt.contains {
				if !strings.Contains(output, want) {
					t.Errorf("printCommandUsage(%q) output missing %q", tt.cmd, want)
				}
			}
		})
	}
}

func TestPrintCommandUsage_Unknown(t *testing.T) {
	output := captureStdout(t, func() {
		printCommandUsage("nonexistent")
	})
	if !strings.Contains(output, "mkvdup") {
		t.Errorf("printCommandUsage(%q) output = %q, want it to contain %q", "nonexistent", output, "mkvdup")
	}
}

func TestParseWarnFlags(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantThreshold float64
		wantRemaining []string
	}{
		{
			name:          "defaults",
			args:          []string{"file.mkv", "/source"},
			wantThreshold: 75.0,
			wantRemaining: []string{"file.mkv", "/source"},
		},
		{
			name:          "custom threshold",
			args:          []string{"--warn-threshold", "50", "file.mkv", "/source"},
			wantThreshold: 50.0,
			wantRemaining: []string{"file.mkv", "/source"},
		},
		{
			name:          "flags after positional args",
			args:          []string{"file.mkv", "/source", "--warn-threshold", "60"},
			wantThreshold: 60.0,
			wantRemaining: []string{"file.mkv", "/source"},
		},
		{
			name:          "threshold zero",
			args:          []string{"--warn-threshold", "0", "file.mkv"},
			wantThreshold: 0.0,
			wantRemaining: []string{"file.mkv"},
		},
		{
			name:          "threshold 100",
			args:          []string{"--warn-threshold", "100", "file.mkv"},
			wantThreshold: 100.0,
			wantRemaining: []string{"file.mkv"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			threshold, remaining := parseWarnFlags(tt.args)
			if threshold != tt.wantThreshold {
				t.Errorf("threshold = %f, want %f", threshold, tt.wantThreshold)
			}
			if len(remaining) != len(tt.wantRemaining) {
				t.Errorf("remaining = %v, want %v", remaining, tt.wantRemaining)
				return
			}
			for i := range remaining {
				if remaining[i] != tt.wantRemaining[i] {
					t.Errorf("remaining[%d] = %q, want %q", i, remaining[i], tt.wantRemaining[i])
				}
			}
		})
	}
}
