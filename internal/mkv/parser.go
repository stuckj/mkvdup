package mkv

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/exp/mmap"
)

// Packet represents a codec data packet extracted from an MKV file.
type Packet struct {
	Offset    int64  // Offset in the MKV file where packet data starts
	Size      int64  // Size of packet data
	TrackNum  uint64 // Track number this packet belongs to
	Timestamp int64  // Absolute timestamp (cluster + block relative)
	Keyframe  bool   // Whether this is a keyframe
}

// Track represents an MKV track (video, audio, etc).
type Track struct {
	Number  uint64
	UID     uint64
	Type    int
	CodecID string
}

// Parser parses MKV files to extract codec packets.
type Parser struct {
	path    string
	reader  *mmap.ReaderAt
	size    int64
	tracks  []Track
	packets []Packet
}

// NewParser creates a new MKV parser for the given file.
func NewParser(path string) (*Parser, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	reader, err := mmap.Open(path)
	if err != nil {
		return nil, fmt.Errorf("mmap file: %w", err)
	}

	return &Parser{
		path:   path,
		reader: reader,
		size:   info.Size(),
	}, nil
}

// Close releases resources used by the parser.
func (p *Parser) Close() error {
	if p.reader != nil {
		return p.reader.Close()
	}
	return nil
}

// Size returns the file size.
func (p *Parser) Size() int64 {
	return p.size
}

// ProgressFunc is called to report parsing progress.
type ProgressFunc func(processed, total int64)

// Parse parses the MKV file and extracts all codec packets.
// If progress is non-nil, it will be called periodically.
func (p *Parser) Parse(progress ProgressFunc) error {
	offset := int64(0)

	// Parse EBML header
	elem, err := p.readElementAt(offset)
	if err != nil {
		return fmt.Errorf("read EBML header: %w", err)
	}
	if elem.ID != IDEBMLHeader {
		return fmt.Errorf("expected EBML header, got 0x%X", elem.ID)
	}
	offset = elem.DataOffset + elem.Size

	// Parse Segment
	elem, err = p.readElementAt(offset)
	if err != nil {
		return fmt.Errorf("read Segment: %w", err)
	}
	if elem.ID != IDSegment {
		return fmt.Errorf("expected Segment, got 0x%X", elem.ID)
	}

	segmentDataStart := elem.DataOffset
	segmentEnd := p.size
	if elem.Size > 0 {
		segmentEnd = elem.DataOffset + elem.Size
	}

	// Parse segment contents
	offset = segmentDataStart
	var clusterTimestamp int64

	for offset < segmentEnd {
		if progress != nil && offset%(1024*1024) == 0 {
			progress(offset, p.size)
		}

		elem, err = p.readElementAt(offset)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read element at %d: %w", offset, err)
		}

		switch elem.ID {
		case IDTracks:
			if err := p.parseTracks(elem); err != nil {
				return fmt.Errorf("parse tracks: %w", err)
			}

		case IDCluster:
			if err := p.parseCluster(elem, &clusterTimestamp); err != nil {
				return fmt.Errorf("parse cluster at %d: %w", offset, err)
			}
		}

		// Move to next element
		if elem.Size < 0 {
			// Unknown size - need to scan for next element
			// For now, we'll just move past the header
			offset = elem.DataOffset
		} else {
			offset = elem.DataOffset + elem.Size
		}
	}

	if progress != nil {
		progress(p.size, p.size)
	}

	return nil
}

// readElementAt reads an EBML element header at the given offset.
func (p *Parser) readElementAt(offset int64) (Element, error) {
	if offset >= p.size {
		return Element{}, io.EOF
	}

	r := io.NewSectionReader(p.reader, offset, p.size-offset)
	return ReadElementHeader(r, offset)
}

// parseTracks parses the Tracks element to extract track information.
func (p *Parser) parseTracks(tracksElem Element) error {
	offset := tracksElem.DataOffset
	end := tracksElem.DataOffset + tracksElem.Size

	for offset < end {
		elem, err := p.readElementAt(offset)
		if err != nil {
			return err
		}

		if elem.ID == IDTrackEntry {
			track, err := p.parseTrackEntry(elem)
			if err != nil {
				return fmt.Errorf("parse track entry: %w", err)
			}
			p.tracks = append(p.tracks, track)
		}

		offset = elem.DataOffset + elem.Size
	}

	return nil
}

// parseTrackEntry parses a TrackEntry element.
func (p *Parser) parseTrackEntry(trackElem Element) (Track, error) {
	var track Track
	offset := trackElem.DataOffset
	end := trackElem.DataOffset + trackElem.Size

	for offset < end {
		elem, err := p.readElementAt(offset)
		if err != nil {
			return track, err
		}

		r := io.NewSectionReader(p.reader, elem.DataOffset, elem.Size)

		switch elem.ID {
		case IDTrackNum:
			track.Number, _ = ReadUint(r, elem.Size)
		case IDTrackUID:
			track.UID, _ = ReadUint(r, elem.Size)
		case IDTrackType:
			t, _ := ReadUint(r, elem.Size)
			track.Type = int(t)
		case IDCodecID:
			track.CodecID, _ = ReadString(r, elem.Size)
		}

		offset = elem.DataOffset + elem.Size
	}

	return track, nil
}

// parseCluster parses a Cluster element and extracts packets.
func (p *Parser) parseCluster(clusterElem Element, clusterTimestamp *int64) error {
	offset := clusterElem.DataOffset
	end := clusterElem.DataOffset + clusterElem.Size
	if clusterElem.Size < 0 {
		// Unknown size - parse until we hit another top-level element
		end = p.size
	}

	for offset < end {
		elem, err := p.readElementAt(offset)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		// Check if we've hit a top-level element (end of cluster with unknown size)
		if isTopLevelElement(elem.ID) && clusterElem.Size < 0 {
			break
		}

		switch elem.ID {
		case IDTimestamp:
			r := io.NewSectionReader(p.reader, elem.DataOffset, elem.Size)
			ts, _ := ReadUint(r, elem.Size)
			*clusterTimestamp = int64(ts)

		case IDSimpleBlock:
			if err := p.parseSimpleBlock(elem, *clusterTimestamp); err != nil {
				return fmt.Errorf("parse SimpleBlock: %w", err)
			}

		case IDBlockGroup:
			if err := p.parseBlockGroup(elem, *clusterTimestamp); err != nil {
				return fmt.Errorf("parse BlockGroup: %w", err)
			}
		}

		offset = elem.DataOffset + elem.Size
	}

	return nil
}

// parseSimpleBlock parses a SimpleBlock element and adds packets.
func (p *Parser) parseSimpleBlock(elem Element, clusterTimestamp int64) error {
	// Read header bytes to parse track number, timestamp, and flags
	headerBuf := make([]byte, 16) // More than enough for header
	readSize := elem.Size
	if readSize > int64(len(headerBuf)) {
		readSize = int64(len(headerBuf))
	}

	r := io.NewSectionReader(p.reader, elem.DataOffset, readSize)
	n, err := r.Read(headerBuf)
	if err != nil || n < 4 {
		return fmt.Errorf("read SimpleBlock header: %w", err)
	}

	header, err := ParseSimpleBlockHeader(headerBuf[:n])
	if err != nil {
		return err
	}

	// The packet data follows the header
	packetOffset := elem.DataOffset + int64(header.HeaderSize)
	packetSize := elem.Size - int64(header.HeaderSize)

	// Handle lacing if present
	if header.LacingType() != LacingNone {
		// For now, treat the entire laced data as one packet
		// A more complete implementation would parse individual frames
		p.packets = append(p.packets, Packet{
			Offset:    packetOffset,
			Size:      packetSize,
			TrackNum:  header.TrackNumber,
			Timestamp: clusterTimestamp + int64(header.Timestamp),
			Keyframe:  header.IsKeyframe(),
		})
	} else {
		p.packets = append(p.packets, Packet{
			Offset:    packetOffset,
			Size:      packetSize,
			TrackNum:  header.TrackNumber,
			Timestamp: clusterTimestamp + int64(header.Timestamp),
			Keyframe:  header.IsKeyframe(),
		})
	}

	return nil
}

// parseBlockGroup parses a BlockGroup element and adds packets.
func (p *Parser) parseBlockGroup(groupElem Element, clusterTimestamp int64) error {
	offset := groupElem.DataOffset
	end := groupElem.DataOffset + groupElem.Size

	for offset < end {
		elem, err := p.readElementAt(offset)
		if err != nil {
			return err
		}

		if elem.ID == IDBlock {
			// Block has same format as SimpleBlock for the header
			headerBuf := make([]byte, 16)
			readSize := elem.Size
			if readSize > int64(len(headerBuf)) {
				readSize = int64(len(headerBuf))
			}

			r := io.NewSectionReader(p.reader, elem.DataOffset, readSize)
			n, err := r.Read(headerBuf)
			if err != nil || n < 4 {
				return fmt.Errorf("read Block header: %w", err)
			}

			header, err := ParseSimpleBlockHeader(headerBuf[:n])
			if err != nil {
				return err
			}

			packetOffset := elem.DataOffset + int64(header.HeaderSize)
			packetSize := elem.Size - int64(header.HeaderSize)

			p.packets = append(p.packets, Packet{
				Offset:    packetOffset,
				Size:      packetSize,
				TrackNum:  header.TrackNumber,
				Timestamp: clusterTimestamp + int64(header.Timestamp),
				Keyframe:  false, // Block doesn't have keyframe flag, would need ReferenceBlock
			})
		}

		offset = elem.DataOffset + elem.Size
	}

	return nil
}

// isTopLevelElement returns true if the element ID is a top-level segment child.
func isTopLevelElement(id uint64) bool {
	switch id {
	case IDSeekHead, IDInfo, IDTracks, IDChapters, IDCluster, IDCues, IDTags:
		return true
	}
	return false
}

// Packets returns all parsed packets.
func (p *Parser) Packets() []Packet {
	return p.packets
}

// Tracks returns all parsed tracks.
func (p *Parser) Tracks() []Track {
	return p.tracks
}

// PacketCount returns the number of packets parsed.
func (p *Parser) PacketCount() int {
	return len(p.packets)
}

// VideoPacketCount returns the number of video packets.
func (p *Parser) VideoPacketCount() int {
	count := 0
	videoTracks := make(map[uint64]bool)
	for _, t := range p.tracks {
		if t.Type == TrackTypeVideo {
			videoTracks[t.Number] = true
		}
	}
	for _, pkt := range p.packets {
		if videoTracks[pkt.TrackNum] {
			count++
		}
	}
	return count
}

// AudioPacketCount returns the number of audio packets.
func (p *Parser) AudioPacketCount() int {
	count := 0
	audioTracks := make(map[uint64]bool)
	for _, t := range p.tracks {
		if t.Type == TrackTypeAudio {
			audioTracks[t.Number] = true
		}
	}
	for _, pkt := range p.packets {
		if audioTracks[pkt.TrackNum] {
			count++
		}
	}
	return count
}

// ReadPacketData reads the data for a packet.
func (p *Parser) ReadPacketData(pkt Packet) ([]byte, error) {
	data := make([]byte, pkt.Size)
	n, err := p.reader.ReadAt(data, pkt.Offset)
	if err != nil && int64(n) != pkt.Size {
		return nil, fmt.Errorf("read packet data: %w", err)
	}
	return data[:n], nil
}
