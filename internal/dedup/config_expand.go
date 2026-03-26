package dedup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

// ExpandSource represents a single source entry in an expand-config input file.
type ExpandSource struct {
	Path    string `yaml:"path"`
	Pattern string `yaml:"pattern"`
}

// ExpandConfig represents the input config for expand-config, containing
// glob patterns that resolve to .mkvdup.yaml files.
type ExpandConfig struct {
	Sources []ExpandSource `yaml:"sources"`
}

// ExpandedConfig represents the output of expand-config: an explicit list
// of resolved .mkvdup.yaml file paths, using the standard includes format.
type ExpandedConfig struct {
	Includes []string `yaml:"includes"`
}

// ReadExpandConfig reads and validates an expand-config input file.
func ReadExpandConfig(configPath string) (*ExpandConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read expand config: %w", err)
	}

	var cfg ExpandConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse expand config %s: %w", configPath, err)
	}

	if len(cfg.Sources) == 0 {
		return nil, fmt.Errorf("expand config %s: sources list is empty", configPath)
	}

	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}
	configDir := filepath.Dir(absPath)

	for i := range cfg.Sources {
		s := &cfg.Sources[i]
		if s.Path == "" {
			return nil, fmt.Errorf("expand config %s: sources[%d] missing required 'path' field", configPath, i)
		}
		if s.Pattern == "" {
			return nil, fmt.Errorf("expand config %s: sources[%d] missing required 'pattern' field", configPath, i)
		}
		s.Path = resolveRelative(configDir, s.Path)
	}

	return &cfg, nil
}

// ResolveExpandConfig walks all sources and resolves glob patterns to produce
// a sorted, deduplicated list of absolute .mkvdup.yaml file paths.
func ResolveExpandConfig(cfg *ExpandConfig) ([]string, error) {
	seen := make(map[string]bool)
	var files []string

	for _, src := range cfg.Sources {
		pattern := filepath.Join(src.Path, src.Pattern)
		matches, err := doublestar.FilepathGlob(pattern)
		if err != nil {
			return nil, fmt.Errorf("expand glob pattern %q: %w", pattern, err)
		}
		for _, m := range matches {
			abs, err := filepath.Abs(m)
			if err != nil {
				return nil, fmt.Errorf("resolve path %s: %w", m, err)
			}
			if !seen[abs] {
				seen[abs] = true
				files = append(files, abs)
			}
		}
	}

	sort.Strings(files)
	return files, nil
}
