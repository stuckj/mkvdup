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
	MKVPath      string
	SourcePath   string
	MatchCount   int
	TotalSamples int
	MatchPercent float64
}

// mkvProbeData holds the pre-computed probe hashes for a single MKV file.
type mkvProbeData struct {
	Path        string
	HashCount   int
	ProbeHashes []matcher.ProbeHash
	Error       string // non-empty if MKV could not be parsed
}

// probe tests if one or more MKV files match one or more source directories.
// When multiple MKVs are provided, each source is indexed only once and all
// MKV hash sets are checked against it, making multi-MKV probing much faster.
func probe(mkvPaths []string, sourceDirs []string) error {
	fmt.Printf("Probing %d MKV(s) against %d source(s)...\n", len(mkvPaths), len(sourceDirs))
	fmt.Println()

	windowSize := source.DefaultWindowSize

	// Phase 1: Parse all MKVs and compute probe hashes
	mkvData := make([]mkvProbeData, 0, len(mkvPaths))
	for i, mkvPath := range mkvPaths {
		if len(mkvPaths) > 1 {
			fmt.Printf("[%d/%d] Parsing %s...\n", i+1, len(mkvPaths), filepath.Base(mkvPath))
		} else {
			fmt.Printf("Parsing %s...\n", filepath.Base(mkvPath))
		}

		hashes, err := computeProbeHashes(mkvPath, windowSize)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			mkvData = append(mkvData, mkvProbeData{
				Path:  mkvPath,
				Error: err.Error(),
			})
			continue
		}

		fmt.Printf("  Computed %d probe hashes\n", len(hashes))
		mkvData = append(mkvData, mkvProbeData{
			Path:        mkvPath,
			HashCount:   len(hashes),
			ProbeHashes: hashes,
		})
	}
	fmt.Println()

	// Phase 2: For each source, index once and check all MKV hash sets
	// results[mkvIdx] = []ProbeResult for that MKV
	results := make([][]ProbeResult, len(mkvData))
	for i := range results {
		results[i] = make([]ProbeResult, 0, len(sourceDirs))
	}

	for _, sourceDir := range sourceDirs {
		fmt.Printf("Indexing source: %s...\n", sourceDir)

		indexer, err := source.NewIndexer(sourceDir, windowSize)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			for i, md := range mkvData {
				if md.Error != "" {
					continue
				}
				results[i] = append(results[i], ProbeResult{
					MKVPath:      md.Path,
					SourcePath:   sourceDir,
					TotalSamples: md.HashCount,
				})
			}
			continue
		}
		indexer.SetVerboseWriter(verboseWriter())

		if err := indexer.Build(nil); err != nil {
			fmt.Printf("  Error building index: %v\n", err)
			for i, md := range mkvData {
				if md.Error != "" {
					continue
				}
				results[i] = append(results[i], ProbeResult{
					MKVPath:      md.Path,
					SourcePath:   sourceDir,
					TotalSamples: md.HashCount,
				})
			}
			continue
		}

		index := indexer.Index()

		// Check each MKV's hashes against this source
		for i, md := range mkvData {
			if md.Error != "" {
				continue
			}

			matchCount := 0
			for _, ph := range md.ProbeHashes {
				if locs, ok := index.HashToLocations[ph.Hash]; ok {
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

			matchPercent := float64(matchCount) / float64(md.HashCount) * 100
			results[i] = append(results[i], ProbeResult{
				MKVPath:      md.Path,
				SourcePath:   sourceDir,
				MatchCount:   matchCount,
				TotalSamples: md.HashCount,
				MatchPercent: matchPercent,
			})

			if len(mkvPaths) > 1 {
				fmt.Printf("  %s: %d/%d (%.0f%%)\n",
					filepath.Base(md.Path), matchCount, md.HashCount, matchPercent)
			} else {
				fmt.Printf("  Matched %d/%d hashes (%.0f%%)\n",
					matchCount, md.HashCount, matchPercent)
			}
		}

		index.Close()
	}

	// Phase 3: Print results
	fmt.Println()
	fmt.Println("=== Results ===")

	for i, md := range mkvData {
		if md.Error != "" {
			fmt.Printf("\n  %s: ERROR: %s\n", filepath.Base(md.Path), md.Error)
			continue
		}

		if len(mkvPaths) > 1 {
			fmt.Printf("\n  %s:\n", filepath.Base(md.Path))
		} else {
			fmt.Println()
		}

		// Sort this MKV's results by match percentage
		sort.Slice(results[i], func(a, b int) bool {
			return results[i][a].MatchPercent > results[i][b].MatchPercent
		})

		for _, r := range results[i] {
			indicator := ""
			if r.MatchPercent >= 80 {
				indicator = " ← likely match"
			} else if r.MatchPercent >= 40 {
				indicator = " ← possible match"
			}
			if len(mkvPaths) > 1 {
				fmt.Printf("    %s  %d/%d matches (%.0f%%)%s\n",
					r.SourcePath, r.MatchCount, r.TotalSamples, r.MatchPercent, indicator)
			} else {
				fmt.Printf("  %s  %d/%d matches (%.0f%%)%s\n",
					r.SourcePath, r.MatchCount, r.TotalSamples, r.MatchPercent, indicator)
			}
		}
	}

	fmt.Println()
	fmt.Println("Interpretation:")
	fmt.Println("  80-100%: Very likely the correct source")
	fmt.Println("  40-80%:  Possible match (may be partial content)")
	fmt.Println("  <40%:    Unlikely to be the source")

	return nil
}

// computeProbeHashes parses an MKV and returns its probe hashes.
func computeProbeHashes(mkvPath string, windowSize int) ([]matcher.ProbeHash, error) {
	parser, _, err := parseMKVWithProgress(mkvPath, "")
	if err != nil {
		return nil, err
	}
	defer parser.Close()

	packets := parser.Packets()
	if len(packets) == 0 {
		return nil, fmt.Errorf("no packets found in MKV")
	}

	trackTypes := make(map[int]int)
	trackNALLengthSize := make(map[int]int)
	for _, t := range parser.Tracks() {
		trackTypes[int(t.Number)] = t.Type
		trackNALLengthSize[int(t.Number)] = matcher.NALLengthSizeForTrack(t.CodecID, t.CodecPrivate)
	}

	samples := samplePackets(packets, 20)

	mkvFile, err := os.Open(mkvPath)
	if err != nil {
		return nil, fmt.Errorf("open MKV: %w", err)
	}
	defer mkvFile.Close()

	var probeHashes []matcher.ProbeHash
	for _, pkt := range samples {
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

		trackType := trackTypes[int(pkt.TrackNum)]
		isVideo := trackType == mkv.TrackTypeVideo
		nalLenSize := trackNALLengthSize[int(pkt.TrackNum)]

		hashes := matcher.ExtractProbeHashes(data[:n], isVideo, windowSize, nalLenSize)
		if len(hashes) > 0 {
			probeHashes = append(probeHashes, hashes[0])
		}
	}

	if len(probeHashes) == 0 {
		return nil, fmt.Errorf("no valid hashes computed from sampled packets")
	}

	return probeHashes, nil
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
