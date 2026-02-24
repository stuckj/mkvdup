package source

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/stuckj/mkvdup/internal/mkv"
)

// CodecType represents a broad codec family.
type CodecType int

// Codec type constants.
const (
	CodecUnknown CodecType = iota
	CodecMPEG1Video
	CodecMPEG2Video
	CodecH264Video
	CodecH265Video
	CodecVC1Video
	CodecAC3Audio
	CodecEAC3Audio
	CodecDTSAudio
	CodecDTSHDAudio
	CodecTrueHDAudio
	CodecLPCMAudio
	CodecMPEGAudio
	CodecAACaudio
	CodecFLACAudio
	CodecOpusAudio
	CodecPGSSubtitle
)

// CodecTypeName returns a human-readable name for a codec type.
func CodecTypeName(ct CodecType) string {
	switch ct {
	case CodecMPEG1Video:
		return "MPEG-1"
	case CodecMPEG2Video:
		return "MPEG-2"
	case CodecH264Video:
		return "H.264"
	case CodecH265Video:
		return "H.265"
	case CodecVC1Video:
		return "VC-1"
	case CodecAC3Audio:
		return "AC3"
	case CodecEAC3Audio:
		return "E-AC3"
	case CodecDTSAudio:
		return "DTS"
	case CodecDTSHDAudio:
		return "DTS-HD"
	case CodecTrueHDAudio:
		return "TrueHD"
	case CodecLPCMAudio:
		return "LPCM"
	case CodecMPEGAudio:
		return "MPEG Audio"
	case CodecAACaudio:
		return "AAC"
	case CodecFLACAudio:
		return "FLAC"
	case CodecOpusAudio:
		return "Opus"
	case CodecPGSSubtitle:
		return "PGS"
	default:
		return "Unknown"
	}
}

// IsVideoCodec returns true if the codec type is a video codec.
func IsVideoCodec(ct CodecType) bool {
	switch ct {
	case CodecMPEG1Video, CodecMPEG2Video, CodecH264Video, CodecH265Video, CodecVC1Video:
		return true
	}
	return false
}

// IsSubtitleCodec returns true if the codec type is a subtitle codec.
func IsSubtitleCodec(ct CodecType) bool {
	return ct == CodecPGSSubtitle
}

// IsAudioCodec returns true if the codec type is an audio codec.
func IsAudioCodec(ct CodecType) bool {
	switch ct {
	case CodecAC3Audio, CodecEAC3Audio, CodecDTSAudio, CodecDTSHDAudio,
		CodecTrueHDAudio, CodecLPCMAudio, CodecMPEGAudio, CodecAACaudio,
		CodecFLACAudio, CodecOpusAudio:
		return true
	}
	return false
}

// MKVCodecToType maps an MKV CodecID string to a CodecType.
func MKVCodecToType(codecID string) CodecType {
	switch {
	case codecID == "V_MPEG1":
		return CodecMPEG1Video
	case codecID == "V_MPEG2":
		return CodecMPEG2Video
	case codecID == "V_MPEG4/ISO/AVC":
		return CodecH264Video
	case codecID == "V_MPEGH/ISO/HEVC":
		return CodecH265Video
	case codecID == "V_MS/VFW/FOURCC":
		// Could be VC-1 or other; can't determine without codec private data
		return CodecUnknown
	case codecID == "A_AC3":
		return CodecAC3Audio
	case codecID == "A_EAC3":
		return CodecEAC3Audio
	case codecID == "A_DTS":
		return CodecDTSAudio
	case strings.HasPrefix(codecID, "A_DTS/"):
		// A_DTS/EXPRESS, A_DTS/LOSSLESS, etc.
		return CodecDTSHDAudio
	case codecID == "A_TRUEHD":
		return CodecTrueHDAudio
	case strings.HasPrefix(codecID, "A_PCM/"):
		// A_PCM/INT/LIT, A_PCM/INT/BIG, A_PCM/FLOAT/IEEE
		return CodecLPCMAudio
	case strings.HasPrefix(codecID, "A_MPEG/"):
		// A_MPEG/L2, A_MPEG/L3
		return CodecMPEGAudio
	case strings.HasPrefix(codecID, "A_AAC"):
		// A_AAC, A_AAC/MPEG2/MAIN, etc.
		return CodecAACaudio
	case codecID == "A_FLAC":
		return CodecFLACAudio
	case codecID == "A_OPUS":
		return CodecOpusAudio
	case codecID == "S_HDMV/PGS":
		return CodecPGSSubtitle
	default:
		return CodecUnknown
	}
}

// SourceCodecs describes the codecs found in a source media.
type SourceCodecs struct {
	VideoCodecs    []CodecType
	AudioCodecs    []CodecType
	SubtitleCodecs []CodecType
}

// CodecMismatch describes a detected codec mismatch between MKV and source.
type CodecMismatch struct {
	TrackType    string      // "video" or "audio"
	MKVCodecID   string      // e.g. "V_MPEG4/ISO/AVC"
	MKVCodecType CodecType   // resolved codec type
	SourceCodecs []CodecType // codecs found in source for this track type
}

// DetectSourceCodecs determines what codecs are present in the source media.
// For DVD sources, it extracts codec info from the already-parsed MPEG-PS data.
// For Blu-ray sources, it performs a lightweight PMT scan of the first M2TS file.
func DetectSourceCodecs(index *Index) (*SourceCodecs, error) {
	switch index.SourceType {
	case TypeDVD:
		return detectDVDCodecs(index)
	case TypeBluray:
		return detectBlurayCodecs(index)
	default:
		return nil, fmt.Errorf("unknown source type")
	}
}

// DetectSourceCodecsFromDir performs a lightweight codec detection from a source
// directory without building the full hash index. This allows codec compatibility
// checks to run before the expensive indexing step.
func DetectSourceCodecsFromDir(sourceDir string) (*SourceCodecs, error) {
	sourceType, err := DetectType(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("detect source type: %w", err)
	}

	files, err := EnumerateMediaFiles(sourceDir, sourceType)
	if err != nil {
		return nil, fmt.Errorf("enumerate files: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no media files found in %s", sourceDir)
	}

	// Find the largest file (most likely the main feature)
	var largestFile string
	var largestSize int64
	for _, f := range files {
		fullPath := filepath.Join(sourceDir, f)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}
		if info.Size() > largestSize {
			largestSize = info.Size()
			largestFile = f
		}
	}
	if largestFile == "" {
		return nil, fmt.Errorf("no accessible media files found")
	}

	fullPath := filepath.Join(sourceDir, largestFile)

	switch sourceType {
	case TypeBluray:
		return detectBlurayCodecsFromFile(fullPath)
	case TypeDVD:
		return detectDVDCodecsFromFile(fullPath)
	default:
		return nil, fmt.Errorf("unknown source type")
	}
}

// CheckCodecCompatibility compares MKV track codecs against source codecs.
// Returns nil if all codecs are compatible, or a list of mismatches.
func CheckCodecCompatibility(tracks []mkv.Track, sourceCodecs *SourceCodecs) []CodecMismatch {
	var mismatches []CodecMismatch

	for _, track := range tracks {
		ct := MKVCodecToType(track.CodecID)
		if ct == CodecUnknown {
			continue // Skip unknown codecs — no false alarms
		}

		if track.Type == mkv.TrackTypeVideo && IsVideoCodec(ct) {
			if len(sourceCodecs.VideoCodecs) == 0 {
				continue // No source video info available
			}
			if !codecFamilyMatch(ct, sourceCodecs.VideoCodecs) {
				mismatches = append(mismatches, CodecMismatch{
					TrackType:    "video",
					MKVCodecID:   track.CodecID,
					MKVCodecType: ct,
					SourceCodecs: sourceCodecs.VideoCodecs,
				})
			}
		} else if track.Type == mkv.TrackTypeAudio && IsAudioCodec(ct) {
			if len(sourceCodecs.AudioCodecs) == 0 {
				continue // No source audio info available
			}
			if !codecFamilyMatch(ct, sourceCodecs.AudioCodecs) {
				mismatches = append(mismatches, CodecMismatch{
					TrackType:    "audio",
					MKVCodecID:   track.CodecID,
					MKVCodecType: ct,
					SourceCodecs: sourceCodecs.AudioCodecs,
				})
			}
		} else if track.Type == mkv.TrackTypeSubtitle && IsSubtitleCodec(ct) {
			if len(sourceCodecs.SubtitleCodecs) == 0 {
				continue // No source subtitle info available
			}
			if !codecFamilyMatch(ct, sourceCodecs.SubtitleCodecs) {
				mismatches = append(mismatches, CodecMismatch{
					TrackType:    "subtitle",
					MKVCodecID:   track.CodecID,
					MKVCodecType: ct,
					SourceCodecs: sourceCodecs.SubtitleCodecs,
				})
			}
		}
	}

	return mismatches
}

// codecFamilyMatch checks if a codec type is compatible with any codec in the list.
// Uses family-based matching (e.g., DTS is compatible with DTS-HD).
func codecFamilyMatch(ct CodecType, sourceCodecs []CodecType) bool {
	family := codecFamily(ct)
	for _, sc := range sourceCodecs {
		if codecFamily(sc) == family {
			return true
		}
	}
	return false
}

// codecFamily returns the codec family for family-based matching.
// Related codecs map to the same family value.
func codecFamily(ct CodecType) int {
	switch ct {
	case CodecMPEG1Video, CodecMPEG2Video:
		return 1
	case CodecH264Video:
		return 2
	case CodecH265Video:
		return 3
	case CodecVC1Video:
		return 4
	case CodecAC3Audio, CodecEAC3Audio:
		return 10
	case CodecDTSAudio, CodecDTSHDAudio:
		return 11
	case CodecTrueHDAudio:
		return 12
	case CodecLPCMAudio:
		return 13
	case CodecMPEGAudio:
		return 14
	case CodecAACaudio:
		return 15
	case CodecFLACAudio:
		return 16
	case CodecOpusAudio:
		return 17
	case CodecPGSSubtitle:
		return 20
	default:
		return 0
	}
}

// containsCodec checks if a codec type is already in the list.
func containsCodec(codecs []CodecType, ct CodecType) bool {
	for _, c := range codecs {
		if c == ct {
			return true
		}
	}
	return false
}
