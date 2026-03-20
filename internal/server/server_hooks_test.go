package server

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/bsp"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/qc"
)

// newServerTestVM prepares the server's VM for tests with reasonable defaults.
func newServerTestVM(s *Server, maxEdicts int) *qc.VM {
	vm := s.QCVM
	vm.Globals = make([]float32, 256)
	vm.MaxEdicts = maxEdicts
	vm.NumEdicts = 1
	vm.EntityFields = 128
	vm.EdictSize = 92 + vm.EntityFields*4
	vm.Edicts = make([]byte, vm.EdictSize*maxEdicts)
	return vm
}

func TestServerHooksSpawnAndRemove(t *testing.T) {
	s := NewServer()
	defer qc.RegisterServerHooks(nil)

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
	defer qc.RegisterServerHooks(nil)

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
	defer qc.RegisterServerHooks(nil)

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)

	// Prepare multiple entities for search tests
	s.Edicts = []*Edict{
		{Vars: &EntVars{}},
		{Vars: &EntVars{}},
		{Vars: &EntVars{}},
		{Vars: &EntVars{}},
	}
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = 4
	for entNum, ent := range s.Edicts {
		syncEdictToQCVM(vm, entNum, ent)
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
	if got := vm.GetString(modelIdx); got != "progs/test.mdl" {
		t.Fatalf("model string = %q", got)
	}
	if got := vm.EFloat(1, qc.EntFieldModelIndex); got != 1 {
		t.Fatalf("modelindex = %v, want 1", got)
	}
}

func TestServerHooksSearchFunctionsSkipFreedEdicts(t *testing.T) {
	s := NewServer()
	defer qc.RegisterServerHooks(nil)

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)

	s.Edicts = []*Edict{
		{Vars: &EntVars{}},
		{Vars: &EntVars{}},
		{Vars: &EntVars{}, Free: true},
		{Vars: &EntVars{}},
	}
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts
	for entNum, ent := range s.Edicts {
		syncEdictToQCVM(vm, entNum, ent)
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

func TestServerHooksSetModelUsesBrushBounds(t *testing.T) {
	s := NewServer()
	defer qc.RegisterServerHooks(nil)

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("failed to alloc edict")
	}
	vm.NumEdicts = s.NumEdicts
	ent.Vars.Origin = [3]float32{64, 32, 16}
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
	if got := vm.GetString(vm.EInt(1, qc.EntFieldModel)); got != "*1" {
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
	defer qc.RegisterServerHooks(nil)

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)
	_ = s.AllocEdict()
	vm.NumEdicts = s.NumEdicts

	defer func() {
		if recover() == nil {
			t.Fatal("setmodel did not panic for non-precached model")
		}
	}()

	vm.SetGInt(qc.OFSParm0, 1)
	vm.SetGString(qc.OFSParm1, "progs/missing.mdl")
	vm.Builtins[3](vm)
}

func TestServerHooksWalkMoveAndDropToFloor(t *testing.T) {
	s := NewServer()
	defer qc.RegisterServerHooks(nil)
	s.WorldModel = CreateSyntheticWorldModel()
	if world := s.EdictNum(0); world != nil && world.Vars != nil {
		world.Vars.Solid = float32(SolidBSP)
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

	ent.Vars.Origin = [3]float32{0, 0, 24}
	ent.Vars.Mins = [3]float32{-16, -16, -24}
	ent.Vars.Maxs = [3]float32{16, 16, 32}
	ent.Vars.Solid = float32(SolidSlideBox)
	ent.Vars.Flags = float32(FlagOnGround)
	s.LinkEdict(ent, false)
	syncEdictToQCVM(vm, entNum, ent)
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

	ent.Vars.Origin = [3]float32{0, 0, 96}
	ent.Vars.Flags = 0
	ent.Vars.GroundEntity = 0
	s.LinkEdict(ent, false)
	syncEdictToQCVM(vm, entNum, ent)
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

func TestServerHooksWalkMoveSyncsFullEdictStateBackToQC(t *testing.T) {
	s := NewServer()
	defer qc.RegisterServerHooks(nil)

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("AllocEdict returned nil")
	}
	entNum := s.NumForEdict(ent)
	vm.NumEdicts = s.NumEdicts

	ent.Vars.Flags = float32(FlagOnGround)
	ent.Vars.Angles[1] = 10
	ent.Vars.YawSpeed = 15
	syncEdictToQCVM(vm, entNum, ent)

	vm.SetGInt(qc.OFSSelf, int32(entNum))
	vm.SetGFloat(qc.OFSParm0, 350)
	vm.SetGFloat(qc.OFSParm1, 10)
	if fn := vm.Builtins[32]; fn == nil {
		t.Fatal("walkmove builtin not registered")
	} else {
		fn(vm)
	}

	if got := vm.EFloat(entNum, qc.EntFieldIdealYaw); got != 350 {
		t.Fatalf("ideal_yaw = %v, want 350", got)
	}
	if got := vm.EVector(entNum, qc.EntFieldAngles); got[1] < 354.99 || got[1] > 355.01 {
		t.Fatalf("angles yaw = %v, want ~355 after changeyaw sync", got)
	}
}

func TestServerHooksWalkMoveRequiresMovementFlags(t *testing.T) {
	s := NewServer()
	defer qc.RegisterServerHooks(nil)

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("AllocEdict returned nil")
	}
	entNum := s.NumForEdict(ent)
	vm.NumEdicts = s.NumEdicts

	ent.Vars.Origin = [3]float32{0, 0, 0}
	ent.Vars.Mins = [3]float32{-16, -16, -24}
	ent.Vars.Maxs = [3]float32{16, 16, 32}
	ent.Vars.Solid = float32(SolidSlideBox)
	syncEdictToQCVM(vm, entNum, ent)

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
	defer qc.RegisterServerHooks(nil)
	s.WorldModel = CreateSyntheticWorldModel()
	if world := s.EdictNum(0); world != nil && world.Vars != nil {
		world.Vars.Solid = float32(SolidBSP)
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
	mover.Vars.Origin = [3]float32{0, 0, 24}
	mover.Vars.Mins = [3]float32{-16, -16, -24}
	mover.Vars.Maxs = [3]float32{16, 16, 32}
	mover.Vars.Solid = float32(SolidSlideBox)
	mover.Vars.Flags = float32(FlagOnGround)

	trigger.Vars.Origin = [3]float32{24, 0, 24}
	trigger.Vars.Mins = [3]float32{-16, -16, -24}
	trigger.Vars.Maxs = [3]float32{16, 16, 32}
	trigger.Vars.Solid = float32(SolidTrigger)
	trigger.Vars.Touch = 1

	s.LinkEdict(mover, false)
	s.LinkEdict(trigger, false)
	syncEdictToQCVM(vm, moverNum, mover)
	syncEdictToQCVM(vm, s.NumForEdict(trigger), trigger)

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

func TestServerHooksTraceContentsAndPrecacheBuiltins(t *testing.T) {
	s := NewServer()
	defer qc.RegisterServerHooks(nil)
	s.Datagram = NewMessageBuffer(MaxDatagram)
	s.Static = &ServerStatic{Clients: []*Client{{Active: true, Message: NewMessageBuffer(MaxDatagram)}}}

	s.WorldModel = CreateSyntheticWorldModel()
	if world := s.EdictNum(0); world != nil && world.Vars != nil {
		world.Vars.Solid = float32(SolidBSP)
	}

	e := s.AllocEdict()
	e.Vars.Origin = [3]float32{0, 0, 24}
	e.Vars.Mins = [3]float32{-16, -16, -24}
	e.Vars.Maxs = [3]float32{16, 16, 32}
	e.Vars.Solid = float32(SolidSlideBox)

	vm := newServerTestVM(s, 8)
	vm.NumEdicts = s.NumEdicts
	qc.RegisterBuiltins(vm)

	// traceline: from above ground into the floor.
	vm.SetGInt(qc.OFSSelf, int32(s.NumForEdict(e)))
	vm.SetGVector(qc.OFSParm0, [3]float32{0, 0, 32})
	vm.SetGVector(qc.OFSParm1, [3]float32{0, 0, -32})
	vm.SetGFloat(qc.OFSParm2, 0)
	if fn := vm.Builtins[16]; fn == nil {
		t.Fatal("traceline builtin not registered")
	} else {
		fn(vm)
	}
	if got := vm.GFloat(qc.OFSTraceFraction); got >= 1 {
		t.Fatalf("trace_fraction = %v, want < 1", got)
	}
	if got := vm.GVector(qc.OFSTraceEndPos); got[2] > DistEpsilon || got[2] < -DistEpsilon {
		t.Fatalf("trace_endpos.z = %v, want approximately 0", got[2])
	}

	// checkbottom: entity resting on the synthetic plane should be supported.
	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(e)))
	if fn := vm.Builtins[40]; fn == nil {
		t.Fatal("checkbottom builtin not registered")
	} else {
		fn(vm)
	}
	if got := vm.GFloat(qc.OFSReturn); got != 1 {
		t.Fatalf("checkbottom return = %v, want 1", got)
	}

	// pointcontents below the plane should be solid.
	vm.SetGVector(qc.OFSParm0, [3]float32{0, 0, -1})
	if fn := vm.Builtins[41]; fn == nil {
		t.Fatal("pointcontents builtin not registered")
	} else {
		fn(vm)
	}
	if got := int(vm.GFloat(qc.OFSReturn)); got == 0 { // pointcontents returns a float content type
		t.Fatalf("pointcontents returned empty contents for solid point")
	}

	// precache_sound/model should populate server lookup tables.
	vm.SetGString(qc.OFSParm0, "misc/menu1.wav")
	vm.Builtins[19](vm)
	if got := s.FindSound("misc/menu1.wav"); got < 0 {
		t.Fatalf("precache_sound did not register sample")
	}

	vm.SetGString(qc.OFSParm0, "progs/player.mdl")
	vm.Builtins[20](vm)
	if got := s.FindModel("progs/player.mdl"); got == 0 {
		t.Fatalf("precache_model did not register model")
	}

	// Attach client to the test entity for client-directed builtins.
	s.Static.Clients[0].Edict = e

	// sound and particle should write to the datagram.
	datagramBefore := s.Datagram.Len()
	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(e)))
	vm.SetGFloat(qc.OFSParm1, 1)
	vm.SetGString(qc.OFSParm2, "misc/menu1.wav")
	vm.SetGFloat(qc.OFSParm3, DefaultSoundVolume)
	vm.SetGFloat(qc.OFSParm4, DefaultSoundAttenuation)
	vm.Builtins[8](vm)
	if s.Datagram.Len() <= datagramBefore {
		t.Fatalf("sound builtin did not write to datagram")
	}

	datagramBefore = s.Datagram.Len()
	vm.SetGVector(qc.OFSParm0, [3]float32{0, 0, 10})
	vm.SetGVector(qc.OFSParm1, [3]float32{0, 0, 1})
	vm.SetGFloat(qc.OFSParm2, 5)
	vm.SetGFloat(qc.OFSParm3, 8)
	vm.Builtins[48](vm)
	if s.Datagram.Len() <= datagramBefore {
		t.Fatalf("particle builtin did not write to datagram")
	}

	// client-targeted messaging should write into the client's reliable buffer.
	clientBefore := s.Static.Clients[0].Message.Len()
	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(e)))
	vm.SetGString(qc.OFSParm1, "bf\n")
	vm.Builtins[21](vm)
	if s.Static.Clients[0].Message.Len() <= clientBefore {
		t.Fatalf("stuffcmd builtin did not write to client message")
	}
	if got := s.Static.Clients[0].Message.Data[clientBefore]; got != byte(inet.SVCStuffText) {
		t.Fatalf("stuffcmd opcode = %d, want %d", got, inet.SVCStuffText)
	}

	clientBefore = s.Static.Clients[0].Message.Len()
	vm.SetGFloat(qc.OFSParm0, 0)
	vm.SetGString(qc.OFSParm1, "m")
	vm.Builtins[35](vm)
	if s.Static.Clients[0].Message.Len() <= clientBefore {
		t.Fatalf("lightstyle builtin did not write to client message")
	}
	if got := s.Static.Clients[0].Message.Data[clientBefore]; got != byte(inet.SVCLightStyle) {
		t.Fatalf("lightstyle opcode = %d, want %d", got, inet.SVCLightStyle)
	}
	if got := s.LightStyles[0]; got != "m" {
		t.Fatalf("stored lightstyle = %q, want %q", got, "m")
	}

	clientBefore = s.Static.Clients[0].Message.Len()
	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(e)))
	vm.SetGString(qc.OFSParm1, "centered")
	vm.Builtins[73](vm)
	if s.Static.Clients[0].Message.Len() <= clientBefore {
		t.Fatalf("centerprint builtin did not write to client message")
	}
	if got := s.Static.Clients[0].Message.Data[clientBefore]; got != byte(inet.SVCCenterPrint) {
		t.Fatalf("centerprint opcode = %d, want %d", got, inet.SVCCenterPrint)
	}

	clientBefore = s.Static.Clients[0].Message.Len()
	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(e)))
	vm.SetGString(qc.OFSParm1, "misc/menu1.wav")
	vm.Builtins[80](vm)
	if s.Static.Clients[0].Message.Len() <= clientBefore {
		t.Fatalf("localsound builtin did not write to client message")
	}

	// Write* builtins: MSG_ONE should use msg_entity -> client message.
	vm.SetGInt(qc.OFSMsgEntity, int32(s.NumForEdict(e)))
	clientBefore = s.Static.Clients[0].Message.Len()
	vm.SetGFloat(qc.OFSParm0, 1)
	vm.SetGFloat(qc.OFSParm1, 42)
	vm.Builtins[52](vm)
	vm.Builtins[53](vm)
	vm.Builtins[54](vm)
	vm.Builtins[55](vm)
	vm.SetGFloat(qc.OFSParm1, 12.5)
	vm.Builtins[56](vm)
	vm.Builtins[57](vm)
	vm.SetGString(qc.OFSParm1, "hello")
	vm.Builtins[58](vm)
	vm.SetGInt(qc.OFSParm1, int32(s.NumForEdict(e)))
	vm.Builtins[59](vm)
	if s.Static.Clients[0].Message.Len() <= clientBefore {
		t.Fatalf("Write* builtins did not write to MSG_ONE buffer")
	}

	// MSG_BROADCAST should use the datagram.
	datagramBefore = s.Datagram.Len()
	vm.SetGFloat(qc.OFSParm0, 0)
	vm.SetGFloat(qc.OFSParm1, 7)
	vm.Builtins[52](vm)
	if s.Datagram.Len() <= datagramBefore {
		t.Fatalf("WriteByte builtin did not write to MSG_BROADCAST datagram")
	}
}

func TestServerHooksCheckClientAimAndSetSpawnParms(t *testing.T) {
	s := NewServer()
	defer qc.RegisterServerHooks(nil)
	s.Datagram = NewMessageBuffer(MaxDatagram)

	self := s.AllocEdict()
	target := s.AllocEdict()
	self.Vars.Origin = [3]float32{0, 0, 0}
	self.Vars.ViewOfs = [3]float32{0, 0, 16}
	self.Vars.AimEnt = int32(s.NumForEdict(target))
	target.Vars.Health = 100
	target.Vars.Origin = [3]float32{0, 100, 16}
	target.Vars.Mins = [3]float32{-16, -16, -24}
	target.Vars.Maxs = [3]float32{16, 16, 32}

	s.Static = &ServerStatic{Clients: []*Client{
		{Active: true, Message: NewMessageBuffer(MaxDatagram), Edict: self},
		{Active: true, Message: NewMessageBuffer(MaxDatagram), Edict: target},
	}}
	s.Static.Clients[1].SpawnParms[0] = 10
	s.Static.Clients[1].SpawnParms[1] = 20

	vm := newServerTestVM(s, 16)
	vm.NumEdicts = s.NumEdicts
	qc.RegisterBuiltins(vm)

	vm.SetGInt(qc.OFSSelf, int32(s.NumForEdict(self)))
	if fn := vm.Builtins[17]; fn == nil {
		t.Fatal("checkclient builtin not registered")
	} else {
		fn(vm)
	}
	if got := int(vm.GInt(qc.OFSReturn)); got != s.NumForEdict(target) {
		t.Fatalf("checkclient = %d, want %d", got, s.NumForEdict(target))
	}

	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(self)))
	vm.SetGFloat(qc.OFSParm1, 0)
	if fn := vm.Builtins[44]; fn == nil {
		t.Fatal("aim builtin not registered")
	} else {
		fn(vm)
	}
	aim := vm.GVector(qc.OFSReturn)
	if aim[1] <= 0.9 {
		t.Fatalf("aim vector = %v, want mostly +Y", aim)
	}

	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(target)))
	if fn := vm.Builtins[78]; fn == nil {
		t.Fatal("setspawnparms builtin not registered")
	} else {
		fn(vm)
	}
	if got := vm.GFloat(qc.OFSParmStart); got != 10 {
		t.Fatalf("parm1 = %v, want 10", got)
	}
	if got := vm.GFloat(qc.OFSParmStart + 1); got != 20 {
		t.Fatalf("parm2 = %v, want 20", got)
	}
}

func TestServerHooksCheckClientRespectsPVS(t *testing.T) {
	s := NewServer()
	defer qc.RegisterServerHooks(nil)
	s.Datagram = NewMessageBuffer(MaxDatagram)

	self := s.AllocEdict()
	target := s.AllocEdict()
	self.Vars.Origin = [3]float32{-64, 0, 0}
	self.Vars.ViewOfs = [3]float32{}
	target.Vars.Origin = [3]float32{64, 0, 0}
	target.Vars.ViewOfs = [3]float32{}
	target.Vars.Health = 100

	s.Static = &ServerStatic{
		MaxClients: 2,
		Clients: []*Client{
			{Active: true, Message: NewMessageBuffer(MaxDatagram), Edict: self},
			{Active: true, Message: NewMessageBuffer(MaxDatagram), Edict: target},
		},
	}
	s.WorldTree = &bsp.Tree{
		Planes: []bsp.DPlane{{Normal: [3]float32{1, 0, 0}, Dist: 0, Type: 0}},
		Nodes: []bsp.TreeNode{{
			PlaneNum: 0,
			Children: [2]bsp.TreeChild{{IsLeaf: true, Index: 0}, {IsLeaf: true, Index: 1}},
		}},
		Leafs:      []bsp.TreeLeaf{{VisOfs: 0}, {VisOfs: 1}},
		Visibility: []byte{0x01, 0x00, 0x01},
	}

	vm := newServerTestVM(s, 16)
	vm.NumEdicts = s.NumEdicts
	qc.RegisterBuiltins(vm)

	s.Time = 0.2
	vm.SetGInt(qc.OFSSelf, int32(s.NumForEdict(self)))
	if fn := vm.Builtins[17]; fn == nil {
		t.Fatal("checkclient builtin not registered")
	} else {
		fn(vm)
	}
	if got := int(vm.GInt(qc.OFSReturn)); got != 0 {
		t.Fatalf("checkclient with self outside target PVS = %d, want 0", got)
	}

	self.Vars.Origin = [3]float32{64, 0, 0}
	s.Time = 0.25
	if fn := vm.Builtins[17]; fn == nil {
		t.Fatal("checkclient builtin not registered")
	} else {
		fn(vm)
	}
	if got := int(vm.GInt(qc.OFSReturn)); got != s.NumForEdict(target) {
		t.Fatalf("checkclient with self inside target PVS = %d, want %d", got, s.NumForEdict(target))
	}
}

func TestServerHooksMakeStaticAndAmbientSound(t *testing.T) {
	s := NewServer()
	defer qc.RegisterServerHooks(nil)
	s.Datagram = NewMessageBuffer(MaxDatagram)
	clientMsg := NewMessageBuffer(MaxDatagram)
	world := s.EdictNum(0)
	if world == nil {
		t.Fatal("missing world edict")
	}
	s.Static = &ServerStatic{Clients: []*Client{{Active: true, Message: clientMsg, Edict: world}}}
	s.SoundPrecache = make([]string, MaxSounds)
	s.SoundPrecache[1] = "ambience/drip.wav"

	ent := s.AllocEdict()
	ent.Vars.Origin = [3]float32{1, 2, 3}
	ent.Vars.Angles = [3]float32{0, 90, 0}
	ent.Vars.ModelIndex = 7
	ent.Vars.Frame = 2
	ent.Vars.Colormap = 3
	ent.Vars.Skin = 4

	vm := newServerTestVM(s, 16)
	vm.NumEdicts = s.NumEdicts
	qc.RegisterBuiltins(vm)

	before := clientMsg.Len()
	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(ent)))
	if fn := vm.Builtins[69]; fn == nil {
		t.Fatal("makestatic builtin not registered")
	} else {
		fn(vm)
	}
	if got := len(s.StaticEntities); got != 1 {
		t.Fatalf("static entities len = %d, want 1", got)
	}
	if !ent.Free {
		t.Fatalf("entity not freed after makestatic")
	}
	if clientMsg.Len() <= before {
		t.Fatalf("makestatic did not write to client message")
	}

	before = clientMsg.Len()
	vm.SetGVector(qc.OFSParm0, [3]float32{4, 5, 6})
	vm.SetGString(qc.OFSParm1, "ambience/drip.wav")
	vm.SetGFloat(qc.OFSParm2, 255)
	vm.SetGFloat(qc.OFSParm3, 1)
	if fn := vm.Builtins[74]; fn == nil {
		t.Fatal("ambientsound builtin not registered")
	} else {
		fn(vm)
	}
	if got := len(s.StaticSounds); got != 1 {
		t.Fatalf("static sounds len = %d, want 1", got)
	}
	if clientMsg.Len() <= before {
		t.Fatalf("ambientsound did not write to client message")
	}

	newClient := &Client{Edict: world, Message: NewMessageBuffer(MaxDatagram)}
	s.SendServerInfo(newClient)
	// Static entities and sounds are now in signon buffers (populated during
	// SpawnServer). Build them here to simulate the full flow.
	if err := s.buildSignonBuffers(); err != nil {
		t.Fatalf("buildSignonBuffers: %v", err)
	}
	s.SendSignonBuffers(newClient)
	if newClient.Message.Len() == 0 {
		t.Fatalf("SendServerInfo did not produce signon message")
	}
	data := newClient.Message.Data[:newClient.Message.Len()]
	foundStatic := false
	foundAmbient := false
	for _, b := range data {
		if b == byte(inet.SVCSpawnStatic) || b == byte(inet.SVCSpawnStatic2) {
			foundStatic = true
		}
		if b == byte(inet.SVCSpawnStaticSound) || b == byte(inet.SVCSpawnStaticSound2) {
			foundAmbient = true
		}
	}
	if !foundStatic {
		t.Fatalf("SendServerInfo missing spawnstatic message")
	}
	if !foundAmbient {
		t.Fatalf("SendServerInfo missing spawnstaticsound message")
	}
}

func TestServerHooksMoveToGoalAndChangeYaw(t *testing.T) {
	s := NewServer()
	defer qc.RegisterServerHooks(nil)

	s.WorldModel = CreateSyntheticWorldModel()
	if world := s.EdictNum(0); world != nil && world.Vars != nil {
		world.Vars.Solid = float32(SolidBSP)
	}
	s.ClearWorld()

	self := s.AllocEdict()
	goal := s.AllocEdict()
	self.Vars.Origin = [3]float32{0, 0, 16}
	self.Vars.Mins = [3]float32{-1, -1, 0}
	self.Vars.Maxs = [3]float32{1, 1, 56}
	self.Vars.Solid = float32(SolidBSP)
	self.Vars.Flags = float32(FlagOnGround)
	self.Vars.IdealYaw = 0
	self.Vars.YawSpeed = 360
	goal.Vars.Origin = [3]float32{64, 0, 16}
	goal.Vars.Mins = [3]float32{-1, -1, 0}
	goal.Vars.Maxs = [3]float32{1, 1, 56}
	self.Vars.GoalEntity = int32(s.NumForEdict(goal))

	s.LinkEdict(self, false)
	s.LinkEdict(goal, false)

	vm := newServerTestVM(s, 16)
	vm.NumEdicts = s.NumEdicts
	qc.RegisterBuiltins(vm)

	selfNum := s.NumForEdict(self)
	vm.SetGInt(qc.OFSSelf, int32(selfNum))
	vm.SetEVector(selfNum, qc.EntFieldOrigin, self.Vars.Origin)
	vm.SetEVector(selfNum, qc.EntFieldAngles, self.Vars.Angles)
	vm.SetEVector(selfNum, qc.EntFieldAbsMin, self.Vars.AbsMin)
	vm.SetEVector(selfNum, qc.EntFieldAbsMax, self.Vars.AbsMax)
	vm.SetEFloat(selfNum, qc.EntFieldFlags, self.Vars.Flags)
	vm.SetEInt(selfNum, qc.EntFieldGoalEntity, self.Vars.GoalEntity)

	vm.SetGFloat(qc.OFSParm0, 16)
	if fn := vm.Builtins[67]; fn == nil {
		t.Fatal("movetogoal builtin not registered")
	} else {
		fn(vm)
	}
	if self.Vars.Origin[0] <= 0 {
		t.Fatalf("movetogoal did not move entity forward: origin=%v", self.Vars.Origin)
	}
	if got := vm.EVector(selfNum, qc.EntFieldOrigin); got != self.Vars.Origin {
		t.Fatalf("vm origin not synchronized after movetogoal: got=%v want=%v", got, self.Vars.Origin)
	}

	self.Vars.Angles[1] = 10
	self.Vars.IdealYaw = 350
	self.Vars.YawSpeed = 15
	vm.SetEVector(selfNum, qc.EntFieldAngles, self.Vars.Angles)
	vm.SetEFloat(selfNum, qc.EntFieldIdealYaw, self.Vars.IdealYaw)

	if fn := vm.Builtins[49]; fn == nil {
		t.Fatal("changeyaw builtin not registered")
	} else {
		fn(vm)
	}
	// anglemod uses 16-bit quantization matching C, so 355 becomes ~355.00122
	if got := self.Vars.Angles[1]; got < 354.99 || got > 355.01 {
		t.Fatalf("changeyaw yaw = %v, want ~355", got)
	}
	if got := vm.EVector(selfNum, qc.EntFieldAngles); got[1] < 354.99 || got[1] > 355.01 {
		t.Fatalf("vm yaw not synchronized after changeyaw: got=%v", got[1])
	}
}

func TestServerHooksMoveToGoalRestoresQCContextAfterNestedTouch(t *testing.T) {
	s := NewServer()
	defer qc.RegisterServerHooks(nil)

	s.WorldModel = CreateSyntheticWorldModel()
	if world := s.EdictNum(0); world != nil && world.Vars != nil {
		world.Vars.Solid = float32(SolidBSP)
	}
	s.ClearWorld()

	vm := newServerTestVM(s, 16)
	vm.NumEdicts = s.NumEdicts
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

	self := s.AllocEdict()
	goal := s.AllocEdict()
	trigger := s.AllocEdict()
	if self == nil || goal == nil || trigger == nil {
		t.Fatal("failed to allocate edicts")
	}
	vm.NumEdicts = s.NumEdicts

	selfNum := s.NumForEdict(self)
	self.Vars.Origin = [3]float32{0, 0, 24}
	self.Vars.Mins = [3]float32{-16, -16, -24}
	self.Vars.Maxs = [3]float32{16, 16, 32}
	self.Vars.Solid = float32(SolidSlideBox)
	self.Vars.Flags = float32(FlagOnGround)
	self.Vars.IdealYaw = 0
	self.Vars.YawSpeed = 360

	goal.Vars.Origin = [3]float32{64, 0, 24}
	goal.Vars.Mins = [3]float32{-16, -16, -24}
	goal.Vars.Maxs = [3]float32{16, 16, 32}
	self.Vars.GoalEntity = int32(s.NumForEdict(goal))

	trigger.Vars.Origin = [3]float32{24, 0, 24}
	trigger.Vars.Mins = [3]float32{-16, -16, -24}
	trigger.Vars.Maxs = [3]float32{16, 16, 32}
	trigger.Vars.Solid = float32(SolidTrigger)
	trigger.Vars.Touch = 1

	s.LinkEdict(self, false)
	s.LinkEdict(goal, false)
	s.LinkEdict(trigger, false)
	syncEdictToQCVM(vm, selfNum, self)
	syncEdictToQCVM(vm, s.NumForEdict(goal), goal)
	syncEdictToQCVM(vm, s.NumForEdict(trigger), trigger)

	vm.SetGInt(qc.OFSSelf, int32(selfNum))
	vm.SetGInt(qc.OFSOther, 77)
	vm.XFunction = &vm.Functions[2]
	vm.XFunctionIndex = 2
	vm.SetGFloat(qc.OFSParm0, 24)
	if fn := vm.Builtins[67]; fn == nil {
		t.Fatal("movetogoal builtin not registered")
	} else {
		fn(vm)
	}

	if got := vm.GInt(qc.OFSSelf); got != int32(selfNum) {
		t.Fatalf("self after nested movetogoal = %d, want %d", got, selfNum)
	}
	if got := vm.GInt(qc.OFSOther); got != 77 {
		t.Fatalf("other after nested movetogoal = %d, want 77", got)
	}
	if vm.XFunction != &vm.Functions[2] || vm.XFunctionIndex != 2 {
		t.Fatalf("qc context not restored: xfunction=%p idx=%d", vm.XFunction, vm.XFunctionIndex)
	}
}
