package source

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// UDF descriptor tag IDs.
const (
	udfTagAVDP          = 2
	udfTagPartitionDesc = 5
	udfTagLogicalVolume = 6
	udfTagFileSetDesc   = 256
	udfTagFileEntry     = 261
	udfTagFID           = 257
	udfTagExtFileEntry  = 266
)

// udfDescriptorTag is the 16-byte tag at the start of every UDF descriptor.
type udfDescriptorTag struct {
	TagID   uint16
	Version uint16
}

// udfExtent represents a physical extent (offset + length) on disk.
type udfExtent struct {
	Length   uint32
	Location uint32
}

// udfLongAD is a "long allocation descriptor" (16 bytes) used to reference
// data across partitions.
type udfLongAD struct {
	Length   uint32
	Location uint32 // logical block number within partition
	PartRef  uint16 // partition reference number
}

// udfShortAD is a "short allocation descriptor" (8 bytes).
type udfShortAD struct {
	Length   uint32
	Position uint32 // logical block number
}

// udfPartitionDesc holds fields from a UDF Partition Descriptor (tag 5).
type udfPartitionDesc struct {
	PartitionNumber  uint16
	StartingLocation uint32 // physical sector number
}

// udfLogicalVolumeDesc holds fields from a UDF Logical Volume Descriptor (tag 6).
type udfLogicalVolumeDesc struct {
	BlockSize     uint32
	FSDLocation   udfLongAD // File Set Descriptor location
	PartitionMaps []udfPartitionMap
}

// udfPartitionMap describes a partition map entry from the Logical Volume Descriptor.
type udfPartitionMap struct {
	Type         byte // 1 = physical, 2 = metadata/virtual/sparable
	PartitionNum uint16
	IsMetadata   bool
	MetaFileLoc  uint32 // for metadata partitions: file location
}

// udfFileEntry holds parsed fields from a File Entry (tag 261) or
// Extended File Entry (tag 266).
type udfFileEntry struct {
	ICBTag     byte // file type (4=directory, 5=file)
	InfoLength uint64
	AllocDescs []byte // raw allocation descriptors
	AllocType  byte   // 0=short_ad, 1=long_ad, 3=immediate/inline
	PartRef    uint16 // partition reference where this FE resides
}

// udfFID represents a File Identifier Descriptor (tag 257).
type udfFID struct {
	Name        string
	IsDir       bool
	IsParent    bool
	ICBLocation udfLongAD
}

// isUDFImage checks whether the file contains UDF Volume Recognition Sequence
// markers (BEA01/NSR02/NSR03) in sectors 16+.
func isUDFImage(f *os.File) bool {
	// VRS starts at sector 16. Scan up to sector 31 for BEA01 + NSR0x.
	buf := make([]byte, isoSectorSize)
	foundBEA := false
	foundNSR := false

	for sector := 16; sector < 32; sector++ {
		n, err := f.ReadAt(buf, int64(sector)*isoSectorSize)
		if err != nil || n < 6 {
			continue
		}
		ident := string(buf[1:6])
		switch ident {
		case "BEA01":
			foundBEA = true
		case "NSR02", "NSR03":
			foundNSR = true
		}
		if foundBEA && foundNSR {
			return true
		}
		if ident == "TEA01" {
			break
		}
	}
	return foundBEA && foundNSR
}

// detectUDFISOType navigates the UDF root directory to determine whether
// the ISO contains a Blu-ray (BDMV/) or DVD (VIDEO_TS/) structure.
func detectUDFISOType(f *os.File) (Type, error) {
	rootEntries, err := readUDFRootDir(f)
	if err != nil {
		return TypeDVD, nil // can't parse UDF, default to DVD
	}

	hasBDMV := false
	hasVideoTS := false
	for _, fid := range rootEntries {
		name := strings.ToUpper(fid.Name)
		if name == "BDMV" {
			hasBDMV = true
		}
		if name == "VIDEO_TS" {
			hasVideoTS = true
		}
	}

	if hasBDMV {
		return TypeBluray, nil
	}
	if hasVideoTS {
		return TypeDVD, nil
	}
	return TypeDVD, nil
}

// findBlurayM2TSInUDF navigates the UDF filesystem to find M2TS files
// under BDMV/STREAM/. Returns isoFileExtent entries compatible with
// the ISO9660 code path.
func findBlurayM2TSInUDF(f *os.File) ([]isoFileExtent, error) {
	ctx, err := newUDFContext(f)
	if err != nil {
		return nil, err
	}

	// Read root directory
	rootFIDs, err := ctx.readDirectoryFromFE(ctx.rootFE)
	if err != nil {
		return nil, fmt.Errorf("read UDF root directory: %w", err)
	}

	// Navigate to BDMV
	bdmvFE, err := ctx.lookupDir(rootFIDs, "BDMV")
	if err != nil {
		return nil, fmt.Errorf("find BDMV: %w", err)
	}

	bdmvFIDs, err := ctx.readDirectoryFromFE(bdmvFE)
	if err != nil {
		return nil, fmt.Errorf("read BDMV directory: %w", err)
	}

	// Navigate to STREAM
	streamFE, err := ctx.lookupDir(bdmvFIDs, "STREAM")
	if err != nil {
		return nil, fmt.Errorf("find STREAM: %w", err)
	}

	streamFIDs, err := ctx.readDirectoryFromFE(streamFE)
	if err != nil {
		return nil, fmt.Errorf("read STREAM directory: %w", err)
	}

	// Collect M2TS files
	var m2tsFiles []isoFileExtent
	for _, fid := range streamFIDs {
		if fid.IsDir || fid.IsParent {
			continue
		}
		name := strings.ToUpper(fid.Name)
		if !strings.HasSuffix(name, ".M2TS") {
			continue
		}

		fe, err := ctx.readFileEntryAt(fid.ICBLocation)
		if err != nil {
			continue
		}

		// Collect all physical extents for this file.
		extents, err := ctx.resolveAllExtents(fe)
		if err != nil || len(extents) == 0 {
			continue
		}

		m2ts := isoFileExtent{
			Name:   name,
			Offset: extents[0].ISOOffset,
			Size:   int64(fe.InfoLength),
			IsDir:  false,
		}

		// Only populate Extents if the data is non-contiguous.
		if !extentsContiguous(extents) {
			m2ts.Extents = extents
		}

		m2tsFiles = append(m2tsFiles, m2ts)
	}

	if len(m2tsFiles) == 0 {
		return nil, fmt.Errorf("no M2TS files found in UDF BDMV/STREAM/")
	}

	return m2tsFiles, nil
}

// udfContext holds the parsed UDF volume structures needed for navigation.
type udfContext struct {
	f          *os.File
	blockSize  uint32
	partStart  uint32 // physical sector of partition start
	partitions []udfPartitionDesc
	partMaps   []udfPartitionMap
	metaData   []byte // loaded metadata partition file (nil if Type 1 only)
	rootFE     *udfFileEntry
}

// newUDFContext reads and parses the UDF volume structures.
func newUDFContext(f *os.File) (*udfContext, error) {
	// Step 1: Read AVDP at sector 256 to find the VDS
	vdsExtent, err := readAVDP(f)
	if err != nil {
		return nil, fmt.Errorf("read AVDP: %w", err)
	}

	// Step 2: Read VDS to get partition and logical volume descriptors
	partDescs, lvd, err := readVDS(f, vdsExtent)
	if err != nil {
		return nil, fmt.Errorf("read VDS: %w", err)
	}
	if len(partDescs) == 0 {
		return nil, fmt.Errorf("no partition descriptors found in VDS")
	}

	ctx := &udfContext{
		f:          f,
		blockSize:  lvd.BlockSize,
		partStart:  partDescs[0].StartingLocation,
		partitions: partDescs,
		partMaps:   lvd.PartitionMaps,
	}

	// Step 3: Load metadata partition if present
	for _, pm := range lvd.PartitionMaps {
		if pm.IsMetadata {
			metaData, err := ctx.readMetadataFile(pm.MetaFileLoc)
			if err != nil {
				return nil, fmt.Errorf("read metadata partition: %w", err)
			}
			ctx.metaData = metaData
			break
		}
	}

	// Step 4: Read FSD to get root directory ICB
	rootFE, err := ctx.readFSDAndRoot(lvd.FSDLocation)
	if err != nil {
		return nil, fmt.Errorf("read FSD/root: %w", err)
	}
	ctx.rootFE = rootFE

	return ctx, nil
}

// readAVDP reads the Anchor Volume Descriptor Pointer at sector 256.
// Returns the extent of the Main Volume Descriptor Sequence.
func readAVDP(f *os.File) (udfExtent, error) {
	buf := make([]byte, isoSectorSize)
	if _, err := f.ReadAt(buf, 256*isoSectorSize); err != nil {
		return udfExtent{}, fmt.Errorf("read sector 256: %w", err)
	}

	tag := parseDescriptorTag(buf)
	if tag.TagID != udfTagAVDP {
		return udfExtent{}, fmt.Errorf("sector 256: expected AVDP (tag 2), got tag %d", tag.TagID)
	}

	// Main VDS extent at offset 16 (8 bytes: length + location)
	return udfExtent{
		Length:   binary.LittleEndian.Uint32(buf[16:20]),
		Location: binary.LittleEndian.Uint32(buf[20:24]),
	}, nil
}

// readVDS reads the Volume Descriptor Sequence and extracts partition
// descriptors and the logical volume descriptor.
func readVDS(f *os.File, extent udfExtent) ([]udfPartitionDesc, *udfLogicalVolumeDesc, error) {
	var partDescs []udfPartitionDesc
	var lvd *udfLogicalVolumeDesc

	sectors := int(extent.Length) / isoSectorSize
	if sectors > 64 {
		sectors = 64
	}

	buf := make([]byte, isoSectorSize)
	for i := 0; i < sectors; i++ {
		offset := int64(extent.Location+uint32(i)) * isoSectorSize
		if _, err := f.ReadAt(buf, offset); err != nil {
			break
		}

		tag := parseDescriptorTag(buf)
		switch tag.TagID {
		case udfTagPartitionDesc:
			pd := udfPartitionDesc{
				PartitionNumber:  binary.LittleEndian.Uint16(buf[22:24]),
				StartingLocation: binary.LittleEndian.Uint32(buf[188:192]),
			}
			partDescs = append(partDescs, pd)

		case udfTagLogicalVolume:
			blockSize := binary.LittleEndian.Uint32(buf[212:216])

			// FSD location at offset 248 (16-byte long_ad)
			fsdLoc := parseLongAD(buf[248:264])

			// Partition maps at offset 440
			mapTableLen := binary.LittleEndian.Uint32(buf[264:268])
			numMaps := binary.LittleEndian.Uint32(buf[268:272])
			mapData := buf[440:]
			if int(mapTableLen) < len(mapData) {
				mapData = mapData[:mapTableLen]
			}
			partMaps := parsePartitionMaps(mapData, int(numMaps))

			lvd = &udfLogicalVolumeDesc{
				BlockSize:     blockSize,
				FSDLocation:   fsdLoc,
				PartitionMaps: partMaps,
			}

		case 8: // Terminating Descriptor
			// handled below
		}

		if tag.TagID == 8 {
			break
		}
	}

	if lvd == nil {
		return nil, nil, fmt.Errorf("no Logical Volume Descriptor found")
	}

	return partDescs, lvd, nil
}

// parsePartitionMaps parses the partition map table from the LVD.
func parsePartitionMaps(data []byte, count int) []udfPartitionMap {
	var maps []udfPartitionMap
	offset := 0
	for i := 0; i < count && offset < len(data); i++ {
		if offset+2 > len(data) {
			break
		}
		mapType := data[offset]
		mapLen := int(data[offset+1])
		if mapLen == 0 || offset+mapLen > len(data) {
			break
		}

		pm := udfPartitionMap{Type: mapType}

		switch mapType {
		case 1:
			// Type 1: Physical partition (6 bytes)
			if mapLen >= 6 {
				pm.PartitionNum = binary.LittleEndian.Uint16(data[offset+4 : offset+6])
			}
		case 2:
			// Type 2: Could be metadata, virtual, or sparable (64 bytes)
			if mapLen >= 64 {
				pm.PartitionNum = binary.LittleEndian.Uint16(data[offset+38 : offset+40])
				// Check for metadata partition identifier at offset 4
				ident := string(data[offset+4 : offset+36])
				if strings.Contains(ident, "*UDF Metadata Partition") {
					pm.IsMetadata = true
					pm.MetaFileLoc = binary.LittleEndian.Uint32(data[offset+40 : offset+44])
				}
			}
		}

		maps = append(maps, pm)
		offset += mapLen
	}
	return maps
}

// readMetadataFile loads the metadata virtual file from the partition.
// The metadata file is a File Entry at partStart + metaFileLoc, whose
// allocation descriptors point to the actual metadata data.
func (ctx *udfContext) readMetadataFile(metaFileLoc uint32) ([]byte, error) {
	// Read the File Entry for the metadata file
	physSector := ctx.partStart + metaFileLoc
	buf := make([]byte, ctx.blockSize)
	if _, err := ctx.f.ReadAt(buf, int64(physSector)*int64(ctx.blockSize)); err != nil {
		return nil, fmt.Errorf("read metadata file entry at sector %d: %w", physSector, err)
	}

	fe, err := parseFileEntry(buf)
	if err != nil {
		return nil, fmt.Errorf("parse metadata file entry: %w", err)
	}

	// The metadata file's FE is on the physical partition. Find the
	// physical (Type 1) partition map index so short_ad resolves correctly.
	for i, pm := range ctx.partMaps {
		if pm.Type == 1 {
			fe.PartRef = uint16(i)
			break
		}
	}

	return ctx.readFileData(fe)
}

// readFSDAndRoot reads the File Set Descriptor and follows it to the root
// directory File Entry.
func (ctx *udfContext) readFSDAndRoot(fsdLoc udfLongAD) (*udfFileEntry, error) {
	fsdData, err := ctx.readBlock(fsdLoc.Location, fsdLoc.PartRef)
	if err != nil {
		return nil, fmt.Errorf("read FSD block: %w", err)
	}

	tag := parseDescriptorTag(fsdData)
	if tag.TagID != udfTagFileSetDesc {
		return nil, fmt.Errorf("expected FSD (tag 256), got tag %d", tag.TagID)
	}

	// Root directory ICB at offset 400 (16-byte long_ad)
	if len(fsdData) < 416 {
		return nil, fmt.Errorf("FSD too short")
	}
	rootICB := parseLongAD(fsdData[400:416])

	return ctx.readFileEntryAt(rootICB)
}

// readFileEntryAt reads and parses a File Entry at the given location.
func (ctx *udfContext) readFileEntryAt(loc udfLongAD) (*udfFileEntry, error) {
	data, err := ctx.readBlock(loc.Location, loc.PartRef)
	if err != nil {
		return nil, fmt.Errorf("read file entry block %d (part %d): %w", loc.Location, loc.PartRef, err)
	}
	fe, err := parseFileEntry(data)
	if err != nil {
		return nil, err
	}
	fe.PartRef = loc.PartRef
	return fe, nil
}

// readDirectoryFromFE reads directory data from a File Entry and parses FIDs.
func (ctx *udfContext) readDirectoryFromFE(fe *udfFileEntry) ([]udfFID, error) {
	dirData, err := ctx.readFileData(fe)
	if err != nil {
		return nil, err
	}
	return parseUDFDirectory(dirData), nil
}

// lookupDir finds a named subdirectory in a list of FIDs and reads its File Entry.
func (ctx *udfContext) lookupDir(fids []udfFID, name string) (*udfFileEntry, error) {
	upper := strings.ToUpper(name)
	for _, fid := range fids {
		if fid.IsParent {
			continue
		}
		if strings.ToUpper(fid.Name) == upper {
			return ctx.readFileEntryAt(fid.ICBLocation)
		}
	}
	return nil, fmt.Errorf("%q not found in directory", name)
}

// resolveAllExtents collects all physical extents for a file entry.
// For long_ad, each AD has an explicit partition reference.
// For short_ad, the partition is inherited from the FE.
func (ctx *udfContext) resolveAllExtents(fe *udfFileEntry) ([]isoPhysicalRange, error) {
	switch fe.AllocType & 0x07 {
	case 0: // short_ad
		if int(fe.PartRef) < len(ctx.partMaps) && ctx.partMaps[fe.PartRef].IsMetadata {
			return nil, fmt.Errorf("short_ad on metadata partition not supported for file extents")
		}
		var extents []isoPhysicalRange
		remaining := int64(fe.InfoLength)
		for off := 0; off+8 <= len(fe.AllocDescs) && remaining > 0; off += 8 {
			ad := parseShortAD(fe.AllocDescs[off : off+8])
			extLen := int64(ad.Length & 0x3FFFFFFF)
			if extLen == 0 {
				break
			}
			if extLen > remaining {
				extLen = remaining
			}
			extents = append(extents, isoPhysicalRange{
				ISOOffset: ctx.resolveBlockPhysical(ad.Position),
				Length:    extLen,
			})
			remaining -= extLen
		}
		return extents, nil

	case 1: // long_ad
		var extents []isoPhysicalRange
		remaining := int64(fe.InfoLength)
		for off := 0; off+16 <= len(fe.AllocDescs) && remaining > 0; off += 16 {
			ad := parseLongAD(fe.AllocDescs[off : off+16])
			extLen := int64(ad.Length & 0x3FFFFFFF)
			if extLen == 0 {
				break
			}
			if int(ad.PartRef) < len(ctx.partMaps) && ctx.partMaps[ad.PartRef].IsMetadata {
				return nil, fmt.Errorf("long_ad extent on metadata partition")
			}
			if extLen > remaining {
				extLen = remaining
			}
			extents = append(extents, isoPhysicalRange{
				ISOOffset: ctx.resolveBlockPhysical(ad.Location),
				Length:    extLen,
			})
			remaining -= extLen
		}
		return extents, nil

	default:
		return nil, fmt.Errorf("unsupported alloc type %d for extent resolution", fe.AllocType&0x07)
	}
}

// extentsContiguous returns true if all extents are physically adjacent.
func extentsContiguous(extents []isoPhysicalRange) bool {
	for i := 1; i < len(extents); i++ {
		prevEnd := extents[i-1].ISOOffset + extents[i-1].Length
		if extents[i].ISOOffset != prevEnd {
			return false
		}
	}
	return true
}

// readBlock reads one block from the given logical block number within
// the specified partition reference.
func (ctx *udfContext) readBlock(blockNum uint32, partRef uint16) ([]byte, error) {
	// Determine which partition map this references
	if int(partRef) < len(ctx.partMaps) && ctx.partMaps[partRef].IsMetadata {
		// Metadata partition: block is an offset into the loaded metadata data
		byteOffset := int64(blockNum) * int64(ctx.blockSize)
		if ctx.metaData == nil {
			return nil, fmt.Errorf("metadata partition referenced but not loaded")
		}
		if byteOffset+int64(ctx.blockSize) > int64(len(ctx.metaData)) {
			return nil, fmt.Errorf("metadata block %d out of range", blockNum)
		}
		result := make([]byte, ctx.blockSize)
		copy(result, ctx.metaData[byteOffset:byteOffset+int64(ctx.blockSize)])
		return result, nil
	}

	// Physical partition: blockNum is relative to partition start
	physOffset := int64(ctx.partStart+blockNum) * int64(ctx.blockSize)

	buf := make([]byte, ctx.blockSize)
	if _, err := ctx.f.ReadAt(buf, physOffset); err != nil {
		return nil, err
	}
	return buf, nil
}

// resolveBlockPhysical converts a logical block number to a physical byte offset
// using the default (first physical) partition.
func (ctx *udfContext) resolveBlockPhysical(blockNum uint32) int64 {
	return int64(ctx.partStart+blockNum) * int64(ctx.blockSize)
}

// readFileData reads the complete data of a file described by a File Entry.
func (ctx *udfContext) readFileData(fe *udfFileEntry) ([]byte, error) {
	if fe.InfoLength == 0 {
		return nil, nil
	}

	switch fe.AllocType & 0x07 {
	case 3: // inline/immediate
		if uint64(len(fe.AllocDescs)) < fe.InfoLength {
			return fe.AllocDescs, nil
		}
		return fe.AllocDescs[:fe.InfoLength], nil

	case 0: // short_ad
		return ctx.readFromShortADs(fe)

	case 1: // long_ad
		return ctx.readFromLongADs(fe)

	default:
		return nil, fmt.Errorf("unsupported allocation type %d", fe.AllocType&0x07)
	}
}

// readFromShortADs reads file data described by short allocation descriptors.
// Short ADs don't carry an explicit partition reference — they inherit the
// partition of the File Entry that contains them.
func (ctx *udfContext) readFromShortADs(fe *udfFileEntry) ([]byte, error) {
	// Determine if this FE's partition is the metadata partition.
	isMeta := int(fe.PartRef) < len(ctx.partMaps) && ctx.partMaps[fe.PartRef].IsMetadata && ctx.metaData != nil

	result := make([]byte, 0, fe.InfoLength)
	remaining := int64(fe.InfoLength)

	for off := 0; off+8 <= len(fe.AllocDescs) && remaining > 0; off += 8 {
		ad := parseShortAD(fe.AllocDescs[off : off+8])
		extLen := int64(ad.Length & 0x3FFFFFFF) // mask off extent type bits
		if extLen == 0 {
			break
		}

		toRead := min(extLen, remaining)

		if isMeta {
			// Resolve within the loaded metadata data.
			byteOffset := int64(ad.Position) * int64(ctx.blockSize)
			if byteOffset+toRead > int64(len(ctx.metaData)) {
				return nil, fmt.Errorf("metadata short_ad extent out of range (offset %d, len %d, metaLen %d)",
					byteOffset, toRead, len(ctx.metaData))
			}
			result = append(result, ctx.metaData[byteOffset:byteOffset+toRead]...)
		} else {
			physOffset := int64(ctx.partStart+ad.Position) * int64(ctx.blockSize)
			buf := make([]byte, toRead)
			if _, err := ctx.f.ReadAt(buf, physOffset); err != nil {
				return nil, fmt.Errorf("read short_ad extent at offset %d: %w", physOffset, err)
			}
			result = append(result, buf...)
		}
		remaining -= toRead
	}

	return result, nil
}

// readFromLongADs reads file data described by long allocation descriptors.
func (ctx *udfContext) readFromLongADs(fe *udfFileEntry) ([]byte, error) {
	result := make([]byte, 0, fe.InfoLength)
	remaining := int64(fe.InfoLength)

	for off := 0; off+16 <= len(fe.AllocDescs) && remaining > 0; off += 16 {
		ad := parseLongAD(fe.AllocDescs[off : off+16])
		extLen := int64(ad.Length & 0x3FFFFFFF) // mask off extent type bits
		if extLen == 0 {
			break
		}

		toRead := min(extLen, remaining)

		// Check if this references the metadata partition
		if int(ad.PartRef) < len(ctx.partMaps) && ctx.partMaps[ad.PartRef].IsMetadata && ctx.metaData != nil {
			byteOffset := int64(ad.Location) * int64(ctx.blockSize)
			if byteOffset+toRead > int64(len(ctx.metaData)) {
				return nil, fmt.Errorf("metadata extent out of range")
			}
			result = append(result, ctx.metaData[byteOffset:byteOffset+toRead]...)
		} else {
			physOffset := int64(ctx.partStart+ad.Location) * int64(ctx.blockSize)
			buf := make([]byte, toRead)
			if _, err := ctx.f.ReadAt(buf, physOffset); err != nil {
				return nil, fmt.Errorf("read long_ad extent at offset %d: %w", physOffset, err)
			}
			result = append(result, buf...)
		}

		remaining -= toRead
	}

	return result, nil
}

// readUDFRootDir is a convenience function that reads just the root directory
// entries from a UDF filesystem. Used by detectUDFISOType.
func readUDFRootDir(f *os.File) ([]udfFID, error) {
	ctx, err := newUDFContext(f)
	if err != nil {
		return nil, err
	}
	return ctx.readDirectoryFromFE(ctx.rootFE)
}

// --- Low-level parsing helpers ---

// parseDescriptorTag parses the 16-byte UDF descriptor tag at the start of buf.
func parseDescriptorTag(buf []byte) udfDescriptorTag {
	if len(buf) < 16 {
		return udfDescriptorTag{}
	}
	return udfDescriptorTag{
		TagID:   binary.LittleEndian.Uint16(buf[0:2]),
		Version: binary.LittleEndian.Uint16(buf[2:4]),
	}
}

// parseLongAD parses a 16-byte long allocation descriptor.
func parseLongAD(buf []byte) udfLongAD {
	return udfLongAD{
		Length:   binary.LittleEndian.Uint32(buf[0:4]),
		Location: binary.LittleEndian.Uint32(buf[4:8]),
		PartRef:  binary.LittleEndian.Uint16(buf[8:10]),
	}
}

// parseShortAD parses an 8-byte short allocation descriptor.
func parseShortAD(buf []byte) udfShortAD {
	return udfShortAD{
		Length:   binary.LittleEndian.Uint32(buf[0:4]),
		Position: binary.LittleEndian.Uint32(buf[4:8]),
	}
}

// parseFileEntry parses a UDF File Entry (tag 261) or Extended File Entry (tag 266).
func parseFileEntry(data []byte) (*udfFileEntry, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("data too short for file entry")
	}

	tag := parseDescriptorTag(data)
	if tag.TagID != udfTagFileEntry && tag.TagID != udfTagExtFileEntry {
		return nil, fmt.Errorf("expected File Entry (tag 261/266), got tag %d", tag.TagID)
	}

	// ICB Tag at offset 16 (20 bytes), file type at ICB tag offset 11 (= data offset 27)
	if len(data) < 28 {
		return nil, fmt.Errorf("data too short for ICB tag")
	}
	fileType := data[27]

	var infoLength uint64
	var allocDescsOffset int
	var allocDescsLength uint32
	var icbFlags uint16

	if tag.TagID == udfTagFileEntry {
		// File Entry (tag 261)
		// ECMA-167 14.9: L_EA at 168, L_AD at 172, alloc descs at 176+L_EA
		if len(data) < 176 {
			return nil, fmt.Errorf("file entry too short")
		}
		infoLength = binary.LittleEndian.Uint64(data[56:64])
		icbFlags = binary.LittleEndian.Uint16(data[34:36])

		eaLen := binary.LittleEndian.Uint32(data[168:172])
		allocDescsLength = binary.LittleEndian.Uint32(data[172:176])
		allocDescsOffset = 176 + int(eaLen)
	} else {
		// Extended File Entry (tag 266)
		// ECMA-167 14.17: L_EA at 208, L_AD at 212, alloc descs at 216+L_EA
		if len(data) < 216 {
			return nil, fmt.Errorf("extended file entry too short")
		}
		infoLength = binary.LittleEndian.Uint64(data[56:64])
		icbFlags = binary.LittleEndian.Uint16(data[34:36])

		eaLen := binary.LittleEndian.Uint32(data[208:212])
		allocDescsLength = binary.LittleEndian.Uint32(data[212:216])
		allocDescsOffset = 216 + int(eaLen)
	}

	// Guard against overflow or out-of-bounds from malformed eaLen
	if allocDescsOffset < 0 || allocDescsOffset > len(data) {
		return nil, fmt.Errorf("file entry alloc descs offset out of bounds: %d", allocDescsOffset)
	}

	var allocDescs []byte
	if allocDescsOffset+int(allocDescsLength) <= len(data) {
		allocDescs = make([]byte, allocDescsLength)
		copy(allocDescs, data[allocDescsOffset:allocDescsOffset+int(allocDescsLength)])
	}

	return &udfFileEntry{
		ICBTag:     fileType,
		InfoLength: infoLength,
		AllocDescs: allocDescs,
		AllocType:  byte(icbFlags & 0x07),
	}, nil
}

// parseUDFDirectory parses raw directory data into a list of FIDs.
func parseUDFDirectory(dirData []byte) []udfFID {
	var fids []udfFID
	offset := 0

	for offset+38 <= len(dirData) {
		tag := parseDescriptorTag(dirData[offset:])
		if tag.TagID != udfTagFID {
			break
		}

		characteristics := dirData[offset+18]
		nameLen := int(dirData[offset+19])
		icbLoc := parseLongAD(dirData[offset+20 : offset+36])
		implUseLen := int(binary.LittleEndian.Uint16(dirData[offset+36 : offset+38]))

		nameStart := offset + 38 + implUseLen
		if nameStart+nameLen > len(dirData) {
			break
		}

		isParent := characteristics&0x08 != 0
		isDir := characteristics&0x02 != 0

		name := ""
		if nameLen > 0 && !isParent {
			nameBytes := dirData[nameStart : nameStart+nameLen]
			name = decodeUDFString(nameBytes)
		}

		fids = append(fids, udfFID{
			Name:        name,
			IsDir:       isDir,
			IsParent:    isParent,
			ICBLocation: icbLoc,
		})

		// FID total length: 38 + implUseLen + nameLen, padded to 4-byte boundary
		fidLen := 38 + implUseLen + nameLen
		fidLen = (fidLen + 3) &^ 3
		offset += fidLen
	}

	return fids
}

// decodeUDFString decodes a UDF d-string/d-characters identifier.
// UDF uses either 8-bit (compression ID 8) or 16-bit (compression ID 16) encoding.
func decodeUDFString(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	compressionID := data[0]
	payload := data[1:]

	switch compressionID {
	case 8:
		// 8-bit characters (Latin-1 / ASCII subset)
		return string(payload)
	case 16:
		// 16-bit big-endian Unicode (UCS-2)
		var sb strings.Builder
		for i := 0; i+1 < len(payload); i += 2 {
			ch := rune(payload[i])<<8 | rune(payload[i+1])
			sb.WriteRune(ch)
		}
		return sb.String()
	default:
		// Unknown compression ID — try as raw bytes
		return string(payload)
	}
}
