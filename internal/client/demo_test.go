// What: Demo recording and playback tests.
// Why: Verifies that gameplay sessions can be saved to and loaded from disk.
// Where in C: cl_demo.c

package client

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	inet "github.com/ironwail/ironwail-go/internal/net"
)

// TestDemoRecordingOpenClose verifies that demo recording can be started and stopped correctly.
// Why: Core functionality for saving gameplay sessions depends on reliable file lifecycle management.
// Where in C: cl_demo.c, CL_BeginRecord_f, CL_Stop_f.
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

// TestDemoRecordingAlreadyRecording ensures that starting a recording when one is already active returns an error.
// Why: Prevents accidental overwriting of ongoing recordings or internal state corruption.
// Where in C: cl_demo.c, CL_BeginRecord_f.
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

// TestDemoFrameRoundTrip verifies that individual demo frames (server messages and view angles) are correctly serialized and deserialized.
// Why: Demos must preserve the exact state of server messages for accurate playback.
// Where in C: cl_demo.c, CL_WriteDemoMessage, CL_ReadDemoMessage.
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

// TestDemoHeaderValidation ensures that demo files have the correct format and metadata (like CD track).
// Why: Prevents the engine from attempting to play incompatible or corrupted demo files.
// Where in C: cl_demo.c, CL_OpenDemo.
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

// TestDemoPlaybackSequence verifies that a series of recorded frames are played back in the correct order.
// Why: Ensures temporal consistency during demo playback.
// Where in C: cl_demo.c, CL_ReadDemoMessage.
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

// TestDemoSeekFrameReplaysFromOffset verifies that seeking to a specific frame correctly restarts playback from that point.
// Why: Essential for user-friendly demo navigation such as fast-forwarding or rewinding.
// Where in C: cl_demo.c.
func TestDemoSeekFrameReplaysFromOffset(t *testing.T) {
	defer os.RemoveAll("demos")

	demo := NewDemoState()
	if err := demo.StartDemoRecording("seek_test", 0); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}
	for i := 0; i < 4; i++ {
		msg := []byte{byte(i), byte(i + 1)}
		angles := [3]float32{float32(i), 0, 0}
		if err := demo.WriteDemoFrame(msg, angles); err != nil {
			t.Fatalf("WriteDemoFrame %d failed: %v", i, err)
		}
	}
	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	if err := demo.StartDemoPlayback("seek_test"); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}
	defer demo.StopPlayback()

	for i := 0; i < 4; i++ {
		if _, _, err := demo.ReadDemoFrame(); err != nil {
			t.Fatalf("ReadDemoFrame %d failed: %v", i, err)
		}
	}

	if err := demo.SeekFrame(1); err != nil {
		t.Fatalf("SeekFrame failed: %v", err)
	}
	msg, angles, err := demo.ReadDemoFrame()
	if err != nil {
		t.Fatalf("ReadDemoFrame after seek failed: %v", err)
	}
	if !bytes.Equal(msg, []byte{1, 2}) {
		t.Fatalf("seeked frame message = %v, want [1 2]", msg)
	}
	if angles[0] != 1 {
		t.Fatalf("seeked frame angle = %v, want 1", angles[0])
	}
}

// TestDemoPlaybackIndexesFramesAtStart verifies that the demo system indexes all frames when a demo is opened.
// Why: Large demo files require an index for efficient random-access seeking.
// Where in C: cl_demo.c.
func TestDemoPlaybackIndexesFramesAtStart(t *testing.T) {
	defer os.RemoveAll("demos")

	demo := NewDemoState()
	if err := demo.StartDemoRecording("indexed_seek", 0); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}
	for i := 0; i < 3; i++ {
		msg := []byte{byte(i), byte(i + 1)}
		angles := [3]float32{float32(i), 0, 0}
		if err := demo.WriteDemoFrame(msg, angles); err != nil {
			t.Fatalf("WriteDemoFrame %d failed: %v", i, err)
		}
	}
	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	if err := demo.StartDemoPlayback("indexed_seek"); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}
	defer demo.StopPlayback()

	if got := len(demo.Frames); got != 3 {
		t.Fatalf("indexed frame count = %d, want 3", got)
	}
	if err := demo.SeekFrame(2); err != nil {
		t.Fatalf("SeekFrame failed: %v", err)
	}
	msg, angles, err := demo.ReadDemoFrame()
	if err != nil {
		t.Fatalf("ReadDemoFrame after seek failed: %v", err)
	}
	if !bytes.Equal(msg, []byte{2, 3}) {
		t.Fatalf("seeked frame message = %v, want [2 3]", msg)
	}
	if angles[0] != 2 {
		t.Fatalf("seeked frame angle = %v, want 2", angles[0])
	}
}

// TestWriteDisconnectTrailerRoundTrip verifies that the end-of-demo disconnect marker is correctly written and read.
// Why: Signals the playback system to stop when the recorded session ends.
// Where in C: cl_demo.c, CL_Stop_f.
func TestWriteDisconnectTrailerRoundTrip(t *testing.T) {
	defer os.RemoveAll("demos")

	demo := NewDemoState()
	if err := demo.StartDemoRecording("disconnect_trailer", 0); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}

	wantAngles := [3]float32{4, 5, 6}
	if err := demo.WriteDisconnectTrailer(wantAngles); err != nil {
		t.Fatalf("WriteDisconnectTrailer failed: %v", err)
	}
	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	if err := demo.StartDemoPlayback("disconnect_trailer"); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}
	defer demo.StopPlayback()

	message, angles, err := demo.ReadDemoFrame()
	if err != nil {
		t.Fatalf("ReadDemoFrame failed: %v", err)
	}
	if len(message) != 1 || message[0] != inet.SVCDisconnect {
		t.Fatalf("disconnect message = %v, want [%d]", message, inet.SVCDisconnect)
	}
	if angles != wantAngles {
		t.Fatalf("disconnect angles = %v, want %v", angles, wantAngles)
	}
}

// TestWriteInitialStateSnapshotRoundTrip verifies that a comprehensive snapshot of the game state is recorded at the start.
// Why: Allows a demo to be played back from the beginning by reconstructing the initial environment (models, sounds, player state).
// Where in C: cl_demo.c, CL_BeginRecord_f.
func TestWriteInitialStateSnapshotRoundTrip(t *testing.T) {
	defer os.RemoveAll("demos")

	source := NewClient()
	source.State = StateActive
	source.Signon = 4
	source.Protocol = inet.PROTOCOL_FITZQUAKE
	source.MaxClients = 2
	source.LevelName = "Snapshot Test"
	source.GameType = 1
	source.ModelPrecache = []string{"maps/start.bsp", "progs/player.mdl"}
	source.SoundPrecache = []string{"misc/null.wav"}
	source.ViewEntity = 1
	source.CDTrack = 7
	source.LoopTrack = 7
	source.ViewAngles = [3]float32{11, 22, 33}
	source.PlayerNames[0] = "PlayerZero"
	source.PlayerNames[1] = "PlayerOne"
	source.PlayerColors[1] = 0x4f
	source.Frags[1] = 12
	source.Stats[3] = 77
	source.Stats[5] = 9
	source.LightStyles[0] = LightStyle{Length: 2, Map: "az"}
	source.StaticEntities = []inet.EntityState{{
		ModelIndex: 3,
		Frame:      2,
		Colormap:   4,
		Skin:       5,
		Origin:     [3]float32{10, 20, 30},
		Angles:     [3]float32{0, 90, 180},
		Alpha:      inet.ENTALPHA_DEFAULT,
		Scale:      inet.ENTSCALE_DEFAULT,
	}}
	source.StaticSounds = []StaticSound{{
		Origin:      [3]float32{1, 2, 3},
		SoundIndex:  9,
		Volume:      255,
		Attenuation: 1,
	}}
	source.SkyboxName = "env/test"
	source.FogDensity = 128
	source.FogColor = [3]byte{200, 100, 50}

	demo := NewDemoState()
	if err := demo.StartDemoRecording("initial_snapshot", source.CDTrack); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}
	if err := demo.WriteInitialStateSnapshot(source); err != nil {
		t.Fatalf("WriteInitialStateSnapshot failed: %v", err)
	}
	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	if err := demo.StartDemoPlayback("initial_snapshot"); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}
	defer demo.StopPlayback()

	playback := NewClient()
	parser := NewParser(playback)
	for frame := 0; frame < 3; frame++ {
		message, angles, err := demo.ReadDemoFrame()
		if err != nil {
			t.Fatalf("ReadDemoFrame(%d) failed: %v", frame, err)
		}
		if angles != source.ViewAngles {
			t.Fatalf("frame %d angles = %v, want %v", frame, angles, source.ViewAngles)
		}
		if err := parser.ParseServerMessage(message); err != nil {
			t.Fatalf("ParseServerMessage(%d) failed: %v", frame, err)
		}
	}

	if playback.Protocol != source.Protocol {
		t.Fatalf("protocol = %d, want %d", playback.Protocol, source.Protocol)
	}
	if playback.MaxClients != source.MaxClients {
		t.Fatalf("maxclients = %d, want %d", playback.MaxClients, source.MaxClients)
	}
	if playback.GameType != source.GameType {
		t.Fatalf("gametype = %d, want %d", playback.GameType, source.GameType)
	}
	if playback.LevelName != source.LevelName {
		t.Fatalf("levelname = %q, want %q", playback.LevelName, source.LevelName)
	}
	if playback.MapName != "start" {
		t.Fatalf("mapname = %q, want start", playback.MapName)
	}
	if playback.CDTrack != source.CDTrack || playback.LoopTrack != source.LoopTrack {
		t.Fatalf("cd/loop track = %d/%d, want %d/%d", playback.CDTrack, playback.LoopTrack, source.CDTrack, source.LoopTrack)
	}
	if playback.ViewEntity != source.ViewEntity {
		t.Fatalf("viewentity = %d, want %d", playback.ViewEntity, source.ViewEntity)
	}
	if playback.PlayerNames[1] != source.PlayerNames[1] {
		t.Fatalf("player name = %q, want %q", playback.PlayerNames[1], source.PlayerNames[1])
	}
	if playback.PlayerColors[1] != source.PlayerColors[1] {
		t.Fatalf("player color = %d, want %d", playback.PlayerColors[1], source.PlayerColors[1])
	}
	if playback.Frags[1] != source.Frags[1] {
		t.Fatalf("player frags = %d, want %d", playback.Frags[1], source.Frags[1])
	}
	if playback.LightStyles[0].Map != source.LightStyles[0].Map {
		t.Fatalf("lightstyle 0 = %q, want %q", playback.LightStyles[0].Map, source.LightStyles[0].Map)
	}
	if playback.Stats[3] != source.Stats[3] || playback.Stats[5] != source.Stats[5] {
		t.Fatalf("stats = %v, want stat[3]=%d stat[5]=%d", playback.Stats, source.Stats[3], source.Stats[5])
	}
	if len(playback.StaticEntities) != 1 || playback.StaticEntities[0].Origin != source.StaticEntities[0].Origin {
		t.Fatalf("static entities = %v, want %v", playback.StaticEntities, source.StaticEntities)
	}
	if len(playback.StaticSounds) != 1 || playback.StaticSounds[0].SoundIndex != source.StaticSounds[0].SoundIndex {
		t.Fatalf("static sounds = %v, want %v", playback.StaticSounds, source.StaticSounds)
	}
	if playback.SkyboxName != source.SkyboxName {
		t.Fatalf("skybox = %q, want %q", playback.SkyboxName, source.SkyboxName)
	}
	if playback.FogDensity != source.FogDensity || playback.FogColor != source.FogColor {
		t.Fatalf("fog = %v/%v, want %v/%v", playback.FogDensity, playback.FogColor, source.FogDensity, source.FogColor)
	}
	if playback.Signon != 3 {
		t.Fatalf("signon = %d, want 3", playback.Signon)
	}
	if playback.State != StateConnected {
		t.Fatalf("state = %v, want connected", playback.State)
	}
}

// TestDemoPlaybackNonExistentFile ensures that attempting to play a missing file results in an appropriate error.
// Why: Prevents crashes and provides user feedback for missing resources.
// Where in C: cl_demo.c, CL_OpenDemo.
func TestDemoPlaybackNonExistentFile(t *testing.T) {
	demo := NewDemoState()

	if err := demo.StartDemoPlayback("nonexistent_demo_file"); err == nil {
		t.Error("Expected error when opening nonexistent demo file")
		demo.StopPlayback()
	}
}

// TestDemoCannotRecordDuringPlayback ensures that recording and playback are mutually exclusive operations.
// Why: The demo system uses shared state that cannot handle both operations simultaneously.
// Where in C: cl_demo.c, CL_BeginRecord_f.
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

// TestDemoEmptyFile ensures that empty or minimal demo files are handled gracefully without crashing.
// Why: Robustness against failed recording attempts or truncated files.
// Where in C: cl_demo.c, CL_OpenDemo.
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

// TestDemoFrameCount verifies the reported total number of frames in a demo.
// Why: Used for progress bars, UI feedback, and seek limits.
// Where in C: cl_demo.c.
func TestDemoFrameCount(t *testing.T) {
	defer os.RemoveAll("demos")

	demo := NewDemoState()
	if got := demo.FrameCount(); got != 0 {
		t.Fatalf("FrameCount() on fresh state = %d, want 0", got)
	}

	if err := demo.StartDemoRecording("framecount", 0); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}
	for i := 0; i < 5; i++ {
		if err := demo.WriteDemoFrame([]byte{byte(i)}, [3]float32{0, 0, 0}); err != nil {
			t.Fatalf("WriteDemoFrame %d failed: %v", i, err)
		}
	}
	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	if err := demo.StartDemoPlayback("framecount"); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}
	defer demo.StopPlayback()

	if got := demo.FrameCount(); got != 5 {
		t.Fatalf("FrameCount() = %d, want 5", got)
	}
}

// TestDemoProgress verifies the reported playback progress as a percentage.
// Why: Essential for user interface feedback during playback.
// Where in C: cl_demo.c.
func TestDemoProgress(t *testing.T) {
	defer os.RemoveAll("demos")

	demo := NewDemoState()
	if got := demo.Progress(); got != 0 {
		t.Fatalf("Progress() on fresh state = %f, want 0", got)
	}

	if err := demo.StartDemoRecording("progress", 0); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}
	for i := 0; i < 4; i++ {
		if err := demo.WriteDemoFrame([]byte{byte(i)}, [3]float32{0, 0, 0}); err != nil {
			t.Fatalf("WriteDemoFrame %d failed: %v", i, err)
		}
	}
	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	if err := demo.StartDemoPlayback("progress"); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}
	defer demo.StopPlayback()

	if got := demo.Progress(); got != 0 {
		t.Fatalf("Progress() at start = %f, want 0", got)
	}

	// Read 2 of 4 frames → 50%
	for i := 0; i < 2; i++ {
		if _, _, err := demo.ReadDemoFrame(); err != nil {
			t.Fatalf("ReadDemoFrame %d failed: %v", i, err)
		}
	}
	if got := demo.Progress(); got != 0.5 {
		t.Fatalf("Progress() after 2/4 frames = %f, want 0.5", got)
	}

	// Read remaining 2 frames → 100%
	for i := 0; i < 2; i++ {
		if _, _, err := demo.ReadDemoFrame(); err != nil {
			t.Fatalf("ReadDemoFrame %d failed: %v", i+2, err)
		}
	}
	if got := demo.Progress(); got != 1.0 {
		t.Fatalf("Progress() after 4/4 frames = %f, want 1.0", got)
	}
}

// TestDemoTogglePause verifies that demo playback can be paused and resumed.
// Why: Standard media control functionality for user convenience.
// Where in C: cl_demo.c.
func TestDemoTogglePause(t *testing.T) {
	demo := NewDemoState()
	if demo.Paused {
		t.Fatal("expected not paused initially")
	}

	if got := demo.TogglePause(); !got {
		t.Fatal("TogglePause() returned false, want true (paused)")
	}
	if !demo.Paused {
		t.Fatal("expected Paused to be true")
	}

	if got := demo.TogglePause(); got {
		t.Fatal("TogglePause() returned true, want false (unpaused)")
	}
	if demo.Paused {
		t.Fatal("expected Paused to be false")
	}
}

// TestDemoSetSpeed verifies that the playback speed can be adjusted (e.g., slow-motion, fast-forward).
// Why: Enhances analysis and viewing of recorded gameplay.
// Where in C: cl_demo.c.
func TestDemoSetSpeed(t *testing.T) {
	demo := NewDemoState()

	demo.SetSpeed(2.0)
	if demo.Speed != 2.0 {
		t.Fatalf("Speed = %f, want 2.0", demo.Speed)
	}

	demo.SetSpeed(0.5)
	if demo.Speed != 0.5 {
		t.Fatalf("Speed = %f, want 0.5", demo.Speed)
	}
	if demo.BaseSpeed != 0.5 {
		t.Fatalf("BaseSpeed = %f, want 0.5", demo.BaseSpeed)
	}

	demo.SetSpeed(0)
	if demo.Speed != 0 {
		t.Fatalf("Speed = %f, want 0 after setting 0", demo.Speed)
	}
	if !demo.Paused {
		t.Fatal("SetSpeed(0) should pause playback")
	}

	demo.SetSpeed(-5)
	if demo.Speed != -5 {
		t.Fatalf("Speed = %f, want -5 after setting -5", demo.Speed)
	}
	if demo.BaseSpeed != -5 {
		t.Fatalf("BaseSpeed = %f, want -5 after setting -5", demo.BaseSpeed)
	}
	if demo.Paused {
		t.Fatal("SetSpeed(-5) should resume playback")
	}
}

// TestDemoUpdatePlaybackSpeedSupportsTemporaryRewind verifies that speed adjustments can handle negative directions for rewinding.
// Why: Advanced demo navigation features for finding specific moments.
// Where in C: cl_demo.c.
func TestDemoUpdatePlaybackSpeedSupportsTemporaryRewind(t *testing.T) {
	demo := NewDemoState()
	demo.UpdatePlaybackSpeed(true, true, false, false)
	if demo.Speed != -5 {
		t.Fatalf("rewind speed = %f, want -5", demo.Speed)
	}

	demo.UpdatePlaybackSpeed(true, true, false, true)
	if demo.Speed != -1.25 {
		t.Fatalf("slow rewind speed = %f, want -1.25", demo.Speed)
	}

	demo.SetRewindBackstop(true)
	demo.UpdatePlaybackSpeed(true, false, true, false)
	if demo.Speed != 5 {
		t.Fatalf("forward speed = %f, want 5", demo.Speed)
	}
	if demo.RewindBackstop() {
		t.Fatal("positive playback should clear rewind backstop")
	}
}

// TestTimeDemoStartsCountingOnSecondPlaybackFrame verifies timedemo benchmarking logic.
// Why: Accurate performance measurement requires skipping the initial loading/setup frame to measure sustained FPS.
// Where in C: cl_demo.c, CL_ReadDemoMessage.
func TestTimeDemoStartsCountingOnSecondPlaybackFrame(t *testing.T) {
	demo := NewDemoState()
	demo.EnableTimeDemo()

	demo.NotePlaybackFrame()
	if got := demo.timedemoFrames; got != 0 {
		t.Fatalf("timedemo frames after first playback frame = %d, want 0", got)
	}
	if !demo.timedemoStart.IsZero() {
		t.Fatal("timedemo start should remain unset on the first playback frame")
	}

	demo.NotePlaybackFrame()
	if got := demo.timedemoFrames; got != 1 {
		t.Fatalf("timedemo frames after second playback frame = %d, want 1", got)
	}
	if demo.timedemoStart.IsZero() {
		t.Fatal("timedemo start should be set on the second playback frame")
	}
}

// TestDemoFrameForTime verifies the mapping between playback time (seconds) and frame index.
// Why: Enables time-based navigation (e.g., \"seek to 2:30\").
// Where in C: cl_demo.c.
func TestDemoFrameForTime(t *testing.T) {
	defer os.RemoveAll("demos")

	demo := NewDemoState()

	// No frames → always 0
	if got := demo.FrameForTime(1.0); got != 0 {
		t.Fatalf("FrameForTime(1.0) with no frames = %d, want 0", got)
	}

	if err := demo.StartDemoRecording("timeseek", 0); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}
	for i := 0; i < 144; i++ { // 2 seconds at 72 Hz
		if err := demo.WriteDemoFrame([]byte{byte(i % 256)}, [3]float32{0, 0, 0}); err != nil {
			t.Fatalf("WriteDemoFrame %d failed: %v", i, err)
		}
	}
	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	if err := demo.StartDemoPlayback("timeseek"); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}
	defer demo.StopPlayback()

	tests := []struct {
		seconds float64
		want    int
	}{
		{0, 0},
		{0.5, 36},   // 0.5 * 72 = 36
		{1.0, 72},   // 1.0 * 72 = 72
		{2.0, 143},  // 2.0 * 72 = 144, clamped to 143
		{10.0, 143}, // Way past end, clamped
		{-1.0, 0},   // Negative, clamped to 0
	}
	for _, tt := range tests {
		if got := demo.FrameForTime(tt.seconds); got != tt.want {
			t.Errorf("FrameForTime(%f) = %d, want %d", tt.seconds, got, tt.want)
		}
	}
}

// TestDemoTimeForFrame verifies the mapping between frame index and playback time (seconds).
// Why: Used for displaying the current timestamp in the playback UI.
// Where in C: cl_demo.c.
func TestDemoTimeForFrame(t *testing.T) {
	demo := NewDemoState()

	tests := []struct {
		frame int
		want  float64
	}{
		{0, 0},
		{72, 1.0},
		{36, 0.5},
	}
	for _, tt := range tests {
		if got := demo.TimeForFrame(tt.frame); got != tt.want {
			t.Errorf("TimeForFrame(%d) = %f, want %f", tt.frame, got, tt.want)
		}
	}
}

// TestDemoSeekToFrame0 verifies seeking to the very first frame of a demo.
// Why: Common requirement to restart a demo from the beginning.
// Where in C: cl_demo.c.
func TestDemoSeekToFrame0(t *testing.T) {
	defer os.RemoveAll("demos")

	demo := NewDemoState()
	if err := demo.StartDemoRecording("seek_zero", 0); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := demo.WriteDemoFrame([]byte{byte(i)}, [3]float32{float32(i), 0, 0}); err != nil {
			t.Fatalf("WriteDemoFrame %d failed: %v", i, err)
		}
	}
	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	if err := demo.StartDemoPlayback("seek_zero"); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}
	defer demo.StopPlayback()

	// Read all frames
	for i := 0; i < 3; i++ {
		if _, _, err := demo.ReadDemoFrame(); err != nil {
			t.Fatalf("ReadDemoFrame %d failed: %v", i, err)
		}
	}
	if demo.FrameIndex != 3 {
		t.Fatalf("FrameIndex = %d, want 3", demo.FrameIndex)
	}

	// Seek back to 0
	if err := demo.SeekFrame(0); err != nil {
		t.Fatalf("SeekFrame(0) failed: %v", err)
	}
	if demo.FrameIndex != 0 {
		t.Fatalf("FrameIndex after seek = %d, want 0", demo.FrameIndex)
	}

	// Read first frame again
	msg, angles, err := demo.ReadDemoFrame()
	if err != nil {
		t.Fatalf("ReadDemoFrame after seek to 0 failed: %v", err)
	}
	if !bytes.Equal(msg, []byte{0}) {
		t.Fatalf("frame 0 message = %v, want [0]", msg)
	}
	if angles[0] != 0 {
		t.Fatalf("frame 0 angle = %v, want 0", angles[0])
	}
}

// TestDemoSeekPastEnd verifies that seeking past the last frame of a demo is handled gracefully as an error.
// Why: Prevents the system from entering an undefined state or crashing.
// Where in C: cl_demo.c.
func TestDemoSeekPastEnd(t *testing.T) {
	defer os.RemoveAll("demos")

	demo := NewDemoState()
	if err := demo.StartDemoRecording("seek_past_end", 0); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := demo.WriteDemoFrame([]byte{byte(i)}, [3]float32{0, 0, 0}); err != nil {
			t.Fatalf("WriteDemoFrame %d failed: %v", i, err)
		}
	}
	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	if err := demo.StartDemoPlayback("seek_past_end"); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}
	defer demo.StopPlayback()

	// Seek past end should error
	if err := demo.SeekFrame(10); err == nil {
		t.Fatal("SeekFrame(10) with 3 frames should error")
	}

	// Seek to exactly frame count should error
	if err := demo.SeekFrame(3); err == nil {
		t.Fatal("SeekFrame(3) with 3 frames should error")
	}

	// Negative frame should error
	if err := demo.SeekFrame(-1); err == nil {
		t.Fatal("SeekFrame(-1) should error")
	}
}

// TestNilDemoStateConvenienceMethods ensures that calling progress and count methods on a nil demo state is safe.
// Why: Prevents null pointer dereferences in common UI and state-checking code paths.
// Where in C: cl_demo.c.
func TestNilDemoStateConvenienceMethods(t *testing.T) {
	var d *DemoState

	if got := d.FrameCount(); got != 0 {
		t.Fatalf("nil.FrameCount() = %d, want 0", got)
	}
	if got := d.Progress(); got != 0 {
		t.Fatalf("nil.Progress() = %f, want 0", got)
	}
	if got := d.TogglePause(); got {
		t.Fatal("nil.TogglePause() = true, want false")
	}
	if got := d.FrameForTime(1.0); got != 0 {
		t.Fatalf("nil.FrameForTime() = %d, want 0", got)
	}
	d.SetSpeed(2.0) // Should not panic
}

// TestDemoRecordingNegativeTrack ensures that recording correctly handles and preserves negative CD track numbers.
// Why: Some game modes or mods might use specific track indices for special music behavior.
// Where in C: cl_demo.c.
func TestDemoRecordingNegativeTrack(t *testing.T) {
	defer os.RemoveAll("demos")

	demo := NewDemoState()
	if err := demo.StartDemoRecording("negtrack_test", -1); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}

	if err := demo.WriteDemoFrame([]byte{0x01}, [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame failed: %v", err)
	}
	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	if err := demo.StartDemoPlayback("negtrack_test"); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}
	if demo.CDTrack != -1 {
		t.Errorf("CDTrack = %d, want -1", demo.CDTrack)
	}
	demo.StopPlayback()
}

// TestDemoRecordingMidLevelSnapshot verifies that a game state snapshot can be taken and recorded while a recording is already in progress.
// Why: Supports features like 'demo_capture' where a mid-game state is needed for a new demo.
// Where in C: cl_demo.c.
func TestDemoRecordingMidLevelSnapshot(t *testing.T) {
	defer os.RemoveAll("demos")

	demo := NewDemoState()
	if err := demo.StartDemoRecording("midlevel_test", 3); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}

	c := &Client{
		State:      StateConnected,
		Signon:     2,
		MaxClients: 1,
		Protocol:   inet.PROTOCOL_FITZQUAKE,
		LevelName:  "start",
		CDTrack:    3,
		LoopTrack:  3,
		ViewEntity: 1,
		ViewAngles: [3]float32{10, 20, 30},
	}

	if err := demo.WriteInitialStateSnapshot(c); err != nil {
		t.Fatalf("WriteInitialStateSnapshot failed: %v", err)
	}
	if err := demo.WriteDisconnectTrailer([3]float32{}); err != nil {
		t.Fatalf("WriteDisconnectTrailer failed: %v", err)
	}
	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	if err := demo.StartDemoPlayback("midlevel_test"); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}
	if demo.CDTrack != 3 {
		t.Errorf("CDTrack = %d, want 3", demo.CDTrack)
	}

	frameCount := 0
	for {
		if _, _, err := demo.ReadDemoFrame(); err != nil {
			break
		}
		frameCount++
	}
	// 3 snapshot frames (serverinfo, signon, state) + 1 disconnect trailer
	if frameCount != 4 {
		t.Errorf("frame count = %d, want 4", frameCount)
	}
	demo.StopPlayback()
}

// TestDemoDisconnectDuringRecording ensures that a recording is closed cleanly when the client disconnects.
// Why: Prevents data loss and ensures the recorded file is valid even if the session ends abruptly.
// Where in C: cl_demo.c, CL_Disconnect.
func TestDemoDisconnectDuringRecording(t *testing.T) {
	defer os.RemoveAll("demos")

	demo := NewDemoState()
	if err := demo.StartDemoRecording("disconnect_test", -1); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := demo.WriteDemoFrame([]byte{byte(i)}, [3]float32{float32(i), 0, 0}); err != nil {
			t.Fatalf("WriteDemoFrame %d failed: %v", i, err)
		}
	}

	if err := demo.WriteDisconnectTrailer([3]float32{}); err != nil {
		t.Fatalf("WriteDisconnectTrailer failed: %v", err)
	}
	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	if err := demo.StartDemoPlayback("disconnect_test"); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}

	frameCount := 0
	var lastMsg []byte
	for {
		msg, _, err := demo.ReadDemoFrame()
		if err != nil {
			break
		}
		lastMsg = msg
		frameCount++
	}

	if frameCount != 4 {
		t.Errorf("frame count = %d, want 4 (3 data + 1 disconnect)", frameCount)
	}
	if len(lastMsg) != 1 || lastMsg[0] != inet.SVCDisconnect {
		t.Errorf("last message = %v, want [%d] (svc_disconnect)", lastMsg, inet.SVCDisconnect)
	}
	demo.StopPlayback()
}
