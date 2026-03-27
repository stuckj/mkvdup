package dedup

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

// resolveIncludePaths reads standard config files and resolves their includes
// glob patterns into a sorted, deduplicated list of absolute file paths.
// It can be used to compute the explicit set of config files that contribute
// mappings from a wildcard-based configuration.
func resolveIncludePaths(configPaths []string) ([]string, error) {
	seen := make(map[string]bool)
	var files []string

	for _, configPath := range configPaths {
		err := walkConfig(configPath, seen, func(phase, realPath string, cf *configFile, _ string) error {
			if phase != "pre" {
				return nil
			}

			if err := validateConfigFields(realPath, cf); err != nil {
				return err
			}

			// Collect paths of configs that contribute any mappings.
			hasDirectMapping := cf.Name != "" && cf.DedupFile != "" && cf.SourceDir != ""
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
	realPath, _, cf, err := openConfigFile(configPath)
	if err != nil {
		return nil, err
	}

	if err := validateConfigFields(realPath, cf); err != nil {
		return nil, err
	}

	// Validate on_error_command (same check as ResolveConfigs) so that
	// expand-config fails fast on invalid input.
	if cf.OnErrorCommand != nil && len(cf.OnErrorCommand.Command.Args) == 0 {
		return nil, fmt.Errorf("%s: on_error_command.command must not be empty", realPath)
	}

	// If there are no includes, marshal the parsed config (not raw data) to
	// ensure consistent output formatting and avoid accumulating headers when
	// expand-config is run on already-expanded output.
	if len(cf.Includes) == 0 {
		return yaml.Marshal(cf)
	}

	// Resolve each include glob pattern to explicit paths (single level only).
	configDir := filepath.Dir(realPath)
	seen := make(map[string]bool)
	var resolved []string
	for _, pattern := range cf.Includes {
		pattern = resolveRelative(configDir, pattern)
		matches, err := doublestar.FilepathGlob(pattern)
		if err != nil {
			return nil, fmt.Errorf("expand include pattern %q in %s: %w", pattern, realPath, err)
		}
		sort.Strings(matches)
		for _, match := range matches {
			abs, err := filepath.Abs(match)
			if err != nil {
				return nil, fmt.Errorf("resolve path %s: %w", match, err)
			}
			// Canonicalize via EvalSymlinks for dedup, matching walkConfig's behavior.
			canon := abs
			if real, err := filepath.EvalSymlinks(abs); err == nil {
				canon = real
			}
			if !seen[canon] {
				seen[canon] = true
				resolved = append(resolved, canon)
			}
		}
	}

	// Replace includes with the resolved explicit paths (sorted globally).
	sort.Strings(resolved)
	cf.Includes = resolved

	out, err := yaml.Marshal(cf)
	if err != nil {
		return nil, fmt.Errorf("marshal expanded config: %w", err)
	}

	return out, nil
}
