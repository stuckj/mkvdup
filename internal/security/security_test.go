package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckFileOwnership_SkipsWhenNotRoot(t *testing.T) {
	old := Geteuid
	defer func() { Geteuid = old }()
	Geteuid = func() int { return 1000 }

	// Should pass even for nonexistent file since check is skipped
	if err := CheckFileOwnership("/nonexistent/file"); err != nil {
		t.Fatalf("expected nil when not root, got: %v", err)
	}
}

func TestCheckFileOwnership_RootMode(t *testing.T) {
	old := Geteuid
	defer func() { Geteuid = old }()
	Geteuid = func() int { return 0 }

	// Create temp file - will be owned by current user
	f, err := os.CreateTemp(t.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	if os.Geteuid() != 0 {
		// Non-root: file is owned by current user, should fail
		err := CheckFileOwnership(f.Name())
		if err == nil {
			t.Fatal("expected error for non-root-owned file")
		}
		t.Logf("got expected error: %v", err)
	} else {
		// Root: file is owned by root, should pass
		if err := CheckFileOwnership(f.Name()); err != nil {
			t.Fatalf("expected nil for root-owned file, got: %v", err)
		}
	}
}

func TestCheckFileOwnership_GroupWritable(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}

	old := Geteuid
	defer func() { Geteuid = old }()
	Geteuid = func() int { return 0 }

	f, err := os.CreateTemp(t.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := os.Chmod(f.Name(), 0664); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	err = CheckFileOwnership(f.Name())
	if err == nil {
		t.Fatal("expected error for group-writable file")
	}
}

func TestCheckFileOwnership_WorldWritable(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}

	old := Geteuid
	defer func() { Geteuid = old }()
	Geteuid = func() int { return 0 }

	f, err := os.CreateTemp(t.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := os.Chmod(f.Name(), 0646); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	err = CheckFileOwnership(f.Name())
	if err == nil {
		t.Fatal("expected error for world-writable file")
	}
}

func TestCheckFileOwnership_ResolvesSymlinks(t *testing.T) {
	old := Geteuid
	defer func() { Geteuid = old }()
	Geteuid = func() int { return 0 }

	dir := t.TempDir()

	// Create a target file (owned by current user)
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink to the target
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if os.Geteuid() != 0 {
		// Non-root: symlink target is owned by current user, should fail
		err := CheckFileOwnership(link)
		if err == nil {
			t.Fatal("expected error: symlink target is not root-owned")
		}
	} else {
		// Root: symlink target is owned by root, should pass
		if err := CheckFileOwnership(link); err != nil {
			t.Fatalf("expected nil for root-owned symlink target, got: %v", err)
		}
	}
}

func TestCheckPathConfinement_AbsolutePathBlocked(t *testing.T) {
	// Absolute relPath should be rejected regardless of euid
	for _, euid := range []int{0, 1000} {
		old := Geteuid
		Geteuid = func() int { return euid }
		_, err := CheckPathConfinement("/some/dir", "/etc/shadow")
		Geteuid = old
		if err == nil {
			t.Fatalf("euid=%d: expected error for absolute relPath, got nil", euid)
		}
	}
}

func TestCheckPathConfinement_SkipsWhenNotRoot(t *testing.T) {
	old := Geteuid
	defer func() { Geteuid = old }()
	Geteuid = func() int { return 1000 }

	path, err := CheckPathConfinement("/some/dir", "../../etc/shadow")
	if err != nil {
		t.Fatalf("expected nil when not root, got: %v", err)
	}
	// Should return simple joined path
	expected := filepath.Join("/some/dir", "../../etc/shadow")
	if path != expected {
		t.Fatalf("expected %s, got %s", expected, path)
	}
}

func TestCheckPathConfinement_ValidPath(t *testing.T) {
	old := Geteuid
	defer func() { Geteuid = old }()
	Geteuid = func() int { return 0 }

	// Create a real directory structure
	dir := t.TempDir()
	subdir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	f, err := os.Create(filepath.Join(subdir, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	path, err := CheckPathConfinement(dir, "subdir/file.txt")
	if err != nil {
		t.Fatalf("expected valid path, got error: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("expected absolute path, got: %s", path)
	}
}

func TestCheckPathConfinement_TraversalBlocked(t *testing.T) {
	old := Geteuid
	defer func() { Geteuid = old }()
	Geteuid = func() int { return 0 }

	dir := t.TempDir()

	_, err := CheckPathConfinement(dir, "../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestCheckPathConfinement_SymlinkBlocked(t *testing.T) {
	old := Geteuid
	defer func() { Geteuid = old }()
	Geteuid = func() int { return 0 }

	dir := t.TempDir()
	// Create a symlink that points outside the source dir
	link := filepath.Join(dir, "escape")
	if err := os.Symlink("/etc", link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	_, err := CheckPathConfinement(dir, "escape/passwd")
	if err == nil {
		t.Fatal("expected error for symlink escape")
	}
}

func TestCheckPathConfinement_PrefixAttackBlocked(t *testing.T) {
	old := Geteuid
	defer func() { Geteuid = old }()
	Geteuid = func() int { return 0 }

	// Create two dirs where one is a prefix of the other
	parent := t.TempDir()
	sourceDir := filepath.Join(parent, "source")
	evilDir := filepath.Join(parent, "source-evil")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.MkdirAll(evilDir, 0755); err != nil {
		t.Fatalf("mkdir evil: %v", err)
	}

	// Create a file in the evil dir
	f, err := os.Create(filepath.Join(evilDir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Create symlink inside sourceDir pointing to evil dir
	link := filepath.Join(sourceDir, "link")
	if err := os.Symlink(evilDir, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	_, err = CheckPathConfinement(sourceDir, "link/data")
	if err == nil {
		t.Fatal("expected error for prefix attack via symlink")
	}
}

func TestCheckDirectory_SkipsWhenNotRoot(t *testing.T) {
	old := Geteuid
	defer func() { Geteuid = old }()
	Geteuid = func() int { return 1000 }

	if err := CheckDirectory("/nonexistent"); err != nil {
		t.Fatalf("expected nil when not root, got: %v", err)
	}
}

func TestCheckDirectory_RejectsNonDirectory(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}

	old := Geteuid
	defer func() { Geteuid = old }()
	Geteuid = func() int { return 0 }

	// Create a regular file
	f, err := os.CreateTemp(t.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	err = CheckDirectory(f.Name())
	if err == nil {
		t.Fatal("expected error for non-directory path")
	}
}
