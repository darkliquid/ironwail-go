package client

import (
	"math"

	"github.com/darkliquid/ironwail-go/pkg/types"
)

// PredictPlayers updates the predicted player position and velocity based on
// accumulated input commands. This provides client-side movement prediction
// to reduce perceived lag. The prediction is corrected when server updates arrive.
//
// This should be called once per frame after input processing but before rendering.
// The predicted position (c.PredictedOrigin) is currently used as a fallback when
// authoritative player origin is unavailable, because prediction remains
// collisionless and should not override the server-driven render origin.
//
// Algorithm:
//  1. Start with last known server entity state
//  2. Apply all accumulated user commands since last server update
//  3. For each command: apply friction/acceleration, gravity, update position
//  4. Calculate prediction error (difference from server position)
//  5. Smoothly correct error over time using lerp
//
// The prediction is framerate-independent and uses simplified physics
// (no collision detection). Full collision-aware prediction is future work.
func (c *Client) resetLocalTeleportPrediction(origin types.Vec3) {
	if c == nil {
		return
	}
	c.LastServerOrigin = origin
	c.PredictedOrigin = origin
	c.PredictionError = types.Vec3{}
	c.PredictionValid = false
	c.PredictionEntityNum = 0
	c.PredictionFrameTime = 0
	c.Velocity = types.Vec3{}
	c.MVelocity = [2]types.Vec3{}
	c.PredictedVelocity = types.Vec3{}
	c.CommandCount = 0
}

func (c *Client) PredictPlayers(frametime float32) {
	if c == nil {
		return
	}
	if c.State != StateActive {
		c.PredictionValid = false
		c.PredictionEntityNum = 0
		c.PredictionFrameTime = 0
		c.LastPredictionReplayTelemetry = PredictionReplayTelemetry{}
		return
	}

	entNum := c.predictionEntityNum()
	telemetry := PredictionReplayTelemetry{
		FrameTime:               c.Time,
		EntityNum:               entNum,
		PendingCmd:              c.PendingCmd,
		PreviousPredictedOrigin: c.PredictedOrigin,
		CommandCountBeforeAck:   c.CommandCount,
	}
	c.PredictionValid = false
	c.PredictionEntityNum = entNum
	c.PredictionFrameTime = c.Time

	ent, ok := c.Entities[entNum]
	if !ok {
		// No player entity yet, can't predict
		telemetry.CommandCountAfterAck = c.CommandCount
		telemetry.RebasedPredictedOrigin = c.PredictedOrigin
		telemetry.RebasedPredictedVelocity = c.PredictedVelocity
		telemetry.OutputPredictedOrigin = c.PredictedOrigin
		telemetry.OutputPredictedVelocity = c.PredictedVelocity
		c.LastPredictionReplayTelemetry = telemetry
		return
	}
	telemetry.EntityFound = true
	telemetry.ServerBaseOrigin = ent.Origin
	telemetry.ServerBaseVelocity = c.Velocity

	// On first run or server update, initialize prediction state
	if c.LastServerOrigin == (types.Vec3{}) {
		c.LastServerOrigin = ent.Origin
		c.PredictedOrigin = ent.Origin
		c.PredictedVelocity = c.Velocity
		// Don't return - continue to run prediction this frame
	}

	// Check if server sent a new update (origin changed)
	if ent.Origin != c.LastServerOrigin {
		// Calculate prediction error (where we predicted vs where server says we are)
		c.PredictionError = ent.Origin.Sub(c.PredictedOrigin)

		// Update last known server position
		telemetry.ServerBaseChanged = true
		c.LastServerOrigin = ent.Origin
	}
	telemetry.CommandCountAfterAck = c.CommandCount

	// Keep prediction error as a decaying telemetry/guard signal.
	if c.PredictionError != (types.Vec3{}) {
		errorLerpSpeed := c.PredictionErrorLerp * frametime * 60.0 // Scale for 60fps baseline
		if errorLerpSpeed > 1.0 {
			errorLerpSpeed = 1.0
		}

		c.PredictionError = c.PredictionError.Scale(1.0 - errorLerpSpeed)

		// Clear error if very small
		if c.PredictionError.LenSq() < 0.000001 {
			c.PredictionError = types.Vec3{}
		}
	}
	commands := c.bufferedCommands()
	if telemetry.ServerBaseChanged || len(commands) > 0 {
		// When replaying a buffered backlog, restart from the latest
		// authoritative base so old commands are not compounded frame-over-frame.
		c.PredictedOrigin = ent.Origin
		c.PredictedVelocity = c.Velocity
	}
	if len(commands) == 0 {
		// PendingCmd is a between-send preview only. Restart from the current
		// authoritative base each render frame so stale predicted velocity does
		// not compound while waiting for the next real send/ack.
		c.PredictedOrigin = ent.Origin
		c.PredictedVelocity = c.Velocity
		commands = append(commands, c.PendingCmd)
		telemetry.UsedPendingCmdFallback = true
	}
	telemetry.RebasedPredictedOrigin = c.PredictedOrigin
	telemetry.RebasedPredictedVelocity = c.PredictedVelocity
	telemetry.ReplayedCommandCount = len(commands)
	if len(commands) > 0 {
		telemetry.HasReplayedCmds = true
		telemetry.OldestReplayedCmd = commands[0]
		telemetry.NewestReplayedCmd = commands[len(commands)-1]
	}
	for i := range commands {
		cmdFrametime := frametime / float32(len(commands))
		if commands[i].Msec > 0 {
			cmdFrametime = float32(commands[i].Msec) / 1000.0
		}
		c.predictMovement(&commands[i], cmdFrametime)
	}
	telemetry.OutputPredictedOrigin = c.PredictedOrigin
	telemetry.OutputPredictedVelocity = c.PredictedVelocity
	telemetry.Valid = true
	c.PredictionValid = true
	c.LastPredictionReplayTelemetry = telemetry
}

func (c *Client) predictionEntityNum() int {
	if c == nil {
		return 0
	}
	if c.ViewEntity != 0 {
		return c.ViewEntity
	}
	// FitzQuake single-player viewentity is typically 1 after svc_setview.
	// Before or around signon, prefer entity 1 when present so local prediction
	// tracks the actual player instead of a nonexistent entity 0.
	if _, ok := c.Entities[1]; ok {
		return 1
	}
	return 0
}

// predictMovement simulates player movement for a single command.
// This is a simplified movement model without collision detection.
func (c *Client) predictMovement(cmd *UserCmd, frametime float32) {
	if cmd == nil || frametime <= 0 {
		return
	}

	// Match the server movement basis from SV_ClientThink/SV_AirMove:
	// use v_angle + punch, then derive movement angles with pitch scaled by -1/3
	// and roll from velocity. Keep water movement on raw view angles to match
	// the server's water movement path.
	angles := c.predictionMovementAngles(cmd.ViewAngles)
	forward, right, _ := types.AngleVectors(angles)

	if c.OnGround {
		applyGroundFriction(&c.PredictedVelocity, c.PredictionFriction, c.PredictionStopSpeed, frametime)
	}

	wishVelX := forward.X*cmd.Forward + right.X*cmd.Side
	wishVelY := forward.Y*cmd.Forward + right.Y*cmd.Side
	wishSpeed := sqrtFloat32(wishVelX*wishVelX + wishVelY*wishVelY)
	if wishSpeed > 0 {
		wishDirX := wishVelX / wishSpeed
		wishDirY := wishVelY / wishSpeed
		if wishSpeed > c.PredictionMaxSpeed {
			wishSpeed = c.PredictionMaxSpeed
		}
		currentSpeed := c.PredictedVelocity.X*wishDirX + c.PredictedVelocity.Y*wishDirY
		addSpeed := wishSpeed - currentSpeed
		if addSpeed > 0 {
			accelSpeed := c.PredictionAccel * frametime * wishSpeed
			if accelSpeed > addSpeed {
				accelSpeed = addSpeed
			}
			c.PredictedVelocity.X += wishDirX * accelSpeed
			c.PredictedVelocity.Y += wishDirY * accelSpeed
		}
	}

	if cmd.Up != 0 {
		c.PredictedVelocity.Z += cmd.Up * c.PredictionAccel * frametime
	}
	if !c.OnGround {
		c.PredictedVelocity.Z -= c.PredictionGravity * frametime
	}

	// Update position
	c.PredictedOrigin = c.PredictedOrigin.Add(c.PredictedVelocity.Scale(frametime))
}

func (c *Client) predictionMovementAngles(viewAngles types.Vec3) types.Vec3 {
	if c != nil && c.InWater {
		return viewAngles
	}

	punchAngles := types.Vec3{}
	predictedVelocity := types.Vec3{}
	if c != nil {
		punchAngles = c.PunchAngle
		predictedVelocity = c.PredictedVelocity
	}

	vAngle := viewAngles.Add(punchAngles)
	angles := types.Vec3{
		X: -vAngle.X / 3,
		Y: vAngle.Y,
	}
	angles.Z = predictionCalcRoll(angles, predictedVelocity) * 4
	return angles
}

func predictionCalcRoll(angles, velocity types.Vec3) float32 {
	_, right, _ := types.AngleVectors(angles)

	side := velocity.Dot(right)
	sign := float32(1)
	if side < 0 {
		sign = -1
		side = -side
	}

	const (
		rollAngle = float32(2.0)
		rollSpeed = float32(200.0)
	)

	if rollSpeed == 0 {
		return 0
	}
	if side < rollSpeed {
		side = side * rollAngle / rollSpeed
	} else {
		side = rollAngle
	}

	return side * sign
}

func applyGroundFriction(velocity *types.Vec3, friction, stopSpeed, frametime float32) {
	if velocity == nil || frametime <= 0 {
		return
	}
	speed := sqrtFloat32(velocity.X*velocity.X + velocity.Y*velocity.Y)
	if speed <= 0 {
		return
	}
	control := speed
	if stopSpeed > control {
		control = stopSpeed
	}
	drop := control * friction * frametime
	newSpeed := speed - drop
	if newSpeed < 0 {
		newSpeed = 0
	}
	if newSpeed == speed {
		return
	}
	scale := newSpeed / speed
	velocity.X *= scale
	velocity.Y *= scale
}

// GetPredictedOrigin returns the predicted player origin for rendering.
// This should be used instead of the raw server entity origin to reduce lag.
// Retains its Get prefix because the Client struct already exposes a
// PredictedOrigin field; see go-guide.md §2.
func (c *Client) GetPredictedOrigin() types.Vec3 {
	if c == nil {
		return types.Vec3{}
	}
	return c.PredictedOrigin
}

// GetPredictedVelocity returns the predicted player velocity.
// Retains its Get prefix for the same reason as GetPredictedOrigin.
func (c *Client) GetPredictedVelocity() types.Vec3 {
	if c == nil {
		return types.Vec3{}
	}
	return c.PredictedVelocity
}

// sqrtFloat32 is a helper that wraps math.Sqrt for float32.
func sqrtFloat32(x float32) float32 {
	return float32(math.Sqrt(float64(x)))
}
