package server

import (
	"bytes"
	"math"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/fs"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/testutil"
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

	ent := allocPhysicsTestEdict(s)
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
	msg.PutByte(3)
	msg.PutByte(7)
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
	if ent.Button0(s) != 1 || ent.Button2(s) != 1 || ent.Impulse(s) != 7 {
		t.Fatalf("edict button/impulse state not updated: b0=%v b2=%v impulse=%v", ent.Button0(s), ent.Button2(s), ent.Impulse(s))
	}
	if client.NumPings != 1 {
		t.Fatalf("num pings = %d, want 1", client.NumPings)
	}
}

func TestSVClientThinkNoclip(t *testing.T) {
	s := NewServer()
	s.FrameTime = 0.1

	ent := allocPhysicsTestEdict(s)
	ent.SetMoveType(s, float32(MoveTypeNoClip))
	ent.SetHealth(s, 100)
	ent.SetVAngle(s, [3]float32{30, 0, 0})

	client := &Client{
		Edict: ent,
		LastCmd: UserCmd{
			ForwardMove: 100,
			SideMove:    50,
			UpMove:      20,
		},
	}

	s.SV_ClientThink(client)

	if ent.Angles(s)[0] != -10 {
		t.Fatalf("pitch = %v, want -10", ent.Angles(s)[0])
	}
	if ent.Velocity(s) == [3]float32{} {
		t.Fatalf("noclip move did not update velocity")
	}
}

func TestRunClientQCThinkSyncsThirdPartyCombatStateFromQCVM(t *testing.T) {
	s := NewServer()
	s.QCVM = qc.NewVM()
	vm := newServerTestVM(s, 8)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSFrameTime), Name: vm.AllocString("frametime")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSMsgEntity), Name: vm.AllocString("msg_entity")},
	}

	clientEnt := s.AllocEdict()
	monster := s.AllocEdict()
	if clientEnt == nil || monster == nil {
		t.Fatal("failed to allocate edicts")
	}
	clientNum := s.NumForEdict(clientEnt)
	monsterNum := s.NumForEdict(monster)

	const mutateBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {
		vm.SetEFloat(monsterNum, qc.EntFieldHealth, 15)
		vm.SetEInt(monsterNum, qc.EntFieldEnemy, int32(clientNum))
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("player_postthink_test"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(mutateBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(mutateBuiltinOfs, -1)
	vm.NumEdicts = s.NumEdicts

	client := &Client{Edict: clientEnt}
	monster.SetHealth(s, 100)
	s.Time = 1
	s.FrameTime = 0.1

	s.runClientQCThink(client, "player_postthink_test")

	if got := monster.Health(s); got != 15 {
		t.Fatalf("monster health = %v, want 15", got)
	}
	if got := monster.Enemy(s); got != int32(clientNum) {
		t.Fatalf("monster enemy = %v, want %d", got, clientNum)
	}
}

func withUserCVars(t *testing.T, s *Server, values map[string]string) {
	t.Helper()
	for name, value := range values {
		if s.CVar.Get(name) == nil {
			s.CVar.Register(name, "0", cvar.FlagServerInfo, "")
		}
		s.CVar.Set(name, value)
	}
}

func TestSVClientThinkNoclipAltStyleUsesViewPitch(t *testing.T) {
	s := NewServer()
	withUserCVars(t, s, map[string]string{"sv_altnoclip": "1"})
	s.FrameTime = 0.1
	ent := allocPhysicsTestEdict(s)
	ent.SetMoveType(s, float32(MoveTypeNoClip))
	ent.SetHealth(s, 100)
	ent.SetVAngle(s, [3]float32{45, 0, 0})

	client := &Client{
		Edict: ent,
		LastCmd: UserCmd{
			ForwardMove: 100,
		},
	}

	s.SV_ClientThink(client)

	if ent.Velocity(s)[2] == 0 {
		t.Fatalf("sv_altnoclip=1 expected pitched noclip to include vertical velocity, got %v", ent.Velocity(s))
	}
}

func TestSVClientThinkNoclipClassicIgnoresPitch(t *testing.T) {
	s := NewServer()
	withUserCVars(t, s, map[string]string{"sv_altnoclip": "0"})
	s.FrameTime = 0.1
	ent := allocPhysicsTestEdict(s)
	ent.SetMoveType(s, float32(MoveTypeNoClip))
	ent.SetHealth(s, 100)
	ent.SetVAngle(s, [3]float32{45, 0, 0})

	client := &Client{
		Edict: ent,
		LastCmd: UserCmd{
			ForwardMove: 100,
		},
	}

	s.SV_ClientThink(client)

	if ent.Velocity(s)[2] != 0 {
		t.Fatalf("sv_altnoclip=0 expected horizontal noclip forward move, got %v", ent.Velocity(s))
	}
	if ent.Velocity(s)[0] == 0 && ent.Velocity(s)[1] == 0 {
		t.Fatalf("sv_altnoclip=0 expected non-zero horizontal velocity, got %v", ent.Velocity(s))
	}
}

func TestSVClientThinkWalkForwardIgnoresPitchVerticalProjection(t *testing.T) {
	s := NewServer()
	s.FrameTime = 0.05

	ent := allocPhysicsTestEdict(s)
	ent.SetMoveType(s, float32(MoveTypeWalk))
	ent.SetHealth(s, 100)
	ent.SetFlags(s, float32(FlagOnGround))
	ent.SetVAngle(s, [3]float32{60, 0, 0})

	client := &Client{
		Edict: ent,
		LastCmd: UserCmd{
			ForwardMove: 200,
		},
	}

	s.SV_ClientThink(client)

	if ent.Velocity(s)[2] != 0 {
		t.Fatalf("walk velocity z = %v, want 0", ent.Velocity(s)[2])
	}
	if ent.Velocity(s)[0] == 0 && ent.Velocity(s)[1] == 0 {
		t.Fatalf("walk forward move did not produce horizontal velocity: %v", ent.Velocity(s))
	}
}

func TestSVClientThinkGroundFrictionFeedsAccelerate(t *testing.T) {
	s := NewServer()
	s.FrameTime = 0.1

	ent := allocPhysicsTestEdict(s)
	ent.SetMoveType(s, float32(MoveTypeWalk))
	ent.SetHealth(s, 100)
	ent.SetFlags(s, float32(FlagOnGround))
	ent.SetVAngle(s, [3]float32{0, 0, 0})
	ent.SetVelocity(s, [3]float32{100, 0, 0})

	client := &Client{
		Edict: ent,
		LastCmd: UserCmd{
			ForwardMove: 200,
		},
	}

	s.SV_ClientThink(client)

	if diff := math.Abs(float64(ent.Velocity(s)[0] - 200)); diff > 0.001 {
		t.Fatalf("ground accelerate used stale pre-friction speed: got %.3f want 200", ent.Velocity(s)[0])
	}
}

func TestRunClientsProcessesMoveOnSpawnedMap(t *testing.T) {
	s := newStartMapDiagnosticsServer(t)

	s.ConnectClient(0)
	client := s.Static.Clients[0]
	client.Spawned = true

	pos, ok, diag := findWalkablePointWithDiagnostics(s)
	if !ok {
		t.Skipf("no walkable point found on start map; %s", diag.String())
	}

	ent := client.Edict
	ent.SetOrigin(s, pos)
	ent.SetMins(s, [3]float32{-16, -16, -24})
	ent.SetMaxs(s, [3]float32{16, 16, 32})
	ent.SetSolid(s, float32(SolidSlideBox))
	ent.SetMoveType(s, float32(MoveTypeWalk))
	ent.SetHealth(s, 100)
	ent.SetFlags(s, float32(FlagOnGround))
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
	msg.PutByte(0)
	msg.PutByte(0)
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
	s := newStartMapDiagnosticsServer(t)

	s.ConnectClient(0)
	client := s.Static.Clients[0]
	client.Spawned = true

	pos, ok, diag := findWalkablePointWithDiagnostics(s)
	if !ok {
		t.Skipf("no walkable point found on start map; %s", diag.String())
	}

	ent := client.Edict
	ent.SetOrigin(s, pos)
	ent.SetMins(s, [3]float32{-16, -16, -24})
	ent.SetMaxs(s, [3]float32{16, 16, 32})
	ent.SetSolid(s, float32(SolidSlideBox))
	ent.SetMoveType(s, float32(MoveTypeWalk))
	ent.SetHealth(s, 100)
	ent.SetFlags(s, float32(FlagOnGround))
	s.LinkEdict(ent, false)

	start := ent.Origin(s)
	if err := s.SubmitLoopbackCmd(0, [3]float32{0, 0, 0}, 200, 0, 0, 0, 0, float64(s.Time)); err != nil {
		t.Fatalf("SubmitLoopbackCmd: %v", err)
	}
	if err := s.Frame(0.05); err != nil {
		t.Fatalf("Frame: %v", err)
	}

	end := ent.Origin(s)
	if end == start {
		t.Fatalf("authoritative origin did not move: start=%v end=%v", start, end)
	}
	if dx, dy := end[0]-start[0], end[1]-start[1]; dx == 0 && dy == 0 {
		t.Fatalf("authoritative origin only changed vertically: start=%v end=%v", start, end)
	}
}

func TestLoopbackCmdWalkForwardWithPitchMovesHorizontally(t *testing.T) {
	s := newStartMapDiagnosticsServer(t)

	s.ConnectClient(0)
	client := s.Static.Clients[0]
	client.Spawned = true

	pos, ok, diag := findWalkablePointWithDiagnostics(s)
	if !ok {
		t.Skipf("no walkable point found on start map; %s", diag.String())
	}

	ent := client.Edict
	ent.SetOrigin(s, pos)
	ent.SetMins(s, [3]float32{-16, -16, -24})
	ent.SetMaxs(s, [3]float32{16, 16, 32})
	ent.SetSolid(s, float32(SolidSlideBox))
	ent.SetMoveType(s, float32(MoveTypeWalk))
	ent.SetHealth(s, 100)
	ent.SetFlags(s, float32(FlagOnGround))
	s.LinkEdict(ent, false)

	start := ent.Origin(s)
	if err := s.SubmitLoopbackCmd(0, [3]float32{45, 0, 0}, 200, 0, 0, 0, 0, float64(s.Time)); err != nil {
		t.Fatalf("SubmitLoopbackCmd: %v", err)
	}
	if err := s.Frame(0.05); err != nil {
		t.Fatalf("Frame: %v", err)
	}

	end := ent.Origin(s)
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

	pos, ok, diag := findWalkablePointWithDiagnostics(s)
	if !ok {
		t.Skipf("no walkable point found on start map; %s", diag.String())
	}

	ent := client.Edict
	ent.SetOrigin(s, pos)
	ent.SetVelocity(s, [3]float32{})
	ent.SetFlags(s, float32(FlagOnGround|FlagJumpReleased))
	ent.SetGroundEntity(s, 1)
	s.LinkEdict(ent, false)

	start := ent.Origin(s)
	if err := s.SubmitLoopbackCmd(0, [3]float32{}, 0, 0, 0, 2, 0, float64(s.Time)); err != nil {
		t.Fatalf("SubmitLoopbackCmd: %v", err)
	}
	if err := s.Frame(0.05); err != nil {
		t.Fatalf("Frame: %v", err)
	}

	if ent.Velocity(s)[2] <= 0 {
		t.Fatalf("jump did not apply upward velocity: velocity=%v", ent.Velocity(s))
	}
	if ent.Origin(s)[2] <= start[2] {
		t.Fatalf("jump did not move player upward: start=%v end=%v", start, ent.Origin(s))
	}
	if uint32(ent.Flags(s))&FlagOnGround != 0 {
		t.Fatalf("jump left player grounded: flags=0x%x", uint32(ent.Flags(s)))
	}
}

func TestFrameRealProgsDeathRespawnClearsPressedButtons(t *testing.T) {
	s := newStartMapDiagnosticsServer(t)
	withRuleCVars(t, s, map[string]string{
		"coop":       "0",
		"deathmatch": "1",
	})
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

	ent := client.Edict
	ent.SetHealth(s, 0)
	ent.SetDeadFlag(s, float32(DeadDead))
	ent.SetVelocity(s, [3]float32{})

	if err := s.SubmitLoopbackCmd(0, [3]float32{}, 0, 0, 0, 1, 0, float64(s.Time)); err != nil {
		t.Fatalf("SubmitLoopbackCmd(hold attack): %v", err)
	}
	if err := s.Frame(0.05); err != nil {
		t.Fatalf("Frame(hold attack): %v", err)
	}
	if ent.Health(s) > 0 {
		t.Fatalf("held attack should not respawn immediately: health=%v", ent.Health(s))
	}
	if got, want := DeadFlag(ent.DeadFlag(s)), DeadDead; got != want {
		t.Fatalf("deadflag after held attack = %v, want %v", got, want)
	}

	if err := s.SubmitLoopbackCmd(0, [3]float32{}, 0, 0, 0, 0, 0, float64(s.Time)); err != nil {
		t.Fatalf("SubmitLoopbackCmd(release): %v", err)
	}
	if err := s.Frame(0.05); err != nil {
		t.Fatalf("Frame(release): %v", err)
	}
	if ent.Health(s) > 0 {
		t.Fatalf("release should only mark respawnable: health=%v", ent.Health(s))
	}
	if got, want := DeadFlag(ent.DeadFlag(s)), DeadRespawnable; got != want {
		t.Fatalf("deadflag after release = %v, want %v", got, want)
	}

	if err := s.SubmitLoopbackCmd(0, [3]float32{}, 0, 0, 0, 1, 0, float64(s.Time)); err != nil {
		t.Fatalf("SubmitLoopbackCmd(respawn press): %v", err)
	}
	if err := s.Frame(0.05); err != nil {
		t.Fatalf("Frame(respawn press): %v", err)
	}
	if ent.Health(s) <= 0 {
		t.Fatalf("respawn press did not restore health: health=%v", ent.Health(s))
	}
	if got, want := DeadFlag(ent.DeadFlag(s)), DeadNo; got != want {
		t.Fatalf("deadflag after respawn = %v, want %v", got, want)
	}
	if ent.Button0(s) != 0 || ent.Button1(s) != 0 || ent.Button2(s) != 0 {
		t.Fatalf("QC respawn should clear held buttons: b0=%v b1=%v b2=%v", ent.Button0(s), ent.Button1(s), ent.Button2(s))
	}
}

func TestPhysicsWalkClearsStaleGroundFlagWhenUnsupported(t *testing.T) {
	s := newStartMapDiagnosticsServer(t)

	pos, ok, diag := findWalkablePointWithDiagnostics(s)
	if !ok {
		t.Skipf("no walkable point found on start map; %s", diag.String())
	}

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("AllocEdict returned nil")
	}

	pos[2] += 96
	ent.SetOrigin(s, pos)
	ent.SetMins(s, [3]float32{-16, -16, -24})
	ent.SetMaxs(s, [3]float32{16, 16, 32})
	ent.SetSolid(s, float32(SolidSlideBox))
	ent.SetMoveType(s, float32(MoveTypeWalk))
	ent.SetHealth(s, 100)
	ent.SetFlags(s, float32(FlagOnGround))
	s.LinkEdict(ent, false)

	if s.CheckBottom(ent) {
		t.Skipf("lifted test position unexpectedly has support: origin=%v", ent.Origin(s))
	}

	start := ent.Origin(s)
	s.FrameTime = 0.05
	s.PhysicsWalk(ent)

	if uint32(ent.Flags(s))&FlagOnGround != 0 {
		t.Fatalf("stale onground flag was not cleared: flags=0x%x", uint32(ent.Flags(s)))
	}
	if ent.GroundEntity(s) != 0 {
		t.Fatalf("ground entity = %v, want 0", ent.GroundEntity(s))
	}
	if ent.Origin(s)[2] >= start[2] {
		t.Fatalf("entity did not fall after losing support: start=%v end=%v", start, ent.Origin(s))
	}
}

func TestRunClientSpawnQCCallsClientConnectBeforePutClientInServer(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	client := s.Static.Clients[0]
	client.Edict = s.EdictNum(1)

	vm := newServerTestVM(s, 16)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSFrameTime), Name: vm.AllocString("frametime")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSMsgEntity), Name: vm.AllocString("msg_entity")},
	}
	var order []string
	vm.Builtins[1] = func(vm *qc.VM) { order = append(order, "ClientConnect") }
	vm.Builtins[2] = func(vm *qc.VM) { order = append(order, "PutClientInServer") }
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("ClientConnect"), FirstStatement: 0},
		{Name: vm.AllocString("PutClientInServer"), FirstStatement: 2},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: 10},
		{Op: uint16(qc.OPDone)},
		{Op: uint16(qc.OPCall0), A: 11},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(10, -1)
	vm.SetGInt(11, -2)
	s.QCVM = vm

	if err := s.runClientSpawnQC(client); err != nil {
		t.Fatalf("runClientSpawnQC failed: %v", err)
	}

	if len(order) != 2 || order[0] != "ClientConnect" || order[1] != "PutClientInServer" {
		t.Fatalf("spawn QC order = %v, want [ClientConnect PutClientInServer]", order)
	}
}

func TestRunClientSpawnQCSyncsEntitiesSpawnedByClientConnect(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	client := s.Static.Clients[0]
	client.Edict = s.EdictNum(1)

	vm := newServerTestVM(s, 16)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSFrameTime), Name: vm.AllocString("frametime")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSMsgEntity), Name: vm.AllocString("msg_entity")},
	}
	vm.FieldDefs = []qc.DDef{
		{Type: uint16(qc.EvString), Ofs: uint16(qc.EntFieldClassName), Name: vm.AllocString("classname")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.EntFieldOwner), Name: vm.AllocString("owner")},
	}
	animClassname := vm.AllocString("animcontroller")
	vm.Builtins[1] = func(vm *qc.VM) {
		controller := s.AllocEdict()
		controllerNum := s.NumForEdict(controller)
		vm.SetEString(controllerNum, qc.EntFieldClassName, int32(animClassname))
		vm.SetEEntity(controllerNum, qc.EntFieldOwner, int32(vm.GInt(qc.OFSSelf)))
	}
	vm.Builtins[2] = func(vm *qc.VM) {}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("ClientConnect"), FirstStatement: -1},
		{Name: vm.AllocString("PutClientInServer"), FirstStatement: -2},
	}
	s.QCVM = vm

	if err := s.runClientSpawnQC(client); err != nil {
		t.Fatalf("runClientSpawnQC failed: %v", err)
	}

	controller := s.EdictNum(2)
	if controller == nil {
		t.Fatal("ClientConnect-spawned edict was not mirrored into the server")
	}
	if got := s.String(controller.ClassName(s)); got != "animcontroller" {
		t.Fatalf("spawned edict classname = %q, want animcontroller", got)
	}
	if got := controller.Owner(s); got != 1 {
		t.Fatalf("spawned edict owner = %d, want player edict 1", got)
	}
}

func TestExecuteClientStringCommandFallsBackToSVParseClientCommandQC(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	client := s.Static.Clients[0]
	client.Active = true
	client.Edict = s.EdictNum(1)

	vm := newServerTestVM(s, 8)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSMsgEntity), Name: vm.AllocString("msg_entity")},
		{Type: uint16(qc.EvString), Ofs: uint16(qc.OFSParm0), Name: vm.AllocString("parm0")},
	}
	var got string
	vm.Builtins[1] = func(vm *qc.VM) { got = vm.GString(qc.OFSParm0) }
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("SV_ParseClientCommand"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: 10},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(10, -1)
	s.QCVM = vm

	if err := s.executeClientStringCommand(client, "mod_custom 1 2"); err != nil {
		t.Fatalf("executeClientStringCommand() error = %v", err)
	}
	if got != "mod_custom 1 2" {
		t.Fatalf("SV_ParseClientCommand parm0 = %q, want %q", got, "mod_custom 1 2")
	}
}

func TestDropClientBroadcastsRosterClearsWithoutFreeingPlayerEdict(t *testing.T) {
	s := NewServer()
	if err := s.Init(2); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	loop := inet.NewLoopback()
	if err := loop.Init(); err != nil {
		t.Fatalf("loopback init: %v", err)
	}
	clientPeer := loop.Connect()
	serverSock := loop.CheckNewConnections()
	if serverSock == nil {
		t.Fatal("server socket missing")
	}
	defer inet.DefaultNetwork().Close(clientPeer)

	dropped := s.Static.Clients[0]
	observer := s.Static.Clients[1]
	dropped.Active = true
	dropped.Spawned = true
	dropped.Name = "player1"
	dropped.Color = 77
	dropped.OldFrags = 4
	dropped.Edict = s.EdictNum(1)
	dropped.Edict.SetFrags(s, 4)
	dropped.NetConnection = serverSock
	observer.Active = true
	observer.Message = NewMessageBuffer(MaxDatagram)

	s.DropClient(dropped, false)

	if dropped.Active {
		t.Fatal("client should be inactive after drop")
	}
	if dropped.NetConnection != nil {
		t.Fatal("drop should clear client net connection")
	}
	if dropped.Edict.Free {
		t.Fatal("drop should preserve player edict allocation")
	}
	if got := dropped.Edict.Frags(s); got != 0 {
		t.Fatalf("player frags = %v, want 0 after roster clear", got)
	}
	if got := string(observer.Message.Data[:observer.Message.Len()]); !bytes.Contains([]byte(got), []byte{byte(inet.SVCUpdateName), 0}) {
		t.Fatalf("observer message missing roster-clear update: %v", observer.Message.Data[:observer.Message.Len()])
	}
	msgType, payload := inet.DefaultNetwork().Message(clientPeer)
	if msgType != 1 {
		t.Fatalf("disconnect message type = %d, want 1", msgType)
	}
	if !bytes.Contains(payload, []byte{byte(inet.SVCDisconnect)}) {
		t.Fatalf("disconnect payload missing SVCDisconnect: %v", payload)
	}
}
