package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// logFile is set by --log-file to duplicate output to a file.
// Console output is unchanged; the log file receives non-TTY-style output
// (milestones instead of progress bars, no ANSI escape sequences).
var logFile *os.File

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
// When showProgress is false (non-TTY), milestone percentages are printed at
// 10% intervals so redirected logs still show progress.
// When quiet is true, nothing is printed to stdout (log file still receives output).
type progressBar struct {
	prefix        string
	total         int64
	processed     int64
	startTime     time.Time
	lastDraw      time.Time
	unit          string // "bytes" or "packets"
	done          bool
	lastMilestone int // last 10% milestone printed (0-10)
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
	if logFile != nil {
		fmt.Fprintln(logFile, prefix)
	}
	return p
}

// Update sets the current progress and redraws the bar (throttled to 500ms).
func (p *progressBar) Update(processed int64) {
	if p.done {
		return
	}
	p.processed = processed

	// Milestone progress for non-TTY stdout and/or log file
	if (!showProgress && !quiet) || logFile != nil {
		p.updateMilestone()
	}

	if quiet || !showProgress {
		return
	}

	if time.Since(p.lastDraw) < 500*time.Millisecond {
		return
	}
	p.lastDraw = time.Now()
	p.draw()
}

// updateMilestone prints percentage milestones at 10% intervals.
// Output goes to stdout (when non-TTY) and/or the log file.
func (p *progressBar) updateMilestone() {
	if p.total <= 0 {
		return
	}

	pct := float64(p.processed) / float64(p.total) * 100
	milestone := int(pct / 10)
	if milestone > 10 {
		milestone = 10
	}
	if milestone <= p.lastMilestone {
		return
	}
	p.lastMilestone = milestone

	elapsed := time.Since(p.startTime)
	line := fmt.Sprintf("  %d%% (%s)\n", milestone*10, formatDuration(elapsed))

	if !showProgress && !quiet {
		fmt.Print(line)
	}
	if logFile != nil {
		fmt.Fprint(logFile, line)
	}
}

// Cancel cleans up a progress bar on error without printing "done".
// It prints a newline to move past any partial bar line. Safe to call
// after Finish() (no-op if already done).
func (p *progressBar) Cancel() {
	if p.done {
		return
	}
	p.done = true
	if !quiet && showProgress {
		// Clear partial bar line and move to next line
		fmt.Print("\r\033[2K\n")
	}
}

// Finish completes the progress bar and prints the elapsed time.
func (p *progressBar) Finish() {
	if p.done {
		return
	}
	p.done = true
	elapsed := time.Since(p.startTime)

	if !quiet {
		if showProgress {
			// Clear the bar line, move up, and overwrite the prefix line with completion
			fmt.Printf("\r\033[2K\033[A\r\033[2K%s done (%s)\n", p.prefix, formatDuration(elapsed))
		} else {
			fmt.Printf("%s done (%s)\n", p.prefix, formatDuration(elapsed))
		}
	}
	if logFile != nil {
		fmt.Fprintf(logFile, "%s done (%s)\n", p.prefix, formatDuration(elapsed))
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

	fmt.Printf("\r\033[2K%s", line)
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

// printInfo prints informational output, suppressed on stdout when quiet is true.
// Always written to logFile if one is open.
func printInfo(format string, a ...any) {
	if !quiet {
		fmt.Printf(format, a...)
	}
	if logFile != nil {
		fmt.Fprintf(logFile, format, a...)
	}
}

// printInfoln prints informational output with a newline, suppressed on stdout when quiet is true.
// Always written to logFile if one is open.
func printInfoln(a ...any) {
	if !quiet {
		fmt.Println(a...)
	}
	if logFile != nil {
		fmt.Fprintln(logFile, a...)
	}
}
