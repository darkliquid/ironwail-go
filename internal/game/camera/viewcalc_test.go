package camera

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

// mockCVars is a minimal CVarReader for tests.
type mockCVars map[string]*cvar.CVar

func (m mockCVars) Get(name string) *cvar.CVar {
	return m[name]
}

func floatCVar(v float64) *cvar.CVar {
	return &cvar.CVar{Float: v, Int: int(v)}
}

func TestCalcBobNoCvars(t *testing.T) {
	// With no cvars registered, bob is zero (matches nil-guard behavior).
	if got := CalcBob(mockCVars{}, 0, qtypes.Vec3{}); got != 0 {
		t.Fatalf("CalcBob with no cvars = %v, want 0", got)
	}
}

func TestCalcBobZeroCycle(t *testing.T) {
	cv := mockCVars{
		"cl_bobcycle": floatCVar(0),
		"cl_bobup":    floatCVar(0.5),
		"cl_bob":      floatCVar(0.02),
	}
	if got := CalcBob(cv, 10, qtypes.Vec3{}); got != 0 {
		t.Fatalf("CalcBob with zero bobcycle = %v, want 0", got)
	}
}

func TestCalcBobClamps(t *testing.T) {
	cv := mockCVars{
		"cl_bobcycle": floatCVar(2),
		"cl_bobup":    floatCVar(0.5),
		"cl_bob":      floatCVar(1),
	}
	// Large velocity forces bob component; result must stay in [-7, 4].
	got := CalcBob(cv, 0, qtypes.Vec3{X: 1000, Y: 1000, Z: 0})
	if got > 4 || got < -7 {
		t.Fatalf("CalcBob out of range: %v", got)
	}
}

func TestCalcBobStationary(t *testing.T) {
	cv := mockCVars{
		"cl_bobcycle": floatCVar(2),
		"cl_bobup":    floatCVar(0.5),
		"cl_bob":      floatCVar(0.02),
	}
	// Zero velocity -> zero speed component; still cycles the sin term.
	got := CalcBob(cv, 1, qtypes.Vec3{})
	// speed is 0, so bob = 0*0.3 + 0*0.7*sin(...) = 0
	if got != 0 {
		t.Fatalf("CalcBob stationary = %v, want 0", got)
	}
}

func TestCalcRollNoCvars(t *testing.T) {
	if got := CalcRoll(mockCVars{}, qtypes.Vec3{}, qtypes.Vec3{}); got != 0 {
		t.Fatalf("CalcRoll with no cvars = %v, want 0", got)
	}
}

func TestCalcRollPositiveSide(t *testing.T) {
	cv := mockCVars{
		"cl_rollangle": floatCVar(2),
		"cl_rollspeed": floatCVar(1),
	}
	// At yaw 0, Quake's right vector points -Y, so +Y motion yields negative
	// side and the sign flips. Large side saturates at rollangle.
	got := CalcRoll(cv, qtypes.Vec3{}, qtypes.Vec3{X: 0, Y: 400, Z: 0})
	if got != -2 {
		t.Fatalf("CalcRoll +Y side = %v, want -2 (sign flip at yaw 0)", got)
	}
}

func TestCalcRollNegativeSign(t *testing.T) {
	cv := mockCVars{
		"cl_rollangle": floatCVar(2),
		"cl_rollspeed": floatCVar(1),
	}
	got := CalcRoll(cv, qtypes.Vec3{}, qtypes.Vec3{X: 0, Y: -400, Z: 0})
	if got != 2 {
		t.Fatalf("CalcRoll -Y side = %v, want 2 (sign flip at yaw 0)", got)
	}
}

func TestCalcRollZeroSpeed(t *testing.T) {
	cv := mockCVars{
		"cl_rollangle": floatCVar(2),
		"cl_rollspeed": floatCVar(0),
	}
	if got := CalcRoll(cv, qtypes.Vec3{}, qtypes.Vec3{X: 0, Y: 400, Z: 0}); got != 0 {
		t.Fatalf("CalcRoll zero rollspeed = %v, want 0", got)
	}
}

func TestAddIdleNoCvars(t *testing.T) {
	angles := qtypes.Vec3{X: 1, Y: 2, Z: 3}
	got := AddIdle(mockCVars{}, angles, 0)
	if got != angles {
		t.Fatalf("AddIdle no cvars = %v, want unchanged %v", got, angles)
	}
}

func TestAddIdleAppliesSway(t *testing.T) {
	cv := mockCVars{
		"v_idlescale":    floatCVar(1),
		"v_iroll_cycle":  floatCVar(1),
		"v_iroll_level":  floatCVar(0.5),
		"v_ipitch_cycle": floatCVar(1),
		"v_ipitch_level": floatCVar(0.5),
		"v_iyaw_cycle":   floatCVar(1),
		"v_iyaw_level":   floatCVar(0.5),
	}
	got := AddIdle(cv, qtypes.Vec3{}, 0)
	// sin(0)=0 for all -> angles unchanged.
	if got != (qtypes.Vec3{}) {
		t.Fatalf("AddIdle at t=0 = %v, want unchanged", got)
	}
	got = AddIdle(cv, qtypes.Vec3{}, 3.14159/2)
	// sin(pi/2) = 1 for each -> each axis += 0.5
	if !approx(got.X, 0.5, 0.01) || !approx(got.Y, 0.5, 0.01) || !approx(got.Z, 0.5, 0.01) {
		t.Fatalf("AddIdle at t=pi/2 = %v, want ~0.5 on each axis", got)
	}
}

func TestApplyViewmodelQuakeFudgeDisabled(t *testing.T) {
	cv := mockCVars{"r_viewmodel_quake": &cvar.CVar{}}
	origin := qtypes.Vec3{X: 1, Y: 2, Z: 3}
	got := ApplyViewmodelQuakeFudge(cv, origin, 100)
	if got != origin {
		t.Fatalf("fudge disabled = %v, want unchanged %v", got, origin)
	}
}

func TestApplyViewmodelQuakeFudgeSizes(t *testing.T) {
	cv := mockCVars{"r_viewmodel_quake": &cvar.CVar{Int: 1, Float: 1}}
	// scr_viewsize 100 -> +2 Z
	got := ApplyViewmodelQuakeFudge(cv, qtypes.Vec3{}, 100)
	if got.Z != 2 {
		t.Fatalf("fudge size 100 Z = %v, want 2", got.Z)
	}
	got = ApplyViewmodelQuakeFudge(cv, qtypes.Vec3{}, 80)
	if got.Z != 0.5 {
		t.Fatalf("fudge size 80 Z = %v, want 0.5", got.Z)
	}
	got = ApplyViewmodelQuakeFudge(cv, qtypes.Vec3{}, 50)
	if got.Z != 0 {
		t.Fatalf("fudge size 50 Z = %v, want 0 (unhandled size)", got.Z)
	}
}
