// Package server implements the Quake server physics and game logic.
//
// sv_user.go handles client command processing and player movement.
// This includes:
//   - Client message parsing (movement, impulses, string commands)
//   - Player physics (walking, swimming, noclip)
//   - Friction and acceleration
//   - View angle calculation (ideal pitch)
package server

import (
	"math"
	"strings"
)

// IdealPitch constants for slope-based view adjustment.
const (
	MaxForward      = 6     // Number of trace samples for pitch calc
	IdealPitchScale = 0.8   // Scale factor for ideal pitch
	EdgeFriction    = 2.0   // Additional friction at edges
	StopSpeed       = 100.0 // Minimum speed threshold for friction
	MaxSpeed        = 320.0 // Maximum player speed
	Accelerate      = 10.0  // Acceleration factor
)

// ClientMoveContext holds temporary state during client movement processing.
// Used to avoid passing many parameters between functions.
type ClientMoveContext struct {
	Player   *Edict     // The player entity being processed
	Origin   [3]float32 // Current origin (pointer to ent.v.origin)
	Velocity [3]float32 // Current velocity (pointer to ent.v.velocity)
	Angles   [3]float32 // Current angles (pointer to ent.v.angles)
	Cmd      UserCmd    // The input command for this frame
	OnGround bool       // Whether player is on ground

	// Direction vectors calculated from angles
	Forward [3]float32
	Right   [3]float32
	Up      [3]float32
}

// SetIdealPitch calculates the ideal view pitch based on the slope of the
// ground the player is walking on. This gives a natural view adjustment
// when walking up or down slopes.
//
// The algorithm traces forward at several distances, measuring the ground
// height at each point. If the ground rises or falls consistently, it
// adjusts the ideal pitch to match.
func (s *Server) SetIdealPitch(ent *Edict) {
	// Only adjust pitch if on ground
	if uint32(ent.Vars.Flags)&FlagOnGround == 0 {
		return
	}

	// Calculate forward direction from yaw angle
	angle := float64(ent.Vars.Angles[1] * math.Pi * 2 / 360)
	sinVal := float32(math.Sin(angle))
	cosVal := float32(math.Cos(angle))

	// Sample ground heights at increasing distances
	z := [MaxForward]float32{}
	var i, j int
	var step, dir float32
	var steps int

	for i = 0; i < MaxForward; i++ {
		// Trace point at distance (i+3)*12 forward
		top := [3]float32{
			ent.Vars.Origin[0] + cosVal*float32(i+3)*12,
			ent.Vars.Origin[1] + sinVal*float32(i+3)*12,
			ent.Vars.Origin[2] + ent.Vars.ViewOfs[2],
		}
		bottom := [3]float32{
			top[0],
			top[1],
			top[2] - 160,
		}

		tr := s.Move(top, [3]float32{}, [3]float32{}, bottom, MoveTypeNone, ent)
		if tr.AllSolid {
			return // Looking at a wall, leave ideal pitch as is
		}
		if tr.Fraction == 1 {
			return // Near a dropoff
		}
		z[i] = top[2] + tr.Fraction*(bottom[2]-top[2])
	}

	// Analyze the slope pattern
	dir = 0
	steps = 0
	for j = 1; j < i; j++ {
		step = z[j] - z[j-1]
		if step > -OneEpsilon && step < OneEpsilon {
			continue
		}
		if dir != 0 && (step-dir > OneEpsilon || step-dir < -OneEpsilon) {
			return // Mixed slope changes, don't adjust
		}
		steps++
		dir = step
	}

	if dir == 0 {
		ent.Vars.IdealPitch = 0
		return
	}

	if steps < 2 {
		return
	}

	ent.Vars.IdealPitch = -dir * IdealPitchScale
}

// UserFriction applies friction to the player's horizontal velocity.
// If the player is near a dropoff (edge), additional edge friction is applied
// to prevent them from sliding off.
func (s *Server) UserFriction(ctx *ClientMoveContext) {
	speed := float32(math.Sqrt(float64(ctx.Velocity[0]*ctx.Velocity[0] +
		ctx.Velocity[1]*ctx.Velocity[1])))
	if speed == 0 {
		return
	}

	// Trace ahead to check for dropoffs
	start := [3]float32{
		ctx.Origin[0] + ctx.Velocity[0]/speed*16,
		ctx.Origin[1] + ctx.Velocity[1]/speed*16,
		ctx.Origin[2] + ctx.Player.Vars.Mins[2],
	}
	stop := [3]float32{
		start[0],
		start[1],
		start[2] - 34,
	}

	trace := s.Move(start, [3]float32{}, [3]float32{}, stop, MoveTypeNone, ctx.Player)

	// Apply edge friction if near dropoff
	var friction float32
	if trace.Fraction == 1.0 {
		friction = s.Friction * EdgeFriction
	} else {
		friction = s.Friction
	}

	// Calculate new speed after friction
	control := speed
	if control < StopSpeed {
		control = StopSpeed
	}
	newspeed := speed - s.FrameTime*control*friction
	if newspeed < 0 {
		newspeed = 0
	}
	newspeed /= speed

	// Apply to velocity
	ctx.Player.Vars.Velocity[0] *= newspeed
	ctx.Player.Vars.Velocity[1] *= newspeed
	ctx.Player.Vars.Velocity[2] *= newspeed
}

// Accelerate increases velocity towards a desired direction and speed.
// Used for ground movement.
func (s *Server) Accelerate(ctx *ClientMoveContext, wishspeed float32, wishdir [3]float32) {
	currentspeed := VecDot(ctx.Velocity, wishdir)
	addspeed := wishspeed - currentspeed
	if addspeed <= 0 {
		return
	}

	accelspeed := Accelerate * s.FrameTime * wishspeed
	if accelspeed > addspeed {
		accelspeed = addspeed
	}

	ctx.Player.Vars.Velocity[0] += accelspeed * wishdir[0]
	ctx.Player.Vars.Velocity[1] += accelspeed * wishdir[1]
	ctx.Player.Vars.Velocity[2] += accelspeed * wishdir[2]
}

// AirAccelerate increases velocity while airborne.
// Has a lower speed cap than ground acceleration.
func (s *Server) AirAccelerate(ctx *ClientMoveContext, wishspeed float32, wishveloc [3]float32) {
	wishspd := VecNormalize(&wishveloc)
	if wishspd > 30 {
		wishspd = 30
	}

	currentspeed := VecDot(ctx.Velocity, wishveloc)
	addspeed := wishspd - currentspeed
	if addspeed <= 0 {
		return
	}

	accelspeed := Accelerate * wishspeed * s.FrameTime
	if accelspeed > addspeed {
		accelspeed = addspeed
	}

	ctx.Player.Vars.Velocity[0] += accelspeed * wishveloc[0]
	ctx.Player.Vars.Velocity[1] += accelspeed * wishveloc[1]
	ctx.Player.Vars.Velocity[2] += accelspeed * wishveloc[2]
}

// DropPunchAngle gradually reduces the punchangle (view kick from damage).
func (s *Server) DropPunchAngle(ent *Edict) {
	len := VecLen(ent.Vars.PunchAngle)
	if len == 0 {
		return
	}

	len -= 10 * s.FrameTime
	if len < 0 {
		len = 0
	}

	// Normalize and rescale
	VecNormalize(&ent.Vars.PunchAngle)
	ent.Vars.PunchAngle[0] *= len
	ent.Vars.PunchAngle[1] *= len
	ent.Vars.PunchAngle[2] *= len
}

// WaterMove handles player movement while swimming.
// Players can move in all directions and slowly sink if not moving.
func (s *Server) WaterMove(ctx *ClientMoveContext) {
	// Calculate wish velocity from input
	AngleVectors(ctx.Player.Vars.VAngle, &ctx.Forward, &ctx.Right, &ctx.Up)

	var wishvel [3]float32
	for i := 0; i < 3; i++ {
		wishvel[i] = ctx.Forward[i]*ctx.Cmd.ForwardMove + ctx.Right[i]*ctx.Cmd.SideMove
	}

	// Drift down if no input
	if ctx.Cmd.ForwardMove == 0 && ctx.Cmd.SideMove == 0 && ctx.Cmd.UpMove == 0 {
		wishvel[2] -= 60
	} else {
		wishvel[2] += ctx.Cmd.UpMove
	}

	wishspeed := VecLen(wishvel)
	if wishspeed > MaxSpeed {
		wishvel = VecScale(wishvel, MaxSpeed/wishspeed)
		wishspeed = MaxSpeed
	}
	wishspeed *= 0.7 // Water is slower

	// Apply water friction
	speed := VecLen(ctx.Velocity)
	var newspeed float32
	if speed > 0 {
		newspeed = speed - s.FrameTime*speed*s.Friction
		if newspeed < 0 {
			newspeed = 0
		}
		ctx.Player.Vars.Velocity = VecScale(ctx.Player.Vars.Velocity, newspeed/speed)
	}

	// Apply water acceleration
	if wishspeed == 0 {
		return
	}

	addspeed := wishspeed - newspeed
	if addspeed <= 0 {
		return
	}

	VecNormalize(&wishvel)
	accelspeed := Accelerate * wishspeed * s.FrameTime
	if accelspeed > addspeed {
		accelspeed = addspeed
	}

	ctx.Player.Vars.Velocity[0] += accelspeed * wishvel[0]
	ctx.Player.Vars.Velocity[1] += accelspeed * wishvel[1]
	ctx.Player.Vars.Velocity[2] += accelspeed * wishvel[2]
}

// WaterJump handles the water jump behavior (leaping out of water).
func (s *Server) WaterJump(ent *Edict) {
	// End water jump when teleport time expires or out of water
	if s.Time > ent.Vars.TeleportTime || ent.Vars.WaterLevel < 2 {
		ent.Vars.Flags = float32(uint32(ent.Vars.Flags) & ^uint32(FlagWaterJump))
		ent.Vars.TeleportTime = 0
	}

	// Continue with water jump velocity
	ent.Vars.Velocity[0] = ent.Vars.MoveDir[0]
	ent.Vars.Velocity[1] = ent.Vars.MoveDir[1]
}

// NoclipMove handles movement when noclip is enabled.
// The player can fly through walls at increased speed.
func (s *Server) NoclipMove(ctx *ClientMoveContext) {
	AngleVectors(ctx.Player.Vars.VAngle, &ctx.Forward, &ctx.Right, &ctx.Up)

	ctx.Player.Vars.Velocity[0] = ctx.Forward[0]*ctx.Cmd.ForwardMove +
		ctx.Right[0]*ctx.Cmd.SideMove
	ctx.Player.Vars.Velocity[1] = ctx.Forward[1]*ctx.Cmd.ForwardMove +
		ctx.Right[1]*ctx.Cmd.SideMove
	ctx.Player.Vars.Velocity[2] = ctx.Forward[2]*ctx.Cmd.ForwardMove +
		ctx.Right[2]*ctx.Cmd.SideMove
	ctx.Player.Vars.Velocity[2] += ctx.Cmd.UpMove * 2 // Doubled to match running speed

	// Cap at max speed
	speed := VecLen(ctx.Player.Vars.Velocity)
	if speed > MaxSpeed {
		VecNormalize(&ctx.Player.Vars.Velocity)
		ctx.Player.Vars.Velocity = VecScale(ctx.Player.Vars.Velocity, MaxSpeed)
	}
}

// AirMove handles player movement in air (walking, falling, jumping).
// This is the main movement function for normal gameplay.
func (s *Server) AirMove(ctx *ClientMoveContext) {
	AngleVectors(ctx.Player.Vars.Angles, &ctx.Forward, &ctx.Right, &ctx.Up)

	fmove := ctx.Cmd.ForwardMove
	smove := ctx.Cmd.SideMove

	// Hack to not let player back into teleporter
	if s.Time < ctx.Player.Vars.TeleportTime && fmove < 0 {
		fmove = 0
	}

	// Calculate wish velocity
	var wishvel, wishdir [3]float32
	for i := 0; i < 3; i++ {
		wishvel[i] = ctx.Forward[i]*fmove + ctx.Right[i]*smove
	}

	if MoveType(ctx.Player.Vars.MoveType) != MoveTypeWalk {
		wishvel[2] = ctx.Cmd.UpMove
	} else {
		wishvel[2] = 0
	}

	VecCopy(wishvel, &wishdir)
	wishspeed := VecNormalize(&wishdir)
	if wishspeed > MaxSpeed {
		wishvel = VecScale(wishvel, MaxSpeed/wishspeed)
		wishspeed = MaxSpeed
	}

	// Apply appropriate movement
	if MoveType(ctx.Player.Vars.MoveType) == MoveTypeNoClip {
		ctx.Player.Vars.Velocity = wishvel
	} else if ctx.OnGround {
		s.UserFriction(ctx)
		s.Accelerate(ctx, wishspeed, wishdir)
	} else {
		s.AirAccelerate(ctx, wishspeed, wishvel)
	}
}

// CalcRoll calculates view roll based on velocity (banking effect).
func CalcRoll(angles, velocity [3]float32) float32 {
	var forward, right, up [3]float32
	AngleVectors(angles, &forward, &right, &up)

	side := VecDot(velocity, right)
	var sign, value float32

	if side < 0 {
		sign = -1
		side = -side
	} else {
		sign = 1
	}

	value = side * 0.05 // Roll factor

	if value > 0.3 { // Max roll
		value = 0.3
	}

	return value * sign
}

// ClientThink processes a single client's input for one frame.
// Updates angles, handles movement, and executes impulses.
func (s *Server) ClientThink(client *Client) {
	ent := client.Edict
	if ent == nil || ent.Free {
		return
	}

	// Skip if entity can't move
	if MoveType(ent.Vars.MoveType) == MoveTypeNone {
		return
	}

	ctx := &ClientMoveContext{
		Player:   ent,
		Cmd:      client.LastCmd,
		OnGround: uint32(ent.Vars.Flags)&FlagOnGround != 0,
	}
	VecCopy(ent.Vars.Origin, &ctx.Origin)
	VecCopy(ent.Vars.Velocity, &ctx.Velocity)
	VecCopy(ent.Vars.Angles, &ctx.Angles)

	// Reduce punch angle from damage
	s.DropPunchAngle(ent)

	// Dead players don't move
	if ent.Vars.Health <= 0 {
		return
	}

	vAngle := VecAdd(ent.Vars.VAngle, ent.Vars.PunchAngle)



	// Calculate roll from velocity
	ent.Vars.Angles[2] = CalcRoll(ent.Vars.Angles, ent.Vars.Velocity) * 4

	// Update view angles
	if ent.Vars.FixAngle == 0 {
		ent.Vars.Angles[0] = -vAngle[0] / 3 // Pitch scaled down for view
		ent.Vars.Angles[1] = vAngle[1]      // Yaw unchanged
	}

	// Handle water jump
	if uint32(ent.Vars.Flags)&FlagWaterJump != 0 {
		s.WaterJump(ent)
		return
	}

	// Choose movement mode
	if MoveType(ent.Vars.MoveType) == MoveTypeNoClip {
		s.NoclipMove(ctx)
	} else if ent.Vars.WaterLevel >= 2 && MoveType(ent.Vars.MoveType) != MoveTypeNoClip {
		s.WaterMove(ctx)
	} else {
		s.AirMove(ctx)
	}
}

// ReadClientMove reads a client movement command from the message buffer.
// Updates the client's command state and entity view angles.
func (s *Server) ReadClientMove(client *Client, buf *MessageBuffer) UserCmd {
	var cmd UserCmd

	// Read ping time (time when packet was sent)
	pingTime := buf.ReadFloat()
	client.PingTimes[client.NumPings%16] = s.Time - pingTime
	client.NumPings++

	// Read view angles
	for i := 0; i < 3; i++ {
		cmd.ViewAngles[i] = buf.ReadAngle16()
	}
	client.Edict.Vars.VAngle = cmd.ViewAngles

	// Read movement
	cmd.ForwardMove = float32(buf.ReadShort())
	cmd.SideMove = float32(buf.ReadShort())
	cmd.UpMove = float32(buf.ReadShort())

	// Read buttons
	bits := buf.ReadByte()
	client.Edict.Vars.Button0 = float32(bits & 1)
	client.Edict.Vars.Button2 = float32((bits & 2) >> 1)

	// Read impulse
	impulse := buf.ReadByte()
	if impulse != 0 {
		client.Edict.Vars.Impulse = float32(impulse)
	}

	return cmd
}

// ReadClientMessage reads and processes a single message from a client.
// Returns false if the client should be disconnected.
func (s *Server) ReadClientMessage(client *Client, buf *MessageBuffer) bool {
	for {
		msgType := buf.ReadByte()
		if buf.BadRead {
			return false
		}

		switch NetMessageType(msgType) {
		case -1: // End of message (blocking read returned nothing)
			return true

		case CLCNop:
			// No operation

		case CLCStringCmd:
			// String command from client
			cmd := buf.ReadString()
			if !s.ExecuteClientString(client, cmd) {
				return false
			}

		case CLCDisconnect:
			return false

		case CLCMove:
			// Movement command
			client.LastCmd = s.ReadClientMove(client, buf)

		default:
			// Unknown command
			return false
		}

		if !client.Active {
			return false
		}
	}
}

// ExecuteClientString executes a string command from a client.
// Returns false if the client should be kicked.
func (s *Server) ExecuteClientString(client *Client, cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if len(cmd) == 0 {
		return true
	}

	// Parse command and arguments
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return true
	}

	command := strings.ToLower(parts[0])

	// Handle standard commands
	switch command {
	case "status", "god", "notarget", "fly", "name", "noclip",
		"setpos", "say", "say_team", "tell", "color", "kill",
		"pause", "spawn", "begin", "prespawn", "kick", "ping", "give":
		// These would normally be forwarded to the command system
		// For now, just log them
		// log.Printf("Client %s command: %s", client.Name, cmd)

	default:
		// Unknown command - could be QuakeC extension
		// log.Printf("Client %s tried to: %s", client.Name, cmd)
	}

	return true
}

// RunClients processes all connected clients for one frame.
// Reads messages and runs client thinking.
func (s *Server) RunClients() {
	for _, client := range s.Static.Clients {
		if !client.Active {
			continue
		}

		// Read client messages
		if !s.ReadClientMessage(client, client.Message) {
			s.DropClient(client, false)
			continue
		}

		// Skip if not spawned yet
		if !client.Spawned {
			// Clear movement until new packet
			client.LastCmd = UserCmd{}
			continue
		}

		// Run client thinking (unless paused in single player)
		if !s.Paused {
			s.ClientThink(client)
		}
	}
}

// DropClient disconnects a client from the server.
// If crash is true, it's due to a network error.
func (s *Server) DropClient(client *Client, crash bool) {
	if !client.Active {
		return
	}

	// Notify QuakeC of disconnection
	if client.Edict != nil && s.QCVM != nil {
		funcIdx := s.QCVM.FindFunction("ClientDisconnect")
		if funcIdx >= 0 {
			s.QCVM.Time = float64(s.Time)
			s.QCVM.SetGlobal("self", s.NumForEdict(client.Edict))
			s.QCVM.ExecuteFunction(funcIdx)
		}
	}

	// Clear client state
	client.Active = false
	client.Spawned = false
	client.Edict.Free = true
	client.Edict.FreeTime = s.Time
}

// AngleVectors converts angles (pitch, yaw, roll) to forward/right/up vectors.
func AngleVectors(angles [3]float32, forward, right, up *[3]float32) {
	angle := float64(angles[0]) * (math.Pi * 2 / 360)
	sp := float32(math.Sin(angle))
	cp := float32(math.Cos(angle))

	angle = float64(angles[1]) * (math.Pi * 2 / 360)
	sy := float32(math.Sin(angle))
	cy := float32(math.Cos(angle))

	angle = float64(angles[2]) * (math.Pi * 2 / 360)
	sr := float32(math.Sin(angle))
	cr := float32(math.Cos(angle))

	forward[0] = cp * cy
	forward[1] = cp * sy
	forward[2] = -sp

	right[0] = (-1*sr*sp*cy + -1*cr*-sy)
	right[1] = (-1*sr*sp*sy + -1*cr*cy)
	right[2] = -1 * sr * cp

	up[0] = (cr*sp*cy + -sr*-sy)
	up[1] = (cr*sp*sy + -sr*cy)
	up[2] = cr * cp
}

























































