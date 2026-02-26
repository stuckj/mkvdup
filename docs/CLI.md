# CLI Commands

*Command-line interface for mkvdup.*

[Back to Architecture Overview](../DESIGN.md)

## Global Options

```bash
# Enable verbose/debug output for any command
mkvdup -v <command> [args...]
mkvdup --verbose <command> [args...]

# Suppress informational progress output (errors still go to stderr)
mkvdup -q <command> [args...]
mkvdup --quiet <command> [args...]

# Disable progress bars (phase labels and completion times still shown)
mkvdup --no-progress <command> [args...]

# Duplicate output to a log file (non-TTY style)
mkvdup --log-file /path/to/logfile <command> [args...]

# Enable verbose diagnostics in log file only (not on console)
mkvdup --log-verbose --log-file /path/to/logfile <command> [args...]

# Examples:
mkvdup -v create video.mkv /source/dir
mkvdup -q create video.mkv /source/dir
mkvdup --no-progress create video.mkv /source/dir
mkvdup -v mount /mnt/media config1.yaml config2.yaml
mkvdup -v verify video.mkvdup /source/dir video.mkv
mkvdup --log-verbose --log-file out.log create video.mkv /source/dir video.mkvdup
```

**Verbose mode enables:**
- FUSE operation logging (Open, Read, Lookup, Readdir)
- Detailed verification output with byte comparisons
- Debug information for troubleshooting

**Quiet mode (`-q`, `--quiet`):**
- Suppresses informational progress output during `create`, `batch-create`, `verify`, and `extract` (phase labels, progress bars, statistics)
- Errors still go to stderr
- Implies `--no-progress`

**No-progress mode (`--no-progress`):**
- Disables visual progress bars
- Phase labels and completion times are still printed
- Milestone percentages are printed at 10% intervals (so redirected logs still show progress)
- Automatically enabled when stdout is not a terminal (e.g., piped to a file)

**Log file (`--log-file PATH`):**
- Duplicates all informational output to the specified file
- Uses non-TTY style: milestone percentages at 10% intervals instead of progress bars, no ANSI escape sequences
- Output is written regardless of `--quiet` (quiet only suppresses stdout)
- Useful for capturing progress when running in the background or via systemd

**Log-verbose mode (`--log-verbose`):**
- Enables verbose diagnostic output in the log file only (not on console)
- Requires `--log-file` to have effect (without a log file, there is nowhere to write)
- When `--verbose` is also set, `--verbose` takes precedence (diagnostics go to both stderr and log file)
- Useful for background or headless runs where you want diagnostics captured for later review without cluttering the console

## Commands

### create

Create a dedup file from an MKV and its source directory.

```bash
mkvdup create [options] <mkv-file> <source-dir> <output> [name]

# Examples:
mkvdup create movie.mkv /media/dvd-backups movie.mkvdup
mkvdup create movie.mkv /media/dvd-backups movie.mkvdup "Movies/Action/My Movie"
mkvdup create --warn-threshold 50 movie.mkv /media/dvd-backups movie.mkvdup
mkvdup create --non-interactive movie.mkv /media/dvd-backups movie.mkvdup
```

**Arguments:**
- `<mkv-file>` — Path to the MKV file to deduplicate
- `<source-dir>` — Directory containing source media (ISO files or BDMV folders)
- `<output>` — Output `.mkvdup` file path
- `[name]` — Display name in FUSE mount (default: basename of mkv-file; `.mkv` extension auto-added if missing)

**Options:**

| Option | Description |
|--------|-------------|
| `--warn-threshold N` | Minimum space savings percentage to avoid warning (default: `75`) |
| `--non-interactive` | Don't prompt on codec mismatch (show warning and continue) |

**Codec check:** Before matching, codecs in the MKV are compared against the source media. If a mismatch is detected (e.g., MKV has H.264 but source is MPEG-2), you will be prompted to continue or abort. Use `--non-interactive` for scripted usage. When stdin is not a terminal, non-interactive mode is used automatically.

**Outputs:**
- `video.mkvdup` — The dedup data file (index + delta)
- `video.mkvdup.yaml` — Config file for this mapping

**Directory paths in `name`:**
The `name` argument supports directory paths (e.g., `"Movies/Action/Video1.mkv"`). Each `create` command produces one `.mkvdup` file with one name stored in its config. The directory structure becomes visible when mounting multiple configs together—directories are auto-created from path components across all mounted files. See [FUSE Directory Structure](FUSE.md#directory-structure) for details.

### batch-create

Create multiple dedup files from a YAML manifest. Files sharing the same source directory are grouped and the source is indexed once per group, which is significantly faster than running `create` separately for each file. A single manifest can reference multiple source directories. Codec compatibility is checked for each file; if a mismatch is detected, a warning is printed but processing continues (always non-interactive). Use `--skip-codec-mismatch` to skip mismatched files instead.

```bash
mkvdup batch-create [options] <manifest.yaml>

# Examples:
mkvdup batch-create episodes.yaml
mkvdup batch-create --warn-threshold 50 episodes.yaml
mkvdup batch-create --skip-codec-mismatch episodes.yaml
```

**Arguments:**
- `<manifest.yaml>` — YAML manifest specifying source directory(ies) and MKV files

**Options:**

| Option | Description |
|--------|-------------|
| `--warn-threshold N` | Minimum space savings percentage to avoid warning (default: `75`) |
| `--skip-codec-mismatch` | Skip MKVs with codec mismatch instead of processing them |

**Manifest format:**

```yaml
# Single source (all files share the same source):
source_dir: /media/dvd-backups/disc1

files:
  - mkv: episode1.mkv
    output: episode1.mkvdup
    name: "Show/S01/Episode 1" # optional (.mkv auto-added)

  - mkv: episode2.mkv
    output: episode2.mkvdup
```

```yaml
# Multiple sources (per-file source_dir with optional top-level default):
source_dir: /media/dvd-backups/disc1   # default for files without source_dir

files:
  - mkv: episode1.mkv
    output: episode1.mkvdup
    # uses top-level source_dir

  - mkv: movie.mkv
    output: movie.mkvdup
    source_dir: /media/dvd-backups/disc2   # per-file override
```

**Manifest fields:**

| Field | Required | Description |
|-------|----------|-------------|
| `source_dir` | No* | Default source directory for files that don't specify their own |
| `files` | Yes | List of MKV files to process (at least one) |
| `files[].mkv` | Yes | Path to the MKV file |
| `files[].output` | Yes | Output `.mkvdup` file |
| `files[].source_dir` | No* | Source directory for this file (overrides top-level default) |
| `files[].name` | No | Display name in FUSE mount (default: basename of mkv; `.mkv` auto-added if missing) |

\* At least one of top-level `source_dir` or per-file `source_dir` must be set for every file entry.

Relative paths are resolved against the manifest file's directory.

**Partial failure handling:**
- If one file fails, processing continues for the remaining files
- If indexing fails for one source, all files in that group are marked as failed and processing continues with the next source
- A summary at the end shows OK/FAIL status for each file
- Exit code is 1 if any file failed, 0 if all succeeded

### mount

Mount virtual filesystem from config files.

```bash
# Mount from config file(s)
mkvdup mount /mnt/videos config1.mkvdup.yaml config2.mkvdup.yaml

# Mount from a config with includes
mkvdup mount /mnt/videos /etc/mkvdup/mount.yaml

# Mount from config directory
mkvdup mount --config-dir /mnt/videos /etc/mkvdup.d/

# Mount with custom default permissions
mkvdup mount --default-uid 1000 --default-gid 1000 /mnt/videos config.yaml

# Mount with allow-other (for other users to access)
mkvdup mount --allow-other /mnt/videos config.yaml

# Run in foreground (for debugging or systemd)
mkvdup mount --foreground /mnt/videos config.yaml
```

Config files support `includes` (glob patterns referencing other configs, including
`**` recursive globs) and `virtual_files` (inline file definitions). See
[FUSE Configuration](FUSE.md#config-files-with-includes) for details.

**Options:**

| Option | Description |
|--------|-------------|
| `--allow-other` | Allow other users to access the mount (requires `/etc/fuse.conf` setting) |
| `--foreground`, `-f` | Run in foreground (don't daemonize) |
| `--config-dir` | Treat config argument as directory of YAML files (`.yaml`, `.yml`) |
| `--pid-file PATH` | Write daemon PID to file |
| `--daemon-timeout DUR` | Timeout waiting for daemon startup (default: `30s`) |

**Permission Options:**

| Option | Description |
|--------|-------------|
| `--default-uid UID` | Default UID for files and directories (default: calling user's UID) |
| `--default-gid GID` | Default GID for files and directories (default: calling user's GID) |
| `--default-file-mode MODE` | Default mode for files, in octal (default: `0444`) |
| `--default-dir-mode MODE` | Default mode for directories, in octal (default: `0555`) |
| `--permissions-file PATH` | Explicit path to permissions file |

**Source Watch Options:**

| Option | Description |
|--------|-------------|
| `--no-source-watch` | Disable source file watching |
| `--on-source-change ACTION` | Action on source change: `warn`, `disable`, `checksum` (default: `checksum`) |
| `--source-watch-poll-interval DUR` | Polling interval for network FS (default: `60s`) |
| `--source-read-timeout DUR` | Timeout for source file reads on network FS (default: `30s`) |

Error notification on source integrity issues is configured via `on_error_command` in a YAML config file rather than CLI flags. See [Error Notification](FUSE.md#error-notification) for details.

**Permissions file search order:**
1. `--permissions-file PATH` (if specified)
2. `~/.config/mkvdup/permissions.yaml` (if exists)
3. `/etc/mkvdup/permissions.yaml` (if exists)

New permissions are written to:
- `~/.config/mkvdup/permissions.yaml` (for non-root users)
- `/etc/mkvdup/permissions.yaml` (when running as root, unless `~/.config/mkvdup/permissions.yaml` exists)

### verify

Verify an existing dedup file against the original MKV.

```bash
mkvdup verify <dedup-file> <source-dir> <original-mkv>

# Example:
mkvdup verify movie.mkvdup /media/dvd-backups original.mkv
```

### check

Check integrity of a dedup file and its source files without requiring the original MKV.

```bash
mkvdup check <dedup-file> <source-dir>
mkvdup check --source-checksums <dedup-file> <source-dir>
```

**Arguments:**
- `<dedup-file>` — Path to the .mkvdup file
- `<source-dir>` — Directory containing the source media

**Options:**

| Option | Description |
|--------|-------------|
| `--source-checksums` | Verify source file checksums (reads entire source files) |

**Checks performed:**
1. Dedup file integrity: index and delta internal checksums
2. Source file existence: all referenced source files must be present
3. Source file sizes: actual sizes must match expected sizes
4. Source file checksums (`--source-checksums` only): xxhash verification of source file contents

**Use case:** After archiving, verify that your dedup files and source media are intact without needing the original MKV files. This sits between `validate` (config-level checks) and `verify` (full byte-for-byte reconstruction requiring the original MKV).

**Examples:**

```bash
# Quick check (dedup integrity + source existence/sizes)
mkvdup check movie.mkvdup /media/dvd-backups

# Full check including source file checksums
mkvdup check --source-checksums movie.mkvdup /media/dvd-backups
```

### stats

Show space savings and file statistics for mkvdup-managed files.

```bash
mkvdup stats <config.yaml...>
mkvdup stats --config-dir /etc/mkvdup.d/
```

**Arguments:**
- `<config.yaml...>` — YAML config files (same format as mount/validate)

**Options:**

| Option | Description |
|--------|-------------|
| `--config-dir` | Treat config argument as directory of YAML files (`.yaml`, `.yml`) |

**Output:**

Per-file statistics:
- Original MKV size
- Dedup file size on disk
- Space savings (absolute and percentage)
- Source type (DVD or Blu-ray)
- Source directory
- Source file count
- Index entry count

When multiple files are present, a rollup summary shows totals across all files including the number of unique source directories.

**Examples:**

```bash
# Stats for a single config
mkvdup stats movie.yaml

# Stats for all configs in a directory
mkvdup stats --config-dir /etc/mkvdup.d/

# Stats for multiple configs
mkvdup stats movie1.yaml movie2.yaml
```

### validate

Validate configuration files for correctness before mounting.

```bash
mkvdup validate [config.yaml...]
mkvdup validate --config-dir /etc/mkvdup.d/
mkvdup validate --deep config.yaml
mkvdup validate --strict config1.yaml config2.yaml
```

**Arguments:**
- `<config.yaml...>` — YAML config files to validate

**Options:**

| Option | Description |
|--------|-------------|
| `--config-dir` | Treat config argument as directory of YAML files (`.yaml`, `.yml`) |
| `--deep` | Verify dedup file headers and internal checksums |
| `--strict` | Treat warnings as errors (exit code 1 on warnings) |

**Validations performed:**
1. YAML syntax and required fields (`name`, `dedup_file`, `source_dir`)
2. Include resolution (glob patterns, cycle detection)
3. Path existence: dedup file exists, source directory exists and is a directory
4. Dedup file header: magic number, version, source file metadata
5. Name validation: rejects `..` components and empty names
6. Duplicate detection: warns on duplicate virtual file names across configs
7. Conflict detection: warns when a file name conflicts with a directory path
8. Deep checksums (`--deep` only): verifies index and delta integrity checksums

**Exit codes:**
- `0` — All valid (warnings are OK unless `--strict`)
- `1` — Errors found, or warnings with `--strict`

**Examples:**

```bash
# Validate a single config
mkvdup validate movie.mkvdup.yaml

# Validate all configs in a directory
mkvdup validate --config-dir /etc/mkvdup.d/

# Full integrity check
mkvdup validate --deep /etc/mkvdup.conf

# CI/pre-mount check (warnings = failure)
mkvdup validate --strict /etc/mkvdup.conf
```

### info

Show information about a dedup file.

```bash
mkvdup info [options] <dedup-file>

# Example:
mkvdup info movie.mkvdup
mkvdup info --hide-unused-files movie.mkvdup
```

| Option | Description |
|--------|-------------|
| `--hide-unused-files` | Hide source files not referenced by any index entry |

Source files are listed with their sizes. For V7/V8 dedup files, unused source files are marked `(unused)`. Use `--hide-unused-files` to omit them entirely.

### extract

Rebuild the original MKV from a dedup file and source media.

```bash
mkvdup extract <dedup-file> <source-dir> <output-mkv>
```

| Argument | Description |
|----------|-------------|
| `<dedup-file>` | Path to the `.mkvdup` file |
| `<source-dir>` | Directory containing the source media |
| `<output-mkv>` | Path for the reconstructed MKV file |

Example:

```bash
mkvdup extract movie.mkvdup /media/dvd-backups restored-movie.mkv
```

### probe

Quick test if MKV file(s) likely match source(s). Useful for multi-disc sets.

```bash
# Single MKV against multiple sources (backward compatible)
mkvdup probe /path/to/video.mkv /path/to/source1 /path/to/source2

# Multiple MKVs against multiple sources (use -- separator)
mkvdup probe ep1.mkv ep2.mkv ep3.mkv -- /path/to/disc1 /path/to/disc2
```

When multiple MKVs are provided, each source is indexed only once and all MKV hash sets are checked against it. This makes batch probing (e.g., 26 TV episodes against 5 disc sources) dramatically faster than probing each MKV individually.

**Use case:** You have 5 ISOs from a multi-disc set and 20 MKV files. Rather than trying each combination with full dedup (which takes minutes per attempt), probe can test all combinations in a single pass.

**Algorithm:**
1. Parse each MKV file and sample 20 packets per MKV (5 from first 10%, 10 from middle 80%, 5 from last 10%)
2. For each source: index once, then check all MKV hash sets against it
3. Report match percentages per MKV, sorted by likelihood

**Output interpretation:**
- 80-100% match: Very likely the correct source
- 40-80% match: Possible match (may be partial content or different encode settings)
- <40% match: Unlikely to be the source

### reload

Reload a running daemon's configuration by validating the config and sending SIGHUP.

```bash
mkvdup reload --pid-file /run/mkvdup.pid config.yaml
mkvdup reload --pid-file /run/mkvdup.pid --config-dir /etc/mkvdup.d/
mkvdup reload --pid-file /run/mkvdup.pid
mkvdup reload --pid $(pidof mkvdup)
```

**Required (one of):**

| Option | Description |
|--------|-------------|
| `--pid-file PATH` | PID file of the running daemon (must match mount's `--pid-file`) |
| `--pid PID` | PID of the running daemon (e.g., for foreground mode) |

**Options:**

| Option | Description |
|--------|-------------|
| `--config-dir` | Treat config argument as directory of YAML files (`.yaml`, `.yml`) |

If config files are provided, they are validated before sending the signal. If validation fails, the signal is not sent and errors are reported. If no config files are specified, the signal is sent without pre-validation (the daemon validates internally on SIGHUP).

The daemon re-reads its original config paths on SIGHUP, expanding include globs and `--config-dir` directories to pick up new files. See [Hot Reload](FUSE.md#hot-reload-via-sighup) for daemon-side behavior.

**systemd integration:**
```ini
[Service]
ExecStart=/usr/bin/mkvdup mount --foreground --pid-file /run/mkvdup.pid /mnt/videos /etc/mkvdup.conf
ExecReload=/usr/bin/mkvdup reload --pid-file /run/mkvdup.pid /etc/mkvdup.conf
```

### deltadiag

Analyze unmatched (delta) regions in a dedup file by cross-referencing with the original MKV to classify what stream type each delta region belongs to.

```bash
mkvdup deltadiag <dedup-file> <mkv-file>

# Example:
mkvdup deltadiag movie.mkvdup movie.mkv
```

**Arguments:**
- `<dedup-file>` -- Path to the .mkvdup file
- `<mkv-file>` -- Path to the original MKV file

**Output includes:**
- Total delta breakdown by stream type (video, audio, container)
- H.264 NAL type breakdown for video delta (SPS, PPS, SEI, slices, etc.)
- Slice NAL size distribution (small vs large)
- Summary with percentages of original file size

**Use case:** After creating a dedup file, use deltadiag to understand where the unmatched bytes are. This helps identify matching issues (e.g., audio streams that should be matching but aren't) and validate that improvements to the matching algorithm are working.

### Debug Commands

```bash
# Parse and display MKV structure
mkvdup parse-mkv /path/to/video.mkv

# Index a source directory
mkvdup index-source /path/to/source_dir

# Match packets (debugging)
mkvdup match video.mkv /path/to/source_dir
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error (invalid arguments, file not found, verification failed, etc.) |

## Progress Output

Commands that perform long-running operations (`create`, `batch-create`, `verify`, `extract`) display progress bars with ETA estimates:

```
Phase 3/6: Parsing MKV file...
  [████████████████████░░░░░░░░░░░░░░░░░░░░]  52%  2.3 GB / 4.5 GB  ETA: 00:00:14
```

After completion, the bar is replaced with:
```
Phase 3/6: Parsing MKV file... done (00:00:27)
```

Fast phases (codec checks, checksum calculations) display status lines without progress bars.

**`batch-create`** indexes each source once, then shows per-file progress:
```
Indexing source directory...
  [████████████████████████████████████████░░]  95%  4.2 GB / 4.5 GB  ETA: 00:00:01
Indexing source directory... done (00:00:28)

[1/3] episode1.mkv
Phase 1/4: Parsing MKV file...
```

When multiple source directories are used, source group headers are shown:
```
--- Source 1/2: /media/dvd-backups/disc1 (2 files) ---

Indexing source 1/2...
```

Progress bars are automatically disabled when stdout is not a terminal. When disabled, milestone percentages are printed at 10% intervals:
```
Phase 3/6: Parsing MKV file...
  10% (00:00:03)
  20% (00:00:06)
  ...
Phase 3/6: Parsing MKV file... done (00:00:27)
```

Use `--no-progress` to disable progress bars manually (phase labels and completion times are still shown). Use `--quiet` to suppress all informational output on stdout. Use `--log-file PATH` to capture output to a file (always uses non-TTY milestone style, written even when `--quiet` is set).

## Warning Threshold

If space savings fall below the warning threshold (default: 75%), a warning is shown but the file is still created:

```
WARNING: Space savings (28.4%) below 75%
  This may indicate wrong source, transcoded MKV, or very small MKV file.
```

Small MKV files (under ~200 MB) may naturally show lower space savings because the fixed overhead of the dedup file format (offset tables, MKV container headers, chapter data) becomes a larger proportion of the total file size. This is expected and does not indicate a matching problem.

Use `--warn-threshold N` to customize the percentage. Use `--quiet` to suppress all informational output including warnings.

```bash
# Lower the threshold to 50%
mkvdup create --warn-threshold 50 movie.mkv /media/dvd-backups
```

## Statistics Output

The `create` command shows detailed statistics:

```
=== Results ===
MKV file size:      3,420,000,000 bytes (3261.19 MB)
Matched bytes:      3,418,000,000 bytes (3259.28 MB, 99.9%)
Delta (unmatched):  1,500,000 bytes (1.43 MB, 0.0%)

Dedup file size:    52,700,000 bytes (50.24 MB)
Space savings:      98.5%

Packets matched:    2,139,988 / 2,139,988 (100.0%)
Index entries:      2,139,988
```

## Related Documentation

- [FUSE Configuration](FUSE.md) - Mount configuration and daemon options
- [File Format](FILE_FORMAT.md) - Binary format of .mkvdup files
