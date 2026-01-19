#!/bin/bash
#
# Generate test data for mkvdup integration tests.
#
# This script downloads the Big Buck Bunny DVD ISO and creates an MKV using ffmpeg.
# Using ffmpeg (not MakeMKV) ensures reproducible results across environments.
#
# Usage:
#   ./generate-test-data.sh [--output-dir DIR]
#
# Output structure:
#   <output-dir>/
#   ├── bigbuckbunny/
#   │   └── bbb-pal.iso          # Big Buck Bunny PAL DVD (7.5GB)
#   └── bigbuckbunny-mkv/
#       └── bigbuckbunny.mkv     # MKV extracted via ffmpeg
#

set -euo pipefail

# Default output directory is testdata/generated/ (relative to script location)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="${OUTPUT_DIR:-${SCRIPT_DIR}/generated}"

# Big Buck Bunny PAL DVD
ISO_URL="https://archive.org/download/BigBuckBunny/big_buck_bunny_pal_dvd.iso"
ISO_MD5="cb67e9bc8e97b9d625e7cd7ee0d85e08"
ISO_NAME="bbb-pal.iso"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
}

die() {
    error "$@"
    exit 1
}

# Check for required tools
check_dependencies() {
    local missing=()

    for cmd in wget md5sum ffmpeg; do
        if ! command -v "$cmd" &> /dev/null; then
            missing+=("$cmd")
        fi
    done

    if [[ ${#missing[@]} -gt 0 ]]; then
        die "Missing required tools: ${missing[*]}"
    fi
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --output-dir)
                OUTPUT_DIR="$2"
                shift 2
                ;;
            --help|-h)
                echo "Usage: $0 [--output-dir DIR]"
                echo ""
                echo "Downloads Big Buck Bunny DVD ISO and creates an MKV for testing."
                echo ""
                echo "Options:"
                echo "  --output-dir DIR    Output directory (default: testdata/generated/)"
                echo "  --help, -h          Show this help message"
                exit 0
                ;;
            *)
                die "Unknown option: $1"
                ;;
        esac
    done
}

# Download and verify the ISO
download_iso() {
    local iso_dir="${OUTPUT_DIR}/bigbuckbunny"
    local iso_path="${iso_dir}/${ISO_NAME}"

    mkdir -p "$iso_dir"

    if [[ -f "$iso_path" ]]; then
        info "ISO already exists, verifying checksum..."
        local actual_md5
        actual_md5=$(md5sum "$iso_path" | awk '{print $1}')
        if [[ "$actual_md5" == "$ISO_MD5" ]]; then
            info "ISO checksum verified: $iso_path"
            return 0
        else
            warn "ISO checksum mismatch, re-downloading..."
            rm -f "$iso_path"
        fi
    fi

    info "Downloading Big Buck Bunny PAL DVD ISO (~7.5GB)..."
    info "URL: $ISO_URL"
    info "This may take a while depending on your connection..."

    wget --progress=bar:force:noscroll -O "$iso_path" "$ISO_URL" || die "Download failed"

    info "Verifying checksum..."
    local actual_md5
    actual_md5=$(md5sum "$iso_path" | awk '{print $1}')
    if [[ "$actual_md5" != "$ISO_MD5" ]]; then
        die "Checksum mismatch! Expected: $ISO_MD5, Got: $actual_md5"
    fi

    info "ISO downloaded and verified: $iso_path"
}

# Create MKV from ISO using ffmpeg
create_mkv() {
    local iso_path="${OUTPUT_DIR}/bigbuckbunny/${ISO_NAME}"
    local mkv_dir="${OUTPUT_DIR}/bigbuckbunny-mkv"
    local mkv_path="${mkv_dir}/bigbuckbunny.mkv"

    mkdir -p "$mkv_dir"

    if [[ -f "$mkv_path" ]]; then
        info "MKV already exists: $mkv_path"
        return 0
    fi

    info "Mounting ISO to extract content..."

    # Create a temporary mount point
    local mount_point
    mount_point=$(mktemp -d)
    trap "sudo umount '$mount_point' 2>/dev/null || true; rmdir '$mount_point' 2>/dev/null || true" EXIT

    # Mount the ISO (requires sudo)
    if ! sudo mount -o loop,ro "$iso_path" "$mount_point"; then
        die "Failed to mount ISO. Make sure you have sudo access and loop device support."
    fi

    info "ISO mounted at $mount_point"

    # Find the main VOB files (VIDEO_TS)
    local vob_dir="${mount_point}/VIDEO_TS"
    if [[ ! -d "$vob_dir" ]]; then
        die "VIDEO_TS directory not found in ISO"
    fi

    # Big Buck Bunny main title is typically VTS_01_*.VOB
    # We'll use ffmpeg to concatenate and remux the VOBs
    info "Creating MKV from DVD structure using ffmpeg..."
    info "This performs a lossless remux (no transcoding)..."

    # Use ffmpeg with the dvd protocol to read the title directly
    # The concat demuxer with VOB files is the most reliable approach
    local vob_files
    vob_files=$(ls -1 "${vob_dir}"/VTS_01_[1-9].VOB 2>/dev/null | sort -V | tr '\n' '|' | sed 's/|$//')

    if [[ -z "$vob_files" ]]; then
        die "No VTS_01_*.VOB files found"
    fi

    info "VOB files: $vob_files"

    # Use ffmpeg to remux - copy all streams without transcoding
    ffmpeg -y -i "concat:${vob_files}" \
        -map 0:v -map 0:a \
        -c copy \
        -f matroska \
        "$mkv_path" || die "ffmpeg remux failed"

    # Unmount
    sudo umount "$mount_point"
    rmdir "$mount_point"
    trap - EXIT

    info "MKV created: $mkv_path"
    info "Size: $(du -h "$mkv_path" | cut -f1)"
}

# Main
main() {
    parse_args "$@"
    check_dependencies

    info "Output directory: $OUTPUT_DIR"
    mkdir -p "$OUTPUT_DIR"

    download_iso
    create_mkv

    echo ""
    info "Test data generation complete!"
    info ""
    info "Files created:"
    info "  ISO: ${OUTPUT_DIR}/bigbuckbunny/${ISO_NAME}"
    info "  MKV: ${OUTPUT_DIR}/bigbuckbunny-mkv/bigbuckbunny.mkv"
    info ""
    info "To run integration tests:"
    info "  go test -v ./..."
}

main "$@"
