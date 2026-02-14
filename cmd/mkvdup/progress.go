package main

import (
	"fmt"
	"strings"
	"time"
)

// progressBar renders an in-place progress bar with ETA.
//
// When showProgress is true (TTY mode), it renders:
//
//	Phase 2/6: Building source index...
//	  [████████████████████░░░░░░░░░░░░░░░░░░░░]  52%  2.3 GB / 4.5 GB  ETA: 00:00:14
//
// On Finish(), the bar line is cleared and replaced with:
//
//	Phase 2/6: Building source index... done (00:00:27)
//
// When showProgress is false, only the prefix and completion line are printed.
// When quiet is true, nothing is printed.
type progressBar struct {
	prefix    string
	total     int64
	processed int64
	startTime time.Time
	lastDraw  time.Time
	unit      string // "bytes" or "packets"
	done      bool
}

const barWidth = 40

// newProgressBar creates and displays a new progress bar.
// The prefix (e.g., "Phase 2/6: Building source index...") is printed immediately.
// Unit should be "bytes" or "packets".
func newProgressBar(prefix string, total int64, unit string) *progressBar {
	p := &progressBar{
		prefix:    prefix,
		total:     total,
		unit:      unit,
		startTime: time.Now(),
	}
	if !quiet {
		fmt.Println(prefix)
	}
	return p
}

// Update sets the current progress and redraws the bar (throttled to 500ms).
func (p *progressBar) Update(processed int64) {
	if p.done || quiet {
		return
	}
	p.processed = processed

	if !showProgress {
		return
	}

	if time.Since(p.lastDraw) < 500*time.Millisecond {
		return
	}
	p.lastDraw = time.Now()
	p.draw()
}

// Finish completes the progress bar and prints the elapsed time.
func (p *progressBar) Finish() {
	if p.done || quiet {
		return
	}
	p.done = true
	elapsed := time.Since(p.startTime)

	if showProgress {
		// Clear the bar line
		fmt.Print("\r" + strings.Repeat(" ", 120) + "\r")
		// Move cursor up one line and overwrite the prefix line with completion
		fmt.Printf("\033[A\r%s done (%s)\n", p.prefix, formatDuration(elapsed))
	} else {
		fmt.Printf("%s done (%s)\n", p.prefix, formatDuration(elapsed))
	}
}

// draw renders the progress bar line.
func (p *progressBar) draw() {
	if p.total <= 0 {
		return
	}

	pct := float64(p.processed) / float64(p.total)
	if pct > 1.0 {
		pct = 1.0
	}

	// Build the bar: [████████░░░░░░░░░░░░]
	filled := int(pct * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	// Build the stats portion
	var stats string
	switch p.unit {
	case "bytes":
		stats = fmt.Sprintf("%s / %s", formatSize(p.processed), formatSize(p.total))
	case "packets":
		stats = fmt.Sprintf("%s / %s", formatInt(p.processed), formatInt(p.total))
	}

	// ETA
	eta := p.eta()

	line := fmt.Sprintf("  [%s] %3.0f%%  %s  ETA: %s", bar, pct*100, stats, eta)

	// Pad to clear any previous longer line
	if len(line) < 120 {
		line += strings.Repeat(" ", 120-len(line))
	}
	fmt.Printf("\r%s", line)
}

// eta calculates the estimated time remaining.
func (p *progressBar) eta() string {
	elapsed := time.Since(p.startTime)

	// Don't show ETA for first 2 seconds or when no progress
	if elapsed < 2*time.Second || p.processed <= 0 || p.total <= 0 {
		return "--:--:--"
	}

	rate := float64(p.processed) / elapsed.Seconds()
	remaining := float64(p.total-p.processed) / rate
	if remaining < 0 {
		remaining = 0
	}

	return formatDuration(time.Duration(remaining * float64(time.Second)))
}

// formatDuration formats a duration as HH:MM:SS.
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// formatSize formats a byte count as a human-readable string.
func formatSize(n int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(gb))
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// printInfo prints informational output, suppressed when quiet is true.
func printInfo(format string, a ...any) {
	if !quiet {
		fmt.Printf(format, a...)
	}
}

// printInfoln prints informational output with a newline, suppressed when quiet is true.
func printInfoln(a ...any) {
	if !quiet {
		fmt.Println(a...)
	}
}
