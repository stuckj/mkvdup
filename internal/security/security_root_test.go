//go:build rootonly

// Tests that can only run as root. They live behind the `rootonly` build tag
// so that in a non-root context they are never scheduled, rather than being
// scheduled and skipped. A skipped test looks identical to a passing one in a
// green build, which is how these went unexecuted in CI entirely (see #201).
package security

import (
	"os"
	"testing"
)

// requireRoot fails rather than skips when the rootonly tag is built outside a
// root context. Skipping here would recreate the invisible-gap problem this
// tag exists to prevent.
func requireRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Fatal("tests built with the 'rootonly' tag must be run as root")
	}
}

func TestCheckFileOwnership_GroupWritable(t *testing.T) {
	requireRoot(t)

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
	requireRoot(t)

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

func TestCheckDirectory_RejectsNonDirectory(t *testing.T) {
	requireRoot(t)

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
