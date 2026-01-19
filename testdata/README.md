# Test Data Setup

This project uses Big Buck Bunny DVD ISO for integration testing. The test data is NOT stored in the repository due to size (~7.5GB).

## Test Data Location

Test data should be stored in one of these locations (checked in order):

1. `$MKVDUP_TESTDATA` environment variable (if set)
2. `~/.cache/mkvdup/testdata/`
3. `/tmp/mkvdup-testdata/` (for CI/ephemeral environments)

## Required Files

After setup, you should have:

```
~/.cache/mkvdup/testdata/
├── bigbuckbunny/
│   └── bbb-pal.iso          # Big Buck Bunny PAL DVD (7.5GB)
└── bigbuckbunny-mkv/
    └── title_t00.mkv        # MKV ripped from the ISO (you create this)
```

## Setup Instructions

### 1. Download the ISO

Download Big Buck Bunny PAL DVD ISO from Internet Archive:

```bash
mkdir -p ~/.cache/mkvdup/testdata/bigbuckbunny
cd ~/.cache/mkvdup/testdata/bigbuckbunny

# Download PAL version (7.5GB) - smaller than NTSC
wget "https://archive.org/download/BigBuckBunny/big_buck_bunny_pal_dvd.iso" -O bbb-pal.iso

# Verify checksum (MD5: cb67e9bc8e97b9d625e7cd7ee0d85e08)
md5sum bbb-pal.iso
```

Alternative: Use the NTSC version if preferred:
```bash
wget "https://archive.org/download/BigBuckBunny/big_buck_bunny_ntsc_dvd.iso" -O bbb-ntsc.iso
# MD5: 966758b02da2c5c183ab7de2e0a5e96b
```

### 2. Create MKV from ISO

Use MakeMKV or similar to rip the main title to MKV:

```bash
mkdir -p ~/.cache/mkvdup/testdata/bigbuckbunny-mkv

# Using MakeMKV CLI (if installed):
makemkvcon mkv iso:~/.cache/mkvdup/testdata/bigbuckbunny/bbb-pal.iso all \
    ~/.cache/mkvdup/testdata/bigbuckbunny-mkv/

# Or use the MakeMKV GUI to rip the main title
```

Important: Use **lossless remux** (no transcoding). The MKV must contain the original
DVD codec data for deduplication to work.

### 3. Verify Setup

Run the test data check:

```bash
go test -v ./testdata -run TestDataAvailable
```

Or manually check:

```bash
ls -la ~/.cache/mkvdup/testdata/bigbuckbunny/
ls -la ~/.cache/mkvdup/testdata/bigbuckbunny-mkv/
```

## Running Integration Tests

Integration tests that require the test data will be skipped automatically if it's not available:

```bash
# Run all tests (integration tests skip if no test data)
go test ./...

# Run with specific test data path
MKVDUP_TESTDATA=/path/to/testdata go test ./...
```

## License

Big Buck Bunny is licensed under Creative Commons Attribution 3.0.
See: https://peach.blender.org/
