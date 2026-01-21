#!/bin/bash
# Compare current benchmarks against saved baseline using benchstat.
# Usage:
#   ./scripts/benchmark-compare.sh        # Run and compare against baseline
#   ./scripts/benchmark-compare.sh save   # Save current results as baseline
#   ./scripts/benchmark-compare.sh check  # Run, compare, and exit non-zero on regression
#   ./scripts/benchmark-compare.sh help   # Show this help

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BASELINE_FILE="$PROJECT_ROOT/benchmarks/baseline.txt"
BENCH_COUNT=10  # Number of iterations for statistical significance

show_help() {
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  (none)    Run benchmarks and compare against baseline"
    echo "  save      Save current benchmark results as baseline"
    echo "  check     Run, compare, and exit non-zero if regression detected"
    echo "  help      Show this help message"
    echo ""
    echo "This script runs Go benchmarks and compares against a saved baseline."
    echo "Uses -count=$BENCH_COUNT for statistical significance with benchstat."
    echo ""
    echo "Install benchstat: go install golang.org/x/perf/cmd/benchstat@latest"
    echo ""
    echo "Environment variables:"
    echo "  BENCHMARK_REGRESSION_THRESHOLD  Regression threshold in % (default: 10)"
}

run_benchmarks() {
    echo "Running benchmarks (count=$BENCH_COUNT)..."
    cd "$PROJECT_ROOT"
    go test -bench=. -benchmem -count=$BENCH_COUNT ./internal/dedup/... 2>&1
}

save_baseline() {
    echo "Saving baseline to $BASELINE_FILE..."
    mkdir -p "$(dirname "$BASELINE_FILE")"
    run_benchmarks > "$BASELINE_FILE"
    echo "Baseline saved."
}

compare_benchmarks() {
    local check_mode="${1:-false}"

    if [ ! -f "$BASELINE_FILE" ]; then
        echo "No baseline found at $BASELINE_FILE"
        echo "Run '$0 save' first to create a baseline."
        exit 1
    fi

    CURRENT_FILE=$(mktemp)
    BENCHSTAT_FILE=$(mktemp)
    trap 'rm -f "$CURRENT_FILE" "$BENCHSTAT_FILE"' EXIT

    run_benchmarks > "$CURRENT_FILE"

    if ! command -v benchstat &> /dev/null; then
        echo ""
        echo "=========================================="
        echo "benchstat not found. Install it for comparison:"
        echo "  go install golang.org/x/perf/cmd/benchstat@latest"
        echo "=========================================="
        echo ""
        echo "Raw benchmark results:"
        cat "$CURRENT_FILE"
        exit 1
    fi

    echo ""
    echo "=========================================="
    echo "Comparison with baseline (benchstat):"
    echo "=========================================="
    benchstat "$BASELINE_FILE" "$CURRENT_FILE" | tee "$BENCHSTAT_FILE"

    if [ "$check_mode" = "true" ]; then
        echo ""
        echo "=========================================="
        echo "Checking for regressions..."
        echo "=========================================="
        if "$SCRIPT_DIR/check-benchstat.sh" "$BENCHSTAT_FILE"; then
            exit 0
        else
            exit 1
        fi
    fi
}

case "${1:-}" in
    save)
        save_baseline
        ;;
    check)
        compare_benchmarks true
        ;;
    help|--help|-h)
        show_help
        ;;
    "")
        compare_benchmarks false
        ;;
    *)
        echo "Unknown command: $1"
        show_help
        exit 1
        ;;
esac
