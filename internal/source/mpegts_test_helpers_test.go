package source

import (
	"testing"
)

// --- Helper functions for building synthetic TS/M2TS data ---

func makeM2TSPacket(pid uint16, pusi bool, afc byte, adaptLen int, cc byte, payload []byte) []byte {
	pkt := make([]byte, 192)
	pkt[4] = 0x47
	pkt[5] = byte(pid>>8) & 0x1F
	if pusi {
		pkt[5] |= 0x40
	}
	pkt[6] = byte(pid & 0xFF)
	pkt[7] = (afc << 4) | (cc & 0x0F)
	off := 8
	if afc == 0x02 || afc == 0x03 {
		pkt[off] = byte(adaptLen)
		off += 1 + adaptLen
	}
	if payload != nil && off < 192 {
		copy(pkt[off:], payload)
	}
	return pkt
}

func makeTSPacket(pid uint16, pusi bool, afc byte, adaptLen int, cc byte, payload []byte) []byte {
	pkt := make([]byte, 188)
	pkt[0] = 0x47
	pkt[1] = byte(pid>>8) & 0x1F
	if pusi {
		pkt[1] |= 0x40
	}
	pkt[2] = byte(pid & 0xFF)
	pkt[3] = (afc << 4) | (cc & 0x0F)
	off := 4
	if afc == 0x02 || afc == 0x03 {
		pkt[off] = byte(adaptLen)
		off += 1 + adaptLen
	}
	if payload != nil && off < 188 {
		copy(pkt[off:], payload)
	}
	return pkt
}

func makePATPayload(pmtPID uint16) []byte {
	p := make([]byte, 184)
	p[0] = 0x00 // pointer_field
	p[1] = 0x00 // table_id = PAT
	sectionLen := 13
	p[2] = 0xB0 | byte((sectionLen>>8)&0x0F)
	p[3] = byte(sectionLen & 0xFF)
	p[4] = 0x00 // TSID
	p[5] = 0x01
	p[6] = 0xC1 // reserved, version=0, current_next=1
	p[7] = 0x00
	p[8] = 0x00
	p[9] = 0x00 // program_number = 1
	p[10] = 0x01
	p[11] = 0xE0 | byte((pmtPID>>8)&0x1F)
	p[12] = byte(pmtPID & 0xFF)
	return p
}

func makePMTPayload(videoPID uint16, videoType byte, audioPIDs []uint16, audioTypes []byte) []byte {
	p := make([]byte, 184)
	p[0] = 0x00 // pointer_field
	p[1] = 0x02 // table_id = PMT
	numStreams := len(audioPIDs)
	if videoPID != 0 {
		numStreams++
	}
	sectionLen := 9 + numStreams*5 + 4
	p[2] = 0xB0 | byte((sectionLen>>8)&0x0F)
	p[3] = byte(sectionLen & 0xFF)
	p[4] = 0x00
	p[5] = 0x01
	p[6] = 0xC1
	p[7] = 0x00
	p[8] = 0x00
	pcrPID := videoPID
	if pcrPID == 0 && len(audioPIDs) > 0 {
		pcrPID = audioPIDs[0]
	}
	p[9] = 0xE0 | byte((pcrPID>>8)&0x1F)
	p[10] = byte(pcrPID & 0xFF)
	p[11] = 0xF0
	p[12] = 0x00
	off := 13
	if videoPID != 0 {
		p[off] = videoType
		p[off+1] = 0xE0 | byte((videoPID>>8)&0x1F)
		p[off+2] = byte(videoPID & 0xFF)
		p[off+3] = 0xF0
		p[off+4] = 0x00
		off += 5
	}
	for i, aPID := range audioPIDs {
		p[off] = audioTypes[i]
		p[off+1] = 0xE0 | byte((aPID>>8)&0x1F)
		p[off+2] = byte(aPID & 0xFF)
		p[off+3] = 0xF0
		p[off+4] = 0x00
		off += 5
	}
	return p
}

func makePESStart(streamID byte, headerDataLen int, esData []byte) []byte {
	hdrSize := 9 + headerDataLen
	out := make([]byte, hdrSize+len(esData))
	out[0] = 0x00
	out[1] = 0x00
	out[2] = 0x01
	out[3] = streamID
	out[6] = 0x80
	out[8] = byte(headerDataLen)
	copy(out[hdrSize:], esData)
	return out
}

func seqBytes(start, size int) []byte {
	b := make([]byte, size)
	for i := range b {
		b[i] = byte((start + i) & 0xFF)
	}
	return b
}

// buildBasicM2TSData creates synthetic M2TS data:
//
//	Pkt 0 (pos=0):   PAT → PMT PID 0x0100
//	Pkt 1 (pos=192): PMT → H.264 video 0x1011, AC3 audio 0x1101
//	Pkt 2 (pos=384): Video PUSI  - ES[0:175]   file offset 401
//	Pkt 3 (pos=576): Video cont  - ES[175:359] file offset 584
//	Pkt 4 (pos=768): Audio PUSI  - ES[0:175]   file offset 785
//	Pkt 5 (pos=960): Video PUSI  - ES[359:534] file offset 977
func buildBasicM2TSData() []byte {
	const (
		pmtPID   = uint16(0x0100)
		videoPID = uint16(0x1011)
		audioPID = uint16(0x1101)
	)
	var data []byte
	data = append(data, makeM2TSPacket(0, true, 0x01, 0, 0, makePATPayload(pmtPID))...)
	data = append(data, makeM2TSPacket(pmtPID, true, 0x01, 0, 0,
		makePMTPayload(videoPID, 0x1B, []uint16{audioPID}, []byte{0x81}))...)
	data = append(data, makeM2TSPacket(videoPID, true, 0x01, 0, 1,
		makePESStart(0xE0, 0, seqBytes(0, 175)))...)
	data = append(data, makeM2TSPacket(videoPID, false, 0x01, 0, 2, seqBytes(175, 184))...)
	data = append(data, makeM2TSPacket(audioPID, true, 0x01, 0, 0,
		makePESStart(0xFD, 0, seqBytes(0x80, 175)))...)
	// 359 & 0xFF = 103
	data = append(data, makeM2TSPacket(videoPID, true, 0x01, 0, 3,
		makePESStart(0xE0, 0, seqBytes(103, 175)))...)
	return data
}

// makeAC3Frame creates a synthetic AC3 frame with sync word 0B 77 at the start.
// fscod=0 (48kHz), frmsizecod=4 → frame size = 192 bytes.
func makeAC3Frame(fillByte byte) []byte {
	frame := make([]byte, 192)
	frame[0] = 0x0B
	frame[1] = 0x77
	frame[2] = 0xAA         // CRC1
	frame[3] = 0xBB         // CRC1
	frame[4] = (0 << 6) | 4 // fscod=0, frmsizecod=4
	for i := 5; i < 192; i++ {
		frame[i] = fillByte
	}
	return frame
}

// makeTrueHDUnit creates a synthetic TrueHD access unit.
// Starts with the TrueHD major sync word: F8 72 6F BA.
func makeTrueHDUnit(size int, fillByte byte) []byte {
	unit := make([]byte, size)
	if size >= 4 {
		unit[0] = 0xF8
		unit[1] = 0x72
		unit[2] = 0x6F
		unit[3] = 0xBA
	}
	for i := 4; i < size; i++ {
		unit[i] = fillByte
	}
	return unit
}

// buildTrueHDAC3M2TSData creates M2TS data with a combined TrueHD+AC3 stream.
// The PES payload contains: [AC3 frame][TrueHD unit][AC3 frame][TrueHD unit]
//
// Payload sizes are chosen so the total exactly fills M2TS packets:
// First PUSI packet carries 175 bytes ES (184 - 9 PES header).
// Continuations carry 184 bytes each. Total = 175 + 4×184 = 911 bytes.
// AC3: 2 × 192 = 384 bytes. TrueHD: 300 + 227 = 527 bytes.
func buildTrueHDAC3M2TSData() []byte {
	const (
		pmtPID   = uint16(0x0100)
		videoPID = uint16(0x1011)
		audioPID = uint16(0x1101)
	)

	// Build combined TrueHD+AC3 payload (911 bytes = 175 + 4×184)
	var audioPayload []byte
	audioPayload = append(audioPayload, makeAC3Frame(0x11)...)        // 192 bytes AC3
	audioPayload = append(audioPayload, makeTrueHDUnit(300, 0x22)...) // 300 bytes TrueHD
	audioPayload = append(audioPayload, makeAC3Frame(0x33)...)        // 192 bytes AC3
	audioPayload = append(audioPayload, makeTrueHDUnit(227, 0x44)...) // 227 bytes TrueHD
	// Total: 911 bytes

	var data []byte
	data = append(data, makeM2TSPacket(0, true, 0x01, 0, 0, makePATPayload(pmtPID))...)
	data = append(data, makeM2TSPacket(pmtPID, true, 0x01, 0, 0,
		makePMTPayload(videoPID, 0x1B,
			[]uint16{audioPID},
			[]byte{0x83}))...) // 0x83 = TrueHD

	// Video PUSI
	data = append(data, makeM2TSPacket(videoPID, true, 0x01, 0, 1,
		makePESStart(0xE0, 0, seqBytes(0, 175)))...)

	// Audio PUSI - PES header + start of audioPayload
	pesHdr := makePESStart(0xFD, 0, nil) // 9-byte PES header
	firstChunkSize := 184 - len(pesHdr)  // 175 bytes
	firstPayload := make([]byte, 184)
	copy(firstPayload, pesHdr)
	copy(firstPayload[len(pesHdr):], audioPayload[:firstChunkSize])
	data = append(data, makeM2TSPacket(audioPID, true, 0x01, 0, 0, firstPayload)...)

	// Audio continuation packets for remaining data
	remaining := audioPayload[firstChunkSize:]
	cc := byte(1)
	for len(remaining) > 0 {
		chunkSize := 184
		if chunkSize > len(remaining) {
			chunkSize = len(remaining)
		}
		chunk := make([]byte, 184)
		copy(chunk, remaining[:chunkSize])
		data = append(data, makeM2TSPacket(audioPID, false, 0x01, 0, cc, chunk)...)
		remaining = remaining[chunkSize:]
		cc++
	}

	return data
}

// testStream describes a stream for building test M2TS data.
type testStream struct {
	streamType byte
}

// buildTestM2TS creates synthetic M2TS data with a PAT and PMT containing
// the specified streams. Uses 192-byte M2TS packets.
func buildTestM2TS(t *testing.T, streams []testStream) []byte {
	t.Helper()

	const packetSize = 192
	const tsPayloadStart = 4 // M2TS timestamp prefix

	// Need at least 4 packets for detectTSPacketSize to verify sync pattern.
	// Packet 1: PAT, Packet 2: PMT, Packets 3+: null padding
	numPackets := 6
	data := make([]byte, packetSize*numPackets)

	// Add sync bytes to all packets (null/padding packets)
	for i := 0; i < numPackets; i++ {
		data[i*packetSize+tsPayloadStart] = 0x47
		// Set PID to 0x1FFF (null packet) with no payload
		data[i*packetSize+tsPayloadStart+1] = 0x1F
		data[i*packetSize+tsPayloadStart+2] = 0xFF
		data[i*packetSize+tsPayloadStart+3] = 0x10
	}

	// Packet 1: PAT
	patOffset := 0
	// 4-byte M2TS timestamp (zeros)
	ts := data[patOffset+tsPayloadStart:]
	ts[0] = 0x47                   // Sync byte
	ts[1] = 0x40                   // Payload unit start + PID high = 0
	ts[2] = 0x00                   // PID low = 0 (PAT)
	ts[3] = 0x10                   // No adaptation, payload only
	ts[4] = 0x00                   // Pointer field = 0
	ts[5] = 0x00                   // Table ID = 0 (PAT)
	ts[6] = 0xB0                   // Section syntax + length high
	ts[7] = 0x0D                   // Section length = 13 (5 header + 4 program + 4 CRC)
	ts[8] = 0x00                   // Transport stream ID high
	ts[9] = 0x01                   // Transport stream ID low
	ts[10] = 0xC1                  // Version 0, current
	ts[11] = 0x00                  // Section number
	ts[12] = 0x00                  // Last section number
	ts[13] = 0x00                  // Program number high = 0
	ts[14] = 0x01                  // Program number low = 1
	ts[15] = 0xE0 | byte(0x100>>8) // PMT PID high (0x100)
	ts[16] = byte(0x100 & 0xFF)    // PMT PID low
	// CRC (just zeros for test — we don't validate CRC)

	// Packet 2: PMT (PID 0x100)
	pmtOffset := packetSize
	ts = data[pmtOffset+tsPayloadStart:]
	ts[0] = 0x47 // Sync byte
	ts[1] = 0x41 // Payload unit start + PID high = 1
	ts[2] = 0x00 // PID low = 0x00 → PID = 0x100
	ts[3] = 0x10 // No adaptation, payload only
	ts[4] = 0x00 // Pointer field = 0
	ts[5] = 0x02 // Table ID = 2 (PMT)

	// Build PMT section
	// Section: 5 bytes header + 4 bytes (PCR PID + prog info len) + 5*len(streams) + 4 CRC
	sectionLen := 5 + 4 + 5*len(streams) + 4
	ts[6] = 0xB0 | byte(sectionLen>>8) // Section syntax + length high
	ts[7] = byte(sectionLen)           // Section length low
	ts[8] = 0x00                       // Program number high
	ts[9] = 0x01                       // Program number low
	ts[10] = 0xC1                      // Version 0, current
	ts[11] = 0x00                      // Section number
	ts[12] = 0x00                      // Last section number
	ts[13] = 0xE0 | 0x01               // PCR PID high (0x101)
	ts[14] = 0x01                      // PCR PID low
	ts[15] = 0xF0                      // Program info length high
	ts[16] = 0x00                      // Program info length low = 0

	// Stream descriptors
	offset := 17
	for i, s := range streams {
		pid := 0x101 + i
		ts[offset] = s.streamType          // stream_type
		ts[offset+1] = 0xE0 | byte(pid>>8) // elementary PID high
		ts[offset+2] = byte(pid & 0xFF)    // elementary PID low
		ts[offset+3] = 0xF0                // ES info length high
		ts[offset+4] = 0x00                // ES info length low = 0
		offset += 5
	}

	return data
}
