package fuse

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/stuckj/mkvdup/internal/dedup"
)

// MKVFSOptions contains options for creating an MKVFS filesystem.
type MKVFSOptions struct {
	Verbose         bool
	PermissionsPath string
	// Defaults holds the default permissions to use when a PermissionStore is configured.
	// If nil, DefaultPerms() is used. Set to a non-nil value to use specific defaults.
	// Note: explicit-zero defaults only work when provided programmatically here;
	// they are not persisted to or loaded from the permissions YAML file.
	Defaults *Defaults
}

// NewMKVFS creates a new MKVFS root from a list of config files.
// Config files are resolved recursively (includes and virtual_files are expanded).
// Set verbose=true to enable debug logging.
func NewMKVFS(configPaths []string, verbose bool) (*MKVFSRoot, error) {
	configs, _, err := dedup.ResolveConfigs(configPaths)
	if err != nil {
		return nil, fmt.Errorf("resolve configs: %w", err)
	}
	return NewMKVFSFromConfigs(configs, verbose, &DefaultReaderFactory{}, nil)
}

// NewMKVFSWithPermissions creates a new MKVFS root with a permission store.
// Config files are resolved recursively (includes and virtual_files are expanded).
func NewMKVFSWithPermissions(configPaths []string, verbose bool, permStore *PermissionStore) (*MKVFSRoot, error) {
	configs, _, err := dedup.ResolveConfigs(configPaths)
	if err != nil {
		return nil, fmt.Errorf("resolve configs: %w", err)
	}
	return NewMKVFSFromConfigs(configs, verbose, &DefaultReaderFactory{}, permStore)
}

// NewMKVFSWithOptions creates a new MKVFS root with the given options.
// Config files are resolved recursively (includes and virtual_files are expanded).
func NewMKVFSWithOptions(configPaths []string, opts MKVFSOptions) (*MKVFSRoot, error) {
	var permStore *PermissionStore
	if opts.PermissionsPath != "" {
		defaults := DefaultPerms()
		if opts.Defaults != nil {
			defaults = *opts.Defaults
		}
		permStore = NewPermissionStore(opts.PermissionsPath, defaults, opts.Verbose)
		if err := permStore.Load(); err != nil {
			return nil, fmt.Errorf("load permissions: %w", err)
		}
	}
	configs, _, err := dedup.ResolveConfigs(configPaths)
	if err != nil {
		return nil, fmt.Errorf("resolve configs: %w", err)
	}
	return NewMKVFSFromConfigs(configs, opts.Verbose, &DefaultReaderFactory{}, permStore)
}

// NewMKVFSWithFactories creates a new MKVFS root with custom factories.
// This allows injecting mock implementations for testing.
func NewMKVFSWithFactories(configPaths []string, verbose bool, readerFactory ReaderFactory, configReader ConfigReader, permStore *PermissionStore) (*MKVFSRoot, error) {
	root := &MKVFSRoot{
		files:         make(map[string]*MKVFile),
		verbose:       verbose,
		readerFactory: readerFactory,
		configReader:  configReader,
		permStore:     permStore,
	}

	if verbose {
		log.Printf("Creating MKVFS with %d config files", len(configPaths))
	}

	for _, configPath := range configPaths {
		if verbose {
			log.Printf("Reading config: %s", configPath)
		}
		config, err := root.configReader.ReadConfig(configPath)
		if err != nil {
			return nil, fmt.Errorf("read config %s: %w", configPath, err)
		}
		if verbose {
			log.Printf("Config: name=%s, dedup=%s, source=%s", config.Name, config.DedupFile, config.SourceDir)
		}

		// Resolve relative paths
		configDir := filepath.Dir(configPath)
		dedupPath := config.DedupFile
		if !filepath.IsAbs(dedupPath) {
			dedupPath = filepath.Join(configDir, dedupPath)
		}
		sourceDir := config.SourceDir
		if !filepath.IsAbs(sourceDir) {
			sourceDir = filepath.Join(configDir, sourceDir)
		}

		// Open dedup file to get size (lazy loading - only reads header)
		if verbose {
			log.Printf("Opening dedup file: %s", dedupPath)
		}
		reader, err := root.readerFactory.NewReaderLazy(dedupPath, sourceDir)
		if err != nil {
			if verbose {
				log.Printf("Failed to open dedup file: %v", err)
			}
			return nil, fmt.Errorf("open dedup file %s: %w", dedupPath, err)
		}

		mkvFile := &MKVFile{
			Name:          config.Name,
			DedupPath:     dedupPath,
			SourceDir:     sourceDir,
			Size:          reader.OriginalSize(),
			readerFactory: root.readerFactory,
		}

		// Don't keep reader open - we'll open it lazily
		reader.Close()

		root.files[config.Name] = mkvFile
		if verbose {
			log.Printf("Added file: %s (size=%d)", config.Name, mkvFile.Size)
		}
	}

	if verbose {
		log.Printf("Total files: %d", len(root.files))
	}

	// Build directory tree from collected files
	fileList := make([]*MKVFile, 0, len(root.files))
	for _, f := range root.files {
		fileList = append(fileList, f)
	}
	root.rootDir = BuildDirectoryTree(fileList, verbose, readerFactory, permStore)

	// Clean up stale permission entries if we have a permission store
	if permStore != nil {
		validFiles, validDirs := root.collectValidPaths()
		removed := permStore.CleanupStale(validFiles, validDirs)
		if removed > 0 {
			if verbose {
				log.Printf("Cleaned up %d stale permission entries", removed)
			}
			if err := permStore.Save(); err != nil {
				log.Printf("Warning: failed to save permissions after cleanup: %v", err)
			}
		}
	}

	if verbose {
		log.Printf("Directory tree built with %d root entries", len(root.rootDir.files)+len(root.rootDir.subdirs))
	}

	return root, nil
}

// NewMKVFSFromConfigs creates a new MKVFS root from already-resolved configs.
// Paths in configs must already be absolute (as returned by dedup.ResolveConfigs).
func NewMKVFSFromConfigs(configs []dedup.Config, verbose bool, readerFactory ReaderFactory, permStore *PermissionStore) (*MKVFSRoot, error) {
	root := &MKVFSRoot{
		files:         make(map[string]*MKVFile),
		verbose:       verbose,
		readerFactory: readerFactory,
		permStore:     permStore,
	}

	if verbose {
		log.Printf("Creating MKVFS with %d resolved configs", len(configs))
	}

	for _, config := range configs {
		if verbose {
			log.Printf("Config: name=%s, dedup=%s, source=%s", config.Name, config.DedupFile, config.SourceDir)
		}

		// Open dedup file to get size (lazy loading - only reads header)
		if verbose {
			log.Printf("Opening dedup file: %s", config.DedupFile)
		}
		reader, err := root.readerFactory.NewReaderLazy(config.DedupFile, config.SourceDir)
		if err != nil {
			if verbose {
				log.Printf("Failed to open dedup file: %v", err)
			}
			return nil, fmt.Errorf("open dedup file %s: %w", config.DedupFile, err)
		}

		mkvFile := &MKVFile{
			Name:          config.Name,
			DedupPath:     config.DedupFile,
			SourceDir:     config.SourceDir,
			Size:          reader.OriginalSize(),
			readerFactory: root.readerFactory,
		}

		// Don't keep reader open - we'll open it lazily
		reader.Close()

		root.files[config.Name] = mkvFile
		if verbose {
			log.Printf("Added file: %s (size=%d)", config.Name, mkvFile.Size)
		}
	}

	if verbose {
		log.Printf("Total files: %d", len(root.files))
	}

	// Build directory tree from collected files
	fileList := make([]*MKVFile, 0, len(root.files))
	for _, f := range root.files {
		fileList = append(fileList, f)
	}
	root.rootDir = BuildDirectoryTree(fileList, verbose, readerFactory, permStore)

	// Clean up stale permission entries if we have a permission store
	if permStore != nil {
		validFiles, validDirs := root.collectValidPaths()
		removed := permStore.CleanupStale(validFiles, validDirs)
		if removed > 0 {
			if verbose {
				log.Printf("Cleaned up %d stale permission entries", removed)
			}
			if err := permStore.Save(); err != nil {
				log.Printf("Warning: failed to save permissions after cleanup: %v", err)
			}
		}
	}

	if verbose {
		log.Printf("Directory tree built with %d root entries", len(root.rootDir.files)+len(root.rootDir.subdirs))
	}

	return root, nil
}
