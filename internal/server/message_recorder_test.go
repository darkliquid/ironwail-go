// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"testing"
)

func TestMessageStreamRecorder(t *testing.T) {
	rec := NewMessageStreamRecorder()
	if rec.IsEnabled() {
		t.Fatalf("expected recorder to start disabled")
	}

	// Should not record when disabled
	rec.Record(1, 0, []byte{1, 2, 3})
	if len(rec.Frames()) != 0 {
		t.Fatalf("expected 0 frames when disabled, got %d", len(rec.Frames()))
	}

	rec.SetEnabled(true)
	if !rec.IsEnabled() {
		t.Fatalf("expected recorder to be enabled")
	}

	rec.Record(1, 0, []byte{0x03, 0x01, 0x02}) // svc_updatestat, etc.
	rec.Record(2, 0, []byte{0x04, 0x05})

	frames := rec.Frames()
	if len(frames) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(frames))
	}
	if frames[0].Frame != 1 || frames[0].ByteHex != "030102" {
		t.Fatalf("unexpected frame 0: %+v", frames[0])
	}
	if frames[1].Frame != 2 || frames[1].ByteHex != "0405" {
		t.Fatalf("unexpected frame 1: %+v", frames[1])
	}

	hash1 := rec.Hash()
	if hash1 == "" {
		t.Fatalf("expected non-empty hash")
	}

	// Compare with identical second recorder
	rec2 := NewMessageStreamRecorder()
	rec2.SetEnabled(true)
	rec2.Record(1, 0, []byte{0x03, 0x01, 0x02})
	rec2.Record(2, 0, []byte{0x04, 0x05})

	diffIdx, diffReason := DiffStreams(rec.Frames(), rec2.Frames())
	if diffIdx != -1 {
		t.Fatalf("expected streams to match, got diff at %d: %s", diffIdx, diffReason)
	}
	if rec2.Hash() != hash1 {
		t.Fatalf("expected identical hash for identical message stream")
	}

	// Perturbation test
	rec3 := NewMessageStreamRecorder()
	rec3.SetEnabled(true)
	rec3.Record(1, 0, []byte{0x03, 0x01, 0x02})
	rec3.Record(2, 0, []byte{0x04, 0x06}) // Diff byte

	diffIdx3, _ := DiffStreams(rec.Frames(), rec3.Frames())
	if diffIdx3 != 1 {
		t.Fatalf("expected diff at frame index 1, got %d", diffIdx3)
	}
}
