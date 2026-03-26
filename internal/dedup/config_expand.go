package dedup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

// ResolveIncludePaths reads a standard config file and resolves its includes
// glob patterns into a sorted, deduplicated list of absolute file paths.
// The input config uses the same format as mount/validate (includes field
// with glob patterns). This is used by expand-config to generate an explicit
// config from a wildcard-based one.
func ResolveIncludePaths(configPaths []string) ([]string, error) {
	seen := make(map[string]bool)
	var files []string

	for _, configPath := range configPaths {
		paths, err := resolveIncludePathsFromFile(configPath, seen)
		if err != nil {
			return nil, err
		}
		files = append(files, paths...)
	}

	sort.Strings(files)
	return files, nil
}

// resolveIncludePathsFromFile reads a single config file and recursively
// resolves its includes into absolute file paths.
func resolveIncludePathsFromFile(configPath string, seen map[string]bool) ([]string, error) {
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("resolve path %s: %w", configPath, err)
	}

	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return nil, fmt.Errorf("resolve symlinks %s: %w", absPath, err)
	}

	if seen[realPath] {
		return nil, nil
	}
	seen[realPath] = true

	data, err := os.ReadFile(realPath)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", realPath, err)
	}

	var cf configFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", realPath, err)
	}

	configDir := filepath.Dir(realPath)
	var files []string

	// If this file is a direct config (has name/dedup_file/source_dir),
	// include it in the output.
	if cf.Name != "" && cf.DedupFile != "" && cf.SourceDir != "" {
		files = append(files, realPath)
	}

	// Resolve includes globs.
	for _, pattern := range cf.Includes {
		pattern = resolveRelative(configDir, pattern)
		matches, err := doublestar.FilepathGlob(pattern)
		if err != nil {
			return nil, fmt.Errorf("expand include pattern %q in %s: %w", pattern, realPath, err)
		}
		sort.Strings(matches)
		for _, match := range matches {
			sub, err := resolveIncludePathsFromFile(match, seen)
			if err != nil {
				return nil, err
			}
			files = append(files, sub...)
		}
	}

	// Virtual files are inline — they don't have separate file paths to include.
	// They'll be in the output as part of the original config file if it was
	// included above.

	return files, nil
}
