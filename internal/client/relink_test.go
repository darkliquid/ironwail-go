package client

import (
	"math"
	"testing"
)

func TestRelinkEntities_DemoViewAngleInterpolation(t *testing.T) {
	c := &Client{}
	// Set up time so LerpPoint returns 0.5 (midway between frames).
	// MTime[0] - MTime[1] = 0.1 (frame interval), Time at midpoint.
	c.MTime = [2]float64{1.0, 0.9}
	c.Time = 0.95

	// Enable demo playback interpolation.
	c.DemoPlayback = true

	// Set double-buffered view angles: old=[0,0,0], new=[90,180,45]
	c.MViewAngles[1] = [3]float32{0, 0, 0}   // old frame
	c.MViewAngles[0] = [3]float32{90, 180, 45} // new frame

	c.RelinkEntities()

	// At frac=0.5, expect midpoint: [45, 90, 22.5]
	want := [3]float32{45, 90, 22.5}
	for j := 0; j < 3; j++ {
		if math.Abs(float64(c.ViewAngles[j]-want[j])) > 0.01 {
			t.Errorf("ViewAngles[%d] = %v, want %v", j, c.ViewAngles[j], want[j])
		}
	}
}

func TestRelinkEntities_DemoViewAngleWraparound(t *testing.T) {
	c := &Client{}
	c.MTime = [2]float64{1.0, 0.9}
	c.Time = 0.95
	c.DemoPlayback = true

	// Test wraparound: old=350°, new=10° → delta should be +20° (not -340°)
	c.MViewAngles[1] = [3]float32{0, 350, 0}
	c.MViewAngles[0] = [3]float32{0, 10, 0}

	c.RelinkEntities()

	// At frac=0.5: 350 + 0.5*20 = 360 → should be 360 (or wrapped)
	// C doesn't wrap the result, so 360 is fine.
	want := float32(360)
	if math.Abs(float64(c.ViewAngles[1]-want)) > 0.01 {
		t.Errorf("ViewAngles[1] = %v, want %v (wraparound interpolation)", c.ViewAngles[1], want)
	}
}

func TestRelinkEntities_NoDemoNoViewAngleChange(t *testing.T) {
	c := &Client{}
	c.MTime = [2]float64{1.0, 0.9}
	c.Time = 0.95
	c.DemoPlayback = false

	// Set view angles manually.
	c.ViewAngles = [3]float32{10, 20, 30}
	c.MViewAngles[1] = [3]float32{0, 0, 0}
	c.MViewAngles[0] = [3]float32{90, 180, 45}

	c.RelinkEntities()

	// Without demo playback, view angles should not be modified.
	want := [3]float32{10, 20, 30}
	if c.ViewAngles != want {
		t.Errorf("ViewAngles = %v, want %v (should be unchanged without demo)", c.ViewAngles, want)
	}
}
