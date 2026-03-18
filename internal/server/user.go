package server

import (
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"

	"github.com/ironwail/ironwail-go/internal/cvar"
	inet "github.com/ironwail/ironwail-go/internal/net"
)

const (
	maxForwardSamples = 6
	idealPitchScale   = 0.8
	edgeFriction      = 2.0
	svMaxSpeed        = 320.0
	svAccelerate      = 10.0
)

var svAllowedUserCommands = []string{
	"status",
	"god",
	"notarget",
	"fly",
	"name",
	"noclip",
	"setpos",
	"say",
	"say_team",
	"tell",
	"color",
	"kill",
	"pause",
	"spawn",
	"begin",
	"prespawn",
	"kick",
	"ping",
	"give",
	"ban",
}

type clientMoveContext struct {
	player   *Edict
	origin   [3]float32
	velocity [3]float32
	cmd      UserCmd
	onground bool
	forward  [3]float32
	right    [3]float32
	up       [3]float32
}

func (s *Server) SetIdealPitch(ent *Edict) {
	if ent == nil || uint32(ent.Vars.Flags)&FlagOnGround == 0 {
		return
	}

	angle := float64(ent.Vars.Angles[1]) * math.Pi * 2 / 360
	sinVal := float32(math.Sin(angle))
	cosVal := float32(math.Cos(angle))

	var z [maxForwardSamples]float32
	for i := 0; i < maxForwardSamples; i++ {
		top := [3]float32{
			ent.Vars.Origin[0] + cosVal*float32(i+3)*12,
			ent.Vars.Origin[1] + sinVal*float32(i+3)*12,
			ent.Vars.Origin[2] + ent.Vars.ViewOfs[2],
		}
		bottom := [3]float32{top[0], top[1], top[2] - 160}

		tr := s.Move(top, [3]float32{}, [3]float32{}, bottom, MoveTypeNone, ent)
		if tr.AllSolid || tr.Fraction == 1 {
			return
		}

		z[i] = top[2] + tr.Fraction*(bottom[2]-top[2])
	}

	var dir float32
	steps := 0
	for i := 1; i < maxForwardSamples; i++ {
		step := z[i] - z[i-1]
		if step > -OneEpsilon && step < OneEpsilon {
			continue
		}

		if dir != 0 && (step-dir > OneEpsilon || step-dir < -OneEpsilon) {
			return
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

	ent.Vars.IdealPitch = -dir * idealPitchScale
}

func (s *Server) userFriction(ctx *clientMoveContext) {
	vel := ctx.velocity
	speed := float32(math.Sqrt(float64(vel[0]*vel[0] + vel[1]*vel[1])))
	if speed == 0 {
		return
	}

	start := [3]float32{
		ctx.origin[0] + vel[0]/speed*16,
		ctx.origin[1] + vel[1]/speed*16,
		ctx.origin[2] + ctx.player.Vars.Mins[2],
	}
	stop := [3]float32{start[0], start[1], start[2] - 34}

	trace := s.Move(start, [3]float32{}, [3]float32{}, stop, MoveTypeNone, ctx.player)

	friction := s.Friction
	if trace.Fraction == 1.0 {
		friction *= edgeFriction
	}

	control := speed
	if control < s.StopSpeed {
		control = s.StopSpeed
	}

	newspeed := speed - s.FrameTime*control*friction
	if newspeed < 0 {
		newspeed = 0
	}
	newspeed /= speed

	ctx.player.Vars.Velocity[0] *= newspeed
	ctx.player.Vars.Velocity[1] *= newspeed
	ctx.player.Vars.Velocity[2] *= newspeed
	ctx.velocity = ctx.player.Vars.Velocity
}

func (s *Server) accelerate(wishspeed float32, wishdir [3]float32, ctx *clientMoveContext) {
	currentSpeed := VecDot(ctx.player.Vars.Velocity, wishdir)
	addspeed := wishspeed - currentSpeed
	if addspeed <= 0 {
		return
	}

	accelspeed := float32(svAccelerate) * s.FrameTime * wishspeed
	if accelspeed > addspeed {
		accelspeed = addspeed
	}

	ctx.player.Vars.Velocity[0] += accelspeed * wishdir[0]
	ctx.player.Vars.Velocity[1] += accelspeed * wishdir[1]
	ctx.player.Vars.Velocity[2] += accelspeed * wishdir[2]
	ctx.velocity = ctx.player.Vars.Velocity
}

func (s *Server) airAccelerate(wishspeed float32, wishvel [3]float32, ctx *clientMoveContext) {
	wishspd := VecNormalize(&wishvel)
	if wishspd > 30 {
		wishspd = 30
	}

	currentSpeed := VecDot(ctx.velocity, wishvel)
	addspeed := wishspd - currentSpeed
	if addspeed <= 0 {
		return
	}

	accelspeed := float32(svAccelerate) * wishspeed * s.FrameTime
	if accelspeed > addspeed {
		accelspeed = addspeed
	}

	ctx.player.Vars.Velocity[0] += accelspeed * wishvel[0]
	ctx.player.Vars.Velocity[1] += accelspeed * wishvel[1]
	ctx.player.Vars.Velocity[2] += accelspeed * wishvel[2]
	ctx.velocity = ctx.player.Vars.Velocity
}

func (s *Server) dropPunchAngle(ent *Edict) {
	len := VecNormalize(&ent.Vars.PunchAngle)
	len -= 10 * s.FrameTime
	if len < 0 {
		len = 0
	}
	ent.Vars.PunchAngle = VecScale(ent.Vars.PunchAngle, len)
}

func (s *Server) waterMove(ctx *clientMoveContext) {
	AngleVectors(ctx.player.Vars.VAngle, &ctx.forward, &ctx.right, &ctx.up)

	var wishvel [3]float32
	for i := 0; i < 3; i++ {
		wishvel[i] = ctx.forward[i]*ctx.cmd.ForwardMove + ctx.right[i]*ctx.cmd.SideMove
	}

	if ctx.cmd.ForwardMove == 0 && ctx.cmd.SideMove == 0 && ctx.cmd.UpMove == 0 {
		wishvel[2] -= 60
	} else {
		wishvel[2] += ctx.cmd.UpMove
	}

	wishspeed := VecLen(wishvel)
	if wishspeed > svMaxSpeed {
		wishvel = VecScale(wishvel, svMaxSpeed/wishspeed)
		wishspeed = svMaxSpeed
	}
	wishspeed *= 0.7

	speed := VecLen(ctx.velocity)
	newspeed := float32(0)
	if speed != 0 {
		newspeed = speed - s.FrameTime*speed*s.Friction
		if newspeed < 0 {
			newspeed = 0
		}
		ctx.player.Vars.Velocity = VecScale(ctx.player.Vars.Velocity, newspeed/speed)
	}

	if wishspeed == 0 {
		return
	}

	addspeed := wishspeed - newspeed
	if addspeed <= 0 {
		return
	}

	VecNormalize(&wishvel)
	accelspeed := float32(svAccelerate) * wishspeed * s.FrameTime
	if accelspeed > addspeed {
		accelspeed = addspeed
	}

	ctx.player.Vars.Velocity[0] += accelspeed * wishvel[0]
	ctx.player.Vars.Velocity[1] += accelspeed * wishvel[1]
	ctx.player.Vars.Velocity[2] += accelspeed * wishvel[2]
}

func (s *Server) waterJump(ent *Edict) {
	if s.Time > ent.Vars.TeleportTime || ent.Vars.WaterLevel <= 0 {
		ent.Vars.Flags = float32(uint32(ent.Vars.Flags) & ^uint32(FlagWaterJump))
		ent.Vars.TeleportTime = 0
	}

	ent.Vars.Velocity[0] = ent.Vars.MoveDir[0]
	ent.Vars.Velocity[1] = ent.Vars.MoveDir[1]
}

func (s *Server) noclipMove(ctx *clientMoveContext) {
	viewAngles := ctx.player.Vars.VAngle
	// Ironwail parity: sv_altnoclip 0 keeps noclip movement horizontal by
	// ignoring pitch for forward/strafe vectors.
	if cv := cvar.Get("sv_altnoclip"); cv != nil && !cv.Bool() {
		viewAngles[0] = 0
	}
	AngleVectors(viewAngles, &ctx.forward, &ctx.right, &ctx.up)

	ctx.player.Vars.Velocity[0] = ctx.forward[0]*ctx.cmd.ForwardMove + ctx.right[0]*ctx.cmd.SideMove
	ctx.player.Vars.Velocity[1] = ctx.forward[1]*ctx.cmd.ForwardMove + ctx.right[1]*ctx.cmd.SideMove
	ctx.player.Vars.Velocity[2] = ctx.forward[2]*ctx.cmd.ForwardMove + ctx.right[2]*ctx.cmd.SideMove
	ctx.player.Vars.Velocity[2] += ctx.cmd.UpMove * 2

	if VecLen(ctx.player.Vars.Velocity) > svMaxSpeed {
		VecNormalize(&ctx.player.Vars.Velocity)
		ctx.player.Vars.Velocity = VecScale(ctx.player.Vars.Velocity, svMaxSpeed)
	}
}

func (s *Server) airMove(ctx *clientMoveContext) {
	AngleVectors(ctx.player.Vars.Angles, &ctx.forward, &ctx.right, &ctx.up)

	fmove := ctx.cmd.ForwardMove
	smove := ctx.cmd.SideMove

	if s.Time < ctx.player.Vars.TeleportTime && fmove < 0 {
		fmove = 0
	}

	var wishvel [3]float32
	for i := 0; i < 3; i++ {
		wishvel[i] = ctx.forward[i]*fmove + ctx.right[i]*smove
	}

	if MoveType(ctx.player.Vars.MoveType) != MoveTypeWalk {
		wishvel[2] = ctx.cmd.UpMove
	} else {
		wishvel[2] = 0
	}

	wishdir := wishvel
	wishspeed := VecNormalize(&wishdir)
	if wishspeed > svMaxSpeed {
		wishvel = VecScale(wishvel, svMaxSpeed/wishspeed)
		wishspeed = svMaxSpeed
	}

	if MoveType(ctx.player.Vars.MoveType) == MoveTypeNoClip {
		ctx.player.Vars.Velocity = wishvel
		return
	}

	if ctx.onground {
		s.userFriction(ctx)
		s.accelerate(wishspeed, wishdir, ctx)
		return
	}

	s.airAccelerate(wishspeed, wishvel, ctx)
}

func CalcRoll(angles, velocity [3]float32) float32 {
	var forward, right, up [3]float32
	AngleVectors(angles, &forward, &right, &up)

	side := VecDot(velocity, right)
	sign := float32(1)
	if side < 0 {
		sign = -1
		side = -side
	}

	// Use cl_rollangle and cl_rollspeed cvars, matching C Ironwail V_CalcRoll
	rollAngle := float32(2.0)
	rollSpeed := float32(200.0)
	if cv := cvar.Get("cl_rollangle"); cv != nil {
		rollAngle = float32(cv.Float)
	}
	if cv := cvar.Get("cl_rollspeed"); cv != nil {
		rollSpeed = float32(cv.Float)
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

func (s *Server) SV_ClientThink(client *Client) {
	if client == nil || client.Edict == nil || client.Edict.Free {
		return
	}

	ent := client.Edict
	if MoveType(ent.Vars.MoveType) == MoveTypeNone {
		return
	}

	ctx := &clientMoveContext{
		player:   ent,
		origin:   ent.Vars.Origin,
		velocity: ent.Vars.Velocity,
		cmd:      client.LastCmd,
		onground: uint32(ent.Vars.Flags)&FlagOnGround != 0,
	}

	s.dropPunchAngle(ent)

	if ent.Vars.Health <= 0 {
		return
	}

	vAngle := VecAdd(ent.Vars.VAngle, ent.Vars.PunchAngle)
	ent.Vars.Angles[2] = CalcRoll(ent.Vars.Angles, ent.Vars.Velocity) * 4
	if ent.Vars.FixAngle == 0 {
		ent.Vars.Angles[0] = -vAngle[0] / 3
		ent.Vars.Angles[1] = vAngle[1]
	}

	if uint32(ent.Vars.Flags)&FlagWaterJump != 0 {
		s.waterJump(ent)
		return
	}

	if MoveType(ent.Vars.MoveType) == MoveTypeNoClip {
		s.noclipMove(ctx)
		return
	}
	if ent.Vars.WaterLevel >= 2 {
		s.waterMove(ctx)
		return
	}
	s.airMove(ctx)
}

func (s *Server) playerClient(ent *Edict) *Client {
	if s == nil || s.Static == nil || ent == nil {
		return nil
	}

	entNum := s.NumForEdict(ent)
	if entNum <= 0 || entNum > len(s.Static.Clients) {
		return nil
	}

	client := s.Static.Clients[entNum-1]
	if client == nil || !client.Active || !client.Spawned || client.Edict != ent {
		return nil
	}

	return client
}

func (s *Server) runClientQCThink(client *Client, funcName string) {
	if s == nil || s.QCVM == nil || client == nil || client.Edict == nil || client.Edict.Free {
		return
	}

	funcIdx := s.QCVM.FindFunction(funcName)
	if funcIdx < 0 {
		return
	}

	entNum := s.NumForEdict(client.Edict)
	if entNum <= 0 {
		return
	}

	s.syncQCVMState()
	syncEdictToQCVM(s.QCVM, entNum, client.Edict)
	s.QCVM.Time = float64(s.Time)
	s.QCVM.SetGlobal("time", s.Time)
	s.QCVM.SetGlobal("frametime", s.FrameTime)
	s.QCVM.SetGlobal("self", entNum)
	s.QCVM.SetGlobal("other", 0)
	s.QCVM.SetGlobal("msg_entity", entNum)
	if err := s.QCVM.ExecuteFunction(funcIdx); err != nil {
		slog.Warn("client think QC failed", "function", funcName, "entity", entNum, "error", err)
		return
	}
	syncEdictFromQCVM(s.QCVM, entNum, client.Edict)
}

func (s *Server) ClientThink(client *Client) {
	s.SV_ClientThink(client)
}

func (s *Server) ReadClientMove(client *Client, buf *MessageBuffer) UserCmd {
	var cmd UserCmd

	pingTime := buf.ReadFloat()
	client.PingTimes[client.NumPings%NumPingTimes] = s.Time - pingTime
	client.NumPings++

	for i := 0; i < 3; i++ {
		// NetQuake uses 8-bit angles, FitzQuake/RMQ use 16-bit
		if s.Protocol == ProtocolNetQuake {
			cmd.ViewAngles[i] = buf.ReadAngle(0)
		} else {
			cmd.ViewAngles[i] = buf.ReadAngle16()
		}
	}

	if client.Edict != nil {
		client.Edict.Vars.VAngle = cmd.ViewAngles
	}

	cmd.ForwardMove = float32(buf.ReadShort())
	cmd.SideMove = float32(buf.ReadShort())
	cmd.UpMove = float32(buf.ReadShort())

	bits := buf.ReadByte()
	cmd.Buttons = bits
	if client.Edict != nil {
		client.Edict.Vars.Button0 = float32(bits & 1)
		client.Edict.Vars.Button2 = float32((bits & 2) >> 1)
	}

	impulse := buf.ReadByte()
	cmd.Impulse = impulse
	if impulse != 0 && client.Edict != nil {
		client.Edict.Vars.Impulse = float32(impulse)
	}

	return cmd
}

func (s *Server) SV_ExecuteUserCommand(client *Client, cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return true
	}

	lower := strings.ToLower(cmd)
	args := strings.Fields(cmd)
	if len(args) == 0 {
		return true
	}
	verb := strings.ToLower(args[0])

	switch verb {
	case "say":
		if len(args) < 2 {
			return true
		}
		msg := strings.Join(args[1:], " ")
		s.SV_BroadcastPrintf("%s: %s\n", client.Name, msg)
		return true
	case "say_team":
		if len(args) < 2 {
			return true
		}
		msg := strings.Join(args[1:], " ")
		if s.Static == nil {
			return true
		}
		for _, c := range s.Static.Clients {
			if c == nil || !c.Active || c.Edict == nil {
				continue
			}
			if c.Edict.Vars.Team == client.Edict.Vars.Team {
				s.SV_ClientPrintf(c, "(team) %s: %s\n", client.Name, msg)
			}
		}
		return true
	case "tell":
		if len(args) < 3 {
			return true
		}
		targetName := args[1]
		msg := strings.Join(args[2:], " ")
		if s.Static == nil {
			return true
		}
		for _, c := range s.Static.Clients {
			if c == nil || !c.Active {
				continue
			}
			if strings.EqualFold(c.Name, targetName) {
				s.SV_ClientPrintf(c, "%s tells you: %s\n", client.Name, msg)
				s.SV_ClientPrintf(client, "you tell %s: %s\n", c.Name, msg)
				return true
			}
		}
		s.SV_ClientPrintf(client, "player %s not found\n", targetName)
		return true
	case "name":
		if len(args) < 2 {
			s.SV_ClientPrintf(client, "name is %s\n", client.Name)
			return true
		}
		newName := args[1]
		if len(newName) > 15 {
			newName = newName[:15]
		}
		s.SV_BroadcastPrintf("%s changed name to %s\n", client.Name, newName)
		client.Name = newName
		if client.Edict != nil && s.QCVM != nil {
			client.Edict.Vars.NetName = s.QCVM.AllocString(client.Name)
		}
		return true
	case "color":
		if len(args) < 2 {
			s.SV_ClientPrintf(client, "color is %d\n", client.Color)
			return true
		}
		color, _ := strconv.Atoi(args[1])
		client.Color = color
		if client.Edict != nil {
			client.Edict.Vars.Team = float32(color + 1)
		}
		return true
	}

	for _, allowed := range svAllowedUserCommands {
		if strings.HasPrefix(lower, allowed) {
			return true
		}
	}

	return false
}

func (s *Server) ExecuteClientString(client *Client, cmd string) bool {
	return s.SV_ExecuteUserCommand(client, cmd)
}

func (s *Server) findLocalSpawnPoint() *Edict {
	for _, className := range []string{"info_player_start", "testplayerstart"} {
		for entNum := 1; entNum < s.NumEdicts; entNum++ {
			ent := s.Edicts[entNum]
			if ent == nil || ent.Free || ent.Vars == nil {
				continue
			}
			if s.GetString(ent.Vars.ClassName) == className {
				return ent
			}
		}
	}
	return nil
}

func (s *Server) runClientSpawnQC(client *Client) error {
	if client == nil || client.Edict == nil {
		return fmt.Errorf("client edict missing")
	}

	entNum := s.NumForEdict(client.Edict)
	if entNum <= 0 {
		return fmt.Errorf("invalid client edict %d", entNum)
	}

	ent := client.Edict
	ent.Free = false
	ent.Vars.Colormap = float32(entNum)
	ent.Vars.Team = float32(client.Color + 1)
	ent.Vars.Health = 100
	ent.Vars.TakeDamage = 1
	ent.Vars.MoveType = float32(MoveTypeWalk)
	ent.Vars.Solid = float32(SolidSlideBox)
	ent.Vars.ViewOfs = [3]float32{0, 0, ViewHeight}
	ent.Vars.Mins = [3]float32{-16, -16, -24}
	ent.Vars.Maxs = [3]float32{16, 16, 32}
	ent.Vars.Size = [3]float32{32, 32, 56}
	ent.Vars.Velocity = [3]float32{}
	ent.Vars.AVelocity = [3]float32{}
	ent.Vars.FixAngle = 1

	if spawn := s.findLocalSpawnPoint(); spawn != nil && spawn.Vars != nil {
		ent.Vars.Origin = spawn.Vars.Origin
		ent.Vars.Angles = spawn.Vars.Angles
		ent.Vars.VAngle = spawn.Vars.Angles
	}
	ent.Vars.AbsMin = [3]float32{ent.Vars.Origin[0] + ent.Vars.Mins[0], ent.Vars.Origin[1] + ent.Vars.Mins[1], ent.Vars.Origin[2] + ent.Vars.Mins[2]}
	ent.Vars.AbsMax = [3]float32{ent.Vars.Origin[0] + ent.Vars.Maxs[0], ent.Vars.Origin[1] + ent.Vars.Maxs[1], ent.Vars.Origin[2] + ent.Vars.Maxs[2]}

	if client.Name == "" {
		client.Name = "player"
	}
	if s.QCVM != nil {
		ent.Vars.ClassName = s.QCVM.AllocString("player")
		ent.Vars.NetName = s.QCVM.AllocString(client.Name)
		if playerModel := s.FindModel("progs/player.mdl"); playerModel != 0 {
			ent.Vars.ModelIndex = float32(playerModel)
			ent.Vars.Model = s.QCVM.AllocString("progs/player.mdl")
		}
	}

	s.LinkEdict(ent, true)
	return nil
}

func (s *Server) runClientPutInServerQC(client *Client) error {
	if client == nil || client.Edict == nil {
		return fmt.Errorf("client edict missing")
	}
	if s.QCVM == nil {
		return nil // No VM, skip execution
	}

	funcNum := s.QCVM.FindFunction("PutClientInServer")
	if funcNum < 0 {
		return nil // Function not in progs, skip
	}

	entNum := s.NumForEdict(client.Edict)
	if entNum <= 0 {
		return fmt.Errorf("invalid client edict %d", entNum)
	}

	// Sync QCVM state and prepare for function call
	s.syncQCVMState()
	syncEdictToQCVM(s.QCVM, entNum, client.Edict)

	// Set up global variables for PutClientInServer
	s.QCVM.Time = float64(s.Time)
	s.QCVM.SetGlobal("time", s.Time)
	s.QCVM.SetGlobal("frametime", s.FrameTime)
	s.QCVM.SetGlobal("self", entNum)
	s.QCVM.SetGlobal("other", 0)
	s.QCVM.SetGlobal("msg_entity", entNum)
	for i := 0; i < len(client.SpawnParms); i++ {
		s.QCVM.SetGlobal(fmt.Sprintf("parm%d", i+1), client.SpawnParms[i])
	}

	// Execute PutClientInServer
	if err := s.QCVM.ExecuteFunction(funcNum); err != nil {
		return fmt.Errorf("PutClientInServer execution failed: %w", err)
	}

	// Sync changes back from QCVM to edict
	syncEdictFromQCVM(s.QCVM, entNum, client.Edict)

	return nil
}

func (s *Server) SubmitLoopbackStringCommand(clientNum int, cmd string) error {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return fmt.Errorf("invalid client number %d", clientNum)
	}
	client := s.Static.Clients[clientNum]
	if client == nil {
		return fmt.Errorf("client %d is nil", clientNum)
	}
	client.Loopback = true
	if !s.SV_ExecuteUserCommand(client, cmd) {
		return fmt.Errorf("command %q rejected", cmd)
	}
	if client.Message == nil {
		client.Message = NewMessageBuffer(MaxDatagram)
	}

	switch strings.ToLower(strings.TrimSpace(cmd)) {
	case "prespawn":
		if client.SendSignon != SignonFlush {
			return fmt.Errorf("prespawn out of order")
		}
		// Send accumulated signon buffers (static entities, ambient sounds)
		// to this client before advancing the signon stage.
		s.SendSignonBuffers(client)
		client.Message.WriteByte(byte(inet.SVCSignOnNum))
		client.Message.WriteByte(2)
		client.SendSignon = SignonSignonBufs
	case "spawn":
		if client.SendSignon != SignonSignonBufs {
			return fmt.Errorf("spawn out of order")
		}
		if !s.LoadGame {
			if err := s.runClientSpawnQC(client); err != nil {
				return err
			}
		}
		for style, value := range s.LightStyles {
			client.Message.WriteByte(byte(inet.SVCLightStyle))
			client.Message.WriteByte(byte(style))
			client.Message.WriteString(value)
		}
		client.Message.WriteByte(byte(inet.SVCSignOnNum))
		client.Message.WriteByte(3)
		client.SendSignon = SignonSignonMsg
	case "begin":
		if client.SendSignon != SignonSignonMsg {
			return fmt.Errorf("begin out of order")
		}
		client.Message.WriteByte(byte(inet.SVCSignOnNum))
		client.Message.WriteByte(4)
		client.Spawned = true
		client.SendSignon = SignonDone
		// Execute PutClientInServer to finalize player initialization via QC
		if !s.LoadGame {
			if err := s.runClientPutInServerQC(client); err != nil {
				return err
			}
		}
	default:
		// Other allowed commands have no special loopback response yet.
	}

	return nil
}

func (s *Server) SubmitLoopbackCmd(clientNum int, viewAngles [3]float32, forward, side, up float32, buttons, impulse int, sentTime float64) error {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return fmt.Errorf("invalid client number %d", clientNum)
	}
	client := s.Static.Clients[clientNum]
	if client == nil {
		return fmt.Errorf("client %d is nil", clientNum)
	}
	client.Loopback = true

	client.LastCmd = UserCmd{
		ViewAngles:  viewAngles,
		ForwardMove: forward,
		SideMove:    side,
		UpMove:      up,
		Buttons:     uint8(buttons),
		Impulse:     uint8(impulse),
	}
	client.LoopbackCmdPending = true
	client.PingTimes[client.NumPings%NumPingTimes] = s.Time - float32(sentTime)
	client.NumPings++

	if client.Edict != nil {
		client.Edict.Vars.VAngle = viewAngles
		client.Edict.Vars.Button0 = float32(uint8(buttons) & 1)
		client.Edict.Vars.Button2 = float32((uint8(buttons) & 2) >> 1)
		if impulse != 0 {
			client.Edict.Vars.Impulse = float32(uint8(impulse))
		}
	}

	return nil
}

func (s *Server) SV_ReadClientMessage(client *Client, buf *MessageBuffer) bool {
	for {
		ccmd := int(buf.ReadChar())
		if buf.BadRead {
			return false
		}

		switch ccmd {
		case -1:
			return true
		case int(CLCNop):
			continue
		case int(CLCStringCmd):
			cmd := buf.ReadString()
			if !s.SV_ExecuteUserCommand(client, cmd) {
				return false
			}
		case int(CLCDisconnect):
			return false
		case int(CLCMove):
			client.LastCmd = s.ReadClientMove(client, buf)
		default:
			return false
		}

		if !client.Active {
			return false
		}
	}
}

func (s *Server) ReadClientMessage(client *Client, buf *MessageBuffer) bool {
	return s.SV_ReadClientMessage(client, buf)
}

func (s *Server) RunClients() {
	if s.Static == nil {
		return
	}

	for _, client := range s.Static.Clients {
		if client == nil || !client.Active {
			continue
		}

		if client.Loopback {
			if !client.LoopbackCmdPending {
				client.LastCmd = UserCmd{}
			}
			client.LoopbackCmdPending = false
		} else {
			if client.Message == nil || !s.SV_ReadClientMessage(client, client.Message) {
				s.DropClient(client, false)
				continue
			}
		}

		if !client.Spawned {
			client.LastCmd = UserCmd{}
			continue
		}
		if s.handleDeathmatchRespawn(client) {
			client.LastCmd = UserCmd{}
			continue
		}

		if !s.Paused {
			s.SV_ClientThink(client)
		}
	}
}

func (s *Server) DropClient(client *Client, crash bool) {
	_ = crash
	if client == nil || !client.Active {
		return
	}

	if client.Edict != nil && s.QCVM != nil {
		funcIdx := s.QCVM.FindFunction("ClientDisconnect")
		if funcIdx >= 0 {
			s.QCVM.Time = float64(s.Time)
			s.QCVM.SetGlobal("self", s.NumForEdict(client.Edict))
			s.QCVM.ExecuteFunction(funcIdx)
		}
	}

	client.Active = false
	client.Spawned = false
	client.RespawnTime = 0
	if client.Edict != nil {
		client.Edict.Free = true
		client.Edict.FreeTime = s.Time
	}
}

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

	right[0] = -sr*sp*cy + -cr*-sy
	right[1] = -sr*sp*sy + -cr*cy
	right[2] = -sr * cp

	up[0] = cr*sp*cy + -sr*-sy
	up[1] = cr*sp*sy + -sr*cy
	up[2] = cr * cp
}
