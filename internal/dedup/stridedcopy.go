package dedup

// stridedCopy copies count blocks of payloadSize bytes from src into
// contiguous dst. Blocks in src are separated by stride bytes
// (stride >= payloadSize). This avoids per-block copy() call overhead
// when extracting many small payloads (e.g. 184-byte M2TS PES payloads
// at 192-byte stride).
func stridedCopy(dst, src []byte, count, payloadSize, stride int) {
	dp := 0
	sp := 0
	for i := 0; i < count; i++ {
		copy(dst[dp:dp+payloadSize], src[sp:sp+payloadSize])
		dp += payloadSize
		sp += stride
	}
}
