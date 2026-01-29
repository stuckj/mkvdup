//go:build debug

package main

import (
	"log"
	"os"
	"runtime/pprof"
	"strings"
)

// parseCPUProfileFlag extracts the --cpuprofile flag from args and returns the
// filtered args. Only available in debug builds (go build -tags debug).
func parseCPUProfileFlag(args []string) ([]string, string) {
	var filtered []string
	var cpuprofile string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--cpuprofile" && i+1 < len(args):
			i++
			cpuprofile = args[i]
		case strings.HasPrefix(arg, "--cpuprofile="):
			cpuprofile = strings.TrimPrefix(arg, "--cpuprofile=")
		default:
			filtered = append(filtered, arg)
		}
	}

	return filtered, cpuprofile
}

// debugOptionsHelp returns help text for debug-only options.
func debugOptionsHelp() string {
	return "\nDebug options (debug build only):\n  --cpuprofile FILE  Write CPU profile to FILE\n"
}

// startCPUProfile starts CPU profiling to the given path. Returns a stop
// function that must be called (typically via defer) to flush the profile.
// Only available in debug builds (go build -tags debug).
func startCPUProfile(path string) func() {
	if path == "" {
		return func() {}
	}

	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("could not create CPU profile: %v", err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		log.Fatalf("could not start CPU profile: %v", err)
	}

	return func() {
		pprof.StopCPUProfile()
		f.Close()
	}
}
