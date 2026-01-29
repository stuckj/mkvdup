// Command mkvdup is the CLI tool for MKV-ISO deduplication.
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
)

// MountOptions holds all options for the mount command.
type MountOptions struct {
	AllowOther      bool
	Foreground      bool
	ConfigDir       bool
	PidFile         string
	DaemonTimeout   time.Duration
	PermissionsFile string
	DefaultUID      uint32
	DefaultGID      uint32
	DefaultFileMode uint32
	DefaultDirMode  uint32
}

// parseUint32 parses a string as uint32.
func parseUint32(s string) (uint32, error) {
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(v), nil
}

// parseOctalMode parses a string as an octal file mode.
func parseOctalMode(s string) (uint32, error) {
	// Strip leading 0 prefix for octal if present
	v, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, err
	}
	return uint32(v), nil
}

// version is set at build time via -ldflags
var version = "dev"

// verbose is set to true when -v flag is passed
var verbose bool

func printVersion() {
	fmt.Printf("mkvdup version %s\n", version)
}

func printUsage() {
	fmt.Print(`mkvdup - MKV deduplication tool using FUSE

Usage: mkvdup [options] <command> [args...]

Commands:
  create       Create dedup file from MKV + source directory
  probe        Quick test if MKV matches source(s)
  mount        Mount dedup files as FUSE filesystem
  info         Show dedup file information
  verify       Verify dedup file against original MKV

Debug commands:
  parse-mkv    Parse MKV and show packet info
  index-source Index source directory
  match        Match MKV packets to source

Options:
  -v, --verbose   Enable verbose output
  -h, --help      Show help
  --version       Show version
`)
	fmt.Print(debugOptionsHelp())
	fmt.Print(`Run 'mkvdup <command> --help' for more information on a command.
See 'man mkvdup' for detailed documentation.
`)
}

func printCommandUsage(cmd string) {
	switch cmd {
	case "create":
		fmt.Print(`Usage: mkvdup create <mkv-file> <source-dir> [output] [name]

Create a dedup file from an MKV and its source media.

Arguments:
    <mkv-file>    Path to the MKV file to deduplicate
    <source-dir>  Directory containing source media (ISO files or BDMV folders)
    [output]      Output .mkvdup file (default: <mkv-file>.mkvdup)
    [name]        Display name in FUSE mount (default: basename of mkv-file)

Examples:
    mkvdup create movie.mkv /media/dvd-backups
    mkvdup create movie.mkv /media/dvd-backups movie.mkvdup "My Movie"
`)
	case "probe":
		fmt.Print(`Usage: mkvdup probe <mkv-file> <source-dir>...

Quick test to check if an MKV matches one or more source directories.

Arguments:
    <mkv-file>    Path to the MKV file to test
    <source-dir>  One or more directories to test against

Examples:
    mkvdup probe movie.mkv /media/disc1 /media/disc2 /media/disc3
`)
	case "mount":
		fmt.Print(`Usage: mkvdup mount [options] <mountpoint> [config.yaml...]

Mount dedup files as a FUSE filesystem.

Arguments:
    <mountpoint>   Directory to mount the filesystem
    [config.yaml]  YAML config files (default: /etc/mkvdup.conf)

Options:
    --allow-other          Allow other users to access the mount
    --foreground           Run in foreground (for debugging or systemd)
    --config-dir           Treat config argument as directory of .yaml files
    --pid-file PATH        Write daemon PID to file
    --daemon-timeout DUR   Timeout waiting for daemon startup (default: 30s)

Permission Options:
    --default-uid UID          Default UID for files and directories (default: calling user's UID)
    --default-gid GID          Default GID for files and directories (default: calling user's GID)
    --default-file-mode MODE   Default mode for files (octal, default: 0444)
    --default-dir-mode MODE    Default mode for directories (octal, default: 0555)
    --permissions-file PATH    Path to permissions file (overrides default locations)

By default, mkvdup daemonizes after the mount is ready and returns.
Use --foreground to keep it attached to the terminal.

Permission files are searched in order:
  1. --permissions-file (if specified)
  2. ~/.config/mkvdup/permissions.yaml (if exists)
  3. /etc/mkvdup/permissions.yaml (if exists)
New permissions are written to ~/.config/mkvdup/permissions.yaml (user) or
/etc/mkvdup/permissions.yaml (root).

Examples:
    mkvdup mount /mnt/videos movie.mkvdup.yaml
    mkvdup mount /mnt/videos *.yaml
    mkvdup mount --allow-other /mnt/videos
    mkvdup mount --config-dir /mnt/videos /etc/mkvdup.d/
    mkvdup mount --foreground /mnt/videos config.yaml
    mkvdup mount --default-uid 1000 --default-gid 1000 /mnt/videos config.yaml
`)
	case "info":
		fmt.Print(`Usage: mkvdup info <dedup-file>

Show information about a dedup file.

Arguments:
    <dedup-file>  Path to the .mkvdup file

Examples:
    mkvdup info movie.mkvdup
`)
	case "verify":
		fmt.Print(`Usage: mkvdup verify <dedup-file> <source-dir> <original-mkv>

Verify that a dedup file correctly reconstructs the original MKV.

Arguments:
    <dedup-file>    Path to the .mkvdup file
    <source-dir>    Directory containing the source media
    <original-mkv>  Path to the original MKV for comparison

Examples:
    mkvdup verify movie.mkvdup /media/dvd-backups original.mkv
`)
	case "parse-mkv":
		fmt.Print(`Usage: mkvdup parse-mkv <mkv-file>

Parse an MKV file and display packet information (debugging).

Arguments:
    <mkv-file>  Path to the MKV file to parse

Examples:
    mkvdup parse-mkv movie.mkv
`)
	case "index-source":
		fmt.Print(`Usage: mkvdup index-source <source-dir>

Index a source directory and display statistics (debugging).

Arguments:
    <source-dir>  Directory containing source media (ISO files or BDMV folders)

Examples:
    mkvdup index-source /media/dvd-backups
`)
	case "match":
		fmt.Print(`Usage: mkvdup match <mkv-file> <source-dir>

Match MKV packets to source and show detailed results (debugging).

Arguments:
    <mkv-file>    Path to the MKV file
    <source-dir>  Directory containing source media

Examples:
    mkvdup match movie.mkv /media/dvd-backups
`)
	default:
		printUsage()
	}
}

func main() {
	// Process global flags before command
	args := os.Args[1:]
	var filteredArgs []string
	showHelp := false
	showVersion := false

	// Extract --cpuprofile flag (only available in debug builds)
	args, cpuprofile := parseCPUProfileFlag(args)
	defer startCPUProfile(cpuprofile)()

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-v" || arg == "--verbose":
			verbose = true
		case arg == "-h" || arg == "--help":
			showHelp = true
		case arg == "--version":
			showVersion = true
		default:
			filteredArgs = append(filteredArgs, arg)
		}
	}
	args = filteredArgs

	// Handle --version (always top-level)
	if showVersion {
		printVersion()
		os.Exit(0)
	}

	// If no command given, show appropriate help
	if len(args) < 1 {
		if showHelp {
			printUsage()
			os.Exit(0)
		}
		printUsage()
		os.Exit(1)
	}

	cmd := args[0]
	args = args[1:]

	// If help flag was given with a command, show command-specific help
	if showHelp {
		printCommandUsage(cmd)
		os.Exit(0)
	}

	switch cmd {
	case "create":
		if len(args) < 2 {
			printCommandUsage("create")
			os.Exit(1)
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
			printCommandUsage("probe")
			os.Exit(1)
		}
		if err := probe(args[0], args[1:]); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "mount":
		// Parse mount-specific options
		allowOther := false
		foreground := false
		configDir := false
		pidFile := ""
		daemonTimeout := 30 * time.Second
		permissionsFile := ""
		defaultUID := uint32(os.Getuid())
		defaultGID := uint32(os.Getgid())
		defaultFileMode := uint32(0444)
		defaultDirMode := uint32(0555)
		var mountArgs []string
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--allow-other":
				allowOther = true
			case "--foreground", "-f":
				foreground = true
			case "--config-dir":
				configDir = true
			case "--pid-file":
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					pidFile = args[i+1]
					i++
				} else {
					log.Fatalf("Error: --pid-file requires a path argument")
				}
			case "--daemon-timeout":
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					d, err := time.ParseDuration(args[i+1])
					if err != nil {
						log.Fatalf("Error: --daemon-timeout invalid duration: %v", err)
					}
					daemonTimeout = d
					i++
				} else {
					log.Fatalf("Error: --daemon-timeout requires a duration argument (e.g., 30s, 1m)")
				}
			case "--permissions-file":
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					permissionsFile = args[i+1]
					i++
				} else {
					log.Fatalf("Error: --permissions-file requires a path argument")
				}
			case "--default-uid":
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					uid, err := parseUint32(args[i+1])
					if err != nil {
						log.Fatalf("Error: --default-uid invalid: %v", err)
					}
					defaultUID = uid
					i++
				} else {
					log.Fatalf("Error: --default-uid requires a numeric argument")
				}
			case "--default-gid":
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					gid, err := parseUint32(args[i+1])
					if err != nil {
						log.Fatalf("Error: --default-gid invalid: %v", err)
					}
					defaultGID = gid
					i++
				} else {
					log.Fatalf("Error: --default-gid requires a numeric argument")
				}
			case "--default-file-mode":
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					mode, err := parseOctalMode(args[i+1])
					if err != nil {
						log.Fatalf("Error: --default-file-mode invalid: %v", err)
					}
					defaultFileMode = mode
					i++
				} else {
					log.Fatalf("Error: --default-file-mode requires an octal mode argument")
				}
			case "--default-dir-mode":
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					mode, err := parseOctalMode(args[i+1])
					if err != nil {
						log.Fatalf("Error: --default-dir-mode invalid: %v", err)
					}
					defaultDirMode = mode
					i++
				} else {
					log.Fatalf("Error: --default-dir-mode requires an octal mode argument")
				}
			default:
				mountArgs = append(mountArgs, args[i])
			}
		}
		if len(mountArgs) < 1 {
			printCommandUsage("mount")
			os.Exit(1)
		}
		mountpoint := mountArgs[0]
		configPaths := mountArgs[1:]
		mountOpts := MountOptions{
			AllowOther:      allowOther,
			Foreground:      foreground,
			ConfigDir:       configDir,
			PidFile:         pidFile,
			DaemonTimeout:   daemonTimeout,
			PermissionsFile: permissionsFile,
			DefaultUID:      defaultUID,
			DefaultGID:      defaultGID,
			DefaultFileMode: defaultFileMode,
			DefaultDirMode:  defaultDirMode,
		}
		if err := mountFuse(mountpoint, configPaths, mountOpts); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "info":
		if len(args) < 1 {
			printCommandUsage("info")
			os.Exit(1)
		}
		if err := showInfo(args[0]); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "verify":
		if len(args) < 3 {
			printCommandUsage("verify")
			os.Exit(1)
		}
		if err := verifyDedup(args[0], args[1], args[2]); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "parse-mkv":
		if len(args) < 1 {
			printCommandUsage("parse-mkv")
			os.Exit(1)
		}
		if err := parseMKV(args[0]); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "index-source":
		if len(args) < 1 {
			printCommandUsage("index-source")
			os.Exit(1)
		}
		if err := indexSource(args[0]); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "match":
		if len(args) < 2 {
			printCommandUsage("match")
			os.Exit(1)
		}
		if err := matchMKV(args[0], args[1]); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "help":
		if len(args) > 0 {
			printCommandUsage(args[0])
		} else {
			printUsage()
		}
		os.Exit(0)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
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
