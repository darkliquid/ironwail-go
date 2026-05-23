package qc

import (
	"math"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/cvar"
)

func newBuiltinsTestVM(maxEdicts int) *VM {
	vm := NewVM()
	vm.Globals = make([]float32, 256)
	vm.MaxEdicts = maxEdicts
	vm.NumEdicts = 1
	vm.EntityFields = 128
	vm.EdictSize = 28 + vm.EntityFields*4
	vm.Edicts = make([]byte, vm.EdictSize*maxEdicts)
	vm.Cvars = cvar.NewCVarSystem()
	return vm
}

func TestLocalizedTextMessageDecodesEscapedControlCharacters(t *testing.T) {
	got := localizedTextMessage(`line1\nline2\t\"quoted\"\\tail`)
	want := "line1\nline2\t\"quoted\"\\tail"
	if got != want {
		t.Fatalf("localizedTextMessage() = %q, want %q", got, want)
	}
}

func TestWriteStringBuiltinDecodesEscapedNewlines(t *testing.T) {
	vm := newBuiltinsTestVM(8)
	var got string
	vm.ServerHooks = ServerBuiltinHooks{
		WriteString: func(vm *VM, dest int, value string) {
			got = value
		},
	}

	vm.SetGFloat(OFSParm0, 1)
	vm.SetGString(OFSParm1, `line1\nline2`)
	writeStringBuiltin(vm)

	if got != "line1\nline2" {
		t.Fatalf("WriteString decoded value = %q, want real newline payload", got)
	}
}

func TestRegisterBuiltinsIncludesNoopExtensionProbes(t *testing.T) {
	vm := newBuiltinsTestVM(4)

	RegisterBuiltins(vm)

	for _, builtinNum := range []int{99, 100} {
		builtin := vm.Builtins[builtinNum]
		if builtin == nil {
			t.Fatalf("builtin %d was not registered", builtinNum)
		}
		vm.SetGFloat(OFSReturn, 123)
		builtin(vm)
		if got := vm.GFloat(OFSReturn); got != 0 {
			t.Fatalf("builtin %d return = %v, want 0", builtinNum, got)
		}
	}
}

func TestModBuiltinWarnsOnZeroDivisor(t *testing.T) {
	vm := newBuiltinsTestVM(4)
	var printed []string
	console.SetPrintCallback(func(msg string) {
		printed = append(printed, msg)
	})
	t.Cleanup(func() {
		console.SetPrintCallback(nil)
	})

	vm.SetGFloat(OFSParm0, 7)
	vm.SetGFloat(OFSParm1, 0)

	modBuiltin(vm)

	if got := vm.GFloat(OFSReturn); got != 0 {
		t.Fatalf("modBuiltin return = %v, want 0", got)
	}
	if len(printed) != 1 || printed[0] != "PF_mod: mod by zero\n" {
		t.Fatalf("printed = %q, want %q", printed, "PF_mod: mod by zero\n")
	}
}

func TestModBuiltinBehaviorMatrixMatchesC(t *testing.T) {
	vm := newBuiltinsTestVM(4)
	negZero := float32(math.Copysign(0, -1))

	tests := []struct {
		name        string
		a           float32
		b           float32
		want        float32
		wantWarning bool
	}{
		{name: "7 mod 3", a: 7, b: 3, want: 1},
		{name: "-7 mod 3", a: -7, b: 3, want: -1},
		{name: "7 mod -3", a: 7, b: -3, want: 1},
		{name: "-7 mod -3", a: -7, b: -3, want: -1},
		{name: "5.5 mod 2", a: 5.5, b: 2, want: 1.5},
		{name: "-5.5 mod 2", a: -5.5, b: 2, want: -1.5},
		{name: "0 mod 3", a: 0, b: 3, want: 0},
		{name: "7 mod +0", a: 7, b: 0, want: 0, wantWarning: true},
		{name: "7 mod -0", a: 7, b: negZero, want: 0, wantWarning: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var printed []string
			console.SetPrintCallback(func(msg string) {
				printed = append(printed, msg)
			})
			defer console.SetPrintCallback(nil)

			vm.SetGFloat(OFSParm0, tc.a)
			vm.SetGFloat(OFSParm1, tc.b)
			vm.SetGFloat(OFSReturn, -999)

			modBuiltin(vm)

			if got := vm.GFloat(OFSReturn); got != tc.want {
				t.Fatalf("modBuiltin(%v,%v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}

			if tc.wantWarning {
				if len(printed) != 1 || printed[0] != "PF_mod: mod by zero\n" {
					t.Fatalf("printed = %q, want %q", printed, "PF_mod: mod by zero\n")
				}
				return
			}
			if len(printed) != 0 {
				t.Fatalf("printed = %q, want no warning", printed)
			}
		})
	}
}

func TestSoundBuiltinScalesVolumeAndPreservesZeroAttenuation(t *testing.T) {
	vm := newBuiltinsTestVM(4)
	vm.Globals = make([]float32, OFSParm4+2)
	vm.SetGInt(OFSParm0, 7)
	vm.SetGFloat(OFSParm1, 3)
	vm.SetGString(OFSParm2, "misc/hit.wav")
	vm.SetGFloat(OFSParm3, 1)
	vm.SetGFloat(OFSParm4, 0)

	var (
		gotEntNum      int
		gotChannel     int
		gotSample      string
		gotVolume      int
		gotAttenuation float32
	)
	vm.ServerHooks = ServerBuiltinHooks{
		Sound: func(_ *VM, entNum, channel int, sample string, volume int, attenuation float32) {
			gotEntNum = entNum
			gotChannel = channel
			gotSample = sample
			gotVolume = volume
			gotAttenuation = attenuation
		},
	}

	sound(vm)

	if gotEntNum != 7 || gotChannel != 3 || gotSample != "misc/hit.wav" {
		t.Fatalf("sound hook identity args = (%d,%d,%q), want (7,3,%q)", gotEntNum, gotChannel, gotSample, "misc/hit.wav")
	}
	if gotVolume != 255 {
		t.Fatalf("sound volume = %d, want 255", gotVolume)
	}
	if gotAttenuation != 0 {
		t.Fatalf("sound attenuation = %v, want 0", gotAttenuation)
	}
}

func TestTraceBuiltinsToggleVMTraceFlag(t *testing.T) {
	vm := newBuiltinsTestVM(4)

	traceOnBuiltin(vm)
	if !vm.Trace {
		t.Fatal("traceOnBuiltin should enable vm.Trace")
	}

	traceOffBuiltin(vm)
	if vm.Trace {
		t.Fatal("traceOffBuiltin should disable vm.Trace")
	}
}

func TestCoredumpBuiltinPrintsAllAllocatedEntities(t *testing.T) {
	vm := newBuiltinsTestVM(8)
	vm.NumEdicts = 3

	var printed []string
	console.SetPrintCallback(func(msg string) {
		printed = append(printed, msg)
	})
	t.Cleanup(func() {
		console.SetPrintCallback(nil)
	})

	coredumpBuiltin(vm)

	want := []string{"entity 0\n", "entity 1\n", "entity 2\n"}
	if len(printed) != len(want) {
		t.Fatalf("printed len = %d, want %d (%q)", len(printed), len(want), printed)
	}
	for i := range want {
		if printed[i] != want[i] {
			t.Fatalf("printed[%d] = %q, want %q", i, printed[i], want[i])
		}
	}
}

func TestSpawnAllocatesEntity(t *testing.T) {
	vm := newBuiltinsTestVM(8)

	spawn(vm)

	if got := int(vm.GInt(OFSReturn)); got != 1 {
		t.Fatalf("spawn return = %d, want 1", got)
	}
	if vm.NumEdicts != 2 {
		t.Fatalf("NumEdicts = %d, want 2", vm.NumEdicts)
	}
}

func TestRemoveClearsEntityData(t *testing.T) {
	vm := newBuiltinsTestVM(8)
	vm.NumEdicts = 2

	vm.SetEFloat(1, EntFieldHealth, 99)
	vm.SetEVector(1, EntFieldOrigin, [3]float32{1, 2, 3})
	vm.SetGInt(OFSParm0, 1)

	remove(vm)

	if got := vm.EFloat(1, EntFieldHealth); got != 0 {
		t.Fatalf("health after remove = %f, want 0", got)
	}
	if got := vm.EVector(1, EntFieldOrigin); got != [3]float32{} {
		t.Fatalf("origin after remove = %v, want zero", got)
	}
}

func TestSetOriginUpdatesAbsBounds(t *testing.T) {
	vm := newBuiltinsTestVM(8)
	vm.NumEdicts = 2

	vm.SetEVector(1, EntFieldMins, [3]float32{-1, -2, -3})
	vm.SetEVector(1, EntFieldMaxs, [3]float32{4, 5, 6})
	vm.SetGInt(OFSParm0, 1)
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
	vm := newBuiltinsTestVM(8)
	vm.NumEdicts = 2

	vm.SetEVector(1, EntFieldOrigin, [3]float32{10, 20, 30})
	vm.SetGInt(OFSParm0, 1)
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
	vm := newBuiltinsTestVM(8)
	vm.NumEdicts = 2

	vm.SetGInt(OFSParm0, 1)
	vm.SetGString(OFSParm1, "progs/test.mdl")

	setmodel(vm)

	modelIdx := vm.EInt(1, EntFieldModel)
	if got := vm.String(modelIdx); got != "progs/test.mdl" {
		t.Fatalf("model string = %q", got)
	}
	if got := vm.EFloat(1, EntFieldModelIndex); got != 1 {
		t.Fatalf("modelindex = %f, want 1", got)
	}
}

func TestPrecacheBuiltinsFallbackToCSQCHooks(t *testing.T) {
	SetCSQCClientHooks(CSQCClientHooks{})
	defer SetCSQCClientHooks(CSQCClientHooks{})

	vm := newBuiltinsTestVM(4)

	var gotSound string
	var gotModel string
	SetCSQCClientHooks(CSQCClientHooks{
		PrecacheSound: func(name string) int {
			gotSound = name
			return 1
		},
		PrecacheModel: func(name string) int {
			gotModel = name
			return 1
		},
	})

	vm.SetGString(OFSParm0, "weapons/rocket1i.wav")
	precacheSound(vm)
	if gotSound != "weapons/rocket1i.wav" {
		t.Fatalf("precacheSound CSQC hook name = %q, want %q", gotSound, "weapons/rocket1i.wav")
	}
	if got := vm.GString(OFSReturn); got != "weapons/rocket1i.wav" {
		t.Fatalf("precacheSound return = %q, want input string", got)
	}

	vm.SetGString(OFSParm0, "progs/player.mdl")
	precacheModel(vm)
	if gotModel != "progs/player.mdl" {
		t.Fatalf("precacheModel CSQC hook name = %q, want %q", gotModel, "progs/player.mdl")
	}
	if got := vm.GString(OFSReturn); got != "progs/player.mdl" {
		t.Fatalf("precacheModel return = %q, want input string", got)
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

	vm := newBuiltinsTestVM(8)
	vm.ServerHooks = ServerBuiltinHooks{
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
	}

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
	if got := int(vm.GInt(OFSReturn)); got != 10 {
		t.Fatalf("checkclient return = %d, want 10", got)
	}
	vm.SetGInt(OFSParm0, 1)
	vm.SetGFloat(OFSParm1, 0)
	aimBuiltin(vm)
	if got := vm.GVector(OFSReturn); got != [3]float32{0, 1, 0} {
		t.Fatalf("aim return = %v", got)
	}

	spawn(vm)
	if got := int(vm.GInt(OFSReturn)); got != 5 {
		t.Fatalf("spawn return = %d, want 5", got)
	}

	vm.SetGInt(OFSParm0, 1)
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
	if got := int(vm.GFloat(OFSReturn)); got != 1 {
		t.Fatalf("droptofloor return = %d, want 1", got)
	}
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

func TestDPrintFallbackRequiresDeveloperCvar(t *testing.T) {
	vm := newBuiltinsTestVM(1)
	vm.Cvars = cvar.NewCVarSystem()
	vm.SetGString(OFSParm0, "debug line\n")

	var printed []string
	console.SetPrintCallback(func(msg string) {
		printed = append(printed, msg)
	})
	t.Cleanup(func() {
		console.SetPrintCallback(nil)
	})

	dprint(vm)
	if len(printed) != 0 {
		t.Fatalf("dprint emitted with developer disabled: %q", printed)
	}

	vm.Cvars.Set("developer", "1")
	dprint(vm)
	if len(printed) != 1 || printed[0] != "debug line\n" {
		t.Fatalf("dprint emitted %q with developer enabled, want one debug line", printed)
	}
}
