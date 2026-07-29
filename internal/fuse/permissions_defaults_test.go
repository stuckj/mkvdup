package fuse

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The headline bug: uid/gid 0 is root, and the default for every fstab mount,
// but the old plain-uint32 defaults could not tell 0 from "not specified", so
// the file could never pin it.
func TestDefaults_ZeroUIDIsRepresentable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permissions.yaml")
	if err := os.WriteFile(path, []byte(
		"defaults:\n  file_uid: 0\n  file_gid: 0\n  file_mode: 0444\n  dir_uid: 0\n  dir_gid: 0\n  dir_mode: 0555\n",
	), 0644); err != nil {
		t.Fatal(err)
	}

	// Flags say 1000; the file explicitly says 0 and must win.
	s := NewPermissionStore(path, Defaults{FileUID: 1000, FileGID: 1000, FileMode: 0444, DirUID: 1000, DirGID: 1000, DirMode: 0555}, false)
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}

	uid, gid, _ := s.GetFilePerms("anything")
	if uid != 0 || gid != 0 {
		t.Errorf("file said uid/gid 0 but store reports %d/%d", uid, gid)
	}
}

// An absent field must leave the --default-* flag value alone.
func TestDefaults_AbsentFieldKeepsFlagValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permissions.yaml")
	// Only file_mode present.
	if err := os.WriteFile(path, []byte("defaults:\n  file_mode: 0640\n"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewPermissionStore(path, Defaults{FileUID: 1000, FileGID: 1001, FileMode: 0444, DirUID: 1002, DirGID: 1003, DirMode: 0555}, false)
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}

	uid, gid, m := s.GetFilePerms("anything")
	if m != 0640 {
		t.Errorf("file_mode = %#o, want 0640 (from file)", m)
	}
	if uid != 1000 || gid != 1001 {
		t.Errorf("uid/gid = %d/%d, want 1000/1001 (from flags, absent in file)", uid, gid)
	}
	if _, _, dm := s.GetDirPerms("d"); dm != 0555 {
		t.Errorf("dir_mode = %#o, want 0555 (from flags)", dm)
	}
}

// Files written before this change stored modes as decimal. They must still load.
func TestDefaults_LegacyDecimalModesStillLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permissions.yaml")
	// 292 == 0444, 365 == 0555, 416 == 0640
	if err := os.WriteFile(path, []byte(
		"defaults:\n  file_mode: 292\n  dir_mode: 365\nfiles:\n  \"one.mkv\":\n    mode: 416\n",
	), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewPermissionStore(path, DefaultPerms(), false)
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}
	if _, _, m := s.GetFilePerms("one.mkv"); m != 0640 {
		t.Errorf("legacy decimal mode 416 loaded as %#o, want 0640", m)
	}
	if _, _, m := s.GetFilePerms("other.mkv"); m != 0444 {
		t.Errorf("legacy decimal file_mode 292 loaded as %#o, want 0444", m)
	}
	if _, _, m := s.GetDirPerms("d"); m != 0555 {
		t.Errorf("legacy decimal dir_mode 365 loaded as %#o, want 0555", m)
	}
}

// docs/FUSE.md has always said modes are stored in octal. Now they are.
func TestDefaults_ModesAreWrittenInOctal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permissions.yaml")
	s := NewPermissionStore(path, DefaultPerms(), false)
	if err := s.SetFilePerms("one.mkv", nil, nil, mode(0640)); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDirPerms("d", nil, nil, mode(0750)); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)

	for _, want := range []string{"mode: 0640", "mode: 0750", "file_mode: 0444", "dir_mode: 0555"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
	// No decimal leftovers: a bare mode value with no leading zero.
	if m := regexp.MustCompile(`(?m)mode: [1-9]\d*$`).FindString(body); m != "" {
		t.Errorf("mode written in decimal (%q):\n%s", m, body)
	}
	// Quoted modes do not decode back into integers, so they must not appear.
	if strings.Contains(body, `mode: "`) {
		t.Errorf("mode written as a quoted string, which will not load:\n%s", body)
	}
}

// Whatever is written must read back identically, octal and all.
func TestDefaults_OctalRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permissions.yaml")
	s := NewPermissionStore(path, Defaults{FileUID: 0, FileGID: 0, FileMode: 0444, DirUID: 0, DirGID: 0, DirMode: 0555}, false)
	if err := s.SetFilePerms("one.mkv", nil, nil, mode(0640)); err != nil {
		t.Fatal(err)
	}

	// Flags deliberately different, so anything inherited proves it came from disk.
	fresh := NewPermissionStore(path, Defaults{FileUID: 9, FileGID: 9, FileMode: 0400, DirUID: 9, DirGID: 9, DirMode: 0500}, false)
	if err := fresh.Load(); err != nil {
		t.Fatal(err)
	}

	uid, gid, m := fresh.GetFilePerms("one.mkv")
	if m != 0640 {
		t.Errorf("mode round-trip = %#o, want 0640", m)
	}
	if uid != 0 || gid != 0 {
		t.Errorf("defaults round-trip = %d/%d, want 0/0", uid, gid)
	}
	if _, _, dm := fresh.GetDirPerms("d"); dm != 0555 {
		t.Errorf("dir_mode round-trip = %#o, want 0555", dm)
	}
}
