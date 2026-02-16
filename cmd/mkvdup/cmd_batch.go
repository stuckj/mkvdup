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
		printInfo("Batch create: %d files from %d sources\n\n", len(manifest.Files), len(groups))
	} else {
		printInfo("Batch create: %d files from %s\n\n", len(manifest.Files), groups[0].sourceDir)
	}

	results := make([]*createResult, len(manifest.Files))
	skipSet := make([]bool, len(manifest.Files))
	var totalIndexDuration time.Duration
	processed := 0

	for gi, g := range groups {
		if multiSource {
			printInfo("--- Source %d/%d: %s (%d files) ---\n\n", gi+1, len(groups), g.sourceDir, len(g.indices))
		}

		// Pre-check: detect source codecs and warn about incompatible MKVs
		// before the expensive indexing step.
		sourceCodecs, codecErr := source.DetectSourceCodecsFromDir(g.sourceDir)
		if codecErr != nil {
			if verbose {
				printInfo("Note: could not detect source codecs for %s: %v\n", g.sourceDir, codecErr)
			}
		} else {
			for _, fi := range g.indices {
				f := manifest.Files[fi]
				codecParser, err := mkv.NewParser(f.MKV)
				if err != nil {
					if verbose {
						printInfo("Note: skipping codec pre-check for %s: %v\n", filepath.Base(f.MKV), err)
					}
					continue
				}
				if err := codecParser.ParseTracksOnly(); err != nil {
					codecParser.Close()
					if verbose {
						printInfo("Note: skipping codec pre-check for %s: %v\n", filepath.Base(f.MKV), err)
					}
					continue
				}
				mismatches := source.CheckCodecCompatibility(codecParser.Tracks(), sourceCodecs)
				codecParser.Close()
				if skipCodecMismatch && len(mismatches) > 0 {
					reportCodecMismatches(mismatches, codecMismatchSkip)
					skipSet[fi] = true
					continue
				}
				if err := reportCodecMismatches(mismatches, codecMismatchContinue); err != nil {
					return err
				}
			}
			printInfoln()
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
			// Mark all files in this group as failed
			for _, fi := range g.indices {
				results[fi] = &createResult{MkvPath: manifest.Files[fi].MKV, Err: fmt.Errorf("index %s: %w", g.sourceDir, err)}
				processed++
			}
			fmt.Fprintf(os.Stderr, "  ERROR indexing %s: %v\n", g.sourceDir, err)
			if gi < len(groups)-1 {
				fmt.Fprintln(os.Stderr, "  Continuing with remaining sources...")
			}
			continue
		}

		// Process files in this group
		for _, fi := range g.indices {
			processed++
			f := manifest.Files[fi]
			printInfo("\n[%d/%d] %s\n", processed, len(manifest.Files), filepath.Base(f.MKV))
			if skipSet[fi] {
				results[fi] = &createResult{MkvPath: f.MKV, Skipped: true}
				printInfo("  Skipping (codec mismatch)\n")
				continue
			}
			results[fi] = createDedupWithIndex(f.MKV, f.SourceDir, f.Output, f.Name, indexer, index, 1, 4, true, skipCodecMismatch)
			if results[fi].Skipped {
				printInfo("  Skipping (codec mismatch)\n")
			} else if results[fi].Err != nil {
				fmt.Fprintf(os.Stderr, "  ERROR: %v\n", results[fi].Err)
				if processed < len(manifest.Files) {
					fmt.Fprintln(os.Stderr, "  Continuing with remaining files...")
				}
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

// printBatchSummary prints the aggregate results of a batch create operation.
func printBatchSummary(results []*createResult, indexDuration time.Duration, totalStart time.Time, warnThreshold float64) {
	printInfoln()
	printInfoln("=== Batch Results ===")
	printInfo("Total time: %v (indexing: %v)\n\n", time.Since(totalStart), indexDuration)

	succeeded := 0
	skipped := 0
	var lowSavings []string
	for _, r := range results {
		base := filepath.Base(r.MkvPath)
		if r.Skipped {
			printInfo("  SKIP  %s: codec mismatch\n", base)
			skipped++
		} else if r.Err != nil {
			fmt.Fprintf(os.Stderr, "  FAIL  %s: %v\n", base, r.Err)
		} else {
			printInfo("  OK    %s -> %s (%.1f%% savings)\n", base, filepath.Base(r.OutputPath), r.Savings)
			succeeded++
			if r.Savings < warnThreshold {
				lowSavings = append(lowSavings, fmt.Sprintf("  %s: %.1f%% savings", base, r.Savings))
			}
		}
	}
	if skipped > 0 {
		printInfo("\nSucceeded: %d/%d (%d skipped)\n", succeeded, len(results), skipped)
	} else {
		printInfo("\nSucceeded: %d/%d\n", succeeded, len(results))
	}

	if !quiet && len(lowSavings) > 0 {
		printInfo("\nWARNING: %d file(s) with space savings below %.0f%%:\n", len(lowSavings), warnThreshold)
		for _, s := range lowSavings {
			printInfoln(s)
		}
		printInfoln("  This may indicate wrong source or transcoded MKV.")
	}
}
