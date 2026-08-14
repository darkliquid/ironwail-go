// clientmove.go implements the client movement simulation (SV_ClientThink and
// its acceleration/friction/water/noclip helpers) as a component driven by
// injected dependencies. Mirrors sv_user.c / sv_phys.c movement in the C
// reference.
package physics

import (
	"math"

	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// Movement tuning constants (mirror sv_user.c).
const (
	edgeFriction = 2.0
	svMaxSpeed   = 320.0
	svAccelerate = 10.0
)

// ClientMover runs the per-client movement simulation for the server frame.
// It is created per frame with the injected facade and collision world so the
// move helpers run against narrow interfaces, not the whole server.
// MoveConfig is the narrow configuration surface the client mover needs:
// physics tuning constants, frame timing, and cvar reads.
type MoveConfig interface {
	srvtypes.PhysicsConfig
	srvtypes.FrameTiming
	srvtypes.CVarReader
}

type ClientMover struct {
	cfg MoveConfig
	col srvtypes.CollisionWorld
	sh  srvtypes.ServerHandle
}

// NewClientMover creates a ClientMover with injected dependencies.
func NewClientMover(cfg MoveConfig, col srvtypes.CollisionWorld, sh srvtypes.ServerHandle) *ClientMover {
	return &ClientMover{cfg: cfg, col: col, sh: sh}
}

// clientMoveContext carries per-frame movement state.
type clientMoveContext struct {
	player   *srvtypes.Edict
	origin   [3]float32
	velocity [3]float32
	cmd      srvtypes.UserCmd
	onground bool
	forward  [3]float32
	right    [3]float32
	up       [3]float32
}

// SV_ClientThink applies the client's movement commands to its edict for one
// frame: roll/pitch view, punch decay, and the water/noclip/air move paths.
func (m *ClientMover) SV_ClientThink(client *srvtypes.Client) {
	if client == nil || client.Edict == nil || client.Edict.Free {
		return
	}

	facade := m.cfg
	sh := m.sh
	ent := client.Edict
	if srvtypes.MoveType(ent.MoveType(sh)) == srvtypes.MoveTypeNone {
		return
	}

	ctx := &clientMoveContext{
		player:   ent,
		origin:   ent.Origin(sh),
		velocity: ent.Velocity(sh),
		cmd:      client.LastCmd,
		onground: uint32(ent.Flags(sh))&srvtypes.FlagOnGround != 0,
	}

	m.dropPunchAngle(ent)

	if ent.Health(sh) <= 0 {
		return
	}

	punchAngle := ent.PunchAngle(sh)
	vAngle := srvtypes.VecAdd(ent.VAngle(sh), punchAngle)
	angles := ent.Angles(sh)
	angles[2] = CalcRoll(facade, angles, ent.Velocity(sh)) * 4
	if ent.FixAngle(sh) == 0 {
		angles[0] = -vAngle[0] / 3
		angles[1] = vAngle[1]
	}
	ent.SetAngles(sh, angles)

	if uint32(ent.Flags(sh))&srvtypes.FlagWaterJump != 0 {
		m.waterJump(ent)
		return
	}

	if srvtypes.MoveType(ent.MoveType(sh)) == srvtypes.MoveTypeNoClip {
		m.noclipMove(ctx)
		return
	}
	if ent.WaterLevel(sh) >= 2 {
		m.waterMove(ctx)
		return
	}
	m.airMove(ctx)
}

// userFriction applies ground friction based on a short trace.
func (m *ClientMover) userFriction(ctx *clientMoveContext) {
	facade := m.cfg
	sh := m.sh
	vel := ctx.velocity
	speed := float32(math.Sqrt(float64(vel[0]*vel[0] + vel[1]*vel[1])))
	if speed == 0 {
		return
	}

	start := [3]float32{
		ctx.origin[0] + vel[0]/speed*16,
		ctx.origin[1] + vel[1]/speed*16,
		ctx.origin[2] + ctx.player.Mins(sh)[2],
	}
	stop := [3]float32{start[0], start[1], start[2] - 34}

	trace := m.col.SV_Move(start, [3]float32{}, [3]float32{}, stop, srvtypes.MoveNoMonsters, ctx.player)

	friction := facade.GetFriction()
	if trace.Fraction == 1.0 {
		friction *= edgeFriction
	}

	control := speed
	if control < facade.GetStopSpeed() {
		control = facade.GetStopSpeed()
	}

	newspeed := speed - facade.GetFrameTime()*control*friction
	if newspeed < 0 {
		newspeed = 0
	}
	newspeed /= speed

	vel = ctx.player.Velocity(sh)
	vel[0] *= newspeed
	vel[1] *= newspeed
	vel[2] *= newspeed
	ctx.player.SetVelocity(sh, vel)
	ctx.velocity = vel
}

// accelerate applies forward acceleration while grounded.
func (m *ClientMover) accelerate(wishspeed float32, wishdir [3]float32, ctx *clientMoveContext) {
	facade := m.cfg
	sh := m.sh
	currentSpeed := srvtypes.VecDot(ctx.player.Velocity(sh), wishdir)
	addspeed := wishspeed - currentSpeed
	if addspeed <= 0 {
		return
	}

	accelspeed := float32(svAccelerate) * facade.GetFrameTime() * wishspeed
	if accelspeed > addspeed {
		accelspeed = addspeed
	}

	vel := ctx.player.Velocity(sh)
	vel[0] += accelspeed * wishdir[0]
	vel[1] += accelspeed * wishdir[1]
	vel[2] += accelspeed * wishdir[2]
	ctx.player.SetVelocity(sh, vel)
	ctx.velocity = vel
}

// airAccelerate applies wish-direction acceleration while airborne.
func (m *ClientMover) airAccelerate(wishspeed float32, wishvel [3]float32, ctx *clientMoveContext) {
	facade := m.cfg
	sh := m.sh
	wishspd := srvtypes.VecNormalize(&wishvel)
	if wishspd > 30 {
		wishspd = 30
	}

	currentSpeed := srvtypes.VecDot(ctx.velocity, wishvel)
	addspeed := wishspd - currentSpeed
	if addspeed <= 0 {
		return
	}

	accelspeed := float32(svAccelerate) * wishspeed * facade.GetFrameTime()
	if accelspeed > addspeed {
		accelspeed = addspeed
	}

	vel := ctx.player.Velocity(sh)
	vel[0] += accelspeed * wishvel[0]
	vel[1] += accelspeed * wishvel[1]
	vel[2] += accelspeed * wishvel[2]
	ctx.player.SetVelocity(sh, vel)
	ctx.velocity = vel
}

// dropPunchAngle decays the punch angle each frame.
func (m *ClientMover) dropPunchAngle(ent *srvtypes.Edict) {
	sh := m.sh
	punch := ent.PunchAngle(sh)
	length := srvtypes.VecNormalize(&punch)
	length -= 10 * m.cfg.GetFrameTime()
	if length < 0 {
		length = 0
	}
	ent.SetPunchAngle(sh, srvtypes.VecScale(punch, length))
}

// waterMove applies drag and acceleration while swimming.
func (m *ClientMover) waterMove(ctx *clientMoveContext) {
	facade := m.cfg
	sh := m.sh
	srvtypes.AngleVectors(ctx.player.VAngle(sh), &ctx.forward, &ctx.right, &ctx.up)

	var wishvel [3]float32
	for i := 0; i < 3; i++ {
		wishvel[i] = ctx.forward[i]*ctx.cmd.ForwardMove + ctx.right[i]*ctx.cmd.SideMove
	}

	if ctx.cmd.ForwardMove == 0 && ctx.cmd.SideMove == 0 && ctx.cmd.UpMove == 0 {
		wishvel[2] -= 60
	} else {
		wishvel[2] += ctx.cmd.UpMove
	}

	wishspeed := srvtypes.VecLen(wishvel)
	if wishspeed > svMaxSpeed {
		wishvel = srvtypes.VecScale(wishvel, svMaxSpeed/wishspeed)
		wishspeed = svMaxSpeed
	}
	wishspeed *= 0.7

	speed := srvtypes.VecLen(ctx.velocity)
	newspeed := float32(0)
	if speed != 0 {
		newspeed = speed - facade.GetFrameTime()*speed*facade.GetFriction()
		if newspeed < 0 {
			newspeed = 0
		}
		vel := srvtypes.VecScale(ctx.player.Velocity(sh), newspeed/speed)
		ctx.player.SetVelocity(sh, vel)
		ctx.velocity = vel
	}

	if wishspeed == 0 {
		return
	}

	addspeed := wishspeed - newspeed
	if addspeed <= 0 {
		return
	}

	srvtypes.VecNormalize(&wishvel)
	accelspeed := float32(svAccelerate) * wishspeed * facade.GetFrameTime()
	if accelspeed > addspeed {
		accelspeed = addspeed
	}

	vel := ctx.player.Velocity(sh)
	vel[0] += accelspeed * wishvel[0]
	vel[1] += accelspeed * wishvel[1]
	vel[2] += accelspeed * wishvel[2]
	ctx.player.SetVelocity(sh, vel)
	ctx.velocity = vel
}

// waterJump handles the water-jump directional impulse.
func (m *ClientMover) waterJump(ent *srvtypes.Edict) {
	sh := m.sh
	if m.cfg.GetTime() > ent.TeleportTime(sh) || ent.WaterLevel(sh) <= 0 {
		ent.SetFlags(sh, float32(uint32(ent.Flags(sh))&^uint32(srvtypes.FlagWaterJump)))
		ent.SetTeleportTime(sh, 0)
	}

	moveDir := ent.MoveDir(sh)
	vel := ent.Velocity(sh)
	vel[0] = moveDir[0]
	vel[1] = moveDir[1]
	ent.SetVelocity(sh, vel)
}

// noclipMove sets velocity directly from input in noclip mode.
func (m *ClientMover) noclipMove(ctx *clientMoveContext) {
	facade := m.cfg
	sh := m.sh
	viewAngles := ctx.player.VAngle(sh)
	// Ironwail parity: sv_altnoclip 0 keeps noclip movement horizontal by
	// ignoring pitch for forward/strafe vectors.
	if cv := facade.Get("sv_altnoclip"); cv != nil && !cv.Bool() {
		viewAngles[0] = 0
	}
	srvtypes.AngleVectors(viewAngles, &ctx.forward, &ctx.right, &ctx.up)

	vel := ctx.player.Velocity(sh)
	vel[0] = ctx.forward[0]*ctx.cmd.ForwardMove + ctx.right[0]*ctx.cmd.SideMove
	vel[1] = ctx.forward[1]*ctx.cmd.ForwardMove + ctx.right[1]*ctx.cmd.SideMove
	vel[2] = ctx.forward[2]*ctx.cmd.ForwardMove + ctx.right[2]*ctx.cmd.SideMove
	vel[2] += ctx.cmd.UpMove * 2

	if srvtypes.VecLen(vel) > svMaxSpeed {
		srvtypes.VecNormalize(&vel)
		vel = srvtypes.VecScale(vel, svMaxSpeed)
	}
	ctx.player.SetVelocity(sh, vel)
	ctx.velocity = vel
}

// airMove applies air control and gravity-free acceleration.
func (m *ClientMover) airMove(ctx *clientMoveContext) {
	facade := m.cfg
	sh := m.sh
	srvtypes.AngleVectors(ctx.player.Angles(sh), &ctx.forward, &ctx.right, &ctx.up)

	fmove := ctx.cmd.ForwardMove
	smove := ctx.cmd.SideMove

	if facade.GetTime() < ctx.player.TeleportTime(sh) && fmove < 0 {
		fmove = 0
	}

	var wishvel [3]float32
	for i := 0; i < 3; i++ {
		wishvel[i] = ctx.forward[i]*fmove + ctx.right[i]*smove
	}

	if srvtypes.MoveType(ctx.player.MoveType(sh)) != srvtypes.MoveTypeWalk {
		wishvel[2] = ctx.cmd.UpMove
	} else {
		wishvel[2] = 0
	}

	wishdir := wishvel
	wishspeed := srvtypes.VecNormalize(&wishdir)
	if wishspeed > svMaxSpeed {
		wishvel = srvtypes.VecScale(wishvel, svMaxSpeed/wishspeed)
		wishspeed = svMaxSpeed
	}

	if srvtypes.MoveType(ctx.player.MoveType(sh)) == srvtypes.MoveTypeNoClip {
		ctx.player.SetVelocity(sh, wishvel)
		ctx.velocity = wishvel
		return
	}

	if ctx.onground {
		m.userFriction(ctx)
		m.accelerate(wishspeed, wishdir, ctx)
		return
	}

	m.airAccelerate(wishspeed, wishvel, ctx)
}

// CalcRoll computes the view roll from lateral velocity, matching C Ironwail
// V_CalcRoll (cl_rollangle / cl_rollspeed).
func CalcRoll(cvr srvtypes.CVarReader, angles, velocity [3]float32) float32 {
	var forward, right, up [3]float32
	srvtypes.AngleVectors(angles, &forward, &right, &up)

	side := srvtypes.VecDot(velocity, right)
	sign := float32(1)
	if side < 0 {
		sign = -1
		side = -side
	}

	// Use cl_rollangle and cl_rollspeed cvars, matching C Ironwail V_CalcRoll
	rollAngle := float32(2.0)
	rollSpeed := float32(200.0)
	if cvr != nil {
		if c := cvr.Get("cl_rollangle"); c != nil {
			rollAngle = float32(c.Float32())
		}
		if c := cvr.Get("cl_rollspeed"); c != nil {
			rollSpeed = float32(c.Float32())
		}
	}

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
