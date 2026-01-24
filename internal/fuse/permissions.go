// Package fuse provides a FUSE filesystem for accessing deduplicated MKV files.
package fuse

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fuse"
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
	// Deep copy the maps to avoid data races during marshalling.
	// We copy both the map and the Perms values to ensure complete isolation.
	pf := permissionsFile{
		Defaults: s.defaults,
	}
	if s.files != nil {
		pf.Files = make(map[string]*Perms, len(s.files))
		for k, v := range s.files {
			if v != nil {
				permsCopy := *v // copy the Perms struct
				pf.Files[k] = &permsCopy
			}
		}
	}
	if s.dirs != nil {
		pf.Directories = make(map[string]*Perms, len(s.dirs))
		for k, v := range s.dirs {
			if v != nil {
				permsCopy := *v // copy the Perms struct
				pf.Directories[k] = &permsCopy
			}
		}
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

	// Only update non-nil values; copy values so the store owns their lifetime.
	if uid != nil {
		v := *uid
		p.UID = &v
	}
	if gid != nil {
		v := *gid
		p.GID = &v
	}
	if mode != nil {
		v := *mode
		p.Mode = &v
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

	// Only update non-nil values; copy values so the store owns their lifetime.
	if uid != nil {
		v := *uid
		p.UID = &v
	}
	if gid != nil {
		v := *gid
		p.GID = &v
	}
	if mode != nil {
		v := *mode
		p.Mode = &v
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
//  2. ~/.config/mkvdup/permissions.yaml (if exists) - for both root and non-root
//  3. /etc/mkvdup/permissions.yaml (if exists AND running as root)
//  4. Default based on euid: root uses /etc/, non-root uses ~/.config/
//
// Non-root users always get a user-writable path (unless explicitly overridden)
// to avoid EACCES errors when saving permission changes.
func ResolvePermissionsPath(explicitPath string) string {
	if explicitPath != "" {
		return explicitPath
	}

	home, err := os.UserHomeDir()
	userPath := ""
	if err == nil {
		userPath = filepath.Join(home, ".config", "mkvdup", "permissions.yaml")
	}

	// Check user config - takes priority for both root and non-root
	if userPath != "" {
		if _, err := os.Stat(userPath); err == nil {
			return userPath
		}
	}

	systemPath := "/etc/mkvdup/permissions.yaml"

	// For root: check system config, then default to system path
	if os.Geteuid() == 0 {
		if _, err := os.Stat(systemPath); err == nil {
			return systemPath
		}
		return systemPath
	}

	// For non-root: always use user path to ensure writability.
	// Do NOT use system path even if it exists, as non-root users
	// typically cannot write to /etc/ and chmod/chown operations
	// would fail with EACCES.
	if userPath != "" {
		return userPath
	}

	// Fallback if no home directory (unusual for non-root)
	return systemPath
}

// CallerInfo represents the calling process's credentials.
type CallerInfo struct {
	Uid uint32
	Gid uint32
}

// testCallerKey is used to inject caller credentials in tests.
type testCallerKeyType struct{}

var testCallerKey = testCallerKeyType{}

// GetCaller extracts caller credentials from the FUSE context.
// Falls back to test-injected caller if FUSE context unavailable.
// Returns root (uid=0, gid=0) as default for backwards compatibility.
func GetCaller(ctx context.Context) CallerInfo {
	if caller, ok := fuse.FromContext(ctx); ok {
		return CallerInfo{Uid: caller.Uid, Gid: caller.Gid}
	}
	// Check for test-injected caller
	if caller, ok := ctx.Value(testCallerKey).(CallerInfo); ok {
		return caller
	}
	// Default to root (maintains backwards compatibility with existing tests)
	return CallerInfo{Uid: 0, Gid: 0}
}

// ContextWithCaller creates a context with injected caller credentials for testing.
func ContextWithCaller(ctx context.Context, uid, gid uint32) context.Context {
	return context.WithValue(ctx, testCallerKey, CallerInfo{Uid: uid, Gid: gid})
}

// IsRoot returns true if the caller is root (uid 0).
func (c CallerInfo) IsRoot() bool {
	return c.Uid == 0
}

// AccessMode represents the type of access being requested.
type AccessMode uint32

const (
	AccessRead    AccessMode = 0004
	AccessWrite   AccessMode = 0002
	AccessExecute AccessMode = 0001
)

// CheckAccess verifies the caller has the requested access to a file or directory.
// Returns 0 if access is granted, syscall.EACCES if denied.
// Root (uid 0) bypasses all permission checks.
func CheckAccess(caller CallerInfo, fileUID, fileGID, mode uint32, access AccessMode) syscall.Errno {
	if caller.IsRoot() {
		return 0
	}

	var permBits uint32
	if caller.Uid == fileUID {
		// Owner bits
		permBits = (mode >> 6) & 0007
	} else if caller.Gid == fileGID {
		// Group bits
		permBits = (mode >> 3) & 0007
	} else {
		// Other bits
		permBits = mode & 0007
	}

	if permBits&uint32(access) != 0 {
		return 0
	}
	return syscall.EACCES
}

// CheckChown verifies the caller can change file ownership.
// Returns 0 if allowed, syscall.EPERM if denied.
// Only root can change UID. Root or file owner can change GID.
func CheckChown(caller CallerInfo, fileUID uint32, newUID, newGID *uint32) syscall.Errno {
	// Only root can change UID to a different user
	if newUID != nil && *newUID != fileUID && !caller.IsRoot() {
		return syscall.EPERM
	}

	// Only root or owner can change GID
	if newGID != nil && !caller.IsRoot() && caller.Uid != fileUID {
		return syscall.EPERM
	}

	return 0
}

// CheckChmod verifies the caller can change file mode.
// Returns 0 if allowed, syscall.EPERM if denied.
// Only root or file owner can chmod.
func CheckChmod(caller CallerInfo, fileUID uint32) syscall.Errno {
	if caller.IsRoot() || caller.Uid == fileUID {
		return 0
	}
	return syscall.EPERM
}
