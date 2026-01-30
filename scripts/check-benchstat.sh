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

THRESHOLD=${BENCHMARK_REGRESSION_THRESHOLD:-20}  # Default 20% threshold

if [ -z "$1" ]; then
    echo "Usage: $0 <benchstat-output-file>"
    exit 1
fi

INPUT_FILE="$1"

if [ ! -f "$INPUT_FILE" ]; then
    echo "Error: File not found: $INPUT_FILE"
    exit 1
fi

# Parse benchstat output for significant changes
# Look for lines with percentage changes that are statistically significant
# Format: BenchmarkName  old  new  +/-XX.XX% (p=0.XXX n=XX)
#
# benchstat groups results by metric type (sec/op, B/op, allocs/op)
# We track the current metric type to provide clearer output

REGRESSIONS=()
IMPROVEMENTS=()
CURRENT_METRIC=""

# ANSI color codes (only if outputting to terminal)
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    NC='\033[0m' # No Color
else
    RED=''
    GREEN=''
    NC=''
fi

while IFS= read -r line; do
    # Track which metric section we're in based on header
    if echo "$line" | grep -qE 'sec/op.*vs base'; then
        CURRENT_METRIC="time (sec/op)"
        continue
    elif echo "$line" | grep -qE 'B/op.*vs base'; then
        CURRENT_METRIC="memory (B/op)"
        continue
    elif echo "$line" | grep -qE 'allocs/op.*vs base'; then
        CURRENT_METRIC="allocations (allocs/op)"
        continue
    fi

    # Skip lines with ~ (not statistically significant) or no percentage
    if echo "$line" | grep -qE '~\s*\(p='; then
        continue
    fi

    # Extract the benchmark name (first column)
    benchmark_name=$(echo "$line" | awk '{print $1}')

    # Check for regression (positive percentage)
    if echo "$line" | grep -qE '\+[0-9]+\.[0-9]+%'; then
        pct=$(echo "$line" | grep -oE '\+[0-9]+\.[0-9]+%' | head -1 | tr -d '+%')
        if [ -n "$pct" ]; then
            # Check if it exceeds threshold
            is_significant=$(awk "BEGIN {print ($pct > $THRESHOLD)}")
            if [ "$is_significant" -eq 1 ]; then
                REGRESSIONS+=("${benchmark_name}|+${pct}%|${CURRENT_METRIC}")
            fi
        fi
    # Check for improvement (negative percentage)
    elif echo "$line" | grep -qE '\-[0-9]+\.[0-9]+%'; then
        pct=$(echo "$line" | grep -oE '\-[0-9]+\.[0-9]+%' | head -1)
        if [ -n "$pct" ]; then
            IMPROVEMENTS+=("${benchmark_name}|${pct}|${CURRENT_METRIC}")
        fi
    fi
done < "$INPUT_FILE"

# Display results
echo ""
echo "=== Benchmark Comparison Summary ==="
echo ""

if [ ${#IMPROVEMENTS[@]} -gt 0 ]; then
    echo -e "${GREEN}✅ Improvements:${NC}"
    for item in "${IMPROVEMENTS[@]}"; do
        IFS='|' read -r name pct metric <<< "$item"
        echo -e "    ${GREEN}${name}: ${pct} ${metric}${NC}"
    done
    echo ""
fi

if [ ${#REGRESSIONS[@]} -gt 0 ]; then
    echo -e "${RED}❌ Regressions (>${THRESHOLD}%):${NC}"
    for item in "${REGRESSIONS[@]}"; do
        IFS='|' read -r name pct metric <<< "$item"
        echo -e "    ${RED}${name}: ${pct} ${metric}${NC}"
    done
    echo ""
    echo "Review these changes to ensure the regressions are acceptable."
    exit 1
else
    if [ ${#IMPROVEMENTS[@]} -eq 0 ]; then
        echo "No significant changes detected."
    fi
    echo ""
    echo -e "${GREEN}✅ No significant regressions detected (threshold: ${THRESHOLD}%)${NC}"
    exit 0
fi
