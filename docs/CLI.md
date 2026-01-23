# CLI Commands

*Command-line interface for mkvdup.*

[Back to Architecture Overview](../DESIGN.md)

## Global Options

```bash
# Enable verbose/debug output for any command
mkvdup -v <command> [args...]
mkvdup --verbose <command> [args...]

# Examples:
mkvdup -v create video.mkv /source/dir
mkvdup -v mount /mnt/media config1.yaml config2.yaml
mkvdup -v verify video.mkvdup /source/dir video.mkv
```

**Verbose mode enables:**
- FUSE operation logging (Open, Read, Lookup, Readdir)
- Detailed verification output with byte comparisons
- Debug information for troubleshooting

## Commands

### create

Create a dedup file from an MKV and its source directory.

```bash
# Basic usage
mkvdup create \
    --mkv /path/to/video.mkv \
    --source /path/to/source_dir \
    --output /path/to/video.mkvdup \
    --name "Videos/video.mkv"  # Virtual path in FUSE mount

# With automatic deletion of original after verification
mkvdup create \
    --mkv /path/to/video.mkv \
    --source /path/to/source_dir \
    --output /path/to/video.mkvdup \
    --name "Videos/video.mkv" \
    --delete-original

# Custom warning threshold (default 75%)
mkvdup create \
    --mkv /path/to/video.mkv \
    --source /path/to/source_dir \
    --output /path/to/video.mkvdup \
    --warn-threshold 80  # Warn if space savings < 80%
```

**Outputs:**
- `video.mkvdup` - The dedup data file (index + delta)
- `video.mkvdup.yaml` - Config file for this mapping

**Directory paths in `--name`:**
The `--name` option supports directory paths (e.g., `"Movies/Action/Video1.mkv"`). Each `create` command produces one `.mkvdup` file with one name stored in its config. The directory structure becomes visible when mounting multiple configs together—directories are auto-created from path components across all mounted files. See [FUSE Directory Structure](FUSE.md#directory-structure) for details.

### mount

Mount virtual filesystem from config files.

```bash
# Mount from config file(s)
mkvdup mount /mnt/videos config1.mkvdup.yaml config2.mkvdup.yaml

# Mount from config directory
mkvdup mount --config-dir /mnt/videos /etc/mkvdup.d/

# Mount with custom default permissions
mkvdup mount --default-uid 1000 --default-gid 1000 /mnt/videos config.yaml

# Mount with allow-other (for other users to access)
mkvdup mount --allow-other /mnt/videos config.yaml

# Run in foreground (for debugging or systemd)
mkvdup mount --foreground /mnt/videos config.yaml
```

**Options:**

| Option | Description |
|--------|-------------|
| `--allow-other` | Allow other users to access the mount (requires `/etc/fuse.conf` setting) |
| `--foreground`, `-f` | Run in foreground (don't daemonize) |
| `--config-dir` | Treat config argument as directory of `.yaml` files |
| `--pid-file PATH` | Write daemon PID to file |
| `--daemon-timeout DUR` | Timeout waiting for daemon startup (default: `30s`) |

**Permission Options:**

| Option | Description |
|--------|-------------|
| `--default-uid UID` | Default UID for files and directories (default: `0`) |
| `--default-gid GID` | Default GID for files and directories (default: `0`) |
| `--default-file-mode MODE` | Default mode for files, in octal (default: `0444`) |
| `--default-dir-mode MODE` | Default mode for directories, in octal (default: `0555`) |
| `--permissions-file PATH` | Explicit path to permissions file |

**Permissions file search order:**
1. `--permissions-file PATH` (if specified)
2. `~/.config/mkvdup/permissions.yaml` (if exists)
3. `/etc/mkvdup/permissions.yaml` (if exists)

New permissions are written to:
- `~/.config/mkvdup/permissions.yaml` (for non-root users)
- `/etc/mkvdup/permissions.yaml` (when running as root)

### verify

Verify an existing dedup file against the original MKV.

```bash
mkvdup verify \
    --dedup /path/to/video.mkvdup \
    --source /path/to/source_dir \
    --original /path/to/video.mkv
```

### info

Show information about a dedup file.

```bash
mkvdup info --dedup /path/to/video.mkvdup
```

### extract

Rebuild/extract original MKV from dedup + source.

```bash
mkvdup extract \
    --dedup /path/to/video.mkvdup \
    --source /path/to/source_dir \
    --output /path/to/restored.mkv
```

### probe

Quick test if an MKV likely matches a source. Useful for multi-disc sets.

```bash
mkvdup probe /path/to/video.mkv /path/to/source1 /path/to/source2 ...

# Example output:
#   Probing video.mkv against 3 sources...
#   Sampling 20 packets from MKV...
#
#   Results:
#     /data/disc1  18/20 matches (90%) ← likely match
#     /data/disc2   2/20 matches (10%)
#     /data/disc3   0/20 matches (0%)
```

**Use case:** You have 5 ISOs from a multi-disc set and 20 MKV files. Rather than trying each combination with full dedup (which takes minutes per attempt), probe can test all combinations in under a minute.

**Algorithm:**
1. Parse MKV file (quick scan, not full parse)
2. Sample 20 packets from different positions (5 from first 10%, 10 from middle 80%, 5 from last 10%)
3. For each source: look up each sampled packet hash, count matches
4. Report match percentages, sorted by likelihood

**Output interpretation:**
- 80-100% match: Very likely the correct source
- 40-80% match: Possible match (may be partial content or different encode settings)
- <40% match: Unlikely to be the source

### reload

Reload running daemon's config.

```bash
mkvdup reload  # Sends SIGHUP to running daemon
```

### Debug Commands

```bash
# Parse and display MKV structure
mkvdup parse-mkv /path/to/video.mkv

# Index a source directory
mkvdup index-source /path/to/source_dir

# Match packets (debugging)
mkvdup match --mkv video.mkv --source /path/to/source_dir
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success (includes success with low space savings warning) |
| 1 | General error |
| 2 | Verification failed |
| 3 | Source directory not found or invalid |
| 4 | MKV file not found or invalid |

## Warning Threshold

If space savings fall below threshold (default 75%), a warning is shown but the file is still created:

```
Deduplication Results:
  ┌─────────────────────────────────────────────────────────┐
  │  Original MKV size:          3.42 GB                    │
  │  Matched data (from source): 1.02 GB  (29.8%)           │
  │  Unique data (delta):        2.40 GB  (70.2%)           │
  │  Index overhead:             51.2 MB  (1.5%)            │
  │  ─────────────────────────────────────────────────────  │
  │  Dedup file size:            2.45 GB                    │
  │  Space savings:              970 MB   (28.4%)           │
  └─────────────────────────────────────────────────────────┘

  WARNING: Space savings (28.4%) below threshold (75%)
      This may indicate:
      - Wrong source directory (MKV not from this disc)
      - Source files modified after ripping
      - Transcoded MKV (not lossless remux)

      Use --warn-threshold to adjust the threshold, or
      --quiet to suppress this warning.
```

## Statistics Output

The `create` command shows detailed statistics:

```
Matched bytes:      3,418,000,000 bytes (3259.28 MB, 99.9%)
Unmatched bytes:    1,500,000 bytes (1.43 MB, 0.0%)
Matched packets:    2,139,988 / 2,139,988  (100.0%)

Output files:
  Dedup file:  /path/to/video.mkvdup (52.7 MB)
  Config file: /path/to/video.mkvdup.yaml
```

*Note: The matcher tracks video vs audio internally but currently reports combined statistics.*

## Related Documentation

- [FUSE Configuration](FUSE.md) - Mount configuration and daemon options
- [File Format](FILE_FORMAT.md) - Binary format of .mkvdup files
