// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/qc"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
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

	entities := `
{
"classname" "worldspawn"
}
{
"classname" "func_particlefield"
"model" "*1"
"count" ""
}
`
	if err := s.loadMapEntities(entities); err != nil {
		t.Fatalf("loadMapEntities unexpectedly failed on empty count field: %v", err)
	}

	if s.NumEdicts != 2 {
		t.Fatalf("NumEdicts = %d, want 2", s.NumEdicts)
	}
	if got := vm.EFloat(1, 90); got != 0 {
		t.Fatalf("count field value = %v, want 0", got)
	}
}

func TestLoadMapEntitiesPreservesWorldEdictFieldsAcrossReuse(t *testing.T) {
	s := NewServer()
	vm := newServerTestVM(s, 8)
	s.ClearWorld()
	vm.FieldDefs = []qc.DDef{
		{Type: uint16(qc.EvString), Ofs: uint16(qc.EntFieldClassName), Name: vm.AllocString("classname")},
		{Type: uint16(qc.EvFloat), Ofs: 90, Name: vm.AllocString("custom_field")},
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("worldspawn"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPDone)},
	}

	// First load sets a QC-only field on worldspawn
	if err := s.loadMapEntities("{\n\"classname\" \"worldspawn\"\n\"custom_field\" \"42\"\n}\n"); err != nil {
		t.Fatalf("first loadMapEntities failed: %v", err)
	}
	if got := vm.EFloat(0, 90); got != 42 {
		t.Fatalf("world custom_field after first load = %v, want 42", got)
	}

	// Second load without custom_field must clear the field from worldspawn (edict 0)
	clearQCVMEdictData(vm, 0)
	if err := s.loadMapEntities("{\n\"classname\" \"worldspawn\"\n}\n"); err != nil {
		t.Fatalf("second loadMapEntities failed: %v", err)
	}
	if got := vm.EFloat(0, 90); got != 0 {
		t.Fatalf("world custom_field after second load = %v, want 0 (should be cleared)", got)
	}
}

func TestLoadMapEntitiesReusesSpawnedEdictState(t *testing.T) {
	s := NewServer()
	vm := newServerTestVM(s, 8)
	s.ClearWorld()
	vm.FieldDefs = []qc.DDef{
		{Type: uint16(qc.EvString), Ofs: uint16(qc.EntFieldClassName), Name: vm.AllocString("classname")},
		{Type: uint16(qc.EvFloat), Ofs: 90, Name: vm.AllocString("custom_field")},
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("worldspawn"), FirstStatement: 0},
		{Name: vm.AllocString("info_null"), FirstStatement: 1},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPDone)},
		{Op: uint16(qc.OPDone)},
	}

	// Spawn an edict with a custom field
	if err := s.loadMapEntities("{\n\"classname\" \"worldspawn\"\n}\n{\n\"classname\" \"info_null\"\n\"custom_field\" \"42\"\n}\n"); err != nil {
		t.Fatalf("first loadMapEntities: %v", err)
	}
	if got := vm.EFloat(1, 90); got != 42 {
		t.Fatalf("custom_field = %v, want 42", got)
	}

	// Next load without custom_field must clear the reused edict
	s.Edicts = s.Edicts[:1]
	s.NumEdicts = 1
	if err := s.loadMapEntities("{\n\"classname\" \"worldspawn\"\n}\n{\n\"classname\" \"info_null\"\n}\n"); err != nil {
		t.Fatalf("second loadMapEntities: %v", err)
	}
	if got := vm.EFloat(1, 90); got != 0 {
		t.Fatalf("QC-only field on reused spawned edict = %v, want 0", got)
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
	vm.SetEInt(1, qc.EntFieldTargetName, vm.AllocString("door"))
	vm.SetEVector(1, qc.EntFieldOrigin, qtypes.Vec3{X: 100, Y: 0, Z: 0})
	vm.SetEInt(2, qc.EntFieldTargetName, vm.AllocString("trigger"))
	vm.SetEFloat(2, qc.EntFieldHealth, 100)
	vm.SetEFloat(2, qc.EntFieldSolid, float32(SolidBBox))
	vm.SetEVector(2, qc.EntFieldOrigin, qtypes.Vec3{X: 10, Y: 0, Z: 0})
	vm.SetEFloat(3, qc.EntFieldSolid, float32(SolidBBox))
	vm.SetEVector(3, qc.EntFieldOrigin, qtypes.Vec3{X: 40, Y: 0, Z: 0})

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
	vm.SetGVector(qc.OFSParm0, qtypes.Vec3{})
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
	ent.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	ent.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	ent.SetSolid(s, float32(SolidSlideBox))
	ent.SetOrigin(s, qtypes.Vec3{X: 0, Y: 0, Z: 24})
	s.LinkEdict(ent, false)

	entNum := s.NumForEdict(ent)
	vm.NumEdicts = s.NumEdicts
	vm.SetEVector(entNum, qc.EntFieldOrigin, qtypes.Vec3{X: 0, Y: 0, Z: 128})
	vm.SetEVector(entNum, qc.EntFieldAbsMin, qtypes.Vec3{X: -16, Y: -16, Z: 104})
	vm.SetEVector(entNum, qc.EntFieldAbsMax, qtypes.Vec3{X: 16, Y: 16, Z: 160})

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
	vm.SetEVector(s.NumForEdict(ent), qc.EntFieldOrigin, qtypes.Vec3{X: 64, Y: 32, Z: 16})
	s.ClearWorld()

	s.ModelName = "maps/test.bsp"
	s.ModelPrecache = make([]string, MaxModels)
	s.ModelPrecache[1] = s.ModelName
	s.ModelPrecache[2] = "*1"
	s.WorldTree = &bsp.Tree{Models: []bsp.DModel{
		{BoundsMin: qtypes.Vec3{X: -256, Y: -256, Z: -128}, BoundsMax: qtypes.Vec3{X: 256, Y: 256, Z: 128}},
		{BoundsMin: qtypes.Vec3{X: -16, Y: -24, Z: -32}, BoundsMax: qtypes.Vec3{X: 48, Y: 56, Z: 72}},
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
	if got := vm.EVector(1, qc.EntFieldMins); got != (qtypes.Vec3{X: -16, Y: -24, Z: -32}) {
		t.Fatalf("mins = %v", got)
	}
	if got := vm.EVector(1, qc.EntFieldMaxs); got != (qtypes.Vec3{X: 48, Y: 56, Z: 72}) {
		t.Fatalf("maxs = %v", got)
	}
	if got := vm.EVector(1, qc.EntFieldSize); got != (qtypes.Vec3{X: 64, Y: 80, Z: 104}) {
		t.Fatalf("size = %v", got)
	}
	if got := vm.EVector(1, qc.EntFieldAbsMin); got != (qtypes.Vec3{X: 47, Y: 7, Z: -17}) {
		t.Fatalf("absmin = %v", got)
	}
	if got := vm.EVector(1, qc.EntFieldAbsMax); got != (qtypes.Vec3{X: 113, Y: 89, Z: 89}) {
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

	ent.SetOrigin(s, qtypes.Vec3{})
	ent.SetMins(s, qtypes.Vec3{X: -1, Y: -1, Z: -1})
	ent.SetMaxs(s, qtypes.Vec3{X: 1, Y: 1, Z: 1})

	vm.SetEVector(entNum, qc.EntFieldMins, qtypes.Vec3{X: -16, Y: -8, Z: -4})
	vm.SetEVector(entNum, qc.EntFieldMaxs, qtypes.Vec3{X: 16, Y: 8, Z: 12})
	vm.SetGInt(qc.OFSParm0, int32(entNum))
	vm.SetGVector(qc.OFSParm1, qtypes.Vec3{X: 100, Y: 50, Z: 25})
	if fn := vm.Builtins[2]; fn == nil {
		t.Fatal("setorigin builtin not registered")
	} else {
		fn(vm)
	}

	if got := ent.AbsMin(s); got != (qtypes.Vec3{X: 83, Y: 41, Z: 20}) {
		t.Fatalf("server absmin = %v, want bounds from QC mins with link expansion", got)
	}
	if got := ent.AbsMax(s); got != (qtypes.Vec3{X: 117, Y: 59, Z: 38}) {
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

	ent.SetOrigin(s, qtypes.Vec3{})
	ent.SetMins(s, qtypes.Vec3{X: -1, Y: -1, Z: -1})
	ent.SetMaxs(s, qtypes.Vec3{X: 1, Y: 1, Z: 1})

	vm.SetEVector(entNum, qc.EntFieldOrigin, qtypes.Vec3{X: 200, Y: 20, Z: 8})
	vm.SetGInt(qc.OFSParm0, int32(entNum))
	vm.SetGVector(qc.OFSParm1, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	vm.SetGVector(qc.OFSParm2, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	if fn := vm.Builtins[4]; fn == nil {
		t.Fatal("setsize builtin not registered")
	} else {
		fn(vm)
	}

	if got := ent.AbsMin(s); got != (qtypes.Vec3{X: 183, Y: 3, Z: -17}) {
		t.Fatalf("server absmin = %v, want bounds from QC origin with link expansion", got)
	}
	if got := ent.AbsMax(s); got != (qtypes.Vec3{X: 217, Y: 37, Z: 41}) {
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

	ent.SetOrigin(s, qtypes.Vec3{})
	vm.SetEVector(entNum, qc.EntFieldOrigin, qtypes.Vec3{X: 64, Y: 32, Z: 16})

	s.ModelName = "maps/test.bsp"
	s.ModelPrecache = make([]string, MaxModels)
	s.ModelPrecache[1] = s.ModelName
	s.ModelPrecache[2] = "*1"
	s.WorldTree = &bsp.Tree{Models: []bsp.DModel{
		{BoundsMin: qtypes.Vec3{X: -256, Y: -256, Z: -128}, BoundsMax: qtypes.Vec3{X: 256, Y: 256, Z: 128}},
		{BoundsMin: qtypes.Vec3{X: -16, Y: -24, Z: -32}, BoundsMax: qtypes.Vec3{X: 48, Y: 56, Z: 72}},
	}}
	s.WorldModel = worldModelFromBSPTree(s.ModelName, s.WorldTree)

	vm.SetGInt(qc.OFSParm0, int32(entNum))
	vm.SetGString(qc.OFSParm1, "*1")
	if fn := vm.Builtins[3]; fn == nil {
		t.Fatal("setmodel builtin not registered")
	} else {
		fn(vm)
	}

	if got := vm.EVector(entNum, qc.EntFieldAbsMin); got != (qtypes.Vec3{X: 47, Y: 7, Z: -17}) {
		t.Fatalf("absmin = %v, want bounds linked from QC-pending origin", got)
	}
	if got := vm.EVector(entNum, qc.EntFieldAbsMax); got != (qtypes.Vec3{X: 113, Y: 89, Z: 89}) {
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

	ent.SetOrigin(s, qtypes.Vec3{X: 0, Y: 0, Z: 24})
	ent.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	ent.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	ent.SetSolid(s, float32(SolidSlideBox))
	ent.SetFlags(s, float32(FlagOnGround))
	s.LinkEdict(ent, false)
	vm.SetGInt(qc.OFSSelf, int32(entNum))

	// Walk forward 10 units at yaw=0
	vm.SetGFloat(qc.OFSParm0, 0)
	vm.SetGFloat(qc.OFSParm1, 10)
	if fn := vm.Builtins[32]; fn != nil {
		fn(vm)
	}
	if got := vm.EVector(entNum, qc.EntFieldOrigin); got.X == 0 && got.Y == 0 {
		t.Fatalf("walkmove did not change origin: %v", got)
	}

	ent.SetOrigin(s, qtypes.Vec3{X: 0, Y: 0, Z: 96})
	ent.SetFlags(s, 0)
	ent.SetGroundEntity(s, 0)
	s.LinkEdict(ent, false)
	if fn := vm.Builtins[34]; fn != nil {
		fn(vm)
	}
	if got := vm.EVector(entNum, qc.EntFieldOrigin); got.Z < 23.99 || got.Z > 24.05 {
		t.Fatalf("droptofloor origin.z = %v, want ~24", got.Z)
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

	ent.SetOrigin(s, qtypes.Vec3{X: 0, Y: 0, Z: 24})
	ent.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	ent.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	ent.SetSolid(s, float32(SolidSlideBox))
	angles := ent.Angles(s)
	angles.Y = 45
	ent.SetAngles(s, angles)
	ent.SetIdealYaw(s, 123)
	vm.SetEFloat(entNum, qc.EntFieldFlags, float32(FlagOnGround))
	vm.SetEVector(entNum, qc.EntFieldAngles, qtypes.Vec3{X: 0, Y: 33, Z: 0})
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

	if got := vm.EVector(entNum, qc.EntFieldOrigin); got.X <= 0 {
		t.Fatalf("walkmove did not use QC-only onground flag: origin=%v", got)
	}
	if got := vm.EFloat(entNum, qc.EntFieldIdealYaw); got != 77 {
		t.Fatalf("ideal_yaw = %v, want QC-only value 77", got)
	}
	if got := vm.EVector(entNum, qc.EntFieldAngles); got.Y != 33 {
		t.Fatalf("angles yaw = %v, want QC-only value 33 without StepDirection yaw mutation", got.Y)
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

	ent.SetOrigin(s, qtypes.Vec3{X: 0, Y: 0, Z: 96})
	ent.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	ent.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	ent.SetSolid(s, float32(SolidSlideBox))
	s.LinkEdict(ent, false)

	vm.SetEVector(entNum, qc.EntFieldMins, qtypes.Vec3{X: -16, Y: -16, Z: -8})
	vm.SetEVector(entNum, qc.EntFieldMaxs, qtypes.Vec3{X: 16, Y: 16, Z: 8})
	vm.SetGInt(qc.OFSSelf, int32(entNum))

	if fn := vm.Builtins[34]; fn == nil {
		t.Fatal("droptofloor builtin not registered")
	} else {
		fn(vm)
	}

	if got := vm.EVector(entNum, qc.EntFieldOrigin); got.Z < 7.99 || got.Z > 8.05 {
		t.Fatalf("droptofloor origin.z = %v, want ~8 from QC-only mins", got.Z)
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

	ent.SetOrigin(s, qtypes.Vec3{})
	ent.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	ent.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	ent.SetSolid(s, float32(SolidSlideBox))

	vm.SetGInt(qc.OFSSelf, int32(entNum))
	vm.SetGFloat(qc.OFSParm0, 0)
	vm.SetGFloat(qc.OFSParm1, 10)
	if fn := vm.Builtins[32]; fn == nil {
		t.Fatal("walkmove builtin not registered")
	} else {
		fn(vm)
	}

	if got := vm.EVector(entNum, qc.EntFieldOrigin); got != (qtypes.Vec3{}) {
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
	mover.SetOrigin(s, qtypes.Vec3{X: 0, Y: 0, Z: 24})
	mover.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	mover.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	mover.SetSolid(s, float32(SolidSlideBox))
	mover.SetFlags(s, float32(FlagOnGround))

	trigger.SetOrigin(s, qtypes.Vec3{X: 24, Y: 0, Z: 24})
	trigger.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	trigger.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	trigger.SetSolid(s, float32(SolidTrigger))
	s.QCVM.SetEInt(trigger.Num, qc.EntFieldTouch, 1)

	s.LinkEdict(mover, false)
	s.LinkEdict(trigger, false)

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
