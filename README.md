# mkvdup

[![Tests](https://github.com/stuckj/mkvdup/actions/workflows/coverage.yml/badge.svg)](https://github.com/stuckj/mkvdup/actions/workflows/coverage.yml)
[![Coverage](https://img.shields.io/badge/coverage-see%20report-blue)](https://stuckj.github.io/mkvdup/coverage/)

A storage deduplication tool for MKV files and their source media (DVD ISOs, Blu-ray backups).

## Overview

mkvdup reduces storage requirements for MKV files by referencing their source media. Since the underlying codec data (video frames, audio packets) is identical between an MKV and its source—just at different offsets with different container framing—we can store only the unique MKV data plus an index mapping MKV offsets to source offsets.

**Example:** A 3.4GB MKV can be stored as ~50MB by referencing the source ISO.

## Legal Notice

This tool is intended for personal backup and archival of legally owned media. It does not perform any copy protection circumvention.

## Features

- **DVD support** - Works with ISO files containing VOB (MPEG-PS) content
- **Blu-ray support** - Works with BDMV directory structures and Blu-ray ISO files
- **FUSE filesystem** - Mount deduplicated files and access them transparently
- **Permission customization** - `chmod`/`chown` support with persistent metadata storage
- **Verification** - Byte-for-byte verification of reconstructed files

## Installation

### macOS / Linux (Homebrew)

```bash
brew tap stuckj/mkvdup
brew install mkvdup
```

**Canary (pre-release):** `brew install mkvdup-canary` — installs alongside stable as `mkvdup-canary`

### Debian/Ubuntu (APT)

```bash
# Add the GPG key
curl -fsSL https://stuckj.github.io/mkvdup/gpg-key.asc | sudo gpg --dearmor -o /usr/share/keyrings/mkvdup.gpg

# Add the repository
echo "deb [signed-by=/usr/share/keyrings/mkvdup.gpg arch=amd64,arm64] https://stuckj.github.io/mkvdup/apt stable main" | sudo tee /etc/apt/sources.list.d/mkvdup.list

# Install
sudo apt update
sudo apt install mkvdup
```

<details>
<summary><strong>Canary (pre-release)</strong></summary>

```bash
# Add the GPG key (same as stable)
curl -fsSL https://stuckj.github.io/mkvdup/gpg-key.asc | sudo gpg --dearmor -o /usr/share/keyrings/mkvdup.gpg

# Add the canary repository
echo "deb [signed-by=/usr/share/keyrings/mkvdup.gpg arch=amd64,arm64] https://stuckj.github.io/mkvdup/apt canary main" | sudo tee /etc/apt/sources.list.d/mkvdup-canary.list

# Install
sudo apt update
sudo apt install mkvdup-canary
```

</details>

### RHEL/Fedora (DNF)

```bash
# Add the repository
sudo tee /etc/yum.repos.d/mkvdup.repo << 'EOF'
[mkvdup]
name=mkvdup
baseurl=https://stuckj.github.io/mkvdup/yum
enabled=1
gpgcheck=1
gpgkey=https://stuckj.github.io/mkvdup/yum/gpg-key.asc
EOF

# Install
sudo dnf install mkvdup
```

<details>
<summary><strong>Canary (pre-release)</strong></summary>

```bash
# Add the canary repository
sudo tee /etc/yum.repos.d/mkvdup-canary.repo << 'EOF'
[mkvdup-canary]
name=mkvdup-canary
baseurl=https://stuckj.github.io/mkvdup/yum-canary
enabled=1
gpgcheck=1
gpgkey=https://stuckj.github.io/mkvdup/yum-canary/gpg-key.asc
EOF

# Install
sudo dnf install mkvdup-canary
```

</details>

### From Source

```bash
go install github.com/stuckj/mkvdup/cmd/mkvdup@latest
```

**Canary:** `go install github.com/stuckj/mkvdup/cmd/mkvdup@v0.x.x-canary.N` (see [releases](https://github.com/stuckj/mkvdup/releases) for versions)

## Usage

### Create a deduplicated file

```bash
mkvdup create video.mkv /path/to/source/dir video.mkvdup
```

### Mount deduplicated files

```bash
mkvdup mount /mnt/videos config.yaml
```

### Organize with config includes

Each `.mkvdup` file gets a companion YAML config:

```yaml
# /data/dedup/video1.mkvdup.yaml
name: "Movies/Video 1.mkv"
dedup_file: video1.mkvdup
source_dir: /data/sources/Video1_DVD
```

A top-level config includes them all using glob patterns:

```yaml
# /etc/mkvdup.conf
includes:
  - "/data/dedup/**/*.mkvdup.yaml"
```

### Expand wildcard configs

The FUSE mount's file watcher doesn't detect new files added to directories
matched by wildcards. Use `expand-config` to generate an explicit file list
that the watcher can monitor:

```bash
# Create a wildcard config (source of truth)
cat > wildcard.yaml <<EOF
sources:
  - path: /data/dedup
    pattern: "**/*.mkvdup.yaml"
EOF

# Expand to explicit file list
mkvdup expand-config wildcard.yaml --output expanded.yaml

# Mount using the expanded config
mkvdup mount /mnt/videos expanded.yaml
```

When new `.mkvdup.yaml` files are added, re-run `expand-config` to regenerate the
explicit config. The mount detects the change and adds the new virtual files.
See [docs/CLI.md](docs/CLI.md#expand-config) for full details.

### Mount via fstab

```
/etc/mkvdup.conf  /mnt/videos  fuse.mkvdup  nofail  0  0
```

`nofail` lets the system boot normally if the mount fails (e.g., source media unavailable). The mount helper automatically enables `allow_other` so that non-root users can access the filesystem.

For a directory of config files instead of a single file:

```
/etc/mkvdup.d  /mnt/videos  fuse.mkvdup  config_dir,nofail  0  0
```

See [docs/FUSE.md](docs/FUSE.md) for full configuration details including source watching, error notifications, and permissions.

### Verify reconstruction

```bash
mkvdup verify video.mkvdup /path/to/source/dir original.mkv
```

### Show dedup file info

```bash
mkvdup info video.mkvdup
```

## How It Works

1. **Index the source** - Parse the DVD/Blu-ray container and build a hash index of codec packets
2. **Parse the MKV** - Extract codec data locations from the MKV file
3. **Match packets** - Find MKV codec data in the source using hash lookups
4. **Create dedup file** - Store the index mapping plus any MKV-only data (headers, chapters, etc.)
5. **Reconstruct on-demand** - FUSE filesystem stitches data from source files and the dedup file

## Documentation

See [docs/CLI.md](docs/CLI.md) for the full command-line reference.

- [DESIGN.md](DESIGN.md) - Architecture overview and technical decisions
- [docs/MATCHING.md](docs/MATCHING.md) - Matching algorithms and ES-aware indexing
- [docs/FILE_FORMAT.md](docs/FILE_FORMAT.md) - Binary specification for .mkvdup files
- [docs/FUSE.md](docs/FUSE.md) - FUSE filesystem configuration
- [docs/CLI.md](docs/CLI.md) - Command-line interface reference
- [CONTRIBUTING.md](CONTRIBUTING.md) - Development guidelines
- [Performance Benchmarks](https://stuckj.github.io/mkvdup/benchmarks/) - Historical performance tracking

## License

MIT
