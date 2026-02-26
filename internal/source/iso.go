package source

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// errNotISO9660 is returned when the image lacks a valid ISO9660 PVD,
// signaling the caller to try an alternative filesystem (e.g. UDF).
var errNotISO9660 = errors.New("not an ISO9660 image")

const isoSectorSize = 2048

// isoFileExtent represents a file within an ISO9660 filesystem.
type isoFileExtent struct {
	Name    string             // filename (uppercase, no version suffix)
	Offset  int64              // byte offset in ISO (first extent)
	Size    int64              // data length in bytes
	IsDir   bool               // true if this is a directory entry
	Extents []isoPhysicalRange // non-nil for multi-extent UDF files
}

// isoPhysicalRange describes one contiguous physical region within an ISO.
type isoPhysicalRange struct {
	ISOOffset int64 // byte offset in the ISO file
	Length    int64 // number of bytes
}

// findBlurayM2TSInISO parses an ISO9660 filesystem to find M2TS files
// under the BDMV/STREAM/ directory. Returns the files sorted by name.
func findBlurayM2TSInISO(isoPath string) ([]isoFileExtent, error) {
	f, err := os.Open(isoPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read root directory from PVD
	rootExtent, rootDataLen, err := readISOPVDRoot(f)
	if err != nil {
		if errors.Is(err, errNotISO9660) {
			// No valid ISO9660 PVD, try UDF (Blu-ray ISOs from CloneBD)
			return findBlurayM2TSInUDF(f)
		}
		return nil, fmt.Errorf("read ISO PVD: %w", err)
	}

	// Navigate: root → BDMV → STREAM
	rootEntries, err := readISODirectory(f, rootExtent, rootDataLen)
	if err != nil {
		return nil, fmt.Errorf("read ISO root directory: %w", err)
	}

	bdmv, err := findISOEntry(rootEntries, "BDMV")
	if err != nil {
		return nil, fmt.Errorf("find BDMV directory: %w", err)
	}

	bdmvEntries, err := readISODirectory(f, uint32(bdmv.Offset/isoSectorSize), uint32(bdmv.Size))
	if err != nil {
		return nil, fmt.Errorf("read BDMV directory: %w", err)
	}

	stream, err := findISOEntry(bdmvEntries, "STREAM")
	if err != nil {
		return nil, fmt.Errorf("find STREAM directory: %w", err)
	}

	streamEntries, err := readISODirectory(f, uint32(stream.Offset/isoSectorSize), uint32(stream.Size))
	if err != nil {
		return nil, fmt.Errorf("read STREAM directory: %w", err)
	}

	// Collect M2TS files
	var m2tsFiles []isoFileExtent
	for _, e := range streamEntries {
		if !e.IsDir && strings.HasSuffix(e.Name, ".M2TS") {
			m2tsFiles = append(m2tsFiles, e)
		}
	}

	return m2tsFiles, nil
}

// readISOPVDRoot reads the Primary Volume Descriptor and returns the root
// directory extent LBA and data length.
func readISOPVDRoot(f *os.File) (extentLBA uint32, dataLen uint32, err error) {
	const pvdOffset = 16 * isoSectorSize

	pvd := make([]byte, isoSectorSize)
	if _, err := f.ReadAt(pvd, pvdOffset); err != nil {
		return 0, 0, err
	}

	// Verify PVD: type=1, signature="CD001"
	if pvd[0] != 1 || string(pvd[1:6]) != "CD001" {
		return 0, 0, fmt.Errorf("%w: invalid primary volume descriptor", errNotISO9660)
	}

	// Root directory record at offset 156
	root := pvd[156:]
	if len(root) < 34 {
		return 0, 0, fmt.Errorf("%w: root directory record too short", errNotISO9660)
	}

	extentLBA = uint32(root[2]) | uint32(root[3])<<8 |
		uint32(root[4])<<16 | uint32(root[5])<<24
	dataLen = uint32(root[10]) | uint32(root[11])<<8 |
		uint32(root[12])<<16 | uint32(root[13])<<24

	return extentLBA, dataLen, nil
}

// readISODirectory reads and parses an ISO9660 directory at the given extent.
func readISODirectory(f *os.File, extentLBA, dataLen uint32) ([]isoFileExtent, error) {
	// Cap directory read to 256KB to avoid huge allocations
	if dataLen > 256*1024 {
		dataLen = 256 * 1024
	}

	dirData := make([]byte, dataLen)
	if _, err := f.ReadAt(dirData, int64(extentLBA)*isoSectorSize); err != nil {
		return nil, err
	}

	var entries []isoFileExtent
	offset := 0
	for offset < len(dirData) {
		recLen := int(dirData[offset])
		if recLen == 0 {
			// Padding at end of sector — skip to next sector boundary
			nextSector := ((offset / isoSectorSize) + 1) * isoSectorSize
			if nextSector >= len(dirData) {
				break
			}
			offset = nextSector
			continue
		}
		if offset+recLen > len(dirData) {
			break
		}
		if offset+33 > len(dirData) {
			break
		}

		nameLen := int(dirData[offset+32])
		if nameLen == 0 || offset+33+nameLen > len(dirData) {
			offset += recLen
			continue
		}

		name := string(dirData[offset+33 : offset+33+nameLen])

		// Skip "." and ".." entries (single byte 0x00 or 0x01)
		if nameLen == 1 && (name[0] == 0x00 || name[0] == 0x01) {
			offset += recLen
			continue
		}

		// Normalize: uppercase, strip version (";1") and trailing dot
		name = strings.ToUpper(name)
		if idx := strings.Index(name, ";"); idx >= 0 {
			name = name[:idx]
		}
		name = strings.TrimSuffix(name, ".")

		// Extract extent LBA (bytes 2-5, little-endian)
		eLBA := uint32(dirData[offset+2]) | uint32(dirData[offset+3])<<8 |
			uint32(dirData[offset+4])<<16 | uint32(dirData[offset+5])<<24
		// Extract data length (bytes 10-13, little-endian)
		eLen := uint32(dirData[offset+10]) | uint32(dirData[offset+11])<<8 |
			uint32(dirData[offset+12])<<16 | uint32(dirData[offset+13])<<24

		// File flags byte 25: bit 1 = directory
		isDir := dirData[offset+25]&0x02 != 0

		entries = append(entries, isoFileExtent{
			Name:   name,
			Offset: int64(eLBA) * isoSectorSize,
			Size:   int64(eLen),
			IsDir:  isDir,
		})

		offset += recLen
	}

	return entries, nil
}

// findISOEntry finds a named directory entry (case-insensitive).
func findISOEntry(entries []isoFileExtent, name string) (*isoFileExtent, error) {
	upper := strings.ToUpper(name)
	for i := range entries {
		if entries[i].Name == upper {
			return &entries[i], nil
		}
	}
	return nil, fmt.Errorf("%q not found", name)
}
