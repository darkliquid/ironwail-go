package client

import "math"

// PredictPlayers updates the predicted player position and velocity based on
// accumulated input commands. This provides client-side movement prediction
// to reduce perceived lag. The prediction is corrected when server updates arrive.
//
// This should be called once per frame after input processing but before rendering.
// The predicted position (c.PredictedOrigin) should be used for view setup instead
// of the raw entity origin from the server.
//
// Algorithm:
//  1. Start with last known server entity state
//  2. Apply all accumulated user commands since last server update
//  3. For each command: apply acceleration, friction, clamp speed, update position
//  4. Calculate prediction error (difference from server position)
//  5. Smoothly correct error over time using lerp
//
// The prediction is framerate-independent and uses simplified physics
// (no collision detection, no gravity). Full physics prediction is future work.
func (c *Client) PredictPlayers(frametime float32) {
	if c == nil || c.State != StateActive {
		return
	}

	// Get player entity (view entity or entity 0)
	entNum := c.ViewEntity
	if entNum == 0 {
		entNum = 0 // Player is always entity 0 in single player
	}

	ent, ok := c.Entities[entNum]
	if !ok {
		// No player entity yet, can't predict
		return
	}

	// On first run or server update, initialize prediction state
	if c.LastServerOrigin == [3]float32{} {
		c.LastServerOrigin = ent.Origin
		c.PredictedOrigin = ent.Origin
		c.PredictedVelocity = c.Velocity
		// Don't return - continue to run prediction this frame
	}

	// Check if server sent a new update (origin changed)
	if ent.Origin != c.LastServerOrigin {
		// Calculate prediction error (where we predicted vs where server says we are)
		c.PredictionError = [3]float32{
			ent.Origin[0] - c.PredictedOrigin[0],
			ent.Origin[1] - c.PredictedOrigin[1],
			ent.Origin[2] - c.PredictedOrigin[2],
		}

		// Update last known server position
		c.LastServerOrigin = ent.Origin
		// Don't reset predicted origin immediately - error correction will smooth it
		// Reset velocity to server's velocity
		c.PredictedVelocity = c.Velocity
	}

	// Apply error correction (smooth lerp towards server)
	// This prevents jumpy corrections when prediction mismatches server
	if c.PredictionError != [3]float32{} {
		errorLerpSpeed := c.PredictionErrorLerp * frametime * 60.0 // Scale for 60fps baseline
		if errorLerpSpeed > 1.0 {
			errorLerpSpeed = 1.0
		}

		c.PredictedOrigin[0] += c.PredictionError[0] * errorLerpSpeed
		c.PredictedOrigin[1] += c.PredictionError[1] * errorLerpSpeed
		c.PredictedOrigin[2] += c.PredictionError[2] * errorLerpSpeed

		// Reduce error
		c.PredictionError[0] *= (1.0 - errorLerpSpeed)
		c.PredictionError[1] *= (1.0 - errorLerpSpeed)
		c.PredictionError[2] *= (1.0 - errorLerpSpeed)

		// Clear error if very small
		if absFloat32(c.PredictionError[0]) < 0.001 &&
			absFloat32(c.PredictionError[1]) < 0.001 &&
			absFloat32(c.PredictionError[2]) < 0.001 {
			c.PredictionError = [3]float32{}
		}
	}

	// Predict position based on current command
	// In a full implementation, we'd loop through c.CommandBuffer
	// For MVP, we predict based on current PendingCmd
	c.predictMovement(&c.PendingCmd, frametime)
}

// predictMovement simulates player movement for a single command.
// This is a simplified physics model without collision detection.
func (c *Client) predictMovement(cmd *UserCmd, frametime float32) {
	if cmd == nil || frametime <= 0 {
		return
	}

	// Get forward, right, up vectors from view angles
	angles := [3]float32{cmd.ViewAngles[0], cmd.ViewAngles[1], cmd.ViewAngles[2]}
	forward, right, up := angleVectorsQuake(angles)

	// Apply input to velocity (acceleration)
	// Forward/back movement
	if cmd.Forward != 0 {
		c.PredictedVelocity[0] += forward[0] * cmd.Forward * c.PredictionAccel * frametime
		c.PredictedVelocity[1] += forward[1] * cmd.Forward * c.PredictionAccel * frametime
		c.PredictedVelocity[2] += forward[2] * cmd.Forward * c.PredictionAccel * frametime
	}

	// Side (strafe) movement
	if cmd.Side != 0 {
		c.PredictedVelocity[0] += right[0] * cmd.Side * c.PredictionAccel * frametime
		c.PredictedVelocity[1] += right[1] * cmd.Side * c.PredictionAccel * frametime
		c.PredictedVelocity[2] += right[2] * cmd.Side * c.PredictionAccel * frametime
	}

	// Up movement (jump/swim)
	if cmd.Up != 0 {
		c.PredictedVelocity[0] += up[0] * cmd.Up * c.PredictionAccel * frametime
		c.PredictedVelocity[1] += up[1] * cmd.Up * c.PredictionAccel * frametime
		c.PredictedVelocity[2] += up[2] * cmd.Up * c.PredictionAccel * frametime
	}

	// Apply friction (velocity decay)
	speed := sqrtFloat32(
		c.PredictedVelocity[0]*c.PredictedVelocity[0] +
			c.PredictedVelocity[1]*c.PredictedVelocity[1] +
			c.PredictedVelocity[2]*c.PredictedVelocity[2])

	if speed > 0 {
		// Friction reduces velocity over time
		frictionScale := maxFloat32(0, 1.0-c.PredictionFriction*frametime)
		c.PredictedVelocity[0] *= frictionScale
		c.PredictedVelocity[1] *= frictionScale
		c.PredictedVelocity[2] *= frictionScale
	}

	// Clamp to max speed
	speed = sqrtFloat32(
		c.PredictedVelocity[0]*c.PredictedVelocity[0] +
			c.PredictedVelocity[1]*c.PredictedVelocity[1] +
			c.PredictedVelocity[2]*c.PredictedVelocity[2])

	if speed > c.PredictionMaxSpeed {
		scale := c.PredictionMaxSpeed / speed
		c.PredictedVelocity[0] *= scale
		c.PredictedVelocity[1] *= scale
		c.PredictedVelocity[2] *= scale
	}

	// Update position
	c.PredictedOrigin[0] += c.PredictedVelocity[0] * frametime
	c.PredictedOrigin[1] += c.PredictedVelocity[1] * frametime
	c.PredictedOrigin[2] += c.PredictedVelocity[2] * frametime
}

// GetPredictedOrigin returns the predicted player origin for rendering.
// This should be used instead of the raw server entity origin to reduce lag.
func (c *Client) GetPredictedOrigin() [3]float32 {
	if c == nil {
		return [3]float32{}
	}
	return c.PredictedOrigin
}

// GetPredictedVelocity returns the predicted player velocity.
func (c *Client) GetPredictedVelocity() [3]float32 {
	if c == nil {
		return [3]float32{}
	}
	return c.PredictedVelocity
}

// angleVectorsQuake calculates forward, right, and up vectors from angles.
// This is a local implementation to avoid circular imports with pkg/types.
func angleVectorsQuake(angles [3]float32) (forward, right, up [3]float32) {
	sy := math.Sin(float64(angles[1]) * (math.Pi * 2 / 360))
	cy := math.Cos(float64(angles[1]) * (math.Pi * 2 / 360))
	sp := math.Sin(float64(angles[0]) * (math.Pi * 2 / 360))
	cp := math.Cos(float64(angles[0]) * (math.Pi * 2 / 360))
	sr := math.Sin(float64(angles[2]) * (math.Pi * 2 / 360))
	cr := math.Cos(float64(angles[2]) * (math.Pi * 2 / 360))

	forward[0] = float32(cp * cy)
	forward[1] = float32(cp * sy)
	forward[2] = float32(-sp)

	right[0] = float32(-1*sr*sp*cy + -1*cr*-sy)
	right[1] = float32(-1*sr*sp*sy + -1*cr*cy)
	right[2] = float32(-1 * sr * cp)

	up[0] = float32(cr*sp*cy + -sr*-sy)
	up[1] = float32(cr*sp*sy + -sr*cy)
	up[2] = float32(cr * cp)
	return
}

// absFloat32 returns the absolute value of a float32.
func absFloat32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// maxFloat32 returns the maximum of two float32 values.
func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// sqrtFloat32 is a helper that wraps math.Sqrt for float32.
func sqrtFloat32(x float32) float32 {
	return float32(math.Sqrt(float64(x)))
}
