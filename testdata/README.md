# Test Data Setup

This project uses Big Buck Bunny DVD ISO for integration testing. The test data is NOT stored in the repository due to size (~7.5GB).

## Quick Start (Recommended)

Use the provided script to download and generate test data:

```bash
cd testdata
./generate-test-data.sh
```

This will:
1. Download the Big Buck Bunny PAL DVD ISO (~7.5GB) from Internet Archive
2. Create an MKV using ffmpeg (lossless remux, no transcoding)
3. Store files in `testdata/generated/` (git-ignored)

**Requirements:**
- `wget` - for downloading
- `ffmpeg` - for creating the MKV (with libdvdread for best results)
- `sudo` access - only needed if ffmpeg lacks libdvdread support

## Test Data Locations

Test data is searched in this order:

1. `$MKVDUP_TESTDATA` environment variable (if set)
2. `testdata/generated/` (created by `generate-test-data.sh`)
3. `~/.cache/mkvdup/testdata/`
4. `/tmp/mkvdup-testdata/` (for CI/ephemeral environments)

## Directory Structure

After running `generate-test-data.sh`:

```
testdata/generated/
├── bigbuckbunny/
│   └── bbb-pal.iso          # Big Buck Bunny PAL DVD (7.5GB)
└── bigbuckbunny-mkv/
    └── bigbuckbunny.mkv     # MKV created via ffmpeg
```

## Manual Setup

If you prefer manual setup or need to customize:

### 1. Download the ISO

```bash
mkdir -p ~/.cache/mkvdup/testdata/bigbuckbunny
cd ~/.cache/mkvdup/testdata/bigbuckbunny

# Download PAL version (7.5GB)
wget "https://archive.org/download/BigBuckBunny/big_buck_bunny_pal_dvd.iso" -O bbb-pal.iso

# Verify checksum
echo "cb67e9bc8e97b9d625e7cd7ee0d85e08  bbb-pal.iso" | md5sum -c
```

Alternative NTSC version:
```bash
wget "https://archive.org/download/BigBuckBunny/big_buck_bunny_ntsc_dvd.iso" -O bbb-ntsc.iso
# MD5: 966758b02da2c5c183ab7de2e0a5e96b
```

### 2. Create MKV from ISO

**Using ffmpeg dvd:// protocol (recommended, no sudo required):**

```bash
mkdir -p ~/.cache/mkvdup/testdata/bigbuckbunny-mkv

# Requires ffmpeg compiled with libdvdread
ffmpeg -i "dvd://bbb-pal.iso" \
    -map 0:v -map 0:a -c copy \
    ~/.cache/mkvdup/testdata/bigbuckbunny-mkv/bigbuckbunny.mkv
```

**Using ffmpeg with mount (fallback if libdvdread unavailable):**

```bash
mkdir -p ~/.cache/mkvdup/testdata/bigbuckbunny-mkv

# Mount the ISO
sudo mkdir -p /mnt/bbb
sudo mount -o loop,ro bbb-pal.iso /mnt/bbb

# Remux VOBs to MKV (lossless, no transcoding)
ffmpeg -i "concat:/mnt/bbb/VIDEO_TS/VTS_01_1.VOB|/mnt/bbb/VIDEO_TS/VTS_01_2.VOB|/mnt/bbb/VIDEO_TS/VTS_01_3.VOB" \
    -map 0:v -map 0:a -c copy \
    ~/.cache/mkvdup/testdata/bigbuckbunny-mkv/bigbuckbunny.mkv

# Unmount
sudo umount /mnt/bbb
```

### 3. Verify Setup

```bash
go test -v ./testdata -run TestDataAvailable
```

## Running Integration Tests

Integration tests automatically skip if test data is not available:

```bash
# Run all tests (skips integration tests if no test data)
go test ./...

# Run with specific test data path
MKVDUP_TESTDATA=/path/to/testdata go test ./...
```

## Performance Benchmarks

The performance stats in DESIGN.md and docs/MATCHING.md are measured using this test data:

- **Source:** Big Buck Bunny PAL DVD ISO (bbb-pal.iso)
- **MKV:** Created via ffmpeg lossless remux
- **Size:** ~560MB MKV from ~7.5GB ISO

## License

Big Buck Bunny is licensed under Creative Commons Attribution 3.0.
See: https://peach.blender.org/
