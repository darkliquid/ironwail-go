// Demo playback tests split from demo_test.go.

package client

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	inet "github.com/darkliquid/ironwail-go/internal/net"
)

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

func TestStopPlaybackWithSummaryPrintsTimedemoLine(t *testing.T) {
	demo := NewDemoState()
	demo.Playback = true
	demo.TimeDemo = true
	demo.timedemoFrames = 10
	demo.timedemoStart = time.Now().Add(-2 * time.Second)

	var output strings.Builder
	if err := demo.StopPlaybackWithSummary(func(msg string) {
		output.WriteString(msg)
	}); err != nil {
		t.Fatalf("StopPlaybackWithSummary error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, "timedemo: 10 frames") {
		t.Fatalf("summary output = %q, want timedemo line", got)
	}
	if demo.TimeDemo {
		t.Fatal("TimeDemo = true after stop, want false")
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
