package main

import (
	"fmt"
	"os"

	"github.com/stuckj/mkvdup/internal/dedup"
)

// fileStats holds statistics for a single dedup file.
type fileStats struct {
	name        string
	dedupFile   string
	sourceDir   string
	origSize    int64
	dedupSize   int64
	sourceType  string
	sourceFiles int
	entryCount  int
	err         error
}

// showStats displays space savings and file statistics for mkvdup-managed files.
func showStats(configPaths []string, configDir bool) error {
	resolved, err := resolveConfigPaths(configPaths, configDir)
	if err != nil {
		return err
	}

	// Resolve each config independently so a single bad config doesn't
	// abort the entire stats run.
	var configs []dedup.Config
	for _, cfgPath := range resolved {
		cfgs, _, cfgErr := dedup.ResolveConfigs([]string{cfgPath})
		if cfgErr != nil {
			printWarn("Failed to load config %s: %v\n", cfgPath, cfgErr)
			continue
		}
		configs = append(configs, cfgs...)
	}

	if len(configs) == 0 {
		printInfoln("No files found.")
		return nil
	}

	var stats []fileStats
	for _, cfg := range configs {
		fs := collectFileStats(cfg)
		stats = append(stats, fs)

		if fs.err != nil {
			printWarn("%s\n  Error: %v\n\n", fs.name, fs.err)
			continue
		}

		printFileStats(fs)
	}

	printRollupStats(stats)

	return nil
}

// collectFileStats gathers statistics for a single dedup file from its config.
func collectFileStats(cfg dedup.Config) fileStats {
	fs := fileStats{
		name:      cfg.Name,
		dedupFile: cfg.DedupFile,
		sourceDir: cfg.SourceDir,
	}

	reader, err := dedup.NewReaderLazy(cfg.DedupFile, cfg.SourceDir)
	if err != nil {
		fs.err = fmt.Errorf("open dedup file: %w", err)
		return fs
	}
	defer reader.Close()

	info := reader.Info()
	if errMsg, ok := info["error"]; ok {
		fs.err = fmt.Errorf("read dedup file: %v", errMsg)
		return fs
	}

	fs.origSize = info["original_size"].(int64)
	fs.sourceFiles = info["source_file_count"].(int)
	fs.entryCount = info["entry_count"].(int)

	switch info["source_type"].(uint8) {
	case 0:
		fs.sourceType = "DVD"
	case 1:
		fs.sourceType = "Blu-ray"
	default:
		fs.sourceType = "Unknown"
	}

	dedupInfo, err := os.Stat(cfg.DedupFile)
	if err != nil {
		fs.err = fmt.Errorf("stat dedup file: %w", err)
		return fs
	}
	fs.dedupSize = dedupInfo.Size()

	return fs
}

// printFileStats prints per-file statistics.
func printFileStats(fs fileStats) {
	savings := float64(0)
	if fs.origSize > 0 {
		savings = float64(fs.origSize-fs.dedupSize) / float64(fs.origSize) * 100
	}

	printInfo("%s\n", fs.name)
	printInfo("  Original size:     %s bytes (%s)\n", formatInt(fs.origSize), formatSize(fs.origSize))
	printInfo("  Dedup file size:   %s bytes (%s)\n", formatInt(fs.dedupSize), formatSize(fs.dedupSize))
	printInfo("  Space savings:     %s bytes (%.2f%%)\n", formatInt(fs.origSize-fs.dedupSize), savings)
	printInfo("  Source type:       %s\n", fs.sourceType)
	printInfo("  Source directory:  %s\n", fs.sourceDir)
	printInfo("  Source files:      %d\n", fs.sourceFiles)
	printInfo("  Index entries:     %s\n", formatInt(int64(fs.entryCount)))
	printInfoln()
}

// printRollupStats prints aggregate statistics across all successful files.
func printRollupStats(stats []fileStats) {
	var totalOrig, totalDedup int64
	var succeeded int
	uniqueSources := map[string]struct{}{}

	for _, fs := range stats {
		if fs.err != nil {
			continue
		}
		succeeded++
		totalOrig += fs.origSize
		totalDedup += fs.dedupSize
		uniqueSources[fs.sourceDir] = struct{}{}
	}

	if succeeded < 2 {
		return
	}

	savings := float64(0)
	if totalOrig > 0 {
		savings = float64(totalOrig-totalDedup) / float64(totalOrig) * 100
	}

	printInfo("Totals (%d %s):\n", succeeded, plural(succeeded, "file", "files"))
	printInfo("  Original size:     %s bytes (%s)\n", formatInt(totalOrig), formatSize(totalOrig))
	printInfo("  Dedup file size:   %s bytes (%s)\n", formatInt(totalDedup), formatSize(totalDedup))
	printInfo("  Space savings:     %s bytes (%.2f%%)\n", formatInt(totalOrig-totalDedup), savings)
	printInfo("  Unique sources:    %d\n", len(uniqueSources))
}
