#!/bin/bash
# Check benchstat output for significant performance regressions.
# Exit 0 if no significant regressions, exit 1 if regressions detected.
#
# Usage: ./scripts/check-benchstat.sh <benchstat-output-file>
#
# A regression is detected when:
# - The benchmark is slower (positive percentage)
# - The change is statistically significant (no ~ marker)
# - The slowdown exceeds the threshold (default 10%)

set -e

THRESHOLD=${BENCHMARK_REGRESSION_THRESHOLD:-10}  # Default 10% threshold

if [ -z "$1" ]; then
    echo "Usage: $0 <benchstat-output-file>"
    exit 1
fi

INPUT_FILE="$1"

if [ ! -f "$INPUT_FILE" ]; then
    echo "Error: File not found: $INPUT_FILE"
    exit 1
fi

# Parse benchstat output for significant regressions
# Look for lines with "+XX.XX%" that don't have "~" (which indicates no significant change)
# Format: BenchmarkName  old  new  +XX.XX% (p=0.XXX n=XX)

REGRESSIONS=()

while IFS= read -r line; do
    # Skip header lines and lines without percentage changes
    if ! echo "$line" | grep -qE '\+[0-9]+\.[0-9]+%'; then
        continue
    fi

    # Skip lines with ~ (not statistically significant)
    if echo "$line" | grep -qE '~.*\(p='; then
        continue
    fi

    # Extract the percentage change
    pct=$(echo "$line" | grep -oE '\+[0-9]+\.[0-9]+%' | head -1 | tr -d '+%')

    if [ -n "$pct" ]; then
        # Compare against threshold (using bc for floating point)
        is_regression=$(echo "$pct > $THRESHOLD" | bc -l)
        if [ "$is_regression" -eq 1 ]; then
            REGRESSIONS+=("$line")
        fi
    fi
done < "$INPUT_FILE"

if [ ${#REGRESSIONS[@]} -gt 0 ]; then
    echo "❌ Significant performance regressions detected (>${THRESHOLD}%):"
    echo ""
    for regression in "${REGRESSIONS[@]}"; do
        echo "  $regression"
    done
    echo ""
    echo "Review these changes to ensure the regression is acceptable."
    exit 1
else
    echo "✅ No significant performance regressions detected (threshold: ${THRESHOLD}%)"
    exit 0
fi
