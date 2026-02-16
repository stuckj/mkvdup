package fuse

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/stuckj/mkvdup/internal/dedup"
)

// ErrorEvent describes a source integrity issue detected by the watcher.
type ErrorEvent struct {
	SourcePath    string   // absolute path of the changed source file
	AffectedFiles []string // virtual file names affected
	Event         string   // "changed", "missing", "size_changed", "checksum_mismatch", "checksum_queue_full"
}

// ErrorNotifier batches integrity error events and executes an external
// command with placeholder substitution. Events are collected for a
// configurable batch interval; when the interval expires, the command
// is executed once with all accumulated events.
type ErrorNotifier struct {
	config dedup.ErrorCommandConfig
	logFn  func(string, ...interface{})

	mu      sync.Mutex
	pending []ErrorEvent
	timer   *time.Timer
	stopped bool
}

// NewErrorNotifier creates a notifier from the given config.
func NewErrorNotifier(config dedup.ErrorCommandConfig, logFn func(string, ...interface{})) *ErrorNotifier {
	if logFn == nil {
		logFn = func(string, ...interface{}) {}
	}
	return &ErrorNotifier{
		config: config,
		logFn:  logFn,
	}
}

// Notify adds an error event to the batch. If this is the first event in
// the batch, a timer is started. Subsequent events reset the timer so that
// rapid bursts are coalesced into a single command execution.
func (n *ErrorNotifier) Notify(event ErrorEvent) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.stopped {
		return
	}

	n.pending = append(n.pending, event)

	// Start or reset the debounce timer.
	if n.timer == nil {
		n.timer = time.AfterFunc(n.config.BatchInterval, n.flush)
	} else {
		n.timer.Reset(n.config.BatchInterval)
	}
}

// Stop flushes any pending events and prevents future notifications.
func (n *ErrorNotifier) Stop() {
	n.mu.Lock()
	n.stopped = true
	if n.timer != nil {
		n.timer.Stop()
		n.timer = nil
	}
	events := n.pending
	n.pending = nil
	n.mu.Unlock()

	if len(events) > 0 {
		n.executeCommand(events)
	}
}

// flush is called when the debounce timer fires.
func (n *ErrorNotifier) flush() {
	n.mu.Lock()
	events := n.pending
	n.pending = nil
	n.timer = nil
	n.mu.Unlock()

	if len(events) > 0 {
		n.executeCommand(events)
	}
}

// executeCommand runs the configured external command with placeholders
// substituted from the batched events. The command runs with a timeout
// and its output is logged on failure.
func (n *ErrorNotifier) executeCommand(events []ErrorEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), n.config.Timeout)
	defer cancel()

	var cmd *exec.Cmd
	if n.config.Command.IsShell {
		// String form: run via sh -c
		cmdStr := substitutePlaceholders(n.config.Command.Args[0], events)
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
	} else {
		// List form: substitute placeholders in each argument
		args := make([]string, len(n.config.Command.Args))
		for i, arg := range n.config.Command.Args {
			args[i] = substitutePlaceholders(arg, events)
		}
		cmd = exec.CommandContext(ctx, args[0], args[1:]...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		n.logFn("source-watch: on_error_command failed: %v (output: %s)", err, strings.TrimSpace(string(output)))
	}
}

// substitutePlaceholders replaces %source%, %files%, and %event% in s
// with values derived from the batched events.
func substitutePlaceholders(s string, events []ErrorEvent) string {
	// Build source list (newline-separated, deduplicated)
	sourceSet := make(map[string]bool)
	var sources []string
	for _, e := range events {
		if !sourceSet[e.SourcePath] {
			sourceSet[e.SourcePath] = true
			sources = append(sources, e.SourcePath)
		}
	}

	// Build file list (comma-separated, deduplicated)
	fileSet := make(map[string]bool)
	var files []string
	for _, e := range events {
		for _, f := range e.AffectedFiles {
			if !fileSet[f] {
				fileSet[f] = true
				files = append(files, f)
			}
		}
	}

	// Build event list
	var eventStrs []string
	if len(events) == 1 {
		eventStrs = append(eventStrs, events[0].Event)
	} else {
		for _, e := range events {
			eventStrs = append(eventStrs, fmt.Sprintf("%s: %s", e.SourcePath, e.Event))
		}
	}

	s = strings.ReplaceAll(s, "%source%", strings.Join(sources, "\n"))
	s = strings.ReplaceAll(s, "%files%", strings.Join(files, ", "))
	s = strings.ReplaceAll(s, "%event%", strings.Join(eventStrs, "\n"))
	return s
}
