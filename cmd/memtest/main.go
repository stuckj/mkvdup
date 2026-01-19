package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/stuckj/mkvdup/internal/source"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: memtest <source_dir>")
		os.Exit(1)
	}

	sourceDir := os.Args[1]

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("Initial: Alloc=%dMB, Sys=%dMB\n", m.Alloc/1024/1024, m.Sys/1024/1024)

	fmt.Print("Detecting source type...")
	sourceType, err := source.DetectType(sourceDir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	runtime.ReadMemStats(&m)
	fmt.Printf(" %s (Alloc=%dMB)\n", sourceType, m.Alloc/1024/1024)

	fmt.Print("Creating indexer...")
	// Create indexer
	indexer, err := source.NewIndexer(sourceDir, 64)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	runtime.ReadMemStats(&m)
	fmt.Printf(" Done (Alloc=%dMB)\n", m.Alloc/1024/1024)

	fmt.Printf("Source type: %s\n", indexer.SourceType())

	runtime.ReadMemStats(&m)
	fmt.Printf("After NewIndexer: Alloc=%dMB, Sys=%dMB\n", m.Alloc/1024/1024, m.Sys/1024/1024)

	fmt.Println("Starting build...")
	// Build with progress
	count := 0
	err = indexer.Build(func(processed, total int64) {
		count++
		if count%10 == 0 { // More frequent output
			runtime.ReadMemStats(&m)
			fmt.Printf("Progress %dMB/%dMB: Alloc=%dMB, Sys=%dMB\n",
				processed/1024/1024, total/1024/1024, m.Alloc/1024/1024, m.Sys/1024/1024)

			// Stop if memory gets too high
			if m.Alloc > 2*1024*1024*1024 { // 2GB limit
				fmt.Println("STOPPING: Memory exceeded 2GB!")
				// Write heap profile
				f, err := os.Create("/tmp/mkvdup-heap.prof")
				if err == nil {
					runtime.GC() // Force GC before profile
					pprof.WriteHeapProfile(f)
					f.Close()
					fmt.Println("Heap profile written to /tmp/mkvdup-heap.prof")
					fmt.Println("Analyze with: go tool pprof /tmp/mkvdup-heap.prof")
				}
				os.Exit(1)
			}
		}
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	runtime.ReadMemStats(&m)
	fmt.Printf("Final: Alloc=%dMB, Sys=%dMB\n", m.Alloc/1024/1024, m.Sys/1024/1024)

	idx := indexer.Index()
	fmt.Printf("Hash entries: %d\n", len(idx.HashToLocations))
}
