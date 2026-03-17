// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"testing"
)

func TestDetectTrackerFormat(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"track01.mod", "mod"},
		{"track01.MOD", "mod"},
		{"track01.s3m", "s3m"},
		{"track01.S3M", "s3m"},
		{"track01.xm", "xm"},
		{"track01.XM", "xm"},
		{"track01.it", "it"},
		{"track01.IT", "it"},
		{"track01.ogg", ""},
		{"track01.mp3", ""},
		{"track01", ""},
	}

	for _, tt := range tests {
		got := detectTrackerFormat(tt.filename)
		if got != tt.want {
			t.Errorf("detectTrackerFormat(%q) = %q, want %q", tt.filename, got, tt.want)
		}
	}
}

func TestDecodeMusicTrackerUnsupportedFormat(t *testing.T) {
	_, err := decodeMusicTracker("test.unknown", []byte{0x00})
	if err == nil {
		t.Error("decodeMusicTracker should fail for unsupported format")
	}
	expectedErr := "unsupported tracker format"
	if err != nil && len(err.Error()) > 0 && err.Error()[:len(expectedErr)] != expectedErr {
		t.Errorf("expected error to start with %q, got %q", expectedErr, err.Error())
	}
}

func TestDecodeMusicTrackerInvalidData(t *testing.T) {
	// Invalid MOD data (too short)
	_, err := decodeMusicTracker("test.mod", []byte{0x00, 0x01, 0x02})
	if err == nil {
		t.Error("decodeMusicTracker should fail for invalid data")
	}
}

func TestTrackerFormatsRegisteredInMusicSystem(t *testing.T) {
	// Check that tracker formats are in the supported extensions list
	trackerExts := []string{".mod", ".s3m", ".xm", ".it"}
	for _, ext := range trackerExts {
		found := false
		for _, supported := range supportedMusicExtensions {
			if supported == ext {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("tracker extension %q not found in supportedMusicExtensions", ext)
		}
	}
}

func TestDecodeMusicTrackDispatchesTrackerFormats(t *testing.T) {
	// Test that the dispatcher in decodeMusicTrack correctly routes tracker formats
	// We expect all of these to fail (invalid data), but they should reach the tracker decoder
	trackerFiles := []struct {
		name string
		data []byte
	}{
		{"test.mod", []byte{0x00}},
		{"test.s3m", []byte{0x00}},
		{"test.xm", []byte{0x00}},
		{"test.it", []byte{0x00}},
	}

	for _, tt := range trackerFiles {
		_, err := decodeMusicTrack(tt.name, tt.data)
		if err == nil {
			t.Errorf("decodeMusicTrack(%q) should fail with invalid data", tt.name)
		}
		// The error should come from the tracker loader, not "unsupported music file type"
		if err != nil {
			errStr := err.Error()
			if errStr == "unsupported music file type for "+tt.name {
				t.Errorf("decodeMusicTrack(%q) returned wrong error: tracker format should be recognized", tt.name)
			}
		}
	}
}
