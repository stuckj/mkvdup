package testdata

import (
	"os"
	"testing"
)

func TestFind(t *testing.T) {
	p := Find()

	t.Logf("Test data root: %s", p.Root)
	t.Logf("ISO file: %s", p.ISOFile)
	t.Logf("MKV file: %s", p.MKVFile)
	t.Logf("Available: %v", p.Available)

	if !p.Available {
		t.Log("Test data not found. This is expected if you haven't set it up yet.")
		t.Log("See testdata/README.md for setup instructions.")
	}
}

func TestDataAvailable(t *testing.T) {
	p := SkipIfNotAvailable(t)

	// Verify paths exist
	if _, err := os.Stat(p.ISOFile); err != nil {
		t.Errorf("ISO file not found: %s", p.ISOFile)
	}

	if _, err := os.Stat(p.MKVFile); err != nil {
		t.Errorf("MKV file not found: %s", p.MKVFile)
	}

	// Log file sizes
	if info, err := os.Stat(p.ISOFile); err == nil {
		t.Logf("ISO size: %.2f GB", float64(info.Size())/(1024*1024*1024))
	}

	if info, err := os.Stat(p.MKVFile); err == nil {
		t.Logf("MKV size: %.2f GB", float64(info.Size())/(1024*1024*1024))
	}
}
