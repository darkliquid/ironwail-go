// What: Movement prediction tests.
// Why: Confirms player movement feels responsive by anticipating position updates.
// Where in C: cl_main.c

package client

import (
	"testing"

	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

// TestPredictPlayersInitialization verifies that the client correctly initializes its prediction state from server data.
// Why: Reliable client-side prediction depends on the client and server starting from a common state.
// Where in C: cl_main.c, CL_PredictMove.
func TestPredictPlayersInitialization(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.OnGround = true
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{
		Origin: types.Vec3{X: 100, Y: 200, Z: 300},
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

// TestPredictPlayersPrefersEntityOneWhenViewEntityUnset ensures that the local player's entity (usually index 1) is used for prediction by default.
// Why: The client needs to know which entity represents the local player to perform self-prediction.
// Where in C: cl_main.c, CL_PredictMove.
func TestPredictPlayersPrefersEntityOneWhenViewEntityUnset(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.ViewEntity = 0
	c.OnGround = true
	c.Entities[1] = inet.EntityState{
		Origin: types.Vec3{X: 10, Y: 20, Z: 30},
	}

	c.PredictPlayers(0.016)

	if c.LastServerOrigin != c.Entities[1].Origin {
		t.Fatalf("LastServerOrigin = %v, want entity 1 origin %v", c.LastServerOrigin, c.Entities[1].Origin)
	}
	if c.PredictedOrigin != c.Entities[1].Origin {
		t.Fatalf("PredictedOrigin = %v, want entity 1 origin %v", c.PredictedOrigin, c.Entities[1].Origin)
	}
}

// TestPredictPlayersForwardMovement verifies that the client correctly predicts movement in the direction the player is facing.
// Why: Responsiveness depends on the client seeing its own movement immediately before the server confirms it.
// Where in C: cl_main.c, CL_PredictMove.
func TestPredictPlayersForwardMovement(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{
		Origin: types.Vec3{X: 0, Y: 0, Z: 0},
	}

	// Initialize prediction
	c.PredictPlayers(0.016)

	// Apply forward movement command
	c.PendingCmd = UserCmd{
		ViewAngles: types.Vec3{X: 0, Y: 0, Z: 0}, // Facing forward (along +X)
		Forward:    200,                         // Forward speed
	}

	initialOrigin := c.PredictedOrigin

	// Predict with forward movement
	c.PredictPlayers(0.016)

	// Position should have moved forward
	if c.PredictedOrigin == initialOrigin {
		t.Error("Position did not change with forward movement")
	}

	// Velocity should be non-zero
	speed := c.PredictedVelocity.Len()

	if speed == 0 {
		t.Error("Velocity is zero with forward movement")
	}
}

// TestPredictPlayersFriction verifies that ground friction is correctly applied during prediction.
// Why: Friction is essential for stopping and controlled movement; it must match the server exactly to avoid drift.
// Where in C: cl_main.c, CL_PredictMove.
func TestPredictPlayersFriction(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.OnGround = true
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{
		Origin: types.Vec3{X: 0, Y: 0, Z: 0},
	}

	// Initialize with some velocity
	c.PredictPlayers(0.016)
	c.PredictedVelocity = types.Vec3{X: 100, Y: 0, Z: 0}

	initialVelocity := c.PredictedVelocity.X

	// Apply prediction with no input (only friction)
	c.PendingCmd = UserCmd{
		ViewAngles: types.Vec3{X: 0, Y: 0, Z: 0},
		Forward:    0,
		Side:       0,
		Up:         0,
	}

	c.PredictPlayers(0.016)

	// Velocity should have decreased due to friction
	if c.PredictedVelocity.X >= initialVelocity {
		t.Errorf("Friction did not reduce velocity: initial=%.2f, after=%.2f",
			initialVelocity, c.PredictedVelocity.X)
	}

	// Velocity should not be negative (friction doesn't reverse)
	if c.PredictedVelocity.X < 0 {
		t.Error("Friction caused velocity to go negative")
	}
}

// TestPredictPlayersSpeedClamping ensures that predicted movement speed is capped at the server-enforced maximum.
// Why: Prevents the client from predicting impossible movements that the server will later reject.
// Where in C: cl_main.c, CL_PredictMove.
func TestPredictPlayersSpeedClamping(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.OnGround = true
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{
		Origin: types.Vec3{X: 0, Y: 0, Z: 0},
	}

	// Initialize prediction
	c.PredictPlayers(0.016)

	// Apply prediction with oversized desired speed.
	c.PendingCmd = UserCmd{
		ViewAngles: types.Vec3{X: 0, Y: 0, Z: 0},
		Forward:    1000,
	}
	for i := 0; i < 60; i++ {
		c.PredictPlayers(0.016)
	}

	// Calculate speed
	speed := c.PredictedVelocity.Len()

	// Speed should remain bounded by configured max speed.
	if speed > c.PredictionMaxSpeed+0.1 {
		t.Errorf("Speed not clamped: got %.2f, max %.2f",
			speed, c.PredictionMaxSpeed)
	}
}

// TestPredictPlayersAirborneNoGroundFriction verifies that friction is not applied when the player is in the air.
// Why: Players should maintain their horizontal momentum while jumping, as per Quake physics.
// Where in C: cl_main.c, CL_PredictMove.
func TestPredictPlayersAirborneNoGroundFriction(t *testing.T) {
	c := NewClient()
	c.OnGround = false
	c.PredictionGravity = 0
	c.PredictedVelocity = types.Vec3{X: 100, Y: 0, Z: 0}

	c.predictMovement(&UserCmd{ViewAngles: types.Vec3{X: 0, Y: 0, Z: 0}}, 0.016)

	if absFloat32(c.PredictedVelocity.X-100) > 0.001 {
		t.Fatalf("airborne x velocity changed by ground friction: got %.3f, want 100", c.PredictedVelocity.X)
	}
}

// TestPredictPlayersAirborneGravity verifies that gravity is correctly applied to airborne players during prediction.
// Why: Gravity is a fundamental part of the movement physics that must be predicted for smooth jumping and falling.
// Where in C: cl_main.c, CL_PredictMove.
func TestPredictPlayersAirborneGravity(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.OnGround = false
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{Origin: types.Vec3{X: 1, Y: 0, Z: 0}}
	c.PredictPlayers(0.016)

	c.PendingCmd = UserCmd{ViewAngles: types.Vec3{X: 0, Y: 0, Z: 0}}
	c.PredictPlayers(0.016)

	if c.PredictedVelocity.Z >= 0 {
		t.Fatalf("airborne gravity not applied: z velocity %.3f", c.PredictedVelocity.Z)
	}
}

// TestPredictPlayersErrorCorrection verifies that the client smoothly corrects its position when it drifts from the server's state.
// Why: Network jitter and floating-point differences cause small drifts; smoothing these out prevents jarring snaps.
// Where in C: cl_main.c, CL_PredictMove.
func TestPredictPlayersErrorCorrection(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{
		Origin: types.Vec3{X: 100, Y: 100, Z: 100},
	}

	// Initialize prediction
	c.PredictPlayers(0.016)

	// Simulate prediction drift
	c.PredictedOrigin = types.Vec3{X: 110, Y: 105, Z: 102}

	// Server sends update with different position
	c.Entities[0] = inet.EntityState{
		Origin: types.Vec3{X: 115, Y: 110, Z: 105},
	}

	// Apply prediction
	c.PendingCmd = UserCmd{
		ViewAngles: types.Vec3{X: 0, Y: 0, Z: 0},
	}
	c.PredictPlayers(0.016)

	// Prediction error should be calculated
	if c.PredictionError == (types.Vec3{}) {
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
	currentError := c.PredictionError.Len()
	initialErrorMag := initialError.Len()

	if currentError >= initialErrorMag {
		t.Errorf("Error not corrected: initial=%.4f, current=%.4f",
			initialErrorMag, currentError)
	}
}

// TestPredictPlayersNoEntityDoesNotPanic ensures that the prediction logic handles cases where the player's entity is missing.
// Why: Robustness against transient network states or server-side entity removal.
// Where in C: cl_main.c, CL_PredictMove.
func TestPredictPlayersNoEntityDoesNotPanic(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.ViewEntity = 0
	// No entities in map

	// Should not panic
	c.PredictPlayers(0.016)
}

// TestPredictPlayersInactiveStateDoesNothing ensures that prediction is disabled when the client is not in an active game state.
// Why: Prevents unnecessary processing and potential state corruption during menu navigation or connection.
// Where in C: cl_main.c, CL_PredictMove.
func TestPredictPlayersInactiveStateDoesNothing(t *testing.T) {
	c := NewClient()
	c.State = StateDisconnected
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{
		Origin: types.Vec3{X: 100, Y: 200, Z: 300},
	}

	c.PredictPlayers(0.016)

	// Should not initialize when not active
	if c.LastServerOrigin != (types.Vec3{}) {
		t.Error("Prediction initialized in non-active state")
	}
}

// TestPredictPlayersStrafeMovement verifies that sideway movement (strafing) is correctly predicted.
// Why: Strafe-jumping and precise lateral control are core Quake mechanics that require accurate prediction.
// Where in C: cl_main.c, CL_PredictMove.
func TestPredictPlayersStrafeMovement(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{
		Origin: types.Vec3{X: 0, Y: 0, Z: 0},
	}

	// Initialize prediction
	c.PredictPlayers(0.016)

	// Apply strafe movement command
	c.PendingCmd = UserCmd{
		ViewAngles: types.Vec3{X: 0, Y: 0, Z: 0}, // Facing forward
		Side:       350,                         // Strafe right
	}

	initialOrigin := c.PredictedOrigin

	// Predict with strafe movement
	c.PredictPlayers(0.016)

	// Position should have moved
	if c.PredictedOrigin == initialOrigin {
		t.Error("Position did not change with strafe movement")
	}
}

// TestPredictPlayersMultipleFrames verifies that prediction remains stable over multiple successive frames.
// Why: Ensures that prediction errors do not compound rapidly over time.
// Where in C: cl_main.c, CL_PredictMove.
func TestPredictPlayersMultipleFrames(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{
		Origin: types.Vec3{X: 0, Y: 0, Z: 0},
	}

	// Initialize
	c.PredictPlayers(0.016)

	// Apply movement over multiple frames
	c.PendingCmd = UserCmd{
		ViewAngles: types.Vec3{X: 0, Y: 0, Z: 0},
		Forward:    200,
	}

	for i := 0; i < 60; i++ {
		c.PredictPlayers(0.016)
	}

	// Should have moved after 60 frames (~1 second)
	distance := c.PredictedOrigin.Len()

	if distance < 0.1 {
		t.Errorf("Distance too small after 60 frames: %.2f", distance)
	}
}

// TestPredictPlayersConsumesBufferedCommands verifies that the client replays all unacknowledged commands during prediction.
// Why: Prediction must account for all inputs sent to the server that have not yet been reflected in a server update.
// Where in C: cl_main.c, CL_PredictMove.
func TestPredictPlayersConsumesBufferedCommands(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.Signon = Signons
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{Origin: types.Vec3{X: 0, Y: 0, Z: 0}}
	c.PredictPlayers(0.016)

	c.enqueueCommand(UserCmd{ViewAngles: types.Vec3{X: 0, Y: 0, Z: 0}, Forward: 200})
	c.enqueueCommand(UserCmd{ViewAngles: types.Vec3{X: 0, Y: 90, Z: 0}, Side: 200})

	c.PredictPlayers(0.032)
	if c.CommandCount != 2 {
		t.Fatalf("command count after prediction = %d, want 2 (unacknowledged)", c.CommandCount)
	}
	if c.PredictedOrigin == (types.Vec3{}) {
		t.Fatal("predicted origin unchanged after buffered command prediction")
	}
}

// TestConsumeCommandBufferHandlesNegativeSequence ensures that the command buffer correctly handles sequence number wrap-around.
// Why: Protocol stability over long play sessions requires handling integer overflows gracefully.
// Where in C: cl_main.c, CL_PredictMove.
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

// TestPredictPlayersRebasesFromServerOriginEachFrame verifies that each prediction starts from the latest authoritative server position.
// Why: Prevents local prediction errors from accumulating; the server is the source of truth.
// Where in C: cl_main.c, CL_PredictMove.
func TestPredictPlayersRebasesFromServerOriginEachFrame(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.ViewEntity = 0
	c.OnGround = true
	c.Entities[0] = inet.EntityState{Origin: types.Vec3{X: 0, Y: 0, Z: 0}}
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

// TestPredictPlayersPendingFallbackRebasesEachFrame verifies fallback behavior when no new server updates are available.
// Why: Maintains movement responsiveness during brief periods of packet loss.
// Where in C: cl_main.c, CL_PredictMove.
func TestPredictPlayersPendingFallbackRebasesEachFrame(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.ViewEntity = 0
	c.OnGround = false
	c.PredictionGravity = 0
	c.Entities[0] = inet.EntityState{Origin: types.Vec3{X: 10, Y: 20, Z: 30}}
	c.Velocity = types.Vec3{X: 0, Y: 100, Z: 0}
	c.PendingCmd = UserCmd{ViewAngles: types.Vec3{X: 0, Y: 0, Z: 0}, Msec: 16}

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

// TestPredictPlayersRecordsCurrentFrameTelemetryAndValidity ensures that prediction results and diagnostic data are correctly recorded.
// Why: Necessary for debugging prediction issues and providing feedback to the user/developer.
// Where in C: cl_main.c, CL_PredictMove.
func TestPredictPlayersRecordsCurrentFrameTelemetryAndValidity(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.Time = 1.25
	c.ViewEntity = 1
	c.OnGround = true
	c.Entities[1] = inet.EntityState{Origin: types.Vec3{X: 10, Y: 20, Z: 30}}
	c.PendingCmd = UserCmd{
		ViewAngles: types.Vec3{X: 0, Y: 0, Z: 0},
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
	if telemetry.ServerBaseOrigin != (types.Vec3{X: 10, Y: 20, Z: 30}) {
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

// TestPredictPlayersInvalidatesMissingEntityAndTelemetry ensures prediction is marked invalid if the local entity is missing.
// Why: Informs other systems (like the renderer) that the current predicted state is unreliable.
// Where in C: cl_main.c, CL_PredictMove.
func TestPredictPlayersInvalidatesMissingEntityAndTelemetry(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.Time = 2.5
	c.ViewEntity = 1
	c.PredictedOrigin = types.Vec3{X: 99, Y: 88, Z: 77}
	c.PredictedVelocity = types.Vec3{X: 1, Y: 2, Z: 3}
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
	if telemetry.OutputPredictedOrigin != (types.Vec3{X: 99, Y: 88, Z: 77}) {
		t.Fatalf("telemetry.OutputPredictedOrigin = %v, want stale predicted origin snapshot", telemetry.OutputPredictedOrigin)
	}
	if telemetry.OutputPredictedVelocity != (types.Vec3{X: 1, Y: 2, Z: 3}) {
		t.Fatalf("telemetry.OutputPredictedVelocity = %v, want stale predicted velocity snapshot", telemetry.OutputPredictedVelocity)
	}
}

// TestGetPredictedOriginReturnsCorrectValue verifies the accessor for the predicted position.
// Why: Provides a clean interface for the renderer to retrieve the smooth predicted origin.
// Where in C: cl_main.c, CL_PredictMove.
func TestGetPredictedOriginReturnsCorrectValue(t *testing.T) {
	c := NewClient()
	c.PredictedOrigin = types.Vec3{X: 10, Y: 20, Z: 30}

	origin := c.GetPredictedOrigin()
	if origin != (types.Vec3{X: 10, Y: 20, Z: 30}) {
		t.Errorf("PredictedOrigin returned %v, want [10 20 30]", origin)
	}
}

// TestGetPredictedVelocityReturnsCorrectValue verifies the accessor for the predicted velocity.
// Why: Allows effects (like wind or movement-based particles) to use the predicted velocity.
// Where in C: cl_main.c, CL_PredictMove.
func TestGetPredictedVelocityReturnsCorrectValue(t *testing.T) {
	c := NewClient()
	c.PredictedVelocity = types.Vec3{X: 100, Y: 50, Z: 25}

	velocity := c.GetPredictedVelocity()
	if velocity != (types.Vec3{X: 100, Y: 50, Z: 25}) {
		t.Errorf("PredictedVelocity returned %v, want [100 50 25]", velocity)
	}
}

// TestAngleVectorsQuake verifies the Quake-specific implementation of angle-to-vector conversion.
// Why: Quake uses a specific coordinate system and rotation order that must be matched exactly for correct movement.
// Where in C: mathlib.c, AngleVectors.
func TestAngleVectorsQuake(t *testing.T) {
	// Test forward vector (no rotation)
	angles := types.Vec3{X: 0, Y: 0, Z: 0}
	forward, _, _ := types.AngleVectors(angles)

	// Forward should be approximately (1, 0, 0)
	if absFloat32(forward.X-1.0) > 0.01 || absFloat32(forward.Y) > 0.01 || absFloat32(forward.Z) > 0.01 {
		t.Errorf("Forward vector incorrect: got %v, want ~[1 0 0]", forward)
	}

	// Test 90 degree yaw rotation
	angles = types.Vec3{X: 0, Y: 90, Z: 0}
	forward, _, _ = types.AngleVectors(angles)

	// Forward should be approximately (0, 1, 0) after 90 degree yaw
	if absFloat32(forward.X) > 0.01 || absFloat32(forward.Y-1.0) > 0.01 || absFloat32(forward.Z) > 0.01 {
		t.Errorf("Forward vector after 90° yaw incorrect: got %v, want ~[0 1 0]", forward)
	}
}

// TestPredictionMovementAnglesMatchesServerSemantics ensures that client-side movement angles match the server's calculation.
// Why: Differences in angle calculation would lead to incorrect movement directions and prediction drift.
// Where in C: cl_main.c, CL_PredictMove.
func TestPredictionMovementAnglesMatchesServerSemantics(t *testing.T) {
	c := NewClient()
	c.PunchAngle = types.Vec3{X: 6, Y: -15, Z: 4}

	got := c.predictionMovementAngles(types.Vec3{X: -30, Y: 90, Z: 17})
	want := types.Vec3{X: 8, Y: 75, Z: 0}

	if got != want {
		t.Fatalf("predictionMovementAngles = %v, want %v", got, want)
	}
}

// TestPredictMovementUsesServerStylePitchForAcceleration verifies that pitch affects movement acceleration as it does on the server.
// Why: Quake allows small amounts of vertical acceleration based on pitch in some movement modes (e.g., swimming).
// Where in C: cl_main.c, CL_PredictMove.
func TestPredictMovementUsesServerStylePitchForAcceleration(t *testing.T) {
	c := NewClient()
	c.OnGround = false
	c.PredictionGravity = 0
	c.PredictionAccel = 10
	c.PredictionMaxSpeed = 1000
	c.PunchAngle = types.Vec3{X: 30, Y: 0, Z: 0}

	cmd := UserCmd{
		ViewAngles: types.Vec3{X: -30, Y: 0, Z: 15},
		Forward:    320,
	}

	c.predictMovement(&cmd, 0.016)

	wantAccel := float32(c.PredictionAccel * 0.016 * cmd.Forward)
	if absFloat32(c.PredictedVelocity.X-wantAccel) > 0.001 {
		t.Fatalf("PredictedVelocity[0] = %.3f, want %.3f from server-style move pitch", c.PredictedVelocity.X, wantAccel)
	}
	if absFloat32(c.PredictedVelocity.Y) > 0.001 || absFloat32(c.PredictedVelocity.Z) > 0.001 {
		t.Fatalf("PredictedVelocity = %v, want only +X acceleration", c.PredictedVelocity)
	}
}

// TestPredictionMovementAnglesIncludeServerStyleRoll verifies that view roll is accounted for in movement direction.
// Why: Ensures that side-to-side tilting (roll) doesn't break movement prediction.
// Where in C: cl_main.c, CL_PredictMove.
func TestPredictionMovementAnglesIncludeServerStyleRoll(t *testing.T) {
	c := NewClient()
	c.PredictedVelocity = types.Vec3{X: 0, Y: 200, Z: 0}

	got := c.predictionMovementAngles(types.Vec3{})
	want := types.Vec3{X: 0, Y: 0, Z: -8}

	if got != want {
		t.Fatalf("predictionMovementAngles roll = %v, want %v", got, want)
	}
}

func absFloat32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// TestAbsFloat32 verifies the absolute value helper for float32.
// Why: Essential math utility for distance and error calculations.
// Where in C: mathlib.c.
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

// TestMaxFloat32 verifies the maximum value helper for float32.
// Why: Essential math utility for clamping and bounds checking.
// Where in C: mathlib.c.
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

// TestSqrtFloat32 verifies the square root helper for float32.
// Why: Essential math utility for calculating vector magnitudes and distances.
// Where in C: mathlib.c.
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
