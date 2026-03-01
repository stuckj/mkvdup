// Command mkvdup is the CLI tool for MKV-ISO deduplication.
package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/stuckj/mkvdup/internal/daemon"
	"github.com/stuckj/mkvdup/internal/dedup"
)

// MountOptions holds all options for the mount command.
type MountOptions struct {
	AllowOther              bool
	Foreground              bool
	ConfigDir               bool
	PidFile                 string
	DaemonTimeout           time.Duration
	PermissionsFile         string
	DefaultUID              uint32
	DefaultGID              uint32
	DefaultFileMode         uint32
	DefaultDirMode          uint32
	NoSourceWatch           bool                      // Disable source file watching
	OnSourceChange          string                    // Action on source change: "warn", "disable", "checksum"
	SourceWatchPollInterval time.Duration             // Poll interval for network FS source watching (0 = 60s default)
	SourceReadTimeout       time.Duration             // Pread timeout for network FS sources (0 = disabled; CLI default 30s)
	OnErrorCommand          *dedup.ErrorCommandConfig // External command to run on source integrity error (from YAML config)
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

// parseWarnFlags extracts --warn-threshold from args, returning the
// parsed value and the remaining positional arguments.
func parseWarnFlags(args []string) (warnThreshold float64, remaining []string) {
	warnThreshold = 75.0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--warn-threshold":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				v, err := strconv.ParseFloat(args[i+1], 64)
				if err != nil {
					log.Fatalf("Error: --warn-threshold invalid: %v", err)
				}
				if v < 0 || v > 100 {
					log.Fatalf("Error: --warn-threshold must be between 0 and 100")
				}
				warnThreshold = v
				i++
			} else {
				log.Fatalf("Error: --warn-threshold requires a numeric argument")
			}
		default:
			remaining = append(remaining, args[i])
		}
	}
	return
}

// isTerminalStdout returns true if stdout is a terminal (not piped/redirected).
func isTerminalStdout() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// version is set at build time via -ldflags
var version = "dev"

// verbose is set to true when -v flag is passed
var verbose bool

// logVerbose enables verbose diagnostics only in the log file (not on console)
var logVerbose bool

// showProgress controls whether progress bars are rendered. Set to false by
// --no-progress, --quiet, or when stdout is not a TTY.
var showProgress = true

// quiet suppresses all informational stdout output. Errors still go to stderr.
var quiet bool

func printVersion() {
	fmt.Printf("mkvdup version %s\n", version)
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
		case arg == "--log-verbose":
			logVerbose = true
		case arg == "--no-progress":
			showProgress = false
		case arg == "-q" || arg == "--quiet":
			quiet = true
			showProgress = false
		case arg == "--log-file":
			if i+1 < len(args) {
				i++
				var err error
				logFile, err = os.Create(args[i])
				if err != nil {
					log.Fatalf("Error: cannot create log file %s: %v", args[i], err)
				}
			} else {
				log.Fatalf("Error: --log-file requires a path argument")
			}
		default:
			filteredArgs = append(filteredArgs, arg)
		}
	}
	args = filteredArgs

	// Auto-disable progress bars when stdout is not a TTY
	if !isTerminalStdout() {
		showProgress = false
	}

	// Duplicate log package output (used for warnings and fatal errors) to
	// the log file so that log.Printf and log.Fatalf messages appear there too.
	if logFile != nil {
		log.SetOutput(io.MultiWriter(os.Stderr, logFile))
		defer logFile.Close()
	}

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
		warnThreshold, remaining := parseWarnFlags(args)
		nonInteractive := false
		var createArgs []string
		for i := 0; i < len(remaining); i++ {
			switch remaining[i] {
			case "--non-interactive":
				nonInteractive = true
			default:
				createArgs = append(createArgs, remaining[i])
			}
		}
		if len(createArgs) < 3 {
			printCommandUsage("create")
			os.Exit(1)
		}
		output := createArgs[2]
		name := ""
		if len(createArgs) >= 4 {
			name = createArgs[3]
		}
		if err := createDedup(createArgs[0], createArgs[1], output, name, warnThreshold, nonInteractive); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "batch-create":
		warnThreshold, remaining := parseWarnFlags(args)
		skipCodecMismatch := false
		var batchArgs []string
		for _, arg := range remaining {
			if arg == "--skip-codec-mismatch" {
				skipCodecMismatch = true
			} else {
				batchArgs = append(batchArgs, arg)
			}
		}
		if len(batchArgs) < 1 {
			printCommandUsage("batch-create")
			os.Exit(1)
		}
		if err := createBatch(batchArgs[0], warnThreshold, skipCodecMismatch); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "probe":
		if len(args) < 2 {
			printCommandUsage("probe")
			os.Exit(1)
		}
		// Split on "--": MKVs before, sources after
		// For backward compat: if no "--", first arg is MKV, rest are sources
		var mkvPaths, sourceDirs []string
		sepIdx := -1
		for i, a := range args {
			if a == "--" {
				sepIdx = i
				break
			}
		}
		if sepIdx >= 0 {
			mkvPaths = args[:sepIdx]
			sourceDirs = args[sepIdx+1:]
		} else {
			mkvPaths = args[:1]
			sourceDirs = args[1:]
		}
		if len(mkvPaths) == 0 || len(sourceDirs) == 0 {
			printCommandUsage("probe")
			os.Exit(1)
		}
		if err := probe(mkvPaths, sourceDirs); err != nil {
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
		noSourceWatch := false
		onSourceChange := "checksum"
		sourceWatchPollInterval := time.Duration(0)
		sourceReadTimeout := 30 * time.Second
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
			case "--no-source-watch":
				noSourceWatch = true
			case "--on-source-change":
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					onSourceChange = args[i+1]
					switch onSourceChange {
					case "warn", "disable", "checksum":
						// valid
					default:
						log.Fatalf("Error: --on-source-change must be warn, disable, or checksum")
					}
					i++
				} else {
					log.Fatalf("Error: --on-source-change requires an argument (warn, disable, or checksum)")
				}
			case "--source-watch-poll-interval":
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					d, err := time.ParseDuration(args[i+1])
					if err != nil {
						log.Fatalf("Error: --source-watch-poll-interval invalid duration: %v", err)
					}
					if d <= 0 {
						log.Fatalf("Error: --source-watch-poll-interval must be positive")
					}
					sourceWatchPollInterval = d
					i++
				} else {
					log.Fatalf("Error: --source-watch-poll-interval requires a duration argument (e.g., 10s, 5m)")
				}
			case "--source-read-timeout":
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					d, err := time.ParseDuration(args[i+1])
					if err != nil {
						log.Fatalf("Error: --source-read-timeout invalid duration: %v", err)
					}
					if d < 0 {
						log.Fatalf("Error: --source-read-timeout must be non-negative")
					}
					sourceReadTimeout = d
					i++
				} else {
					log.Fatalf("Error: --source-read-timeout requires a duration argument (e.g., 30s, 1m)")
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
			AllowOther:              allowOther,
			Foreground:              foreground,
			ConfigDir:               configDir,
			PidFile:                 pidFile,
			DaemonTimeout:           daemonTimeout,
			PermissionsFile:         permissionsFile,
			DefaultUID:              defaultUID,
			DefaultGID:              defaultGID,
			DefaultFileMode:         defaultFileMode,
			DefaultDirMode:          defaultDirMode,
			NoSourceWatch:           noSourceWatch,
			OnSourceChange:          onSourceChange,
			SourceWatchPollInterval: sourceWatchPollInterval,
			SourceReadTimeout:       sourceReadTimeout,
		}
		if err := mountFuse(mountpoint, configPaths, mountOpts); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "info":
		hideUnused := false
		var infoArgs []string
		for _, a := range args {
			if a == "--hide-unused-files" {
				hideUnused = true
			} else {
				infoArgs = append(infoArgs, a)
			}
		}
		if len(infoArgs) < 1 {
			printCommandUsage("info")
			os.Exit(1)
		}
		if err := showInfo(infoArgs[0], hideUnused); err != nil {
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

	case "extract":
		if len(args) < 3 {
			printCommandUsage("extract")
			os.Exit(1)
		}
		if err := extractDedup(args[0], args[1], args[2]); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "check":
		sourceChecksums := false
		var checkArgs []string
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--source-checksums":
				sourceChecksums = true
			default:
				checkArgs = append(checkArgs, args[i])
			}
		}
		if len(checkArgs) < 2 {
			printCommandUsage("check")
			os.Exit(1)
		}
		if err := checkDedup(checkArgs[0], checkArgs[1], sourceChecksums); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "stats":
		configDir := false
		var statsArgs []string
		for _, arg := range args {
			if arg == "--config-dir" {
				configDir = true
			} else {
				statsArgs = append(statsArgs, arg)
			}
		}
		if len(statsArgs) < 1 {
			printCommandUsage("stats")
			os.Exit(1)
		}
		if err := showStats(statsArgs, configDir); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "validate":
		configDir := false
		deep := false
		strict := false
		var valArgs []string
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--config-dir":
				configDir = true
			case "--deep":
				deep = true
			case "--strict":
				strict = true
			default:
				valArgs = append(valArgs, args[i])
			}
		}
		if len(valArgs) < 1 {
			printCommandUsage("validate")
			os.Exit(1)
		}
		os.Exit(validateConfigs(valArgs, configDir, deep, strict))

	case "reload":
		if len(args) == 0 {
			printCommandUsage("reload")
			os.Exit(1)
		}
		pidFile := ""
		pidDirect := 0
		configDir := false
		var reloadArgs []string
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--pid-file":
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					pidFile = args[i+1]
					i++
				} else {
					log.Fatalf("Error: --pid-file requires a path argument")
				}
			case "--pid":
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					p, err := strconv.Atoi(args[i+1])
					if err != nil || p <= 0 {
						log.Fatalf("Error: --pid requires a positive integer argument")
					}
					pidDirect = p
					i++
				} else {
					log.Fatalf("Error: --pid requires a PID argument")
				}
			case "--config-dir":
				configDir = true
			default:
				reloadArgs = append(reloadArgs, args[i])
			}
		}
		if pidFile != "" && pidDirect != 0 {
			log.Fatalf("Error: --pid-file and --pid are mutually exclusive")
		}
		var pid int
		if pidDirect != 0 {
			pid = pidDirect
		} else if pidFile != "" {
			var err error
			pid, err = daemon.ReadPidFile(pidFile)
			if err != nil {
				log.Fatalf("Error: %v", err)
			}
		} else {
			log.Fatalf("Error: --pid-file or --pid is required for reload")
		}
		if err := reloadDaemon(pid, reloadArgs, configDir); err != nil {
			log.Fatalf("Error: %v", err)
		}

	case "deltadiag":
		if len(args) < 2 {
			printCommandUsage("deltadiag")
			os.Exit(1)
		}
		if err := deltadiag(args[0], args[1]); err != nil {
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
		printWarn("Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}
