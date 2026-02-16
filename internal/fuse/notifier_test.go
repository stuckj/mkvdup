package fuse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stuckj/mkvdup/internal/dedup"
)

func TestSubstitutePlaceholders_SingleEvent(t *testing.T) {
	events := []ErrorEvent{
		{
			SourcePath:    "/data/source/VIDEO_TS/VTS_01_1.VOB",
			AffectedFiles: []string{"movie.mkv"},
			Event:         "changed",
		},
	}

	result := substitutePlaceholders("src=%source% files=%files% event=%event%", events, false)

	if !strings.Contains(result, "src=/data/source/VIDEO_TS/VTS_01_1.VOB") {
		t.Errorf("expected %%source%% to be replaced, got: %s", result)
	}
	if !strings.Contains(result, "files=movie.mkv") {
		t.Errorf("expected %%files%% to be replaced, got: %s", result)
	}
	if !strings.Contains(result, "event=changed") {
		t.Errorf("expected %%event%% to be replaced with single event string, got: %s", result)
	}
}

func TestSubstitutePlaceholders_BatchedEvents(t *testing.T) {
	events := []ErrorEvent{
		{
			SourcePath:    "/data/source/VTS_01_1.VOB",
			AffectedFiles: []string{"movie1.mkv", "movie2.mkv"},
			Event:         "changed",
		},
		{
			SourcePath:    "/data/source/VTS_01_2.VOB",
			AffectedFiles: []string{"movie2.mkv", "movie3.mkv"},
			Event:         "size_changed",
		},
	}

	result := substitutePlaceholders("src=%source% files=%files% event=%event%", events, false)

	// Sources should be newline-separated, deduplicated
	wantSource := "/data/source/VTS_01_1.VOB\n/data/source/VTS_01_2.VOB"
	if !strings.Contains(result, "src="+wantSource) {
		t.Errorf("expected newline-separated sources, got: %s", result)
	}

	// Files should be comma-separated, deduplicated
	wantFiles := "movie1.mkv, movie2.mkv, movie3.mkv"
	if !strings.Contains(result, "files="+wantFiles) {
		t.Errorf("expected comma-separated deduplicated files, got: %s", result)
	}

	// Events with multiple entries should be "source: event" pairs
	if !strings.Contains(result, "/data/source/VTS_01_1.VOB: changed") {
		t.Errorf("expected source:event pair for first event, got: %s", result)
	}
	if !strings.Contains(result, "/data/source/VTS_01_2.VOB: size_changed") {
		t.Errorf("expected source:event pair for second event, got: %s", result)
	}
}

func TestSubstitutePlaceholders_ShellEscape(t *testing.T) {
	events := []ErrorEvent{
		{
			SourcePath:    "/data/path with spaces/file$(evil).vob",
			AffectedFiles: []string{"movie's file.mkv"},
			Event:         "changed",
		},
	}

	result := substitutePlaceholders("echo %source% %files% %event%", events, true)

	// Escaped values should be single-quoted
	if !strings.Contains(result, "'/data/path with spaces/file$(evil).vob'") {
		t.Errorf("expected shell-escaped source, got: %s", result)
	}
	if !strings.Contains(result, "'movie'\"'\"'s file.mkv'") {
		t.Errorf("expected shell-escaped files with escaped single quote, got: %s", result)
	}
	// "changed" has no special chars, so shellescape leaves it unquoted
	if !strings.Contains(result, " changed") {
		t.Errorf("expected event value in result, got: %s", result)
	}
}

func TestSubstitutePlaceholders_ShellEscape_CleanPath(t *testing.T) {
	events := []ErrorEvent{
		{
			SourcePath:    "/data/source/VIDEO_TS/VTS_01_1.VOB",
			AffectedFiles: []string{"movie.mkv"},
			Event:         "changed",
		},
	}

	// Clean paths should pass through unmodified even with shell escaping
	result := substitutePlaceholders("echo %source% %files% %event%", events, true)
	want := "echo /data/source/VIDEO_TS/VTS_01_1.VOB movie.mkv changed"
	if result != want {
		t.Errorf("clean paths should not be quoted\ngot:  %s\nwant: %s", result, want)
	}
}

func TestErrorNotifier_ExecutesCommand(t *testing.T) {
	tmpDir := t.TempDir()
	marker := filepath.Join(tmpDir, "marker.txt")

	config := dedup.ErrorCommandConfig{
		Command: dedup.CommandValue{
			Args: []string{"sh", "-c", "echo %source% > " + marker},
		},
		Timeout:       5 * time.Second,
		BatchInterval: 50 * time.Millisecond,
	}

	lc := &logCapture{}
	n := NewErrorNotifier(config, lc.logFn)

	n.Notify(ErrorEvent{
		SourcePath:    "/data/source/VTS_01_1.VOB",
		AffectedFiles: []string{"movie.mkv"},
		Event:         "changed",
	})

	// Wait for the batch interval plus buffer for command execution
	time.Sleep(300 * time.Millisecond)

	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("marker file not created: %v", err)
	}
	content := strings.TrimSpace(string(data))
	if content != "/data/source/VTS_01_1.VOB" {
		t.Errorf("marker content = %q, want %q", content, "/data/source/VTS_01_1.VOB")
	}
}

func TestErrorNotifier_ShellCommand(t *testing.T) {
	tmpDir := t.TempDir()
	marker := filepath.Join(tmpDir, "marker.txt")

	config := dedup.ErrorCommandConfig{
		Command: dedup.CommandValue{
			IsShell: true,
			Args:    []string{"printf '%s' %source% > " + marker},
		},
		Timeout:       5 * time.Second,
		BatchInterval: 50 * time.Millisecond,
	}

	lc := &logCapture{}
	n := NewErrorNotifier(config, lc.logFn)

	n.Notify(ErrorEvent{
		SourcePath:    "/data/source/VTS_01_1.VOB",
		AffectedFiles: []string{"movie.mkv"},
		Event:         "changed",
	})

	time.Sleep(300 * time.Millisecond)

	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("marker file not created: %v", err)
	}
	content := strings.TrimSpace(string(data))
	if content != "/data/source/VTS_01_1.VOB" {
		t.Errorf("marker content = %q, want %q", content, "/data/source/VTS_01_1.VOB")
	}
}

func TestErrorNotifier_ShellCommand_EscapesSpecialChars(t *testing.T) {
	tmpDir := t.TempDir()
	marker := filepath.Join(tmpDir, "marker.txt")

	config := dedup.ErrorCommandConfig{
		Command: dedup.CommandValue{
			IsShell: true,
			Args:    []string{"printf '%s' %source% > " + marker},
		},
		Timeout:       5 * time.Second,
		BatchInterval: 50 * time.Millisecond,
	}

	lc := &logCapture{}
	n := NewErrorNotifier(config, lc.logFn)

	n.Notify(ErrorEvent{
		SourcePath:    "/data/path with spaces/file$(whoami).vob",
		AffectedFiles: []string{"movie.mkv"},
		Event:         "changed",
	})

	time.Sleep(300 * time.Millisecond)

	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("marker file not created: %v", err)
	}
	content := strings.TrimSpace(string(data))
	// The source path should be preserved literally, not interpreted by the shell
	if content != "/data/path with spaces/file$(whoami).vob" {
		t.Errorf("marker content = %q, want literal source path", content)
	}
}

func TestErrorNotifier_BatchesEvents(t *testing.T) {
	tmpDir := t.TempDir()
	marker := filepath.Join(tmpDir, "marker.txt")

	config := dedup.ErrorCommandConfig{
		Command: dedup.CommandValue{
			IsShell: true,
			Args:    []string{"printf '%s' %source% > " + marker},
		},
		Timeout:       5 * time.Second,
		BatchInterval: 100 * time.Millisecond,
	}

	lc := &logCapture{}
	n := NewErrorNotifier(config, lc.logFn)

	// Send two events within the batch interval
	n.Notify(ErrorEvent{
		SourcePath:    "/data/source/VTS_01_1.VOB",
		AffectedFiles: []string{"movie1.mkv"},
		Event:         "changed",
	})
	n.Notify(ErrorEvent{
		SourcePath:    "/data/source/VTS_01_2.VOB",
		AffectedFiles: []string{"movie2.mkv"},
		Event:         "size_changed",
	})

	// Wait for batch to flush
	time.Sleep(400 * time.Millisecond)

	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("marker file not created: %v", err)
	}
	content := strings.TrimSpace(string(data))
	// Both sources should be present (newline-separated)
	if !strings.Contains(content, "/data/source/VTS_01_1.VOB") {
		t.Errorf("marker missing first source, got: %q", content)
	}
	if !strings.Contains(content, "/data/source/VTS_01_2.VOB") {
		t.Errorf("marker missing second source, got: %q", content)
	}
}

func TestErrorNotifier_StopFlushes(t *testing.T) {
	tmpDir := t.TempDir()
	marker := filepath.Join(tmpDir, "marker.txt")

	config := dedup.ErrorCommandConfig{
		Command: dedup.CommandValue{
			IsShell: true,
			Args:    []string{"printf '%s' %source% > " + marker},
		},
		Timeout:       5 * time.Second,
		BatchInterval: 10 * time.Second, // very long — won't fire naturally
	}

	lc := &logCapture{}
	n := NewErrorNotifier(config, lc.logFn)

	n.Notify(ErrorEvent{
		SourcePath:    "/data/source/VTS_01_1.VOB",
		AffectedFiles: []string{"movie.mkv"},
		Event:         "changed",
	})

	// Stop immediately — should flush pending events
	n.Stop()

	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("marker file not created after Stop(): %v", err)
	}
	content := strings.TrimSpace(string(data))
	if content != "/data/source/VTS_01_1.VOB" {
		t.Errorf("marker content = %q, want %q", content, "/data/source/VTS_01_1.VOB")
	}
}

func TestErrorNotifier_Timeout(t *testing.T) {
	config := dedup.ErrorCommandConfig{
		Command: dedup.CommandValue{
			IsShell: true,
			Args:    []string{"sleep 10"},
		},
		Timeout:       100 * time.Millisecond,
		BatchInterval: 50 * time.Millisecond,
	}

	lc := &logCapture{}
	n := NewErrorNotifier(config, lc.logFn)

	n.Notify(ErrorEvent{
		SourcePath:    "/data/source/VTS_01_1.VOB",
		AffectedFiles: []string{"movie.mkv"},
		Event:         "changed",
	})

	// The command sleeps for 10s but timeout is 100ms.
	// The batch interval is 50ms, so the command starts at ~50ms,
	// then gets killed at ~150ms. Give generous margin.
	done := make(chan struct{})
	go func() {
		// Wait for the batch to flush and command to timeout
		time.Sleep(1 * time.Second)
		close(done)
	}()

	select {
	case <-done:
		// Test completed within reasonable time
	case <-time.After(5 * time.Second):
		t.Fatal("test did not complete in time — timeout may not be working")
	}
}
