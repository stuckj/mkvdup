package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectType_DVD(t *testing.T) {
	// Create temp directory with ISO file
	tmpDir := t.TempDir()
	isoPath := filepath.Join(tmpDir, "test.iso")
	if err := os.WriteFile(isoPath, []byte("fake iso content"), 0644); err != nil {
		t.Fatal(err)
	}

	sourceType, err := DetectType(tmpDir)
	if err != nil {
		t.Fatalf("DetectType() error = %v", err)
	}
	if sourceType != TypeDVD {
		t.Errorf("DetectType() = %v, want %v", sourceType, TypeDVD)
	}
}

func TestDetectType_DVDSubdirectory(t *testing.T) {
	// Create temp directory with ISO in subdirectory
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "disc")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	isoPath := filepath.Join(subDir, "test.iso")
	if err := os.WriteFile(isoPath, []byte("fake iso content"), 0644); err != nil {
		t.Fatal(err)
	}

	sourceType, err := DetectType(tmpDir)
	if err != nil {
		t.Fatalf("DetectType() error = %v", err)
	}
	if sourceType != TypeDVD {
		t.Errorf("DetectType() = %v, want %v", sourceType, TypeDVD)
	}
}

func TestDetectType_Bluray(t *testing.T) {
	// Create temp directory with Blu-ray structure
	tmpDir := t.TempDir()
	streamDir := filepath.Join(tmpDir, "BDMV", "STREAM")
	if err := os.MkdirAll(streamDir, 0755); err != nil {
		t.Fatal(err)
	}
	m2tsPath := filepath.Join(streamDir, "00001.m2ts")
	if err := os.WriteFile(m2tsPath, []byte("fake m2ts content"), 0644); err != nil {
		t.Fatal(err)
	}

	sourceType, err := DetectType(tmpDir)
	if err != nil {
		t.Fatalf("DetectType() error = %v", err)
	}
	if sourceType != TypeBluray {
		t.Errorf("DetectType() = %v, want %v", sourceType, TypeBluray)
	}
}

func TestDetectType_Unknown(t *testing.T) {
	// Create empty temp directory
	tmpDir := t.TempDir()

	_, err := DetectType(tmpDir)
	if err != ErrUnknownSourceType {
		t.Errorf("DetectType() error = %v, want %v", err, ErrUnknownSourceType)
	}
}

func TestTypeString(t *testing.T) {
	tests := []struct {
		t        Type
		expected string
	}{
		{TypeDVD, "DVD"},
		{TypeBluray, "Blu-ray"},
		{Type(99), "Unknown"},
	}

	for _, tt := range tests {
		if got := tt.t.String(); got != tt.expected {
			t.Errorf("Type(%d).String() = %v, want %v", tt.t, got, tt.expected)
		}
	}
}

func TestEnumerateMediaFiles_DVD(t *testing.T) {
	// Create temp directory with ISO files
	tmpDir := t.TempDir()
	iso1 := filepath.Join(tmpDir, "test1.iso")
	iso2 := filepath.Join(tmpDir, "test2.iso")
	if err := os.WriteFile(iso1, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(iso2, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := EnumerateMediaFiles(tmpDir, TypeDVD)
	if err != nil {
		t.Fatalf("EnumerateMediaFiles() error = %v", err)
	}

	if len(files) != 2 {
		t.Errorf("EnumerateMediaFiles() returned %d files, want 2", len(files))
	}
}

func TestEnumerateMediaFiles_Bluray(t *testing.T) {
	// Create temp directory with Blu-ray structure
	tmpDir := t.TempDir()
	streamDir := filepath.Join(tmpDir, "BDMV", "STREAM")
	if err := os.MkdirAll(streamDir, 0755); err != nil {
		t.Fatal(err)
	}
	m2ts1 := filepath.Join(streamDir, "00001.m2ts")
	m2ts2 := filepath.Join(streamDir, "00002.m2ts")
	if err := os.WriteFile(m2ts1, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m2ts2, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := EnumerateMediaFiles(tmpDir, TypeBluray)
	if err != nil {
		t.Fatalf("EnumerateMediaFiles() error = %v", err)
	}

	if len(files) != 2 {
		t.Errorf("EnumerateMediaFiles() returned %d files, want 2", len(files))
	}

	// Check relative paths
	for _, f := range files {
		if !filepath.IsAbs(f) && filepath.IsAbs(filepath.Join(tmpDir, f)) {
			// Good - it's a relative path
		} else if filepath.IsAbs(f) {
			t.Errorf("EnumerateMediaFiles() returned absolute path %s, want relative", f)
		}
	}
}

func TestGetFileInfo(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bin")
	content := []byte("hello world")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	size, err := GetFileInfo(testFile)
	if err != nil {
		t.Fatalf("GetFileInfo() error = %v", err)
	}
	if size != int64(len(content)) {
		t.Errorf("GetFileInfo() = %d, want %d", size, len(content))
	}
}

func TestGetFileInfo_NotFound(t *testing.T) {
	_, err := GetFileInfo("/nonexistent/file")
	if err == nil {
		t.Error("GetFileInfo() expected error for nonexistent file")
	}
}

func TestNewIndex(t *testing.T) {
	idx := NewIndex("/test/dir", TypeDVD, 64)

	if idx == nil {
		t.Fatal("NewIndex() returned nil")
	}
	if idx.SourceDir != "/test/dir" {
		t.Errorf("SourceDir = %q, want %q", idx.SourceDir, "/test/dir")
	}
	if idx.SourceType != TypeDVD {
		t.Errorf("SourceType = %v, want %v", idx.SourceType, TypeDVD)
	}
	if idx.WindowSize != 64 {
		t.Errorf("WindowSize = %d, want 64", idx.WindowSize)
	}
	if idx.HashToLocations == nil {
		t.Error("HashToLocations map not initialized")
	}
}

func TestIndex_Lookup(t *testing.T) {
	idx := NewIndex("/test/dir", TypeDVD, 64)

	// Add some test locations
	hash1 := uint64(12345)
	hash2 := uint64(67890)
	loc1 := Location{FileIndex: 0, Offset: 100, IsVideo: true}
	loc2 := Location{FileIndex: 0, Offset: 200, IsVideo: true}
	loc3 := Location{FileIndex: 1, Offset: 300, IsVideo: false}

	idx.HashToLocations[hash1] = []Location{loc1, loc2}
	idx.HashToLocations[hash2] = []Location{loc3}

	// Test lookup for existing hash
	locs := idx.Lookup(hash1)
	if len(locs) != 2 {
		t.Errorf("Lookup(hash1) returned %d locations, want 2", len(locs))
	}
	if locs[0].Offset != 100 || locs[1].Offset != 200 {
		t.Errorf("Lookup(hash1) returned wrong offsets")
	}

	// Test lookup for another hash
	locs = idx.Lookup(hash2)
	if len(locs) != 1 {
		t.Errorf("Lookup(hash2) returned %d locations, want 1", len(locs))
	}
	if locs[0].Offset != 300 {
		t.Errorf("Lookup(hash2) returned wrong offset: %d", locs[0].Offset)
	}

	// Test lookup for non-existent hash
	locs = idx.Lookup(99999)
	if len(locs) != 0 {
		t.Errorf("Lookup(99999) returned %d locations, want 0", len(locs))
	}
}

func TestDetectType_NonExistentDir(t *testing.T) {
	_, err := DetectType("/nonexistent/directory/path")
	if err == nil {
		t.Error("DetectType should fail for nonexistent directory")
	}
}

func TestDetectType_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Empty directory should return ErrUnknownSourceType
	_, err := DetectType(tmpDir)
	if err != ErrUnknownSourceType {
		t.Errorf("DetectType() error = %v, want %v", err, ErrUnknownSourceType)
	}
}

func TestEnumerateMediaFiles_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	// DVD: Empty directory should return empty list
	files, err := EnumerateMediaFiles(tmpDir, TypeDVD)
	if err != nil {
		t.Fatalf("EnumerateMediaFiles() error = %v", err)
	}
	if len(files) != 0 {
		t.Errorf("EnumerateMediaFiles() returned %d files, want 0", len(files))
	}

	// Blu-ray: Empty directory should return empty list
	files, err = EnumerateMediaFiles(tmpDir, TypeBluray)
	if err != nil {
		t.Fatalf("EnumerateMediaFiles() error = %v", err)
	}
	if len(files) != 0 {
		t.Errorf("EnumerateMediaFiles() returned %d files, want 0", len(files))
	}
}

func TestNewIndexer_UnknownType(t *testing.T) {
	// Create empty directory (unknown type)
	tmpDir := t.TempDir()

	_, err := NewIndexer(tmpDir, DefaultWindowSize)
	if err == nil {
		t.Error("NewIndexer should fail for unknown source type")
	}
}

func TestNewIndexer_WindowSizeClamping(t *testing.T) {
	// Create directory with ISO to pass detection
	tmpDir := t.TempDir()
	isoPath := filepath.Join(tmpDir, "test.iso")
	if err := os.WriteFile(isoPath, []byte("fake iso content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test window size below minimum
	idx, err := NewIndexerWithOptions(tmpDir, 10, true) // 10 < MinWindowSize
	if err != nil {
		t.Fatalf("NewIndexerWithOptions failed: %v", err)
	}
	if idx.windowSize != MinWindowSize {
		t.Errorf("windowSize = %d, want %d (clamped to min)", idx.windowSize, MinWindowSize)
	}

	// Test window size above maximum
	idx, err = NewIndexerWithOptions(tmpDir, 10000, true) // 10000 > MaxWindowSize
	if err != nil {
		t.Fatalf("NewIndexerWithOptions failed: %v", err)
	}
	if idx.windowSize != MaxWindowSize {
		t.Errorf("windowSize = %d, want %d (clamped to max)", idx.windowSize, MaxWindowSize)
	}
}

func TestIndexer_SourceTypeAndDir(t *testing.T) {
	tmpDir := t.TempDir()
	isoPath := filepath.Join(tmpDir, "test.iso")
	if err := os.WriteFile(isoPath, []byte("fake iso content"), 0644); err != nil {
		t.Fatal(err)
	}

	idx, err := NewIndexer(tmpDir, DefaultWindowSize)
	if err != nil {
		t.Fatalf("NewIndexer failed: %v", err)
	}

	if idx.SourceType() != TypeDVD {
		t.Errorf("SourceType() = %v, want %v", idx.SourceType(), TypeDVD)
	}

	if idx.SourceDir() != tmpDir {
		t.Errorf("SourceDir() = %q, want %q", idx.SourceDir(), tmpDir)
	}
}

func TestIndexer_Index(t *testing.T) {
	tmpDir := t.TempDir()
	isoPath := filepath.Join(tmpDir, "test.iso")
	if err := os.WriteFile(isoPath, []byte("fake iso content"), 0644); err != nil {
		t.Fatal(err)
	}

	idx, err := NewIndexer(tmpDir, DefaultWindowSize)
	if err != nil {
		t.Fatalf("NewIndexer failed: %v", err)
	}

	// Index should return non-nil even before Build
	if idx.Index() == nil {
		t.Error("Index() returned nil")
	}
}

func TestIndex_Close(t *testing.T) {
	idx := NewIndex("/test/dir", TypeDVD, 64)

	// Close on empty index should not panic
	if err := idx.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}

	// Close multiple times should be safe
	if err := idx.Close(); err != nil {
		t.Errorf("Second Close() failed: %v", err)
	}
}

func TestIndex_ReadRawDataAt_NoReader(t *testing.T) {
	idx := NewIndex("/test/dir", TypeDVD, 64)

	loc := Location{FileIndex: 0, Offset: 100}
	_, err := idx.ReadRawDataAt(loc, 64)
	if err == nil {
		t.Error("ReadRawDataAt should fail when no raw reader is available")
	}
}

func TestIndex_RawSlice_NoReader(t *testing.T) {
	idx := NewIndex("/test/dir", TypeDVD, 64)

	loc := Location{FileIndex: 0, Offset: 100}
	data := idx.RawSlice(loc, 64)
	if data != nil {
		t.Error("RawSlice should return nil when no raw reader is available")
	}
}

func TestIndex_ReadESDataAt_NoReader(t *testing.T) {
	idx := NewIndex("/test/dir", TypeDVD, 64)

	loc := Location{FileIndex: 0, Offset: 100, IsVideo: true}
	_, err := idx.ReadESDataAt(loc, 64)
	if err == nil {
		t.Error("ReadESDataAt should fail when no ES reader is available")
	}
}

func TestComputeHash(t *testing.T) {
	// Test that ComputeHash returns consistent results
	data := []byte("hello world")
	hash1 := ComputeHash(data)
	hash2 := ComputeHash(data)

	if hash1 != hash2 {
		t.Errorf("ComputeHash() inconsistent: got %d and %d", hash1, hash2)
	}

	// Different data should produce different hash
	otherData := []byte("different data")
	otherHash := ComputeHash(otherData)
	if hash1 == otherHash {
		t.Error("ComputeHash() produced same hash for different data")
	}
}

func TestComputeHash_Empty(t *testing.T) {
	// Empty data should produce a valid hash
	hash := ComputeHash([]byte{})
	if hash == 0 {
		t.Error("ComputeHash() of empty data should not be zero")
	}
}

func TestWindowSizeConstants(t *testing.T) {
	// Verify constant relationships
	if MinWindowSize >= MaxWindowSize {
		t.Errorf("MinWindowSize (%d) should be less than MaxWindowSize (%d)",
			MinWindowSize, MaxWindowSize)
	}
	if DefaultWindowSize < MinWindowSize || DefaultWindowSize > MaxWindowSize {
		t.Errorf("DefaultWindowSize (%d) should be between min (%d) and max (%d)",
			DefaultWindowSize, MinWindowSize, MaxWindowSize)
	}
}

func TestType_String_AllTypes(t *testing.T) {
	// Make sure all defined types have string representations
	tests := []struct {
		t        Type
		expected string
	}{
		{TypeDVD, "DVD"},
		{TypeBluray, "Blu-ray"},
		{Type(-1), "Unknown"},
		{Type(100), "Unknown"},
	}

	for _, tt := range tests {
		if got := tt.t.String(); got != tt.expected {
			t.Errorf("Type(%d).String() = %q, want %q", tt.t, got, tt.expected)
		}
	}
}

func TestIndex_LookupWithMultipleLocations(t *testing.T) {
	idx := NewIndex("/test/dir", TypeDVD, 64)

	// Add multiple locations for the same hash (hash collision scenario)
	hash := uint64(12345)
	idx.HashToLocations[hash] = []Location{
		{FileIndex: 0, Offset: 100, IsVideo: true},
		{FileIndex: 0, Offset: 500, IsVideo: true},
		{FileIndex: 1, Offset: 200, IsVideo: false},
	}

	locs := idx.Lookup(hash)
	if len(locs) != 3 {
		t.Errorf("Lookup returned %d locations, want 3", len(locs))
	}

	// Verify locations are returned in order
	expectedOffsets := []int64{100, 500, 200}
	for i, loc := range locs {
		if loc.Offset != expectedOffsets[i] {
			t.Errorf("Location %d offset = %d, want %d", i, loc.Offset, expectedOffsets[i])
		}
	}
}

func TestNewIndex_Fields(t *testing.T) {
	idx := NewIndex("/test/source", TypeBluray, 128)

	if idx.SourceDir != "/test/source" {
		t.Errorf("SourceDir = %q, want %q", idx.SourceDir, "/test/source")
	}
	if idx.SourceType != TypeBluray {
		t.Errorf("SourceType = %v, want %v", idx.SourceType, TypeBluray)
	}
	if idx.WindowSize != 128 {
		t.Errorf("WindowSize = %d, want 128", idx.WindowSize)
	}
	if idx.HashToLocations == nil {
		t.Error("HashToLocations should be initialized")
	}
	if len(idx.HashToLocations) != 0 {
		t.Errorf("HashToLocations should be empty, has %d entries", len(idx.HashToLocations))
	}
}
