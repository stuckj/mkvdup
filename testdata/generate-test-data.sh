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
ISO_URL="https://archive.org/download/BigBuckBunny/big-buck-bunny-PAL.iso"
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
                if [[ $# -lt 2 || "$2" == -* ]]; then
                    echo "Error: --output-dir requires a directory argument." >&2
                    echo "Usage: $0 [--output-dir DIR]" >&2
                    exit 1
                fi
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

# Create MKV from ISO using ffmpeg dvd:// protocol (no sudo required)
create_mkv_dvd_protocol() {
    local iso_path="$1"
    local mkv_path="$2"

    info "Trying ffmpeg dvd:// protocol (no sudo required)..."

    # The dvd:// protocol requires libdvdread support in ffmpeg
    # Title 1 is typically the main feature
    if ffmpeg -y -i "dvd://${iso_path}" \
        -map 0:v -map 0:a \
        -c copy \
        -f matroska \
        "$mkv_path" 2>/dev/null; then
        return 0
    fi

    return 1
}

# Create MKV from ISO by mounting (requires sudo)
create_mkv_mount() {
    local iso_path="$1"
    local mkv_path="$2"

    info "Falling back to mount method (requires sudo)..."

    # Check for mount-specific dependencies
    for cmd in sudo mount umount; do
        if ! command -v "$cmd" &> /dev/null; then
            die "Missing required tool for mount fallback: $cmd"
        fi
    done

    # Create a temporary mount point
    local mount_point
    mount_point=$(mktemp -d)

    # Mount the ISO (requires sudo)
    if ! sudo mount -o loop,ro "$iso_path" "$mount_point"; then
        rmdir "$mount_point"
        die "Failed to mount ISO. Make sure you have sudo access and loop device support."
    fi

    # Install cleanup trap only after successful mount
    trap "sudo umount '$mount_point' 2>/dev/null || true; rmdir '$mount_point' 2>/dev/null || true" EXIT

    info "ISO mounted at $mount_point"

    # Find the main VOB files (VIDEO_TS)
    local vob_dir="${mount_point}/VIDEO_TS"
    if [[ ! -d "$vob_dir" ]]; then
        sudo umount "$mount_point"
        rmdir "$mount_point"
        trap - EXIT
        die "VIDEO_TS directory not found in ISO"
    fi

    # Find the title set with the largest total VOB size (main feature)
    # VTS_XX_0.VOB is menu data, so we skip it and look at VTS_XX_[1-9]*.VOB
    local best_vts=""
    local best_size=0
    shopt -s nullglob
    for vts_num in 01 02 03 04 05 06 07 08 09 10; do
        local vts_vobs=("${vob_dir}"/VTS_${vts_num}_[1-9].VOB "${vob_dir}"/VTS_${vts_num}_[1-9][0-9].VOB)
        if [[ ${#vts_vobs[@]} -gt 0 ]]; then
            local total_size=0
            for vob in "${vts_vobs[@]}"; do
                local size
                size=$(stat -c%s "$vob" 2>/dev/null || echo 0)
                total_size=$((total_size + size))
            done
            if [[ $total_size -gt $best_size ]]; then
                best_size=$total_size
                best_vts=$vts_num
            fi
        fi
    done
    shopt -u nullglob

    if [[ -z "$best_vts" ]]; then
        sudo umount "$mount_point"
        rmdir "$mount_point"
        trap - EXIT
        die "No VTS_XX_*.VOB files found"
    fi

    info "Using title set VTS_${best_vts} (largest content: $((best_size / 1024 / 1024))MB)"

    # Collect VOB files for the selected title set
    shopt -s nullglob
    local vob_array=("${vob_dir}"/VTS_${best_vts}_[1-9].VOB "${vob_dir}"/VTS_${best_vts}_[1-9][0-9].VOB)
    shopt -u nullglob

    # Sort VOB files naturally and join with '|' for ffmpeg concat
    local vob_files
    vob_files=$(printf '%s\n' "${vob_array[@]}" | sort -V | paste -sd'|' -)

    info "VOB files: $vob_files"

    # Use ffmpeg to remux - copy all streams without transcoding
    # -fflags +genpts generates timestamps for packets that lack them (common in VOBs)
    if ! ffmpeg -fflags +genpts -y -i "concat:${vob_files}" \
        -map 0:v -map 0:a \
        -c copy \
        -f matroska \
        "$mkv_path"; then
        sudo umount "$mount_point"
        rmdir "$mount_point"
        trap - EXIT
        die "ffmpeg remux failed"
    fi

    # Unmount
    sudo umount "$mount_point"
    rmdir "$mount_point"
    trap - EXIT
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

    info "Creating MKV from DVD structure using ffmpeg..."
    info "This performs a lossless remux (no transcoding)..."

    # Try dvd:// protocol first (no sudo required, needs libdvdread)
    if create_mkv_dvd_protocol "$iso_path" "$mkv_path"; then
        info "MKV created using dvd:// protocol: $mkv_path"
        info "Size: $(du -h "$mkv_path" | cut -f1)"
        return 0
    fi

    # Fall back to mounting
    create_mkv_mount "$iso_path" "$mkv_path"

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
