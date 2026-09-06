// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"bytes"
	"testing"
)

func TestOGGStreamInterface(t *testing.T) {
	var _ musicStream = (*oggStream)(nil)
}

func TestDecodeMusicOGGReturnsStreamingTrack(t *testing.T) {
	// Create an OGG test fixture with 256 frames of stereo 44.1kHz audio
	oggData := testMusicOGG(t, 44100, 2, 2, 256)

	track, err := decodeMusicOGG("music/test.ogg", oggData)
	if err != nil {
		t.Fatalf("decodeMusicOGG failed: %v", err)
	}

	if track.stream == nil {
		t.Fatalf("expected track.stream != nil for streaming OGG track")
	}
	if track.data != nil {
		t.Fatalf("expected track.data == nil for streaming OGG track, got len=%d", len(track.data))
	}
	if track.samples <= 0 {
		t.Fatalf("expected track.samples > 0, got %d", track.samples)
	}
	if track.rate != 44100 {
		t.Fatalf("track.rate = %d, want 44100", track.rate)
	}
	if track.channels != 2 {
		t.Fatalf("track.channels = %d, want 2", track.channels)
	}
	if track.width != 2 {
		t.Fatalf("track.width = %d, want 2", track.width)
	}

	// Verify track.stream.ReadFrames decodes frames in chunks
	frameSize := track.channels * track.width
	chunkFrames := 32
	dst1 := make([]byte, chunkFrames*frameSize)
	n1, err := track.stream.ReadFrames(dst1)
	if err != nil {
		t.Fatalf("ReadFrames chunk 1 failed: %v", err)
	}
	if n1 != chunkFrames {
		t.Fatalf("ReadFrames chunk 1 read %d frames, want %d", n1, chunkFrames)
	}

	dst2 := make([]byte, chunkFrames*frameSize)
	n2, err := track.stream.ReadFrames(dst2)
	if err != nil {
		t.Fatalf("ReadFrames chunk 2 failed: %v", err)
	}
	if n2 != chunkFrames {
		t.Fatalf("ReadFrames chunk 2 read %d frames, want %d", n2, chunkFrames)
	}

	// Verify track.stream.SeekFrame(0) resets position to 0
	if err := track.stream.SeekFrame(0); err != nil {
		t.Fatalf("SeekFrame(0) failed: %v", err)
	}

	dstRewind := make([]byte, chunkFrames*frameSize)
	nRewind, err := track.stream.ReadFrames(dstRewind)
	if err != nil {
		t.Fatalf("ReadFrames after SeekFrame(0) failed: %v", err)
	}
	if nRewind != n1 {
		t.Fatalf("ReadFrames after SeekFrame(0) read %d frames, want %d", nRewind, n1)
	}
	if !bytes.Equal(dst1, dstRewind) {
		t.Fatalf("ReadFrames after SeekFrame(0) returned different data than initial read")
	}

	if err := track.stream.Close(); err != nil {
		t.Fatalf("track.stream.Close() failed: %v", err)
	}
}

func TestDecodeMusicOGGInvalidData(t *testing.T) {
	_, err := decodeMusicOGG("music/corrupt.ogg", []byte("not an ogg file"))
	if err == nil {
		t.Fatalf("expected decodeMusicOGG to fail on invalid data")
	}
}

func TestOGGStreamEdgeCases(t *testing.T) {
	oggData := testMusicOGG(t, 44100, 2, 2, 64)
	track, err := decodeMusicOGG("music/edge.ogg", oggData)
	if err != nil {
		t.Fatalf("decodeMusicOGG failed: %v", err)
	}

	// Buffer smaller than 1 frame (4 bytes for stereo 16-bit)
	tinyDst := make([]byte, 2)
	n, err := track.stream.ReadFrames(tinyDst)
	if err != nil {
		t.Fatalf("ReadFrames with small buffer failed: %v", err)
	}
	if n != 0 {
		t.Fatalf("ReadFrames with small buffer returned %d frames, want 0", n)
	}

	// Read until EOF
	totalFrames := 0
	buf := make([]byte, 32*4)
	for {
		readN, readErr := track.stream.ReadFrames(buf)
		totalFrames += readN
		if readErr != nil {
			break
		}
	}
	if totalFrames == 0 {
		t.Fatalf("expected to read frames before EOF")
	}
}

