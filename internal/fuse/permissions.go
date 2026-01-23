// Package fuse provides a FUSE filesystem for accessing deduplicated MKV files.
package fuse

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Perms holds uid, gid, and mode for a file or directory.
// Nil values indicate the field should inherit from defaults.
type Perms struct {
	UID  *uint32 `yaml:"uid,omitempty"`
	GID  *uint32 `yaml:"gid,omitempty"`
	Mode *uint32 `yaml:"mode,omitempty"`
}

// Defaults holds default permissions for files and directories.
type Defaults struct {
	FileUID  uint32 `yaml:"file_uid"`
	FileGID  uint32 `yaml:"file_gid"`
	FileMode uint32 `yaml:"file_mode"`
	DirUID   uint32 `yaml:"dir_uid"`
	DirGID   uint32 `yaml:"dir_gid"`
	DirMode  uint32 `yaml:"dir_mode"`
}

// DefaultPerms returns the default permission values.
func DefaultPerms() Defaults {
	return Defaults{
		FileUID:  0,
		FileGID:  0,
		FileMode: 0444,
		DirUID:   0,
		DirGID:   0,
		DirMode:  0555,
	}
}

// permissionsFile is the structure of the permissions YAML file.
type permissionsFile struct {
	Defaults    Defaults          `yaml:"defaults"`
	Files       map[string]*Perms `yaml:"files,omitempty"`
	Directories map[string]*Perms `yaml:"directories,omitempty"`
}

// PermissionStore manages file/directory permissions with persistence.
type PermissionStore struct {
	path     string
	defaults Defaults
	files    map[string]*Perms
	dirs     map[string]*Perms
	mu       sync.RWMutex
	verbose  bool
}

// NewPermissionStore creates a new permission store.
// If path is empty, permissions will not be persisted.
func NewPermissionStore(path string, defaults Defaults, verbose bool) *PermissionStore {
	return &PermissionStore{
		path:     path,
		defaults: defaults,
		files:    make(map[string]*Perms),
		dirs:     make(map[string]*Perms),
		verbose:  verbose,
	}
}

// Load loads permissions from the file.
// If the file doesn't exist, the store remains empty (using defaults).
func (s *PermissionStore) Load() error {
	if s.path == "" {
		return nil
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			if s.verbose {
				log.Printf("Permissions file %s does not exist, using defaults", s.path)
			}
			return nil
		}
		return fmt.Errorf("read permissions file: %w", err)
	}

	var pf permissionsFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return fmt.Errorf("parse permissions file: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Override defaults if specified in file
	if pf.Defaults.FileMode != 0 || pf.Defaults.FileUID != 0 || pf.Defaults.FileGID != 0 ||
		pf.Defaults.DirMode != 0 || pf.Defaults.DirUID != 0 || pf.Defaults.DirGID != 0 {
		// Only override non-zero values from file
		if pf.Defaults.FileMode != 0 {
			s.defaults.FileMode = pf.Defaults.FileMode
		}
		if pf.Defaults.FileUID != 0 {
			s.defaults.FileUID = pf.Defaults.FileUID
		}
		if pf.Defaults.FileGID != 0 {
			s.defaults.FileGID = pf.Defaults.FileGID
		}
		if pf.Defaults.DirMode != 0 {
			s.defaults.DirMode = pf.Defaults.DirMode
		}
		if pf.Defaults.DirUID != 0 {
			s.defaults.DirUID = pf.Defaults.DirUID
		}
		if pf.Defaults.DirGID != 0 {
			s.defaults.DirGID = pf.Defaults.DirGID
		}
	}

	if pf.Files != nil {
		s.files = pf.Files
	}
	if pf.Directories != nil {
		s.dirs = pf.Directories
	}

	if s.verbose {
		log.Printf("Loaded permissions: %d files, %d directories", len(s.files), len(s.dirs))
	}

	return nil
}

// Save saves permissions to the file.
func (s *PermissionStore) Save() error {
	if s.path == "" {
		return nil
	}

	s.mu.RLock()
	pf := permissionsFile{
		Defaults:    s.defaults,
		Files:       s.files,
		Directories: s.dirs,
	}
	s.mu.RUnlock()

	// Create parent directory if needed
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create permissions directory: %w", err)
	}

	data, err := yaml.Marshal(&pf)
	if err != nil {
		return fmt.Errorf("marshal permissions: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("write permissions file: %w", err)
	}

	if s.verbose {
		log.Printf("Saved permissions to %s", s.path)
	}

	return nil
}

// GetFilePerms returns the effective permissions for a file.
// Returns uid, gid, mode with defaults applied for any unset values.
func (s *PermissionStore) GetFilePerms(path string) (uid, gid, mode uint32) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	uid = s.defaults.FileUID
	gid = s.defaults.FileGID
	mode = s.defaults.FileMode

	if p, ok := s.files[path]; ok {
		if p.UID != nil {
			uid = *p.UID
		}
		if p.GID != nil {
			gid = *p.GID
		}
		if p.Mode != nil {
			mode = *p.Mode
		}
	}

	return uid, gid, mode
}

// GetDirPerms returns the effective permissions for a directory.
// Returns uid, gid, mode with defaults applied for any unset values.
func (s *PermissionStore) GetDirPerms(path string) (uid, gid, mode uint32) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	uid = s.defaults.DirUID
	gid = s.defaults.DirGID
	mode = s.defaults.DirMode

	if p, ok := s.dirs[path]; ok {
		if p.UID != nil {
			uid = *p.UID
		}
		if p.GID != nil {
			gid = *p.GID
		}
		if p.Mode != nil {
			mode = *p.Mode
		}
	}

	return uid, gid, mode
}

// SetFilePerms sets permissions for a file.
// Only non-nil values are updated; nil values leave existing values unchanged.
// Automatically saves to disk.
func (s *PermissionStore) SetFilePerms(path string, uid, gid *uint32, mode *uint32) error {
	s.mu.Lock()

	// If all values are nil, nothing to do
	if uid == nil && gid == nil && mode == nil {
		s.mu.Unlock()
		return nil
	}

	p, ok := s.files[path]
	if !ok {
		p = &Perms{}
		s.files[path] = p
	}

	// Only update non-nil values
	if uid != nil {
		p.UID = uid
	}
	if gid != nil {
		p.GID = gid
	}
	if mode != nil {
		p.Mode = mode
	}

	s.mu.Unlock()

	if s.verbose {
		log.Printf("SetFilePerms: %s uid=%v gid=%v mode=%v", path, uid, gid, mode)
	}

	return s.Save()
}

// RemoveFilePerms removes all permission overrides for a file.
// The file will use default permissions. Automatically saves to disk.
func (s *PermissionStore) RemoveFilePerms(path string) error {
	s.mu.Lock()
	delete(s.files, path)
	s.mu.Unlock()

	if s.verbose {
		log.Printf("RemoveFilePerms: %s", path)
	}

	return s.Save()
}

// SetDirPerms sets permissions for a directory.
// Only non-nil values are updated; nil values leave existing values unchanged.
// Automatically saves to disk.
func (s *PermissionStore) SetDirPerms(path string, uid, gid *uint32, mode *uint32) error {
	s.mu.Lock()

	// If all values are nil, nothing to do
	if uid == nil && gid == nil && mode == nil {
		s.mu.Unlock()
		return nil
	}

	p, ok := s.dirs[path]
	if !ok {
		p = &Perms{}
		s.dirs[path] = p
	}

	// Only update non-nil values
	if uid != nil {
		p.UID = uid
	}
	if gid != nil {
		p.GID = gid
	}
	if mode != nil {
		p.Mode = mode
	}

	s.mu.Unlock()

	if s.verbose {
		log.Printf("SetDirPerms: %s uid=%v gid=%v mode=%v", path, uid, gid, mode)
	}

	return s.Save()
}

// RemoveDirPerms removes all permission overrides for a directory.
// The directory will use default permissions. Automatically saves to disk.
func (s *PermissionStore) RemoveDirPerms(path string) error {
	s.mu.Lock()
	delete(s.dirs, path)
	s.mu.Unlock()

	if s.verbose {
		log.Printf("RemoveDirPerms: %s", path)
	}

	return s.Save()
}

// CleanupStale removes entries for paths that don't exist in the mounted filesystem.
// validFiles and validDirs are maps of valid paths (value is ignored, just checking keys).
// Returns the number of stale entries removed.
func (s *PermissionStore) CleanupStale(validFiles, validDirs map[string]bool) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := 0

	// Clean up stale file entries
	for path := range s.files {
		if !validFiles[path] {
			delete(s.files, path)
			removed++
			if s.verbose {
				log.Printf("Removed stale file permission entry: %s", path)
			}
		}
	}

	// Clean up stale directory entries
	for path := range s.dirs {
		if !validDirs[path] {
			delete(s.dirs, path)
			removed++
			if s.verbose {
				log.Printf("Removed stale directory permission entry: %s", path)
			}
		}
	}

	return removed
}

// Defaults returns the current default permissions.
func (s *PermissionStore) Defaults() Defaults {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.defaults
}

// ResolvePermissionsPath determines which permissions file to use.
// Priority:
//  1. explicitPath (from --permissions-file flag)
//  2. ~/.config/mkvdup/permissions.yaml (if exists)
//  3. /etc/mkvdup/permissions.yaml (if exists)
//  4. Default based on euid: root uses /etc/, non-root uses ~/.config/
func ResolvePermissionsPath(explicitPath string) string {
	if explicitPath != "" {
		return explicitPath
	}

	// Check user config
	home, err := os.UserHomeDir()
	if err == nil {
		userPath := filepath.Join(home, ".config", "mkvdup", "permissions.yaml")
		if _, err := os.Stat(userPath); err == nil {
			return userPath
		}
	}

	// Check system config
	systemPath := "/etc/mkvdup/permissions.yaml"
	if _, err := os.Stat(systemPath); err == nil {
		return systemPath
	}

	// Default based on euid
	if os.Geteuid() == 0 {
		return systemPath
	}

	if home != "" {
		return filepath.Join(home, ".config", "mkvdup", "permissions.yaml")
	}

	// Fallback if no home directory
	return systemPath
}
