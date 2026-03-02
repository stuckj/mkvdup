// matchdiag is a diagnostic tool for testing Phase 1/Phase 2 matching behavior.
// It builds a source index, parses an MKV, and runs matching while printing
// rolled-up diagnostic stats every 10 seconds. Designed to be killed after
// 10-15 minutes to see if the algorithm changes are working.
package main

import (
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: matchdiag <mkv-file> <source-dir>\n")
		os.Exit(1)
	}
	mkvPath := os.Args[1]
	sourceDir := os.Args[2]

	// Phase 1: Build source index
	fmt.Println("=== Building source index ===")
	indexStart := time.Now()
	indexer, err := source.NewIndexer(sourceDir, source.DefaultWindowSize)
	if err != nil {
		log.Fatalf("create indexer: %v", err)
	}
	indexer.SetVerboseWriter(os.Stdout)

	err = indexer.Build(func(processed, total int64) {
		// Print progress every 10%
		if total > 0 && processed%(total/10+1) == 0 {
			pct := float64(processed) / float64(total) * 100
			fmt.Printf("  Indexing: %.0f%% (%d / %d bytes)\n", pct, processed, total)
		}
	})
	if err != nil {
		log.Fatalf("build index: %v", err)
	}
	index := indexer.Index()
	fmt.Printf("=== Index built in %v: %d hashes ===\n\n", time.Since(indexStart).Round(time.Second), len(index.HashToLocations))
	defer index.Close()

	// Phase 2: Parse MKV
	fmt.Println("=== Parsing MKV ===")
	parser, err := mkv.NewParser(mkvPath)
	if err != nil {
		log.Fatalf("create parser: %v", err)
	}
	defer parser.Close()

	if err := parser.Parse(nil); err != nil {
		log.Fatalf("parse MKV: %v", err)
	}
	packets := parser.Packets()
	tracks := parser.Tracks()
	fmt.Printf("  %d packets, %d tracks\n", len(packets), len(tracks))

	// Print track info
	for _, t := range tracks {
		typeName := "unknown"
		switch t.Type {
		case mkv.TrackTypeVideo:
			typeName = "video"
		case mkv.TrackTypeAudio:
			typeName = "audio"
		case mkv.TrackTypeSubtitle:
			typeName = "subtitle"
		}
		fmt.Printf("  Track %d: %s (%s)\n", t.Number, typeName, t.CodecID)
	}
	fmt.Println()

	// Phase 3: Match with periodic diagnostics
	fmt.Println("=== Starting matching (Ctrl+C to stop) ===")
	m, err := matcher.NewMatcher(index)
	if err != nil {
		log.Fatalf("create matcher: %v", err)
	}
	defer m.Close()
	m.SetVerboseWriter(os.Stdout)

	// Track progress
	var processedPackets atomic.Int64
	matchStart := time.Now()

	// Diagnostic printer goroutine — prints stats every 10 seconds
	var lastStats matcher.DiagStats
	var lastPackets int64
	var lastTime time.Time = matchStart
	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				now := time.Now()
				stats := m.DiagSnapshot()
				pkts := processedPackets.Load()
				elapsed := now.Sub(matchStart).Round(time.Second)

				// Compute deltas since last report
				dPkts := pkts - lastPackets
				dSync := stats.TotalSyncPoints - lastStats.TotalSyncPoints
				dP1Skip := stats.Phase1Skips - lastStats.Phase1Skips
				dP2Fall := stats.Phase2Fallbacks - lastStats.Phase2Fallbacks
				dP2Locs := stats.Phase2Locations - lastStats.Phase2Locations
				dP2Early := stats.Phase2EarlyExits - lastStats.Phase2EarlyExits
				dP2Cap := stats.Phase2Capped - lastStats.Phase2Capped
				dt := now.Sub(lastTime).Seconds()

				pctDone := float64(pkts) / float64(len(packets)) * 100
				pktsPerSec := float64(dPkts) / dt

				// Estimate remaining time
				eta := "?"
				if pktsPerSec > 0 {
					remaining := float64(len(packets)-int(pkts)) / pktsPerSec
					eta = (time.Duration(remaining) * time.Second).Round(time.Second).String()
				}

				avgP2Locs := float64(0)
				if dP2Fall > 0 {
					avgP2Locs = float64(dP2Locs) / float64(dP2Fall)
				}

				fmt.Printf("\n--- %v elapsed | %.1f%% (%d/%d pkts) | %.0f pkts/s | ETA: %s ---\n",
					elapsed, pctDone, pkts, len(packets), pktsPerSec, eta)
				fmt.Printf("  CUMULATIVE: syncPts=%d  p1Skip=%d  p2Fall=%d  p2Locs=%d  p2Early=%d  p2Cap=%d\n",
					stats.TotalSyncPoints, stats.Phase1Skips,
					stats.Phase2Fallbacks, stats.Phase2Locations, stats.Phase2EarlyExits, stats.Phase2Capped)
				fmt.Printf("  LAST 10s:   syncPts=%d  p1Skip=%d  p2Fall=%d  p2Locs=%d (avg %.1f/fall)  p2Early=%d  p2Cap=%d\n",
					dSync, dP1Skip, dP2Fall, dP2Locs, avgP2Locs, dP2Early, dP2Cap)

				// Highlight the key ratio
				if dSync > 0 {
					p2Pct := float64(dP2Fall) / float64(dSync) * 100
					fmt.Printf("  Phase 2 rate: %.1f%% of sync points trigger Phase 2\n", p2Pct)
				}

				lastStats = stats
				lastPackets = pkts
				lastTime = now
			}
		}
	}()

	// Run matching
	_, err = m.Match(mkvPath, packets, tracks, func(processed, total int) {
		processedPackets.Store(int64(processed))
	})
	close(done)

	if err != nil {
		log.Fatalf("match error: %v", err)
	}

	// Final stats
	elapsed := time.Since(matchStart).Round(time.Second)
	stats := m.DiagSnapshot()
	fmt.Printf("\n=== FINAL RESULTS (%v) ===\n", elapsed)
	fmt.Printf("Total sync points:     %d\n", stats.TotalSyncPoints)
	fmt.Printf("Phase 1 skips:         %d\n", stats.Phase1Skips)
	fmt.Printf("Phase 2 fallbacks:     %d\n", stats.Phase2Fallbacks)
	fmt.Printf("Phase 2 locations:     %d\n", stats.Phase2Locations)
	fmt.Printf("Phase 2 early exits:   %d\n", stats.Phase2EarlyExits)
	fmt.Printf("Phase 2 capped:        %d\n", stats.Phase2Capped)
	if stats.Phase2Fallbacks > 0 {
		fmt.Printf("Avg locations/fallback: %.1f\n", float64(stats.Phase2Locations)/float64(stats.Phase2Fallbacks))
	}
	if stats.TotalSyncPoints > 0 {
		fmt.Printf("Phase 2 rate:          %.1f%%\n", float64(stats.Phase2Fallbacks)/float64(stats.TotalSyncPoints)*100)
	}
}
