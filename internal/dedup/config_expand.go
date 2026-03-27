package dedup

import (
	"fmt"
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
