package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stuckj/mkvdup/internal/dedup"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
)

// sourceGroup represents a set of files sharing the same source directory.
type sourceGroup struct {
	sourceDir string
	indices   []int // indices into the manifest Files slice
}

// groupBySource groups batch manifest files by their SourceDir.
// Groups are returned in first-seen order, and file indices within each group
// preserve their original manifest order.
func groupBySource(files []dedup.BatchManifestFile) []sourceGroup {
	var groups []sourceGroup
	seen := map[string]int{} // sourceDir -> index in groups
	for i, f := range files {
		if gi, ok := seen[f.SourceDir]; ok {
			groups[gi].indices = append(groups[gi].indices, i)
		} else {
			seen[f.SourceDir] = len(groups)
			groups = append(groups, sourceGroup{sourceDir: f.SourceDir, indices: []int{i}})
		}
	}
	return groups
}

// codecMismatchAction controls how reportCodecMismatches handles a mismatch.
type codecMismatchAction int

const (
	codecMismatchPrompt   codecMismatchAction = iota // interactive: prompt user
	codecMismatchContinue                            // non-interactive: warn and continue
	codecMismatchSkip                                // skip: warn and signal skip
)

// reportCodecMismatches prints codec mismatch warnings and handles the response
// based on the action: prompt the user, continue without prompting (still logging to stderr),
// or signal a skip. Returns an error if the user declines to continue (prompt mode only).
func reportCodecMismatches(mismatches []source.CodecMismatch, action codecMismatchAction) error {
	if len(mismatches) == 0 {
		return nil
	}

	// Print warning to stderr (always visible, even in quiet mode)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  WARNING: Codec mismatch detected")
	for _, m := range mismatches {
		mkvName := source.CodecTypeName(m.MKVCodecType)
		var sourceNames []string
		for _, sc := range m.SourceCodecs {
			sourceNames = append(sourceNames, source.CodecTypeName(sc))
		}
		fmt.Fprintf(os.Stderr, "    MKV %s:    %s (%s)\n", m.TrackType, mkvName, m.MKVCodecID)
		fmt.Fprintf(os.Stderr, "    Source %s: %s\n", m.TrackType, strings.Join(sourceNames, ", "))
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  Deduplication may produce poor results if the MKV was transcoded.")

	switch action {
	case codecMismatchSkip:
		fmt.Fprintln(os.Stderr, "  Skipping (--skip-codec-mismatch)...")
		fmt.Fprintln(os.Stderr)
		return nil
	case codecMismatchContinue:
		fmt.Fprintln(os.Stderr, "  Continuing (non-interactive mode)...")
		fmt.Fprintln(os.Stderr)
		return nil
	default:
		// Interactive prompt — auto-continue if stdin is not a terminal
		if !isTerminal() {
			fmt.Fprintln(os.Stderr, "  Continuing (non-interactive mode)...")
			fmt.Fprintln(os.Stderr)
			return nil
		}
		fmt.Print("\n  Continue anyway? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			return fmt.Errorf("aborted due to codec mismatch")
		}
		fmt.Println()
		return nil
	}
}

// createBatch processes multiple MKVs from a batch manifest.
// Files are grouped by source directory so each source is indexed once.
// If skipCodecMismatch is true, MKVs with codec mismatches are skipped instead of processed.
func createBatch(manifestPath string, warnThreshold float64, skipCodecMismatch bool) error {
	totalStart := time.Now()

	manifest, err := dedup.ReadBatchManifest(manifestPath)
	if err != nil {
		return err
	}

	groups := groupBySource(manifest.Files)
	multiSource := len(groups) > 1

	if multiSource {
		printInfo("Batch create: %d %s from %d %s\n\n",
			len(manifest.Files), plural(len(manifest.Files), "file", "files"),
			len(groups), plural(len(groups), "source", "sources"))
	} else {
		printInfo("Batch create: %d %s from %s\n\n",
			len(manifest.Files), plural(len(manifest.Files), "file", "files"), groups[0].sourceDir)
	}

	results := make([]*createResult, len(manifest.Files))
	skipReasons := make([]string, len(manifest.Files))
	var totalIndexDuration time.Duration
	processed := 0

	for gi, g := range groups {
		if multiSource {
			if gi > 0 {
				printInfoln()
			}
			fileWord := "files"
			if len(g.indices) == 1 {
				fileWord = "file"
			}
			printInfo("--- Source %d/%d: %s (%d %s) ---\n", gi+1, len(groups), g.sourceDir, len(g.indices), fileWord)
		}

		// Pre-check: skip files whose output already exists (resuming interrupted batch)
		for _, fi := range g.indices {
			f := manifest.Files[fi]
			if _, err := os.Stat(f.Output); err == nil {
				skipReasons[fi] = "output exists"
			}
		}

		// Pre-check: detect source codecs and warn about incompatible MKVs
		// before the expensive indexing step.
		sourceCodecs, codecErr := source.DetectSourceCodecsFromDir(g.sourceDir)
		if codecErr != nil {
			if vw := verboseWriter(); vw != nil {
				fmt.Fprintf(vw, "Note: could not detect source codecs for %s: %v\n", g.sourceDir, codecErr)
			}
			printInfoln()
		} else {
			for _, fi := range g.indices {
				if skipReasons[fi] != "" {
					continue
				}
				f := manifest.Files[fi]
				codecParser, err := mkv.NewParser(f.MKV)
				if err != nil {
					if vw := verboseWriter(); vw != nil {
						fmt.Fprintf(vw, "Note: skipping codec pre-check for %s: %v\n", filepath.Base(f.MKV), err)
					}
					continue
				}
				if err := codecParser.ParseTracksOnly(); err != nil {
					codecParser.Close()
					if vw := verboseWriter(); vw != nil {
						fmt.Fprintf(vw, "Note: skipping codec pre-check for %s: %v\n", filepath.Base(f.MKV), err)
					}
					continue
				}
				mismatches := source.CheckCodecCompatibility(codecParser.Tracks(), sourceCodecs)
				codecParser.Close()
				if skipCodecMismatch && len(mismatches) > 0 {
					reportCodecMismatches(mismatches, codecMismatchSkip)
					skipReasons[fi] = "codec mismatch"
					continue
				}
				if err := reportCodecMismatches(mismatches, codecMismatchContinue); err != nil {
					return err
				}
			}
			printInfoln()
		}

		// Check if all files in this group are already skipped — skip indexing entirely
		allSkipped := true
		for _, fi := range g.indices {
			if skipReasons[fi] == "" {
				allSkipped = false
				break
			}
		}
		if allSkipped {
			for _, fi := range g.indices {
				processed++
				f := manifest.Files[fi]
				printInfo("\n[%d/%d] %s\n", processed, len(manifest.Files), f.MKV)
				results[fi] = newSkipResult(f.MKV, f.Output, skipReasons[fi])
				printSkipStatus(results[fi])
			}
			continue
		}

		// Index this source directory
		indexLabel := "Indexing source directory..."
		if multiSource {
			indexLabel = fmt.Sprintf("Indexing source %d/%d...", gi+1, len(groups))
		}
		indexStart := time.Now()
		indexer, index, err := buildSourceIndex(g.sourceDir, indexLabel)
		totalIndexDuration += time.Since(indexStart)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ERROR indexing %s: %v\n", g.sourceDir, err)
			// Mark non-skipped files in this group as failed
			for _, fi := range g.indices {
				processed++
				f := manifest.Files[fi]
				printInfo("\n[%d/%d] %s\n", processed, len(manifest.Files), f.MKV)
				if skipReasons[fi] != "" {
					results[fi] = newSkipResult(f.MKV, f.Output, skipReasons[fi])
					printSkipStatus(results[fi])
				} else {
					results[fi] = &createResult{MkvPath: f.MKV, Err: fmt.Errorf("index %s: %w", g.sourceDir, err)}
				}
			}
			if gi < len(groups)-1 {
				fmt.Fprintln(os.Stderr, "  Continuing with remaining sources...")
			}
			continue
		}

		// Process files in this group
		for _, fi := range g.indices {
			processed++
			f := manifest.Files[fi]
			printInfo("\n[%d/%d] %s\n", processed, len(manifest.Files), f.MKV)
			if skipReasons[fi] != "" {
				results[fi] = newSkipResult(f.MKV, f.Output, skipReasons[fi])
				printSkipStatus(results[fi])
				continue
			}
			results[fi] = createDedupWithIndex(f.MKV, f.SourceDir, f.Output, f.Name, indexer, index, 1, 4, true, skipCodecMismatch)
			r := results[fi]
			if r.Skipped {
				printSkipStatus(r)
			} else if r.Err != nil {
				fmt.Fprintf(os.Stderr, "  ERROR: %v\n", r.Err)
				if processed < len(manifest.Files) {
					fmt.Fprintln(os.Stderr, "  Continuing with remaining files...")
				}
			} else {
				printInfo("  MKV: %s bytes | Dedup: %s bytes | Savings: %.1f%% | Time: %v\n",
					formatInt(r.MkvSize), formatInt(r.DedupSize), r.Savings, r.Duration.Round(time.Second))
			}
		}
		index.Close()
	}

	// Print summary
	printBatchSummary(results, totalIndexDuration, totalStart, warnThreshold)

	// Return error if any file failed
	for _, r := range results {
		if r.Err != nil {
			return fmt.Errorf("batch create completed with errors")
		}
	}
	return nil
}

// newSkipResult creates a createResult for a skipped file. When the skip reason
// is "output exists", it populates stats from the existing MKV and dedup files.
func newSkipResult(mkvPath, outputPath, reason string) *createResult {
	r := &createResult{MkvPath: mkvPath, Skipped: true, SkipReason: reason}
	if reason == "output exists" {
		r.OutputPath = outputPath
		if mkvStat, err := os.Stat(mkvPath); err == nil {
			r.MkvSize = mkvStat.Size()
		}
		if dedupStat, err := os.Stat(outputPath); err == nil {
			r.DedupSize = dedupStat.Size()
			if r.MkvSize > 0 {
				r.Savings = float64(r.MkvSize-r.DedupSize) / float64(r.MkvSize) * 100
			}
		}
	}
	return r
}

// printSkipStatus prints the per-file skip message during batch processing.
// For "output exists" skips with populated stats, it shows file sizes and savings.
func printSkipStatus(r *createResult) {
	if r.SkipReason == "output exists" && r.MkvSize > 0 {
		printInfo("  Skipping (%s): %s bytes | Dedup: %s bytes | Savings: %.1f%%\n",
			r.SkipReason, formatInt(r.MkvSize), formatInt(r.DedupSize), r.Savings)
	} else {
		printInfo("  Skipping (%s)\n", r.SkipReason)
	}
}

// printBatchSummary prints the aggregate results of a batch create operation.
func printBatchSummary(results []*createResult, indexDuration time.Duration, totalStart time.Time, warnThreshold float64) {
	printInfoln()
	printInfoln("=== Batch Results ===")
	printInfo("Total time: %v (indexing: %v)\n\n", time.Since(totalStart), indexDuration)

	succeeded := 0
	cached := 0
	skipped := 0
	var lowSavings []string
	for _, r := range results {
		if r.Skipped && r.SkipReason == "output exists" {
			// Already-processed files: show as OK with stats
			cached++
			if r.OutputPath != "" {
				printInfo("  OK    %s -> %s (%.1f%% savings) [cached]\n", r.MkvPath, filepath.Base(r.OutputPath), r.Savings)
			} else {
				printInfo("  OK    %s [cached]\n", r.MkvPath)
			}
			if r.Savings < warnThreshold && r.MkvSize > 0 {
				lowSavings = append(lowSavings, fmt.Sprintf("  %s: %.1f%% savings", r.MkvPath, r.Savings))
			}
		} else if r.Skipped {
			printInfo("  SKIP  %s: %s\n", r.MkvPath, r.SkipReason)
			skipped++
		} else if r.Err != nil {
			fmt.Fprintf(os.Stderr, "  FAIL  %s: %v\n", r.MkvPath, r.Err)
		} else {
			printInfo("  OK    %s -> %s (%.1f%% savings)\n", r.MkvPath, filepath.Base(r.OutputPath), r.Savings)
			if r.Savings < warnThreshold {
				lowSavings = append(lowSavings, fmt.Sprintf("  %s: %.1f%% savings", r.MkvPath, r.Savings))
			}
		}
		if !r.Skipped && r.Err == nil || (r.Skipped && r.SkipReason == "output exists") {
			succeeded++
		}
	}
	switch {
	case cached > 0 && skipped > 0:
		printInfo("\nSucceeded: %d/%d (%d cached, %d skipped)\n", succeeded, len(results), cached, skipped)
	case cached > 0:
		printInfo("\nSucceeded: %d/%d (%d cached)\n", succeeded, len(results), cached)
	case skipped > 0:
		printInfo("\nSucceeded: %d/%d (%d skipped)\n", succeeded, len(results), skipped)
	default:
		printInfo("\nSucceeded: %d/%d\n", succeeded, len(results))
	}

	if !quiet && len(lowSavings) > 0 {
		printInfo("\nWARNING: %d %s with space savings below %.0f%%:\n", len(lowSavings), plural(len(lowSavings), "file", "files"), warnThreshold)
		for _, s := range lowSavings {
			printInfoln(s)
		}
		printInfoln("  This may indicate wrong source, transcoded MKV, or very small MKV file.")
	}
}
