package server

import (
	"bytes"
	"math"
	"path/filepath"
	"testing"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/model"
	"github.com/ironwail/ironwail-go/internal/qc"
	"github.com/ironwail/ironwail-go/internal/testutil"
)

func finalizeMessage(m *MessageBuffer) *MessageBuffer {
	m.Data = m.Data[:m.Len()]
	return m
}

func TestSVExecuteUserCommandWhitelist(t *testing.T) {
	s := NewServer()
	client := &Client{}

	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		{name: "status", cmd: "status", want: true},
		{name: "ban", cmd: "ban 1", want: true},
		{name: "spawn", cmd: "spawn", want: true},
		{name: "prefix-match-parity", cmd: "godmode", want: true},
		{name: "unknown", cmd: "foobar", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := s.SV_ExecuteUserCommand(client, tc.cmd)
			if got != tc.want {
				t.Fatalf("SV_ExecuteUserCommand(%q) = %v, want %v", tc.cmd, got, tc.want)
			}
		})
	}
}

func TestSVReadClientMessageMoveCommand(t *testing.T) {
	s := NewServer()
	s.Time = 4.0

	ent := &Edict{Vars: &EntVars{}}
	client := &Client{Active: true, Edict: ent}

	msg := NewMessageBuffer(128)
	msg.WriteChar(int8(CLCMove))
	msg.WriteFloat(1.25)
	msg.WriteShort(16384)
	msg.WriteShort(0)
	msg.WriteShort(-16384)
	msg.WriteShort(120)
	msg.WriteShort(-40)
	msg.WriteShort(18)
	msg.WriteByte(3)
	msg.WriteByte(7)
	msg.WriteChar(-1)
	msg = finalizeMessage(msg)

	if ok := s.SV_ReadClientMessage(client, msg); !ok {
		t.Fatalf("SV_ReadClientMessage returned false")
	}

	if client.LastCmd.ForwardMove != 120 || client.LastCmd.SideMove != -40 || client.LastCmd.UpMove != 18 {
		t.Fatalf("unexpected movement command: %+v", client.LastCmd)
	}
	if client.LastCmd.Buttons != 3 || client.LastCmd.Impulse != 7 {
		t.Fatalf("unexpected buttons/impulse: buttons=%d impulse=%d", client.LastCmd.Buttons, client.LastCmd.Impulse)
	}
	if ent.Vars.Button0 != 1 || ent.Vars.Button2 != 1 || ent.Vars.Impulse != 7 {
		t.Fatalf("edict button/impulse state not updated: b0=%v b2=%v impulse=%v", ent.Vars.Button0, ent.Vars.Button2, ent.Vars.Impulse)
	}
	if client.NumPings != 1 {
		t.Fatalf("num pings = %d, want 1", client.NumPings)
	}
}

func TestSVClientThinkNoclip(t *testing.T) {
	s := NewServer()
	s.FrameTime = 0.1

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.MoveType = float32(MoveTypeNoClip)
	ent.Vars.Health = 100
	ent.Vars.VAngle = [3]float32{30, 0, 0}

	client := &Client{
		Edict: ent,
		LastCmd: UserCmd{
			ForwardMove: 100,
			SideMove:    50,
			UpMove:      20,
		},
	}

	s.SV_ClientThink(client)

	if ent.Vars.Angles[0] != -10 {
		t.Fatalf("pitch = %v, want -10", ent.Vars.Angles[0])
	}
	if ent.Vars.Velocity == [3]float32{} {
		t.Fatalf("noclip move did not update velocity")
	}
}

func TestSVClientThinkWalkForwardIgnoresPitchVerticalProjection(t *testing.T) {
	s := NewServer()
	s.FrameTime = 0.05

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.MoveType = float32(MoveTypeWalk)
	ent.Vars.Health = 100
	ent.Vars.Flags = float32(FlagOnGround)
	ent.Vars.VAngle = [3]float32{60, 0, 0}

	client := &Client{
		Edict: ent,
		LastCmd: UserCmd{
			ForwardMove: 200,
		},
	}

	s.SV_ClientThink(client)

	if ent.Vars.Velocity[2] != 0 {
		t.Fatalf("walk velocity z = %v, want 0", ent.Vars.Velocity[2])
	}
	if ent.Vars.Velocity[0] == 0 && ent.Vars.Velocity[1] == 0 {
		t.Fatalf("walk forward move did not produce horizontal velocity: %v", ent.Vars.Velocity)
	}
}

func TestSVClientThinkGroundFrictionFeedsAccelerate(t *testing.T) {
	s := NewServer()
	s.FrameTime = 0.1

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.MoveType = float32(MoveTypeWalk)
	ent.Vars.Health = 100
	ent.Vars.Flags = float32(FlagOnGround)
	ent.Vars.VAngle = [3]float32{0, 0, 0}
	ent.Vars.Velocity = [3]float32{100, 0, 0}

	client := &Client{
		Edict: ent,
		LastCmd: UserCmd{
			ForwardMove: 200,
		},
	}

	s.SV_ClientThink(client)

	if diff := math.Abs(float64(ent.Vars.Velocity[0] - 200)); diff > 0.001 {
		t.Fatalf("ground accelerate used stale pre-friction speed: got %.3f want 200", ent.Vars.Velocity[0])
	}
}

func findWalkablePointForUserTest(s *Server) ([3]float32, bool) {
	wm, ok := s.WorldModel.(*model.Model)
	if !ok || wm == nil {
		return [3]float32{}, false
	}

	for xi := 1; xi < 15; xi++ {
		x := wm.Mins[0] + (wm.Maxs[0]-wm.Mins[0])*(float32(xi)/16)
		for yi := 1; yi < 15; yi++ {
			y := wm.Mins[1] + (wm.Maxs[1]-wm.Mins[1])*(float32(yi)/16)
			start := [3]float32{x, y, wm.Maxs[2] - 8}
			if s.PointContents(start) == bsp.ContentsSolid {
				continue
			}
			end := [3]float32{x, y, wm.Mins[2] - 256}
			trace := s.SV_Move(start, [3]float32{}, [3]float32{}, end, MoveType(MoveNoMonsters), nil)
			if trace.Fraction == 1 || trace.AllSolid {
				continue
			}
			pos := trace.EndPos
			pos[2] += 24
			if s.PointContents(pos) == bsp.ContentsEmpty {
				return pos, true
			}
		}
	}

	return [3]float32{}, false
}

func TestRunClientsProcessesMoveOnSpawnedMap(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	defer vfs.Close()

	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	if err := s.SpawnServer("start", vfs); err != nil {
		t.Fatalf("spawn server: %v", err)
	}

	s.ConnectClient(0)
	client := s.Static.Clients[0]
	client.Spawned = true

	pos, ok := findWalkablePointForUserTest(s)
	if !ok {
		t.Skip("no walkable point found on start map")
	}

	ent := client.Edict
	ent.Vars.Origin = pos
	ent.Vars.Mins = [3]float32{-16, -16, -24}
	ent.Vars.Maxs = [3]float32{16, 16, 32}
	ent.Vars.Solid = float32(SolidSlideBox)
	ent.Vars.MoveType = float32(MoveTypeWalk)
	ent.Vars.Health = 100
	ent.Vars.Flags = float32(FlagOnGround)
	s.LinkEdict(ent, false)

	msg := NewMessageBuffer(128)
	msg.WriteChar(int8(CLCMove))
	msg.WriteFloat(s.Time - 0.05)
	msg.WriteShort(0)
	msg.WriteShort(2048)
	msg.WriteShort(0)
	msg.WriteShort(100)
	msg.WriteShort(0)
	msg.WriteShort(0)
	msg.WriteByte(0)
	msg.WriteByte(0)
	msg.WriteChar(-1)
	client.Message = finalizeMessage(msg)

	s.RunClients()

	if !client.Active {
		t.Fatalf("client was dropped unexpectedly")
	}
	if client.LastCmd.ForwardMove != 100 {
		t.Fatalf("forwardmove = %v, want 100", client.LastCmd.ForwardMove)
	}
}

func TestLoopbackCmdMovesAuthoritativePlayerOrigin(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	defer vfs.Close()

	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	if err := s.SpawnServer("start", vfs); err != nil {
		t.Fatalf("spawn server: %v", err)
	}

	s.ConnectClient(0)
	client := s.Static.Clients[0]
	client.Spawned = true

	pos, ok := findWalkablePointForUserTest(s)
	if !ok {
		t.Skip("no walkable point found on start map")
	}

	ent := client.Edict
	ent.Vars.Origin = pos
	ent.Vars.Mins = [3]float32{-16, -16, -24}
	ent.Vars.Maxs = [3]float32{16, 16, 32}
	ent.Vars.Solid = float32(SolidSlideBox)
	ent.Vars.MoveType = float32(MoveTypeWalk)
	ent.Vars.Health = 100
	ent.Vars.Flags = float32(FlagOnGround)
	s.LinkEdict(ent, false)

	start := ent.Vars.Origin
	if err := s.SubmitLoopbackCmd(0, [3]float32{0, 0, 0}, 200, 0, 0, 0, 0, float64(s.Time)); err != nil {
		t.Fatalf("SubmitLoopbackCmd: %v", err)
	}
	if err := s.Frame(0.05); err != nil {
		t.Fatalf("Frame: %v", err)
	}

	end := ent.Vars.Origin
	if end == start {
		t.Fatalf("authoritative origin did not move: start=%v end=%v", start, end)
	}
	if dx, dy := end[0]-start[0], end[1]-start[1]; dx == 0 && dy == 0 {
		t.Fatalf("authoritative origin only changed vertically: start=%v end=%v", start, end)
	}
}

func TestLoopbackCmdWalkForwardWithPitchMovesHorizontally(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	defer vfs.Close()

	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	if err := s.SpawnServer("start", vfs); err != nil {
		t.Fatalf("spawn server: %v", err)
	}

	s.ConnectClient(0)
	client := s.Static.Clients[0]
	client.Spawned = true

	pos, ok := findWalkablePointForUserTest(s)
	if !ok {
		t.Skip("no walkable point found on start map")
	}

	ent := client.Edict
	ent.Vars.Origin = pos
	ent.Vars.Mins = [3]float32{-16, -16, -24}
	ent.Vars.Maxs = [3]float32{16, 16, 32}
	ent.Vars.Solid = float32(SolidSlideBox)
	ent.Vars.MoveType = float32(MoveTypeWalk)
	ent.Vars.Health = 100
	ent.Vars.Flags = float32(FlagOnGround)
	s.LinkEdict(ent, false)

	start := ent.Vars.Origin
	if err := s.SubmitLoopbackCmd(0, [3]float32{45, 0, 0}, 200, 0, 0, 0, 0, float64(s.Time)); err != nil {
		t.Fatalf("SubmitLoopbackCmd: %v", err)
	}
	if err := s.Frame(0.05); err != nil {
		t.Fatalf("Frame: %v", err)
	}

	end := ent.Vars.Origin
	if end == start {
		t.Fatalf("authoritative origin did not move: start=%v end=%v", start, end)
	}
	if dx, dy := end[0]-start[0], end[1]-start[1]; dx == 0 && dy == 0 {
		t.Fatalf("authoritative origin only changed vertically with pitched view: start=%v end=%v", start, end)
	}
}

func TestLoopbackJumpAppliesVerticalVelocity(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	defer vfs.Close()

	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	progsData, err := vfs.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("load progs.dat: %v", err)
	}
	if err := s.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(s.QCVM)

	if err := s.SpawnServer("start", vfs); err != nil {
		t.Fatalf("spawn server: %v", err)
	}

	s.ConnectClient(0)
	client := s.Static.Clients[0]
	for _, cmd := range []string{"prespawn", "spawn", "begin"} {
		if err := s.SubmitLoopbackStringCommand(0, cmd); err != nil {
			t.Fatalf("SubmitLoopbackStringCommand(%s): %v", cmd, err)
		}
	}
	if !client.Spawned {
		t.Fatal("client not marked spawned after signon")
	}

	pos, ok := findWalkablePointForUserTest(s)
	if !ok {
		t.Skip("no walkable point found on start map")
	}

	ent := client.Edict
	ent.Vars.Origin = pos
	ent.Vars.Velocity = [3]float32{}
	ent.Vars.Flags = float32(FlagOnGround | FlagJumpReleased)
	ent.Vars.GroundEntity = 1
	s.LinkEdict(ent, false)

	start := ent.Vars.Origin
	if err := s.SubmitLoopbackCmd(0, [3]float32{}, 0, 0, 0, 2, 0, float64(s.Time)); err != nil {
		t.Fatalf("SubmitLoopbackCmd: %v", err)
	}
	if err := s.Frame(0.05); err != nil {
		t.Fatalf("Frame: %v", err)
	}

	if ent.Vars.Velocity[2] <= 0 {
		t.Fatalf("jump did not apply upward velocity: velocity=%v", ent.Vars.Velocity)
	}
	if ent.Vars.Origin[2] <= start[2] {
		t.Fatalf("jump did not move player upward: start=%v end=%v", start, ent.Vars.Origin)
	}
	if uint32(ent.Vars.Flags)&FlagOnGround != 0 {
		t.Fatalf("jump left player grounded: flags=0x%x", uint32(ent.Vars.Flags))
	}
}

func TestPhysicsWalkClearsStaleGroundFlagWhenUnsupported(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	defer vfs.Close()

	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	if err := s.SpawnServer("start", vfs); err != nil {
		t.Fatalf("spawn server: %v", err)
	}

	pos, ok := findWalkablePointForUserTest(s)
	if !ok {
		t.Skip("no walkable point found on start map")
	}

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("AllocEdict returned nil")
	}

	pos[2] += 96
	ent.Vars.Origin = pos
	ent.Vars.Mins = [3]float32{-16, -16, -24}
	ent.Vars.Maxs = [3]float32{16, 16, 32}
	ent.Vars.Solid = float32(SolidSlideBox)
	ent.Vars.MoveType = float32(MoveTypeWalk)
	ent.Vars.Health = 100
	ent.Vars.Flags = float32(FlagOnGround)
	s.LinkEdict(ent, false)

	if s.CheckBottom(ent) {
		t.Skipf("lifted test position unexpectedly has support: origin=%v", ent.Vars.Origin)
	}

	start := ent.Vars.Origin
	s.FrameTime = 0.05
	s.PhysicsWalk(ent)

	if uint32(ent.Vars.Flags)&FlagOnGround != 0 {
		t.Fatalf("stale onground flag was not cleared: flags=0x%x", uint32(ent.Vars.Flags))
	}
	if ent.Vars.GroundEntity != 0 {
		t.Fatalf("ground entity = %v, want 0", ent.Vars.GroundEntity)
	}
	if ent.Vars.Origin[2] >= start[2] {
		t.Fatalf("entity did not fall after losing support: start=%v end=%v", start, ent.Vars.Origin)
	}
}
