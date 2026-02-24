package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/dedup"
)

// showInfo displays information about a dedup file.
func showInfo(dedupPath string, hideUnused bool) error {
	reader, err := dedup.NewReader(dedupPath, "")
	if err != nil {
		return fmt.Errorf("open dedup file: %w", err)
	}
	defer reader.Close()

	info := reader.Info()

	fmt.Printf("Dedup file: %s\n", dedupPath)
	fmt.Println()

	creatorVersion := info["creator_version"].(string)
	if creatorVersion != "" {
		fmt.Printf("Created by:         %s\n", creatorVersion)
	} else {
		fmt.Printf("Created by:         unknown (pre-0.9.0)\n")
	}
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
	hasUsedFlags := reader.HasSourceUsedFlags()
	for _, sf := range reader.SourceFiles() {
		if hideUnused && hasUsedFlags && !sf.Used {
			continue
		}
		suffix := ""
		if hasUsedFlags && !sf.Used {
			suffix = " (unused)"
		}
		fmt.Printf("  %s (%s bytes)%s\n", sf.RelativePath, formatInt(sf.Size), suffix)
	}

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
	fmt.Printf("\nChecking source files (%d %s)...\n", len(sourceFiles), plural(len(sourceFiles), "file", "files"))

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
		return fmt.Errorf("check FAILED: %d %s found", errCount, plural(errCount, "error", "errors"))
	}
	fmt.Println("Check PASSED")
	return nil
}
