// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/qc"
)

// newServerTestVM prepares the server's VM for tests with reasonable defaults.
func newServerTestVM(s *Server, maxEdicts int) *qc.VM {
	vm := s.QCVM
	if vm == nil {
		vm = qc.NewVM()
		s.QCVM = vm
	}
	vm.Globals = make([]float32, 256)
	vm.MaxEdicts = maxEdicts
	vm.NumEdicts = max(s.NumEdicts, maxEdicts)
	vm.EntityFields = 128
	vm.EdictSize = 28 + vm.EntityFields*4
	vm.Edicts = make([]byte, vm.EdictSize*maxEdicts)
	return vm
}

func TestLoadMapEntitiesAllowsEmptyNumericQCFields(t *testing.T) {
	s := NewServer()
	vm := newServerTestVM(s, 8)
	s.ClearWorld()
	vm.FieldDefs = []qc.DDef{
		{Type: uint16(qc.EvString), Ofs: uint16(qc.EntFieldClassName), Name: vm.AllocString("classname")},
		{Type: uint16(qc.EvString), Ofs: uint16(qc.EntFieldModel), Name: vm.AllocString("model")},
		{Type: uint16(qc.EvFloat), Ofs: 90, Name: vm.AllocString("count")},
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("worldspawn"), FirstStatement: 0},
		{Name: vm.AllocString("func_particlefield"), FirstStatement: 1},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPDone)},
		{Op: uint16(qc.OPDone)},
	}

	raw := `{
"classname" "worldspawn"
}
{
"classname" "func_particlefield"
"count" ""
}`

	if err := s.loadMapEntities(raw); err != nil {
		t.Fatalf("loadMapEntities() error = %v", err)
	}

	if got := vm.EFloat(1, 90); got != 0 {
		t.Fatalf("QC count field = %v, want 0", got)
	}
}

func TestServerHooksSpawnAndRemove(t *testing.T) {
	s := NewServer()

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)

	// Call spawn builtin (14)
	if fn := vm.Builtins[14]; fn != nil {
		fn(vm)
	} else {
		t.Fatal("spawn builtin not registered")
	}

	if got := int(vm.GInt(qc.OFSReturn)); got != 1 {
		t.Fatalf("spawn return = %d, want 1", got)
	}
	if s.NumEdicts != 2 {
		t.Fatalf("NumEdicts = %d, want 2", s.NumEdicts)
	}

	// Remove entity via builtin (15)
	vm.SetGInt(qc.OFSParm0, 1)
	if fn := vm.Builtins[15]; fn != nil {
		fn(vm)
	}

	// After removal the VM-backed fields should be cleared
	if got := vm.EFloat(1, qc.EntFieldHealth); got != 0 {
		t.Fatalf("health after remove = %f, want 0", got)
	}
}

func TestServerHooksSpawnClearsQCOnlyFieldsOnReusedEdict(t *testing.T) {
	s := NewServer()

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)

	reused := s.AllocEdict()
	if reused == nil {
		t.Fatal("AllocEdict returned nil")
	}
	entNum := s.NumForEdict(reused)
	vm.SetEFloat(entNum, 110, 123)
	s.FreeEdict(reused)

	if fn := vm.Builtins[14]; fn != nil {
		fn(vm)
	} else {
		t.Fatal("spawn builtin not registered")
	}

	if got := int(vm.GInt(qc.OFSReturn)); got != entNum {
		t.Fatalf("spawn return = %d, want reused edict %d", got, entNum)
	}
	if got := vm.EFloat(entNum, 110); got != 0 {
		t.Fatalf("QC-only field on reused spawned edict = %v, want 0", got)
	}
}

func TestServerHooksSearchAndModelFunctions(t *testing.T) {
	s := NewServer()

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)

	// Prepare multiple entities for search tests
	s.Edicts = []*Edict{
		{},
		{},
		{},
		{},
	}
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = 4
	for entNum, ent := range s.Edicts {
		s.syncEdictToQCVM(entNum, ent)
	}
	vm.SetEInt(1, qc.EntFieldTargetName, vm.AllocString("door"))
	vm.SetEVector(1, qc.EntFieldOrigin, [3]float32{100, 0, 0})
	vm.SetEInt(2, qc.EntFieldTargetName, vm.AllocString("trigger"))
	vm.SetEFloat(2, qc.EntFieldHealth, 100)
	vm.SetEFloat(2, qc.EntFieldSolid, float32(SolidBBox))
	vm.SetEVector(2, qc.EntFieldOrigin, [3]float32{10, 0, 0})
	vm.SetEFloat(3, qc.EntFieldSolid, float32(SolidBBox))
	vm.SetEVector(3, qc.EntFieldOrigin, [3]float32{40, 0, 0})

	// find by string (canonical builtin 18)
	vm.SetGInt(qc.OFSParm0, 0)
	vm.SetGInt(qc.OFSParm1, qc.EntFieldTargetName)
	vm.SetGString(qc.OFSParm2, "trigger")
	if fn := vm.Builtins[18]; fn != nil {
		fn(vm)
	}
	if got := int(vm.GInt(qc.OFSReturn)); got != 2 {
		t.Fatalf("find return = %d, want 2", got)
	}

	// findfloat (temporary non-canonical helper slot)
	vm.SetGInt(qc.OFSParm0, 0)
	vm.SetGInt(qc.OFSParm1, qc.EntFieldHealth)
	vm.SetGFloat(qc.OFSParm2, 100)
	if fn := vm.Builtins[1000]; fn != nil {
		fn(vm)
	}
	if got := int(vm.GInt(qc.OFSReturn)); got != 2 {
		t.Fatalf("findfloat return = %d, want 2", got)
	}

	// nextent (canonical builtin 47)
	vm.SetGInt(qc.OFSParm0, 1)
	if fn := vm.Builtins[47]; fn != nil {
		fn(vm)
	}
	if got := int(vm.GInt(qc.OFSReturn)); got != 2 {
		t.Fatalf("nextent return = %d, want 2", got)
	}

	// findradius (canonical builtin 22)
	vm.SetGVector(qc.OFSParm0, [3]float32{0, 0, 0})
	vm.SetGFloat(qc.OFSParm1, 50)
	if fn := vm.Builtins[22]; fn != nil {
		fn(vm)
	}
	if got := int(vm.GInt(qc.OFSReturn)); got != 3 {
		t.Fatalf("findradius return = %d, want 3", got)
	}
	if got := int(vm.EInt(3, qc.EntFieldChain)); got != 2 {
		t.Fatalf("findradius chain head = %d, want 2", got)
	}
	if got := int(vm.EInt(2, qc.EntFieldChain)); got != 0 {
		t.Fatalf("findradius chain tail = %d, want 0", got)
	}

	// setmodel
	s.ModelPrecache = make([]string, MaxModels)
	s.ModelPrecache[1] = "progs/test.mdl"
	vm.SetGInt(qc.OFSParm0, 1)
	vm.SetGString(qc.OFSParm1, "progs/test.mdl")
	if fn := vm.Builtins[3]; fn != nil {
		fn(vm)
	}
	modelIdx := vm.EInt(1, qc.EntFieldModel)
	if got := vm.String(modelIdx); got != "progs/test.mdl" {
		t.Fatalf("model string = %q", got)
	}
	if got := vm.EFloat(1, qc.EntFieldModelIndex); got != 1 {
		t.Fatalf("modelindex = %v, want 1", got)
	}
}

func TestServerHooksSearchFunctionsSkipFreedEdicts(t *testing.T) {
	s := NewServer()

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)

	s.Edicts = []*Edict{
		{},
		{},
		{Free: true},
		{},
	}
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts
	for entNum, ent := range s.Edicts {
		s.syncEdictToQCVM(entNum, ent)
	}

	vm.SetEInt(2, qc.EntFieldTargetName, vm.AllocString("tele_dest"))
	vm.SetEFloat(2, qc.EntFieldHealth, 100)
	vm.SetEInt(3, qc.EntFieldTargetName, vm.AllocString("tele_dest"))
	vm.SetEFloat(3, qc.EntFieldHealth, 100)

	vm.SetGInt(qc.OFSParm0, 0)
	vm.SetGInt(qc.OFSParm1, qc.EntFieldTargetName)
	vm.SetGString(qc.OFSParm2, "tele_dest")
	if fn := vm.Builtins[18]; fn != nil {
		fn(vm)
	}
	if got := int(vm.GInt(qc.OFSReturn)); got != 3 {
		t.Fatalf("find return = %d, want 3", got)
	}

	vm.SetGInt(qc.OFSParm0, 0)
	vm.SetGInt(qc.OFSParm1, qc.EntFieldHealth)
	vm.SetGFloat(qc.OFSParm2, 100)
	if fn := vm.Builtins[1000]; fn != nil {
		fn(vm)
	}
	if got := int(vm.GInt(qc.OFSReturn)); got != 3 {
		t.Fatalf("findfloat return = %d, want 3", got)
	}

	vm.SetGInt(qc.OFSParm0, 1)
	if fn := vm.Builtins[47]; fn != nil {
		fn(vm)
	}
	if got := int(vm.GInt(qc.OFSReturn)); got != 3 {
		t.Fatalf("nextent return = %d, want 3", got)
	}
}

func TestServerHooksCheckBottomSyncsEntityFromQCVM(t *testing.T) {
	s := NewServer()

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)
	s.WorldModel = CreateSyntheticWorldModel()
	s.Edicts[0].SetSolid(s, float32(SolidBSP))
	s.ClearWorld()

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("AllocEdict returned nil")
	}
	ent.SetMins(s, [3]float32{-16, -16, -24})
	ent.SetMaxs(s, [3]float32{16, 16, 32})
	ent.SetSolid(s, float32(SolidSlideBox))
	ent.SetOrigin(s, [3]float32{0, 0, 24})
	s.LinkEdict(ent, false)

	entNum := s.NumForEdict(ent)
	vm.NumEdicts = s.NumEdicts
	s.syncEdictToQCVM(entNum, ent)
	vm.SetEVector(entNum, qc.EntFieldOrigin, [3]float32{0, 0, 128})
	vm.SetEVector(entNum, qc.EntFieldAbsMin, [3]float32{-16, -16, 104})
	vm.SetEVector(entNum, qc.EntFieldAbsMax, [3]float32{16, 16, 160})

	vm.SetGInt(qc.OFSParm0, int32(entNum))
	if fn := vm.Builtins[40]; fn == nil {
		t.Fatal("checkbottom builtin not registered")
	} else {
		fn(vm)
	}

	if got := vm.GFloat(qc.OFSReturn); got != 0 {
		t.Fatalf("checkbottom return = %v, want 0 after QC moved entity off ground", got)
	}
}

func TestServerHooksSetModelUsesBrushBounds(t *testing.T) {
	s := NewServer()

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("failed to alloc edict")
	}
	vm.NumEdicts = s.NumEdicts
	s.syncEdictToQCVM(s.NumForEdict(ent), ent)
	vm.SetEVector(s.NumForEdict(ent), qc.EntFieldOrigin, [3]float32{64, 32, 16})
	s.ClearWorld()

	s.ModelName = "maps/test.bsp"
	s.ModelPrecache = make([]string, MaxModels)
	s.ModelPrecache[1] = s.ModelName
	s.ModelPrecache[2] = "*1"
	s.WorldTree = &bsp.Tree{Models: []bsp.DModel{
		{BoundsMin: [3]float32{-256, -256, -128}, BoundsMax: [3]float32{256, 256, 128}},
		{BoundsMin: [3]float32{-16, -24, -32}, BoundsMax: [3]float32{48, 56, 72}},
	}}
	s.WorldModel = worldModelFromBSPTree(s.ModelName, s.WorldTree)

	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(ent)))
	vm.SetGString(qc.OFSParm1, "*1")
	if fn := vm.Builtins[3]; fn == nil {
		t.Fatal("setmodel builtin not registered")
	} else {
		fn(vm)
	}

	if got := vm.EFloat(1, qc.EntFieldModelIndex); got != 2 {
		t.Fatalf("modelindex = %v, want 2", got)
	}
	if got := vm.String(vm.EInt(1, qc.EntFieldModel)); got != "*1" {
		t.Fatalf("model string = %q, want *1", got)
	}
	if got := vm.EVector(1, qc.EntFieldMins); got != [3]float32{-16, -24, -32} {
		t.Fatalf("mins = %v", got)
	}
	if got := vm.EVector(1, qc.EntFieldMaxs); got != [3]float32{48, 56, 72} {
		t.Fatalf("maxs = %v", got)
	}
	if got := vm.EVector(1, qc.EntFieldSize); got != [3]float32{64, 80, 104} {
		t.Fatalf("size = %v", got)
	}
	if got := vm.EVector(1, qc.EntFieldAbsMin); got != [3]float32{47, 7, -17} {
		t.Fatalf("absmin = %v", got)
	}
	if got := vm.EVector(1, qc.EntFieldAbsMax); got != [3]float32{113, 89, 89} {
		t.Fatalf("absmax = %v", got)
	}
}

func TestServerHooksSetModelRequiresPrecache(t *testing.T) {
	s := NewServer()

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)
	_ = s.AllocEdict()
	vm.NumEdicts = s.NumEdicts

	vm.SetGInt(qc.OFSParm0, 1)
	vm.SetGString(qc.OFSParm1, "progs/missing.mdl")
	vm.Builtins[3](vm)
	if vm.BuiltinError == nil {
		t.Fatal("setmodel did not raise runtime error for non-precached model")
	}
}

func TestServerHooksSetOriginImportsPendingQCBoundsForLink(t *testing.T) {
	s := NewServer()

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("failed to alloc edict")
	}
	entNum := s.NumForEdict(ent)
	vm.NumEdicts = s.NumEdicts
	s.ClearWorld()

	ent.SetOrigin(s, [3]float32{0, 0, 0})
	ent.SetMins(s, [3]float32{-1, -1, -1})
	ent.SetMaxs(s, [3]float32{1, 1, 1})
	s.syncEdictToQCVM(entNum, ent)

	vm.SetEVector(entNum, qc.EntFieldMins, [3]float32{-16, -8, -4})
	vm.SetEVector(entNum, qc.EntFieldMaxs, [3]float32{16, 8, 12})
	vm.SetGInt(qc.OFSParm0, int32(entNum))
	vm.SetGVector(qc.OFSParm1, [3]float32{100, 50, 25})
	if fn := vm.Builtins[2]; fn == nil {
		t.Fatal("setorigin builtin not registered")
	} else {
		fn(vm)
	}

	if got := ent.AbsMin(s); got != [3]float32{83, 41, 20} {
		t.Fatalf("server absmin = %v, want bounds from QC mins with link expansion", got)
	}
	if got := ent.AbsMax(s); got != [3]float32{117, 59, 38} {
		t.Fatalf("server absmax = %v, want bounds from QC maxs with link expansion", got)
	}
}

func TestServerHooksSetSizeImportsPendingQCOriginForLink(t *testing.T) {
	s := NewServer()

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("failed to alloc edict")
	}
	entNum := s.NumForEdict(ent)
	vm.NumEdicts = s.NumEdicts
	s.ClearWorld()

	ent.SetOrigin(s, [3]float32{0, 0, 0})
	ent.SetMins(s, [3]float32{-1, -1, -1})
	ent.SetMaxs(s, [3]float32{1, 1, 1})
	s.syncEdictToQCVM(entNum, ent)

	vm.SetEVector(entNum, qc.EntFieldOrigin, [3]float32{200, 20, 8})
	vm.SetGInt(qc.OFSParm0, int32(entNum))
	vm.SetGVector(qc.OFSParm1, [3]float32{-16, -16, -24})
	vm.SetGVector(qc.OFSParm2, [3]float32{16, 16, 32})
	if fn := vm.Builtins[4]; fn == nil {
		t.Fatal("setsize builtin not registered")
	} else {
		fn(vm)
	}

	if got := ent.AbsMin(s); got != [3]float32{183, 3, -17} {
		t.Fatalf("server absmin = %v, want bounds from QC origin with link expansion", got)
	}
	if got := ent.AbsMax(s); got != [3]float32{217, 37, 41} {
		t.Fatalf("server absmax = %v, want bounds from QC origin with link expansion", got)
	}
}

func TestServerHooksSetModelImportsPendingQCOriginForLink(t *testing.T) {
	s := NewServer()

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("failed to alloc edict")
	}
	entNum := s.NumForEdict(ent)
	vm.NumEdicts = s.NumEdicts
	s.ClearWorld()

	ent.SetOrigin(s, [3]float32{0, 0, 0})
	s.syncEdictToQCVM(entNum, ent)
	vm.SetEVector(entNum, qc.EntFieldOrigin, [3]float32{64, 32, 16})

	s.ModelName = "maps/test.bsp"
	s.ModelPrecache = make([]string, MaxModels)
	s.ModelPrecache[1] = s.ModelName
	s.ModelPrecache[2] = "*1"
	s.WorldTree = &bsp.Tree{Models: []bsp.DModel{
		{BoundsMin: [3]float32{-256, -256, -128}, BoundsMax: [3]float32{256, 256, 128}},
		{BoundsMin: [3]float32{-16, -24, -32}, BoundsMax: [3]float32{48, 56, 72}},
	}}
	s.WorldModel = worldModelFromBSPTree(s.ModelName, s.WorldTree)

	vm.SetGInt(qc.OFSParm0, int32(entNum))
	vm.SetGString(qc.OFSParm1, "*1")
	if fn := vm.Builtins[3]; fn == nil {
		t.Fatal("setmodel builtin not registered")
	} else {
		fn(vm)
	}

	if got := vm.EVector(entNum, qc.EntFieldAbsMin); got != [3]float32{47, 7, -17} {
		t.Fatalf("absmin = %v, want bounds linked from QC-pending origin", got)
	}
	if got := vm.EVector(entNum, qc.EntFieldAbsMax); got != [3]float32{113, 89, 89} {
		t.Fatalf("absmax = %v, want bounds linked from QC-pending origin", got)
	}
}

func TestServerHooksWalkMoveAndDropToFloor(t *testing.T) {
	s := NewServer()
	s.WorldModel = CreateSyntheticWorldModel()
	if world := s.EdictNum(0); world != nil {
		world.SetSolid(s, float32(SolidBSP))
	}
	s.ClearWorld()

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("AllocEdict returned nil")
	}
	entNum := s.NumForEdict(ent)
	vm.NumEdicts = s.NumEdicts

	ent.SetOrigin(s, [3]float32{0, 0, 24})
	ent.SetMins(s, [3]float32{-16, -16, -24})
	ent.SetMaxs(s, [3]float32{16, 16, 32})
	ent.SetSolid(s, float32(SolidSlideBox))
	ent.SetFlags(s, float32(FlagOnGround))
	s.LinkEdict(ent, false)
	s.syncEdictToQCVM(entNum, ent)
	vm.SetGInt(qc.OFSSelf, int32(entNum))

	// Walk forward 10 units at yaw=0
	vm.SetGFloat(qc.OFSParm0, 0)
	vm.SetGFloat(qc.OFSParm1, 10)
	if fn := vm.Builtins[32]; fn != nil {
		fn(vm)
	}
	if got := vm.EVector(entNum, qc.EntFieldOrigin); got[0] == 0 && got[1] == 0 {
		t.Fatalf("walkmove did not change origin: %v", got)
	}

	ent.SetOrigin(s, [3]float32{0, 0, 96})
	ent.SetFlags(s, 0)
	ent.SetGroundEntity(s, 0)
	s.LinkEdict(ent, false)
	s.syncEdictToQCVM(entNum, ent)
	if fn := vm.Builtins[34]; fn != nil {
		fn(vm)
	}
	if got := vm.EVector(entNum, qc.EntFieldOrigin); got[2] < 23.99 || got[2] > 24.05 {
		t.Fatalf("droptofloor origin.z = %v, want ~24", got[2])
	}
	if got := uint32(vm.EFloat(entNum, qc.EntFieldFlags)); got&FlagOnGround == 0 {
		t.Fatalf("droptofloor flags = %#x, want onground set", got)
	}
	if got := vm.EInt(entNum, qc.EntFieldGroundEnt); got != 0 {
		t.Fatalf("droptofloor groundentity = %d, want world 0", got)
	}
}

func TestServerHooksWalkMoveImportsQCStateWithoutStepDirectionYawMutation(t *testing.T) {
	s := NewServer()
	s.WorldModel = CreateSyntheticWorldModel()
	if world := s.EdictNum(0); world != nil {
		world.SetSolid(s, float32(SolidBSP))
	}
	s.ClearWorld()

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("AllocEdict returned nil")
	}
	entNum := s.NumForEdict(ent)
	vm.NumEdicts = s.NumEdicts

	ent.SetOrigin(s, [3]float32{0, 0, 24})
	ent.SetMins(s, [3]float32{-16, -16, -24})
	ent.SetMaxs(s, [3]float32{16, 16, 32})
	ent.SetSolid(s, float32(SolidSlideBox))
	angles := ent.Angles(s)
	angles[1] = 45
	ent.SetAngles(s, angles)
	ent.SetIdealYaw(s, 123)
	s.syncEdictToQCVM(entNum, ent)
	vm.SetEFloat(entNum, qc.EntFieldFlags, float32(FlagOnGround))
	vm.SetEVector(entNum, qc.EntFieldAngles, [3]float32{0, 33, 0})
	vm.SetEFloat(entNum, qc.EntFieldIdealYaw, 77)
	s.LinkEdict(ent, false)

	vm.SetGInt(qc.OFSSelf, int32(entNum))
	vm.SetGFloat(qc.OFSParm0, 0)
	vm.SetGFloat(qc.OFSParm1, 10)
	if fn := vm.Builtins[32]; fn == nil {
		t.Fatal("walkmove builtin not registered")
	} else {
		fn(vm)
	}

	if got := vm.EVector(entNum, qc.EntFieldOrigin); got[0] <= 0 {
		t.Fatalf("walkmove did not use QC-only onground flag: origin=%v", got)
	}
	if got := vm.EFloat(entNum, qc.EntFieldIdealYaw); got != 77 {
		t.Fatalf("ideal_yaw = %v, want QC-only value 77", got)
	}
	if got := vm.EVector(entNum, qc.EntFieldAngles); got[1] != 33 {
		t.Fatalf("angles yaw = %v, want QC-only value 33 without StepDirection yaw mutation", got[1])
	}
}

func TestServerHooksDropToFloorImportsPendingQCState(t *testing.T) {
	s := NewServer()
	s.WorldModel = CreateSyntheticWorldModel()
	if world := s.EdictNum(0); world != nil {
		world.SetSolid(s, float32(SolidBSP))
	}
	s.ClearWorld()

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("AllocEdict returned nil")
	}
	entNum := s.NumForEdict(ent)
	vm.NumEdicts = s.NumEdicts

	ent.SetOrigin(s, [3]float32{0, 0, 96})
	ent.SetMins(s, [3]float32{-16, -16, -24})
	ent.SetMaxs(s, [3]float32{16, 16, 32})
	ent.SetSolid(s, float32(SolidSlideBox))
	s.LinkEdict(ent, false)
	s.syncEdictToQCVM(entNum, ent)

	vm.SetEVector(entNum, qc.EntFieldMins, [3]float32{-16, -16, -8})
	vm.SetEVector(entNum, qc.EntFieldMaxs, [3]float32{16, 16, 8})
	vm.SetGInt(qc.OFSSelf, int32(entNum))

	if fn := vm.Builtins[34]; fn == nil {
		t.Fatal("droptofloor builtin not registered")
	} else {
		fn(vm)
	}

	if got := vm.EVector(entNum, qc.EntFieldOrigin); got[2] < 7.99 || got[2] > 8.05 {
		t.Fatalf("droptofloor origin.z = %v, want ~8 from QC-only mins", got[2])
	}
}

func TestServerHooksWalkMoveRequiresMovementFlags(t *testing.T) {
	s := NewServer()

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("AllocEdict returned nil")
	}
	entNum := s.NumForEdict(ent)
	vm.NumEdicts = s.NumEdicts

	ent.SetOrigin(s, [3]float32{0, 0, 0})
	ent.SetMins(s, [3]float32{-16, -16, -24})
	ent.SetMaxs(s, [3]float32{16, 16, 32})
	ent.SetSolid(s, float32(SolidSlideBox))
	s.syncEdictToQCVM(entNum, ent)

	vm.SetGInt(qc.OFSSelf, int32(entNum))
	vm.SetGFloat(qc.OFSParm0, 0)
	vm.SetGFloat(qc.OFSParm1, 10)
	if fn := vm.Builtins[32]; fn == nil {
		t.Fatal("walkmove builtin not registered")
	} else {
		fn(vm)
	}

	if got := vm.EVector(entNum, qc.EntFieldOrigin); got != [3]float32{0, 0, 0} {
		t.Fatalf("walkmove changed origin without movement flags: %v", got)
	}
}

func TestServerHooksWalkMoveRestoresQCContextAfterNestedTouch(t *testing.T) {
	s := NewServer()
	s.WorldModel = CreateSyntheticWorldModel()
	if world := s.EdictNum(0); world != nil {
		world.SetSolid(s, float32(SolidBSP))
	}
	s.ClearWorld()

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("touch_callback"), FirstStatement: 0},
		{Name: vm.AllocString("outer_qc_func"), FirstStatement: 1},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPDone)},
		{Op: uint16(qc.OPDone)},
	}

	mover := s.AllocEdict()
	trigger := s.AllocEdict()
	if mover == nil || trigger == nil {
		t.Fatal("failed to allocate edicts")
	}
	vm.NumEdicts = s.NumEdicts

	moverNum := s.NumForEdict(mover)
	mover.SetOrigin(s, [3]float32{0, 0, 24})
	mover.SetMins(s, [3]float32{-16, -16, -24})
	mover.SetMaxs(s, [3]float32{16, 16, 32})
	mover.SetSolid(s, float32(SolidSlideBox))
	mover.SetFlags(s, float32(FlagOnGround))

	trigger.SetOrigin(s, [3]float32{24, 0, 24})
	trigger.SetMins(s, [3]float32{-16, -16, -24})
	trigger.SetMaxs(s, [3]float32{16, 16, 32})
	trigger.SetSolid(s, float32(SolidTrigger))
	s.QCVM.SetEInt(trigger.Num, qc.EntFieldTouch, 1)

	s.LinkEdict(mover, false)
	s.LinkEdict(trigger, false)
	s.syncEdictToQCVM(moverNum, mover)
	s.syncEdictToQCVM(s.NumForEdict(trigger), trigger)

	vm.SetGInt(qc.OFSSelf, int32(moverNum))
	vm.SetGInt(qc.OFSOther, 77)
	vm.XFunction = &vm.Functions[2]
	vm.XFunctionIndex = 2
	vm.SetGFloat(qc.OFSParm0, 0)
	vm.SetGFloat(qc.OFSParm1, 24)
	if fn := vm.Builtins[32]; fn == nil {
		t.Fatal("walkmove builtin not registered")
	} else {
		fn(vm)
	}

	if got := vm.GInt(qc.OFSSelf); got != int32(moverNum) {
		t.Fatalf("self after nested walkmove = %d, want %d", got, moverNum)
	}
	if got := vm.GInt(qc.OFSOther); got != 77 {
		t.Fatalf("other after nested walkmove = %d, want 77", got)
	}
	if vm.XFunction != &vm.Functions[2] || vm.XFunctionIndex != 2 {
		t.Fatalf("qc context not restored: xfunction=%p idx=%d", vm.XFunction, vm.XFunctionIndex)
	}
}
