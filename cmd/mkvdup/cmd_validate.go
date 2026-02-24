package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/stuckj/mkvdup/internal/dedup"
)

// validationEntry tracks the result of validating a single resolved config entry.
type validationEntry struct {
	name       string // virtual file name
	status     string // "OK", "WARN", "ERR"
	message    string // detail message (empty for OK)
	configFile string // which input config file this came from
	dedupFile  string // resolved dedup file path
}

// validateConfigEntries resolves and validates each config file: YAML parsing,
// path existence checks, and dedup file header validation. Returns the
// validation entries, the successfully-parsed configs, and whether any errors
// were found.
func validateConfigEntries(configPaths []string) ([]validationEntry, []dedup.Config, bool) {
	var allEntries []validationEntry
	var allConfigs []dedup.Config
	hasErrors := false

	for _, configPath := range configPaths {
		fmt.Printf("Validating %s...\n", filepath.Base(configPath))

		configs, _, err := dedup.ResolveConfigs([]string{configPath})
		if err != nil {
			fmt.Printf("  ERR  %s\n", err)
			allEntries = append(allEntries, validationEntry{
				name:       filepath.Base(configPath),
				status:     "ERR",
				message:    err.Error(),
				configFile: configPath,
			})
			hasErrors = true
			continue
		}

		if len(configs) == 0 {
			fmt.Printf("  (no entries)\n")
			continue
		}

		for _, cfg := range configs {
			entry := validationEntry{
				name:       cfg.Name,
				status:     "OK",
				configFile: configPath,
				dedupFile:  cfg.DedupFile,
			}

			// Check dedup file exists
			dedupStat, err := os.Stat(cfg.DedupFile)
			if err != nil {
				entry.status = "ERR"
				entry.message = fmt.Sprintf("dedup file: %v", err)
				fmt.Printf("  ERR  %s: %s\n", cfg.Name, entry.message)
				allEntries = append(allEntries, entry)
				hasErrors = true
				continue
			}
			if dedupStat.IsDir() {
				entry.status = "ERR"
				entry.message = fmt.Sprintf("dedup file is a directory: %s", cfg.DedupFile)
				fmt.Printf("  ERR  %s: %s\n", cfg.Name, entry.message)
				allEntries = append(allEntries, entry)
				hasErrors = true
				continue
			}

			// Check source dir exists and is a directory
			sourceStat, err := os.Stat(cfg.SourceDir)
			if err != nil {
				entry.status = "ERR"
				entry.message = fmt.Sprintf("source directory: %v", err)
				fmt.Printf("  ERR  %s: %s\n", cfg.Name, entry.message)
				allEntries = append(allEntries, entry)
				hasErrors = true
				continue
			}
			if !sourceStat.IsDir() {
				entry.status = "ERR"
				entry.message = fmt.Sprintf("source path is not a directory: %s", cfg.SourceDir)
				fmt.Printf("  ERR  %s: %s\n", cfg.Name, entry.message)
				allEntries = append(allEntries, entry)
				hasErrors = true
				continue
			}

			// Validate dedup file header
			reader, err := dedup.NewReaderLazy(cfg.DedupFile, cfg.SourceDir)
			if err != nil {
				entry.status = "ERR"
				entry.message = fmt.Sprintf("invalid dedup file: %v", err)
				fmt.Printf("  ERR  %s: %s\n", cfg.Name, entry.message)
				allEntries = append(allEntries, entry)
				hasErrors = true
				continue
			}
			reader.Close()

			allEntries = append(allEntries, entry)
			allConfigs = append(allConfigs, cfg)
		}
	}

	return allEntries, allConfigs, hasErrors
}

// checkNameConflicts validates virtual file paths and detects duplicate names
// and file/directory conflicts across all entries. Updates entry statuses
// in-place and returns whether any errors or warnings were found.
func checkNameConflicts(entries []validationEntry) (hasErrors, hasWarnings bool) {
	nameToConfig := make(map[string]string)   // clean path -> config file
	dirComponents := make(map[string]string)  // paths used as directories -> config file
	fileComponents := make(map[string]string) // paths used as files -> config file

	for i, entry := range entries {
		if entry.status == "ERR" {
			continue
		}

		name := entry.name

		// Check for ".." path components
		if slices.Contains(strings.Split(name, "/"), "..") {
			entries[i].status = "ERR"
			entries[i].message = "invalid path: contains '..' component"
			fmt.Printf("  ERR  %s: %s\n", name, entries[i].message)
			hasErrors = true
			continue
		}

		// Clean and validate the path (same logic as tree.go insertFile)
		cleanPath := cleanVirtualPath(name)
		if cleanPath == "" {
			entries[i].status = "ERR"
			entries[i].message = "invalid path: empty after cleaning"
			fmt.Printf("  ERR  %s: %s\n", name, entries[i].message)
			hasErrors = true
			continue
		}

		// Check for duplicate names
		if prevConfig, exists := nameToConfig[cleanPath]; exists {
			entries[i].status = "WARN"
			entries[i].message = fmt.Sprintf("duplicate name (also in %s)", filepath.Base(prevConfig))
			fmt.Printf("  WARN %s: %s\n", name, entries[i].message)
			hasWarnings = true
			continue
		}
		nameToConfig[cleanPath] = entry.configFile

		// Check for file/directory conflicts
		parts := strings.Split(cleanPath, "/")
		conflictFound := false

		// Check if any prefix of this path is used as a file
		for j := 0; j < len(parts)-1; j++ {
			dirPath := strings.Join(parts[:j+1], "/")
			if prevConfig, exists := fileComponents[dirPath]; exists {
				entries[i].status = "WARN"
				entries[i].message = fmt.Sprintf("path component %q conflicts with file in %s", dirPath, filepath.Base(prevConfig))
				fmt.Printf("  WARN %s: %s\n", name, entries[i].message)
				hasWarnings = true
				conflictFound = true
				break
			}
			// Record as directory component
			if _, exists := dirComponents[dirPath]; !exists {
				dirComponents[dirPath] = entry.configFile
			}
		}
		if conflictFound {
			continue
		}

		// Check if this file name conflicts with a directory
		if prevConfig, exists := dirComponents[cleanPath]; exists {
			entries[i].status = "WARN"
			entries[i].message = fmt.Sprintf("conflicts with directory from %s", filepath.Base(prevConfig))
			fmt.Printf("  WARN %s: %s\n", name, entries[i].message)
			hasWarnings = true
			continue
		}

		fileComponents[cleanPath] = entry.configFile

		// Print OK for entries that passed all checks
		if entries[i].status == "OK" {
			fmt.Printf("  OK   %s\n", name)
		}
	}

	return hasErrors, hasWarnings
}

// runDeepValidation performs integrity verification on dedup files that passed
// basic validation. Returns whether any errors were found.
func runDeepValidation(entries []validationEntry, configs []dedup.Config) bool {
	fmt.Println()
	fmt.Println("Running deep validation...")
	hasErrors := false
	for _, cfg := range configs {
		// Only deep-validate entries that passed basic validation
		entryOK := false
		for _, e := range entries {
			if e.name == cfg.Name && e.dedupFile == cfg.DedupFile && e.status != "ERR" {
				entryOK = true
				break
			}
		}
		if !entryOK {
			continue
		}

		reader, err := dedup.NewReader(cfg.DedupFile, cfg.SourceDir)
		if err != nil {
			fmt.Printf("  ERR  %s: failed to open: %v\n", cfg.Name, err)
			hasErrors = true
			continue
		}
		if err := reader.VerifyIntegrity(); err != nil {
			fmt.Printf("  ERR  %s: integrity check failed: %v\n", cfg.Name, err)
			reader.Close()
			hasErrors = true
			continue
		}
		reader.Close()
		fmt.Printf("  OK   %s: checksums valid\n", cfg.Name)
	}
	return hasErrors
}

// validateConfigs validates configuration files and returns an exit code.
// Returns 0 if all configs are valid (warnings OK without strict), 1 otherwise.
func validateConfigs(configPaths []string, configDir, deep, strict bool) int {
	resolved, err := resolveConfigPaths(configPaths, configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	allEntries, allConfigs, hasErrors := validateConfigEntries(resolved)

	nameErrors, hasWarnings := checkNameConflicts(allEntries)
	hasErrors = hasErrors || nameErrors

	if deep {
		hasErrors = hasErrors || runDeepValidation(allEntries, allConfigs)
	}

	// Print summary
	var okCount, warnCount, errCount int
	for _, e := range allEntries {
		switch e.status {
		case "OK":
			okCount++
		case "WARN":
			warnCount++
		case "ERR":
			errCount++
		}
	}

	fmt.Println()
	fmt.Printf("Summary: %d %s, %d valid, %d %s, %d %s\n",
		len(allEntries), plural(len(allEntries), "entry", "entries"),
		okCount,
		warnCount, plural(warnCount, "warning", "warnings"),
		errCount, plural(errCount, "error", "errors"))

	if hasErrors {
		return 1
	}
	if strict && hasWarnings {
		return 1
	}
	return 0
}

// cleanVirtualPath normalizes a virtual file path, matching the logic in
// internal/fuse/tree.go insertFile(). Returns empty string if the path is invalid.
func cleanVirtualPath(name string) string {
	// Clean the path using path.Clean (not filepath.Clean) to match
	// internal/fuse/tree.go insertFile() which uses forward-slash paths.
	cleaned := path.Clean(name)
	// Split and filter
	parts := strings.Split(cleaned, "/")
	var valid []string
	for _, p := range parts {
		if p != "" && p != "." {
			valid = append(valid, p)
		}
	}
	if len(valid) == 0 {
		return ""
	}
	return strings.Join(valid, "/")
}
