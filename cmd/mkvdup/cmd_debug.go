package main

import (
	"fmt"
	"time"

	"github.com/stuckj/mkvdup/internal/dedup"
	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
)

func parseMKV(path string) error {
	fmt.Printf("Parsing MKV file: %s\n", path)

	parser, err := mkv.NewParser(path)
	if err != nil {
		return fmt.Errorf("create parser: %w", err)
	}
	defer parser.Close()

	fmt.Printf("File size: %s bytes (%.2f GB)\n", formatInt(parser.Size()), float64(parser.Size())/(1024*1024*1024))

	start := time.Now()
	lastProgress := time.Now()

	err = parser.Parse(func(processed, total int64) {
		if time.Since(lastProgress) > 500*time.Millisecond {
			pct := float64(processed) / float64(total) * 100
			fmt.Printf("\rProgress: %.1f%% (%s / %s bytes)", pct, formatInt(processed), formatInt(total))
			lastProgress = time.Now()
		}
	})
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	elapsed := time.Since(start)
	fmt.Printf("\rProgress: 100.0%% - Complete                    \n")
	fmt.Printf("Parse time: %v\n", elapsed)
	fmt.Println()

	fmt.Printf("Tracks: %d\n", len(parser.Tracks()))
	for _, t := range parser.Tracks() {
		typeStr := "unknown"
		switch t.Type {
		case mkv.TrackTypeVideo:
			typeStr = "video"
		case mkv.TrackTypeAudio:
			typeStr = "audio"
		case mkv.TrackTypeSubtitle:
			typeStr = "subtitle"
		}
		extra := ""
		if t.Type == mkv.TrackTypeVideo {
			nalSize := matcher.NALLengthSizeForTrack(t.CodecID, t.CodecPrivate)
			if nalSize > 0 {
				extra = fmt.Sprintf(", NAL length: %d bytes (AVCC/HVCC)", nalSize)
			} else {
				extra = ", Annex B"
			}
		}
		fmt.Printf("  Track %d: %s (codec: %s%s)\n", t.Number, typeStr, t.CodecID, extra)
	}
	fmt.Println()

	fmt.Printf("Total packets: %d\n", parser.PacketCount())
	fmt.Printf("  Video packets: %d\n", parser.VideoPacketCount())
	fmt.Printf("  Audio packets: %d\n", parser.AudioPacketCount())

	// Show some sample packets
	packets := parser.Packets()
	if len(packets) > 0 {
		fmt.Println()
		fmt.Println("Sample packets (first 5):")
		for i := 0; i < 5 && i < len(packets); i++ {
			p := packets[i]
			fmt.Printf("  Packet %d: offset=%d, size=%d, track=%d, keyframe=%v\n",
				i, p.Offset, p.Size, p.TrackNum, p.Keyframe)
		}
	}

	return nil
}

func indexSource(dir string) error {
	fmt.Printf("Indexing source directory: %s\n", dir)

	indexer, err := source.NewIndexer(dir, source.DefaultWindowSize)
	if err != nil {
		return fmt.Errorf("create indexer: %w", err)
	}

	fmt.Printf("Source type: %s\n", indexer.SourceType())

	start := time.Now()
	lastProgress := time.Now()

	err = indexer.Build(func(processed, total int64) {
		if time.Since(lastProgress) > 500*time.Millisecond {
			pct := float64(processed) / float64(total) * 100
			fmt.Printf("\rProgress: %.1f%% (%s / %s bytes)", pct, formatInt(processed), formatInt(total))
			lastProgress = time.Now()
		}
	})
	if err != nil {
		return fmt.Errorf("build index: %w", err)
	}

	elapsed := time.Since(start)
	fmt.Printf("\rProgress: 100.0%% - Complete                    \n")
	fmt.Printf("Index time: %v\n", elapsed)
	fmt.Println()

	index := indexer.Index()
	defer index.Close()
	fmt.Printf("Source files: %d\n", len(index.Files))
	for _, f := range index.Files {
		fmt.Printf("  %s: %s bytes\n", f.RelativePath, formatInt(f.Size))
	}
	fmt.Println()

	fmt.Printf("Unique hashes: %d\n", len(index.HashToLocations))
	if index.UsesESOffsets {
		containerType := "MPEG-PS"
		if indexer.SourceType() == source.TypeBluray {
			containerType = "MPEG-TS"
		}
		fmt.Printf("Index type: ES-aware (%s)\n", containerType)
	}

	// Count total locations
	totalLocations := 0
	for _, locs := range index.HashToLocations {
		totalLocations += len(locs)
	}
	fmt.Printf("Total indexed locations: %d\n", totalLocations)

	return nil
}

func matchMKV(mkvPath, sourceDir string) error {
	totalStart := time.Now()

	// Phase 1: Parse MKV
	parser, _, err := parseMKVWithProgress(mkvPath, "Phase 1/3: Parsing MKV file...")
	if err != nil {
		return err
	}
	defer parser.Close()

	// Phase 2: Index source
	_, index, err := buildSourceIndex(sourceDir, "Phase 2/3: Indexing source...")
	if err != nil {
		return err
	}
	defer index.Close()

	// Phase 3: Match packets
	fmt.Println("Phase 3/3: Matching packets...")
	m, err := matcher.NewMatcher(index)
	if err != nil {
		return fmt.Errorf("create matcher: %w", err)
	}
	defer m.Close()

	start := time.Now()
	lastProgress := time.Now()
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

	fmt.Printf("Packets matched:    %d / %d (%.1f%%)\n",
		result.MatchedPackets, result.TotalPackets,
		float64(result.MatchedPackets)/float64(result.TotalPackets)*100)
	fmt.Printf("Index entries:      %d\n", len(result.Entries))
	fmt.Println()

	// Storage savings (using actual format constants)
	indexSize := int64(len(result.Entries) * dedup.EntrySize)
	headerSize := int64(dedup.HeaderSize)
	footerSize := int64(dedup.FooterSize)
	totalDedupSize := headerSize + indexSize + int64(len(result.DeltaData)) + footerSize

	// For Blu-ray sources, V4 format includes range map section (estimate)
	rangeMapNote := ""
	if index.UsesESOffsets {
		// Range map is compressed; rough estimate is ~5-10% of index size
		rangeMapEstimate := indexSize / 10
		totalDedupSize += rangeMapEstimate
		footerSize = int64(dedup.FooterV4Size)
		rangeMapNote = fmt.Sprintf(" + ~%s range map", formatInt(rangeMapEstimate))
	}

	savings := float64(mkvSize-totalDedupSize) / float64(mkvSize) * 100

	fmt.Printf("Estimated dedup file size:\n")
	fmt.Printf("  Header:     %s bytes\n", formatInt(headerSize))
	fmt.Printf("  Index:      %s bytes (%s entries × %d)\n", formatInt(indexSize), formatInt(int64(len(result.Entries))), dedup.EntrySize)
	fmt.Printf("  Delta:      %s bytes\n", formatInt(int64(len(result.DeltaData))))
	fmt.Printf("  Footer:     %s bytes\n", formatInt(footerSize))
	fmt.Printf("  Total:      ~%s bytes (%.2f MB)%s\n", formatInt(totalDedupSize), float64(totalDedupSize)/(1024*1024), rangeMapNote)
	fmt.Printf("  Savings:    ~%.1f%% reduction\n", savings)

	return nil
}
