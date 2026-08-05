package warpscale

import (
	"math"
	"testing"
)

func TestWaterwarpFOVScale(t *testing.T) {
	s0 := WaterwarpFOVScale(0)
	if math.Abs(float64(s0-0.97)) > 0.0001 {
		t.Errorf("WaterwarpFOVScale(0) = %f, want 0.97", s0)
	}

	for timeVal := float32(0); timeVal < 10; timeVal += 0.5 {
		scale := WaterwarpFOVScale(timeVal)
		if scale < 0.94 || scale > 1.00 {
			t.Errorf("WaterwarpFOVScale(%f) = %f out of bounds [0.94, 1.00]", timeVal, scale)
		}
	}
}

func TestApplyWaterwarpFOV(t *testing.T) {
	baseFOV := float32(90.0)
	fov := ApplyWaterwarpFOV(baseFOV, 0)
	if fov <= 80 || fov >= 95 {
		t.Errorf("ApplyWaterwarpFOV(90, 0) = %f, expected close to 90", fov)
	}
}
