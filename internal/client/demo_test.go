package client

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDemoRecordingOpenClose(t *testing.T) {
	// Setup: ensure demos directory is clean
	demoDir := "demos"
	testFile := "test_record"
	defer func() {
		os.RemoveAll(demoDir)
	}()

	demo := NewDemoState()

	// Start recording
	if err := demo.StartDemoRecording(testFile, 2); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}

	if !demo.Recording {
		t.Error("Recording flag not set")
	}

	if demo.CDTrack != 2 {
		t.Errorf("CDTrack = %d, want 2", demo.CDTrack)
	}

	// Verify file exists
	expectedPath := filepath.Join(demoDir, testFile+".dem")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Demo file not created at %s", expectedPath)
	}

	// Stop recording
	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	if demo.Recording {
		t.Error("Recording flag still set after stop")
	}

	// Verify file still exists after stopping
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Demo file disappeared after stop")
	}
}

func TestDemoRecordingAlreadyRecording(t *testing.T) {
	defer os.RemoveAll("demos")

	demo := NewDemoState()

	if err := demo.StartDemoRecording("test1", 0); err != nil {
		t.Fatalf("First StartDemoRecording failed: %v", err)
	}
	defer demo.StopRecording()

	// Try to start recording again
	if err := demo.StartDemoRecording("test2", 0); err == nil {
		t.Error("Expected error when starting recording while already recording")
	}
}

func TestDemoFrameRoundTrip(t *testing.T) {
	defer os.RemoveAll("demos")

	// Test data: 10 frames with known data
	testFrames := []struct {
		message    []byte
		viewAngles [3]float32
	}{
		{[]byte{0x01, 0x02, 0x03}, [3]float32{0.0, 0.0, 0.0}},
		{[]byte{0x04, 0x05, 0x06, 0x07}, [3]float32{10.0, 20.0, 30.0}},
		{[]byte{0x08}, [3]float32{-45.0, 90.0, 180.0}},
		{[]byte{0x09, 0x0a, 0x0b, 0x0c, 0x0d}, [3]float32{0.5, 1.5, 2.5}},
		{[]byte{0x0e, 0x0f}, [3]float32{100.0, 200.0, 300.0}},
		{[]byte{0x10, 0x11, 0x12, 0x13}, [3]float32{-90.0, -180.0, 0.0}},
		{[]byte{0x14}, [3]float32{0.0, 0.0, 0.0}},
		{[]byte{0x15, 0x16, 0x17, 0x18, 0x19, 0x1a}, [3]float32{45.0, 45.0, 45.0}},
		{[]byte{0x1b, 0x1c, 0x1d}, [3]float32{1.0, 2.0, 3.0}},
		{[]byte{0x1e, 0x1f, 0x20, 0x21}, [3]float32{360.0, 720.0, 1080.0}},
	}

	demo := NewDemoState()

	// Record frames
	if err := demo.StartDemoRecording("roundtrip", 5); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}

	for i, frame := range testFrames {
		if err := demo.WriteDemoFrame(frame.message, frame.viewAngles); err != nil {
			t.Fatalf("WriteDemoFrame %d failed: %v", i, err)
		}
	}

	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	// Playback and verify
	if err := demo.StartDemoPlayback("roundtrip"); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}

	if demo.CDTrack != 5 {
		t.Errorf("Playback CDTrack = %d, want 5", demo.CDTrack)
	}

	for i, expectedFrame := range testFrames {
		msg, angles, err := demo.ReadDemoFrame()
		if err != nil {
			t.Fatalf("ReadDemoFrame %d failed: %v", i, err)
		}

		if !bytes.Equal(msg, expectedFrame.message) {
			t.Errorf("Frame %d message mismatch: got %v, want %v", i, msg, expectedFrame.message)
		}

		for j := 0; j < 3; j++ {
			if angles[j] != expectedFrame.viewAngles[j] {
				t.Errorf("Frame %d angle[%d] = %f, want %f", i, j, angles[j], expectedFrame.viewAngles[j])
			}
		}
	}

	// Should get EOF after reading all frames
	_, _, err := demo.ReadDemoFrame()
	if err == nil {
		t.Error("Expected EOF after reading all frames")
	}

	if err := demo.StopPlayback(); err != nil {
		t.Fatalf("StopPlayback failed: %v", err)
	}
}

func TestDemoHeaderValidation(t *testing.T) {
	defer os.RemoveAll("demos")

	demo := NewDemoState()

	// Record a simple demo with specific CD track
	cdtrack := 42
	if err := demo.StartDemoRecording("header_test", cdtrack); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}

	// Write a single frame
	testMsg := []byte{0xaa, 0xbb, 0xcc}
	testAngles := [3]float32{1.0, 2.0, 3.0}
	if err := demo.WriteDemoFrame(testMsg, testAngles); err != nil {
		t.Fatalf("WriteDemoFrame failed: %v", err)
	}

	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	// Read and verify header
	if err := demo.StartDemoPlayback("header_test"); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}

	if demo.CDTrack != cdtrack {
		t.Errorf("Header CDTrack = %d, want %d", demo.CDTrack, cdtrack)
	}

	if !demo.Playback {
		t.Error("Playback flag not set")
	}

	if demo.Recording {
		t.Error("Recording flag should not be set during playback")
	}

	// Verify we can read the frame
	msg, angles, err := demo.ReadDemoFrame()
	if err != nil {
		t.Fatalf("ReadDemoFrame failed: %v", err)
	}

	if !bytes.Equal(msg, testMsg) {
		t.Errorf("Message mismatch: got %v, want %v", msg, testMsg)
	}

	for i := 0; i < 3; i++ {
		if angles[i] != testAngles[i] {
			t.Errorf("Angle[%d] = %f, want %f", i, angles[i], testAngles[i])
		}
	}

	if err := demo.StopPlayback(); err != nil {
		t.Fatalf("StopPlayback failed: %v", err)
	}
}

func TestDemoPlaybackSequence(t *testing.T) {
	defer os.RemoveAll("demos")

	// Create a demo with a sequence of messages that simulate a game session
	demo := NewDemoState()

	if err := demo.StartDemoRecording("sequence_test", 0); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}

	// Simulate a sequence of frames with varying view angles (camera rotation)
	numFrames := 20
	for i := 0; i < numFrames; i++ {
		angle := float32(i * 18) // Rotate 18 degrees per frame
		msg := []byte{byte(i), byte(i * 2)}
		angles := [3]float32{angle, 0.0, 0.0}

		if err := demo.WriteDemoFrame(msg, angles); err != nil {
			t.Fatalf("WriteDemoFrame %d failed: %v", i, err)
		}
	}

	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	// Playback and verify sequence
	if err := demo.StartDemoPlayback("sequence_test"); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}

	for i := 0; i < numFrames; i++ {
		msg, angles, err := demo.ReadDemoFrame()
		if err != nil {
			t.Fatalf("ReadDemoFrame %d failed: %v", i, err)
		}

		expectedMsg := []byte{byte(i), byte(i * 2)}
		if !bytes.Equal(msg, expectedMsg) {
			t.Errorf("Frame %d message mismatch: got %v, want %v", i, msg, expectedMsg)
		}

		expectedAngle := float32(i * 18)
		if angles[0] != expectedAngle {
			t.Errorf("Frame %d angle = %f, want %f", i, angles[0], expectedAngle)
		}
	}

	if err := demo.StopPlayback(); err != nil {
		t.Fatalf("StopPlayback failed: %v", err)
	}
}

func TestDemoPlaybackNonExistentFile(t *testing.T) {
	demo := NewDemoState()

	if err := demo.StartDemoPlayback("nonexistent_demo_file"); err == nil {
		t.Error("Expected error when opening nonexistent demo file")
		demo.StopPlayback()
	}
}

func TestDemoCannotRecordDuringPlayback(t *testing.T) {
	defer os.RemoveAll("demos")

	demo := NewDemoState()

	// Create a demo first
	if err := demo.StartDemoRecording("test_conflict", 0); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}
	demo.WriteDemoFrame([]byte{0x01}, [3]float32{0, 0, 0})
	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	// Start playback
	if err := demo.StartDemoPlayback("test_conflict"); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}
	defer demo.StopPlayback()

	// Try to start recording during playback
	if err := demo.StartDemoRecording("test_conflict2", 0); err == nil {
		t.Error("Expected error when starting recording during playback")
		demo.StopRecording()
	}
}

func TestDemoEmptyFile(t *testing.T) {
	defer os.RemoveAll("demos")

	demo := NewDemoState()

	// Record an empty demo (no frames)
	if err := demo.StartDemoRecording("empty_demo", 1); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}

	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	// Try to play it back
	if err := demo.StartDemoPlayback("empty_demo"); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}

	if demo.CDTrack != 1 {
		t.Errorf("CDTrack = %d, want 1", demo.CDTrack)
	}

	// Should immediately get EOF when trying to read
	_, _, err := demo.ReadDemoFrame()
	if err == nil {
		t.Error("Expected EOF when reading empty demo")
	}

	if err := demo.StopPlayback(); err != nil {
		t.Fatalf("StopPlayback failed: %v", err)
	}
}
