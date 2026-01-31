package main

import (
	"bytes"
	"fmt"
	"io"
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

// createDedup creates a .mkvdup file from an MKV and source directory.
func createDedup(mkvPath, sourceDir, outputPath, virtualName string) error {
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

	// Phase 1: Parse MKV
	fmt.Println("Phase 1/5: Parsing MKV file...")
	parser, err := mkv.NewParser(mkvPath)
	if err != nil {
		return fmt.Errorf("create parser: %w", err)
	}
	defer parser.Close()

	start := time.Now()
	if err := parser.Parse(nil); err != nil {
		return fmt.Errorf("parse MKV: %w", err)
	}
	fmt.Printf("  Parsed %d packets in %v\n", parser.PacketCount(), time.Since(start))

	// Calculate MKV checksum
	fmt.Print("  Calculating MKV checksum...")
	mkvChecksum, err := calculateFileChecksum(mkvPath)
	if err != nil {
		return fmt.Errorf("calculate MKV checksum: %w", err)
	}
	fmt.Printf(" done\n")

	// Phase 2: Index source
	fmt.Println("Phase 2/5: Indexing source...")
	indexer, err := source.NewIndexer(sourceDir, source.DefaultWindowSize)
	if err != nil {
		return fmt.Errorf("create indexer: %w", err)
	}

	start = time.Now()
	lastProgress := time.Now()
	err = indexer.Build(func(processed, total int64) {
		if time.Since(lastProgress) > 500*time.Millisecond {
			pct := float64(processed) / float64(total) * 100
			fmt.Printf("\r  Progress: %.1f%%", pct)
			lastProgress = time.Now()
		}
	})
	if err != nil {
		return fmt.Errorf("build index: %w", err)
	}
	index := indexer.Index()
	defer index.Close()
	fmt.Printf("\r  Indexed %d hashes in %v                    \n", len(index.HashToLocations), time.Since(start))

	// Phase 3: Match packets
	fmt.Println("Phase 3/5: Matching packets...")
	m, err := matcher.NewMatcher(index)
	if err != nil {
		return fmt.Errorf("create matcher: %w", err)
	}
	defer m.Close()

	start = time.Now()
	lastProgress = time.Now()
	result, err := m.Match(mkvPath, parser.Packets(), parser.Tracks(), func(processed, total int) {
		if time.Since(lastProgress) > 500*time.Millisecond {
			pct := float64(processed) / float64(total) * 100
			fmt.Printf("\r  Progress: %.1f%% (%d/%d packets)", pct, processed, total)
			lastProgress = time.Now()
		}
	})
	if err != nil {
		return fmt.Errorf("match: %w", err)
	}
	fmt.Printf("\r  Matched in %v                              \n", time.Since(start))

	// Phase 4: Write dedup file
	fmt.Println("Phase 4/5: Writing dedup file...")
	start = time.Now()

	writer, err := dedup.NewWriter(outputPath)
	if err != nil {
		return fmt.Errorf("create dedup writer: %w", err)
	}
	defer writer.Close()

	writer.SetHeader(parser.Size(), mkvChecksum, indexer.SourceType())
	writer.SetSourceFiles(index.Files)

	// Convert ES offsets to raw offsets if we have ES readers (DVD sources)
	var esConverters []source.ESRangeConverter
	if index.UsesESOffsets && len(index.ESReaders) > 0 {
		// ESReaders also implement ESRangeConverter (MPEGPSParser)
		esConverters = make([]source.ESRangeConverter, len(index.ESReaders))
		for i, r := range index.ESReaders {
			if converter, ok := r.(source.ESRangeConverter); ok {
				esConverters[i] = converter
			}
		}
	}

	if err := writer.SetMatchResult(result, esConverters); err != nil {
		os.Remove(outputPath) // Clean up on error
		return fmt.Errorf("set match result: %w", err)
	}

	lastProgress = time.Time{}
	if err := writer.WriteWithProgress(func(written, total int64) {
		if time.Since(lastProgress) > 500*time.Millisecond {
			pct := float64(written) / float64(total) * 100
			fmt.Printf("\r  Progress: %.1f%% (%s/%s bytes)", pct, formatInt(written), formatInt(total))
			lastProgress = time.Now()
		}
	}); err != nil {
		os.Remove(outputPath) // Clean up on error
		return fmt.Errorf("write dedup file: %w", err)
	}
	fmt.Printf("\r  Written in %v                              \n", time.Since(start))

	// Write config file
	configPath := outputPath + ".yaml"
	if err := dedup.WriteConfig(configPath, virtualName, outputPath, sourceDir); err != nil {
		fmt.Printf("  Warning: failed to write config file: %v\n", err)
	} else {
		fmt.Printf("  Config:  %s\n", configPath)
	}

	// Phase 5: Verify
	fmt.Println("Phase 5/5: Verifying reconstruction...")
	start = time.Now()
	if err := verifyReconstruction(outputPath, sourceDir, mkvPath, index, verbose); err != nil {
		// Don't delete files so we can debug
		fmt.Printf("  WARNING: Verification failed: %v\n", err)
		fmt.Printf("  Keeping files for debugging\n")
	} else {
		fmt.Printf("  Verified in %v\n", time.Since(start))
	}

	// Summary
	fmt.Println()
	fmt.Println("=== Results ===")
	fmt.Printf("Total time: %v\n", time.Since(totalStart))
	fmt.Println()

	mkvSize := parser.Size()
	fmt.Printf("MKV file size:      %s bytes (%.2f MB)\n", formatInt(mkvSize), float64(mkvSize)/(1024*1024))
	fmt.Printf("Matched bytes:      %s bytes (%.2f MB, %.1f%%)\n",
		formatInt(result.MatchedBytes), float64(result.MatchedBytes)/(1024*1024),
		float64(result.MatchedBytes)/float64(mkvSize)*100)
	fmt.Printf("Delta (unmatched):  %s bytes (%.2f MB, %.1f%%)\n",
		formatInt(result.UnmatchedBytes), float64(result.UnmatchedBytes)/(1024*1024),
		float64(result.UnmatchedBytes)/float64(mkvSize)*100)
	fmt.Println()

	// Get dedup file size
	dedupInfo, _ := os.Stat(outputPath)
	dedupSize := dedupInfo.Size()
	savings := float64(mkvSize-dedupSize) / float64(mkvSize) * 100

	fmt.Printf("Dedup file size:    %s bytes (%.2f MB)\n", formatInt(dedupSize), float64(dedupSize)/(1024*1024))
	fmt.Printf("Space savings:      %.1f%%\n", savings)
	fmt.Println()

	fmt.Printf("Packets matched:    %s / %s (%.1f%%)\n",
		formatInt(int64(result.MatchedPackets)), formatInt(int64(result.TotalPackets)),
		float64(result.MatchedPackets)/float64(result.TotalPackets)*100)
	fmt.Printf("Index entries:      %s\n", formatInt(int64(len(result.Entries))))

	// Warning for low savings
	if savings < 75 {
		fmt.Println()
		fmt.Printf("WARNING: Space savings (%.1f%%) below 75%%\n", savings)
		fmt.Println("  This may indicate wrong source or transcoded MKV.")
	}

	return nil
}

// verifyReconstruction verifies that the dedup file can reconstruct the original MKV.
// Set verbose=true to enable debug output for troubleshooting.
func verifyReconstruction(dedupPath, sourceDir, originalPath string, index *source.Index, verbose bool) error {
	reader, err := dedup.NewReader(dedupPath, sourceDir)
	if err != nil {
		return fmt.Errorf("open dedup file: %w", err)
	}
	defer reader.Close()

	// Set ES reader if this is an ES-based source
	if reader.UsesESOffsets() && len(index.ESReaders) > 0 {
		reader.SetESReader(index.ESReaders[0])
	} else {
		// Load raw source files
		if err := reader.LoadSourceFiles(); err != nil {
			return fmt.Errorf("load source files: %w", err)
		}
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

	// For ES-based sources, we need to set up the ES reader
	if reader.UsesESOffsets() {
		// Create indexer to get ES reader
		indexer, err := source.NewIndexer(sourceDir, source.DefaultWindowSize)
		if err != nil {
			return fmt.Errorf("create indexer: %w", err)
		}
		if err := indexer.Build(nil); err != nil {
			return fmt.Errorf("build index: %w", err)
		}
		index := indexer.Index()
		defer index.Close()

		if len(index.ESReaders) > 0 {
			reader.SetESReader(index.ESReaders[0])
		}
	} else {
		// Load source files for raw access
		if err := reader.LoadSourceFiles(); err != nil {
			return fmt.Errorf("load source files: %w", err)
		}
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
	fmt.Println("Parsing MKV and sampling packets...")
	parser, err := mkv.NewParser(mkvPath)
	if err != nil {
		return fmt.Errorf("create parser: %w", err)
	}
	defer parser.Close()

	if err := parser.Parse(nil); err != nil {
		return fmt.Errorf("parse MKV: %w", err)
	}

	packets := parser.Packets()
	if len(packets) == 0 {
		return fmt.Errorf("no packets found in MKV")
	}

	// Build track type map
	trackTypes := make(map[int]int)
	for _, t := range parser.Tracks() {
		trackTypes[int(t.Number)] = t.Type
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

		// Use shared function to extract probe hashes
		hashes := matcher.ExtractProbeHashes(data[:n], isVideo, windowSize)
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

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		if !daemon.IsChild() {
			fmt.Println("\nUnmounting...")
		}
		server.Unmount()
	}()

	// Serve until unmounted
	server.Wait()

	if !daemon.IsChild() {
		fmt.Println("Unmounted")
	}

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
		if _, err := os.Stat(defaultConfigPath); err == nil {
			return []string{defaultConfigPath}, nil
		}
		return nil, fmt.Errorf("no config files specified and %s not found\nRun 'mkvdup validate --help' for usage", defaultConfigPath)
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
