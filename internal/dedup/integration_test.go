package dedup_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
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

	matchRate := float64(result.MatchedBytes) / float64(result.MatchedBytes+int64(len(result.DeltaData))) * 100
	t.Logf("  Matched %d bytes (%.1f%%)", result.MatchedBytes, matchRate)
	t.Logf("  Delta: %d bytes", len(result.DeltaData))

	// Phase 4: Write dedup file
	t.Log("Phase 4: Writing dedup file...")
	mkvInfo, _ := os.Stat(paths.MKVFile)

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

	writer.SetHeader(mkvInfo.Size(), mkvChecksum, indexer.SourceType(), index.UsesESOffsets)
	writer.SetSourceFiles(index.Files)
	writer.SetMatchResult(result)

	if err := writer.Write(); err != nil {
		t.Fatalf("Failed to write dedup file: %v", err)
	}

	dedupInfo, _ := os.Stat(dedupPath)
	t.Logf("  Dedup file: %d bytes (%.1f%% of original)",
		dedupInfo.Size(), float64(dedupInfo.Size())/float64(mkvInfo.Size())*100)

	// Phase 5: Read back and verify
	t.Log("Phase 5: Verifying reconstruction...")
	reader, err := dedup.NewReader(dedupPath, paths.ISODir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	if err := reader.LoadSourceFiles(); err != nil {
		t.Fatalf("Failed to load source files: %v", err)
	}

	// Set up ES reader for DVD source (uses ES offsets)
	if len(index.ESReaders) > 0 && index.ESReaders[0] != nil {
		reader.SetESReader(index.ESReaders[0])
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
