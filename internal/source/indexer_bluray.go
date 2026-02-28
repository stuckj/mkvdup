package source

import (
	"fmt"

	"github.com/stuckj/mkvdup/internal/mmap"
	"golang.org/x/sys/unix"
)

// indexM2TSFile processes a Blu-ray M2TS file using ES-aware indexing.
// It parses the MPEG-TS structure to extract elementary stream data and
// indexes sync points within the continuous ES, matching what MKV files contain.
func (idx *Indexer) indexM2TSFile(fileIndex uint16, path string, size int64, progress func(int64)) (uint64, error) {
	mmapFile, err := mmap.Open(path)
	if err != nil {
		return 0, fmt.Errorf("mmap open: %w", err)
	}
	// Note: Don't close mmapFile - it's stored in MmapFiles for later use
	idx.index.MmapFiles = append(idx.index.MmapFiles, mmapFile)

	mmapFile.Advise(unix.MADV_SEQUENTIAL)

	// Phase 1: Parse MPEG-TS structure (0% → 33%)
	parser := NewMPEGTSParser(mmapFile.Data())

	if err := parser.ParseWithProgress(func(processed, total int64) {
		if progress != nil {
			progress(processed / 3)
		}
	}); err != nil {
		return 0, fmt.Errorf("parse MPEG-TS: %w", err)
	}

	// Store parser for later use by matcher
	idx.index.ESReaders = append(idx.index.ESReaders, parser)

	// Phase 2: Checksum (33% → 66%)
	checksum := checksumWithProgress(mmapFile.Data(), func(processed int64) {
		if progress != nil {
			progress(size/3 + processed/3)
		}
	})

	// Phase 3: Index ES data (66% → 100%)
	videoESSize := parser.TotalESSize(true)
	if videoESSize > 0 {
		indexProgress := func(fileOffset int64) {
			if progress != nil {
				progress(2*size/3 + fileOffset/3)
			}
		}
		if err := idx.indexESData(fileIndex, parser, true, videoESSize, indexProgress); err != nil {
			return 0, fmt.Errorf("index video ES: %w", err)
		}
	}

	// Index each audio sub-stream separately
	subtitleIDs := parser.SubtitleSubStreams()
	subtitleSet := make(map[byte]bool, len(subtitleIDs))
	for _, id := range subtitleIDs {
		subtitleSet[id] = true
	}
	for _, subStreamID := range parser.AudioSubStreams() {
		if subtitleSet[subStreamID] {
			continue // indexed below with subtitle-specific sync points
		}
		subStreamSize := parser.AudioSubStreamESSize(subStreamID)
		if subStreamSize > 0 {
			if err := idx.indexAudioSubStream(fileIndex, parser, subStreamID, subStreamSize); err != nil {
				return 0, fmt.Errorf("index audio sub-stream %d: %w", subStreamID, err)
			}
		}
	}

	// Index subtitle sub-streams with PGS sync point detection
	for _, subStreamID := range subtitleIDs {
		subStreamSize := parser.AudioSubStreamESSize(subStreamID)
		if subStreamSize > 0 {
			if err := idx.indexSubStream(fileIndex, parser, subStreamID, subStreamSize, FindPGSSyncPoints); err != nil {
				return 0, fmt.Errorf("index subtitle sub-stream %d: %w", subStreamID, err)
			}
		}
	}

	if progress != nil {
		progress(size)
	}

	return checksum, nil
}

// indexBlurayISOFile processes a Blu-ray ISO file by finding M2TS regions
// within the ISO9660 filesystem and indexing each as a separate source file entry.
// Returns the number of source file entries created and the ISO checksum.
func (idx *Indexer) indexBlurayISOFile(startFileIndex uint16, path, relPath string, size int64, progress func(int64)) (int, uint64, error) {
	// Find M2TS file extents within the ISO
	m2tsFiles, err := findBlurayM2TSInISO(path)
	if err != nil {
		return 0, 0, fmt.Errorf("find M2TS in ISO: %w", err)
	}
	if len(m2tsFiles) == 0 {
		return 0, 0, fmt.Errorf("no M2TS files found in Blu-ray ISO")
	}

	// Memory-map the entire ISO
	mmapFile, err := mmap.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("mmap open: %w", err)
	}
	// Don't close — stored in MmapFiles for later use
	idx.index.MmapFiles = append(idx.index.MmapFiles, mmapFile)

	mmapFile.Advise(unix.MADV_SEQUENTIAL)
	isoData := mmapFile.Data()

	// Phase 1: Parse all M2TS regions (0% → 33%)
	type parsedM2TS struct {
		adapter *isoM2TSAdapter
		extent  isoFileExtent
	}
	var parsed []parsedM2TS

	for _, m2ts := range m2tsFiles {
		var adapter *isoM2TSAdapter

		if m2ts.Extents != nil {
			// Multi-extent UDF file: create virtual contiguous view
			// over the existing mmap sub-slices (zero-copy, no heap allocation)
			mr := newMultiRegionData(m2ts.Extents, isoData)

			parser := NewMPEGTSParserMultiRegion(mr)
			if err := parser.ParseWithProgress(nil); err != nil {
				if idx.verboseWriter != nil {
					fmt.Fprintf(idx.verboseWriter, "  [indexBlurayISO] skipping %s: %v\n", m2ts.Name, err)
				}
				continue
			}
			adapter = newISOAdapterMultiExtent(parser, mr, m2ts.Extents)
		} else {
			// Contiguous file: use sub-slice of mmap'd ISO
			endOffset := m2ts.Offset + m2ts.Size
			if endOffset > int64(len(isoData)) {
				if idx.verboseWriter != nil {
					fmt.Fprintf(idx.verboseWriter, "  [indexBlurayISO] skipping %s: extent beyond ISO bounds (%d + %d > %d)\n",
						m2ts.Name, m2ts.Offset, m2ts.Size, len(isoData))
				}
				continue
			}

			m2tsData := isoData[m2ts.Offset:endOffset]
			parser := NewMPEGTSParser(m2tsData)
			if err := parser.ParseWithProgress(nil); err != nil {
				if idx.verboseWriter != nil {
					fmt.Fprintf(idx.verboseWriter, "  [indexBlurayISO] skipping %s: %v\n", m2ts.Name, err)
				}
				continue
			}
			adapter = newISOAdapter(parser, isoData, m2ts.Offset)
		}

		parsed = append(parsed, parsedM2TS{adapter: adapter, extent: m2ts})
	}

	if len(parsed) == 0 {
		return 0, 0, fmt.Errorf("no valid M2TS streams found in Blu-ray ISO")
	}

	if progress != nil {
		progress(size / 3)
	}

	// Phase 2: Checksum the full ISO (33% → 66%)
	checksum := checksumWithProgress(isoData, func(processed int64) {
		if progress != nil {
			progress(size/3 + processed/3)
		}
	})

	// Phase 3: Index ES data from all M2TS regions (66% → 100%)
	entriesCreated := 0
	for _, p := range parsed {
		fileIndex := startFileIndex + uint16(entriesCreated)
		adapter := p.adapter

		// Store adapter as ESReader for this source file entry
		idx.index.ESReaders = append(idx.index.ESReaders, adapter)

		// Index video ES
		videoESSize := adapter.TotalESSize(true)
		if videoESSize > 0 {
			if err := idx.indexESData(fileIndex, adapter, true, videoESSize, nil); err != nil {
				return 0, 0, fmt.Errorf("index video ES for %s: %w", p.extent.Name, err)
			}
		}

		// Index audio sub-streams
		subtitleIDs := adapter.parser.SubtitleSubStreams()
		subtitleSet := make(map[byte]bool, len(subtitleIDs))
		for _, id := range subtitleIDs {
			subtitleSet[id] = true
		}
		for _, subStreamID := range adapter.AudioSubStreams() {
			if subtitleSet[subStreamID] {
				continue
			}
			subStreamSize := adapter.AudioSubStreamESSize(subStreamID)
			if subStreamSize > 0 {
				if err := idx.indexAudioSubStream(fileIndex, adapter, subStreamID, subStreamSize); err != nil {
					return 0, 0, fmt.Errorf("index audio sub-stream %d for %s: %w", subStreamID, p.extent.Name, err)
				}
			}
		}

		// Index subtitle sub-streams
		for _, subStreamID := range subtitleIDs {
			subStreamSize := adapter.AudioSubStreamESSize(subStreamID)
			if subStreamSize > 0 {
				if err := idx.indexSubStream(fileIndex, adapter, subStreamID, subStreamSize, FindPGSSyncPoints); err != nil {
					return 0, 0, fmt.Errorf("index subtitle sub-stream %d for %s: %w", subStreamID, p.extent.Name, err)
				}
			}
		}

		// Add source file entry — all entries share the same ISO path, size, checksum
		idx.index.Files = append(idx.index.Files, File{
			RelativePath: relPath,
			Size:         size,
			Checksum:     checksum,
		})

		entriesCreated++
	}

	if progress != nil {
		progress(size)
	}

	return entriesCreated, checksum, nil
}
