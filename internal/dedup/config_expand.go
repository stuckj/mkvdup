package dedup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/stuckj/mkvdup/internal/security"
	"gopkg.in/yaml.v3"
)

// ResolveIncludePaths reads standard config files and resolves their includes
// glob patterns into a sorted, deduplicated list of absolute file paths.
// The input config uses the same format as mount/validate (includes field
// with glob patterns). This is used by expand-config to generate an explicit
// config from a wildcard-based one.
func ResolveIncludePaths(configPaths []string) ([]string, error) {
	seen := make(map[string]bool)
	var files []string

	for _, configPath := range configPaths {
		err := walkConfig(configPath, seen, func(phase, realPath string, cf *configFile, _ string) error {
			if phase != "pre" {
				return nil
			}

			// Validate partial top-level fields (same rules as resolveConfig).
			hasName := cf.Name != ""
			hasDedup := cf.DedupFile != ""
			hasSource := cf.SourceDir != ""
			hasDirectMapping := hasName && hasDedup && hasSource
			if (hasName || hasDedup || hasSource) && !hasDirectMapping {
				return fmt.Errorf("config %s: name, dedup_file, and source_dir must all be set if any is set", realPath)
			}

			// Validate virtual_files entries.
			for _, vf := range cf.VirtualFiles {
				if vf.Name == "" || vf.DedupFile == "" || vf.SourceDir == "" {
					return fmt.Errorf("config %s: virtual_files entry missing required fields (name, dedup_file, source_dir)", realPath)
				}
			}

			// Collect paths of configs that contribute any mappings.
			if hasDirectMapping || len(cf.VirtualFiles) > 0 {
				files = append(files, realPath)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	sort.Strings(files)
	return files, nil
}

// ExpandConfigFile reads a config file, resolves its includes glob patterns
// to explicit paths (single level, no recursion), and returns the expanded
// config as YAML bytes. All other settings (on_error_command, virtual_files,
// top-level name/dedup_file/source_dir) are preserved unchanged. The included
// files themselves are not modified — they can still contain their own globs.
func ExpandConfigFile(configPath string) ([]byte, error) {
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("resolve path %s: %w", configPath, err)
	}

	// Resolve symlinks for consistent behavior with mount/validate.
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return nil, fmt.Errorf("resolve symlinks %s: %w", absPath, err)
	}

	// When running as root, verify config file ownership and permissions.
	if err := security.CheckFileOwnershipResolved(realPath); err != nil {
		return nil, fmt.Errorf("config file %s: %w", realPath, err)
	}

	data, err := os.ReadFile(realPath)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", realPath, err)
	}

	var cf configFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", realPath, err)
	}

	// Validate partial top-level fields (same rules as resolveConfig).
	hasName := cf.Name != ""
	hasDedup := cf.DedupFile != ""
	hasSource := cf.SourceDir != ""
	if (hasName || hasDedup || hasSource) && !(hasName && hasDedup && hasSource) {
		return nil, fmt.Errorf("config %s: name, dedup_file, and source_dir must all be set if any is set", realPath)
	}

	// Validate virtual_files entries.
	for _, vf := range cf.VirtualFiles {
		if vf.Name == "" || vf.DedupFile == "" || vf.SourceDir == "" {
			return nil, fmt.Errorf("config %s: virtual_files entry missing required fields (name, dedup_file, source_dir)", realPath)
		}
	}

	// If there are no includes, nothing to expand.
	if len(cf.Includes) == 0 {
		return data, nil
	}

	// Resolve each include glob pattern to explicit paths (single level only).
	configDir := filepath.Dir(realPath)
	seen := make(map[string]bool)
	var resolved []string
	for _, pattern := range cf.Includes {
		pattern = resolveRelative(configDir, pattern)
		matches, err := doublestar.FilepathGlob(pattern)
		if err != nil {
			return nil, fmt.Errorf("expand include pattern %q in %s: %w", pattern, absPath, err)
		}
		sort.Strings(matches)
		for _, match := range matches {
			abs, err := filepath.Abs(match)
			if err != nil {
				return nil, fmt.Errorf("resolve path %s: %w", match, err)
			}
			if !seen[abs] {
				seen[abs] = true
				resolved = append(resolved, abs)
			}
		}
	}

	// Replace includes with the resolved explicit paths (sorted globally).
	sort.Strings(resolved)
	cf.Includes = resolved

	out, err := yaml.Marshal(&cf)
	if err != nil {
		return nil, fmt.Errorf("marshal expanded config: %w", err)
	}

	return out, nil
}
