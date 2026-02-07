package mkv

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestParseTracksOnly_Basic(t *testing.T) {
	// Build a minimal MKV with EBML header + Segment + Tracks (reuse synthetic builder)
	data, _ := createSyntheticMKV(1, 1, 64)

	// Write to temp file
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mkv")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	parser, err := NewParser(path)
	if err != nil {
		t.Fatalf("NewParser error: %v", err)
	}
	defer parser.Close()

	if err := parser.ParseTracksOnly(); err != nil {
		t.Fatalf("ParseTracksOnly error: %v", err)
	}

	tracks := parser.Tracks()
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	if tracks[0].Number != 1 {
		t.Errorf("track number = %d, want 1", tracks[0].Number)
	}
	if tracks[0].Type != TrackTypeVideo {
		t.Errorf("track type = %d, want %d (video)", tracks[0].Type, TrackTypeVideo)
	}
	if tracks[0].CodecID != "V_MPEG4/ISO/AVC" {
		t.Errorf("codec ID = %q, want %q", tracks[0].CodecID, "V_MPEG4/ISO/AVC")
	}

	// Packets should NOT be populated (ParseTracksOnly skips clusters)
	if len(parser.Packets()) != 0 {
		t.Errorf("expected 0 packets from ParseTracksOnly, got %d", len(parser.Packets()))
	}
}

func TestParseTracksOnly_NoTracks(t *testing.T) {
	// Build MKV with EBML header + Segment but no Tracks element
	var buf bytes.Buffer

	// EBML Header
	ebmlHeaderData := []byte{
		0x42, 0x86, 0x81, 0x01, // EBMLVersion = 1
		0x42, 0xF7, 0x81, 0x01, // EBMLReadVersion = 1
		0x42, 0xF2, 0x81, 0x04, // EBMLMaxIDLength = 4
		0x42, 0xF3, 0x81, 0x08, // EBMLMaxSizeLength = 8
		0x42, 0x82, 0x88, 'm', 'a', 't', 'r', 'o', 's', 'k', 'a',
		0x42, 0x87, 0x81, 0x04,
		0x42, 0x85, 0x81, 0x02,
	}
	buf.Write(encodeElementID(IDEBMLHeader))
	buf.Write(encodeVINT(uint64(len(ebmlHeaderData))))
	buf.Write(ebmlHeaderData)

	// Empty Segment (just a SeekHead with 0 bytes)
	segmentData := []byte{}
	buf.Write(encodeElementID(IDSegment))
	buf.Write(encodeVINT(uint64(len(segmentData))))
	buf.Write(segmentData)

	dir := t.TempDir()
	path := filepath.Join(dir, "test.mkv")
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	parser, err := NewParser(path)
	if err != nil {
		t.Fatalf("NewParser error: %v", err)
	}
	defer parser.Close()

	err = parser.ParseTracksOnly()
	if err == nil {
		t.Error("expected error for MKV with no Tracks element, got nil")
	}
}

func TestParseTracksOnly_UnknownSizeElement(t *testing.T) {
	// Build MKV where an element before Tracks has unknown size (-1).
	// ParseTracksOnly should return an error rather than descending into it.
	var buf bytes.Buffer

	// EBML Header
	ebmlHeaderData := []byte{
		0x42, 0x86, 0x81, 0x01,
		0x42, 0xF7, 0x81, 0x01,
		0x42, 0xF2, 0x81, 0x04,
		0x42, 0xF3, 0x81, 0x08,
		0x42, 0x82, 0x88, 'm', 'a', 't', 'r', 'o', 's', 'k', 'a',
		0x42, 0x87, 0x81, 0x04,
		0x42, 0x85, 0x81, 0x02,
	}
	buf.Write(encodeElementID(IDEBMLHeader))
	buf.Write(encodeVINT(uint64(len(ebmlHeaderData))))
	buf.Write(ebmlHeaderData)

	// Segment with unknown size (0x01FFFFFFFFFFFFFF)
	buf.Write(encodeElementID(IDSegment))
	buf.Write([]byte{0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}) // unknown size

	// SeekHead element with unknown size (should trigger error)
	buf.Write(encodeElementID(IDSeekHead))
	buf.Write([]byte{0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}) // unknown size

	// Pad with enough data to prevent read errors
	buf.Write(make([]byte, 256))

	dir := t.TempDir()
	path := filepath.Join(dir, "test.mkv")
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	parser, err := NewParser(path)
	if err != nil {
		t.Fatalf("NewParser error: %v", err)
	}
	defer parser.Close()

	err = parser.ParseTracksOnly()
	if err == nil {
		t.Error("expected error for unknown-size element before Tracks, got nil")
	}
}
