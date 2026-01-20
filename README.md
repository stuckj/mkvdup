# mkvdup

A storage deduplication tool for MKV files and their source media (DVD ISOs, Blu-ray backups).

## Overview

mkvdup reduces storage requirements for MKV files by referencing their source media. Since the underlying codec data (video frames, audio packets) is identical between an MKV and its source—just at different offsets with different container framing—we can store only the unique MKV data plus an index mapping MKV offsets to source offsets.

**Example:** A 3.4GB MKV can be stored as ~50MB by referencing the source ISO.

## Legal Notice

This tool is intended for personal backup and archival of legally owned media. It does not perform any copy protection circumvention.

## Features

- **DVD support** - Works with ISO files containing VOB (MPEG-PS) content
- **Blu-ray support** - Works with BDMV directory structures (planned)
- **FUSE filesystem** - Mount deduplicated files and access them transparently
- **Verification** - Byte-for-byte verification of reconstructed files

## Installation

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

### From Source

```bash
go install github.com/stuckj/mkvdup/cmd/mkvdup@latest
```

## Usage

### Create a deduplicated file

```bash
mkvdup create -m video.mkv -s /path/to/source/dir -o video.mkvdup
```

### Mount deduplicated files

```bash
mkvdup mount config.yaml /mnt/videos
```

### Verify reconstruction

```bash
mkvdup verify video.mkvdup -s /path/to/source/dir -m original.mkv
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

- [DESIGN.md](DESIGN.md) - Architecture overview and technical decisions
- [docs/MATCHING.md](docs/MATCHING.md) - Matching algorithms and ES-aware indexing
- [docs/FILE_FORMAT.md](docs/FILE_FORMAT.md) - Binary specification for .mkvdup files
- [docs/FUSE.md](docs/FUSE.md) - FUSE filesystem configuration
- [docs/CLI.md](docs/CLI.md) - Command-line interface reference
- [CONTRIBUTING.md](CONTRIBUTING.md) - Development guidelines

## License

MIT
