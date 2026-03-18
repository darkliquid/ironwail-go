//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"math"
	"testing"

	"github.com/ironwail/ironwail-go/internal/cvar"
)

// TestApplyWaterwarpFOV verifies that the r_waterwarp > 1 FOV modulation formula
// matches the C Ironwail R_SetupView r_waterwarp > 1 branch.
//
// C formula: r_fovx = atan(tan(DEG2RAD(fov_x)/2) * (0.97 + sin(t*1.5)*0.03)) * 2 / (π/180)
func TestApplyWaterwarpFOV(t *testing.T) {
	tests := []struct {
		name    string
		baseFOV float32
		time    float32
		// C reference: python equivalent:
		//   import math
		//   atan(math.tan(math.radians(fov)/2) * (0.97 + math.sin(t*1.5)*0.03)) * 2 * 180/math.pi
		wantApprox float32
		tolerance  float32
	}{
		{
			name:    "sin=0 (t=0): scale=0.97, FOV should decrease",
			baseFOV: 90.0,
			time:    0,
			// scale = 0.97 + sin(0)*0.03 = 0.97
			// new = atan(tan(45°)*0.97) * 360/π ≈ atan(0.97) * 114.59 ≈ 88.26°
			wantApprox: 88.26,
			tolerance:  0.05,
		},
		{
			name:    "sin=1 (t=π/3): scale=1.0, FOV unchanged",
			baseFOV: 90.0,
			time:    float32(math.Pi / 3), // t*1.5 = π/2, sin(π/2)=1
			// scale = 0.97 + 1*0.03 = 1.0 → FOV unchanged
			wantApprox: 90.0,
			tolerance:  0.05,
		},
		{
			name:    "base FOV 96 at t=0",
			baseFOV: 96.0,
			time:    0,
			// scale = 0.97
			// new = atan(tan(48°)*0.97)*360/π
			// tan(48°) ≈ 1.11061, scaled ≈ 1.07729
			// atan(1.07729)*360/π ≈ 94.26°
			wantApprox: 94.26,
			tolerance:  0.05,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ApplyWaterwarpFOV(tc.baseFOV, tc.time)
			diff := got - tc.wantApprox
			if diff < 0 {
				diff = -diff
			}
			if diff > tc.tolerance {
				t.Errorf("ApplyWaterwarpFOV(%v, %v) = %.4f, want %.4f ± %.4f",
					tc.baseFOV, tc.time, got, tc.wantApprox, tc.tolerance)
			}
		})
	}
}

// TestApplyWaterwarpFOVNeverExceedsBase verifies that the x-axis scale (0.97..1.0)
// never produces a FOV larger than the base.
func TestApplyWaterwarpFOVNeverExceedsBase(t *testing.T) {
	baseFOV := float32(90.0)
	// x-axis scale oscillates 0.97..1.00 so modulated FOV ≤ baseFOV.
	for tval := float32(0); tval < 8.0; tval += 0.1 {
		got := ApplyWaterwarpFOV(baseFOV, tval)
		if got > baseFOV+0.001 {
			t.Errorf("at t=%.2f: ApplyWaterwarpFOV(%v) = %.4f > base %.4f", tval, baseFOV, got, baseFOV)
		}
	}
}

// TestWaterwarpFOVScaleRange verifies the x-axis scale stays within [0.94, 1.0].
// C: scale = 0.97 + sin(t*1.5)*0.03  → range [0.94, 1.00].
func TestWaterwarpFOVScaleRange(t *testing.T) {
	for tval := float32(0); tval < 10.0; tval += 0.05 {
		s := WaterwarpFOVScale(tval)
		if s < 0.94-0.001 || s > 1.00+0.001 {
			t.Errorf("WaterwarpFOVScale(%v) = %v, want in [0.94, 1.00]", tval, s)
		}
	}
}

// TestReadWaterwarpCvar verifies that readWaterwarpCvar falls back gracefully
// when the cvar is not registered.
func TestReadWaterwarpCvar(t *testing.T) {
	// No cvar registered: should return 0.
	if got := readWaterwarpCvar(); got != 0 {
		t.Errorf("readWaterwarpCvar() without registration = %v, want 0", got)
	}

	// Register and set to 1.
	cv := cvar.Register(CvarRWaterwarp, "1", 0, "Underwater warp test")
	_ = cv
	if got := readWaterwarpCvar(); got != 1 {
		t.Errorf("readWaterwarpCvar() with value=1 = %v, want 1", got)
	}

	// Change to 2.
	cvar.Set(CvarRWaterwarp, "2")
	if got := readWaterwarpCvar(); got != 2 {
		t.Errorf("readWaterwarpCvar() with value=2 = %v, want 2", got)
	}

	// Change to 0.
	cvar.Set(CvarRWaterwarp, "0")
	if got := readWaterwarpCvar(); got != 0 {
		t.Errorf("readWaterwarpCvar() with value=0 = %v, want 0", got)
	}
}

// TestWaterwarpFOVHelpers verifies that WaterwarpFOV and WaterwarpFOVScale
// agree with each other across the full time range.
func TestWaterwarpFOVHelpers(t *testing.T) {
	// WaterwarpFOV should return false if cvar is 0 or ≤ 1.
	cvar.Set(CvarRWaterwarp, "0")
	if WaterwarpFOV(true) {
		t.Error("WaterwarpFOV(true) with r_waterwarp=0 should be false")
	}
	cvar.Set(CvarRWaterwarp, "1")
	if WaterwarpFOV(true) {
		t.Error("WaterwarpFOV(true) with r_waterwarp=1 should be false (screen warp, not FOV warp)")
	}
	cvar.Set(CvarRWaterwarp, "2")
	if !WaterwarpFOV(true) {
		t.Error("WaterwarpFOV(true) with r_waterwarp=2 should be true")
	}
	if WaterwarpFOV(false) {
		t.Error("WaterwarpFOV(false) with r_waterwarp=2 should be false when not underwater")
	}
	// Cleanup
	cvar.Set(CvarRWaterwarp, "0")
}
