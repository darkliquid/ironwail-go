package server

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/qc"
	"github.com/ironwail/ironwail-go/internal/testutil"
)

func withSkillCVar(t *testing.T, value string) {
	t.Helper()
	if cvar.Get("skill") == nil {
		cvar.Register("skill", "1", cvar.FlagArchive, "")
	}
	original := cvar.StringValue("skill")
	cvar.Set("skill", value)
	t.Cleanup(func() {
		cvar.Set("skill", original)
	})
}

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
			withSkillCVar(t, tc.value)

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

			if got := int(s.QCVM.GetGlobalFloat("skill")); got != tc.want {
				t.Fatalf("QC skill global = %d, want %d", got, tc.want)
			}
		})
	}
}

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
	debugTelemetryEnableCVar = cvar.Register("sv_debug_telemetry_test_spawned_trigger", "1", cvar.FlagNone, "")
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
