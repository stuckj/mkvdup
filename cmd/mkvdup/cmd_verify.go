package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/stuckj/mkvdup/internal/dedup"
	"github.com/stuckj/mkvdup/internal/source"
)

// verifyReconstruction verifies that the dedup file can reconstruct the original MKV.
// If phasePrefix is non-empty, a progress bar is shown.
func verifyReconstruction(dedupPath, sourceDir, originalPath string, index *source.Index, phasePrefix string) error {
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
	if vw := verboseWriter(); vw != nil {
		origFirst := make([]byte, 32)
		reconFirst := make([]byte, 32)
		n, _ := original.ReadAt(origFirst, 0)
		fmt.Fprintf(vw, "  Debug: Original ReadAt(32, 0) returned %d bytes\n", n)
		n, _ = reader.ReadAt(reconFirst, 0)
		fmt.Fprintf(vw, "  Debug: Reader ReadAt(32, 0) returned %d bytes\n", n)
		fmt.Fprintf(vw, "  Debug: Original first 32 bytes:      %x\n", origFirst)
		fmt.Fprintf(vw, "  Debug: Reconstructed first 32 bytes: %x\n", reconFirst)
		original.Seek(0, 0) // Reset file position
	}

	totalSize := reader.OriginalSize()
	var bar *progressBar
	if phasePrefix != "" {
		bar = newProgressBar(phasePrefix, totalSize, "bytes")
		defer bar.Cancel() // clean up if we return early on error
	}

	// Compare chunk by chunk
	const chunkSize = 1024 * 1024 // 1MB
	originalBuf := make([]byte, chunkSize)
	reconstructedBuf := make([]byte, chunkSize)

	var offset int64
	for {
		n1, err1 := original.Read(originalBuf)
		if n1 == 0 && err1 == io.EOF {
			break
		}
		n2, err2 := reader.ReadAt(reconstructedBuf[:n1], offset)

		if vw := verboseWriter(); vw != nil && offset == 0 {
			fmt.Fprintf(vw, "  Debug: Loop first read - n1=%d, n2=%d, err1=%v, err2=%v\n", n1, n2, err1, err2)
			fmt.Fprintf(vw, "  Debug: originalBuf first 32:      %x\n", originalBuf[:32])
			fmt.Fprintf(vw, "  Debug: reconstructedBuf first 32: %x\n", reconstructedBuf[:32])
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
		if bar != nil {
			bar.Update(offset)
		}

		if err1 != nil && err1 != io.EOF {
			return fmt.Errorf("read original at %d: %w", offset, err1)
		}
		if err2 != nil && err2 != io.EOF {
			return fmt.Errorf("read reconstructed at %d: %w", offset, err2)
		}
	}

	if bar != nil {
		bar.Finish()
	}
	return nil
}

// openDedupReader opens a dedup file with its source directory, verifies
// integrity, loads source files, and checks source file sizes. This is the
// shared preamble for verify, extract, and similar commands.
func openDedupReader(dedupPath, sourceDir string) (*dedup.Reader, error) {
	reader, err := dedup.NewReader(dedupPath, sourceDir)
	if err != nil {
		return nil, fmt.Errorf("open dedup file: %w", err)
	}

	printInfo("Verifying dedup file checksums...")
	if err := reader.VerifyIntegrity(); err != nil {
		printInfoln(" FAILED")
		reader.Close()
		return nil, fmt.Errorf("integrity check: %w", err)
	}
	printInfoln(" OK")

	if err := reader.LoadSourceFiles(); err != nil {
		reader.Close()
		return nil, fmt.Errorf("load source files: %w", err)
	}

	printInfo("Verifying source files...")
	for _, sf := range reader.SourceFiles() {
		path := filepath.Join(sourceDir, sf.RelativePath)
		stat, err := os.Stat(path)
		if err != nil {
			printInfoln(" FAILED")
			reader.Close()
			return nil, fmt.Errorf("source file %s: %w", sf.RelativePath, err)
		}
		if stat.Size() != sf.Size {
			printInfoln(" FAILED")
			reader.Close()
			return nil, fmt.Errorf("source file %s size mismatch: expected %d, got %d",
				sf.RelativePath, sf.Size, stat.Size())
		}
	}
	printInfoln(" OK")

	return reader, nil
}

// verifyDedup verifies a dedup file against the original MKV.
func verifyDedup(dedupPath, sourceDir, originalPath string) error {
	printInfo("Verifying dedup file: %s\n", dedupPath)
	printInfo("Source directory:     %s\n", sourceDir)
	printInfo("Original MKV:         %s\n", originalPath)
	printInfoln()

	reader, err := openDedupReader(dedupPath, sourceDir)
	if err != nil {
		return err
	}
	defer reader.Close()

	// Verify reconstruction matches original
	original, err := os.Open(originalPath)
	if err != nil {
		return fmt.Errorf("open original: %w", err)
	}
	defer original.Close()

	totalSize := reader.OriginalSize()
	bar := newProgressBar("Verifying reconstruction...", totalSize, "bytes")
	defer bar.Cancel() // clean up if we return early on error

	const chunkSize = 4 * 1024 * 1024
	originalBuf := make([]byte, chunkSize)
	reconstructedBuf := make([]byte, chunkSize)
	var offset int64

	for offset < totalSize {
		remaining := totalSize - offset
		readSize := int64(chunkSize)
		if readSize > remaining {
			readSize = remaining
		}

		n1, err1 := original.Read(originalBuf[:readSize])
		n2, err2 := reader.ReadAt(reconstructedBuf[:readSize], offset)

		if n1 != n2 {
			return fmt.Errorf("size mismatch at offset %d", offset)
		}

		if !bytes.Equal(originalBuf[:n1], reconstructedBuf[:n2]) {
			for i := 0; i < n1; i++ {
				if originalBuf[i] != reconstructedBuf[i] {
					return fmt.Errorf("data mismatch at offset %d", offset+int64(i))
				}
			}
		}

		if err1 != nil && err1 != io.EOF {
			return fmt.Errorf("read original: %w", err1)
		}
		if err2 != nil && err2 != io.EOF {
			return fmt.Errorf("read reconstructed: %w", err2)
		}

		offset += int64(n1)
		bar.Update(offset)
	}
	bar.Finish()

	printInfoln()
	printInfoln("Verification PASSED")
	return nil
}

// extractDedup rebuilds the original MKV from a dedup file and source.
func extractDedup(dedupPath, sourceDir, outputPath string) (retErr error) {
	printInfo("Dedup file:        %s\n", dedupPath)
	printInfo("Source directory:  %s\n", sourceDir)
	printInfo("Output MKV:        %s\n", outputPath)
	printInfoln()

	reader, err := openDedupReader(dedupPath, sourceDir)
	if err != nil {
		return err
	}
	defer reader.Close()

	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer func() {
		// Only close if not already closed by the success path below.
		// On error, clean up the partial output file.
		if retErr != nil {
			out.Close()
			os.Remove(outputPath)
		}
	}()

	totalSize := reader.OriginalSize()
	bar := newProgressBar("Extracting...", totalSize, "bytes")
	defer bar.Cancel() // clean up if we return early on error

	const chunkSize = 4 * 1024 * 1024
	buf := make([]byte, chunkSize)
	var offset int64

	for offset < totalSize {
		remaining := totalSize - offset
		readSize := int64(chunkSize)
		if readSize > remaining {
			readSize = remaining
		}

		n, err := reader.ReadAt(buf[:readSize], offset)
		if err != nil && err != io.EOF {
			return fmt.Errorf("read at offset %d: %w", offset, err)
		}

		if n == 0 {
			return fmt.Errorf("unexpected EOF at offset %d (expected %d bytes)", offset, totalSize)
		}

		if _, err := out.Write(buf[:n]); err != nil {
			return fmt.Errorf("write at offset %d: %w", offset, err)
		}

		offset += int64(n)
		bar.Update(offset)
	}
	bar.Finish()

	if err := out.Close(); err != nil {
		return fmt.Errorf("close output: %w", err)
	}

	printInfo("\nExtracted %s bytes to %s\n", formatInt(totalSize), outputPath)
	return nil
}
