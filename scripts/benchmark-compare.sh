#!/bin/bash
# Compare current benchmarks against saved baseline.
# Usage:
#   ./scripts/benchmark-compare.sh        # Run and compare against baseline
#   ./scripts/benchmark-compare.sh save   # Save current results as baseline
#   ./scripts/benchmark-compare.sh help   # Show this help

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BASELINE_FILE="$PROJECT_ROOT/benchmarks/baseline.txt"

show_help() {
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  (none)    Run benchmarks and compare against baseline"
    echo "  save      Save current benchmark results as baseline"
    echo "  help      Show this help message"
    echo ""
    echo "This script runs Go benchmarks and compares against a saved baseline."
    echo "Install benchstat for detailed comparison: go install golang.org/x/perf/cmd/benchstat@latest"
}

run_benchmarks() {
    echo "Running benchmarks..."
    cd "$PROJECT_ROOT"
    go test -bench=. -benchmem -count=5 ./internal/dedup/... 2>&1
}

save_baseline() {
    echo "Saving baseline to $BASELINE_FILE..."
    mkdir -p "$(dirname "$BASELINE_FILE")"
    run_benchmarks > "$BASELINE_FILE"
    echo "Baseline saved."
}

compare_benchmarks() {
    if [ ! -f "$BASELINE_FILE" ]; then
        echo "No baseline found at $BASELINE_FILE"
        echo "Run '$0 save' first to create a baseline."
        exit 1
    fi

    CURRENT_FILE=$(mktemp)
    trap 'rm -f "$CURRENT_FILE"' EXIT

    run_benchmarks > "$CURRENT_FILE"

    echo ""
    echo "=========================================="
    echo "Current benchmark results:"
    echo "=========================================="
    cat "$CURRENT_FILE"

    if command -v benchstat &> /dev/null; then
        echo ""
        echo "=========================================="
        echo "Comparison with baseline (using benchstat):"
        echo "=========================================="
        benchstat "$BASELINE_FILE" "$CURRENT_FILE"
    else
        echo ""
        echo "=========================================="
        echo "Install benchstat for detailed comparison:"
        echo "  go install golang.org/x/perf/cmd/benchstat@latest"
        echo "=========================================="
    fi
}

case "${1:-}" in
    save)
        save_baseline
        ;;
    help|--help|-h)
        show_help
        ;;
    "")
        compare_benchmarks
        ;;
    *)
        echo "Unknown command: $1"
        show_help
        exit 1
        ;;
esac
