package qc

// Math, string, and random builtin tests split from builtins_test.go.

import (
	"math"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cmdsys"
	"github.com/darkliquid/ironwail-go/internal/compatrand"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

func TestRegisterBuiltinsCanonicalMappings(t *testing.T) {
	vm := newBuiltinsTestVM(8)
	RegisterBuiltins(vm)

	for _, slot := range []int{6, 8, 10, 11, 16, 17, 19, 20, 21, 23, 24, 25, 28, 31, 35, 36, 37, 38, 40, 41, 43, 44, 45, 46, 48, 52, 53, 54, 55, 56, 57, 58, 59, 68, 69, 70, 72, 73, 74, 78, 79, 80, 316, 317, 318, 320, 321, 322, 323, 324, 325, 326, 327, 328} {
		if vm.Builtins[slot] == nil {
			t.Fatalf("builtin %d is nil", slot)
		}
	}
	if vm.Builtins[1000] == nil {
		t.Fatalf("temporary findfloat helper slot is nil")
	}
}

func TestMathCVarAndLocalCmdBuiltins(t *testing.T) {
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

	vm.Cvars.Set("qc_test_var", "12.5")
	vm.SetGString(OFSParm0, "qc_test_var")
	cvarBuiltin(vm)
	if got := vm.GFloat(OFSReturn); got != 12.5 {
		t.Fatalf("cvar = %v, want 12.5", got)
	}

	vm.SetGString(OFSParm0, "qc_test_set")
	vm.SetGString(OFSParm1, "99")
	cvarSetBuiltin(vm)
	if got := vm.Cvars.StringValue("qc_test_set"); got != "99" {
		t.Fatalf("cvar_set stored %q, want 99", got)
	}

	executed := false
	cs := cmdsys.NewCmdSystem()
	cs.AddCommand("qc_test_cmd", func(args []string) { executed = true }, "")
	prev := vm.ServerHooks.LocalCommand
	vm.ServerHooks.LocalCommand = func(_ *VM, cmd string) { cs.AddText(cmd) }
	t.Cleanup(func() { vm.ServerHooks.LocalCommand = prev })
	vm.SetGString(OFSParm0, "qc_test_cmd\n")
	localcmd(vm)
	cs.Execute()
	if !executed {
		t.Fatal("localcmd did not enqueue command")
	}
}

func TestVectoyawBuiltinMatchesQuakeYaw(t *testing.T) {
	vm := newBuiltinsTestVM(1)

	tests := []struct {
		name string
		vec  qtypes.Vec3
		want float32
	}{
		{name: "zero", vec: qtypes.Vec3{X: 0, Y: 0, Z: 0}, want: 0},
		{name: "positive x", vec: qtypes.Vec3{X: 1, Y: 0, Z: 0}, want: 0},
		{name: "positive y", vec: qtypes.Vec3{X: 0, Y: 1, Z: 0}, want: 90},
		{name: "negative x", vec: qtypes.Vec3{X: -1, Y: 0, Z: 0}, want: 180},
		{name: "negative y", vec: qtypes.Vec3{X: 0, Y: -1, Z: 0}, want: 270},
		{name: "diagonal", vec: qtypes.Vec3{X: 1, Y: 1, Z: 0}, want: 45},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vm.SetGVector(OFSParm0, tc.vec)
			vectoyaw(vm)
			if got := vm.GFloat(OFSReturn); math.Abs(float64(got-tc.want)) > 0.001 {
				t.Fatalf("vectoyaw(%v) = %v, want %v", tc.vec, got, tc.want)
			}
		})
	}
}

func TestVectoanglesBuiltinUsesQuakeYawConvention(t *testing.T) {
	vm := newBuiltinsTestVM(1)
	vm.SetGVector(OFSParm0, qtypes.Vec3{X: 0, Y: 1, Z: 0})

	vectoangles(vm)

	if got := vm.GVector(OFSReturn); math.Abs(float64(got.X)) > 0.001 || math.Abs(float64(got.Y-90)) > 0.001 || math.Abs(float64(got.Z)) > 0.001 {
		t.Fatalf("vectoangles yaw = %v, want [0 90 0]", got)
	}
}

func TestVectoanglesBuiltinVerticalCasesMatchC(t *testing.T) {
	vm := newBuiltinsTestVM(1)
	tests := []struct {
		name string
		vec  qtypes.Vec3
		want qtypes.Vec3
	}{
		{name: "straight up", vec: qtypes.Vec3{X: 0, Y: 0, Z: 1}, want: qtypes.Vec3{X: 90, Y: 0, Z: 0}},
		{name: "straight down", vec: qtypes.Vec3{X: 0, Y: 0, Z: -1}, want: qtypes.Vec3{X: 270, Y: 0, Z: 0}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vm.SetGVector(OFSParm0, tc.vec)
			vectoangles(vm)
			if got := vm.GVector(OFSReturn); got != tc.want {
				t.Fatalf("vectoangles(%v) = %v, want %v", tc.vec, got, tc.want)
			}
		})
	}
}

func TestMakevectorsMatchesQuakeAngleVectors(t *testing.T) {
	vm := newBuiltinsTestVM(1)

	tests := []struct {
		name   string
		angles qtypes.Vec3
	}{
		{name: "yaw ninety", angles: qtypes.Vec3{X: 0, Y: 90, Z: 0}},
		{name: "pitch yaw", angles: qtypes.Vec3{X: 30, Y: 45, Z: 0}},
		{name: "pitch yaw roll", angles: qtypes.Vec3{X: 10, Y: 20, Z: 30}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vm.SetGVector(OFSParm0, tc.angles)
			makevectors(vm)

			wantForward, wantRight, wantUp := qtypes.AngleVectors(tc.angles)

			assertVecNear := func(name string, got, want qtypes.Vec3) {
				if math.Abs(float64(got.X-want.X)) > 0.001 || math.Abs(float64(got.Y-want.Y)) > 0.001 || math.Abs(float64(got.Z-want.Z)) > 0.001 {
					t.Fatalf("%s = %v, want [%v %v %v]", name, got, want.X, want.Y, want.Z)
				}
			}

			assertVecNear("v_forward", vm.GVector(OFSGlobalVForward), wantForward)
			assertVecNear("v_right", vm.GVector(OFSGlobalVRight), wantRight)
			assertVecNear("v_up", vm.GVector(OFSGlobalVUp), wantUp)

			if tc.name == "pitch yaw roll" {
				if got := vm.GVector(OFSGlobalVUp); math.Abs(float64(got.X)) < 0.001 && math.Abs(float64(got.Y)) < 0.001 && math.Abs(float64(got.Z-1)) < 0.001 {
					t.Fatalf("v_up unexpectedly stayed world-up for rolled angles: %v", got)
				}
				if got := vm.GVector(OFSGlobalVRight); math.Abs(float64(got.Z)) < 0.001 {
					t.Fatalf("v_right z = %v, want non-zero for rolled angles", got.Z)
				}
			}
		})
	}
}

func TestNormalizeBuiltinReturnsUnitVector(t *testing.T) {
	vm := newBuiltinsTestVM(1)
	vm.SetGVector(OFSParm0, qtypes.Vec3{X: 3, Y: 4, Z: 0})

	normalize(vm)

	got := vm.GVector(OFSReturn)
	if math.Abs(float64(got.X-0.6)) > 0.001 || math.Abs(float64(got.Y-0.8)) > 0.001 || math.Abs(float64(got.Z)) > 0.001 {
		t.Fatalf("normalize return = %v, want [0.6 0.8 0]", got)
	}
}

func TestNormalizeBuiltinZeroVector(t *testing.T) {
	vm := newBuiltinsTestVM(1)
	vm.SetGVector(OFSParm0, qtypes.Vec3{X: 0, Y: 0, Z: 0})

	normalize(vm)

	if got := vm.GVector(OFSReturn); got != (qtypes.Vec3{}) {
		t.Fatalf("normalize zero return = %v, want zero vector", got)
	}
}

func TestSearchBuiltinsFallback(t *testing.T) {
	vm := newBuiltinsTestVM(8)
	vm.NumEdicts = 4

	vm.SetEInt(1, EntFieldTargetName, vm.AllocString("door"))
	vm.SetEVector(1, EntFieldOrigin, qtypes.Vec3{X: 100, Y: 0, Z: 0})
	vm.SetEInt(2, EntFieldTargetName, vm.AllocString("trigger"))
	vm.SetEFloat(2, EntFieldHealth, 100)
	vm.SetEVector(2, EntFieldOrigin, qtypes.Vec3{X: 10, Y: 0, Z: 0})
	vm.SetEVector(3, EntFieldOrigin, qtypes.Vec3{X: 40, Y: 0, Z: 0})

	vm.SetGInt(OFSParm0, 0)
	vm.SetGInt(OFSParm1, EntFieldTargetName)
	vm.SetGString(OFSParm2, "trigger")
	find(vm)
	if got := int(vm.GInt(OFSReturn)); got != 2 {
		t.Fatalf("find return = %d, want 2", got)
	}

	vm.SetGInt(OFSParm0, 0)
	vm.SetGInt(OFSParm1, EntFieldHealth)
	vm.SetGFloat(OFSParm2, 100)
	findfloat(vm)
	if got := int(vm.GInt(OFSReturn)); got != 2 {
		t.Fatalf("findfloat return = %d, want 2", got)
	}

	vm.SetGInt(OFSParm0, 1)
	nextent(vm)
	if got := int(vm.GInt(OFSReturn)); got != 2 {
		t.Fatalf("nextent return = %d, want 2", got)
	}

	vm.SetGVector(OFSParm0, qtypes.Vec3{X: 0, Y: 0, Z: 0})
	vm.SetGFloat(OFSParm1, 15)
	findradius(vm)
	if got := int(vm.GInt(OFSReturn)); got != 2 {
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
		{"sin(pi/2)", sinBuiltin, math.Pi / 2, 1.0, 0.001},
		{"sin(0)", sinBuiltin, 0, 0.0, 0.001},
		{"cos(0)", cosBuiltin, 0, 1.0, 0.001},
		{"cos(pi/2)", cosBuiltin, math.Pi / 2, 0.0, 0.001},
		{"sqrt(4)", sqrtBuiltin, 4, 2.0, 0.001},
		{"sqrt(9)", sqrtBuiltin, 9, 3.0, 0.001},
		{"tan(pi/4)", tanBuiltin, math.Pi / 4, 1.0, 0.001},
		{"asin(1)", asinBuiltin, 1, math.Pi / 2, 0.001},
		{"acos(0)", acosBuiltin, 0, math.Pi / 2, 0.001},
		{"atan(1)", atanBuiltin, 1, math.Pi / 4, 0.001},
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

	vm.SetGFloat(OFSParm0, 1)
	vm.SetGFloat(OFSParm0+3, 1)
	atan2Builtin(vm)
	if got := vm.GFloat(OFSReturn); math.Abs(float64(got-math.Pi/4)) > 0.001 {
		t.Errorf("atan2(1,1) = %v, want %v", got, math.Pi/4)
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

	vm.ArgC = 4
	vm.SetGFloat(OFSParm0, 9)
	vm.SetGFloat(OFSParm0+3, 5)
	vm.SetGFloat(OFSParm0+6, -3)
	vm.SetGFloat(OFSParm0+9, 7)
	minBuiltin(vm)
	if got := vm.GFloat(OFSReturn); got != -3 {
		t.Errorf("min(9,5,-3,7) = %v, want -3", got)
	}
	maxBuiltin(vm)
	if got := vm.GFloat(OFSReturn); got != 9 {
		t.Errorf("max(9,5,-3,7) = %v, want 9", got)
	}
	vm.ArgC = 0
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

	vm.ArgC = 3
	vm.SetGString(OFSParm0, "foo")
	vm.SetGString(OFSParm0+3, "bar")
	vm.SetGString(OFSParm0+6, "baz")
	strcatBuiltin(vm)
	if got := vm.GString(OFSReturn); got != "foobarbaz" {
		t.Errorf("strcat(foo,bar,baz) = %q, want foobarbaz", got)
	}
	vm.ArgC = 0

	// substring("hello world", 6, 5) = "world"
	vm.SetGString(OFSParm0, "hello world")
	vm.SetGFloat(OFSParm0+3, 6)
	vm.SetGFloat(OFSParm0+6, 5)
	substringBuiltin(vm)
	if got := vm.GString(OFSReturn); got != "world" {
		t.Errorf("substring(hello world,6,5) = %q, want world", got)
	}

	vm.SetGString(OFSParm0, "hello world")
	vm.SetGFloat(OFSParm0+3, -5)
	vm.SetGFloat(OFSParm0+6, 5)
	substringBuiltin(vm)
	if got := vm.GString(OFSReturn); got != "world" {
		t.Errorf("substring(hello world,-5,5) = %q, want world", got)
	}

	vm.SetGString(OFSParm0, "hello world")
	vm.SetGFloat(OFSParm0+3, 6)
	vm.SetGFloat(OFSParm0+6, -1)
	substringBuiltin(vm)
	if got := vm.GString(OFSReturn); got != "world" {
		t.Errorf("substring(hello world,6,-1) = %q, want world", got)
	}

	// stov("'1 2 3'") = [1,2,3]
	vm.SetGString(OFSParm0, "'1 2 3'")
	stovBuiltin(vm)
	if got := vm.GVector(OFSReturn); got != (qtypes.Vec3{X: 1, Y: 2, Z: 3}) {
		t.Errorf("stov('1 2 3') = %v, want [1 2 3]", got)
	}

	// stof("3.14") = 3.14
	vm.SetGString(OFSParm0, "3.14")
	stofBuiltin(vm)
	got := vm.GFloat(OFSReturn)
	if got < 3.13 || got > 3.15 {
		t.Errorf("stof(3.14) = %v, want ~3.14", got)
	}

	// etos(42) = "entity 42"
	vm.SetGInt(OFSParm0, 42)
	etosBuiltin(vm)
	if got := vm.GString(OFSReturn); got != "entity 42" {
		t.Errorf("etos(42) = %q, want \"entity 42\"", got)
	}

	// chr2str(65) = "A"
	vm.SetGFloat(OFSParm0, 65)
	chr2strBuiltin(vm)
	if got := vm.GString(OFSReturn); got != "A" {
		t.Errorf("chr2str(65) = %q, want A", got)
	}

	vm.ArgC = 2
	vm.SetGString(OFSParm0, "hello")
	vm.SetGString(OFSParm0+3, " world")
	strzoneBuiltin(vm)
	if got := vm.GString(OFSReturn); got != "hello world" {
		t.Errorf("strzone(hello, world) = %q, want hello world", got)
	}
	vm.ArgC = 0

	// str2chr("A", 0) = 65
	vm.SetGString(OFSParm0, "A")
	vm.SetGFloat(OFSParm0+3, 0)
	str2chrBuiltin(vm)
	if got := vm.GFloat(OFSReturn); got != 65 {
		t.Errorf("str2chr(A,0) = %v, want 65", got)
	}

	vm.SetGString(OFSParm0, "ABC")
	vm.SetGFloat(OFSParm0+3, -1)
	str2chrBuiltin(vm)
	if got := vm.GFloat(OFSReturn); got != 'C' {
		t.Errorf("str2chr(ABC,-1) = %v, want %d", got, 'C')
	}

	vm.ArgC = 3
	vm.SetGFloat(OFSParm0, 'Q')
	vm.SetGFloat(OFSParm0+3, 10)
	vm.SetGFloat(OFSParm0+6, 500)
	chr2strBuiltin(vm)
	if got := vm.GString(OFSReturn); got != "Q\n?" {
		t.Errorf("chr2str variadic = %q, want %q", got, "Q\n?")
	}
	vm.ArgC = 0
}

func TestRandomBuiltinDistribution(t *testing.T) {
	vm := newBuiltinsTestVM(4)
	vm.Cvars.Set("sv_gameplayfix_random", "1")
	t.Cleanup(func() { vm.Cvars.Set("sv_gameplayfix_random", "1") })

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

func TestRandomBuiltinMatchesCompatSequence(t *testing.T) {
	vm := newBuiltinsTestVM(4)
	vm.Cvars.Set("sv_gameplayfix_random", "1")
	t.Cleanup(func() { vm.Cvars.Set("sv_gameplayfix_random", "1") })

	want := []float32{
		0.54222107,
		0.27949524,
		0.1907196,
	}

	for i, wantValue := range want {
		random(vm)
		if got := vm.GFloat(OFSReturn); got != wantValue {
			t.Fatalf("random value %d = %v, want %v", i, got, wantValue)
		}
	}
}

func TestRandomBuiltinUsesInjectedCompatRNGState(t *testing.T) {
	vm := newBuiltinsTestVM(4)
	vm.Cvars.Set("sv_gameplayfix_random", "1")
	t.Cleanup(func() { vm.Cvars.Set("sv_gameplayfix_random", "1") })

	rng := compatrand.NewSeed(1)
	if got := rng.Int(); got != 1804289383 {
		t.Fatalf("first compat rand draw = %d, want 1804289383", got)
	}
	vm.SetCompatRNG(rng)

	random(vm)
	if got := vm.GFloat(OFSReturn); got != 0.27949524 {
		t.Fatalf("random() after shared upstream draw = %v, want 0.27949524", got)
	}
}

func TestRandomBuiltinTraceParityDeltaFromUpstreamRandDraw(t *testing.T) {
	const sampleSize = 5

	makeTrace := func(consumeUpstream bool, gameplayFix string) []float32 {
		vm := newBuiltinsTestVM(4)
		vm.Cvars.Set("sv_gameplayfix_random", gameplayFix)

		rng := compatrand.NewSeed(1)
		if consumeUpstream {
			rng.Int()
		}
		vm.SetCompatRNG(rng)

		trace := make([]float32, 0, sampleSize)
		for range sampleSize {
			random(vm)
			trace = append(trace, vm.GFloat(OFSReturn))
		}
		return trace
	}

	t.Run("gameplayfix-on", func(t *testing.T) {
		zeroOffset := makeTrace(false, "1")
		oneDrawOffset := makeTrace(true, "1")

		wantZeroOffset := []float32{
			0.54222107,
			0.27949524,
			0.1907196,
			0.5660248,
			0.7212372,
		}
		wantOneDrawOffset := []float32{
			0.27949524,
			0.1907196,
			0.5660248,
			0.7212372,
			0.72654724,
		}

		for i := range wantZeroOffset {
			if got := zeroOffset[i]; got != wantZeroOffset[i] {
				t.Fatalf("gameplayfix=1 zero-offset[%d] = %v, want %v", i, got, wantZeroOffset[i])
			}
			if got := oneDrawOffset[i]; got != wantOneDrawOffset[i] {
				t.Fatalf("gameplayfix=1 one-draw-offset[%d] = %v, want %v", i, got, wantOneDrawOffset[i])
			}
			if i+1 < len(zeroOffset) && oneDrawOffset[i] != zeroOffset[i+1] {
				t.Fatalf("gameplayfix=1 delta mismatch at %d: one-draw=%v, expected shifted=%v", i, oneDrawOffset[i], zeroOffset[i+1])
			}
		}
	})

	t.Run("gameplayfix-off", func(t *testing.T) {
		zeroOffset := makeTrace(false, "0")
		oneDrawOffset := makeTrace(true, "0")

		wantZeroOffset := []float32{
			0.5422224,
			0.2794885,
			0.19071017,
			0.5660268,
			0.7212439,
		}
		wantOneDrawOffset := []float32{
			0.2794885,
			0.19071017,
			0.5660268,
			0.7212439,
			0.72655416,
		}

		for i := range wantZeroOffset {
			if got := zeroOffset[i]; got != wantZeroOffset[i] {
				t.Fatalf("gameplayfix=0 zero-offset[%d] = %v, want %v", i, got, wantZeroOffset[i])
			}
			if got := oneDrawOffset[i]; got != wantOneDrawOffset[i] {
				t.Fatalf("gameplayfix=0 one-draw-offset[%d] = %v, want %v", i, got, wantOneDrawOffset[i])
			}
			if i+1 < len(zeroOffset) && oneDrawOffset[i] != zeroOffset[i+1] {
				t.Fatalf("gameplayfix=0 delta mismatch at %d: one-draw=%v, expected shifted=%v", i, oneDrawOffset[i], zeroOffset[i+1])
			}
		}
	})
}

func TestRandomBuiltinLegacyFormulaWhenGameplayFixDisabled(t *testing.T) {
	vm := newBuiltinsTestVM(4)
	vm.Cvars.Set("sv_gameplayfix_random", "0")
	t.Cleanup(func() { vm.Cvars.Set("sv_gameplayfix_random", "1") })

	want := []float32{
		0.5422224,
		0.2794885,
		0.19071017,
	}

	for i, wantValue := range want {
		random(vm)
		if got := vm.GFloat(OFSReturn); got != wantValue {
			t.Fatalf("legacy random value %d = %v, want %v", i, got, wantValue)
		}
	}
}
