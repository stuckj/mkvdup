// Package bitshift provides bit-shift transform operations for recovering
// NAL units whose headers were modified by video extraction tools.
//
// When extraction tools modify VLC-coded fields in H.264 slice headers or
// VC-1 frame headers, all subsequent bits shift by 1-7 positions. The
// relationship between source and MKV bytes after the divergence point is:
//
//	mkv[j] = (src[j] << shift) | (src[j+1] >> (8 - shift))
//
// This package provides the forward transform (source → MKV) used during both
// matching (to verify a candidate shift) and reconstruction (FUSE reads).
package bitshift

// Apply transforms source bytes into destination bytes using the given bit-shift
// amount. Each output byte is formed by left-shifting the corresponding source
// byte and OR-ing with the right-shifted next source byte:
//
//	dst[j] = (src[j] << shift) | (src[j+1] >> (8 - shift))
//
// src must be at least len(dst)+1 bytes long to provide the extra byte needed
// for the last output position. shift must be 1-7; other values produce
// undefined results.
func Apply(src []byte, shift uint8, dst []byte) {
	rshift := 8 - shift
	for j := range dst {
		dst[j] = (src[j] << shift) | (src[j+1] >> rshift)
	}
}

// Verify checks whether applying the given bit-shift to src produces bytes
// matching mkv. Returns true if all transformed bytes match. src must be at
// least len(mkv)+1 bytes long. shift must be 1-7.
func Verify(src []byte, shift uint8, mkv []byte) bool {
	rshift := 8 - shift
	for j := range mkv {
		if (src[j]<<shift)|(src[j+1]>>rshift) != mkv[j] {
			return false
		}
	}
	return true
}
