package quake

import "testing"

func TestVec3Methods(t *testing.T) {
	v := MakeVec3(1, 2, 3)
	w := MakeVec3(4, 5, 6)

	if got, want := v.Add(w), (Vec3{5, 7, 9}); got != want {
		t.Fatalf("Add = %v, want %v", got, want)
	}
	if got, want := w.Sub(v), (Vec3{3, 3, 3}); got != want {
		t.Fatalf("Sub = %v, want %v", got, want)
	}
	if got, want := v.Mul(2), (Vec3{2, 4, 6}); got != want {
		t.Fatalf("Mul = %v, want %v", got, want)
	}
	if got, want := v.Div(2), (Vec3{0.5, 1, 1.5}); got != want {
		t.Fatalf("Div = %v, want %v", got, want)
	}
	if got, want := v.Neg(), (Vec3{-1, -2, -3}); got != want {
		t.Fatalf("Neg = %v, want %v", got, want)
	}
	if got, want := v.Dot(w), float32(32); got != want {
		t.Fatalf("Dot = %v, want %v", got, want)
	}
	if got, want := v.Cross(w), (Vec3{-3, 6, -3}); got != want {
		t.Fatalf("Cross = %v, want %v", got, want)
	}
	if got, want := v.Lerp(w, 0.25), (Vec3{1.75, 2.75, 3.75}); got != want {
		t.Fatalf("Lerp = %v, want %v", got, want)
	}
}

func TestVec3OperatorEmulationHelpers(t *testing.T) {
	a := MakeVec3(2, -4, 8)
	b := MakeVec3(1, 2, 3)

	if got, want := OpAddVV(a, b), (Vec3{3, -2, 11}); got != want {
		t.Fatalf("OpAddVV = %v, want %v", got, want)
	}
	if got, want := OpSubVV(a, b), (Vec3{1, -6, 5}); got != want {
		t.Fatalf("OpSubVV = %v, want %v", got, want)
	}
	if got, want := OpMulVF(a, 0.5), (Vec3{1, -2, 4}); got != want {
		t.Fatalf("OpMulVF = %v, want %v", got, want)
	}
	if got, want := OpMulFV(0.5, a), (Vec3{1, -2, 4}); got != want {
		t.Fatalf("OpMulFV = %v, want %v", got, want)
	}
	if got, want := OpMulVV(a, b), float32(18); got != want {
		t.Fatalf("OpMulVV = %v, want %v", got, want)
	}
	if got, want := OpDivVF(a, 2), (Vec3{1, -2, 4}); got != want {
		t.Fatalf("OpDivVF = %v, want %v", got, want)
	}
	if got, want := OpNegV(a), (Vec3{-2, 4, -8}); got != want {
		t.Fatalf("OpNegV = %v, want %v", got, want)
	}
}

func TestEntityFlagsHelpers(t *testing.T) {
	e := &Entity{}
	e.SetFlagsValue(FlagClient | FlagMonster)

	if got, want := e.Flags, float32(40); got != want {
		t.Fatalf("Flags float storage = %v, want %v", got, want)
	}
	if !e.HasFlags(FlagClient) {
		t.Fatalf("HasFlags(FlagClient) = false, want true")
	}
	if e.HasFlags(FlagInWater) {
		t.Fatalf("HasFlags(FlagInWater) = true, want false")
	}

	e.AddFlags(FlagInWater)
	if !e.HasFlags(FlagClient | FlagInWater) {
		t.Fatalf("HasFlags(FlagClient|FlagInWater) = false, want true")
	}

	e.ClearFlags(FlagClient)
	if e.HasFlags(FlagClient) {
		t.Fatalf("HasFlags(FlagClient) = true after clear, want false")
	}
	if got, want := e.FlagsValue(), (FlagMonster | FlagInWater); got != want {
		t.Fatalf("FlagsValue = %v, want %v", got, want)
	}
}

func TestEntityFlagHelpersNilSafetyAndSpawnFlags(t *testing.T) {
	var e *Entity
	e.SetFlagsValue(FlagClient)
	e.AddFlags(FlagMonster)
	e.ClearFlags(FlagInWater)
	e.SetSpawnFlagsValue(FlagObjective)

	if got := e.FlagsValue(); got != 0 {
		t.Fatalf("nil FlagsValue = %v, want 0", got)
	}
	if e.HasFlags(FlagClient) {
		t.Fatalf("nil HasFlags(FlagClient) = true, want false")
	}
	if got := e.SpawnFlagsValue(); got != 0 {
		t.Fatalf("nil SpawnFlagsValue = %v, want 0", got)
	}

	ent := &Entity{}
	ent.SetSpawnFlagsValue(FlagFly | FlagNoMonsters)
	if got, want := ent.SpawnFlags, float32(32769); got != want {
		t.Fatalf("SpawnFlags float storage = %v, want %v", got, want)
	}
	if got, want := ent.SpawnFlagsValue(), (FlagFly | FlagNoMonsters); got != want {
		t.Fatalf("SpawnFlagsValue = %v, want %v", got, want)
	}
}
