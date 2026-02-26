package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/stuckj/mkvdup/internal/dedup"
	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/mmap"
)

// deltaClass accumulates byte count and entry count for delta classification.
type deltaClass struct {
	bytes int64
	count int
}

// deltadiag analyzes delta (unmatched) entries in a .mkvdup file by
// cross-referencing with the original MKV to classify what stream type
// each delta region belongs to (video/audio/container).
func deltadiag(dedupPath, mkvPath string) error {
	// Open dedup file
	reader, err := dedup.NewReader(dedupPath, "")
	if err != nil {
		return fmt.Errorf("open dedup file: %w", err)
	}
	defer reader.Close()

	entryCount := reader.EntryCount()
	origSize := reader.OriginalSize()
	fmt.Fprintf(os.Stderr, "Dedup file: %d %s, original size %s bytes (%.2f MB)\n",
		entryCount, plural(entryCount, "entry", "entries"), formatInt(origSize), float64(origSize)/(1024*1024))

	// Parse MKV to get packet boundaries
	fmt.Fprintf(os.Stderr, "Parsing MKV file...\n")
	mkvParser, err := mkv.NewParser(mkvPath)
	if err != nil {
		return fmt.Errorf("create MKV parser: %w", err)
	}
	defer mkvParser.Close()

	if err := mkvParser.Parse(nil); err != nil {
		return fmt.Errorf("parse MKV: %w", err)
	}

	packets := mkvParser.Packets()
	tracks := mkvParser.Tracks()
	fmt.Fprintf(os.Stderr, "  %d packets, %d tracks\n", len(packets), len(tracks))

	// Build track type map and detect AVCC NAL length size
	trackTypes := make(map[int]int)
	trackCodecs := make(map[int]string)
	nalLenSizes := make(map[int]int)
	isAVCTrack := make(map[int]bool)
	for _, t := range tracks {
		trackTypes[int(t.Number)] = t.Type
		trackCodecs[int(t.Number)] = t.CodecID
		nalLenSizes[int(t.Number)] = matcher.NALLengthSizeForTrack(t.CodecID, t.CodecPrivate)
		if strings.HasPrefix(t.CodecID, "V_MPEG4/ISO/AVC") {
			isAVCTrack[int(t.Number)] = true
		}
	}

	// Memory-map MKV for reading delta bytes
	mkvMmap, err := mmap.Open(mkvPath)
	if err != nil {
		return fmt.Errorf("mmap MKV: %w", err)
	}
	defer mkvMmap.Close()
	mkvData := mkvMmap.Data()

	// Sort packets by offset for binary search
	sort.Slice(packets, func(i, j int) bool {
		return packets[i].Offset < packets[j].Offset
	})

	// Classify each delta entry
	fmt.Fprintf(os.Stderr, "Classifying delta entries...\n")

	var deltaVideo, deltaAudio, deltaContainer deltaClass
	deltaAudioByCodec := make(map[string]*deltaClass)
	var deltaVideoByNAL [32]deltaClass
	var deltaVideoSliceSmall, deltaVideoSliceLarge deltaClass

	for i := 0; i < entryCount; i++ {
		ent, ok := reader.GetEntry(i)
		if !ok {
			continue
		}
		if ent.Source != 0 {
			continue // Skip matched entries
		}

		// Find which MKV packet contains this delta region.
		// NOTE: Classification is approximate — a delta entry can span multiple
		// packets or tracks, but we classify based on the single packet containing
		// the start offset. This is usually accurate since matches expand to
		// cover full NALs/frames, leaving delta gaps within a single packet.
		pktIdx := deltadiagFindPacket(packets, ent.MkvOffset)
		if pktIdx < 0 {
			deltaContainer.bytes += ent.Length
			deltaContainer.count++
			continue
		}

		pkt := packets[pktIdx]
		ttype := trackTypes[int(pkt.TrackNum)]

		if ttype == mkv.TrackTypeVideo {
			deltaVideo.bytes += ent.Length
			deltaVideo.count++

			// Parse AVCC NALs in the delta region (H.264 only — HEVC uses different NAL type encoding)
			nalLenSize := nalLenSizes[int(pkt.TrackNum)]
			if nalLenSize > 0 && isAVCTrack[int(pkt.TrackNum)] && ent.Length >= int64(nalLenSize+1) {
				deltaStart := ent.MkvOffset
				deltaEnd := ent.MkvOffset + ent.Length
				if deltaEnd <= int64(len(mkvData)) {
					deltadiagClassifyAVCC(mkvData, pkt, nalLenSize, deltaStart, deltaEnd,
						&deltaVideoByNAL, &deltaVideoSliceSmall, &deltaVideoSliceLarge)
				}
			}
		} else if ttype == mkv.TrackTypeAudio {
			deltaAudio.bytes += ent.Length
			deltaAudio.count++
			codec := trackCodecs[int(pkt.TrackNum)]
			if codec == "" {
				codec = "unknown"
			}
			dc := deltaAudioByCodec[codec]
			if dc == nil {
				dc = &deltaClass{}
				deltaAudioByCodec[codec] = dc
			}
			dc.bytes += ent.Length
			dc.count++
		} else {
			deltaContainer.bytes += ent.Length
			deltaContainer.count++
		}
	}

	// Print results
	totalDelta := deltaVideo.bytes + deltaAudio.bytes + deltaContainer.bytes
	if totalDelta == 0 {
		fmt.Printf("\nNo delta entries found (100%% matched).\n")
		return nil
	}

	fmt.Printf("\n=== Delta Classification ===\n")
	fmt.Printf("Total delta: %s bytes (%.2f MB)\n\n", formatInt(totalDelta), float64(totalDelta)/(1024*1024))

	fmt.Printf("Video delta:     %12s bytes (%8.2f MB) [%6d entries] (%.1f%% of delta)\n",
		formatInt(deltaVideo.bytes), float64(deltaVideo.bytes)/(1024*1024), deltaVideo.count,
		float64(deltaVideo.bytes)/float64(totalDelta)*100)
	fmt.Printf("Audio delta:     %12s bytes (%8.2f MB) [%6d entries] (%.1f%% of delta)\n",
		formatInt(deltaAudio.bytes), float64(deltaAudio.bytes)/(1024*1024), deltaAudio.count,
		float64(deltaAudio.bytes)/float64(totalDelta)*100)
	fmt.Printf("Container delta: %12s bytes (%8.2f MB) [%6d entries] (%.1f%% of delta)\n",
		formatInt(deltaContainer.bytes), float64(deltaContainer.bytes)/(1024*1024), deltaContainer.count,
		float64(deltaContainer.bytes)/float64(totalDelta)*100)

	// Audio codec breakdown
	if len(deltaAudioByCodec) > 0 {
		fmt.Printf("\n=== Audio Delta by Codec ===\n")
		for codec, dc := range deltaAudioByCodec {
			fmt.Printf("  %-20s: %10s bytes (%8.2f MB) [%6d entries]\n",
				codec, formatInt(dc.bytes), float64(dc.bytes)/(1024*1024), dc.count)
		}
	}

	// Video NAL type breakdown
	nalTypeNames := map[int]string{
		1: "non-IDR slice", 2: "slice A", 3: "slice B", 4: "slice C",
		5: "IDR slice", 6: "SEI", 7: "SPS", 8: "PPS", 9: "AUD", 12: "filler",
	}

	hasNALBreakdown := false
	for i := 0; i < 32; i++ {
		if deltaVideoByNAL[i].count > 0 {
			hasNALBreakdown = true
			break
		}
	}
	if hasNALBreakdown {
		fmt.Printf("\n=== Video Delta by H.264 NAL Type ===\n")
		for i := 0; i < 32; i++ {
			if deltaVideoByNAL[i].count == 0 {
				continue
			}
			name := nalTypeNames[i]
			if name == "" {
				name = fmt.Sprintf("type %d", i)
			}
			fmt.Printf("  %-14s: %10s bytes (%8.2f MB) [%6d NALs]\n",
				name, formatInt(deltaVideoByNAL[i].bytes),
				float64(deltaVideoByNAL[i].bytes)/(1024*1024),
				deltaVideoByNAL[i].count)
		}

		fmt.Printf("\n=== Video Slice Delta Size Breakdown ===\n")
		fmt.Printf("  Slice NALs < 4KB:  %10s bytes (%8.2f MB) [%6d NALs]\n",
			formatInt(deltaVideoSliceSmall.bytes), float64(deltaVideoSliceSmall.bytes)/(1024*1024),
			deltaVideoSliceSmall.count)
		fmt.Printf("  Slice NALs >= 4KB: %10s bytes (%8.2f MB) [%6d NALs]\n",
			formatInt(deltaVideoSliceLarge.bytes), float64(deltaVideoSliceLarge.bytes)/(1024*1024),
			deltaVideoSliceLarge.count)
	}

	// Summary
	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Original file:    %.2f MB\n", float64(origSize)/(1024*1024))
	fmt.Printf("Total delta:      %.2f MB (%.1f%% of original)\n",
		float64(totalDelta)/(1024*1024), float64(totalDelta)/float64(origSize)*100)
	fmt.Printf("  Video delta:    %.2f MB (%.1f%% of delta)\n",
		float64(deltaVideo.bytes)/(1024*1024), float64(deltaVideo.bytes)/float64(totalDelta)*100)
	fmt.Printf("  Audio delta:    %.2f MB (%.1f%% of delta)\n",
		float64(deltaAudio.bytes)/(1024*1024), float64(deltaAudio.bytes)/float64(totalDelta)*100)
	fmt.Printf("  Container:      %.2f MB (%.1f%% of delta)\n",
		float64(deltaContainer.bytes)/(1024*1024), float64(deltaContainer.bytes)/float64(totalDelta)*100)

	return nil
}

// deltadiagFindPacket finds the packet containing the given offset using binary search.
func deltadiagFindPacket(packets []mkv.Packet, offset int64) int {
	low, high := 0, len(packets)-1
	for low <= high {
		mid := (low + high) / 2
		pkt := packets[mid]
		if offset < pkt.Offset {
			high = mid - 1
		} else if offset >= pkt.Offset+pkt.Size {
			low = mid + 1
		} else {
			return mid
		}
	}
	return -1
}

// deltadiagClassifyAVCC parses AVCC NAL units within a packet to classify which
// NAL types fall within the delta region [deltaStart, deltaEnd).
func deltadiagClassifyAVCC(mkvData []byte, pkt mkv.Packet, nalLenSize int,
	deltaStart, deltaEnd int64,
	byNAL *[32]deltaClass, sliceSmall, sliceLarge *deltaClass) {

	pktEnd := pkt.Offset + pkt.Size
	if pktEnd > int64(len(mkvData)) {
		pktEnd = int64(len(mkvData))
	}
	pktData := mkvData[pkt.Offset:pktEnd]

	pos := 0
	for pos+nalLenSize < len(pktData) {
		var nalLen uint32
		switch nalLenSize {
		case 4:
			nalLen = binary.BigEndian.Uint32(pktData[pos:])
		case 2:
			nalLen = uint32(binary.BigEndian.Uint16(pktData[pos:]))
		case 1:
			nalLen = uint32(pktData[pos])
		}

		nalDataStart := pkt.Offset + int64(pos+nalLenSize)
		nalDataEnd := nalDataStart + int64(nalLen)
		if nalLen == 0 || nalDataEnd > pktEnd {
			break
		}

		nalFullStart := pkt.Offset + int64(pos)

		// Check overlap with delta region
		overlapStart := nalFullStart
		if overlapStart < deltaStart {
			overlapStart = deltaStart
		}
		overlapEnd := nalDataEnd
		if overlapEnd > deltaEnd {
			overlapEnd = deltaEnd
		}
		if overlapStart < overlapEnd {
			overlapBytes := overlapEnd - overlapStart

			if nalDataStart < int64(len(mkvData)) {
				nalType := mkvData[nalDataStart] & 0x1F
				byNAL[nalType].bytes += overlapBytes
				byNAL[nalType].count++

				if nalType == 1 || nalType == 5 {
					if nalLen >= 4096 {
						sliceLarge.bytes += overlapBytes
						sliceLarge.count++
					} else {
						sliceSmall.bytes += overlapBytes
						sliceSmall.count++
					}
				}
			}
		}

		pos = int(nalDataEnd - pkt.Offset)
		if pos <= 0 {
			break
		}
	}
}
