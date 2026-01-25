package source

import (
	"math/rand/v2"
	"testing"
)

// createVideoDataWithStartCodes creates test data with video start codes (00 00 01 XX)
// at regular intervals for benchmarking.
func createVideoDataWithStartCodes(size int, startCodeInterval int) []byte {
	data := make([]byte, size)
	// Fill with random-ish data that won't accidentally create start codes
	rng := rand.New(rand.NewPCG(42, 0))
	for i := range data {
		// Avoid 0x00 to prevent accidental start codes
		data[i] = byte(rng.IntN(255)) + 1
	}

	// Insert start codes at regular intervals
	for i := 0; i+4 <= size; i += startCodeInterval {
		data[i] = 0x00
		data[i+1] = 0x00
		data[i+2] = 0x01
		data[i+3] = 0xE0 // Video stream ID
	}

	return data
}

// createAudioDataWithSyncPoints creates test data with AC3 sync patterns (0B 77)
// at regular intervals for benchmarking.
func createAudioDataWithSyncPoints(size int, syncInterval int) []byte {
	data := make([]byte, size)
	// Fill with random-ish data that won't accidentally create sync patterns
	rng := rand.New(rand.NewPCG(42, 0))
	for i := range data {
		// Avoid sync pattern bytes
		b := byte(rng.IntN(256))
		if b == 0x0B || b == 0x77 || b == 0xFF || b == 0x7F {
			b = 0x42
		}
		data[i] = b
	}

	// Insert AC3 sync patterns at regular intervals
	for i := 0; i+2 <= size; i += syncInterval {
		data[i] = 0x0B
		data[i+1] = 0x77
	}

	return data
}

// BenchmarkFindVideoStartCodes_Large benchmarks video start code detection with larger data.
// Complements the existing 1MB benchmark in video_test.go with a 10MB test.
func BenchmarkFindVideoStartCodes_Large(b *testing.B) {
	// 10MB of data with start codes every 2KB (5120 start codes)
	data := createVideoDataWithStartCodes(10*1024*1024, 2048)

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))

	for b.Loop() {
		offsets := FindVideoStartCodes(data)
		if len(offsets) == 0 {
			b.Fatal("expected to find start codes")
		}
	}
}

// BenchmarkFindAudioSyncPoints_Large benchmarks audio sync point detection with larger data.
// Complements the existing 1MB benchmark in audio_test.go with a 10MB test.
func BenchmarkFindAudioSyncPoints_Large(b *testing.B) {
	// 10MB of data with sync points every 2KB (5120 sync points)
	data := createAudioDataWithSyncPoints(10*1024*1024, 2048)

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))

	for b.Loop() {
		offsets := FindAudioSyncPoints(data)
		if len(offsets) == 0 {
			b.Fatal("expected to find sync points")
		}
	}
}

// BenchmarkFindAllSyncPoints_Large benchmarks combined sync point detection with larger data.
func BenchmarkFindAllSyncPoints_Large(b *testing.B) {
	// Create data with both video start codes and audio sync points
	// 10MB with mixed patterns every 4KB
	size := 10 * 1024 * 1024
	data := make([]byte, size)
	rng := rand.New(rand.NewPCG(42, 0))
	for i := range data {
		b := byte(rng.IntN(256))
		if b == 0x00 || b == 0x01 || b == 0x0B || b == 0x77 || b == 0xFF {
			b = 0x42
		}
		data[i] = b
	}

	// Insert alternating video and audio sync points
	for i := 0; i+4 <= size; i += 4096 {
		if (i/4096)%2 == 0 {
			// Video start code
			data[i] = 0x00
			data[i+1] = 0x00
			data[i+2] = 0x01
			data[i+3] = 0xE0
		} else {
			// AC3 sync
			data[i] = 0x0B
			data[i+1] = 0x77
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))

	for b.Loop() {
		offsets := FindAllSyncPoints(data)
		if len(offsets) == 0 {
			b.Fatal("expected to find sync points")
		}
	}
}
