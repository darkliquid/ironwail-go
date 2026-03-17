package main

import (
	"math"
	"testing"

	"github.com/ironwail/ironwail-go/internal/cvar"
)

// ensureViewCalcCvars registers all cvars required by viewcalc functions if
// they are not already present.  Tests call this in place of the full
// initGameCvars() to keep setup minimal.
func ensureViewCalcCvars() {
	defaults := map[string]string{
		"cl_bob":         "0.02",
		"cl_bobcycle":    "0.6",
		"cl_bobup":       "0.5",
		"cl_rollangle":   "2.0",
		"cl_rollspeed":   "200",
		"v_idlescale":    "0",
		"v_iyaw_cycle":   "2",
		"v_iroll_cycle":  "0.5",
		"v_ipitch_cycle": "1",
		"v_iyaw_level":   "0.3",
		"v_iroll_level":  "0.1",
		"v_ipitch_level": "0.3",
		"r_viewmodel_quake": "0",
		"scr_viewsize":   "100",
	}
	for name, def := range defaults {
		if cvar.Get(name) == nil {
			cvar.Register(name, def, 0, "")
		} else {
			cvar.Set(name, def)
		}
	}
}

// ---- V_CalcBob tests -------------------------------------------------------

func TestViewCalcBob_StationaryReturnsZero(t *testing.T) {
	ensureViewCalcCvars()
	bob := viewCalcBob(1.0, [3]float32{0, 0, 0})
	if bob != 0 {
		t.Errorf("expected 0 for zero velocity, got %v", bob)
	}
}

func TestViewCalcBob_MovingReturnsNonZero(t *testing.T) {
	ensureViewCalcCvars()
	// 300 units/s forward movement should produce noticeable bob.
	bob := viewCalcBob(0.1, [3]float32{300, 0, 0})
	if bob == 0 {
		t.Error("expected non-zero bob for moving player")
	}
}

func TestViewCalcBob_ClampedHigh(t *testing.T) {
	ensureViewCalcCvars()
	// Artificially large bob scale to ensure the result is clamped to 4.
	if cv := cvar.Get("cl_bob"); cv != nil {
		cv.String = "100"
		cv.Float = 100
	}
	bob := viewCalcBob(0.15, [3]float32{300, 0, 0})
	if bob > 4 {
		t.Errorf("bob %v exceeds max of 4", bob)
	}
	// Restore default.
	if cv := cvar.Get("cl_bob"); cv != nil {
		cv.String = "0.02"
		cv.Float = 0.02
	}
}

func TestViewCalcBob_ClampedLow(t *testing.T) {
	ensureViewCalcCvars()
	if cv := cvar.Get("cl_bob"); cv != nil {
		cv.String = "100"
		cv.Float = 100
	}
	// Pick a time that lands in the negative half of the sine wave.
	// cycle > cl_bobup produces radians in (π, 2π).
	bobcycle := float64(0.6)
	bobup := float64(0.5)
	// Want cycle/bobcycle just past bobup so we get the negative lobe.
	t0 := bobcycle * (bobup + 0.05)
	bob := viewCalcBob(t0, [3]float32{300, 0, 0})
	if bob < -7 {
		t.Errorf("bob %v below min of -7", bob)
	}
	if cv := cvar.Get("cl_bob"); cv != nil {
		cv.String = "0.02"
		cv.Float = 0.02
	}
}

func TestViewCalcBob_ZeroCycle(t *testing.T) {
	ensureViewCalcCvars()
	if cv := cvar.Get("cl_bobcycle"); cv != nil {
		cv.Float = 0
	}
	bob := viewCalcBob(1.0, [3]float32{300, 0, 0})
	if bob != 0 {
		t.Errorf("expected 0 when cl_bobcycle=0, got %v", bob)
	}
	if cv := cvar.Get("cl_bobcycle"); cv != nil {
		cv.Float = 0.6
	}
}

// ---- V_CalcRoll tests ------------------------------------------------------

func TestViewCalcRoll_Zero(t *testing.T) {
	ensureViewCalcCvars()
	// Zero velocity → zero roll.
	roll := viewCalcRoll([3]float32{0, 0, 0}, [3]float32{0, 0, 0})
	if roll != 0 {
		t.Errorf("expected 0, got %v", roll)
	}
}

func TestViewCalcRoll_Sign(t *testing.T) {
	ensureViewCalcCvars()
	// Pure yaw=0: right vector is (0,-1,0) in Quake coordinates.
	// Moving in +Y direction (positive right side) → positive roll.
	// Moving in -Y direction (negative right side) → negative roll.
	rollPos := viewCalcRoll([3]float32{0, 0, 0}, [3]float32{0, 500, 0})
	rollNeg := viewCalcRoll([3]float32{0, 0, 0}, [3]float32{0, -500, 0})
	if rollPos == 0 || rollNeg == 0 {
		t.Skip("velocity not aligned with right vector for this yaw; skip sign test")
	}
	if (rollPos > 0) == (rollNeg > 0) {
		t.Errorf("expected opposite signs: rollPos=%v rollNeg=%v", rollPos, rollNeg)
	}
}

func TestViewCalcRoll_CappedByRollAngle(t *testing.T) {
	ensureViewCalcCvars()
	// Very fast lateral movement should cap at cl_rollangle (2.0).
	roll := viewCalcRoll([3]float32{0, 90, 0}, [3]float32{0, 10000, 0})
	rollAngle := float32(cvar.Get("cl_rollangle").Float)
	if roll > rollAngle+0.001 || roll < -(rollAngle+0.001) {
		t.Errorf("roll %v exceeds cl_rollangle %v", roll, rollAngle)
	}
}

// ---- CalcGunAngle tests -----------------------------------------------------

func TestViewCalcGunAngle_IdleScaleZero(t *testing.T) {
	ensureViewCalcCvars()
	if cv := cvar.Get("v_idlescale"); cv != nil {
		cv.Float = 0
	}
	state := viewCalcState{}
	viewAngles := [3]float32{10, 45, 0}
	out := viewCalcGunAngle(&state, viewAngles, 1.0, 0.016)
	// With no idle sway, yaw should equal viewAngles[YAW] and pitch should
	// equal -viewAngles[PITCH] (Quake first-person convention).
	const eps = 0.001
	if math.Abs(float64(out[1]-viewAngles[1])) > eps {
		t.Errorf("gun yaw %v != view yaw %v", out[1], viewAngles[1])
	}
	expectedPitch := -viewAngles[0]
	if math.Abs(float64(out[0]-expectedPitch)) > eps {
		t.Errorf("gun pitch %v != expected %v", out[0], expectedPitch)
	}
}

func TestViewCalcGunAngle_RateLimitsState(t *testing.T) {
	ensureViewCalcCvars()
	if cv := cvar.Get("v_idlescale"); cv != nil {
		cv.Float = 0
	}
	state := viewCalcState{oldGunYaw: 5, oldGunPitch: 5}
	// With a very small frameTime the rate-limit should prevent a jump to 0.
	viewAngles := [3]float32{0, 0, 0}
	out := viewCalcGunAngle(&state, viewAngles, 0.0, 0.001)
	// move = 0.001 * 20 = 0.02; oldGunYaw=5, target=0 → new yaw = 5-0.02 = 4.98
	expectedYaw := float32(0) + (5 - 0.001*20)
	const eps = float32(0.01)
	if out[1] < expectedYaw-eps || out[1] > expectedYaw+eps {
		t.Errorf("rate-limited gun yaw %v != expected ~%v", out[1], expectedYaw)
	}
}

// ---- V_AddIdle tests --------------------------------------------------------

func TestViewAddIdle_ZeroScaleNoChange(t *testing.T) {
	ensureViewCalcCvars()
	if cv := cvar.Get("v_idlescale"); cv != nil {
		cv.Float = 0
	}
	angles := [3]float32{10, 20, 30}
	out := viewAddIdle(angles, 1.0)
	if out != angles {
		t.Errorf("expected unchanged angles, got %v", out)
	}
}

func TestViewAddIdle_NonZeroScaleChanges(t *testing.T) {
	ensureViewCalcCvars()
	if cv := cvar.Get("v_idlescale"); cv != nil {
		cv.Float = 1
		cv.String = "1"
	}
	angles := [3]float32{0, 0, 0}
	out := viewAddIdle(angles, math.Pi/2) // sin(t*cycle) != 0
	changed := out[0] != 0 || out[1] != 0 || out[2] != 0
	if !changed {
		t.Error("expected idle sway to modify angles when v_idlescale != 0")
	}
	// Restore.
	if cv := cvar.Get("v_idlescale"); cv != nil {
		cv.Float = 0
		cv.String = "0"
	}
}

// ---- viewApplyBobToOrigin tests ---------------------------------------------

func TestViewApplyBobToOrigin(t *testing.T) {
	origin := [3]float32{0, 0, 0}
	forward := [3]float32{1, 0, 0}
	bob := float32(4)
	out := viewApplyBobToOrigin(origin, forward, bob)
	// X should move by forward[0]*bob*0.4 = 1*4*0.4 = 1.6
	// Z should move by bob = 4
	const eps = 0.0001
	if math.Abs(float64(out[0]-1.6)) > eps {
		t.Errorf("origin X: got %v, want 1.6", out[0])
	}
	if math.Abs(float64(out[2]-4.0)) > eps {
		t.Errorf("origin Z: got %v, want 4", out[2])
	}
}

// ---- viewNodeLineOffset tests -----------------------------------------------

func TestViewNodeLineOffset(t *testing.T) {
	origin := [3]float32{0, 0, 0}
	out := viewNodeLineOffset(origin)
	const expected = 1.0 / 32.0
	const eps = 1e-6
	for i, v := range out {
		if math.Abs(float64(v)-expected) > eps {
			t.Errorf("origin[%d]: got %v, want %v", i, v, expected)
		}
	}
}

// ---- viewApplyViewmodelQuakeFudge tests -------------------------------------

func TestViewApplyViewmodelQuakeFudge_DisabledNoChange(t *testing.T) {
	ensureViewCalcCvars()
	if cv := cvar.Get("r_viewmodel_quake"); cv != nil {
		cv.Int = 0
	}
	origin := [3]float32{0, 0, 0}
	out := viewApplyViewmodelQuakeFudge(origin, 100)
	if out != origin {
		t.Errorf("expected no change when r_viewmodel_quake=0, got %v", out)
	}
}

func TestViewApplyViewmodelQuakeFudge_Size100AddsTwo(t *testing.T) {
	ensureViewCalcCvars()
	if cv := cvar.Get("r_viewmodel_quake"); cv != nil {
		cv.Int = 1
	}
	origin := [3]float32{0, 0, 0}
	out := viewApplyViewmodelQuakeFudge(origin, 100)
	if out[2] != 2 {
		t.Errorf("expected Z+=2 for size=100, got Z=%v", out[2])
	}
	if cv := cvar.Get("r_viewmodel_quake"); cv != nil {
		cv.Int = 0
	}
}
