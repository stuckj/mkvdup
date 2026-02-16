package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
)

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
