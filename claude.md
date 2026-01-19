# Project Notes

## Development Environment

- Use `gvm` (Go Version Manager) for Go. Source it before running Go commands:
  ```bash
  source ~/.gvm/scripts/gvm && go build ...
  ```

## Before Committing

Run these checks before committing code:

```bash
source ~/.gvm/scripts/gvm

# Format all Go files
gofmt -w .

# Check for common issues
go vet ./...

# Run linter
golint ./...

# Run tests with race detection
go test -race ./...

# Build to verify compilation
go build ./...
```

## Project Overview

MKV-ISO deduplication system that matches MKV video files back to their source DVD/Blu-ray ISOs for storage deduplication.

## Key Technical Details

- DVDs use MPEG-PS (Program Stream) container format with PES packet framing
- MKV files contain raw ES (Elementary Stream) data
- Video matching requires ES-aware indexing that accounts for PES headers
- Private Stream 1 (0xBD) contains AC-3 audio (sub-streams 0x80-0x87) and subpictures
