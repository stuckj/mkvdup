package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stuckj/mkvdup/internal/dedup"
	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
)

// parseMKVWithProgress parses an MKV file with progress reporting.
// The phasePrefix is shown during parsing (e.g., "Phase 3/6: Parsing MKV file...").
// Returns the parser (caller must Close it) and an error if any.
func parseMKVWithProgress(mkvPath, phasePrefix string) (*mkv.Parser, time.Duration, error) {
	parser, err := mkv.NewParser(mkvPath)
	if err != nil {
		return nil, 0, fmt.Errorf("create parser: %w", err)
	}

	bar := newProgressBar(phasePrefix, parser.Size(), "bytes")
	parseStart := time.Now()
	if err := parser.Parse(func(processed, total int64) {
		bar.Update(processed)
	}); err != nil {
		bar.Cancel()
		parser.Close()
		return nil, 0, fmt.Errorf("parse MKV: %w", err)
	}
	bar.Finish()
	elapsed := time.Since(parseStart)
	return parser, elapsed, nil
}

// createResult holds per-file statistics from a create operation.
type createResult struct {
	MkvPath        string
	OutputPath     string
	VirtualName    string
	MkvSize        int64
	DedupSize      int64
	MatchedBytes   int64
	UnmatchedBytes int64
	MatchedPackets int
	TotalPackets   int
	IndexEntries   int
	Savings        float64
	Duration       time.Duration
	Err            error
	Skipped        bool   // true when file was skipped (e.g., codec mismatch, output exists)
	SkipReason     string // reason for skipping (shown in summary)
}

// buildSourceIndex indexes a source directory and returns the indexer and index.
// This is the expensive step that should only happen once in batch mode.
// The phasePrefix is shown on the progress bar (e.g., "Phase 2/6: Building source index...").
func buildSourceIndex(sourceDir, phasePrefix string) (*source.Indexer, *source.Index, error) {
	indexer, err := source.NewIndexer(sourceDir, source.DefaultWindowSize)
	if err != nil {
		return nil, nil, fmt.Errorf("create indexer: %w", err)
	}
	indexer.SetVerbose(verbose)

	// We don't know total size until Build starts calling back with it,
	// so create bar with 0 and let first Update set the total.
	bar := newProgressBar(phasePrefix, 0, "bytes")
	err = indexer.Build(func(processed, total int64) {
		if bar.total == 0 && total > 0 {
			bar.total = total
		}
		bar.Update(processed)
	})
	if err != nil {
		bar.Cancel()
		return nil, nil, fmt.Errorf("build index: %w", err)
	}
	bar.Finish()
	index := indexer.Index()
	printInfo("  Indexed %d hashes\n", len(index.HashToLocations))
	if index.UsesESOffsets {
		printInfo("  (Using ES-aware indexing for %v)\n", indexer.SourceType())
	}

	return indexer, index, nil
}

// checkCodecCompatibilityFromDir performs a lightweight codec check using only
// the source directory (no index needed). This runs before the expensive indexing step.
func checkCodecCompatibilityFromDir(tracks []mkv.Track, sourceDir string, nonInteractive bool) error {
	sourceCodecs, err := source.DetectSourceCodecsFromDir(sourceDir)
	if err != nil {
		if verbose {
			fmt.Printf("  Note: could not detect source codecs: %v\n", err)
		}
		return nil
	}

	mismatches := source.CheckCodecCompatibility(tracks, sourceCodecs)
	action := codecMismatchPrompt
	if nonInteractive {
		action = codecMismatchContinue
	}
	return reportCodecMismatches(mismatches, action)
}

// createDedupWithIndex processes a single MKV using a pre-built source index.
// It handles parsing, matching, writing, and verification.
// phaseStart and phaseTotal control phase numbering (e.g., 3,6 for single create; 1,4 for batch).
// If nonInteractive is true, codec mismatch warnings do not prompt the user.
// If skipCodecMismatch is true, the result is marked as Skipped on codec mismatch instead of continuing.
func createDedupWithIndex(mkvPath, sourceDir, outputPath, virtualName string,
	indexer *source.Indexer, index *source.Index, phaseStart, phaseTotal int, nonInteractive, skipCodecMismatch bool) *createResult {
	start := time.Now()
	result := &createResult{
		MkvPath:     mkvPath,
		OutputPath:  outputPath,
		VirtualName: virtualName,
	}

	phaseLabel := func(offset int, label string) string {
		return fmt.Sprintf("Phase %d/%d: %s", phaseStart+offset, phaseTotal, label)
	}

	// Parse MKV
	parser, _, err := parseMKVWithProgress(mkvPath, phaseLabel(0, "Parsing MKV file..."))
	if err != nil {
		result.Err = err
		return result
	}
	defer parser.Close()

	// Fallback codec check using the index (in case the pre-indexing directory-based
	// check was skipped, e.g. detection failure or batch mode with undetectable codecs)
	sourceCodecs, codecErr := source.DetectSourceCodecs(index)
	if codecErr == nil {
		mismatches := source.CheckCodecCompatibility(parser.Tracks(), sourceCodecs)
		if skipCodecMismatch && len(mismatches) > 0 {
			reportCodecMismatches(mismatches, codecMismatchSkip)
			result.Skipped = true
			result.SkipReason = "codec mismatch"
			return result
		}
		action := codecMismatchPrompt
		if nonInteractive {
			action = codecMismatchContinue
		}
		if err := reportCodecMismatches(mismatches, action); err != nil {
			result.Err = err
			return result
		}
	}

	// Calculate MKV checksum
	printInfo("  Calculating MKV checksum...")
	mkvChecksum, err := calculateFileChecksum(mkvPath)
	if err != nil {
		result.Err = fmt.Errorf("calculate MKV checksum: %w", err)
		return result
	}
	printInfo(" done\n")

	// Match packets
	m, err := matcher.NewMatcher(index)
	if err != nil {
		result.Err = fmt.Errorf("create matcher: %w", err)
		return result
	}
	defer m.Close()
	m.SetVerbose(verbose)

	matchBar := newProgressBar(phaseLabel(1, "Matching packets..."), int64(len(parser.Packets())), "packets")
	matchResult, err := m.Match(mkvPath, parser.Packets(), parser.Tracks(), func(processed, total int) {
		matchBar.Update(int64(processed))
	})
	if err != nil {
		matchBar.Cancel()
		result.Err = fmt.Errorf("match: %w", err)
		return result
	}
	defer matchResult.Close()
	matchBar.Finish()

	// Write dedup file
	writer, err := dedup.NewWriter(outputPath)
	if err != nil {
		result.Err = fmt.Errorf("create dedup writer: %w", err)
		return result
	}
	defer writer.Close()

	writer.SetHeader(parser.Size(), mkvChecksum, indexer.SourceType())
	writer.SetCreatorVersion("mkvdup " + version)
	writer.SetSourceFiles(index.Files)

	// For sources with ES offsets, decide between V3 (convert to raw) and V4 (range maps).
	// V4 stores ES offsets with embedded range maps for ES-to-raw translation at read time.
	// V3 converts ES offsets to raw file offsets at write time (simpler, smaller files).
	// V4 is only used for Blu-ray (M2TS) where the TS packet structure makes V3 conversion
	// impractical. DVDs use V3 since MPEG-PS raw offsets are straightforward.
	var esConverters []source.ESRangeConverter
	if index.UsesESOffsets && len(index.ESReaders) > 0 {
		if indexer.SourceType() == source.TypeBluray {
			// V4: use range maps for Blu-ray (preserves ES offsets in entries)
			var rangeMaps []dedup.RangeMapData
			for i, reader := range index.ESReaders {
				if provider, ok := reader.(source.PESRangeProvider); ok {
					rm := dedup.RangeMapData{
						FileIndex:   uint16(i),
						VideoRanges: provider.FilteredVideoRanges(),
					}
					// If this reader provides offset conversion (e.g., ISO adapter),
					// set the converter for range map encoding.
					if adj, ok := reader.(source.FileOffsetAdjuster); ok {
						rm.OffsetFunc = adj.FileOffsetConverter()
					}
					for _, subID := range provider.AudioSubStreams() {
						rm.AudioStreams = append(rm.AudioStreams, dedup.AudioRangeData{
							SubStreamID: subID,
							Ranges:      provider.FilteredAudioRanges(subID),
						})
					}
					rangeMaps = append(rangeMaps, rm)
				}
			}
			if len(rangeMaps) > 0 {
				writer.SetRangeMaps(rangeMaps)
			}
		} else {
			// V3: convert ES offsets to raw offsets for DVDs
			esConverters = make([]source.ESRangeConverter, len(index.ESReaders))
			for i, r := range index.ESReaders {
				if converter, ok := r.(source.ESRangeConverter); ok {
					esConverters[i] = converter
				}
			}
		}
	}

	if err := writer.SetMatchResult(matchResult, esConverters); err != nil {
		os.Remove(outputPath)
		result.Err = fmt.Errorf("set match result: %w", err)
		return result
	}

	// Pre-encode range maps (CPU-intensive) before the progress-tracked write.
	rangeMapSize, err := writer.EncodeRangeMaps()
	if err != nil {
		os.Remove(outputPath)
		result.Err = fmt.Errorf("encode range maps: %w", err)
		return result
	}
	if rangeMapSize > 0 {
		printInfo("  Range maps encoded: %s bytes\n", formatInt(rangeMapSize))
	}

	writeBar := newProgressBar(phaseLabel(2, "Writing dedup file..."), 0, "bytes")
	if err := writer.WriteWithProgress(func(written, total int64) {
		if writeBar.total == 0 && total > 0 {
			writeBar.total = total
		}
		writeBar.Update(written)
	}); err != nil {
		writeBar.Cancel()
		os.Remove(outputPath)
		result.Err = fmt.Errorf("write dedup file: %w", err)
		return result
	}
	writeBar.Finish()

	// Write config file
	configPath := outputPath + ".yaml"
	if err := dedup.WriteConfig(configPath, virtualName, outputPath, sourceDir); err != nil {
		printInfo("  Warning: failed to write config file: %v\n", err)
	} else {
		printInfo("  Config: %s\n", configPath)
	}

	// Verify reconstruction
	verifyPrefix := phaseLabel(3, "Verifying reconstruction...")
	if err := verifyReconstruction(outputPath, sourceDir, mkvPath, index, verbose, verifyPrefix); err != nil {
		printInfo("  WARNING: Verification failed: %v\n", err)
		printInfoln("  Keeping files for debugging")
	}

	// Populate result
	result.MkvSize = parser.Size()
	result.MatchedBytes = matchResult.MatchedBytes
	result.UnmatchedBytes = matchResult.UnmatchedBytes
	result.MatchedPackets = matchResult.MatchedPackets
	result.TotalPackets = matchResult.TotalPackets
	result.IndexEntries = len(matchResult.Entries)

	dedupInfo, _ := os.Stat(outputPath)
	if dedupInfo != nil {
		result.DedupSize = dedupInfo.Size()
		result.Savings = float64(result.MkvSize-result.DedupSize) / float64(result.MkvSize) * 100
	}
	result.Duration = time.Since(start)

	return result
}

// createDedup creates a .mkvdup file from an MKV and source directory.
func createDedup(mkvPath, sourceDir, outputPath, virtualName string, warnThreshold float64, nonInteractive bool) error {
	totalStart := time.Now()

	// Default virtual name
	if virtualName == "" {
		virtualName = filepath.Base(mkvPath)
	}
	// Ensure virtual name has .mkv extension
	if !strings.HasSuffix(strings.ToLower(virtualName), ".mkv") {
		virtualName += ".mkv"
	}

	printInfoln("Creating dedup file...")
	printInfo("  MKV:     %s\n", mkvPath)
	printInfo("  Source:  %s\n", sourceDir)
	printInfo("  Output:  %s\n", outputPath)
	printInfoln()

	// Phase 1: Quick codec compatibility check (only reads MKV track headers, not full file)
	printInfo("Phase 1/6: Checking codec compatibility...")
	codecParser, err := mkv.NewParser(mkvPath)
	if err != nil {
		return fmt.Errorf("open MKV: %w", err)
	}
	if err := codecParser.ParseTracksOnly(); err != nil {
		// Fail open: this fast-path parser can't handle all MKV layouts.
		// Log and continue without the pre-index codec compatibility check.
		log.Printf("Warning: fast MKV track parsing failed for %q: %v; continuing without pre-index codec check", mkvPath, err)
		codecParser.Close()
	} else {
		if err := checkCodecCompatibilityFromDir(codecParser.Tracks(), sourceDir, nonInteractive); err != nil {
			codecParser.Close()
			return err
		}
		codecParser.Close()
	}
	printInfoln(" done")

	// Phase 2: Index source (expensive)
	indexer, index, err := buildSourceIndex(sourceDir, "Phase 2/6: Building source index...")
	if err != nil {
		return err
	}
	defer index.Close()

	// Phase 3-6: Process MKV (re-parses MKV, but parsing is fast relative to indexing)
	result := createDedupWithIndex(mkvPath, sourceDir, outputPath, virtualName, indexer, index, 3, 6, nonInteractive, false)
	if result.Err != nil {
		return result.Err
	}

	// Summary
	printInfoln()
	printInfoln("=== Results ===")
	printInfo("Total time: %v\n", time.Since(totalStart))
	printInfoln()

	printInfo("MKV file size:      %s bytes (%.2f MB)\n", formatInt(result.MkvSize), float64(result.MkvSize)/(1024*1024))
	printInfo("Matched bytes:      %s bytes (%.2f MB, %.1f%%)\n",
		formatInt(result.MatchedBytes), float64(result.MatchedBytes)/(1024*1024),
		float64(result.MatchedBytes)/float64(result.MkvSize)*100)
	printInfo("Delta (unmatched):  %s bytes (%.2f MB, %.1f%%)\n",
		formatInt(result.UnmatchedBytes), float64(result.UnmatchedBytes)/(1024*1024),
		float64(result.UnmatchedBytes)/float64(result.MkvSize)*100)
	printInfoln()

	printInfo("Dedup file size:    %s bytes (%.2f MB)\n", formatInt(result.DedupSize), float64(result.DedupSize)/(1024*1024))
	printInfo("Space savings:      %.1f%%\n", result.Savings)
	printInfoln()

	printInfo("Packets matched:    %s / %s (%.1f%%)\n",
		formatInt(int64(result.MatchedPackets)), formatInt(int64(result.TotalPackets)),
		float64(result.MatchedPackets)/float64(result.TotalPackets)*100)
	printInfo("Index entries:      %s\n", formatInt(int64(result.IndexEntries)))

	// Warning for low savings
	if !quiet && result.Savings < warnThreshold {
		printInfoln()
		printInfo("WARNING: Space savings (%.1f%%) below %.0f%%\n", result.Savings, warnThreshold)
		printInfoln("  This may indicate wrong source, transcoded MKV, or very small MKV file.")
	}

	return nil
}
