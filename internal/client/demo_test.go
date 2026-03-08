package client

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	inet "github.com/ironwail/ironwail-go/internal/net"
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
