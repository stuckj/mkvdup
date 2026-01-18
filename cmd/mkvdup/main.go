// Command mkvdup is the CLI tool for MKV-ISO deduplication.
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
)

// verbose is set to true when -v flag is passed
var verbose bool

func main() {
	// Check for -v flag
	args := os.Args[1:]
	for i, arg := range args {
		if arg == "-v" || arg == "--verbose" {
			verbose = true
			args = append(args[:i], args[i+1:]...)
			break
		}
	}

	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [-v] <command> [args...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nGlobal options:\n")
		fmt.Fprintf(os.Stderr, "  -v, --verbose                          Enable verbose output\n")
		fmt.Fprintf(os.Stderr, "\nCommands:\n")
		fmt.Fprintf(os.Stderr, "  create <mkv> <source> [output] [name]  Create dedup file from MKV + source\n")
		fmt.Fprintf(os.Stderr, "  probe <mkv> <source>...                Quick test if MKV matches source(s)\n")
		fmt.Fprintf(os.Stderr, "  mount <mountpoint> <config.yaml>...    Mount dedup files as FUSE filesystem\n")
		fmt.Fprintf(os.Stderr, "  info <dedup>                           Show dedup file information\n")
		fmt.Fprintf(os.Stderr, "  verify <dedup> <source> <original>     Verify dedup file\n")
		fmt.Fprintf(os.Stderr, "  parse-mkv <file.mkv>                   Parse MKV and show packet info\n")
		fmt.Fprintf(os.Stderr, "  index-source <dir>                     Index source directory\n")
		fmt.Fprintf(os.Stderr, "  match <file.mkv> <source>              Match MKV packets to source\n")
		os.Exit(1)
	}

	cmd := args[0]
	args = args[1:]

	switch cmd {
	case "create":
		if len(args) < 2 {
			log.Fatal("Usage: create <mkv> <source_dir> [output] [name]")
		}
		output := ""
		name := ""
		if len(args) >= 3 {
			output = args[2]
		}
		if len(args) >= 4 {
			name = args[3]
		}
		if err := createDedup(args[0], args[1], output, name); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "probe":
		if len(args) < 2 {
			log.Fatal("Usage: probe <mkv> <source_dir>...")
		}
		if err := probe(args[0], args[1:]); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "mount":
		if len(args) < 2 {
			log.Fatal("Usage: mount <mountpoint> <config.yaml>...")
		}
		if err := mountFuse(args[0], args[1:]); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "info":
		if len(args) < 1 {
			log.Fatal("Usage: info <dedup_file>")
		}
		if err := showInfo(args[0]); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "verify":
		if len(args) < 3 {
			log.Fatal("Usage: verify <dedup_file> <source_dir> <original_mkv>")
		}
		if err := verifyDedup(args[0], args[1], args[2]); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "parse-mkv":
		if len(args) < 1 {
			log.Fatal("Usage: parse-mkv <file.mkv>")
		}
		if err := parseMKV(args[0]); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "index-source":
		if len(args) < 1 {
			log.Fatal("Usage: index-source <dir>")
		}
		if err := indexSource(args[0]); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "match":
		if len(args) < 2 {
			log.Fatal("Usage: match <file.mkv> <source_dir>")
		}
		if err := matchMKV(args[0], args[1]); err != nil {
			log.Fatalf("Error: %v", err)
		}

	default:
		log.Fatalf("Unknown command: %s", cmd)
	}
}

func parseMKV(path string) error {
	fmt.Printf("Parsing MKV file: %s\n", path)

	parser, err := mkv.NewParser(path)
	if err != nil {
		return fmt.Errorf("create parser: %w", err)
	}
	defer parser.Close()

	fmt.Printf("File size: %d bytes (%.2f GB)\n", parser.Size(), float64(parser.Size())/(1024*1024*1024))

	start := time.Now()
	lastProgress := time.Now()

	err = parser.Parse(func(processed, total int64) {
		if time.Since(lastProgress) > 500*time.Millisecond {
			pct := float64(processed) / float64(total) * 100
			fmt.Printf("\rProgress: %.1f%% (%d / %d bytes)", pct, processed, total)
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
		fmt.Printf("  Track %d: %s (codec: %s)\n", t.Number, typeStr, t.CodecID)
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
			fmt.Printf("\rProgress: %.1f%% (%d / %d bytes)", pct, processed, total)
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
		fmt.Printf("  %s: %d bytes\n", f.RelativePath, f.Size)
	}
	fmt.Println()

	fmt.Printf("Unique hashes: %d\n", len(index.HashToLocations))
	if index.UsesESOffsets {
		fmt.Printf("Index type: ES-aware (MPEG-PS)\n")
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
	fmt.Println("Phase 1/3: Parsing MKV file...")
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

	// Phase 2: Index source
	fmt.Println("Phase 2/3: Indexing source...")
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
	if index.UsesESOffsets {
		fmt.Println("  (Using ES-aware indexing for MPEG-PS)")
	}

	// Phase 3: Match packets
	fmt.Println("Phase 3/3: Matching packets...")
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

	fmt.Printf("Packets matched:    %d / %d (%.1f%%)\n",
		result.MatchedPackets, result.TotalPackets,
		float64(result.MatchedPackets)/float64(result.TotalPackets)*100)
	fmt.Printf("Index entries:      %d\n", len(result.Entries))
	fmt.Println()

	// Storage savings
	indexSize := int64(len(result.Entries) * 25) // Approximate: each entry ~25 bytes
	headerSize := int64(57)
	totalDedupSize := headerSize + indexSize + int64(len(result.DeltaData))
	savings := float64(mkvSize-totalDedupSize) / float64(mkvSize) * 100

	fmt.Printf("Estimated dedup file size:\n")
	fmt.Printf("  Header:     %d bytes\n", headerSize)
	fmt.Printf("  Index:      %d bytes (~%d entries × 25)\n", indexSize, len(result.Entries))
	fmt.Printf("  Delta:      %d bytes\n", len(result.DeltaData))
	fmt.Printf("  Total:      %d bytes (%.2f MB)\n", totalDedupSize, float64(totalDedupSize)/(1024*1024))
	fmt.Printf("  Savings:    %.1f%% reduction\n", savings)

	return nil
}
