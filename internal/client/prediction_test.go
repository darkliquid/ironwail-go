package client

import (
	"testing"

	inet "github.com/ironwail/ironwail-go/internal/net"
)

func TestPredictPlayersInitialization(t *testing.T) {
	c := NewClient()
	c.State = StateActive
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
	c.ViewEntity = 0
	c.Entities[0] = inet.EntityState{
		Origin: [3]float32{0, 0, 0},
	}

	// Initialize prediction
	c.PredictPlayers(0.016)

	// Set velocity above max speed
	c.PredictedVelocity = [3]float32{400, 0, 0} // Above default 320

	// Apply prediction
	c.PendingCmd = UserCmd{
		ViewAngles: [3]float32{0, 0, 0},
	}
	c.PredictPlayers(0.016)

	// Calculate speed
	speed := sqrtFloat32(
		c.PredictedVelocity[0]*c.PredictedVelocity[0] +
			c.PredictedVelocity[1]*c.PredictedVelocity[1] +
			c.PredictedVelocity[2]*c.PredictedVelocity[2])

	// Speed should be clamped to max
	if speed > c.PredictionMaxSpeed+0.1 {
		t.Errorf("Speed not clamped: got %.2f, max %.2f",
			speed, c.PredictionMaxSpeed)
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
