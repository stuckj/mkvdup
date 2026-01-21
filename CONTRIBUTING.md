# Contributing

Guidelines for contributing to mkvdup.

**Important:** When making code changes, update relevant documentation (CLI commands, architecture, file formats, etc.) in the same PR.

## Development Environment

Use `gvm` (Go Version Manager) for Go. Source it before running Go commands:
```bash
source ~/.gvm/scripts/gvm && go build ...
```

## Pre-Commit Checklist

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

## Pull Requests

- All PRs are **squash merged** to maintain a clean, linear commit history
- Feature branches are deleted after merge
- Write clear PR titles that describe the change (these become commit messages)
- Include context in the PR description for complex changes

## Code Style

- Run `gofmt` or `goimports` on all code before committing
- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Use `golint` and `go vet` to catch common issues
- Keep functions focused and reasonably sized
- Use meaningful variable and function names

## Testing

- Write tests alongside implementation (test-driven when practical)
- Use table-driven tests for cases with multiple inputs
- Aim for high test coverage on critical paths (matching, file format, FUSE reads)
- Use `go test -race` to detect data races in concurrent code (also run in CI)
- Integration tests should use temporary directories and clean up after themselves

### Key Test Categories

**Unit tests:**
- Parser tests (EBML/MKV, source indexer, hash functions)
- Index tests (binary search, entry lookup, boundary conditions)
- Boundary expansion tests
- Sync pattern detection tests
- Config parsing tests
- Dedup file format tests (header, footer, roundtrip)

**Integration tests:**
- End-to-end: Create dedup → verify → compare SHA256
- FUSE playback: Mount, play in VLC/mpv, verify no artifacts
- Stress test: Multiple concurrent reads, random seeks
- Lazy loading tests (mmap on open, unmap on close)
- Graceful degradation tests (single file error doesn't affect others)

**Edge case tests:**
- Empty files, tiny files, corrupted sources, missing sources
- Deduplication quality validation (delta contains only container overhead)
- Boundary conditions (read at exact file end, spanning multiple entries)

## Error Handling

- Return errors rather than panicking (except for truly unrecoverable situations)
- Wrap errors with context using `fmt.Errorf("context: %w", err)`
- Log errors at appropriate levels (debug, info, warn, error)

## Documentation

- Document exported functions and types with godoc comments
- Keep comments current when code changes
- Complex algorithms should have explanatory comments

## Dependencies

- Prefer standard library when sufficient
- Vet third-party dependencies for maintenance status and security
- Use Go modules for dependency management
- Pin dependency versions in go.mod

## Performance

- Profile before optimizing (`go test -bench`, `pprof`)
- Avoid premature optimization
- Memory-map large files using `internal/mmap` rather than reading into memory. This is critical for ISO files (multi-GB) to avoid excessive memory copies. The mmap package provides zero-copy access via `unix.Mmap`.
- Use sync.Pool for frequently allocated buffers

### Benchmarks

Performance benchmarks track dedup reader operations. CI uses `benchstat` for statistically
significant regression detection (>10% slowdown with p<0.05).

**Run benchmarks locally:**
```bash
go test -bench=. -benchmem -count=5 ./internal/dedup/...
```

**Compare against baseline (optional, for local development):**
```bash
# Install benchstat
go install golang.org/x/perf/cmd/benchstat@latest

# Run and compare against repo baseline
./scripts/benchmark-compare.sh

# Run with regression check (exits non-zero on significant regression)
./scripts/benchmark-compare.sh check
```

**Benchmark results are tracked at:** [Benchmark Dashboard](https://stuckj.github.io/mkvdup/benchmarks/)

The baseline file (`benchmarks/baseline.txt`) is automatically updated by CI on merges to main.
Do not commit baseline changes manually - CI handles this to ensure consistent runner performance.

**Note:** The CI workflow runs `go vet` and `staticcheck` on all PRs.

## Key Technical Details

- DVDs use MPEG-PS (Program Stream) container format with PES packet framing
- MKV files contain raw ES (Elementary Stream) data
- Video matching requires ES-aware indexing that accounts for PES headers
- Private Stream 1 (0xBD) contains AC-3 audio (sub-streams 0x80-0x87) and subpictures

## Project Documentation

- [DESIGN.md](DESIGN.md) - Architecture overview
- [docs/MATCHING.md](docs/MATCHING.md) - Matching algorithms
- [docs/FILE_FORMAT.md](docs/FILE_FORMAT.md) - Binary file format
- [docs/FUSE.md](docs/FUSE.md) - FUSE filesystem configuration
- [docs/CLI.md](docs/CLI.md) - Command-line interface
