package dedup

import (
	"sort"
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
			// Collect paths of configs that define a direct mapping.
			if cf.Name != "" && cf.DedupFile != "" && cf.SourceDir != "" {
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
