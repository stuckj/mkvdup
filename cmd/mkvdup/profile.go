//go:build !debug

package main

// parseCPUProfileFlag is a no-op in release builds.
// The --cpuprofile flag is only available in debug builds (go build -tags debug).
func parseCPUProfileFlag(args []string) ([]string, string) {
	return args, ""
}

// debugOptionsHelp returns empty string in release builds.
func debugOptionsHelp() string {
	return ""
}

// startCPUProfile is a no-op in release builds.
func startCPUProfile(_ string) func() {
	return func() {}
}
