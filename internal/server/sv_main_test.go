package server

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/fs"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/testutil"
)

func withSkillCVar(t *testing.T, s *Server, value string) {
	t.Helper()
	if s.CVar.Get("skill") == nil {
		s.CVar.Register("skill", "1", cvar.FlagArchive, "")
	}
	s.CVar.Set("skill", value)
}

func TestParseWorldspawnSkyboxName(t *testing.T) {
	entities := `{
"classname" "worldspawn"
"sky" "lowres"
"_skyname" "gfx/env/qbj3"
"qlsky" "env/final"
}
{
"classname" "info_player_start"
}`

	if got := parseWorldspawnSkyboxName(entities); got != "env/final" {
		t.Fatalf("parseWorldspawnSkyboxName() = %q, want %q", got, "env/final")
	}
}

// TestSpawnServerSyncsRoundedClampedSkillToQCVM tests spawn-time skill setup.
// C normalizes the skill cvar before worldspawn; worldspawn then publishes it
// to QuakeC so spawn functions and frame logic start from the canonical value.
// Where in C: SV_SpawnServer in sv_main.c
func TestSpawnServerSyncsRoundedClampedSkillToQCVM(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	testCases := []struct {
		name  string
		value string
		want  int
	}{
		{name: "negative clamps to zero", value: "-1", want: 0},
		{name: "fraction rounds to nearest", value: "0.6", want: 1},
		{name: "middle value preserved", value: "2.2", want: 2},
		{name: "high value clamps to three", value: "4", want: 3},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vfs := fs.NewFileSystem()
			if err := vfs.Init(baseDir, "id1"); err != nil {
				t.Fatalf("init filesystem: %v", err)
			}
			defer vfs.Close()

			s := NewServer()
			withSkillCVar(t, s, tc.value)
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

			if got := int(s.QCVM.GlobalFloat("skill")); got != tc.want {
				t.Fatalf("QC skill global = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestSyncQCVMStateDoesNotOverwriteQCSkillGlobal(t *testing.T) {
	s := NewServer()
	withSkillCVar(t, s, "1")
	vm := newServerTestVM(s, 1)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvFloat), Ofs: 10, Name: vm.AllocString("skill")},
	}
	vm.SetGFloat(10, 3)

	s.syncQCVMState()

	if got := vm.GFloat(10); got != 3 {
		t.Fatalf("syncQCVMState overwrote QC skill global = %v, want 3", got)
	}
}

// TestSpawnServerRunsTwoSettlePhysicsFramesBeforeSignon tests the \"warmup\" period after server spawn.
// It replicating the engine's behavior of running a few physics frames to allow entities to settle before accepting clients.
// Where in C: SV_SpawnServer in sv_main.c
func TestSpawnServerRunsTwoSettlePhysicsFramesBeforeSignon(t *testing.T) {
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
	if got := s.Time; got < 1.19 || got > 1.21 {
		t.Fatalf("server time after spawn = %v, want ~1.2 after two settle frames", got)
	}
}

// TestLoadMapEntitiesRelinksSpawnedTriggerAfterQCSpawn tests entity relinking after spawning.
// It ensuring that entities (especially triggers) are correctly linked into the world's area nodes after their QuakeC spawn function has run.
// Where in C: ED_LoadFromFile and ED_NewEntry in sv_main.c / pr_edict.c
func TestLoadMapEntitiesRelinksSpawnedTriggerAfterQCSpawn(t *testing.T) {
	s := NewServer()
	vm := newServerTestVM(s, 8)
	s.ClearWorld()
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSWorld), Name: vm.AllocString("world")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
		{Type: uint16(qc.EvString), Ofs: uint16(qc.OFSMapName), Name: vm.AllocString("mapname")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSDeathmatch), Name: vm.AllocString("deathmatch")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSCoop), Name: vm.AllocString("coop")},
	}

	const (
		triggerInitBuiltinOfs = 10
	)
	vm.Builtins[1] = func(vm *qc.VM) {
		self := int(vm.GInt(qc.OFSSelf))
		origin := vm.EVector(self, qc.EntFieldOrigin)
		mins := [3]float32{-16, -16, -16}
		maxs := [3]float32{16, 16, 16}
		vm.SetEFloat(self, qc.EntFieldSolid, float32(SolidTrigger))
		vm.SetEInt(self, qc.EntFieldTouch, 99)
		vm.SetEVector(self, qc.EntFieldMins, mins)
		vm.SetEVector(self, qc.EntFieldMaxs, maxs)
		vm.SetEVector(self, qc.EntFieldSize, [3]float32{32, 32, 32})
		vm.SetEVector(self, qc.EntFieldAbsMin, [3]float32{origin[0] + mins[0], origin[1] + mins[1], origin[2] + mins[2]})
		vm.SetEVector(self, qc.EntFieldAbsMax, [3]float32{origin[0] + maxs[0], origin[1] + maxs[1], origin[2] + maxs[2]})
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("worldspawn"), FirstStatement: 2},
		{Name: vm.AllocString("trigger_test"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(triggerInitBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(triggerInitBuiltinOfs, -1)

	lines := make([]string, 0, 16)
	s.DebugTelemetry = NewDebugTelemetryWithConfig(func() DebugTelemetryConfig {
		return DebugTelemetryConfig{
			Enabled:      true,
			EventMask:    debugEventMaskTrigger,
			EntityFilter: debugEntityFilter{all: true},
			SummaryMode:  0,
		}
	}, func(line string) {
		lines = append(lines, line)
	})
	oldEnable := debugTelemetryEnableCVar
	debugTelemetryEnableCVar = s.CVar.Register("sv_debug_telemetry_test_spawned_trigger", "1", cvar.FlagNone, "")
	t.Cleanup(func() {
		debugTelemetryEnableCVar = oldEnable
	})

	raw := `{
"classname" "worldspawn"
}
{
"classname" "trigger_test"
"origin" "128 0 0"
}`
	if err := s.loadMapEntities(raw); err != nil {
		t.Fatalf("loadMapEntities() error = %v", err)
	}

	trigger := s.EdictNum(1)
	if trigger == nil || trigger.Vars == nil {
		t.Fatal("spawned trigger missing")
	}
	if got := trigger.Vars.Solid; got != float32(SolidTrigger) {
		t.Fatalf("trigger solid = %v, want %v", got, float32(SolidTrigger))
	}
	if got := trigger.Vars.AbsMin; got != [3]float32{111, -17, -17} {
		t.Fatalf("trigger absmin = %v", got)
	}
	if got := trigger.Vars.AbsMax; got != [3]float32{145, 17, 17} {
		t.Fatalf("trigger absmax = %v", got)
	}

	probe := &Edict{Vars: &EntVars{
		AbsMin: [3]float32{120, -4, -4},
		AbsMax: [3]float32{136, 4, 4},
	}}
	touches := make([]*Edict, 0, 2)
	s.areaTriggerEdicts(probe, &s.Areanodes[0], &touches, s.NumEdicts)
	if len(touches) != 1 || touches[0] != trigger {
		t.Fatalf("areaTriggerEdicts() = %#v, want spawned trigger", touches)
	}

	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"spawn trigger qc begin classname=\"trigger_test\"",
		"spawn trigger qc end classname=\"trigger_test\"",
		"spawn trigger relink classname=\"trigger_test\" link=linked",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in telemetry:\n%s", want, joined)
		}
	}
}

// TestLoadMapEntitiesRelinksSpawnedTriggerWhenReusingFreedEdict tests entity reuse logic during map load.
// It verifying that when the server reuses a freed edict for a new entity, it is correctly unlinked from its old position and relinked into the new one.
// Where in C: ED_NewEntry in pr_edict.c
func TestLoadMapEntitiesRelinksSpawnedTriggerWhenReusingFreedEdict(t *testing.T) {
	s := NewServer()
	vm := newServerTestVM(s, 8)
	s.ClearWorld()
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSWorld), Name: vm.AllocString("world")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
		{Type: uint16(qc.EvString), Ofs: uint16(qc.OFSMapName), Name: vm.AllocString("mapname")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSDeathmatch), Name: vm.AllocString("deathmatch")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSCoop), Name: vm.AllocString("coop")},
	}

	const triggerInitBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {
		self := int(vm.GInt(qc.OFSSelf))
		origin := vm.EVector(self, qc.EntFieldOrigin)
		mins := [3]float32{-16, -16, -16}
		maxs := [3]float32{16, 16, 16}
		vm.SetEFloat(self, qc.EntFieldSolid, float32(SolidTrigger))
		vm.SetEInt(self, qc.EntFieldTouch, 99)
		vm.SetEVector(self, qc.EntFieldMins, mins)
		vm.SetEVector(self, qc.EntFieldMaxs, maxs)
		vm.SetEVector(self, qc.EntFieldSize, [3]float32{32, 32, 32})
		vm.SetEVector(self, qc.EntFieldAbsMin, [3]float32{origin[0] + mins[0], origin[1] + mins[1], origin[2] + mins[2]})
		vm.SetEVector(self, qc.EntFieldAbsMax, [3]float32{origin[0] + maxs[0], origin[1] + maxs[1], origin[2] + maxs[2]})
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("worldspawn"), FirstStatement: 2},
		{Name: vm.AllocString("trigger_test"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(triggerInitBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(triggerInitBuiltinOfs, -1)

	reused := s.AllocEdict()
	if reused == nil {
		t.Fatal("failed to allocate reusable edict")
	}
	s.FreeEdict(reused)

	raw := `{
"classname" "worldspawn"
}
{
"classname" "trigger_test"
"origin" "128 0 0"
}`
	if err := s.loadMapEntities(raw); err != nil {
		t.Fatalf("loadMapEntities() error = %v", err)
	}

	trigger := s.EdictNum(1)
	if trigger == nil || trigger.Vars == nil {
		t.Fatal("spawned trigger missing")
	}
	if trigger.Free {
		t.Fatal("spawned trigger unexpectedly still marked free")
	}
	if got := trigger.Vars.AbsMin; got != [3]float32{111, -17, -17} {
		t.Fatalf("trigger absmin = %v", got)
	}
	if got := trigger.Vars.AbsMax; got != [3]float32{145, 17, 17} {
		t.Fatalf("trigger absmax = %v", got)
	}
	if trigger.AreaPrev == nil || trigger.AreaNext == nil {
		t.Fatalf("spawned trigger was not linked into area tree: prev=%p next=%p", trigger.AreaPrev, trigger.AreaNext)
	}
}

// TestLoadMapEntitiesPreservesQCOnlyMapFieldForSpawn tests persistence of QuakeC-only fields during entity loading.
// It ensuring that fields defined only in progs.dat (and not known to the engine) are correctly populated from the map's entity string.
// Where in C: ED_ParseEdict in pr_edict.c
func TestLoadMapEntitiesPreservesQCOnlyMapFieldForSpawn(t *testing.T) {
	s := NewServer()
	vm := newServerTestVM(s, 8)
	s.ClearWorld()
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSWorld), Name: vm.AllocString("world")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}
	vm.FieldDefs = append(vm.FieldDefs, qc.DDef{
		Type: uint16(qc.EvFloat),
		Ofs:  110,
		Name: vm.AllocString("speed"),
	})

	const inspectBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {
		self := int(vm.GInt(qc.OFSSelf))
		vm.SetEFloat(self, qc.EntFieldHealth, vm.EFloat(self, 110))
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("worldspawn"), FirstStatement: 2},
		{Name: vm.AllocString("trigger_test"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(inspectBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(inspectBuiltinOfs, -1)

	raw := `{
"classname" "worldspawn"
}
{
"classname" "trigger_test"
"speed" "321"
}`
	if err := s.loadMapEntities(raw); err != nil {
		t.Fatalf("loadMapEntities() error = %v", err)
	}

	spawned := s.EdictNum(1)
	if spawned == nil || spawned.Vars == nil {
		t.Fatal("spawned edict missing")
	}
	if got := spawned.Vars.Health; got != 321 {
		t.Fatalf("spawned health = %v, want 321 from QC-only speed field", got)
	}
	if got := vm.EFloat(1, 110); got != 321 {
		t.Fatalf("QC-only speed field = %v, want 321 after spawn", got)
	}
}

func TestLoadMapEntitiesFailsWhenWorldspawnFunctionMissing(t *testing.T) {
	s := NewServer()
	vm := newServerTestVM(s, 8)
	s.ClearWorld()
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSWorld), Name: vm.AllocString("world")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("trigger_test"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPDone)},
	}

	raw := `{
"classname" "worldspawn"
}
{
"classname" "trigger_test"
}`

	err := s.loadMapEntities(raw)
	if err == nil {
		t.Fatal("loadMapEntities() should fail when worldspawn function is missing")
	}
	if !strings.Contains(err.Error(), "worldspawn spawn function not found") {
		t.Fatalf("loadMapEntities() error = %v", err)
	}

	world := s.EdictNum(0)
	if world == nil || world.Free {
		t.Fatalf("world entity corrupted after worldspawn failure: %#v", world)
	}
}

// TestLoadMapEntitiesClearsQCOnlyFieldsBeforeSpawn tests clearing of QuakeC-only fields for reused edicts.
// It preventing state leakage between entities when an edict is reused.
// Where in C: ED_NewEntry in pr_edict.c
func TestLoadMapEntitiesClearsQCOnlyFieldsBeforeSpawn(t *testing.T) {
	s := NewServer()
	vm := newServerTestVM(s, 8)
	s.ClearWorld()
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSWorld), Name: vm.AllocString("world")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}
	vm.FieldDefs = append(vm.FieldDefs, qc.DDef{
		Type: uint16(qc.EvFloat),
		Ofs:  110,
		Name: vm.AllocString("attack_finished"),
	})

	const inspectBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {
		self := int(vm.GInt(qc.OFSSelf))
		if got := vm.EFloat(self, 110); got != 0 {
			vm.SetEFloat(self, qc.EntFieldSolid, float32(SolidNot))
			return
		}
		vm.SetEFloat(self, qc.EntFieldSolid, float32(SolidTrigger))
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("worldspawn"), FirstStatement: 2},
		{Name: vm.AllocString("trigger_test"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(inspectBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(inspectBuiltinOfs, -1)

	reused := s.AllocEdict()
	if reused == nil {
		t.Fatal("failed to allocate reusable edict")
	}
	entNum := s.NumForEdict(reused)
	vm.NumEdicts = s.NumEdicts
	vm.SetEFloat(entNum, 110, 123)
	s.FreeEdict(reused)

	raw := `{
"classname" "worldspawn"
}
{
"classname" "trigger_test"
}`
	if err := s.loadMapEntities(raw); err != nil {
		t.Fatalf("loadMapEntities() error = %v", err)
	}

	spawned := s.EdictNum(entNum)
	if spawned == nil || spawned.Vars == nil {
		t.Fatal("spawned edict missing")
	}
	if got := spawned.Vars.Solid; got != float32(SolidTrigger) {
		t.Fatalf("spawned solid = %v, want %v", got, float32(SolidTrigger))
	}
	if got := vm.EFloat(entNum, 110); got != 0 {
		t.Fatalf("QC-only field attack_finished = %v, want 0 after spawn clear", got)
	}
}

// TestAllocEdictUnlinksReusedFreedEdictBeforeReset tests edict allocation safety.
// It ensuring that an edict is completely removed from all engine systems (like area nodes) before it is reset and returned for reuse.
// Where in C: ED_Alloc in pr_edict.c
func TestAllocEdictUnlinksReusedFreedEdictBeforeReset(t *testing.T) {
	s := NewServer()
	s.Areanodes = make([]AreaNode, AreaNodes)
	s.ClearWorld()

	e := s.AllocEdict()
	if e == nil {
		t.Fatal("failed to alloc edict")
	}
	e.Vars.Origin = [3]float32{64, 0, 0}
	e.Vars.Mins = [3]float32{-16, -16, -16}
	e.Vars.Maxs = [3]float32{16, 16, 16}
	e.Vars.Solid = float32(SolidTrigger)
	s.LinkEdict(e, false)
	if e.AreaPrev == nil || e.AreaNext == nil {
		t.Fatal("expected edict to be linked before free")
	}

	s.FreeEdict(e)
	probe := &Edict{Vars: &EntVars{
		AbsMin: [3]float32{48, -4, -4},
		AbsMax: [3]float32{80, 4, 4},
	}}
	touches := make([]*Edict, 0, 2)
	s.areaTriggerEdicts(probe, &s.Areanodes[0], &touches, s.NumEdicts)
	if len(touches) != 0 {
		t.Fatalf("freed edict still present in trigger links: %#v", touches)
	}

	reused := s.AllocEdict()
	if reused != e {
		t.Fatalf("AllocEdict reused %p, want same edict %p", reused, e)
	}
	if reused.Free {
		t.Fatal("reused edict still marked free")
	}
	if reused.AreaPrev != nil || reused.AreaNext != nil {
		t.Fatalf("reused edict still has area links: prev=%p next=%p", reused.AreaPrev, reused.AreaNext)
	}
}

// TestAllocEdictHonorsReuseCooldownAfterInitialServerWarmup tests the edict reuse cooldown.
// It replicating the engine's 0.5s delay before reusing a freed edict, which prevents network protocol errors from stale entity IDs.
// Where in C: ED_Alloc in pr_edict.c
func TestAllocEdictHonorsReuseCooldownAfterInitialServerWarmup(t *testing.T) {
	s := NewServer()
	s.Time = 3

	e := s.AllocEdict()
	if e == nil {
		t.Fatal("failed to alloc edict")
	}

	s.FreeEdict(e)
	s.Time = 3.25

	notYetReused := s.AllocEdict()
	if notYetReused == nil {
		t.Fatal("failed to alloc replacement edict")
	}
	if notYetReused == e {
		t.Fatal("freed edict reused before 0.5 second cooldown elapsed")
	}

	s.Time = 3.6
	reused := s.AllocEdict()
	if reused != e {
		t.Fatalf("AllocEdict reused %p, want cooled-down edict %p", reused, e)
	}
}

func TestLoadMapEntitiesReservesFreshSignonSpaceAndSeedsServerFlags(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	s.Static.ServerFlags = 7
	initialSignon := NewMessageBuffer(520)
	for i := 0; i < 500; i++ {
		initialSignon.PutByte(0x42)
	}
	s.Signon = initialSignon
	s.SignonBuffers = []*MessageBuffer{initialSignon}

	vm := newServerTestVM(s, 16)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSWorld), Name: vm.AllocString("world")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
		{Type: uint16(qc.EvString), Ofs: uint16(qc.OFSMapName), Name: vm.AllocString("mapname")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSDeathmatch), Name: vm.AllocString("deathmatch")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSCoop), Name: vm.AllocString("coop")},
		{Type: uint16(qc.EvFloat), Ofs: 90, Name: vm.AllocString("serverflags")},
	}
	const inspectBuiltinOfs = 10
	sawFreshSignon := false
	vm.Builtins[1] = func(vm *qc.VM) {
		sawFreshSignon = s.Signon != initialSignon && s.Signon.Len() == 0
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("worldspawn"), FirstStatement: 2},
		{Name: vm.AllocString("trigger_test"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(inspectBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(inspectBuiltinOfs, -1)
	s.syncQCVMState()
	if got := vm.GlobalFloat("serverflags"); got != 7 {
		t.Fatalf("syncQCVMState serverflags = %v, want 7", got)
	}

	raw := `{
"classname" "worldspawn"
}
{
"classname" "trigger_test"
}`
	if err := s.loadMapEntities(raw); err != nil {
		t.Fatalf("loadMapEntities() error = %v", err)
	}

	if !sawFreshSignon {
		t.Fatalf("spawn QC did not observe fresh signon buffer; buffers=%d currentLen=%d", len(s.SignonBuffers), s.Signon.Len())
	}
	if len(s.SignonBuffers) < 2 {
		t.Fatalf("SignonBuffers = %d, want new buffer allocated before spawn", len(s.SignonBuffers))
	}
}

// TestClearWorldAllocatesAreaNodesWhenMissing tests area node initialization.
// It ensuring the spatial partitioning system is correctly set up for the current world model.
// Where in C: SV_ClearWorld in sv_phys.c
func TestClearWorldAllocatesAreaNodesWhenMissing(t *testing.T) {
	s := NewServer()
	if len(s.Areanodes) != 0 {
		t.Fatalf("initial areanodes len = %d, want 0", len(s.Areanodes))
	}

	s.ClearWorld()

	if got := len(s.Areanodes); got != AreaNodes {
		t.Fatalf("ClearWorld areanodes len = %d, want %d", got, AreaNodes)
	}
	if s.numAreaNodes == 0 {
		t.Fatal("ClearWorld did not build any area nodes")
	}
	root := s.Areanodes[0]
	if root.TriggerEdicts.AreaNext == nil || root.TriggerEdicts.AreaPrev == nil {
		t.Fatal("ClearWorld did not initialize trigger sentinel links")
	}
}

// TestClearWorldBuildsFullAreaNodeTree tests the structure of the area node tree.
// It verifying that the spatial partitioning tree has the correct depth and leaf structure for efficient collision queries.
// Where in C: SV_CreateAreaNode in sv_phys.c
func TestClearWorldBuildsFullAreaNodeTree(t *testing.T) {
	s := NewServer()
	s.ClearWorld()

	expectedNodes := (2 << AreaDepth) - 1
	if got := s.numAreaNodes; got != expectedNodes {
		t.Fatalf("numAreaNodes = %d, want %d", got, expectedNodes)
	}
	if got := len(s.Areanodes); got != AreaNodes {
		t.Fatalf("len(Areanodes) = %d, want %d", got, AreaNodes)
	}

	var visit func(node *AreaNode, depth int) int
	visit = func(node *AreaNode, depth int) int {
		if node == nil {
			t.Fatalf("nil node at depth %d", depth)
		}
		if depth < AreaDepth {
			if node.Axis == -1 {
				t.Fatalf("internal node has leaf axis at depth %d", depth)
			}
			if node.Children[0] == nil || node.Children[1] == nil {
				t.Fatalf("internal node missing child at depth %d", depth)
			}
			return 1 + visit(node.Children[0], depth+1) + visit(node.Children[1], depth+1)
		}
		if node.Axis != -1 {
			t.Fatalf("leaf axis = %d, want -1 at depth %d", node.Axis, depth)
		}
		if node.Children[0] != nil || node.Children[1] != nil {
			t.Fatalf("leaf has children at depth %d", depth)
		}
		return 1
	}

	gotNodes := visit(&s.Areanodes[0], 0)
	if gotNodes != expectedNodes {
		t.Fatalf("reachable area nodes = %d, want %d", gotNodes, expectedNodes)
	}
}

func TestSVMapCheckThreshRequiresMapChecksOrDeveloper(t *testing.T) {
	s := NewServer()
	tests := []struct {
		name      string
		mapChecks string
		developer string
		want      bool
	}{
		{name: "both disabled", mapChecks: "0", developer: "0", want: false},
		{name: "map_checks enabled", mapChecks: "1", developer: "0", want: true},
		{name: "developer enabled", mapChecks: "0", developer: "1", want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if s.CVar.Get("map_checks") == nil {
				s.CVar.Register("map_checks", "0", cvar.FlagNone, "")
			}
			if s.CVar.Get("developer") == nil {
				s.CVar.Register("developer", "0", cvar.FlagNone, "")
			}
			origMapChecks := s.CVar.StringValue("map_checks")
			origDeveloper := s.CVar.StringValue("developer")
			s.CVar.Set("map_checks", tc.mapChecks)
			s.CVar.Set("developer", tc.developer)
			t.Cleanup(func() {
				s.CVar.Set("map_checks", origMapChecks)
				s.CVar.Set("developer", origDeveloper)
			})

			if got := s.SV_MapCheckThresh(123); got != tc.want {
				t.Fatalf("SV_MapCheckThresh() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSVPrintMapCheckNoOpWhenDisabledAndReportsViaTelemetryWhenEnabled(t *testing.T) {
	s := NewServer()
	if s.CVar.Get("map_checks") == nil {
		s.CVar.Register("map_checks", "0", cvar.FlagNone, "")
	}
	if s.CVar.Get("developer") == nil {
		s.CVar.Register("developer", "0", cvar.FlagNone, "")
	}
	origMapChecks := s.CVar.StringValue("map_checks")
	origDeveloper := s.CVar.StringValue("developer")
	t.Cleanup(func() {
		s.CVar.Set("map_checks", origMapChecks)
		s.CVar.Set("developer", origDeveloper)
	})

	s.CVar.Set("map_checks", "0")
	s.CVar.Set("developer", "0")
	if got := s.SV_PrintMapCheck("should not emit"); got {
		t.Fatal("SV_PrintMapCheck should no-op when map checks are disabled")
	}

	lines := make([]string, 0, 2)
	s.DebugTelemetry = NewDebugTelemetryWithConfig(func() DebugTelemetryConfig {
		return DebugTelemetryConfig{Enabled: true, EventMask: debugEventMaskPhysics, EntityFilter: debugEntityFilter{all: true}, SummaryMode: 0}
	}, func(line string) {
		lines = append(lines, line)
	})
	s.CVar.Set("map_checks", "1")
	if got := s.SV_PrintMapCheck("issue %d", 42); !got {
		t.Fatal("SV_PrintMapCheck should report when map checks are enabled")
	}
	if len(lines) == 0 {
		t.Fatal("expected telemetry output for enabled map check")
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "mapcheck issue 42") {
		t.Fatalf("telemetry missing mapcheck payload: %s", joined)
	}

	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	client := s.Static.Clients[0]
	client.Active = true
	client.Loopback = false
	client.NetConnection = inet.NewSocket("LOCAL")
	before := client.Message.Len()
	if !s.SV_PrintMapCheck("client visible") {
		t.Fatal("SV_PrintMapCheck should report to local clients")
	}
	if client.Message.Len() <= before {
		t.Fatal("expected local client print message")
	}
	if !strings.Contains(string(client.Message.Data[:client.Message.Len()]), "client visible") {
		t.Fatalf("client message missing mapcheck text: %q", string(client.Message.Data[:client.Message.Len()]))
	}
}

func TestSVPrintMapChecklistReportsHeaderAndNonEmptyChecks(t *testing.T) {
	s := NewServer()
	if s.CVar.Get("map_checks") == nil {
		s.CVar.Register("map_checks", "0", cvar.FlagNone, "")
	}
	origMapChecks := s.CVar.StringValue("map_checks")
	t.Cleanup(func() { s.CVar.Set("map_checks", origMapChecks) })

	lines := make([]string, 0, 4)
	s.DebugTelemetry = NewDebugTelemetryWithConfig(func() DebugTelemetryConfig {
		return DebugTelemetryConfig{Enabled: true, EventMask: debugEventMaskPhysics, EntityFilter: debugEntityFilter{all: true}, SummaryMode: 0}
	}, func(line string) {
		lines = append(lines, line)
	})

	s.CVar.Set("map_checks", "0")
	if got := s.SV_PrintMapChecklist("header", "a"); got != 0 {
		t.Fatalf("SV_PrintMapChecklist() disabled = %d, want 0", got)
	}

	s.CVar.Set("map_checks", "1")
	if got := s.SV_PrintMapChecklist("Map checklist", "first", "", "second"); got != 3 {
		t.Fatalf("SV_PrintMapChecklist() enabled = %d, want 3", got)
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"mapcheck Map checklist", "mapcheck - first", "mapcheck - second"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in telemetry output: %s", want, joined)
		}
	}
}

func TestSVIsLocalClientTreatsLoopbackAndLocalAddressesAsLocal(t *testing.T) {
	s := NewServer()

	if got := s.SV_IsLocalClient(nil); got {
		t.Fatal("nil client should not be local")
	}

	loopback := &Client{Loopback: true}
	if got := s.SV_IsLocalClient(loopback); !got {
		t.Fatal("loopback client should be local")
	}

	local := &Client{NetConnection: inet.NewSocket("LOCAL")}
	if got := s.SV_IsLocalClient(local); !got {
		t.Fatal("LOCAL address should be local")
	}

	localhost := &Client{NetConnection: inet.NewSocket("localhost")}
	if got := s.SV_IsLocalClient(localhost); !got {
		t.Fatal("localhost address should be local")
	}

	remote := &Client{NetConnection: inet.NewSocket("127.0.0.1:26000")}
	if got := s.SV_IsLocalClient(remote); got {
		t.Fatal("remote address should not be treated as local")
	}
}

func TestLoadMapEntitiesParsesAlphaHack(t *testing.T) {
	s := NewServer()
	vm := newServerTestVM(s, 8)
	s.ClearWorld()
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSWorld), Name: vm.AllocString("world")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("worldspawn"), FirstStatement: 1},
		{Name: vm.AllocString("func_wall"), FirstStatement: 1},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPDone)},
		{Op: uint16(qc.OPDone)},
	}

	raw := `{
"classname" "worldspawn"
}
{
"classname" "func_wall"
"alpha" "0.6"
}`
	if err := s.loadMapEntities(raw); err != nil {
		t.Fatalf("loadMapEntities() error = %v", err)
	}

	edict := s.EdictNum(1)
	if edict == nil {
		t.Fatal("expected entity 1 to be spawned")
	}

	expectedAlpha := inet.ENTALPHA_ENCODE(0.6)
	if edict.Alpha != expectedAlpha {
		t.Fatalf("expected edict.Alpha to be %d, got %d", expectedAlpha, edict.Alpha)
	}

	state, ok := s.entityStateForClient(1, edict)
	if !ok {
		t.Fatal("entityStateForClient failed")
	}
	if state.Alpha != expectedAlpha {
		t.Fatalf("expected state.Alpha to be %d, got %d", expectedAlpha, state.Alpha)
	}
}

