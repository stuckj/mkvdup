package main

import (
	"os"
	"strconv"
)

// formatInt formats an integer with thousands separators (e.g., 1234567 → "1,234,567").
func formatInt(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	// Insert commas from the right
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// plural returns singular when n == 1, plural otherwise.
// Example: plural(n, "file", "files")
func plural(n int, singular, pl string) string {
	if n == 1 {
		return singular
	}
	return pl
}

// isTerminal returns true if stdin is a terminal (not piped).
func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
