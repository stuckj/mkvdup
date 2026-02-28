package main

import (
	"fmt"
	"os"
)

func printUsage() {
	fmt.Print(`mkvdup - MKV deduplication tool using FUSE

Usage: mkvdup [options] <command> [args...]

Commands:
  create       Create dedup file from MKV + source directory
  batch-create Create multiple dedup files from one source
  probe        Quick test if MKV matches source(s)
  mount        Mount dedup files as FUSE filesystem
  info         Show dedup file information
  verify       Verify dedup file against original MKV
  extract      Rebuild original MKV from dedup + source
  check        Check dedup + source file integrity
  stats        Show space savings and file statistics
  validate     Validate configuration files
  reload       Reload running daemon's configuration

Analysis commands:
  deltadiag    Analyze unmatched regions by stream type

Debug commands:
  parse-mkv    Parse MKV and show packet info
  index-source Index source directory
  match        Match MKV packets to source

Options:
  -v, --verbose      Enable verbose output
  -q, --quiet        Suppress informational progress output
  --no-progress      Disable progress bars (still show status messages)
  --log-file PATH    Duplicate output to a log file (non-TTY style)
  --log-verbose      Enable verbose output in log file only
  -h, --help         Show help
  --version          Show version
`)
	fmt.Print(debugOptionsHelp())
	fmt.Print(`Run 'mkvdup <command> --help' for more information on a command.
See 'man mkvdup' for detailed documentation.
`)
}

func printCommandUsage(cmd string) {
	switch cmd {
	case "create":
		printCreateUsage()
	case "batch-create":
		printBatchCreateUsage()
	case "probe":
		printProbeUsage()
	case "mount":
		printMountUsage()
	case "info":
		printInfoUsage()
	case "verify":
		printVerifyUsage()
	case "extract":
		printExtractUsage()
	case "check":
		printCheckUsage()
	case "stats":
		printStatsUsage()
	case "validate":
		printValidateUsage()
	case "reload":
		printReloadUsage()
	case "deltadiag":
		printDeltadiagUsage()
	case "parse-mkv":
		printParseMKVUsage()
	case "index-source":
		printIndexSourceUsage()
	case "match":
		printMatchUsage()
	default:
		printUsage()
	}
}

func printCreateUsage() {
	fmt.Print(`Usage: mkvdup create [options] <mkv-file> <source-dir> <output> [name]

Create a dedup file from an MKV and its source media.

Arguments:
    <mkv-file>    Path to the MKV file to deduplicate
    <source-dir>  Directory containing source media (ISO files or BDMV folders)
    <output>      Output .mkvdup file path
    [name]        Display name in FUSE mount (default: basename of mkv-file;
                  .mkv extension auto-added if missing)

Options:
    -v, --verbose       Enable verbose/debug output
    --log-file PATH     Duplicate output to a log file (non-TTY style)
    --log-verbose       Enable verbose output in log file only
    --warn-threshold N  Minimum space savings percentage to avoid warning (default: 75)
    --non-interactive   Don't prompt on codec mismatch (show warning and continue)

Before matching, codecs in the MKV are compared against the source media.
If a mismatch is detected (e.g., MKV has H.264 but source is MPEG-2), you
will be prompted to continue. Use --non-interactive for scripted usage.

Examples:
    mkvdup create movie.mkv /media/dvd-backups movie.mkvdup
    mkvdup create movie.mkv /media/dvd-backups movie.mkvdup "My Movie"
    mkvdup create --warn-threshold 50 movie.mkv /media/dvd-backups movie.mkvdup
    mkvdup create --non-interactive movie.mkv /media/dvd-backups movie.mkvdup
`)
}

func printBatchCreateUsage() {
	fmt.Print(`Usage: mkvdup batch-create [options] <manifest.yaml>

Create multiple dedup files from a YAML manifest. Files sharing the same
source directory are grouped and the source is indexed once per group.

Codec compatibility is checked for each file. If a mismatch is detected,
a warning is printed but processing continues (non-interactive mode).
Use --skip-codec-mismatch to skip mismatched files instead.

Arguments:
    <manifest.yaml>  YAML manifest file specifying source(s) and MKV files

Options:
    -v, --verbose          Enable verbose/debug output
    --log-file PATH        Duplicate output to a log file (non-TTY style)
    --log-verbose          Enable verbose output in log file only
    --warn-threshold N     Minimum space savings percentage to avoid warning (default: 75)
    --skip-codec-mismatch  Skip MKVs with codec mismatch instead of processing them

Manifest format:
    source_dir: /media/dvd-backups/disc1   # default for all files (optional)
    files:
      - mkv: episode1.mkv
        output: episode1.mkvdup
        name: "Show/S01/Episode 1"         # optional (.mkv auto-added)
      - mkv: episode2.mkv
        output: episode2.mkvdup
      - mkv: movie.mkv
        output: movie.mkvdup
        source_dir: /media/dvd-backups/disc2  # per-file override

Fields:
    source_dir          Default source directory (optional if all files specify their own)
    files               List of MKV files to process (required, at least one)
    files[].mkv         Path to MKV file (required)
    files[].output      Output .mkvdup file (required)
    files[].source_dir  Source directory for this file (overrides top-level default)
    files[].name        Display name in FUSE mount (default: basename of mkv;
                        .mkv extension auto-added if missing)

Relative paths are resolved against the manifest file's directory.

Examples:
    mkvdup batch-create episodes.yaml
    mkvdup batch-create --warn-threshold 50 episodes.yaml
    mkvdup batch-create --skip-codec-mismatch episodes.yaml
`)
}

func printProbeUsage() {
	fmt.Print(`Usage: mkvdup probe <mkv-file>... -- <source-dir>...

Quick test to check if MKV file(s) match one or more source directories.
When multiple MKVs are provided, each source is indexed only once.

Arguments:
    <mkv-file>    One or more MKV files to test (before --)
    --            Separator between MKV files and source directories
    <source-dir>  One or more directories to test against (after --)

For backward compatibility, a single MKV without -- is also supported:
    mkvdup probe movie.mkv /media/disc1 /media/disc2

Examples:
    mkvdup probe movie.mkv /media/disc1 /media/disc2
    mkvdup probe ep1.mkv ep2.mkv ep3.mkv -- /media/disc1 /media/disc2
`)
}

func printMountUsage() {
	os.Stdout.WriteString(`Usage: mkvdup mount [options] <mountpoint> [config.yaml...]

Mount dedup files as a FUSE filesystem.

Arguments:
    <mountpoint>   Directory to mount the filesystem
    [config.yaml]  YAML config files (default: /etc/mkvdup.conf)

Options:
    --allow-other          Allow other users to access the mount
    --foreground           Run in foreground (for debugging or systemd)
    --config-dir           Treat config argument as directory of YAML files (.yaml, .yml)
    --pid-file PATH        Write daemon PID to file
    --daemon-timeout DUR   Timeout waiting for daemon startup (default: 30s)

Permission Options:
    --default-uid UID          Default UID for files and directories (default: calling user's UID)
    --default-gid GID          Default GID for files and directories (default: calling user's GID)
    --default-file-mode MODE   Default mode for files (octal, default: 0444)
    --default-dir-mode MODE    Default mode for directories (octal, default: 0555)
    --permissions-file PATH    Path to permissions file (overrides default locations)

Source Watch Options:
    --no-source-watch                    Disable source file monitoring (enabled by default)
    --on-source-change ACTION            Action on source change: warn, disable, checksum (default)
                                         warn     - log a warning
                                         disable  - disable affected virtual files (reads return EIO)
                                         checksum - size change: disable immediately
                                                    timestamp-only: verify checksum in background,
                                                    disable on mismatch, re-enable on pass
    --source-watch-poll-interval DUR     Poll interval for source file changes (default: 60s)
    --source-read-timeout DUR            Read timeout for network FS sources (default: 30s)

Error Notification (configured in YAML config, not CLI):
    on_error_command:
      command: ["/path/to/script", "%source%", "%event%", "%files%"]
      timeout: 30s          # command timeout (default: 30s)
      batch_interval: 5s    # debounce window for batching events (default: 5s)
    Placeholders: %source% (path), %files% (affected files), %event% (error type)
    String form (sh -c) auto-escapes placeholders; do not add your own quotes.
    See docs/FUSE.md for details.

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
    mkvdup mount --source-watch-poll-interval 10s /mnt/videos config.yaml
    mkvdup mount --source-read-timeout 1m /mnt/videos config.yaml
`)
}

func printInfoUsage() {
	fmt.Print(`Usage: mkvdup info [options] <dedup-file>

Show information about a dedup file.

Arguments:
    <dedup-file>  Path to the .mkvdup file

Options:
    --hide-unused-files  Hide source files not referenced by any index entry

Examples:
    mkvdup info movie.mkvdup
    mkvdup info --hide-unused-files movie.mkvdup
`)
}

func printVerifyUsage() {
	fmt.Print(`Usage: mkvdup verify <dedup-file> <source-dir> <original-mkv>

Verify that a dedup file correctly reconstructs the original MKV.

Arguments:
    <dedup-file>    Path to the .mkvdup file
    <source-dir>    Directory containing the source media
    <original-mkv>  Path to the original MKV for comparison

Examples:
    mkvdup verify movie.mkvdup /media/dvd-backups original.mkv
`)
}

func printExtractUsage() {
	fmt.Print(`Usage: mkvdup extract <dedup-file> <source-dir> <output-mkv>

Rebuild the original MKV from a dedup file and source media.

Arguments:
    <dedup-file>    Path to the .mkvdup file
    <source-dir>    Directory containing the source media
    <output-mkv>    Path for the reconstructed MKV file

Examples:
    mkvdup extract movie.mkvdup /media/dvd-backups restored-movie.mkv
`)
}

func printCheckUsage() {
	fmt.Print(`Usage: mkvdup check <dedup-file> <source-dir> [options]

Check integrity of a dedup file and its source files.

Arguments:
    <dedup-file>  Path to the .mkvdup file
    <source-dir>  Directory containing the source media

Options:
    --source-checksums  Verify source file checksums (slow, reads entire files)

Checks performed:
    - Dedup file header validity (magic, version, structure)
    - Index and delta checksum verification
    - Source file existence and size
    With --source-checksums:
    - Source file checksum verification (reads entire files)

Examples:
    mkvdup check movie.mkvdup /media/dvd-backups
    mkvdup check --source-checksums movie.mkvdup /media/dvd-backups
`)
}

func printStatsUsage() {
	fmt.Print(`Usage: mkvdup stats [options] <config.yaml...>

Show space savings and file statistics for mkvdup-managed files.

Arguments:
    <config.yaml>  YAML config files (same format as mount/validate)

Options:
    --config-dir   Treat config argument as directory of YAML files (.yaml, .yml)

Output includes per-file statistics (original size, dedup file size, space
savings, source type) and a rollup summary when multiple files are present.

Examples:
    mkvdup stats config.yaml
    mkvdup stats --config-dir /etc/mkvdup.d/
    mkvdup stats movie1.yaml movie2.yaml
`)
}

func printValidateUsage() {
	fmt.Print(`Usage: mkvdup validate [options] <config.yaml...>

Validate configuration files for correctness before mounting.

Arguments:
    <config.yaml>  YAML config files to validate

Options:
    --config-dir   Treat config argument as directory of YAML files (.yaml, .yml)
    --deep         Verify dedup file headers and internal checksums
    --strict       Treat warnings as errors (exit 1 on warnings)

Validations performed:
    - YAML syntax and required fields (name, dedup_file, source_dir)
    - Include cycle detection
    - Dedup file existence and header validity
    - Source directory existence
    - Duplicate virtual file names (warning)
    - File/directory path conflicts (warning)
    - Invalid path names (empty, contains "..")
    With --deep:
    - Dedup file internal checksum verification

Exit codes:
    0  All configs valid (warnings may be present)
    1  Errors found (or warnings with --strict)

Examples:
    mkvdup validate config.yaml
    mkvdup validate *.yaml
    mkvdup validate --config-dir /etc/mkvdup.d/
    mkvdup validate --deep --strict /etc/mkvdup.conf
`)
}

func printReloadUsage() {
	fmt.Print(`Usage: mkvdup reload {--pid-file PATH | --pid PID} [options] [config.yaml...]

Reload a running daemon's configuration by validating the config
and sending SIGHUP to the daemon process.

The config is validated BEFORE sending the signal. If validation
fails, the signal is not sent and the error is reported.

If no config files are specified, the signal is sent without
pre-validation (the daemon validates internally on SIGHUP).

Arguments:
    [config.yaml]  Config files to validate (same as mount's config args)

Required (one of):
    --pid-file PATH    PID file of running daemon (must match mount's --pid-file)
    --pid PID          PID of the running daemon (e.g., for foreground mode)

Options:
    --config-dir       Treat config argument as directory of YAML files

Examples:
    mkvdup reload --pid-file /run/mkvdup.pid config.yaml
    mkvdup reload --pid-file /run/mkvdup.pid --config-dir /etc/mkvdup.d/
    mkvdup reload --pid-file /run/mkvdup.pid
    mkvdup reload --pid $(pidof mkvdup)
`)
}

func printDeltadiagUsage() {
	fmt.Print(`Usage: mkvdup deltadiag <dedup-file> <mkv-file>

Analyze unmatched (delta) regions in a dedup file by cross-referencing
with the original MKV to determine what stream type each delta region
belongs to (video, audio, or container overhead).

For video delta, further classifies by H.264 NAL type (IDR/non-IDR slices,
SEI, SPS, PPS, etc.) and shows size breakdown.

Works with dedup file versions 3 through 8 (DVD, Blu-ray, and newer).

Arguments:
    <dedup-file>  Path to the .mkvdup file
    <mkv-file>    Path to the original MKV file

Examples:
    mkvdup deltadiag movie.mkvdup movie.mkv
`)
}

func printParseMKVUsage() {
	fmt.Print(`Usage: mkvdup parse-mkv <mkv-file>

Parse an MKV file and display packet information (debugging).

Arguments:
    <mkv-file>  Path to the MKV file to parse

Examples:
    mkvdup parse-mkv movie.mkv
`)
}

func printIndexSourceUsage() {
	fmt.Print(`Usage: mkvdup index-source <source-dir>

Index a source directory and display statistics (debugging).

Arguments:
    <source-dir>  Directory containing source media (ISO files or BDMV folders)

Examples:
    mkvdup index-source /media/dvd-backups
`)
}

func printMatchUsage() {
	fmt.Print(`Usage: mkvdup match <mkv-file> <source-dir>

Match MKV packets to source and show detailed results (debugging).

Arguments:
    <mkv-file>    Path to the MKV file
    <source-dir>  Directory containing source media

Examples:
    mkvdup match movie.mkv /media/dvd-backups
`)
}
