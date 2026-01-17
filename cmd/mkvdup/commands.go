package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/stuckj/mkvdup/internal/dedup"
	mkvfuse "github.com/stuckj/mkvdup/internal/fuse"
	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
)

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

	writer.SetHeader(parser.Size(), mkvChecksum, indexer.SourceType(), index.UsesESOffsets)
	writer.SetSourceFiles(index.Files)
	writer.SetMatchResult(result)

	if err := writer.Write(); err != nil {
		os.Remove(outputPath) // Clean up on error
		return fmt.Errorf("write dedup file: %w", err)
	}
	fmt.Printf("  Written in %v\n", time.Since(start))

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
	fmt.Printf("MKV file size:      %d bytes (%.2f MB)\n", mkvSize, float64(mkvSize)/(1024*1024))
	fmt.Printf("Matched bytes:      %d bytes (%.2f MB, %.1f%%)\n",
		result.MatchedBytes, float64(result.MatchedBytes)/(1024*1024),
		float64(result.MatchedBytes)/float64(mkvSize)*100)
	fmt.Printf("Delta (unmatched):  %d bytes (%.2f MB, %.1f%%)\n",
		result.UnmatchedBytes, float64(result.UnmatchedBytes)/(1024*1024),
		float64(result.UnmatchedBytes)/float64(mkvSize)*100)
	fmt.Println()

	// Get dedup file size
	dedupInfo, _ := os.Stat(outputPath)
	dedupSize := dedupInfo.Size()
	savings := float64(mkvSize-dedupSize) / float64(mkvSize) * 100

	fmt.Printf("Dedup file size:    %d bytes (%.2f MB)\n", dedupSize, float64(dedupSize)/(1024*1024))
	fmt.Printf("Space savings:      %.1f%%\n", savings)
	fmt.Println()

	fmt.Printf("Packets matched:    %d / %d (%.1f%%)\n",
		result.MatchedPackets, result.TotalPackets,
		float64(result.MatchedPackets)/float64(result.TotalPackets)*100)
	fmt.Printf("Index entries:      %d\n", len(result.Entries))

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
	fmt.Printf("Original MKV size:  %d bytes (%.2f MB)\n",
		info["original_size"].(int64),
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
	fmt.Printf("Delta size:         %d bytes (%.2f MB)\n",
		info["delta_size"].(int64),
		float64(info["delta_size"].(int64))/(1024*1024))
	fmt.Println()

	// Source files
	fmt.Println("Source files:")
	for _, sf := range reader.SourceFiles() {
		fmt.Printf("  %s (%d bytes)\n", sf.RelativePath, sf.Size)
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
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	hasher := xxhash.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return 0, err
	}
	return hasher.Sum64(), nil
}

// mountFuse mounts a FUSE filesystem exposing dedup files as MKV files.
func mountFuse(mountpoint string, configPaths []string) error {
	// Create the root filesystem
	root, err := mkvfuse.NewMKVFS(configPaths, verbose)
	if err != nil {
		return fmt.Errorf("create filesystem: %w", err)
	}

	// Mount the filesystem
	opts := &fs.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: false,
			Name:       "mkvdup",
			FsName:     "mkvdup",
		},
	}

	server, err := fs.Mount(mountpoint, root, opts)
	if err != nil {
		return fmt.Errorf("mount: %w", err)
	}

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

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nUnmounting...")
		server.Unmount()
	}()

	// Serve until unmounted
	server.Wait()
	fmt.Println("Unmounted")

	return nil
}
