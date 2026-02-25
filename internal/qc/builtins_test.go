package qc

import "testing"

func newBuiltinsTestVM(maxEdicts int) *VM {
	vm := NewVM()
	vm.Globals = make([]float32, 256)
	vm.MaxEdicts = maxEdicts
	vm.NumEdicts = 1
	vm.EntityFields = 128
	vm.EdictSize = 92 + vm.EntityFields*4
	vm.Edicts = make([]byte, vm.EdictSize*maxEdicts)
	return vm
}

func TestSpawnAllocatesEntity(t *testing.T) {
	SetServerBuiltinHooks(ServerBuiltinHooks{})
	vm := newBuiltinsTestVM(8)

	spawn(vm)

	if got := int(vm.GFloat(OFSReturn)); got != 1 {
		t.Fatalf("spawn return = %d, want 1", got)
	}
	if vm.NumEdicts != 2 {
		t.Fatalf("NumEdicts = %d, want 2", vm.NumEdicts)
	}
}

func TestRemoveClearsEntityData(t *testing.T) {
	SetServerBuiltinHooks(ServerBuiltinHooks{})
	vm := newBuiltinsTestVM(8)
	vm.NumEdicts = 2

	vm.SetEFloat(1, EntFieldHealth, 99)
	vm.SetEVector(1, EntFieldOrigin, [3]float32{1, 2, 3})
	vm.SetGFloat(OFSParm0, 1)

	remove(vm)

	if got := vm.EFloat(1, EntFieldHealth); got != 0 {
		t.Fatalf("health after remove = %f, want 0", got)
	}
	if got := vm.EVector(1, EntFieldOrigin); got != [3]float32{} {
		t.Fatalf("origin after remove = %v, want zero", got)
	}
}

func TestSetOriginUpdatesAbsBounds(t *testing.T) {
	SetServerBuiltinHooks(ServerBuiltinHooks{})
	vm := newBuiltinsTestVM(8)
	vm.NumEdicts = 2

	vm.SetEVector(1, EntFieldMins, [3]float32{-1, -2, -3})
	vm.SetEVector(1, EntFieldMaxs, [3]float32{4, 5, 6})
	vm.SetGFloat(OFSParm0, 1)
	vm.SetGVector(OFSParm1, [3]float32{10, 20, 30})

	setorigin(vm)

	if got := vm.EVector(1, EntFieldOrigin); got != [3]float32{10, 20, 30} {
		t.Fatalf("origin = %v", got)
	}
	if got := vm.EVector(1, EntFieldAbsMin); got != [3]float32{9, 18, 27} {
		t.Fatalf("absmin = %v", got)
	}
	if got := vm.EVector(1, EntFieldAbsMax); got != [3]float32{14, 25, 36} {
		t.Fatalf("absmax = %v", got)
	}
}

func TestSetSizeUpdatesSizeAndAbsBounds(t *testing.T) {
	SetServerBuiltinHooks(ServerBuiltinHooks{})
	vm := newBuiltinsTestVM(8)
	vm.NumEdicts = 2

	vm.SetEVector(1, EntFieldOrigin, [3]float32{10, 20, 30})
	vm.SetGFloat(OFSParm0, 1)
	vm.SetGVector(OFSParm1, [3]float32{-1, -2, -3})
	vm.SetGVector(OFSParm2, [3]float32{4, 5, 6})

	setsize(vm)

	if got := vm.EVector(1, EntFieldMins); got != [3]float32{-1, -2, -3} {
		t.Fatalf("mins = %v", got)
	}
	if got := vm.EVector(1, EntFieldMaxs); got != [3]float32{4, 5, 6} {
		t.Fatalf("maxs = %v", got)
	}
	if got := vm.EVector(1, EntFieldSize); got != [3]float32{5, 7, 9} {
		t.Fatalf("size = %v", got)
	}
	if got := vm.EVector(1, EntFieldAbsMin); got != [3]float32{9, 18, 27} {
		t.Fatalf("absmin = %v", got)
	}
	if got := vm.EVector(1, EntFieldAbsMax); got != [3]float32{14, 25, 36} {
		t.Fatalf("absmax = %v", got)
	}
}

func TestSetModelStoresModelAndModelIndex(t *testing.T) {
	SetServerBuiltinHooks(ServerBuiltinHooks{})
	vm := newBuiltinsTestVM(8)
	vm.NumEdicts = 2

	vm.SetGFloat(OFSParm0, 1)
	vm.SetGString(OFSParm1, "progs/test.mdl")

	setmodel(vm)

	modelIdx := vm.EInt(1, EntFieldModel)
	if got := vm.GetString(modelIdx); got != "progs/test.mdl" {
		t.Fatalf("model string = %q", got)
	}
	if got := vm.EFloat(1, EntFieldModelIndex); got != 1 {
		t.Fatalf("modelindex = %f, want 1", got)
	}
}

func TestBuiltinsUseServerHooksWhenConfigured(t *testing.T) {
	hookCalls := struct {
		spawn     int
		remove    int
		setorigin int
		setsize   int
		setmodel  int
	}{}

	SetServerBuiltinHooks(ServerBuiltinHooks{
		Spawn: func(vm *VM) (int, error) {
			hookCalls.spawn++
			return 5, nil
		},
		Remove: func(vm *VM, entNum int) error {
			hookCalls.remove++
			return nil
		},
		SetOrigin: func(vm *VM, entNum int, org [3]float32) {
			hookCalls.setorigin++
		},
		SetSize: func(vm *VM, entNum int, mins, maxs [3]float32) {
			hookCalls.setsize++
		},
		SetModel: func(vm *VM, entNum int, modelName string) {
			hookCalls.setmodel++
		},
	})
	defer SetServerBuiltinHooks(ServerBuiltinHooks{})

	vm := newBuiltinsTestVM(8)
	spawn(vm)
	if got := int(vm.GFloat(OFSReturn)); got != 5 {
		t.Fatalf("spawn return = %d, want 5", got)
	}

	vm.SetGFloat(OFSParm0, 1)
	remove(vm)

	vm.SetGVector(OFSParm1, [3]float32{1, 2, 3})
	setorigin(vm)

	vm.SetGVector(OFSParm1, [3]float32{-1, -1, -1})
	vm.SetGVector(OFSParm2, [3]float32{1, 1, 1})
	setsize(vm)

	vm.SetGString(OFSParm1, "progs/hook.mdl")
	setmodel(vm)

	if hookCalls.spawn != 1 || hookCalls.remove != 1 || hookCalls.setorigin != 1 || hookCalls.setsize != 1 || hookCalls.setmodel != 1 {
		t.Fatalf("unexpected hook calls: %+v", hookCalls)
	}
}
