package main

import (
	"testing"
	"time"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
		{1073741824, "1.0 GB"},
		{2684354560, "2.5 GB"},
		{4831838208, "4.5 GB"},
	}

	for _, tt := range tests {
		got := formatSize(tt.input)
		if got != tt.want {
			t.Errorf("formatSize(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds int
		want    string
	}{
		{0, "00:00:00"},
		{1, "00:00:01"},
		{59, "00:00:59"},
		{60, "00:01:00"},
		{61, "00:01:01"},
		{3599, "00:59:59"},
		{3600, "01:00:00"},
		{3661, "01:01:01"},
		{86400, "24:00:00"},
	}

	for _, tt := range tests {
		got := formatDuration(time.Duration(tt.seconds) * time.Second)
		if got != tt.want {
			t.Errorf("formatDuration(%ds) = %q, want %q", tt.seconds, got, tt.want)
		}
	}
}

func TestProgressBar_QuietMode(t *testing.T) {
	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	// Should not panic in quiet mode
	bar := newProgressBar("Test phase...", 100, "bytes")
	bar.Update(50)
	bar.Finish()
}

func TestProgressBar_NoProgressMode(t *testing.T) {
	oldShowProgress := showProgress
	oldQuiet := quiet
	showProgress = false
	quiet = false
	defer func() {
		showProgress = oldShowProgress
		quiet = oldQuiet
	}()

	// Should print prefix and done lines but no bar
	output := captureStdout(t, func() {
		bar := newProgressBar("Test phase...", 100, "bytes")
		bar.Update(50)
		bar.Finish()
	})

	if output == "" {
		t.Error("expected some output in no-progress mode")
	}
	if !contains(output, "Test phase...") {
		t.Error("expected prefix in output")
	}
	if !contains(output, "done") {
		t.Error("expected 'done' in output")
	}
}

func TestProgressBar_ZeroTotal(t *testing.T) {
	oldShowProgress := showProgress
	oldQuiet := quiet
	showProgress = true
	quiet = false
	defer func() {
		showProgress = oldShowProgress
		quiet = oldQuiet
	}()

	// Should not panic with zero total
	bar := newProgressBar("Test...", 0, "bytes")
	bar.Update(0)
	bar.Finish()
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

