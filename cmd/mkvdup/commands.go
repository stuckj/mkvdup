package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"log/syslog"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/stuckj/mkvdup/internal/daemon"
	"github.com/stuckj/mkvdup/internal/dedup"
	mkvfuse "github.com/stuckj/mkvdup/internal/fuse"
	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/mmap"
	"github.com/stuckj/mkvdup/internal/source"
)

// formatInt formats an integer with thousands separators (e.g., 1234567 → "1,234,567").
func formatInt(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	// Insert commas from the right
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// parseMKVWithProgress parses an MKV file with progress reporting.
// The progressPrefix is shown during parsing (e.g., "Parsing MKV...", "Phase 1/3: Parsing MKV file...").
// Returns the parser (caller must Close it) and an error if any.
func parseMKVWithProgress(mkvPath, progressPrefix string) (*mkv.Parser, time.Duration, error) {
	fmt.Print(progressPrefix)
	parser, err := mkv.NewParser(mkvPath)
	if err != nil {
		fmt.Println()
		return nil, 0, fmt.Errorf("create parser: %w", err)
	}

	parseStart := time.Now()
	lastProgress := time.Now()
	if err := parser.Parse(func(processed, total int64) {
		if time.Since(lastProgress) > 500*time.Millisecond {
			pct := float64(processed) / float64(total) * 100
			fmt.Printf("\r%s %.1f%%", progressPrefix, pct)
			lastProgress = time.Now()
		}
	}); err != nil {
		parser.Close()
		fmt.Println()
		return nil, 0, fmt.Errorf("parse MKV: %w", err)
	}
	elapsed := time.Since(parseStart)
	fmt.Printf("\r%s done (%d packets in %v)                    \n", progressPrefix, parser.PacketCount(), elapsed)
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
}

// buildSourceIndex indexes a source directory and returns the indexer and index.
// This is the expensive step that should only happen once in batch mode.
func buildSourceIndex(sourceDir string) (*source.Indexer, *source.Index, error) {
	indexer, err := source.NewIndexer(sourceDir, source.DefaultWindowSize)
	if err != nil {
		return nil, nil, fmt.Errorf("create indexer: %w", err)
	}
	indexer.SetVerbose(verbose)

	start := time.Now()
	lastProgress := time.Now()
	err = indexer.Build(func(processed, total int64) {
		if time.Since(lastProgress) > 500*time.Millisecond {
			pct := float64(processed) / float64(total) * 100
			fmt.Printf("\r  Progress: %.1f%%", pct)
			lastProgress = time.Now()
		}
	})
	if err != nil {
		return nil, nil, fmt.Errorf("build index: %w", err)
	}
	index := indexer.Index()
	fmt.Printf("\r  Indexed %d hashes in %v                    \n", len(index.HashToLocations), time.Since(start))
	if index.UsesESOffsets {
		fmt.Println("  (Using ES-aware indexing for MPEG-PS)")
	}

	return indexer, index, nil
}

// reportCodecMismatches prints codec mismatch warnings and handles user prompting.
// Returns an error if the user declines to continue.
func reportCodecMismatches(mismatches []source.CodecMismatch, nonInteractive bool) error {
	if len(mismatches) == 0 {
		return nil
	}

	// Print warning
	fmt.Println()
	fmt.Println("  WARNING: Codec mismatch detected")
	for _, m := range mismatches {
		mkvName := source.CodecTypeName(m.MKVCodecType)
		var sourceNames []string
		for _, sc := range m.SourceCodecs {
			sourceNames = append(sourceNames, source.CodecTypeName(sc))
		}
		fmt.Printf("    MKV %s:    %s (%s)\n", m.TrackType, mkvName, m.MKVCodecID)
		fmt.Printf("    Source %s: %s\n", m.TrackType, strings.Join(sourceNames, ", "))
	}
	fmt.Println()
	fmt.Println("  Deduplication may produce poor results if the MKV was transcoded.")

	// Determine if we should prompt
	if nonInteractive || !isTerminal() {
		fmt.Println("  Continuing (non-interactive mode)...")
		fmt.Println()
		return nil
	}

	// Interactive prompt
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
	return reportCodecMismatches(mismatches, nonInteractive)
}

// isTerminal returns true if stdin is a terminal (not piped).
func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// createDedupWithIndex processes a single MKV using a pre-built source index.
// It handles parsing, matching, writing, and verification.
// If nonInteractive is true, codec mismatch warnings do not prompt the user.
func createDedupWithIndex(mkvPath, sourceDir, outputPath, virtualName string,
	indexer *source.Indexer, index *source.Index, nonInteractive bool) *createResult {
	start := time.Now()
	result := &createResult{
		MkvPath:     mkvPath,
		OutputPath:  outputPath,
		VirtualName: virtualName,
	}

	// Parse MKV
	parser, _, err := parseMKVWithProgress(mkvPath, "  Parsing MKV file...")
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
		if err := reportCodecMismatches(mismatches, nonInteractive); err != nil {
			result.Err = err
			return result
		}
	}

	// Calculate MKV checksum
	fmt.Print("    Calculating MKV checksum...")
	mkvChecksum, err := calculateFileChecksum(mkvPath)
	if err != nil {
		result.Err = fmt.Errorf("calculate MKV checksum: %w", err)
		return result
	}
	fmt.Printf(" done\n")

	// Match packets
	fmt.Println("  Matching packets...")
	m, err := matcher.NewMatcher(index)
	if err != nil {
		result.Err = fmt.Errorf("create matcher: %w", err)
		return result
	}
	defer m.Close()
	m.SetVerbose(verbose)

	matchStart := time.Now()
	lastProgress := time.Now()
	matchResult, err := m.Match(mkvPath, parser.Packets(), parser.Tracks(), func(processed, total int) {
		if time.Since(lastProgress) > 500*time.Millisecond {
			pct := float64(processed) / float64(total) * 100
			fmt.Printf("\r    Progress: %.1f%% (%d/%d packets)", pct, processed, total)
			lastProgress = time.Now()
		}
	})
	if err != nil {
		result.Err = fmt.Errorf("match: %w", err)
		return result
	}
	defer matchResult.Close()
	fmt.Printf("\r    Matched in %v                              \n", time.Since(matchStart))

	// Write dedup file
	fmt.Println("  Writing dedup file...")
	writeStart := time.Now()

	writer, err := dedup.NewWriter(outputPath)
	if err != nil {
		result.Err = fmt.Errorf("create dedup writer: %w", err)
		return result
	}
	defer writer.Close()

	writer.SetHeader(parser.Size(), mkvChecksum, indexer.SourceType())
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
		fmt.Printf("    Range maps encoded: %s bytes\n", formatInt(rangeMapSize))
	}

	lastProgress = time.Time{}
	if err := writer.WriteWithProgress(func(written, total int64) {
		if time.Since(lastProgress) > 500*time.Millisecond {
			pct := float64(written) / float64(total) * 100
			fmt.Printf("\r    Progress: %.1f%% (%s/%s bytes)", pct, formatInt(written), formatInt(total))
			lastProgress = time.Now()
		}
	}); err != nil {
		os.Remove(outputPath)
		result.Err = fmt.Errorf("write dedup file: %w", err)
		return result
	}
	fmt.Printf("\r    Written in %v                              \n", time.Since(writeStart))

	// Write config file
	configPath := outputPath + ".yaml"
	if err := dedup.WriteConfig(configPath, virtualName, outputPath, sourceDir); err != nil {
		fmt.Printf("    Warning: failed to write config file: %v\n", err)
	} else {
		fmt.Printf("    Config: %s\n", configPath)
	}

	// Verify reconstruction
	fmt.Println("  Verifying reconstruction...")
	verifyStart := time.Now()
	if err := verifyReconstruction(outputPath, sourceDir, mkvPath, index, verbose); err != nil {
		fmt.Printf("    WARNING: Verification failed: %v\n", err)
		fmt.Printf("    Keeping files for debugging\n")
	} else {
		fmt.Printf("    Verified in %v\n", time.Since(verifyStart))
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
func createDedup(mkvPath, sourceDir, outputPath, virtualName string, warnThreshold float64, quiet bool, nonInteractive bool) error {
	totalStart := time.Now()

	// Default output path
	if outputPath == "" {
		outputPath = mkvPath + ".mkvdup"
	}

	// Default virtual name
	if virtualName == "" {
		virtualName = filepath.Base(mkvPath)
	}

	fmt.Println("Creating dedup file...")
	fmt.Printf("  MKV:     %s\n", mkvPath)
	fmt.Printf("  Source:  %s\n", sourceDir)
	fmt.Printf("  Output:  %s\n", outputPath)
	fmt.Println()

	// Phase 1: Quick codec compatibility check (only reads MKV track headers, not full file)
	codecParser, err := mkv.NewParser(mkvPath)
	if err != nil {
		return fmt.Errorf("open MKV: %w", err)
	}
	if err := codecParser.ParseTracksOnly(); err != nil {
		codecParser.Close()
		return fmt.Errorf("parse MKV tracks: %w", err)
	}
	if err := checkCodecCompatibilityFromDir(codecParser.Tracks(), sourceDir, nonInteractive); err != nil {
		codecParser.Close()
		return err
	}
	codecParser.Close()

	// Phase 2: Index source (expensive)
	fmt.Println()
	fmt.Println("Indexing source...")
	indexer, index, err := buildSourceIndex(sourceDir)
	if err != nil {
		return err
	}
	defer index.Close()

	// Phase 3-6: Process MKV (re-parses MKV, but parsing is fast relative to indexing)
	fmt.Println()
	result := createDedupWithIndex(mkvPath, sourceDir, outputPath, virtualName, indexer, index, nonInteractive)
	if result.Err != nil {
		return result.Err
	}

	// Summary
	fmt.Println()
	fmt.Println("=== Results ===")
	fmt.Printf("Total time: %v\n", time.Since(totalStart))
	fmt.Println()

	fmt.Printf("MKV file size:      %s bytes (%.2f MB)\n", formatInt(result.MkvSize), float64(result.MkvSize)/(1024*1024))
	fmt.Printf("Matched bytes:      %s bytes (%.2f MB, %.1f%%)\n",
		formatInt(result.MatchedBytes), float64(result.MatchedBytes)/(1024*1024),
		float64(result.MatchedBytes)/float64(result.MkvSize)*100)
	fmt.Printf("Delta (unmatched):  %s bytes (%.2f MB, %.1f%%)\n",
		formatInt(result.UnmatchedBytes), float64(result.UnmatchedBytes)/(1024*1024),
		float64(result.UnmatchedBytes)/float64(result.MkvSize)*100)
	fmt.Println()

	fmt.Printf("Dedup file size:    %s bytes (%.2f MB)\n", formatInt(result.DedupSize), float64(result.DedupSize)/(1024*1024))
	fmt.Printf("Space savings:      %.1f%%\n", result.Savings)
	fmt.Println()

	fmt.Printf("Packets matched:    %s / %s (%.1f%%)\n",
		formatInt(int64(result.MatchedPackets)), formatInt(int64(result.TotalPackets)),
		float64(result.MatchedPackets)/float64(result.TotalPackets)*100)
	fmt.Printf("Index entries:      %s\n", formatInt(int64(result.IndexEntries)))

	// Warning for low savings
	if !quiet && result.Savings < warnThreshold {
		fmt.Println()
		fmt.Printf("WARNING: Space savings (%.1f%%) below %.0f%%\n", result.Savings, warnThreshold)
		fmt.Println("  This may indicate wrong source or transcoded MKV.")
	}

	return nil
}

// createBatch processes multiple MKVs from a batch manifest, indexing the source once.
func createBatch(manifestPath string, warnThreshold float64, quiet bool) error {
	totalStart := time.Now()

	manifest, err := dedup.ReadBatchManifest(manifestPath)
	if err != nil {
		return err
	}

	fmt.Printf("Batch create: %d files from %s\n\n", len(manifest.Files), manifest.SourceDir)

	// Pre-check: detect source codecs and warn about any MKVs with incompatible codecs
	// before the expensive indexing step. Parse each MKV and check codec compatibility.
	sourceCodecs, codecErr := source.DetectSourceCodecsFromDir(manifest.SourceDir)
	if codecErr != nil {
		if verbose {
			fmt.Printf("Note: could not detect source codecs: %v\n", codecErr)
		}
	} else {
		for _, f := range manifest.Files {
			codecParser, err := mkv.NewParser(f.MKV)
			if err != nil {
				return fmt.Errorf("open %s: %w", filepath.Base(f.MKV), err)
			}
			if err := codecParser.ParseTracksOnly(); err != nil {
				codecParser.Close()
				return fmt.Errorf("parse tracks %s: %w", filepath.Base(f.MKV), err)
			}
			mismatches := source.CheckCodecCompatibility(codecParser.Tracks(), sourceCodecs)
			codecParser.Close()
			if err := reportCodecMismatches(mismatches, true); err != nil {
				return err
			}
		}
		fmt.Println()
	}

	// Index source once
	fmt.Println("Indexing source directory...")
	indexer, index, err := buildSourceIndex(manifest.SourceDir)
	if err != nil {
		return err
	}
	defer index.Close()
	indexDuration := time.Since(totalStart)

	// Process each file
	results := make([]*createResult, len(manifest.Files))
	for i, f := range manifest.Files {
		fmt.Printf("\n[%d/%d] %s\n", i+1, len(manifest.Files), filepath.Base(f.MKV))
		results[i] = createDedupWithIndex(f.MKV, manifest.SourceDir, f.Output, f.Name, indexer, index, true)
		if results[i].Err != nil {
			fmt.Printf("  ERROR: %v\n", results[i].Err)
			if i < len(manifest.Files)-1 {
				fmt.Println("  Continuing with remaining files...")
			}
		}
	}

	// Print summary
	printBatchSummary(results, indexDuration, totalStart, warnThreshold, quiet)

	// Return error if any file failed
	for _, r := range results {
		if r.Err != nil {
			return fmt.Errorf("batch create completed with errors")
		}
	}
	return nil
}

// printBatchSummary prints the aggregate results of a batch create operation.
func printBatchSummary(results []*createResult, indexDuration time.Duration, totalStart time.Time, warnThreshold float64, quiet bool) {
	fmt.Println()
	fmt.Println("=== Batch Results ===")
	fmt.Printf("Total time: %v (indexing: %v)\n\n", time.Since(totalStart), indexDuration)

	succeeded := 0
	var lowSavings []string
	for _, r := range results {
		base := filepath.Base(r.MkvPath)
		if r.Err != nil {
			fmt.Printf("  FAIL  %s: %v\n", base, r.Err)
		} else {
			fmt.Printf("  OK    %s -> %s (%.1f%% savings)\n", base, filepath.Base(r.OutputPath), r.Savings)
			succeeded++
			if r.Savings < warnThreshold {
				lowSavings = append(lowSavings, fmt.Sprintf("  %s: %.1f%% savings", base, r.Savings))
			}
		}
	}
	fmt.Printf("\nSucceeded: %d/%d\n", succeeded, len(results))

	if !quiet && len(lowSavings) > 0 {
		fmt.Printf("\nWARNING: %d file(s) with space savings below %.0f%%:\n", len(lowSavings), warnThreshold)
		for _, s := range lowSavings {
			fmt.Println(s)
		}
		fmt.Println("  This may indicate wrong source or transcoded MKV.")
	}
}

// verifyReconstruction verifies that the dedup file can reconstruct the original MKV.
// Set verbose=true to enable debug output for troubleshooting.
func verifyReconstruction(dedupPath, sourceDir, originalPath string, index *source.Index, verbose bool) error {
	reader, err := dedup.NewReader(dedupPath, sourceDir)
	if err != nil {
		return fmt.Errorf("open dedup file: %w", err)
	}
	defer reader.Close()

	if err := reader.LoadSourceFiles(); err != nil {
		return fmt.Errorf("load source files: %w", err)
	}

	// Open original MKV
	original, err := os.Open(originalPath)
	if err != nil {
		return fmt.Errorf("open original: %w", err)
	}
	defer original.Close()

	// Debug: show first few bytes comparison (controlled by verbose flag)
	if verbose {
		origFirst := make([]byte, 32)
		reconFirst := make([]byte, 32)
		n, _ := original.ReadAt(origFirst, 0)
		fmt.Printf("  Debug: Original ReadAt(32, 0) returned %d bytes\n", n)
		n, _ = reader.ReadAt(reconFirst, 0)
		fmt.Printf("  Debug: Reader ReadAt(32, 0) returned %d bytes\n", n)
		fmt.Printf("  Debug: Original first 32 bytes:      %x\n", origFirst)
		fmt.Printf("  Debug: Reconstructed first 32 bytes: %x\n", reconFirst)
		original.Seek(0, 0) // Reset file position
	}

	// Compare chunk by chunk
	const chunkSize = 1024 * 1024 // 1MB
	originalBuf := make([]byte, chunkSize)
	reconstructedBuf := make([]byte, chunkSize)

	var offset int64
	for {
		n1, err1 := original.Read(originalBuf)
		n2, err2 := reader.ReadAt(reconstructedBuf[:n1], offset)

		if verbose && offset == 0 {
			fmt.Printf("  Debug: Loop first read - n1=%d, n2=%d, err1=%v, err2=%v\n", n1, n2, err1, err2)
			fmt.Printf("  Debug: originalBuf first 32:      %x\n", originalBuf[:32])
			fmt.Printf("  Debug: reconstructedBuf first 32: %x\n", reconstructedBuf[:32])
		}

		if n1 != n2 {
			return fmt.Errorf("size mismatch at offset %d: original=%d, reconstructed=%d", offset, n1, n2)
		}

		if !bytes.Equal(originalBuf[:n1], reconstructedBuf[:n2]) {
			// Find first mismatch
			for i := 0; i < n1; i++ {
				if originalBuf[i] != reconstructedBuf[i] {
					return fmt.Errorf("data mismatch at offset %d (orig: %02x, recon: %02x)",
						offset+int64(i), originalBuf[i], reconstructedBuf[i])
				}
			}
		}

		offset += int64(n1)

		if err1 == io.EOF && err2 == io.EOF {
			break
		}
		if err1 == io.EOF || err2 == io.EOF {
			return fmt.Errorf("EOF mismatch at offset %d", offset)
		}
		if err1 != nil {
			return fmt.Errorf("read original at %d: %w", offset, err1)
		}
		if err2 != nil {
			return fmt.Errorf("read reconstructed at %d: %w", offset, err2)
		}
	}

	return nil
}

// showInfo displays information about a dedup file.
func showInfo(dedupPath string) error {
	reader, err := dedup.NewReader(dedupPath, "")
	if err != nil {
		return fmt.Errorf("open dedup file: %w", err)
	}
	defer reader.Close()

	info := reader.Info()

	fmt.Printf("Dedup file: %s\n", dedupPath)
	fmt.Println()
	fmt.Printf("Format version:     %d\n", info["version"].(uint32))
	fmt.Printf("Original MKV size:  %s bytes (%.2f MB)\n",
		formatInt(info["original_size"].(int64)),
		float64(info["original_size"].(int64))/(1024*1024))
	fmt.Printf("Original checksum:  %016x\n", info["original_checksum"].(uint64))
	fmt.Println()

	sourceType := "Unknown"
	switch info["source_type"].(uint8) {
	case 0:
		sourceType = "DVD"
	case 1:
		sourceType = "Blu-ray"
	}
	fmt.Printf("Source type:        %s\n", sourceType)
	fmt.Printf("Uses ES offsets:    %v\n", info["uses_es_offsets"].(bool))
	if info["has_range_maps"].(bool) {
		fmt.Printf("Has range maps:     true\n")
	}
	fmt.Printf("Source file count:  %d\n", info["source_file_count"].(int))
	fmt.Printf("Index entry count:  %d\n", info["entry_count"].(int))
	fmt.Printf("Delta size:         %s bytes (%.2f MB)\n",
		formatInt(info["delta_size"].(int64)),
		float64(info["delta_size"].(int64))/(1024*1024))
	fmt.Println()

	// Source files
	fmt.Println("Source files:")
	for _, sf := range reader.SourceFiles() {
		fmt.Printf("  %s (%s bytes)\n", sf.RelativePath, formatInt(sf.Size))
	}

	return nil
}

// verifyDedup verifies a dedup file against the original MKV.
func verifyDedup(dedupPath, sourceDir, originalPath string) error {
	fmt.Printf("Verifying dedup file: %s\n", dedupPath)
	fmt.Printf("Source directory:     %s\n", sourceDir)
	fmt.Printf("Original MKV:         %s\n", originalPath)
	fmt.Println()

	// Open dedup file
	reader, err := dedup.NewReader(dedupPath, sourceDir)
	if err != nil {
		return fmt.Errorf("open dedup file: %w", err)
	}
	defer reader.Close()

	// Verify internal checksums
	fmt.Print("Verifying dedup file checksums...")
	if err := reader.VerifyIntegrity(); err != nil {
		fmt.Println(" FAILED")
		return fmt.Errorf("integrity check: %w", err)
	}
	fmt.Println(" OK")

	if err := reader.LoadSourceFiles(); err != nil {
		return fmt.Errorf("load source files: %w", err)
	}

	// Verify source file sizes
	fmt.Print("Verifying source files...")
	for _, sf := range reader.SourceFiles() {
		path := filepath.Join(sourceDir, sf.RelativePath)
		stat, err := os.Stat(path)
		if err != nil {
			fmt.Println(" FAILED")
			return fmt.Errorf("source file %s: %w", sf.RelativePath, err)
		}
		if stat.Size() != sf.Size {
			fmt.Println(" FAILED")
			return fmt.Errorf("source file %s size mismatch: expected %d, got %d",
				sf.RelativePath, sf.Size, stat.Size())
		}
	}
	fmt.Println(" OK")

	// Verify reconstruction matches original
	fmt.Print("Verifying reconstruction...")
	original, err := os.Open(originalPath)
	if err != nil {
		fmt.Println(" FAILED")
		return fmt.Errorf("open original: %w", err)
	}
	defer original.Close()

	const chunkSize = 4 * 1024 * 1024
	originalBuf := make([]byte, chunkSize)
	reconstructedBuf := make([]byte, chunkSize)
	var offset int64
	totalSize := reader.OriginalSize()

	for offset < totalSize {
		remaining := totalSize - offset
		readSize := int64(chunkSize)
		if readSize > remaining {
			readSize = remaining
		}

		n1, err1 := original.Read(originalBuf[:readSize])
		n2, err2 := reader.ReadAt(reconstructedBuf[:readSize], offset)

		if n1 != n2 {
			fmt.Println(" FAILED")
			return fmt.Errorf("size mismatch at offset %d", offset)
		}

		if !bytes.Equal(originalBuf[:n1], reconstructedBuf[:n2]) {
			fmt.Println(" FAILED")
			for i := 0; i < n1; i++ {
				if originalBuf[i] != reconstructedBuf[i] {
					return fmt.Errorf("data mismatch at offset %d", offset+int64(i))
				}
			}
		}

		if err1 != nil && err1 != io.EOF {
			fmt.Println(" FAILED")
			return fmt.Errorf("read original: %w", err1)
		}
		if err2 != nil && err2 != io.EOF {
			fmt.Println(" FAILED")
			return fmt.Errorf("read reconstructed: %w", err2)
		}

		offset += int64(n1)

		// Progress
		pct := float64(offset) / float64(totalSize) * 100
		fmt.Printf("\rVerifying reconstruction... %.1f%%", pct)
	}
	fmt.Println(" OK")

	fmt.Println()
	fmt.Println("Verification PASSED")
	return nil
}

// calculateFileChecksum calculates xxhash checksum of a file.
func calculateFileChecksum(path string) (uint64, error) {
	return calculateFileChecksumWithProgress(path, 0, "")
}

// calculateFileChecksumWithProgress calculates xxhash checksum of a file,
// showing inline progress when expectedSize > 0.
func calculateFileChecksumWithProgress(path string, expectedSize int64, displayName string) (uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	hasher := xxhash.New()
	showProgress := expectedSize > 0

	if !showProgress {
		if _, err := io.Copy(hasher, f); err != nil {
			return 0, err
		}
		return hasher.Sum64(), nil
	}

	buf := make([]byte, 4*1024*1024) // 4MB buffer
	var processed int64
	lastProgress := time.Time{}

	for {
		n, err := f.Read(buf)
		if n > 0 {
			if _, werr := hasher.Write(buf[:n]); werr != nil {
				return 0, werr
			}
			processed += int64(n)

			if time.Since(lastProgress) > 500*time.Millisecond {
				pct := float64(processed) / float64(expectedSize) * 100
				fmt.Printf("\r  Verifying %s... %.1f%%", displayName, pct)
				lastProgress = time.Now()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
	}

	// Clear progress line
	progressText := fmt.Sprintf("  Verifying %s... 100.0%%", displayName)
	fmt.Printf("\r%s\r", strings.Repeat(" ", len(progressText)))

	return hasher.Sum64(), nil
}

// checkDedup checks the integrity of a dedup file and its source files.
func checkDedup(dedupPath, sourceDir string, sourceChecksums bool) error {
	fmt.Printf("Checking dedup file: %s\n", dedupPath)
	fmt.Printf("Source directory:    %s\n", sourceDir)
	fmt.Println()

	// Phase 1: Open and verify dedup file integrity
	reader, err := dedup.NewReader(dedupPath, sourceDir)
	if err != nil {
		return fmt.Errorf("open dedup file: %w", err)
	}
	defer reader.Close()

	fmt.Print("Checking dedup file integrity...")
	if err := reader.VerifyIntegrity(); err != nil {
		fmt.Println(" FAILED")
		return fmt.Errorf("integrity check: %w", err)
	}
	fmt.Println(" OK")

	// Phase 2: Check source files exist with correct sizes
	sourceFiles := reader.SourceFiles()
	fmt.Printf("\nChecking source files (%d files)...\n", len(sourceFiles))

	errCount := 0
	for _, sf := range sourceFiles {
		sfPath := filepath.Join(sourceDir, sf.RelativePath)
		stat, err := os.Stat(sfPath)
		if err != nil {
			fmt.Printf("  FAILED  %s: %v\n", sf.RelativePath, err)
			errCount++
			continue
		}
		if stat.Size() != sf.Size {
			fmt.Printf("  FAILED  %s: size mismatch (expected %s, got %s)\n",
				sf.RelativePath, formatInt(sf.Size), formatInt(stat.Size()))
			errCount++
			continue
		}
		fmt.Printf("  OK      %s (%s bytes)\n", sf.RelativePath, formatInt(sf.Size))
	}

	// Phase 3: Optionally verify source file checksums
	if sourceChecksums {
		if errCount > 0 {
			fmt.Println("\nSkipping source checksum verification due to earlier errors")
		} else {
			fmt.Printf("\nVerifying source file checksums...\n")
			for _, sf := range sourceFiles {
				sfPath := filepath.Join(sourceDir, sf.RelativePath)

				checksum, err := calculateFileChecksumWithProgress(sfPath, sf.Size, sf.RelativePath)
				if err != nil {
					fmt.Printf("  FAILED  %s: %v\n", sf.RelativePath, err)
					errCount++
					continue
				}
				if checksum != sf.Checksum {
					fmt.Printf("  FAILED  %s: checksum mismatch (expected %016x, got %016x)\n",
						sf.RelativePath, sf.Checksum, checksum)
					errCount++
					continue
				}
				fmt.Printf("  OK      %s\n", sf.RelativePath)
			}
		}
	}

	// Final summary
	fmt.Println()
	if errCount > 0 {
		return fmt.Errorf("check FAILED: %d error(s) found", errCount)
	}
	fmt.Println("Check PASSED")
	return nil
}

// ProbeResult represents the result of probing a source against an MKV.
type ProbeResult struct {
	SourcePath   string
	MatchCount   int
	TotalSamples int
	MatchPercent float64
}

// probe tests if an MKV likely matches one or more source directories.
// This is a fast test (targeting <30 seconds) for quickly identifying which
// source directory an MKV came from, useful for multi-disc sets.
func probe(mkvPath string, sourceDirs []string) error {
	fmt.Printf("Probing %s against %d source(s)...\n", filepath.Base(mkvPath), len(sourceDirs))
	fmt.Println()

	// Phase 1: Parse MKV and sample packets
	parser, _, err := parseMKVWithProgress(mkvPath, "Parsing MKV...")
	if err != nil {
		return err
	}
	defer parser.Close()

	packets := parser.Packets()
	if len(packets) == 0 {
		return fmt.Errorf("no packets found in MKV")
	}

	// Build track type and codec maps
	trackTypes := make(map[int]int)
	trackNALLengthSize := make(map[int]int) // 0 = Annex B, 1/2/4 = AVCC/HVCC
	for _, t := range parser.Tracks() {
		trackTypes[int(t.Number)] = t.Type
		trackNALLengthSize[int(t.Number)] = matcher.NALLengthSizeForTrack(t.CodecID, t.CodecPrivate)
	}

	// Sample packets from different positions
	// 5 from first 10%, 10 from middle 80%, 5 from last 10%
	samples := samplePackets(packets, 20)
	fmt.Printf("  Sampled %d packets from %d total\n", len(samples), len(packets))

	// Read packet data and compute hashes using the shared sync point detection
	mkvFile, err := os.Open(mkvPath)
	if err != nil {
		return fmt.Errorf("open MKV: %w", err)
	}
	defer mkvFile.Close()

	windowSize := source.DefaultWindowSize
	var probeHashes []matcher.ProbeHash

	for _, pkt := range samples {
		// Read packet data (up to 4096 bytes like the matcher)
		readSize := pkt.Size
		if readSize > 4096 {
			readSize = 4096
		}
		if readSize < int64(windowSize) {
			continue
		}

		data := make([]byte, readSize)
		n, err := mkvFile.ReadAt(data, pkt.Offset)
		if err != nil || n < windowSize {
			continue
		}

		// Determine if this is video or audio
		trackType := trackTypes[int(pkt.TrackNum)]
		isVideo := trackType == mkv.TrackTypeVideo
		nalLenSize := trackNALLengthSize[int(pkt.TrackNum)]

		// Use shared function to extract probe hashes
		hashes := matcher.ExtractProbeHashes(data[:n], isVideo, windowSize, nalLenSize)
		if len(hashes) > 0 {
			// Only need one hash per packet for probing
			probeHashes = append(probeHashes, hashes[0])
		}
	}

	fmt.Printf("  Computed %d probe hashes\n", len(probeHashes))
	fmt.Println()

	if len(probeHashes) == 0 {
		return fmt.Errorf("no valid hashes computed from sampled packets")
	}

	// Phase 2: Test each source directory
	results := make([]ProbeResult, 0, len(sourceDirs))

	for _, sourceDir := range sourceDirs {
		fmt.Printf("Indexing source: %s...\n", sourceDir)

		indexer, err := source.NewIndexer(sourceDir, windowSize)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			results = append(results, ProbeResult{
				SourcePath:   sourceDir,
				MatchCount:   0,
				TotalSamples: len(probeHashes),
				MatchPercent: 0,
			})
			continue
		}
		indexer.SetVerbose(verbose)

		if err := indexer.Build(nil); err != nil {
			fmt.Printf("  Error building index: %v\n", err)
			results = append(results, ProbeResult{
				SourcePath:   sourceDir,
				MatchCount:   0,
				TotalSamples: len(probeHashes),
				MatchPercent: 0,
			})
			continue
		}

		index := indexer.Index()

		// Count matches, respecting video/audio stream type
		matchCount := 0
		for _, ph := range probeHashes {
			if locs, ok := index.HashToLocations[ph.Hash]; ok {
				// For ES-based indexes, check stream type matches
				if index.UsesESOffsets {
					for _, loc := range locs {
						if loc.IsVideo == ph.IsVideo {
							matchCount++
							break
						}
					}
				} else if len(locs) > 0 {
					matchCount++
				}
			}
		}

		index.Close()

		matchPercent := float64(matchCount) / float64(len(probeHashes)) * 100
		results = append(results, ProbeResult{
			SourcePath:   sourceDir,
			MatchCount:   matchCount,
			TotalSamples: len(probeHashes),
			MatchPercent: matchPercent,
		})

		fmt.Printf("  Matched %d/%d hashes (%.0f%%)\n", matchCount, len(probeHashes), matchPercent)
	}

	// Sort results by match percentage (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].MatchPercent > results[j].MatchPercent
	})

	// Print summary
	fmt.Println()
	fmt.Println("=== Results ===")
	fmt.Println()

	for _, r := range results {
		indicator := ""
		if r.MatchPercent >= 80 {
			indicator = " ← likely match"
		} else if r.MatchPercent >= 40 {
			indicator = " ← possible match"
		}
		fmt.Printf("  %s  %d/%d matches (%.0f%%)%s\n",
			r.SourcePath, r.MatchCount, r.TotalSamples, r.MatchPercent, indicator)
	}

	fmt.Println()
	fmt.Println("Interpretation:")
	fmt.Println("  80-100%: Very likely the correct source")
	fmt.Println("  40-80%:  Possible match (may be partial content)")
	fmt.Println("  <40%:    Unlikely to be the source")

	return nil
}

// samplePackets selects N packets distributed across the file:
// - 25% from first 10% of packets (early content)
// - 50% from middle 80% of packets (main content)
// - 25% from last 10% of packets (late content)
func samplePackets(packets []mkv.Packet, n int) []mkv.Packet {
	if len(packets) <= n {
		return packets
	}

	// Calculate distribution
	earlyCount := n / 4                    // 25% from first 10%
	lateCount := n / 4                     // 25% from last 10%
	midCount := n - earlyCount - lateCount // 50% from middle 80%

	// Calculate packet ranges
	earlyEnd := len(packets) / 10
	lateStart := len(packets) - len(packets)/10
	if earlyEnd < 1 {
		earlyEnd = 1
	}
	if lateStart <= earlyEnd {
		lateStart = earlyEnd + 1
	}

	samples := make([]mkv.Packet, 0, n)

	// Sample from early portion (first 10%)
	if earlyCount > 0 && earlyEnd > 0 {
		step := earlyEnd / earlyCount
		if step < 1 {
			step = 1
		}
		for i := 0; i < earlyEnd && len(samples) < earlyCount; i += step {
			samples = append(samples, packets[i])
		}
	}

	// Sample from middle portion (middle 80%)
	midStart := earlyEnd
	midEnd := lateStart
	if midCount > 0 && midEnd > midStart {
		step := (midEnd - midStart) / midCount
		if step < 1 {
			step = 1
		}
		for i := midStart; i < midEnd && len(samples) < earlyCount+midCount; i += step {
			samples = append(samples, packets[i])
		}
	}

	// Sample from late portion (last 10%)
	if lateCount > 0 && lateStart < len(packets) {
		step := (len(packets) - lateStart) / lateCount
		if step < 1 {
			step = 1
		}
		for i := lateStart; i < len(packets) && len(samples) < n; i += step {
			samples = append(samples, packets[i])
		}
	}

	return samples
}

// defaultConfigPath is the default config file location.
const defaultConfigPath = "/etc/mkvdup.conf"

// expandConfigDir expands a directory path to a list of .yaml/.yml files it contains.
func expandConfigDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read config directory %s: %w", dir, err)
	}
	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() && (filepath.Ext(entry.Name()) == ".yaml" || filepath.Ext(entry.Name()) == ".yml") {
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no YAML files (.yaml, .yml) found in %s", dir)
	}
	return paths, nil
}

// mountFuse mounts a FUSE filesystem exposing dedup files as MKV files.
func mountFuse(mountpoint string, configPaths []string, opts MountOptions) error {
	// Daemonize unless --foreground is set or we're already a daemon child
	if !opts.Foreground && !daemon.IsChild() {
		return daemon.Daemonize(opts.PidFile, opts.DaemonTimeout)
	}

	// Write PID file in foreground mode (daemon mode writes it in Daemonize)
	if opts.Foreground && opts.PidFile != "" {
		if err := daemon.WritePidFile(opts.PidFile, os.Getpid()); err != nil {
			return fmt.Errorf("write pid file: %w", err)
		}
	}

	// Clean up PID file on exit (for both foreground and daemon child modes)
	if opts.PidFile != "" && (opts.Foreground || daemon.IsChild()) {
		defer func() {
			_ = daemon.RemovePidFile(opts.PidFile)
		}()
	}

	// If no config paths provided, use default
	if len(configPaths) == 0 {
		if _, err := os.Stat(defaultConfigPath); err == nil {
			configPaths = []string{defaultConfigPath}
		} else {
			if daemon.IsChild() {
				daemon.NotifyError(fmt.Errorf("no config files specified and %s not found", defaultConfigPath))
			}
			return fmt.Errorf("no config files specified and %s not found", defaultConfigPath)
		}
	}

	// Store the config-dir path for SIGHUP re-expansion
	var configDirPath string
	if opts.ConfigDir {
		configDirPath = configPaths[0]
	}

	// If configDir is set, expand directory to list of .yaml files
	if opts.ConfigDir {
		if len(configPaths) != 1 {
			err := fmt.Errorf("--config-dir requires exactly one directory path, got %d", len(configPaths))
			if daemon.IsChild() {
				daemon.NotifyError(err)
			}
			return err
		}
		expanded, err := expandConfigDir(configPaths[0])
		if err != nil {
			if daemon.IsChild() {
				daemon.NotifyError(err)
			}
			return err
		}
		configPaths = expanded
	}

	// Set up permission store
	defaults := mkvfuse.Defaults{
		FileUID:  opts.DefaultUID,
		FileGID:  opts.DefaultGID,
		FileMode: opts.DefaultFileMode,
		DirUID:   opts.DefaultUID,
		DirGID:   opts.DefaultGID,
		DirMode:  opts.DefaultDirMode,
	}
	permPath := mkvfuse.ResolvePermissionsPath(opts.PermissionsFile)
	permStore := mkvfuse.NewPermissionStore(permPath, defaults, verbose)
	if err := permStore.Load(); err != nil {
		if daemon.IsChild() {
			daemon.NotifyError(fmt.Errorf("load permissions: %w", err))
		}
		return fmt.Errorf("load permissions: %w", err)
	}

	// Create the root filesystem
	root, err := mkvfuse.NewMKVFSWithPermissions(configPaths, verbose, permStore)
	if err != nil {
		err = fmt.Errorf("create filesystem: %w", err)
		if daemon.IsChild() {
			daemon.NotifyError(err)
		}
		return err
	}

	// Mount the filesystem
	fuseOpts := &fs.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: opts.AllowOther,
			Name:       "mkvdup",
			FsName:     "mkvdup",
			MaxWrite:   1 << 20, // 1MB max read/write; go-fuse sets max_read = MaxWrite
			// Enable kernel permission checks for standard Unix semantics.
			// This properly handles supplementary groups and matches behavior
			// of real filesystems (ext4, XFS, btrfs, etc.).
			Options: []string{"default_permissions"},
		},
	}

	server, err := fs.Mount(mountpoint, root, fuseOpts)
	if err != nil {
		err = fmt.Errorf("mount: %w", err)
		if daemon.IsChild() {
			daemon.NotifyError(err)
		}
		return err
	}

	// Wait for mount to be ready
	server.WaitMount()

	// If we're a daemon child, signal success and detach from terminal
	if daemon.IsChild() {
		if err := daemon.NotifyReady(); err != nil {
			// Parent may have timed out; log and continue since mount succeeded
			fmt.Fprintf(os.Stderr, "warning: failed to notify parent: %v\n", err)
		}
		daemon.Detach()
	} else {
		// Running in foreground mode - print info
		fmt.Printf("Mounted at %s\n", mountpoint)
		fmt.Printf("Files:\n")
		for _, configPath := range configPaths {
			config, _ := dedup.ReadConfig(configPath)
			if config != nil {
				fmt.Printf("  %s\n", config.Name)
			}
		}
		fmt.Println()
		fmt.Println("Press Ctrl+C to unmount")
	}

	// Set up logging function. In daemon mode, use syslog since
	// stderr is redirected to /dev/null after Detach().
	logFn := func(format string, args ...interface{}) {
		log.Printf(format, args...)
	}
	var syslogWriter *syslog.Writer
	if daemon.IsChild() {
		if w, err := syslog.New(syslog.LOG_INFO|syslog.LOG_DAEMON, "mkvdup"); err == nil {
			syslogWriter = w
			logFn = func(format string, args ...interface{}) {
				syslogWriter.Info(fmt.Sprintf(format, args...))
			}
		}
	}
	if syslogWriter != nil {
		// Redirect global log output to syslog so that log.Printf calls
		// from BuildDirectoryTree (during reload) go to syslog too.
		log.SetOutput(syslogWriter)
		log.SetFlags(0) // syslog adds its own timestamp
		defer syslogWriter.Close()
	}

	// Handle signals for graceful shutdown and config reload
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		for sig := range sigChan {
			switch sig {
			case syscall.SIGHUP:
				logFn("received SIGHUP, reloading config...")

				// Re-expand config-dir if applicable
				var reloadPaths []string
				if configDirPath != "" {
					expanded, err := expandConfigDir(configDirPath)
					if err != nil {
						logFn("reload failed: expand config dir: %v", err)
						continue
					}
					reloadPaths = expanded
				} else {
					reloadPaths = configPaths
				}

				// Resolve configs (expands includes, globs, virtual_files)
				configs, err := dedup.ResolveConfigs(reloadPaths)
				if err != nil {
					logFn("reload failed: resolve configs: %v", err)
					continue
				}

				// Reload the filesystem
				if err := root.Reload(configs, logFn); err != nil {
					logFn("reload failed: %v", err)
					continue
				}

				logFn("config reloaded successfully")

			case syscall.SIGINT, syscall.SIGTERM:
				if !daemon.IsChild() {
					fmt.Println("\nUnmounting...")
				}
				server.Unmount()
				return
			}
		}
	}()

	// Serve until unmounted
	server.Wait()

	if !daemon.IsChild() {
		fmt.Println("Unmounted")
	}

	return nil
}

// reloadDaemon validates config files and sends SIGHUP to the running daemon.
func reloadDaemon(pidFile string, configPaths []string, configDir bool) error {
	// Read PID from file
	pid, err := daemon.ReadPidFile(pidFile)
	if err != nil {
		return err
	}

	// Verify the process exists (on Unix, FindProcess always succeeds;
	// send signal 0 to check if process is actually running)
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return fmt.Errorf("daemon process %d is not running: %w", pid, err)
	}

	// Validate config if paths provided
	if len(configPaths) > 0 {
		resolved, err := resolveConfigPaths(configPaths, configDir)
		if err != nil {
			return fmt.Errorf("resolve config paths: %w", err)
		}

		fmt.Println("Validating configuration...")
		allEntries, _, hasErrors := validateConfigEntries(resolved)
		nameErrors, _ := checkNameConflicts(allEntries)
		if hasErrors || nameErrors {
			return fmt.Errorf("config validation failed, not sending reload signal")
		}
		fmt.Println("Configuration valid.")
		fmt.Println()
	}

	// Send SIGHUP to the daemon
	fmt.Printf("Sending SIGHUP to daemon (pid %d)...\n", pid)
	if err := process.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("send SIGHUP to process %d: %w", pid, err)
	}

	fmt.Println("Reload signal sent successfully.")
	return nil
}

// validationEntry tracks the result of validating a single resolved config entry.
type validationEntry struct {
	name       string // virtual file name
	status     string // "OK", "WARN", "ERR"
	message    string // detail message (empty for OK)
	configFile string // which input config file this came from
	dedupFile  string // resolved dedup file path
}

// resolveConfigPaths expands --config-dir and applies defaults to get the final
// list of config file paths to validate.
func resolveConfigPaths(configPaths []string, configDir bool) ([]string, error) {
	if configDir {
		if len(configPaths) != 1 {
			return nil, fmt.Errorf("--config-dir requires exactly one directory path, got %d", len(configPaths))
		}
		return expandConfigDir(configPaths[0])
	}

	if len(configPaths) == 0 {
		return nil, fmt.Errorf("no config files specified\nRun 'mkvdup validate --help' for usage")
	}

	return configPaths, nil
}

// validateConfigEntries resolves and validates each config file: YAML parsing,
// path existence checks, and dedup file header validation. Returns the
// validation entries, the successfully-parsed configs, and whether any errors
// were found.
func validateConfigEntries(configPaths []string) ([]validationEntry, []dedup.Config, bool) {
	var allEntries []validationEntry
	var allConfigs []dedup.Config
	hasErrors := false

	for _, configPath := range configPaths {
		fmt.Printf("Validating %s...\n", filepath.Base(configPath))

		configs, err := dedup.ResolveConfigs([]string{configPath})
		if err != nil {
			fmt.Printf("  ERR  %s\n", err)
			allEntries = append(allEntries, validationEntry{
				name:       filepath.Base(configPath),
				status:     "ERR",
				message:    err.Error(),
				configFile: configPath,
			})
			hasErrors = true
			continue
		}

		if len(configs) == 0 {
			fmt.Printf("  (no entries)\n")
			continue
		}

		for _, cfg := range configs {
			entry := validationEntry{
				name:       cfg.Name,
				status:     "OK",
				configFile: configPath,
				dedupFile:  cfg.DedupFile,
			}

			// Check dedup file exists
			dedupStat, err := os.Stat(cfg.DedupFile)
			if err != nil {
				entry.status = "ERR"
				entry.message = fmt.Sprintf("dedup file: %v", err)
				fmt.Printf("  ERR  %s: %s\n", cfg.Name, entry.message)
				allEntries = append(allEntries, entry)
				hasErrors = true
				continue
			}
			if dedupStat.IsDir() {
				entry.status = "ERR"
				entry.message = fmt.Sprintf("dedup file is a directory: %s", cfg.DedupFile)
				fmt.Printf("  ERR  %s: %s\n", cfg.Name, entry.message)
				allEntries = append(allEntries, entry)
				hasErrors = true
				continue
			}

			// Check source dir exists and is a directory
			sourceStat, err := os.Stat(cfg.SourceDir)
			if err != nil {
				entry.status = "ERR"
				entry.message = fmt.Sprintf("source directory: %v", err)
				fmt.Printf("  ERR  %s: %s\n", cfg.Name, entry.message)
				allEntries = append(allEntries, entry)
				hasErrors = true
				continue
			}
			if !sourceStat.IsDir() {
				entry.status = "ERR"
				entry.message = fmt.Sprintf("source path is not a directory: %s", cfg.SourceDir)
				fmt.Printf("  ERR  %s: %s\n", cfg.Name, entry.message)
				allEntries = append(allEntries, entry)
				hasErrors = true
				continue
			}

			// Validate dedup file header
			reader, err := dedup.NewReaderLazy(cfg.DedupFile, cfg.SourceDir)
			if err != nil {
				entry.status = "ERR"
				entry.message = fmt.Sprintf("invalid dedup file: %v", err)
				fmt.Printf("  ERR  %s: %s\n", cfg.Name, entry.message)
				allEntries = append(allEntries, entry)
				hasErrors = true
				continue
			}
			reader.Close()

			allEntries = append(allEntries, entry)
			allConfigs = append(allConfigs, cfg)
		}
	}

	return allEntries, allConfigs, hasErrors
}

// checkNameConflicts validates virtual file paths and detects duplicate names
// and file/directory conflicts across all entries. Updates entry statuses
// in-place and returns whether any errors or warnings were found.
func checkNameConflicts(entries []validationEntry) (hasErrors, hasWarnings bool) {
	nameToConfig := make(map[string]string)   // clean path -> config file
	dirComponents := make(map[string]string)  // paths used as directories -> config file
	fileComponents := make(map[string]string) // paths used as files -> config file

	for i, entry := range entries {
		if entry.status == "ERR" {
			continue
		}

		name := entry.name

		// Check for ".." path components
		if slices.Contains(strings.Split(name, "/"), "..") {
			entries[i].status = "ERR"
			entries[i].message = "invalid path: contains '..' component"
			fmt.Printf("  ERR  %s: %s\n", name, entries[i].message)
			hasErrors = true
			continue
		}

		// Clean and validate the path (same logic as tree.go insertFile)
		cleanPath := cleanVirtualPath(name)
		if cleanPath == "" {
			entries[i].status = "ERR"
			entries[i].message = "invalid path: empty after cleaning"
			fmt.Printf("  ERR  %s: %s\n", name, entries[i].message)
			hasErrors = true
			continue
		}

		// Check for duplicate names
		if prevConfig, exists := nameToConfig[cleanPath]; exists {
			entries[i].status = "WARN"
			entries[i].message = fmt.Sprintf("duplicate name (also in %s)", filepath.Base(prevConfig))
			fmt.Printf("  WARN %s: %s\n", name, entries[i].message)
			hasWarnings = true
			continue
		}
		nameToConfig[cleanPath] = entry.configFile

		// Check for file/directory conflicts
		parts := strings.Split(cleanPath, "/")
		conflictFound := false

		// Check if any prefix of this path is used as a file
		for j := 0; j < len(parts)-1; j++ {
			dirPath := strings.Join(parts[:j+1], "/")
			if prevConfig, exists := fileComponents[dirPath]; exists {
				entries[i].status = "WARN"
				entries[i].message = fmt.Sprintf("path component %q conflicts with file in %s", dirPath, filepath.Base(prevConfig))
				fmt.Printf("  WARN %s: %s\n", name, entries[i].message)
				hasWarnings = true
				conflictFound = true
				break
			}
			// Record as directory component
			if _, exists := dirComponents[dirPath]; !exists {
				dirComponents[dirPath] = entry.configFile
			}
		}
		if conflictFound {
			continue
		}

		// Check if this file name conflicts with a directory
		if prevConfig, exists := dirComponents[cleanPath]; exists {
			entries[i].status = "WARN"
			entries[i].message = fmt.Sprintf("conflicts with directory from %s", filepath.Base(prevConfig))
			fmt.Printf("  WARN %s: %s\n", name, entries[i].message)
			hasWarnings = true
			continue
		}

		fileComponents[cleanPath] = entry.configFile

		// Print OK for entries that passed all checks
		if entries[i].status == "OK" {
			fmt.Printf("  OK   %s\n", name)
		}
	}

	return hasErrors, hasWarnings
}

// runDeepValidation performs integrity verification on dedup files that passed
// basic validation. Returns whether any errors were found.
func runDeepValidation(entries []validationEntry, configs []dedup.Config) bool {
	fmt.Println()
	fmt.Println("Running deep validation...")
	hasErrors := false
	for _, cfg := range configs {
		// Only deep-validate entries that passed basic validation
		entryOK := false
		for _, e := range entries {
			if e.name == cfg.Name && e.dedupFile == cfg.DedupFile && e.status != "ERR" {
				entryOK = true
				break
			}
		}
		if !entryOK {
			continue
		}

		reader, err := dedup.NewReader(cfg.DedupFile, cfg.SourceDir)
		if err != nil {
			fmt.Printf("  ERR  %s: failed to open: %v\n", cfg.Name, err)
			hasErrors = true
			continue
		}
		if err := reader.VerifyIntegrity(); err != nil {
			fmt.Printf("  ERR  %s: integrity check failed: %v\n", cfg.Name, err)
			reader.Close()
			hasErrors = true
			continue
		}
		reader.Close()
		fmt.Printf("  OK   %s: checksums valid\n", cfg.Name)
	}
	return hasErrors
}

// validateConfigs validates configuration files and returns an exit code.
// Returns 0 if all configs are valid (warnings OK without strict), 1 otherwise.
func validateConfigs(configPaths []string, configDir, deep, strict bool) int {
	resolved, err := resolveConfigPaths(configPaths, configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	allEntries, allConfigs, hasErrors := validateConfigEntries(resolved)

	nameErrors, hasWarnings := checkNameConflicts(allEntries)
	hasErrors = hasErrors || nameErrors

	if deep {
		hasErrors = hasErrors || runDeepValidation(allEntries, allConfigs)
	}

	// Print summary
	var okCount, warnCount, errCount int
	for _, e := range allEntries {
		switch e.status {
		case "OK":
			okCount++
		case "WARN":
			warnCount++
		case "ERR":
			errCount++
		}
	}

	fmt.Println()
	fmt.Printf("Summary: %d entries, %d valid, %d warnings, %d errors\n",
		len(allEntries), okCount, warnCount, errCount)

	if hasErrors {
		return 1
	}
	if strict && hasWarnings {
		return 1
	}
	return 0
}

// cleanVirtualPath normalizes a virtual file path, matching the logic in
// internal/fuse/tree.go insertFile(). Returns empty string if the path is invalid.
func cleanVirtualPath(name string) string {
	// Clean the path using path.Clean (not filepath.Clean) to match
	// internal/fuse/tree.go insertFile() which uses forward-slash paths.
	cleaned := path.Clean(name)
	// Split and filter
	parts := strings.Split(cleaned, "/")
	var valid []string
	for _, p := range parts {
		if p != "" && p != "." {
			valid = append(valid, p)
		}
	}
	if len(valid) == 0 {
		return ""
	}
	return strings.Join(valid, "/")
}

// deltaClass accumulates byte count and entry count for delta classification.
type deltaClass struct {
	bytes int64
	count int
}

// deltadiag analyzes delta (unmatched) entries in a .mkvdup file by
// cross-referencing with the original MKV to classify what stream type
// each delta region belongs to (video/audio/container).
func deltadiag(dedupPath, mkvPath string) error {
	// Open dedup file
	reader, err := dedup.NewReader(dedupPath, "")
	if err != nil {
		return fmt.Errorf("open dedup file: %w", err)
	}
	defer reader.Close()

	entryCount := reader.EntryCount()
	origSize := reader.OriginalSize()
	fmt.Fprintf(os.Stderr, "Dedup file: %d entries, original size %s bytes (%.2f MB)\n",
		entryCount, formatInt(origSize), float64(origSize)/(1024*1024))

	// Parse MKV to get packet boundaries
	fmt.Fprintf(os.Stderr, "Parsing MKV file...\n")
	mkvParser, err := mkv.NewParser(mkvPath)
	if err != nil {
		return fmt.Errorf("create MKV parser: %w", err)
	}
	defer mkvParser.Close()

	if err := mkvParser.Parse(nil); err != nil {
		return fmt.Errorf("parse MKV: %w", err)
	}

	packets := mkvParser.Packets()
	tracks := mkvParser.Tracks()
	fmt.Fprintf(os.Stderr, "  %d packets, %d tracks\n", len(packets), len(tracks))

	// Build track type map and detect AVCC NAL length size
	trackTypes := make(map[int]int)
	nalLenSizes := make(map[int]int)
	for _, t := range tracks {
		trackTypes[int(t.Number)] = t.Type
		nalLenSizes[int(t.Number)] = matcher.NALLengthSizeForTrack(t.CodecID, t.CodecPrivate)
	}

	// Memory-map MKV for reading delta bytes
	mkvMmap, err := mmap.Open(mkvPath)
	if err != nil {
		return fmt.Errorf("mmap MKV: %w", err)
	}
	defer mkvMmap.Close()
	mkvData := mkvMmap.Data()

	// Sort packets by offset for binary search
	sort.Slice(packets, func(i, j int) bool {
		return packets[i].Offset < packets[j].Offset
	})

	// Classify each delta entry
	fmt.Fprintf(os.Stderr, "Classifying delta entries...\n")

	var deltaVideo, deltaAudio, deltaContainer deltaClass
	var deltaVideoByNAL [32]deltaClass
	var deltaVideoSliceSmall, deltaVideoSliceLarge deltaClass

	for i := 0; i < entryCount; i++ {
		ent, ok := reader.GetEntry(i)
		if !ok {
			continue
		}
		if ent.Source != 0 {
			continue // Skip matched entries
		}

		// Find which MKV packet contains this delta region
		pktIdx := deltadiagFindPacket(packets, ent.MkvOffset)
		if pktIdx < 0 {
			deltaContainer.bytes += ent.Length
			deltaContainer.count++
			continue
		}

		pkt := packets[pktIdx]
		ttype := trackTypes[int(pkt.TrackNum)]

		if ttype == mkv.TrackTypeVideo {
			deltaVideo.bytes += ent.Length
			deltaVideo.count++

			// Parse AVCC NALs in the delta region
			nalLenSize := nalLenSizes[int(pkt.TrackNum)]
			if nalLenSize > 0 && ent.Length >= int64(nalLenSize+1) {
				deltaStart := ent.MkvOffset
				deltaEnd := ent.MkvOffset + ent.Length
				if deltaEnd <= int64(len(mkvData)) {
					deltadiagClassifyAVCC(mkvData, pkt, nalLenSize, deltaStart, deltaEnd,
						&deltaVideoByNAL, &deltaVideoSliceSmall, &deltaVideoSliceLarge)
				}
			}
		} else if ttype == mkv.TrackTypeAudio {
			deltaAudio.bytes += ent.Length
			deltaAudio.count++
		} else {
			deltaContainer.bytes += ent.Length
			deltaContainer.count++
		}
	}

	// Print results
	totalDelta := deltaVideo.bytes + deltaAudio.bytes + deltaContainer.bytes
	if totalDelta == 0 {
		fmt.Printf("\nNo delta entries found (100%% matched).\n")
		return nil
	}

	fmt.Printf("\n=== Delta Classification ===\n")
	fmt.Printf("Total delta: %s bytes (%.2f MB)\n\n", formatInt(totalDelta), float64(totalDelta)/(1024*1024))

	fmt.Printf("Video delta:     %12s bytes (%8.2f MB) [%6d entries] (%.1f%% of delta)\n",
		formatInt(deltaVideo.bytes), float64(deltaVideo.bytes)/(1024*1024), deltaVideo.count,
		float64(deltaVideo.bytes)/float64(totalDelta)*100)
	fmt.Printf("Audio delta:     %12s bytes (%8.2f MB) [%6d entries] (%.1f%% of delta)\n",
		formatInt(deltaAudio.bytes), float64(deltaAudio.bytes)/(1024*1024), deltaAudio.count,
		float64(deltaAudio.bytes)/float64(totalDelta)*100)
	fmt.Printf("Container delta: %12s bytes (%8.2f MB) [%6d entries] (%.1f%% of delta)\n",
		formatInt(deltaContainer.bytes), float64(deltaContainer.bytes)/(1024*1024), deltaContainer.count,
		float64(deltaContainer.bytes)/float64(totalDelta)*100)

	// Video NAL type breakdown
	nalTypeNames := map[int]string{
		1: "non-IDR slice", 2: "slice A", 3: "slice B", 4: "slice C",
		5: "IDR slice", 6: "SEI", 7: "SPS", 8: "PPS", 9: "AUD", 12: "filler",
	}

	hasNALBreakdown := false
	for i := 0; i < 32; i++ {
		if deltaVideoByNAL[i].count > 0 {
			hasNALBreakdown = true
			break
		}
	}
	if hasNALBreakdown {
		fmt.Printf("\n=== Video Delta by H.264 NAL Type ===\n")
		for i := 0; i < 32; i++ {
			if deltaVideoByNAL[i].count == 0 {
				continue
			}
			name := nalTypeNames[i]
			if name == "" {
				name = fmt.Sprintf("type %d", i)
			}
			fmt.Printf("  %-14s: %10s bytes (%8.2f MB) [%6d NALs]\n",
				name, formatInt(deltaVideoByNAL[i].bytes),
				float64(deltaVideoByNAL[i].bytes)/(1024*1024),
				deltaVideoByNAL[i].count)
		}

		fmt.Printf("\n=== Video Slice Delta Size Breakdown ===\n")
		fmt.Printf("  Slice NALs < 4KB:  %10s bytes (%8.2f MB) [%6d NALs]\n",
			formatInt(deltaVideoSliceSmall.bytes), float64(deltaVideoSliceSmall.bytes)/(1024*1024),
			deltaVideoSliceSmall.count)
		fmt.Printf("  Slice NALs >= 4KB: %10s bytes (%8.2f MB) [%6d NALs]\n",
			formatInt(deltaVideoSliceLarge.bytes), float64(deltaVideoSliceLarge.bytes)/(1024*1024),
			deltaVideoSliceLarge.count)
	}

	// Summary
	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Original file:    %.2f MB\n", float64(origSize)/(1024*1024))
	fmt.Printf("Total delta:      %.2f MB (%.1f%% of original)\n",
		float64(totalDelta)/(1024*1024), float64(totalDelta)/float64(origSize)*100)
	fmt.Printf("  Video delta:    %.2f MB (%.1f%% of delta)\n",
		float64(deltaVideo.bytes)/(1024*1024), float64(deltaVideo.bytes)/float64(totalDelta)*100)
	fmt.Printf("  Audio delta:    %.2f MB (%.1f%% of delta)\n",
		float64(deltaAudio.bytes)/(1024*1024), float64(deltaAudio.bytes)/float64(totalDelta)*100)
	fmt.Printf("  Container:      %.2f MB (%.1f%% of delta)\n",
		float64(deltaContainer.bytes)/(1024*1024), float64(deltaContainer.bytes)/float64(totalDelta)*100)

	return nil
}

// deltadiagFindPacket finds the packet containing the given offset using binary search.
func deltadiagFindPacket(packets []mkv.Packet, offset int64) int {
	low, high := 0, len(packets)-1
	for low <= high {
		mid := (low + high) / 2
		pkt := packets[mid]
		if offset < pkt.Offset {
			high = mid - 1
		} else if offset >= pkt.Offset+pkt.Size {
			low = mid + 1
		} else {
			return mid
		}
	}
	return -1
}

// deltadiagClassifyAVCC parses AVCC NAL units within a packet to classify which
// NAL types fall within the delta region [deltaStart, deltaEnd).
func deltadiagClassifyAVCC(mkvData []byte, pkt mkv.Packet, nalLenSize int,
	deltaStart, deltaEnd int64,
	byNAL *[32]deltaClass, sliceSmall, sliceLarge *deltaClass) {

	pktEnd := pkt.Offset + pkt.Size
	if pktEnd > int64(len(mkvData)) {
		pktEnd = int64(len(mkvData))
	}
	pktData := mkvData[pkt.Offset:pktEnd]

	pos := 0
	for pos+nalLenSize < len(pktData) {
		var nalLen uint32
		switch nalLenSize {
		case 4:
			nalLen = binary.BigEndian.Uint32(pktData[pos:])
		case 2:
			nalLen = uint32(binary.BigEndian.Uint16(pktData[pos:]))
		case 1:
			nalLen = uint32(pktData[pos])
		}

		nalDataStart := pkt.Offset + int64(pos+nalLenSize)
		nalDataEnd := nalDataStart + int64(nalLen)
		if nalLen == 0 || nalDataEnd > pktEnd {
			break
		}

		nalFullStart := pkt.Offset + int64(pos)

		// Check overlap with delta region
		overlapStart := nalFullStart
		if overlapStart < deltaStart {
			overlapStart = deltaStart
		}
		overlapEnd := nalDataEnd
		if overlapEnd > deltaEnd {
			overlapEnd = deltaEnd
		}
		if overlapStart < overlapEnd {
			overlapBytes := overlapEnd - overlapStart

			if nalDataStart < int64(len(mkvData)) {
				nalType := mkvData[nalDataStart] & 0x1F
				byNAL[nalType].bytes += overlapBytes
				byNAL[nalType].count++

				if nalType == 1 || nalType == 5 {
					if nalLen >= 4096 {
						sliceLarge.bytes += overlapBytes
						sliceLarge.count++
					} else {
						sliceSmall.bytes += overlapBytes
						sliceSmall.count++
					}
				}
			}
		}

		pos = int(nalDataEnd - pkt.Offset)
		if pos <= 0 {
			break
		}
	}
}
