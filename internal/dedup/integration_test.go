//go:build integration

package dedup_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/dedup"
	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
	"github.com/stuckj/mkvdup/testdata"
)

// TestFullDedupCycle tests the complete dedup -> reconstruct -> verify cycle
// using the Big Buck Bunny test data.
func TestFullDedupCycle(t *testing.T) {
	paths := testdata.SkipIfNotAvailable(t)

	t.Logf("Using ISO: %s", paths.ISOFile)
	t.Logf("Using MKV: %s", paths.MKVFile)

	// Create temp directory for output
	tmpDir, err := os.MkdirTemp("", "mkvdup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dedupPath := filepath.Join(tmpDir, "test.mkvdup")

	// Phase 1: Parse MKV
	t.Log("Phase 1: Parsing MKV...")
	parser, err := mkv.NewParser(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to create MKV parser: %v", err)
	}
	defer parser.Close()

	if err := parser.Parse(nil); err != nil {
		t.Fatalf("Failed to parse MKV: %v", err)
	}
	t.Logf("  Parsed %d packets", parser.PacketCount())

	// Phase 2: Index source
	t.Log("Phase 2: Indexing source...")
	indexer, err := source.NewIndexer(paths.ISODir, source.DefaultWindowSize)
	if err != nil {
		t.Fatalf("Failed to create indexer: %v", err)
	}

	if err := indexer.Build(nil); err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}
	index := indexer.Index()
	defer index.Close()
	t.Logf("  Indexed %d hashes", len(index.HashToLocations))

	// Phase 3: Match packets
	t.Log("Phase 3: Matching packets...")
	m, err := matcher.NewMatcher(index)
	if err != nil {
		t.Fatalf("Failed to create matcher: %v", err)
	}
	defer m.Close()

	result, err := m.Match(paths.MKVFile, parser.Packets(), parser.Tracks(), nil)
	if err != nil {
		t.Fatalf("Failed to match: %v", err)
	}

	defer result.Close()
	matchRate := float64(result.MatchedBytes) / float64(result.MatchedBytes+result.DeltaSize()) * 100
	t.Logf("  Matched %d bytes (%.1f%%)", result.MatchedBytes, matchRate)
	t.Logf("  Delta: %d bytes", result.DeltaSize())

	// Phase 4: Write dedup file
	t.Log("Phase 4: Writing dedup file...")
	mkvInfo, err := os.Stat(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to stat MKV file: %v", err)
	}

	// Calculate MKV checksum
	mkvFile, err := os.Open(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to open MKV for checksum: %v", err)
	}
	h := xxhash.New()
	if _, err := io.Copy(h, mkvFile); err != nil {
		mkvFile.Close()
		t.Fatalf("Failed to checksum MKV: %v", err)
	}
	mkvChecksum := h.Sum64()
	mkvFile.Close()

	writer, err := dedup.NewWriter(dedupPath)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	writer.SetHeader(mkvInfo.Size(), mkvChecksum, indexer.SourceType())
	writer.SetSourceFiles(index.Files)

	// Convert ES offsets to raw offsets if we have ES readers (DVD sources)
	var esConverters []source.ESRangeConverter
	if index.UsesESOffsets && len(index.ESReaders) > 0 {
		esConverters = make([]source.ESRangeConverter, len(index.ESReaders))
		for i, r := range index.ESReaders {
			if converter, ok := r.(source.ESRangeConverter); ok {
				esConverters[i] = converter
			}
		}
	}

	if err := writer.SetMatchResult(result, esConverters); err != nil {
		t.Fatalf("Failed to set match result: %v", err)
	}

	if err := writer.Write(); err != nil {
		t.Fatalf("Failed to write dedup file: %v", err)
	}

	dedupInfo, err := os.Stat(dedupPath)
	if err != nil {
		t.Fatalf("Failed to stat dedup file: %v", err)
	}
	t.Logf("  Dedup file: %d bytes (%.1f%% of original)",
		dedupInfo.Size(), float64(dedupInfo.Size())/float64(mkvInfo.Size())*100)

	// Phase 5: Read back and verify
	t.Log("Phase 5: Verifying reconstruction...")
	reader, err := dedup.NewReader(dedupPath, paths.ISODir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// Set up ES reader or load source files for reconstruction
	if reader.UsesESOffsets() {
		reader.SetESReader(index.ESReaders[0])
	} else {
		if err := reader.LoadSourceFiles(); err != nil {
			t.Fatalf("Failed to load source files: %v", err)
		}
	}

	// Verify size matches
	if reader.OriginalSize() != mkvInfo.Size() {
		t.Errorf("Size mismatch: reader reports %d, original is %d",
			reader.OriginalSize(), mkvInfo.Size())
	}

	// Compare byte-by-byte (in chunks)
	origFile, err := os.Open(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to open original MKV: %v", err)
	}
	defer origFile.Close()

	const chunkSize = 1024 * 1024 // 1MB chunks
	origBuf := make([]byte, chunkSize)
	reconBuf := make([]byte, chunkSize)
	offset := int64(0)
	mismatches := 0

	for {
		nOrig, errOrig := origFile.Read(origBuf)
		if nOrig > 0 {
			nRecon, errRecon := reader.ReadAt(reconBuf[:nOrig], offset)
			if errRecon != nil && errRecon != io.EOF {
				t.Fatalf("Read error at offset %d: %v", offset, errRecon)
			}
			if nRecon != nOrig {
				t.Fatalf("Short read at offset %d: got %d, want %d", offset, nRecon, nOrig)
			}
			if !bytes.Equal(origBuf[:nOrig], reconBuf[:nOrig]) {
				mismatches++
				if mismatches <= 5 {
					// Find first mismatch position
					for i := 0; i < nOrig; i++ {
						if origBuf[i] != reconBuf[i] {
							t.Errorf("Mismatch at offset %d: orig=%02x, recon=%02x",
								offset+int64(i), origBuf[i], reconBuf[i])
							break
						}
					}
				}
			}
			offset += int64(nOrig)
		}
		if errOrig == io.EOF {
			break
		}
		if errOrig != nil {
			t.Fatalf("Read error from original: %v", errOrig)
		}
	}

	if mismatches > 0 {
		if mismatches > 5 {
			t.Logf("  ... and %d more mismatches not shown", mismatches-5)
		}
		t.Errorf("Verification failed: %d chunk mismatches", mismatches)
	} else {
		t.Log("  Verification passed: reconstructed MKV matches original")
	}

	// Summary
	t.Log("")
	t.Log("=== Summary ===")
	t.Logf("Original MKV: %.2f MB", float64(mkvInfo.Size())/(1024*1024))
	t.Logf("Dedup file:   %.2f MB", float64(dedupInfo.Size())/(1024*1024))
	t.Logf("Space saved:  %.1f%%", (1-float64(dedupInfo.Size())/float64(mkvInfo.Size()))*100)
	t.Logf("Match rate:   %.1f%%", matchRate)
}

// TestFullDedupCycle_Bluray tests the complete dedup -> reconstruct -> verify cycle
// using Blu-ray (M2TS) source data created by remuxing the Big Buck Bunny MKV
// via ffmpeg. This exercises the raw-indexing code path (indexRawFile),
// LoadSourceFiles reconstruction, and the UsesESOffsets=false branch.
func TestFullDedupCycle_Bluray(t *testing.T) {
	paths := testdata.SkipIfNotAvailable(t)

	t.Logf("Using MKV: %s", paths.MKVFile)

	// Create temp directory for output
	tmpDir, err := os.MkdirTemp("", "mkvdup-bluray-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Phase 1: Create Blu-ray source data via ffmpeg remux
	t.Log("Phase 1: Creating Blu-ray source data (ffmpeg remux)...")
	blurayDir := paths.CreateBlurayData(t, tmpDir)
	t.Logf("  Blu-ray dir: %s", blurayDir)

	dedupPath := filepath.Join(tmpDir, "bluray-test.mkvdup")

	// Phase 2: Parse MKV
	t.Log("Phase 2: Parsing MKV...")
	parser, err := mkv.NewParser(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to create MKV parser: %v", err)
	}
	defer parser.Close()

	if err := parser.Parse(nil); err != nil {
		t.Fatalf("Failed to parse MKV: %v", err)
	}
	t.Logf("  Parsed %d packets", parser.PacketCount())

	// Phase 3: Index source (should detect TypeBluray and use raw indexing)
	t.Log("Phase 3: Indexing Blu-ray source...")
	indexer, err := source.NewIndexer(blurayDir, source.DefaultWindowSize)
	if err != nil {
		t.Fatalf("Failed to create indexer: %v", err)
	}

	if indexer.SourceType() != source.TypeBluray {
		t.Fatalf("Expected TypeBluray, got %v", indexer.SourceType())
	}

	if err := indexer.Build(nil); err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}
	index := indexer.Index()
	defer index.Close()
	t.Logf("  Indexed %d hashes", len(index.HashToLocations))

	if !index.UsesESOffsets {
		t.Fatal("Expected UsesESOffsets=true for Blu-ray source")
	}

	// Phase 4: Match packets
	t.Log("Phase 4: Matching packets...")
	m, err := matcher.NewMatcher(index)
	if err != nil {
		t.Fatalf("Failed to create matcher: %v", err)
	}
	defer m.Close()

	result, err := m.Match(paths.MKVFile, parser.Packets(), parser.Tracks(), nil)
	if err != nil {
		t.Fatalf("Failed to match: %v", err)
	}

	defer result.Close()
	matchRate := float64(result.MatchedBytes) / float64(result.MatchedBytes+result.DeltaSize()) * 100
	t.Logf("  Matched %d bytes (%.1f%%)", result.MatchedBytes, matchRate)
	t.Logf("  Delta: %d bytes", result.DeltaSize())

	// Phase 5: Write dedup file
	t.Log("Phase 5: Writing dedup file...")
	mkvInfo, err := os.Stat(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to stat MKV file: %v", err)
	}

	// Calculate MKV checksum
	mkvFile, err := os.Open(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to open MKV for checksum: %v", err)
	}
	h := xxhash.New()
	if _, err := io.Copy(h, mkvFile); err != nil {
		mkvFile.Close()
		t.Fatalf("Failed to checksum MKV: %v", err)
	}
	mkvChecksum := h.Sum64()
	mkvFile.Close()

	writer, err := dedup.NewWriter(dedupPath)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	writer.SetHeader(mkvInfo.Size(), mkvChecksum, indexer.SourceType())
	writer.SetSourceFiles(index.Files)

	// Build range maps from ESReaders (V4 format)
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

	if err := writer.SetMatchResult(result, nil); err != nil {
		t.Fatalf("Failed to set match result: %v", err)
	}

	// Pre-encode range maps before writing
	if _, err := writer.EncodeRangeMaps(); err != nil {
		t.Fatalf("Failed to encode range maps: %v", err)
	}

	if err := writer.Write(); err != nil {
		t.Fatalf("Failed to write dedup file: %v", err)
	}

	dedupInfo, err := os.Stat(dedupPath)
	if err != nil {
		t.Fatalf("Failed to stat dedup file: %v", err)
	}
	t.Logf("  Dedup file: %d bytes (%.1f%% of original)",
		dedupInfo.Size(), float64(dedupInfo.Size())/float64(mkvInfo.Size())*100)

	// Phase 6: Read back and verify
	t.Log("Phase 6: Verifying reconstruction...")
	reader, err := dedup.NewReader(dedupPath, blurayDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// V4 Blu-ray path: UsesESOffsets=true (entries contain ES offsets),
	// but reader uses embedded range maps instead of external ES reader
	if !reader.UsesESOffsets() {
		t.Fatal("Reader reports UsesESOffsets=false, expected true for V4 Blu-ray")
	}
	if !reader.HasRangeMaps() {
		t.Fatal("Reader reports no range maps, expected them for V4 Blu-ray")
	}

	if err := reader.LoadSourceFiles(); err != nil {
		t.Fatalf("Failed to load source files: %v", err)
	}

	// Verify size matches
	if reader.OriginalSize() != mkvInfo.Size() {
		t.Errorf("Size mismatch: reader reports %d, original is %d",
			reader.OriginalSize(), mkvInfo.Size())
	}

	// Compare byte-by-byte (in chunks)
	origFile, err := os.Open(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to open original MKV: %v", err)
	}
	defer origFile.Close()

	const chunkSize = 1024 * 1024 // 1MB chunks
	origBuf := make([]byte, chunkSize)
	reconBuf := make([]byte, chunkSize)
	offset := int64(0)
	mismatches := 0

	for {
		nOrig, errOrig := origFile.Read(origBuf)
		if nOrig > 0 {
			nRecon, errRecon := reader.ReadAt(reconBuf[:nOrig], offset)
			if errRecon != nil && errRecon != io.EOF {
				t.Fatalf("Read error at offset %d: %v", offset, errRecon)
			}
			if nRecon != nOrig {
				t.Fatalf("Short read at offset %d: got %d, want %d", offset, nRecon, nOrig)
			}
			if !bytes.Equal(origBuf[:nOrig], reconBuf[:nOrig]) {
				mismatches++
				if mismatches <= 5 {
					// Find first mismatch position
					for i := 0; i < nOrig; i++ {
						if origBuf[i] != reconBuf[i] {
							t.Errorf("Mismatch at offset %d: orig=%02x, recon=%02x",
								offset+int64(i), origBuf[i], reconBuf[i])
							break
						}
					}
				}
			}
			offset += int64(nOrig)
		}
		if errOrig == io.EOF {
			break
		}
		if errOrig != nil {
			t.Fatalf("Read error from original: %v", errOrig)
		}
	}

	if mismatches > 0 {
		if mismatches > 5 {
			t.Logf("  ... and %d more mismatches not shown", mismatches-5)
		}
		t.Errorf("Verification failed: %d chunk mismatches", mismatches)
	} else {
		t.Log("  Verification passed: reconstructed MKV matches original")
	}

	// Summary
	t.Log("")
	t.Log("=== Blu-ray Summary ===")
	t.Logf("Original MKV: %.2f MB", float64(mkvInfo.Size())/(1024*1024))
	t.Logf("Dedup file:   %.2f MB", float64(dedupInfo.Size())/(1024*1024))
	t.Logf("Space saved:  %.1f%%", (1-float64(dedupInfo.Size())/float64(mkvInfo.Size()))*100)
	t.Logf("Match rate:   %.1f%%", matchRate)
}

// TestConcurrentReaders ensures multiple independent readers can access the same dedup file concurrently
// without errors. Each goroutine uses its own dedup.Reader instance; this test does not validate
// internal thread-safety of sharing a single Reader across goroutines.
func TestConcurrentReaders(t *testing.T) {
	paths := testdata.SkipIfNotAvailable(t)

	// First, create a dedup file (similar to TestFullDedupCycle but abbreviated)
	tmpDir, err := os.MkdirTemp("", "mkvdup-concurrent-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dedupPath := filepath.Join(tmpDir, "test.mkvdup")

	// Parse MKV
	parser, err := mkv.NewParser(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to create MKV parser: %v", err)
	}
	defer parser.Close()

	if err := parser.Parse(nil); err != nil {
		t.Fatalf("Failed to parse MKV: %v", err)
	}

	// Index source
	indexer, err := source.NewIndexer(paths.ISODir, source.DefaultWindowSize)
	if err != nil {
		t.Fatalf("Failed to create indexer: %v", err)
	}

	if err := indexer.Build(nil); err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}
	index := indexer.Index()
	defer index.Close()

	// Match packets
	m, err := matcher.NewMatcher(index)
	if err != nil {
		t.Fatalf("Failed to create matcher: %v", err)
	}
	defer m.Close()

	result, err := m.Match(paths.MKVFile, parser.Packets(), parser.Tracks(), nil)
	if err != nil {
		t.Fatalf("Failed to match: %v", err)
	}

	// Write dedup file
	mkvInfo, err := os.Stat(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to stat MKV file: %v", err)
	}

	mkvFile, err := os.Open(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to open MKV for checksum: %v", err)
	}
	h := xxhash.New()
	if _, err := io.Copy(h, mkvFile); err != nil {
		mkvFile.Close()
		t.Fatalf("Failed to checksum MKV: %v", err)
	}
	mkvChecksum := h.Sum64()
	mkvFile.Close()

	writer, err := dedup.NewWriter(dedupPath)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	writer.SetHeader(mkvInfo.Size(), mkvChecksum, indexer.SourceType())
	writer.SetSourceFiles(index.Files)

	var esConverters []source.ESRangeConverter
	if index.UsesESOffsets && len(index.ESReaders) > 0 {
		esConverters = make([]source.ESRangeConverter, len(index.ESReaders))
		for i, r := range index.ESReaders {
			if converter, ok := r.(source.ESRangeConverter); ok {
				esConverters[i] = converter
			}
		}
	}

	if err := writer.SetMatchResult(result, esConverters); err != nil {
		t.Fatalf("Failed to set match result: %v", err)
	}

	if err := writer.Write(); err != nil {
		t.Fatalf("Failed to write dedup file: %v", err)
	}

	// Now test concurrent reading
	t.Log("Testing concurrent readers...")

	const numReaders = 4
	const numReads = 10
	const readSize = 64 * 1024 // 64KB chunks

	// Create readers with per-reader cleanup to avoid leaks if creation fails mid-loop
	readers := make([]*dedup.Reader, numReaders)
	for i := 0; i < numReaders; i++ {
		reader, err := dedup.NewReader(dedupPath, paths.ISODir)
		if err != nil {
			t.Fatalf("Failed to create reader %d: %v", i, err)
		}
		t.Cleanup(func() { reader.Close() })

		if reader.UsesESOffsets() {
			reader.SetESReader(index.ESReaders[0])
		} else {
			if err := reader.LoadSourceFiles(); err != nil {
				t.Fatalf("Failed to load source files for reader %d: %v", i, err)
			}
		}
		readers[i] = reader
	}

	// Open original file for comparison
	origFile, err := os.Open(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to open original MKV: %v", err)
	}
	defer origFile.Close()

	// Run concurrent reads
	var wg sync.WaitGroup
	errCh := make(chan error, numReaders*numReads)

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerIdx int) {
			defer wg.Done()
			reader := readers[readerIdx]
			buf := make([]byte, readSize)
			origBuf := make([]byte, readSize)

			// Read from different positions
			for j := 0; j < numReads; j++ {
				// Calculate offset - spread reads across the file
				offset := int64((readerIdx*numReads + j) * readSize)
				if offset >= reader.OriginalSize() {
					offset = offset % reader.OriginalSize()
				}

				// Read from dedup reader
				n, err := reader.ReadAt(buf, offset)
				if err != nil && err != io.EOF {
					errCh <- fmt.Errorf("reader %d read %d: dedup read error at offset %d: %w",
						readerIdx, j, offset, err)
					return
				}

				// Read from original for comparison
				nOrig, err := origFile.ReadAt(origBuf[:n], offset)
				if err != nil && err != io.EOF {
					errCh <- fmt.Errorf("reader %d read %d: original read error at offset %d: %w",
						readerIdx, j, offset, err)
					return
				}

				if n != nOrig {
					errCh <- fmt.Errorf("reader %d read %d: length mismatch at offset %d: got %d, want %d",
						readerIdx, j, offset, n, nOrig)
					return
				}

				if !bytes.Equal(buf[:n], origBuf[:n]) {
					errCh <- fmt.Errorf("reader %d read %d: data mismatch at offset %d",
						readerIdx, j, offset)
					return
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errCh)

	// Collect any errors
	for err := range errCh {
		t.Errorf("Concurrent read error: %v", err)
	}

	t.Logf("Completed %d concurrent readers with %d reads each", numReaders, numReads)
}

// TestVerifyIntegrity_Integration tests the VerifyIntegrity method with real data.
func TestVerifyIntegrity_Integration(t *testing.T) {
	paths := testdata.SkipIfNotAvailable(t)

	// Create a dedup file (abbreviated from TestFullDedupCycle)
	tmpDir, err := os.MkdirTemp("", "mkvdup-integrity-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dedupPath := filepath.Join(tmpDir, "test.mkvdup")

	// Parse MKV
	parser, err := mkv.NewParser(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to create MKV parser: %v", err)
	}
	defer parser.Close()

	if err := parser.Parse(nil); err != nil {
		t.Fatalf("Failed to parse MKV: %v", err)
	}

	// Index source
	indexer, err := source.NewIndexer(paths.ISODir, source.DefaultWindowSize)
	if err != nil {
		t.Fatalf("Failed to create indexer: %v", err)
	}

	if err := indexer.Build(nil); err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}
	index := indexer.Index()
	defer index.Close()

	// Match packets
	m, err := matcher.NewMatcher(index)
	if err != nil {
		t.Fatalf("Failed to create matcher: %v", err)
	}
	defer m.Close()

	result, err := m.Match(paths.MKVFile, parser.Packets(), parser.Tracks(), nil)
	if err != nil {
		t.Fatalf("Failed to match: %v", err)
	}

	// Write dedup file
	mkvInfo, err := os.Stat(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to stat MKV file: %v", err)
	}

	mkvFile, err := os.Open(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to open MKV for checksum: %v", err)
	}
	h := xxhash.New()
	if _, err := io.Copy(h, mkvFile); err != nil {
		mkvFile.Close()
		t.Fatalf("Failed to checksum MKV: %v", err)
	}
	mkvChecksum := h.Sum64()
	mkvFile.Close()

	writer, err := dedup.NewWriter(dedupPath)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	writer.SetHeader(mkvInfo.Size(), mkvChecksum, indexer.SourceType())
	writer.SetSourceFiles(index.Files)

	var esConverters []source.ESRangeConverter
	if index.UsesESOffsets && len(index.ESReaders) > 0 {
		esConverters = make([]source.ESRangeConverter, len(index.ESReaders))
		for i, r := range index.ESReaders {
			if converter, ok := r.(source.ESRangeConverter); ok {
				esConverters[i] = converter
			}
		}
	}

	if err := writer.SetMatchResult(result, esConverters); err != nil {
		t.Fatalf("Failed to set match result: %v", err)
	}

	if err := writer.Write(); err != nil {
		t.Fatalf("Failed to write dedup file: %v", err)
	}

	// Test integrity verification
	t.Log("Testing integrity verification...")

	reader, err := dedup.NewReader(dedupPath, paths.ISODir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// VerifyIntegrity should pass on valid file
	if err := reader.VerifyIntegrity(); err != nil {
		t.Errorf("VerifyIntegrity failed on valid file: %v", err)
	}

	t.Log("Integrity verification passed")
}

// TestReaderInfo_Integration tests the Info method with real data.
func TestReaderInfo_Integration(t *testing.T) {
	paths := testdata.SkipIfNotAvailable(t)

	// Create a dedup file (abbreviated)
	tmpDir, err := os.MkdirTemp("", "mkvdup-info-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dedupPath := filepath.Join(tmpDir, "test.mkvdup")

	// Parse MKV
	parser, err := mkv.NewParser(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to create MKV parser: %v", err)
	}
	defer parser.Close()

	if err := parser.Parse(nil); err != nil {
		t.Fatalf("Failed to parse MKV: %v", err)
	}

	// Index source
	indexer, err := source.NewIndexer(paths.ISODir, source.DefaultWindowSize)
	if err != nil {
		t.Fatalf("Failed to create indexer: %v", err)
	}

	if err := indexer.Build(nil); err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}
	index := indexer.Index()
	defer index.Close()

	// Match packets
	m, err := matcher.NewMatcher(index)
	if err != nil {
		t.Fatalf("Failed to create matcher: %v", err)
	}
	defer m.Close()

	result, err := m.Match(paths.MKVFile, parser.Packets(), parser.Tracks(), nil)
	if err != nil {
		t.Fatalf("Failed to match: %v", err)
	}

	// Write dedup file
	mkvInfo, err := os.Stat(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to stat MKV file: %v", err)
	}

	mkvFile, err := os.Open(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to open MKV for checksum: %v", err)
	}
	h := xxhash.New()
	if _, err := io.Copy(h, mkvFile); err != nil {
		mkvFile.Close()
		t.Fatalf("Failed to checksum MKV: %v", err)
	}
	mkvChecksum := h.Sum64()
	mkvFile.Close()

	writer, err := dedup.NewWriter(dedupPath)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	writer.SetHeader(mkvInfo.Size(), mkvChecksum, indexer.SourceType())
	writer.SetSourceFiles(index.Files)

	var esConverters []source.ESRangeConverter
	if index.UsesESOffsets && len(index.ESReaders) > 0 {
		esConverters = make([]source.ESRangeConverter, len(index.ESReaders))
		for i, r := range index.ESReaders {
			if converter, ok := r.(source.ESRangeConverter); ok {
				esConverters[i] = converter
			}
		}
	}

	if err := writer.SetMatchResult(result, esConverters); err != nil {
		t.Fatalf("Failed to set match result: %v", err)
	}

	if err := writer.Write(); err != nil {
		t.Fatalf("Failed to write dedup file: %v", err)
	}

	// Test Info method
	reader, err := dedup.NewReader(dedupPath, paths.ISODir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	info := reader.Info()

	// Validate info fields
	if info["version"] == nil {
		t.Error("Info missing 'version' field")
	}
	if info["original_size"] == nil {
		t.Error("Info missing 'original_size' field")
	}
	if info["entry_count"] == nil {
		t.Error("Info missing 'entry_count' field")
	}
	if info["source_file_count"] == nil {
		t.Error("Info missing 'source_file_count' field")
	}

	// Validate values make sense
	originalSize := info["original_size"].(int64)
	if originalSize != mkvInfo.Size() {
		t.Errorf("Info original_size = %d, want %d", originalSize, mkvInfo.Size())
	}

	entryCount := info["entry_count"].(int)
	if entryCount <= 0 {
		t.Errorf("Info entry_count = %d, expected positive value", entryCount)
	}

	t.Logf("Info: version=%v, original_size=%d, entry_count=%d, source_files=%d",
		info["version"], originalSize, entryCount, info["source_file_count"])
}
