package qc

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/cvar"
)

func newBuiltinsTestVM(maxEdicts int) *VM {
	vm := NewVM()
	vm.Globals = make([]float32, 256)
	vm.MaxEdicts = maxEdicts
	vm.NumEdicts = 1
	vm.EntityFields = 128
	vm.EdictSize = 28 + vm.EntityFields*4
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
		traceline      int
		spawn          int
		remove         int
		find           int
		findfloat      int
		nextent        int
		findradius     int
		checkbottom    int
		pointcontents  int
		walkmove       int
		droptofloor    int
		setorigin      int
		setsize        int
		setmodel       int
		precacheSound  int
		precacheModel  int
		broadcastPrint int
		clientPrint    int
		debugPrint     int
		centerPrint    int
		sound          int
		stuffcmd       int
		lightstyle     int
		particle       int
		localsound     int
		checkclient    int
		aim            int
		writeByte      int
		writeChar      int
		writeShort     int
		writeLong      int
		writeCoord     int
		writeAngle     int
		writeString    int
		writeEntity    int
		setspawnparms  int
		movetogoal     int
		changeyaw      int
	}{}

	SetServerBuiltinHooks(ServerBuiltinHooks{
		Traceline: func(vm *VM, start, end [3]float32, noMonsters bool, passEnt int) BuiltinTraceResult {
			hookCalls.traceline++
			return BuiltinTraceResult{Fraction: 0.5, EndPos: [3]float32{4, 5, 6}, PlaneNormal: [3]float32{0, 0, 1}, EntNum: 3}
		},
		Spawn: func(vm *VM) (int, error) {
			hookCalls.spawn++
			return 5, nil
		},
		Remove: func(vm *VM, entNum int) error {
			hookCalls.remove++
			return nil
		},
		Find: func(vm *VM, startEnt, fieldOfs int, match string) int {
			hookCalls.find++
			return 6
		},
		FindFloat: func(vm *VM, startEnt, fieldOfs int, match float32) int {
			hookCalls.findfloat++
			return 7
		},
		NextEnt: func(vm *VM, entNum int) int {
			hookCalls.nextent++
			return 8
		},
		CheckBottom: func(vm *VM, entNum int) bool {
			hookCalls.checkbottom++
			return true
		},
		PointContents: func(vm *VM, point [3]float32) int {
			hookCalls.pointcontents++
			return -2
		},
		FindRadius: func(vm *VM, org [3]float32, radius float32) int {
			hookCalls.findradius++
			return 9
		},
		CheckClient: func(vm *VM) int {
			hookCalls.checkclient++
			return 10
		},
		WalkMove: func(vm *VM, yaw, dist float32) bool {
			hookCalls.walkmove++
			return true
		},
		Aim: func(vm *VM, entNum int, missileSpeed float32) [3]float32 {
			hookCalls.aim++
			return [3]float32{0, 1, 0}
		},
		DropToFloor: func(vm *VM) bool {
			hookCalls.droptofloor++
			return true
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
		PrecacheSound: func(vm *VM, sample string) {
			hookCalls.precacheSound++
		},
		PrecacheModel: func(vm *VM, modelName string) {
			hookCalls.precacheModel++
		},
		BroadcastPrint: func(vm *VM, msg string) {
			hookCalls.broadcastPrint++
		},
		ClientPrint: func(vm *VM, entNum int, msg string) {
			hookCalls.clientPrint++
		},
		DebugPrint: func(vm *VM, msg string) {
			hookCalls.debugPrint++
		},
		CenterPrint: func(vm *VM, entNum int, msg string) {
			hookCalls.centerPrint++
		},
		Sound: func(vm *VM, entNum, channel int, sample string, volume int, attenuation float32) {
			hookCalls.sound++
		},
		StuffCmd: func(vm *VM, entNum int, cmd string) {
			hookCalls.stuffcmd++
		},
		LightStyle: func(vm *VM, style int, value string) {
			hookCalls.lightstyle++
		},
		Particle: func(vm *VM, org, dir [3]float32, color, count int) {
			hookCalls.particle++
		},
		LocalSound: func(vm *VM, entNum int, sample string) {
			hookCalls.localsound++
		},
		WriteByte:     func(vm *VM, dest, value int) { hookCalls.writeByte++ },
		WriteChar:     func(vm *VM, dest, value int) { hookCalls.writeChar++ },
		WriteShort:    func(vm *VM, dest, value int) { hookCalls.writeShort++ },
		WriteLong:     func(vm *VM, dest int, value int32) { hookCalls.writeLong++ },
		WriteCoord:    func(vm *VM, dest int, value float32) { hookCalls.writeCoord++ },
		WriteAngle:    func(vm *VM, dest int, value float32) { hookCalls.writeAngle++ },
		WriteString:   func(vm *VM, dest int, value string) { hookCalls.writeString++ },
		WriteEntity:   func(vm *VM, dest, entNum int) { hookCalls.writeEntity++ },
		SetSpawnParms: func(vm *VM, entNum int) { hookCalls.setspawnparms++ },
		MoveToGoal: func(vm *VM, dist float32) {
			hookCalls.movetogoal++
		},
		ChangeYaw: func(vm *VM) {
			hookCalls.changeyaw++
		},
	})
	defer SetServerBuiltinHooks(ServerBuiltinHooks{})

	vm := newBuiltinsTestVM(8)
	vm.SetGVector(OFSParm0, [3]float32{1, 2, 3})
	vm.SetGVector(OFSParm1, [3]float32{7, 8, 9})
	vm.SetGFloat(OFSParm2, 0)
	traceline(vm)
	if got := vm.GFloat(OFSReturn); got != 0.5 {
		t.Fatalf("traceline return = %v, want 0.5", got)
	}
	if got := vm.GVector(OFSTraceEndPos); got != [3]float32{4, 5, 6} {
		t.Fatalf("trace_endpos = %v", got)
	}
	checkclient(vm)
	if got := int(vm.GFloat(OFSReturn)); got != 10 {
		t.Fatalf("checkclient return = %d, want 10", got)
	}
	vm.SetGFloat(OFSParm0, 1)
	vm.SetGFloat(OFSParm1, 0)
	aimBuiltin(vm)
	if got := vm.GVector(OFSReturn); got != [3]float32{0, 1, 0} {
		t.Fatalf("aim return = %v", got)
	}

	spawn(vm)
	if got := int(vm.GFloat(OFSReturn)); got != 5 {
		t.Fatalf("spawn return = %d, want 5", got)
	}

	vm.SetGFloat(OFSParm0, 1)
	remove(vm)
	sound(vm)
	find(vm)
	findfloat(vm)
	nextent(vm)
	stuffcmd(vm)
	findradius(vm)
	checkbottom(vm)
	pointcontents(vm)
	walkmove(vm)
	droptofloor(vm)
	lightstyle(vm)
	particle(vm)
	precacheSound(vm)
	precacheModel(vm)
	bprint(vm)
	sprint(vm)
	dprint(vm)

	vm.SetGVector(OFSParm1, [3]float32{1, 2, 3})
	setorigin(vm)

	vm.SetGVector(OFSParm1, [3]float32{-1, -1, -1})
	vm.SetGVector(OFSParm2, [3]float32{1, 1, 1})
	setsize(vm)

	vm.SetGString(OFSParm1, "progs/hook.mdl")
	setmodel(vm)
	centerprint(vm)
	localsound(vm)
	writeByteBuiltin(vm)
	writeCharBuiltin(vm)
	writeShortBuiltin(vm)
	writeLongBuiltin(vm)
	writeCoordBuiltin(vm)
	writeAngleBuiltin(vm)
	writeStringBuiltin(vm)
	writeEntityBuiltin(vm)
	setspawnparms(vm)

	vm.SetGFloat(OFSParm0, 1)
	movetogoal(vm)
	changeyaw(vm)

	if got := int(vm.GFloat(OFSReturn)); got != 1 {
		t.Fatalf("droptofloor return = %d, want 1", got)
	}

	if hookCalls.traceline != 1 ||
		hookCalls.checkclient != 1 ||
		hookCalls.aim != 1 ||
		hookCalls.spawn != 1 ||
		hookCalls.remove != 1 ||
		hookCalls.find != 1 ||
		hookCalls.findfloat != 1 ||
		hookCalls.nextent != 1 ||
		hookCalls.findradius != 1 ||
		hookCalls.checkbottom != 1 ||
		hookCalls.pointcontents != 1 ||
		hookCalls.walkmove != 1 ||
		hookCalls.droptofloor != 1 ||
		hookCalls.setorigin != 1 ||
		hookCalls.setsize != 1 ||
		hookCalls.setmodel != 1 ||
		hookCalls.precacheSound != 1 ||
		hookCalls.precacheModel != 1 ||
		hookCalls.broadcastPrint != 1 ||
		hookCalls.clientPrint != 1 ||
		hookCalls.debugPrint != 1 ||
		hookCalls.centerPrint != 1 ||
		hookCalls.sound != 1 ||
		hookCalls.stuffcmd != 1 ||
		hookCalls.lightstyle != 1 ||
		hookCalls.particle != 1 ||
		hookCalls.localsound != 1 ||
		hookCalls.writeByte != 1 ||
		hookCalls.writeChar != 1 ||
		hookCalls.writeShort != 1 ||
		hookCalls.writeLong != 1 ||
		hookCalls.writeCoord != 1 ||
		hookCalls.writeAngle != 1 ||
		hookCalls.writeString != 1 ||
		hookCalls.writeEntity != 1 ||
		hookCalls.setspawnparms != 1 ||
		hookCalls.movetogoal != 1 ||
		hookCalls.changeyaw != 1 {
		t.Fatalf("unexpected hook calls: %+v", hookCalls)
	}
}

func TestRegisterBuiltinsCanonicalMappings(t *testing.T) {
	vm := newBuiltinsTestVM(8)
	RegisterBuiltins(vm)

	for _, slot := range []int{6, 8, 10, 11, 16, 17, 19, 20, 21, 23, 24, 25, 31, 35, 36, 37, 38, 40, 41, 43, 44, 45, 46, 48, 52, 53, 54, 55, 56, 57, 58, 59, 68, 69, 70, 72, 73, 74, 78, 79, 80, 316, 317, 318, 320, 321, 322, 323, 324, 325, 326, 327, 328} {
		if vm.Builtins[slot] == nil {
			t.Fatalf("builtin %d is nil", slot)
		}
	}
	if vm.Builtins[1000] == nil {
		t.Fatalf("temporary findfloat helper slot is nil")
	}
}

func TestMathCVarAndLocalCmdBuiltins(t *testing.T) {
	SetServerBuiltinHooks(ServerBuiltinHooks{})
	vm := newBuiltinsTestVM(8)

	vm.SetGFloat(OFSParm0, 2.6)
	rintBuiltin(vm)
	if got := vm.GFloat(OFSReturn); got != 3 {
		t.Fatalf("rint = %v, want 3", got)
	}
	floorBuiltin(vm)
	if got := vm.GFloat(OFSReturn); got != 2 {
		t.Fatalf("floor = %v, want 2", got)
	}
	ceilBuiltin(vm)
	if got := vm.GFloat(OFSReturn); got != 3 {
		t.Fatalf("ceil = %v, want 3", got)
	}
	vm.SetGFloat(OFSParm0, -2.6)
	fabsBuiltin(vm)
	if got := vm.GFloat(OFSReturn); got != 2.6 {
		t.Fatalf("fabs = %v, want 2.6", got)
	}

	cvar.Set("qc_test_var", "12.5")
	vm.SetGString(OFSParm0, "qc_test_var")
	cvarBuiltin(vm)
	if got := vm.GFloat(OFSReturn); got != 12.5 {
		t.Fatalf("cvar = %v, want 12.5", got)
	}

	vm.SetGString(OFSParm0, "qc_test_set")
	vm.SetGString(OFSParm1, "99")
	cvarSetBuiltin(vm)
	if got := cvar.StringValue("qc_test_set"); got != "99" {
		t.Fatalf("cvar_set stored %q, want 99", got)
	}

	executed := false
	cmdsys.AddCommand("qc_test_cmd", func(args []string) { executed = true }, "")
	defer cmdsys.RemoveCommand("qc_test_cmd")
	vm.SetGString(OFSParm0, "qc_test_cmd\n")
	localcmd(vm)
	cmdsys.Execute()
	if !executed {
		t.Fatal("localcmd did not enqueue command")
	}
}

func TestSearchBuiltinsFallback(t *testing.T) {
	SetServerBuiltinHooks(ServerBuiltinHooks{})
	vm := newBuiltinsTestVM(8)
	vm.NumEdicts = 4

	vm.SetEInt(1, EntFieldTargetName, vm.AllocString("door"))
	vm.SetEVector(1, EntFieldOrigin, [3]float32{100, 0, 0})
	vm.SetEInt(2, EntFieldTargetName, vm.AllocString("trigger"))
	vm.SetEFloat(2, EntFieldHealth, 100)
	vm.SetEVector(2, EntFieldOrigin, [3]float32{10, 0, 0})
	vm.SetEVector(3, EntFieldOrigin, [3]float32{40, 0, 0})

	vm.SetGFloat(OFSParm0, 0)
	vm.SetGInt(OFSParm1, EntFieldTargetName)
	vm.SetGString(OFSParm2, "trigger")
	find(vm)
	if got := int(vm.GFloat(OFSReturn)); got != 2 {
		t.Fatalf("find return = %d, want 2", got)
	}

	vm.SetGFloat(OFSParm0, 0)
	vm.SetGInt(OFSParm1, EntFieldHealth)
	vm.SetGFloat(OFSParm2, 100)
	findfloat(vm)
	if got := int(vm.GFloat(OFSReturn)); got != 2 {
		t.Fatalf("findfloat return = %d, want 2", got)
	}

	vm.SetGFloat(OFSParm0, 1)
	nextent(vm)
	if got := int(vm.GFloat(OFSReturn)); got != 2 {
		t.Fatalf("nextent return = %d, want 2", got)
	}

	vm.SetGVector(OFSParm0, [3]float32{0, 0, 0})
	vm.SetGFloat(OFSParm1, 15)
	findradius(vm)
	if got := int(vm.GFloat(OFSReturn)); got != 2 {
		t.Fatalf("findradius return = %d, want 2", got)
	}
}

func TestMathBuiltins(t *testing.T) {
	vm := newBuiltinsTestVM(4)
	RegisterBuiltins(vm)

	tests := []struct {
		name string
		fn   func(*VM)
		parm float32
		want float32
		tol  float32
	}{
		{"sin(90)", sinBuiltin, 90, 1.0, 0.001},
		{"sin(0)", sinBuiltin, 0, 0.0, 0.001},
		{"cos(0)", cosBuiltin, 0, 1.0, 0.001},
		{"cos(90)", cosBuiltin, 90, 0.0, 0.001},
		{"sqrt(4)", sqrtBuiltin, 4, 2.0, 0.001},
		{"sqrt(9)", sqrtBuiltin, 9, 3.0, 0.001},
		{"tan(45)", tanBuiltin, 45, 1.0, 0.001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm.SetGFloat(OFSParm0, tt.parm)
			tt.fn(vm)
			got := vm.GFloat(OFSReturn)
			diff := got - tt.want
			if diff < -tt.tol || diff > tt.tol {
				t.Errorf("%s = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestMinMaxBoundPow(t *testing.T) {
	vm := newBuiltinsTestVM(4)

	// min(3, 7) = 3
	vm.SetGFloat(OFSParm0, 3)
	vm.SetGFloat(OFSParm0+3, 7)
	minBuiltin(vm)
	if got := vm.GFloat(OFSReturn); got != 3 {
		t.Errorf("min(3,7) = %v, want 3", got)
	}

	// max(3, 7) = 7
	vm.SetGFloat(OFSParm0, 3)
	vm.SetGFloat(OFSParm0+3, 7)
	maxBuiltin(vm)
	if got := vm.GFloat(OFSReturn); got != 7 {
		t.Errorf("max(3,7) = %v, want 7", got)
	}

	// bound(1, 5, 3) = 3 (value clamped to max)
	vm.SetGFloat(OFSParm0, 1)
	vm.SetGFloat(OFSParm0+3, 5)
	vm.SetGFloat(OFSParm0+6, 3)
	boundBuiltin(vm)
	if got := vm.GFloat(OFSReturn); got != 3 {
		t.Errorf("bound(1,5,3) = %v, want 3", got)
	}

	// pow(2, 3) = 8
	vm.SetGFloat(OFSParm0, 2)
	vm.SetGFloat(OFSParm0+3, 3)
	powBuiltin(vm)
	if got := vm.GFloat(OFSReturn); got != 8 {
		t.Errorf("pow(2,3) = %v, want 8", got)
	}
}

func TestStringBuiltins(t *testing.T) {
	vm := newBuiltinsTestVM(4)

	// strlen("hello") = 5
	vm.SetGString(OFSParm0, "hello")
	strlenBuiltin(vm)
	if got := vm.GFloat(OFSReturn); got != 5 {
		t.Errorf("strlen(hello) = %v, want 5", got)
	}

	// strcat("foo", "bar") = "foobar"
	vm.SetGString(OFSParm0, "foo")
	vm.SetGString(OFSParm1, "bar")
	strcatBuiltin(vm)
	if got := vm.GString(OFSReturn); got != "foobar" {
		t.Errorf("strcat(foo,bar) = %q, want foobar", got)
	}

	// substring("hello world", 6, 5) = "world"
	vm.SetGString(OFSParm0, "hello world")
	vm.SetGFloat(OFSParm0+3, 6)
	vm.SetGFloat(OFSParm0+6, 5)
	substringBuiltin(vm)
	if got := vm.GString(OFSReturn); got != "world" {
		t.Errorf("substring(hello world,6,5) = %q, want world", got)
	}

	// stov("'1 2 3'") = [1,2,3]
	vm.SetGString(OFSParm0, "'1 2 3'")
	stovBuiltin(vm)
	if got := vm.GVector(OFSReturn); got != [3]float32{1, 2, 3} {
		t.Errorf("stov('1 2 3') = %v, want [1 2 3]", got)
	}

	// stof("3.14") = 3.14
	vm.SetGString(OFSParm0, "3.14")
	stofBuiltin(vm)
	got := vm.GFloat(OFSReturn)
	if got < 3.13 || got > 3.15 {
		t.Errorf("stof(3.14) = %v, want ~3.14", got)
	}

	// etos(42) = "42"
	vm.SetGFloat(OFSParm0, 42)
	etosBuiltin(vm)
	if got := vm.GString(OFSReturn); got != "42" {
		t.Errorf("etos(42) = %q, want 42", got)
	}

	// chr2str(65) = "A"
	vm.SetGFloat(OFSParm0, 65)
	chr2strBuiltin(vm)
	if got := vm.GString(OFSReturn); got != "A" {
		t.Errorf("chr2str(65) = %q, want A", got)
	}

	// str2chr("A", 0) = 65
	vm.SetGString(OFSParm0, "A")
	vm.SetGFloat(OFSParm0+3, 0)
	str2chrBuiltin(vm)
	if got := vm.GFloat(OFSReturn); got != 65 {
		t.Errorf("str2chr(A,0) = %v, want 65", got)
	}
}

func TestRandomBuiltinDistribution(t *testing.T) {
	vm := newBuiltinsTestVM(4)

	// Verify random() produces values in open interval (0, 1).
	// With the gameplayfix formula: ((r+0.5)/0x8000), min=0.5/32768, max=32767.5/32768.
	for i := 0; i < 1000; i++ {
		random(vm)
		v := vm.GFloat(OFSReturn)
		if v <= 0 || v >= 1 {
			t.Fatalf("random() = %v, want (0,1) exclusive", v)
		}
	}
}
