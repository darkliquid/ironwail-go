package server

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/qc"
	"github.com/ironwail/ironwail-go/internal/testutil"
)

func newPhysicsTestServer() *Server {
	s := &Server{
		Gravity:     800,
		MaxVelocity: 2000,
		FrameTime:   0.1,
		Edicts:      []*Edict{{Vars: &EntVars{}}},
		NumEdicts:   1,
	}
	return s
}

func withPhysicsCVars(t *testing.T, values map[string]string) {
	t.Helper()
	original := make(map[string]string, len(values))
	for name := range values {
		if cvar.Get(name) == nil {
			cvar.Register(name, "0", cvar.FlagServerInfo, "")
		}
		original[name] = cvar.StringValue(name)
	}
	for name, value := range values {
		cvar.Set(name, value)
	}
	t.Cleanup(func() {
		for name, value := range original {
			cvar.Set(name, value)
		}
	})
}

func TestClipVelocity(t *testing.T) {
	in := [3]float32{100, 0.05, -5}
	normal := [3]float32{0, 0, 1}
	out := ClipVelocity(in, normal, 1)

	if out[2] != 0 {
		t.Fatalf("out[2] = %v, want 0", out[2])
	}
}

func TestPhysicsNoClipMovesOriginAndAngles(t *testing.T) {
	s := newPhysicsTestServer()
	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.Velocity = [3]float32{10, -5, 2}
	ent.Vars.AVelocity = [3]float32{0, 90, 0}
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)

	s.PhysicsNoClip(ent)

	if ent.Vars.Origin != [3]float32{1, -0.5, 0.2} {
		t.Fatalf("origin = %v", ent.Vars.Origin)
	}
	if ent.Vars.Angles != [3]float32{0, 9, 0} {
		t.Fatalf("angles = %v", ent.Vars.Angles)
	}
}

func TestPhysicsPusherAdvancesLocalTimeWhenIdle(t *testing.T) {
	s := newPhysicsTestServer()
	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.MoveType = float32(MoveTypePush)
	ent.Vars.LTime = 3
	ent.Vars.NextThink = 10
	s.PhysicsPusher(ent)

	if diff := ent.Vars.LTime - 3.1; diff < -0.0001 || diff > 0.0001 {
		t.Fatalf("ltime = %v, want 3.1", ent.Vars.LTime)
	}
}

func TestPhysicsTossOnGroundDoesNotMove(t *testing.T) {
	s := newPhysicsTestServer()
	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.Flags = float32(FlagOnGround)
	ent.Vars.Origin = [3]float32{1, 2, 3}
	ent.Vars.Velocity = [3]float32{50, 60, 70}

	s.PhysicsToss(ent)

	if ent.Vars.Origin != [3]float32{1, 2, 3} {
		t.Fatalf("origin changed on ground toss: %v", ent.Vars.Origin)
	}
}

func TestFlyMoveDoesNotGroundOnNonBSPFloor(t *testing.T) {
	s := NewServer()
	s.FrameTime = 0.1
	s.WorldModel = CreateSyntheticWorldModel()
	if len(s.Edicts) == 0 || s.Edicts[0] == nil {
		t.Fatal("missing world edict")
	}
	s.Edicts[0].Vars.Solid = float32(SolidBSP)
	s.ClearWorld()

	platform := s.AllocEdict()
	if platform == nil {
		t.Fatal("failed to alloc platform")
	}
	platform.Vars.Origin = [3]float32{0, 0, 72}
	platform.Vars.Mins = [3]float32{-64, -64, -8}
	platform.Vars.Maxs = [3]float32{64, 64, 8}
	platform.Vars.Solid = float32(SolidBBox)
	s.LinkEdict(platform, false)

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("failed to alloc mover")
	}
	ent.Vars.Origin = [3]float32{0, 0, 112}
	ent.Vars.Mins = [3]float32{-16, -16, -24}
	ent.Vars.Maxs = [3]float32{16, 16, 32}
	ent.Vars.Solid = float32(SolidSlideBox)
	ent.Vars.MoveType = float32(MoveTypeWalk)
	ent.Vars.Velocity = [3]float32{0, 0, -200}
	s.LinkEdict(ent, false)

	blocked := s.FlyMove(ent, s.FrameTime, nil)
	if blocked&1 == 0 {
		t.Fatalf("FlyMove blocked=%d, want floor contact bit set", blocked)
	}
	if uint32(ent.Vars.Flags)&FlagOnGround != 0 {
		t.Fatalf("Flags unexpectedly include onground after SolidBBox contact: flags=%#x", uint32(ent.Vars.Flags))
	}
	if ent.Vars.GroundEntity != 0 {
		t.Fatalf("ground entity = %d, want 0 for non-BSP contact", ent.Vars.GroundEntity)
	}
}

func TestPhysicsStepOnGroundSkipsFreefall(t *testing.T) {
	s := newPhysicsTestServer()
	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.Flags = float32(FlagOnGround)
	ent.Vars.Velocity = [3]float32{0, 0, 42}

	s.PhysicsStep(ent)

	if ent.Vars.Velocity[2] != 42 {
		t.Fatalf("z velocity changed: %v", ent.Vars.Velocity[2])
	}
}

func TestPhysicsFrameOnSpawnedMap(t *testing.T) {
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

	before := s.Time
	s.Physics()
	if s.Time <= before {
		t.Fatalf("time did not advance: before=%v after=%v", before, s.Time)
	}
}

func TestPhysicsFreezeNonClientsCVar(t *testing.T) {
	mkServer := func() (*Server, *Edict, *Edict) {
		s := newPhysicsTestServer()
		s.Static = &ServerStatic{MaxClients: 1}
		clientEnt := &Edict{Vars: &EntVars{}}
		clientEnt.Vars.MoveType = float32(MoveTypeNoClip)
		clientEnt.Vars.Velocity = [3]float32{10, 0, 0}
		nonClientEnt := &Edict{Vars: &EntVars{}}
		nonClientEnt.Vars.MoveType = float32(MoveTypeNoClip)
		nonClientEnt.Vars.Velocity = [3]float32{20, 0, 0}
		s.Edicts = append(s.Edicts, clientEnt, nonClientEnt)
		s.NumEdicts = len(s.Edicts)
		return s, clientEnt, nonClientEnt
	}

	t.Run("freeze enabled skips non-clients", func(t *testing.T) {
		withPhysicsCVars(t, map[string]string{"sv_freezenonclients": "1"})
		s, clientEnt, nonClientEnt := mkServer()

		s.Physics()

		if clientEnt.Vars.Origin[0] == 0 {
			t.Fatalf("client entity did not move with freeze enabled: origin=%v", clientEnt.Vars.Origin)
		}
		if nonClientEnt.Vars.Origin[0] != 0 {
			t.Fatalf("non-client entity moved with freeze enabled: origin=%v", nonClientEnt.Vars.Origin)
		}
	})

	t.Run("freeze disabled updates all entities", func(t *testing.T) {
		withPhysicsCVars(t, map[string]string{"sv_freezenonclients": "0"})
		s, clientEnt, nonClientEnt := mkServer()

		s.Physics()

		if clientEnt.Vars.Origin[0] == 0 {
			t.Fatalf("client entity did not move with freeze disabled: origin=%v", clientEnt.Vars.Origin)
		}
		if nonClientEnt.Vars.Origin[0] == 0 {
			t.Fatalf("non-client entity did not move with freeze disabled: origin=%v", nonClientEnt.Vars.Origin)
		}
	})
}

func TestPhysicsTelemetryFrameHooks(t *testing.T) {
	s := newPhysicsTestServer()
	s.QCVM = qc.NewVM()

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.MoveType = float32(MoveTypeNoClip)
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)

	lines := make([]string, 0, 4)
	s.DebugTelemetry = NewDebugTelemetryWithConfig(func() DebugTelemetryConfig {
		return DebugTelemetryConfig{
			Enabled:      true,
			EventMask:    debugEventMaskFrame,
			EntityFilter: debugEntityFilter{all: true},
			SummaryMode:  2,
		}
	}, func(line string) {
		lines = append(lines, line)
	})
	oldEnable := debugTelemetryEnableCVar
	debugTelemetryEnableCVar = cvar.Register("sv_debug_telemetry_test_frame", "1", cvar.FlagNone, "")
	t.Cleanup(func() {
		debugTelemetryEnableCVar = oldEnable
	})

	s.Physics()

	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"kind=frame",
		"physics begin",
		"physics end",
		"summary total=2 qc=0",
		"counts=frame=2",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("telemetry output missing %q in:\n%s", want, joined)
		}
	}
}

func TestRunThinkTelemetry(t *testing.T) {
	s := newPhysicsTestServer()
	s.QCVM = qc.NewVM()

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.NextThink = 0.05
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)

	lines := make([]string, 0, 2)
	s.DebugTelemetry = NewDebugTelemetryWithConfig(func() DebugTelemetryConfig {
		return DebugTelemetryConfig{
			Enabled:      true,
			EventMask:    debugEventMaskThink,
			EntityFilter: debugEntityFilter{all: true},
			SummaryMode:  0,
		}
	}, func(line string) {
		lines = append(lines, line)
	})
	oldEnable := debugTelemetryEnableCVar
	debugTelemetryEnableCVar = cvar.Register("sv_debug_telemetry_test_think", "1", cvar.FlagNone, "")
	t.Cleanup(func() {
		debugTelemetryEnableCVar = oldEnable
	})

	if ok := s.RunThink(ent); !ok {
		t.Fatal("RunThink unexpectedly returned false")
	}

	if len(lines) != 2 {
		t.Fatalf("got %d telemetry lines, want 2", len(lines))
	}
	if !strings.Contains(lines[0], "runthink begin") || !strings.Contains(lines[1], "runthink end") {
		t.Fatalf("unexpected telemetry lines: %#v", lines)
	}
}

func TestRunThinkPublishesQCTimeGlobal(t *testing.T) {
	s := newPhysicsTestServer()
	s.QCVM = qc.NewVM()
	vm := newServerTestVM(s, 8)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("test_think"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPDone)},
	}

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.NextThink = 0.05
	ent.Vars.Think = 1
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)

	if ok := s.RunThink(ent); !ok {
		t.Fatal("RunThink unexpectedly returned false")
	}
	if got := s.QCVM.GetGlobalFloat("time"); got != 0.05 {
		t.Fatalf("QC global time = %v, want 0.05", got)
	}
}
