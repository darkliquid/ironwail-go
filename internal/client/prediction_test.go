package client

import (
	"testing"

	inet "github.com/ironwail/ironwail-go/internal/net"
)

func TestPredictPlayersInitialization(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.OnGround = true
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{
		Origin: [3]float32{100, 200, 300},
	}

	// First call should initialize prediction state
	c.PredictPlayers(0.016)

	if c.LastServerOrigin != c.Entities[0].Origin {
		t.Errorf("LastServerOrigin not initialized: got %v, want %v",
			c.LastServerOrigin, c.Entities[0].Origin)
	}

	if c.PredictedOrigin != c.Entities[0].Origin {
		t.Errorf("PredictedOrigin not initialized: got %v, want %v",
			c.PredictedOrigin, c.Entities[0].Origin)
	}
}

func TestPredictPlayersPrefersEntityOneWhenViewEntityUnset(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.ViewEntity = 0
	c.OnGround = true
	c.Entities[1] = inet.EntityState{
		Origin: [3]float32{10, 20, 30},
	}

	c.PredictPlayers(0.016)

	if c.LastServerOrigin != c.Entities[1].Origin {
		t.Fatalf("LastServerOrigin = %v, want entity 1 origin %v", c.LastServerOrigin, c.Entities[1].Origin)
	}
	if c.PredictedOrigin != c.Entities[1].Origin {
		t.Fatalf("PredictedOrigin = %v, want entity 1 origin %v", c.PredictedOrigin, c.Entities[1].Origin)
	}
}

func TestPredictPlayersForwardMovement(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{
		Origin: [3]float32{0, 0, 0},
	}

	// Initialize prediction
	c.PredictPlayers(0.016)

	// Apply forward movement command
	c.PendingCmd = UserCmd{
		ViewAngles: [3]float32{0, 0, 0}, // Facing forward (along +X)
		Forward:    200,                 // Forward speed
	}

	initialOrigin := c.PredictedOrigin

	// Predict with forward movement
	c.PredictPlayers(0.016)

	// Position should have moved forward
	if c.PredictedOrigin == initialOrigin {
		t.Error("Position did not change with forward movement")
	}

	// Velocity should be non-zero
	speed := sqrtFloat32(
		c.PredictedVelocity[0]*c.PredictedVelocity[0] +
			c.PredictedVelocity[1]*c.PredictedVelocity[1] +
			c.PredictedVelocity[2]*c.PredictedVelocity[2])

	if speed == 0 {
		t.Error("Velocity is zero with forward movement")
	}
}

func TestPredictPlayersFriction(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.OnGround = true
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{
		Origin: [3]float32{0, 0, 0},
	}

	// Initialize with some velocity
	c.PredictPlayers(0.016)
	c.PredictedVelocity = [3]float32{100, 0, 0}

	initialVelocity := c.PredictedVelocity[0]

	// Apply prediction with no input (only friction)
	c.PendingCmd = UserCmd{
		ViewAngles: [3]float32{0, 0, 0},
		Forward:    0,
		Side:       0,
		Up:         0,
	}

	c.PredictPlayers(0.016)

	// Velocity should have decreased due to friction
	if c.PredictedVelocity[0] >= initialVelocity {
		t.Errorf("Friction did not reduce velocity: initial=%.2f, after=%.2f",
			initialVelocity, c.PredictedVelocity[0])
	}

	// Velocity should not be negative (friction doesn't reverse)
	if c.PredictedVelocity[0] < 0 {
		t.Error("Friction caused velocity to go negative")
	}
}

func TestPredictPlayersSpeedClamping(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.OnGround = true
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{
		Origin: [3]float32{0, 0, 0},
	}

	// Initialize prediction
	c.PredictPlayers(0.016)

	// Apply prediction with oversized desired speed.
	c.PendingCmd = UserCmd{
		ViewAngles: [3]float32{0, 0, 0},
		Forward:    1000,
	}
	for i := 0; i < 60; i++ {
		c.PredictPlayers(0.016)
	}

	// Calculate speed
	speed := sqrtFloat32(
		c.PredictedVelocity[0]*c.PredictedVelocity[0] +
			c.PredictedVelocity[1]*c.PredictedVelocity[1] +
			c.PredictedVelocity[2]*c.PredictedVelocity[2])

	// Speed should remain bounded by configured max speed.
	if speed > c.PredictionMaxSpeed+0.1 {
		t.Errorf("Speed not clamped: got %.2f, max %.2f",
			speed, c.PredictionMaxSpeed)
	}
}

func TestPredictPlayersAirborneNoGroundFriction(t *testing.T) {
	c := NewClient()
	c.OnGround = false
	c.PredictionGravity = 0
	c.PredictedVelocity = [3]float32{100, 0, 0}

	c.predictMovement(&UserCmd{ViewAngles: [3]float32{0, 0, 0}}, 0.016)

	if absFloat32(c.PredictedVelocity[0]-100) > 0.001 {
		t.Fatalf("airborne x velocity changed by ground friction: got %.3f, want 100", c.PredictedVelocity[0])
	}
}

func TestPredictPlayersAirborneGravity(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.OnGround = false
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{Origin: [3]float32{1, 0, 0}}
	c.PredictPlayers(0.016)

	c.PendingCmd = UserCmd{ViewAngles: [3]float32{0, 0, 0}}
	c.PredictPlayers(0.016)

	if c.PredictedVelocity[2] >= 0 {
		t.Fatalf("airborne gravity not applied: z velocity %.3f", c.PredictedVelocity[2])
	}
}

func TestPredictPlayersErrorCorrection(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{
		Origin: [3]float32{100, 100, 100},
	}

	// Initialize prediction
	c.PredictPlayers(0.016)

	// Simulate prediction drift
	c.PredictedOrigin = [3]float32{110, 105, 102}

	// Server sends update with different position
	c.Entities[0] = inet.EntityState{
		Origin: [3]float32{115, 110, 105},
	}

	// Apply prediction
	c.PendingCmd = UserCmd{
		ViewAngles: [3]float32{0, 0, 0},
	}
	c.PredictPlayers(0.016)

	// Prediction error should be calculated
	if c.PredictionError == [3]float32{} {
		t.Error("Prediction error not calculated after server update")
	}

	// Predicted origin should NOT immediately snap to server (smooth correction)
	// It should be moving towards the server position
	initialError := c.PredictionError

	// Continue predicting to apply error correction
	for i := 0; i < 10; i++ {
		c.PredictPlayers(0.016)
	}

	// Error should be reduced (lerped towards zero)
	currentError := sqrtFloat32(
		c.PredictionError[0]*c.PredictionError[0] +
			c.PredictionError[1]*c.PredictionError[1] +
			c.PredictionError[2]*c.PredictionError[2])

	initialErrorMag := sqrtFloat32(
		initialError[0]*initialError[0] +
			initialError[1]*initialError[1] +
			initialError[2]*initialError[2])

	if currentError >= initialErrorMag {
		t.Errorf("Error not corrected: initial=%.4f, current=%.4f",
			initialErrorMag, currentError)
	}
}

func TestPredictPlayersNoEntityDoesNotPanic(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.ViewEntity = 0
	// No entities in map

	// Should not panic
	c.PredictPlayers(0.016)
}

func TestPredictPlayersInactiveStateDoesNothing(t *testing.T) {
	c := NewClient()
	c.State = StateDisconnected
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{
		Origin: [3]float32{100, 200, 300},
	}

	c.PredictPlayers(0.016)

	// Should not initialize when not active
	if c.LastServerOrigin != [3]float32{} {
		t.Error("Prediction initialized in non-active state")
	}
}

func TestPredictPlayersStrafeMovement(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{
		Origin: [3]float32{0, 0, 0},
	}

	// Initialize prediction
	c.PredictPlayers(0.016)

	// Apply strafe movement command
	c.PendingCmd = UserCmd{
		ViewAngles: [3]float32{0, 0, 0}, // Facing forward
		Side:       350,                 // Strafe right
	}

	initialOrigin := c.PredictedOrigin

	// Predict with strafe movement
	c.PredictPlayers(0.016)

	// Position should have moved
	if c.PredictedOrigin == initialOrigin {
		t.Error("Position did not change with strafe movement")
	}
}

func TestPredictPlayersMultipleFrames(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{
		Origin: [3]float32{0, 0, 0},
	}

	// Initialize
	c.PredictPlayers(0.016)

	// Apply movement over multiple frames
	c.PendingCmd = UserCmd{
		ViewAngles: [3]float32{0, 0, 0},
		Forward:    200,
	}

	for i := 0; i < 60; i++ {
		c.PredictPlayers(0.016)
	}

	// Should have moved after 60 frames (~1 second)
	distance := sqrtFloat32(
		c.PredictedOrigin[0]*c.PredictedOrigin[0] +
			c.PredictedOrigin[1]*c.PredictedOrigin[1] +
			c.PredictedOrigin[2]*c.PredictedOrigin[2])

	if distance < 0.1 {
		t.Errorf("Distance too small after 60 frames: %.2f", distance)
	}
}

func TestPredictPlayersConsumesBufferedCommands(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.Signon = Signons
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{Origin: [3]float32{0, 0, 0}}
	c.PredictPlayers(0.016)

	c.enqueueCommand(UserCmd{ViewAngles: [3]float32{0, 0, 0}, Forward: 200})
	c.enqueueCommand(UserCmd{ViewAngles: [3]float32{0, 90, 0}, Side: 200})

	c.PredictPlayers(0.032)
	if c.CommandCount != 2 {
		t.Fatalf("command count after prediction = %d, want 2 (unacknowledged)", c.CommandCount)
	}
	if c.PredictedOrigin == [3]float32{} {
		t.Fatal("predicted origin unchanged after buffered command prediction")
	}
}

func TestConsumeCommandBufferHandlesNegativeSequence(t *testing.T) {
	c := NewClient()
	c.CommandCount = 2
	c.CommandSequence = -1
	wantFirst := UserCmd{Forward: 10}
	wantSecond := UserCmd{Forward: 20}
	start := c.CommandSequence - c.CommandCount
	c.CommandBuffer[wrapBufferIndex(start, len(c.CommandBuffer))] = wantFirst
	c.CommandBuffer[wrapBufferIndex(start+1, len(c.CommandBuffer))] = wantSecond

	got := c.bufferedCommands()
	if len(got) != 2 {
		t.Fatalf("bufferedCommands len = %d, want 2", len(got))
	}
	if got[0].Forward != wantFirst.Forward || got[1].Forward != wantSecond.Forward {
		t.Fatalf("bufferedCommands order mismatch: got %+v", got)
	}
	if c.CommandCount != 2 {
		t.Fatalf("command count changed by bufferedCommands: got %d, want 2", c.CommandCount)
	}
}

func TestPredictPlayersRebasesFromServerOriginEachFrame(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.ViewEntity = 0
	c.OnGround = true
	c.Entities[0] = inet.EntityState{Origin: [3]float32{0, 0, 0}}
	c.PredictPlayers(0.016)
	c.enqueueCommand(UserCmd{Forward: 200, Msec: 16})
	c.enqueueCommand(UserCmd{Forward: 200, Msec: 16})

	c.PredictPlayers(0.032)
	first := c.PredictedOrigin

	c.PredictPlayers(0.032)

	if c.PredictedOrigin != first {
		t.Fatalf("PredictedOrigin compounded across frames: first=%v second=%v", first, c.PredictedOrigin)
	}
}

func TestPredictPlayersPendingFallbackRebasesEachFrame(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.ViewEntity = 0
	c.OnGround = false
	c.PredictionGravity = 0
	c.Entities[0] = inet.EntityState{Origin: [3]float32{10, 20, 30}}
	c.Velocity = [3]float32{0, 100, 0}
	c.PendingCmd = UserCmd{ViewAngles: [3]float32{0, 0, 0}, Msec: 16}

	c.PredictPlayers(0.016)
	first := c.PredictedOrigin

	c.PredictPlayers(0.016)

	if c.PredictedOrigin != first {
		t.Fatalf("pending fallback compounded across frames: first=%v second=%v", first, c.PredictedOrigin)
	}
	if !c.LastPredictionReplayTelemetry.UsedPendingCmdFallback {
		t.Fatal("UsedPendingCmdFallback = false, want true")
	}
}

func TestPredictPlayersRecordsCurrentFrameTelemetryAndValidity(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.Time = 1.25
	c.ViewEntity = 1
	c.OnGround = true
	c.Entities[1] = inet.EntityState{Origin: [3]float32{10, 20, 30}}
	c.PendingCmd = UserCmd{
		ViewAngles: [3]float32{0, 0, 0},
		Forward:    100,
		Msec:       16,
	}

	c.PredictPlayers(0.016)

	if !c.PredictionValid {
		t.Fatal("PredictionValid = false, want true")
	}
	if c.PredictionEntityNum != 1 {
		t.Fatalf("PredictionEntityNum = %d, want 1", c.PredictionEntityNum)
	}
	if c.PredictionFrameTime != c.Time {
		t.Fatalf("PredictionFrameTime = %v, want %v", c.PredictionFrameTime, c.Time)
	}
	if !c.HasFreshPredictionForCurrentEntity() {
		t.Fatal("HasFreshPredictionForCurrentEntity() = false, want true")
	}

	telemetry := c.PredictionReplayTelemetrySnapshot()
	if telemetry.FrameTime != c.Time {
		t.Fatalf("telemetry.FrameTime = %v, want %v", telemetry.FrameTime, c.Time)
	}
	if telemetry.EntityNum != 1 {
		t.Fatalf("telemetry.EntityNum = %d, want 1", telemetry.EntityNum)
	}
	if !telemetry.EntityFound {
		t.Fatal("telemetry.EntityFound = false, want true")
	}
	if !telemetry.Valid {
		t.Fatal("telemetry.Valid = false, want true")
	}
	if telemetry.ServerBaseOrigin != [3]float32{10, 20, 30} {
		t.Fatalf("telemetry.ServerBaseOrigin = %v, want [10 20 30]", telemetry.ServerBaseOrigin)
	}
	if !telemetry.UsedPendingCmdFallback {
		t.Fatal("telemetry.UsedPendingCmdFallback = false, want true")
	}
	if telemetry.ReplayedCommandCount != 1 {
		t.Fatalf("telemetry.ReplayedCommandCount = %d, want 1", telemetry.ReplayedCommandCount)
	}
	if !telemetry.HasReplayedCmds {
		t.Fatal("telemetry.HasReplayedCmds = false, want true")
	}
	if telemetry.PendingCmd != c.PendingCmd {
		t.Fatalf("telemetry.PendingCmd = %+v, want %+v", telemetry.PendingCmd, c.PendingCmd)
	}
	if telemetry.OldestReplayedCmd != c.PendingCmd || telemetry.NewestReplayedCmd != c.PendingCmd {
		t.Fatalf("telemetry replayed cmds = oldest %+v newest %+v, want pending cmd %+v", telemetry.OldestReplayedCmd, telemetry.NewestReplayedCmd, c.PendingCmd)
	}
	if telemetry.OutputPredictedOrigin != c.PredictedOrigin {
		t.Fatalf("telemetry.OutputPredictedOrigin = %v, want %v", telemetry.OutputPredictedOrigin, c.PredictedOrigin)
	}
	if telemetry.OutputPredictedVelocity != c.PredictedVelocity {
		t.Fatalf("telemetry.OutputPredictedVelocity = %v, want %v", telemetry.OutputPredictedVelocity, c.PredictedVelocity)
	}
}

func TestPredictPlayersInvalidatesMissingEntityAndTelemetry(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.Time = 2.5
	c.ViewEntity = 1
	c.PredictedOrigin = [3]float32{99, 88, 77}
	c.PredictedVelocity = [3]float32{1, 2, 3}
	c.PredictionValid = true
	c.PredictionEntityNum = 1
	c.PredictionFrameTime = 1.0

	c.PredictPlayers(0.016)

	if c.PredictionValid {
		t.Fatal("PredictionValid = true, want false")
	}
	if c.HasFreshPredictionForCurrentEntity() {
		t.Fatal("HasFreshPredictionForCurrentEntity() = true, want false")
	}
	if c.PredictionEntityNum != 1 {
		t.Fatalf("PredictionEntityNum = %d, want 1", c.PredictionEntityNum)
	}
	if c.PredictionFrameTime != c.Time {
		t.Fatalf("PredictionFrameTime = %v, want %v", c.PredictionFrameTime, c.Time)
	}

	telemetry := c.PredictionReplayTelemetrySnapshot()
	if telemetry.EntityNum != 1 {
		t.Fatalf("telemetry.EntityNum = %d, want 1", telemetry.EntityNum)
	}
	if telemetry.EntityFound {
		t.Fatal("telemetry.EntityFound = true, want false")
	}
	if telemetry.Valid {
		t.Fatal("telemetry.Valid = true, want false")
	}
	if telemetry.OutputPredictedOrigin != [3]float32{99, 88, 77} {
		t.Fatalf("telemetry.OutputPredictedOrigin = %v, want stale predicted origin snapshot", telemetry.OutputPredictedOrigin)
	}
	if telemetry.OutputPredictedVelocity != [3]float32{1, 2, 3} {
		t.Fatalf("telemetry.OutputPredictedVelocity = %v, want stale predicted velocity snapshot", telemetry.OutputPredictedVelocity)
	}
}

func TestGetPredictedOriginReturnsCorrectValue(t *testing.T) {
	c := NewClient()
	c.PredictedOrigin = [3]float32{10, 20, 30}

	origin := c.GetPredictedOrigin()
	if origin != [3]float32{10, 20, 30} {
		t.Errorf("GetPredictedOrigin returned %v, want [10 20 30]", origin)
	}
}

func TestGetPredictedVelocityReturnsCorrectValue(t *testing.T) {
	c := NewClient()
	c.PredictedVelocity = [3]float32{100, 50, 25}

	velocity := c.GetPredictedVelocity()
	if velocity != [3]float32{100, 50, 25} {
		t.Errorf("GetPredictedVelocity returned %v, want [100 50 25]", velocity)
	}
}

func TestAngleVectorsQuake(t *testing.T) {
	// Test forward vector (no rotation)
	angles := [3]float32{0, 0, 0}
	forward, _, _ := angleVectorsQuake(angles)

	// Forward should be approximately (1, 0, 0)
	if absFloat32(forward[0]-1.0) > 0.01 || absFloat32(forward[1]) > 0.01 || absFloat32(forward[2]) > 0.01 {
		t.Errorf("Forward vector incorrect: got %v, want ~[1 0 0]", forward)
	}

	// Test 90 degree yaw rotation
	angles = [3]float32{0, 90, 0}
	forward, _, _ = angleVectorsQuake(angles)

	// Forward should be approximately (0, 1, 0) after 90 degree yaw
	if absFloat32(forward[0]) > 0.01 || absFloat32(forward[1]-1.0) > 0.01 || absFloat32(forward[2]) > 0.01 {
		t.Errorf("Forward vector after 90° yaw incorrect: got %v, want ~[0 1 0]", forward)
	}
}

func TestPredictionMovementAnglesMatchesServerSemantics(t *testing.T) {
	c := NewClient()
	c.PunchAngle = [3]float32{6, -15, 4}

	got := c.predictionMovementAngles([3]float32{-30, 90, 17})
	want := [3]float32{8, 75, 0}

	if got != want {
		t.Fatalf("predictionMovementAngles = %v, want %v", got, want)
	}
}

func TestPredictMovementUsesServerStylePitchForAcceleration(t *testing.T) {
	c := NewClient()
	c.OnGround = false
	c.PredictionGravity = 0
	c.PredictionAccel = 10
	c.PredictionMaxSpeed = 1000
	c.PunchAngle = [3]float32{30, 0, 0}

	cmd := UserCmd{
		ViewAngles: [3]float32{-30, 0, 15},
		Forward:    320,
	}

	c.predictMovement(&cmd, 0.016)

	wantAccel := float32(c.PredictionAccel * 0.016 * cmd.Forward)
	if absFloat32(c.PredictedVelocity[0]-wantAccel) > 0.001 {
		t.Fatalf("PredictedVelocity[0] = %.3f, want %.3f from server-style move pitch", c.PredictedVelocity[0], wantAccel)
	}
	if absFloat32(c.PredictedVelocity[1]) > 0.001 || absFloat32(c.PredictedVelocity[2]) > 0.001 {
		t.Fatalf("PredictedVelocity = %v, want only +X acceleration", c.PredictedVelocity)
	}
}

func TestPredictionMovementAnglesIncludeServerStyleRoll(t *testing.T) {
	c := NewClient()
	c.PredictedVelocity = [3]float32{0, 200, 0}

	got := c.predictionMovementAngles([3]float32{})
	want := [3]float32{0, 0, -8}

	if got != want {
		t.Fatalf("predictionMovementAngles roll = %v, want %v", got, want)
	}
}

func TestAbsFloat32(t *testing.T) {
	if absFloat32(5.0) != 5.0 {
		t.Error("absFloat32(5.0) should be 5.0")
	}
	if absFloat32(-5.0) != 5.0 {
		t.Error("absFloat32(-5.0) should be 5.0")
	}
	if absFloat32(0.0) != 0.0 {
		t.Error("absFloat32(0.0) should be 0.0")
	}
}

func TestMaxFloat32(t *testing.T) {
	if maxFloat32(5.0, 3.0) != 5.0 {
		t.Error("maxFloat32(5.0, 3.0) should be 5.0")
	}
	if maxFloat32(3.0, 5.0) != 5.0 {
		t.Error("maxFloat32(3.0, 5.0) should be 5.0")
	}
	if maxFloat32(5.0, 5.0) != 5.0 {
		t.Error("maxFloat32(5.0, 5.0) should be 5.0")
	}
}

func TestSqrtFloat32(t *testing.T) {
	result := sqrtFloat32(16.0)
	if absFloat32(result-4.0) > 0.001 {
		t.Errorf("sqrtFloat32(16.0) should be ~4.0, got %.4f", result)
	}

	result = sqrtFloat32(0.0)
	if result != 0.0 {
		t.Errorf("sqrtFloat32(0.0) should be 0.0, got %.4f", result)
	}
}
