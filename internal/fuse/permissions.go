// Package fuse provides a FUSE filesystem for accessing deduplicated MKV files.
package fuse

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/user"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fuse"
	"gopkg.in/yaml.v3"
)

// Perms holds uid, gid, mode, and an optional modification-time override for a
// file or directory. Nil uid/gid/mode inherit from defaults; a nil Mtime means
// the timestamp is derived (from the dedup file for files, or from mount time
// and entry add/remove events for directories) rather than overridden.
type Perms struct {
	UID  *uint32 `yaml:"uid,omitempty"`
	GID  *uint32 `yaml:"gid,omitempty"`
	Mode *uint32 `yaml:"mode,omitempty"`
	// Mtime is an explicit modification-time override in Unix seconds, set via
	// touch/utimes. Only mtime is tracked; atime is reported equal to mtime.
	Mtime *int64 `yaml:"mtime,omitempty"`
}

// isEmpty reports whether no field is overridden, meaning the entry carries no
// information and can be dropped rather than persisted as an empty map.
func (p *Perms) isEmpty() bool {
	return p.UID == nil && p.GID == nil && p.Mode == nil && p.Mtime == nil
}

// The optional-override setters take pointers, so logging them with %v prints
// an address rather than a value. These render the value (or "unset") instead.

func logU32(p *uint32) string {
	if p == nil {
		return "unset"
	}
	return strconv.FormatUint(uint64(*p), 10)
}

func logMode(p *uint32) string {
	if p == nil {
		return "unset"
	}
	return "0" + strconv.FormatUint(uint64(*p), 8)
}

func logI64(p *int64) string {
	if p == nil {
		return "unset"
	}
	return strconv.FormatInt(*p, 10)
}

// octalMode is a permission mode that marshals as familiar octal (0644) rather
// than the decimal (420) previous versions wrote — which docs/FUSE.md has always
// claimed was octal.
//
// Decoding is unaffected: YAML reads both an unquoted 0644 (leading zero means
// octal) and a bare 420, so files written by older versions still load. Note a
// *quoted* "0644" does not decode into an integer at all, which is why this
// emits an unquoted !!int node rather than a string.
type octalMode uint32

func (m octalMode) MarshalYAML() (any, error) {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!int",
		Value: "0" + strconv.FormatUint(uint64(m), 8),
	}, nil
}

// MarshalYAML writes Mode in octal while leaving the in-memory type a plain
// *uint32, so callers are unaffected.
func (p Perms) MarshalYAML() (any, error) {
	type entry struct {
		UID   *uint32    `yaml:"uid,omitempty"`
		GID   *uint32    `yaml:"gid,omitempty"`
		Mode  *octalMode `yaml:"mode,omitempty"`
		Mtime *int64     `yaml:"mtime,omitempty"`
	}
	var mode *octalMode
	if p.Mode != nil {
		m := octalMode(*p.Mode)
		mode = &m
	}
	return entry{UID: p.UID, GID: p.GID, Mode: mode, Mtime: p.Mtime}, nil
}

// Defaults holds the effective default permissions for files and directories.
// Every field is always populated, so plain uint32 is right here — the
// "unspecified" case only exists in the file, see fileDefaults.
type Defaults struct {
	FileUID  uint32 `yaml:"file_uid"`
	FileGID  uint32 `yaml:"file_gid"`
	FileMode uint32 `yaml:"file_mode"`
	DirUID   uint32 `yaml:"dir_uid"`
	DirGID   uint32 `yaml:"dir_gid"`
	DirMode  uint32 `yaml:"dir_mode"`
}

// fileDefaults is the on-disk form of Defaults, where a field may legitimately
// be absent.
//
// The pointers exist to distinguish "not specified" from a literal 0. With
// plain uint32 they were the same value, so a uid or gid of 0 — root, and the
// default for every fstab mount, since mount.fuse.mkvdup runs as root — could
// not be written to the file at all: it was read back as "unspecified" and
// silently discarded.
type fileDefaults struct {
	FileUID  *uint32    `yaml:"file_uid,omitempty"`
	FileGID  *uint32    `yaml:"file_gid,omitempty"`
	FileMode *octalMode `yaml:"file_mode,omitempty"`
	DirUID   *uint32    `yaml:"dir_uid,omitempty"`
	DirGID   *uint32    `yaml:"dir_gid,omitempty"`
	DirMode  *octalMode `yaml:"dir_mode,omitempty"`
}

// toFileDefaults renders the effective defaults for writing. All six fields are
// known, so all six are emitted — the file then records exactly what the mount
// used, including zeros.
func toFileDefaults(d Defaults) fileDefaults {
	fileMode, dirMode := octalMode(d.FileMode), octalMode(d.DirMode)
	fileUID, fileGID := d.FileUID, d.FileGID
	dirUID, dirGID := d.DirUID, d.DirGID
	return fileDefaults{
		FileUID:  &fileUID,
		FileGID:  &fileGID,
		FileMode: &fileMode,
		DirUID:   &dirUID,
		DirGID:   &dirGID,
		DirMode:  &dirMode,
	}
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

// applyLoadedDefaults overlays defaults read from a permissions file onto the
// in-memory defaults, which start from the --default-* flags.
//
// A present field wins, including an explicit 0. Absent fields leave the flag
// value alone.
func applyLoadedDefaults(dst *Defaults, src fileDefaults) {
	if src.FileMode != nil {
		dst.FileMode = uint32(*src.FileMode)
	}
	if src.FileUID != nil {
		dst.FileUID = *src.FileUID
	}
	if src.FileGID != nil {
		dst.FileGID = *src.FileGID
	}
	if src.DirMode != nil {
		dst.DirMode = uint32(*src.DirMode)
	}
	if src.DirUID != nil {
		dst.DirUID = *src.DirUID
	}
	if src.DirGID != nil {
		dst.DirGID = *src.DirGID
	}
}

// permissionsFile is the structure of the permissions YAML file.
type permissionsFile struct {
	// Mount records which mountpoint owns this file. Keys are virtual paths
	// relative to a mount root and mean nothing without it, so the stamp is what
	// lets a daemon notice it is looking at another mount's file.
	Mount       string            `yaml:"mount,omitempty"`
	Defaults    fileDefaults      `yaml:"defaults"`
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

	// mount is this store's canonical mountpoint, used to stamp the file and to
	// detect that another mount owns it. Empty means "unknown" (tests,
	// programmatic use), which disables both stamping and the check.
	mount string

	// shared is set when the file carries a different mount's stamp, i.e. two
	// mounts were deliberately pointed at one permissions_file=. The store then
	// degrades safely rather than treating the file as its own: see sharedMode.
	shared bool

	// pending holds the entries whose in-memory value has not reached the file
	// yet. Writes are debounced (see permissions_flush.go) because each one
	// costs a lock, a parse and two fsyncs, which a recursive chmod would
	// otherwise pay per file.
	pending    map[permKey]struct{}
	flushTimer *time.Timer
	firstDirty time.Time
	flushErr   error
	closed     bool
}

// SetMountIdentity records which mountpoint this store belongs to, so its file
// can be stamped and a foreign file recognised. Call before Load.
func (s *PermissionStore) SetMountIdentity(mountpoint string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mount = CanonicalMountpoint(mountpoint)
}

// sharedMode reports whether this store is looking at a file stamped by a
// different mount.
//
// Isolation cannot reach this case: an explicit --permissions-file may
// deliberately point two mounts at one file. Rather than silently corrupting
// it, the store stops doing the two things that assume sole ownership --
// deleting entries it cannot account for, and persisting its own defaults over
// the file's. Key collisions remain possible and are the operator's choice.
func (s *PermissionStore) sharedMode() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.shared
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

	// Load replaces the in-memory maps with the file's contents, so anything
	// still sitting in the debounce window would simply vanish -- a chmod
	// moments before a SIGHUP reload would be silently discarded. Flushing here
	// rather than at each call site means the ordering cannot be forgotten.
	// A flush failure is not fatal: log it and load anyway, since refusing to
	// reload would be worse than losing an unpersisted change.
	if err := s.Flush(); err != nil {
		log.Printf("Warning: failed to flush pending permission changes before reloading %s: %v", s.path, err)
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

	// A stamp naming a different mount means this file is shared. Warn once and
	// degrade: the keys in it belong to someone else's virtual tree, so we must
	// not prune them or overwrite their defaults.
	wasShared := s.shared
	s.shared = pf.Mount != "" && s.mount != "" && pf.Mount != s.mount
	if s.shared && !wasShared {
		log.Printf("Warning: permissions file %s is stamped for mount %q but this mount is %q; "+
			"treating it as shared (stale-entry cleanup disabled, defaults not persisted). "+
			"Give each mount its own permissions file to avoid key collisions.",
			s.path, pf.Mount, s.mount)
	}

	applyLoadedDefaults(&s.defaults, pf.Defaults)

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

// Save writes the whole in-memory state to the file, under the file lock and
// via an atomic rename.
//
// Unlike the mutators, this deliberately does *not* merge with what is on disk:
// it is a "publish my state" operation, used after seeding and after stale-entry
// cleanup, where the in-memory state is the intended result. Individual
// permission changes go through withFileLock instead, which applies a delta to
// the current file contents and so cannot clobber a concurrent writer.
func (s *PermissionStore) Save() error {
	if s.path == "" {
		return nil
	}

	lock, err := acquireFileLock(lockPath(s.path))
	if err != nil {
		return err
	}
	defer lock.Close()

	s.mu.RLock()
	// Deep copy so marshalling cannot race with a concurrent mutation.
	pf := s.snapshotLocked()
	s.mu.RUnlock()

	// In shared mode the stamp belongs to the mount that created the file;
	// republishing without it would hide the sharing from the next Load.
	if s.sharedMode() {
		if onDisk, err := readPermissionsFile(s.path); err == nil {
			pf.Mount = onDisk.Mount
		}
	}

	data, err := yaml.Marshal(&pf)
	if err != nil {
		return fmt.Errorf("marshal permissions: %w", err)
	}

	if err := writeFileAtomic(s.path, data, 0644); err != nil {
		return err
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

// GetFileMtimeOverride returns the explicit mtime override (Unix seconds) for a
// file, or nil if none is set (in which case the derived mtime is used).
func (s *PermissionStore) GetFileMtimeOverride(path string) *int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.files[path]; ok && p.Mtime != nil {
		m := *p.Mtime
		return &m
	}
	return nil
}

// GetDirMtimeOverride returns the explicit mtime override (Unix seconds) for a
// directory, or nil if none is set.
func (s *PermissionStore) GetDirMtimeOverride(path string) *int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.dirs[path]; ok && p.Mtime != nil {
		m := *p.Mtime
		return &m
	}
	return nil
}

// applyOwnership overlays non-nil uid/gid/mode onto an entry, copying the
// values so the store owns their lifetime.
func applyOwnership(p *Perms, uid, gid, mode *uint32) {
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
}

// SetFilePerms sets permissions for a file.
// Only non-nil values are updated; nil values leave existing values unchanged.
// Persisted immediately, as a locked read-modify-write of the file.
func (s *PermissionStore) SetFilePerms(path string, uid, gid *uint32, mode *uint32) error {
	// If all values are nil, nothing to do
	if uid == nil && gid == nil && mode == nil {
		return nil
	}

	if s.verbose {
		log.Printf("SetFilePerms: %s uid=%s gid=%s mode=%s", path, logU32(uid), logU32(gid), logMode(mode))
	}

	s.mu.Lock()
	applyOwnership(entryFor(&s.files, path), uid, gid, mode)
	s.markDirtyLocked(permKey{path: path})
	s.mu.Unlock()

	return s.latchedError()
}

// RemoveFilePerms removes all permission overrides for a file.
// The file will use default permissions. Automatically saves to disk.
func (s *PermissionStore) RemoveFilePerms(path string) error {
	if s.verbose {
		log.Printf("RemoveFilePerms: %s", path)
	}

	s.mu.Lock()
	delete(s.files, path)
	s.markDirtyLocked(permKey{path: path})
	s.mu.Unlock()

	return s.latchedError()
}

// SetDirPerms sets permissions for a directory.
// Only non-nil values are updated; nil values leave existing values unchanged.
// Persisted immediately, as a locked read-modify-write of the file.
func (s *PermissionStore) SetDirPerms(path string, uid, gid *uint32, mode *uint32) error {
	// If all values are nil, nothing to do
	if uid == nil && gid == nil && mode == nil {
		return nil
	}

	if s.verbose {
		log.Printf("SetDirPerms: %s uid=%s gid=%s mode=%s", path, logU32(uid), logU32(gid), logMode(mode))
	}

	s.mu.Lock()
	applyOwnership(entryFor(&s.dirs, path), uid, gid, mode)
	s.markDirtyLocked(permKey{dir: true, path: path})
	s.mu.Unlock()

	return s.latchedError()
}

// RemoveDirPerms removes all permission overrides for a directory.
// The directory will use default permissions. Automatically saves to disk.
func (s *PermissionStore) RemoveDirPerms(path string) error {
	if s.verbose {
		log.Printf("RemoveDirPerms: %s", path)
	}

	s.mu.Lock()
	delete(s.dirs, path)
	s.markDirtyLocked(permKey{dir: true, path: path})
	s.mu.Unlock()

	return s.latchedError()
}

// SetFileMtime sets (or, when mtime is nil, clears) the modification-time
// override for a file. Existing uid/gid/mode overrides are preserved.
// Automatically saves to disk.
func (s *PermissionStore) SetFileMtime(path string, mtime *int64) error {
	if s.verbose {
		log.Printf("SetFileMtime: %s mtime=%s", path, logI64(mtime))
	}

	s.mu.Lock()
	applyMtime(&s.files, path, mtime)
	s.markDirtyLocked(permKey{path: path})
	s.mu.Unlock()

	return s.latchedError()
}

// SetDirMtime sets (or, when mtime is nil, clears) the modification-time
// override for a directory. Existing uid/gid/mode overrides are preserved.
// Persisted immediately, as a locked read-modify-write of the file.
func (s *PermissionStore) SetDirMtime(path string, mtime *int64) error {
	if s.verbose {
		log.Printf("SetDirMtime: %s mtime=%s", path, logI64(mtime))
	}

	s.mu.Lock()
	applyMtime(&s.dirs, path, mtime)
	s.markDirtyLocked(permKey{dir: true, path: path})
	s.mu.Unlock()

	return s.latchedError()
}

// applyMtime sets or clears the mtime override for path, leaving any
// uid/gid/mode overrides alone.
func applyMtime(m *map[string]*Perms, path string, mtime *int64) {
	if mtime == nil {
		// Nothing to clear if there is no entry.
		if *m == nil {
			return
		}
		p, ok := (*m)[path]
		if !ok || p == nil {
			return
		}
		p.Mtime = nil
		// Drop the entry entirely if nothing is overridden anymore, so we don't
		// persist a useless "path: {}" stanza in the permissions file.
		if p.isEmpty() {
			delete(*m, path)
		}
		return
	}

	v := *mtime
	entryFor(m, path).Mtime = &v
}

// CleanupStale removes entries for paths that don't exist in the mounted filesystem.
// validFiles and validDirs are maps of valid paths (value is ignored, just checking keys).
// Returns the number of stale entries removed.
func (s *PermissionStore) CleanupStale(validFiles, validDirs map[string]bool) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	// In shared mode the file holds another mount's entries, whose paths are
	// meaningless against this mount's tree. Pruning "unknown" keys there is
	// exactly the deletion bug this change exists to fix, so do nothing.
	if s.shared {
		if s.verbose {
			log.Printf("Skipping stale-entry cleanup: %s is shared with mount %q", s.path, s.mount)
		}
		return 0
	}

	removed := 0

	// Clean up stale file entries
	for path := range s.files {
		if !validFiles[path] {
			delete(s.files, path)
			s.markDirtyLocked(permKey{path: path})
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
			s.markDirtyLocked(permKey{dir: true, path: path})
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

// ResolvePermissionsPath now lives in permissions_path.go, alongside the
// mountpoint-identity helpers it depends on.

// CallerInfo represents the calling process's credentials.
type CallerInfo struct {
	Uid uint32
	Gid uint32
}

// testCallerHook is set by test code to allow injecting caller credentials.
// This is nil in production, ensuring only real FUSE contexts are trusted.
var testCallerHook func(context.Context) (CallerInfo, bool)

// GetCaller extracts caller credentials from the FUSE context.
// Returns (caller, true) if credentials are available, (zero, false) otherwise.
// Callers should deny access when ok is false to fail closed.
func GetCaller(ctx context.Context) (CallerInfo, bool) {
	if caller, ok := fuse.FromContext(ctx); ok {
		return CallerInfo{Uid: caller.Uid, Gid: caller.Gid}, true
	}
	// Check for test-injected caller (only available in tests)
	if testCallerHook != nil {
		if caller, ok := testCallerHook(ctx); ok {
			return caller, true
		}
	}
	// Fail closed: return zero value and false to indicate no credentials
	return CallerInfo{}, false
}

// IsRoot returns true if the caller is root (uid 0).
func (c CallerInfo) IsRoot() bool {
	return c.Uid == 0
}

// CheckChown verifies the caller can change file ownership.
// Returns 0 if allowed, syscall.EPERM if denied.
// Only root can change UID. Only root or file owner can change GID.
// Non-root owners can change GID to any group they are a member of
// (primary or supplementary). No-op changes (newUID == fileUID or
// newGID == fileGID) are always allowed.
func CheckChown(caller CallerInfo, fileUID, fileGID uint32, newUID, newGID *uint32) syscall.Errno {
	// Only root can change UID to a different user
	if newUID != nil && *newUID != fileUID && !caller.IsRoot() {
		return syscall.EPERM
	}

	// GID changes:
	// - No-op (nil or same as current) is always allowed
	// - Root can change to any GID
	// - Non-root owner can change to any group they belong to
	if newGID != nil && *newGID != fileGID {
		if caller.IsRoot() {
			return 0
		}
		// Non-root: must be owner AND must be a member of target group
		if caller.Uid != fileUID || !isGroupMember(caller.Uid, caller.Gid, *newGID) {
			return syscall.EPERM
		}
	}

	return 0
}

// groupMembershipFunc is the function used to check group membership.
// It can be overridden in tests to avoid OS-level lookups.
var groupMembershipFunc = defaultGroupMembership

// isGroupMember checks if a user is a member of the given group.
// This checks the primary GID and supplementary groups.
func isGroupMember(uid, primaryGID, targetGID uint32) bool {
	return groupMembershipFunc(uid, primaryGID, targetGID)
}

// defaultGroupMembership checks group membership by looking up the user's
// groups from the OS.
func defaultGroupMembership(uid, primaryGID, targetGID uint32) bool {
	// Primary GID is always a member
	if targetGID == primaryGID {
		return true
	}

	// Look up supplementary groups from the OS
	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return false
	}
	groupIDs, err := u.GroupIds()
	if err != nil {
		return false
	}
	targetStr := strconv.FormatUint(uint64(targetGID), 10)
	for _, gid := range groupIDs {
		if gid == targetStr {
			return true
		}
	}
	return false
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
