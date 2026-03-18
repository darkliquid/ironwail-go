package client

import (
	"math"
	"testing"

	"github.com/ironwail/ironwail-go/internal/model"
	inet "github.com/ironwail/ironwail-go/internal/net"
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
	c.MViewAngles[1] = [3]float32{0, 0, 0}     // old frame
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

func TestRelinkEntities_TrailEvents(t *testing.T) {
	c := &Client{}
	c.MTime = [2]float64{1.0, 0.9}
	c.Time = 1.0

	c.Entities = map[int]inet.EntityState{
		1: {
			ModelIndex:  1,
			MsgTime:     1.0,
			MsgOrigins:  [2][3]float32{{100, 200, 300}, {100, 200, 300}},
			TrailOrigin: [3]float32{90, 190, 290},
			ForceLink:   false,
		},
		2: {
			ModelIndex:  2,
			MsgTime:     1.0,
			MsgOrigins:  [2][3]float32{{50, 60, 70}, {50, 60, 70}},
			TrailOrigin: [3]float32{40, 50, 60},
			ForceLink:   false,
		},
		3: {
			ModelIndex:  3,
			MsgTime:     1.0,
			MsgOrigins:  [2][3]float32{{10, 20, 30}, {10, 20, 30}},
			TrailOrigin: [3]float32{5, 15, 25},
			ForceLink:   false,
		},
	}
	c.ModelPrecache = []string{"", "progs/missile.mdl", "progs/grenade.mdl", "progs/player.mdl"}
	c.ModelFlagsFunc = func(name string) int {
		switch name {
		case "progs/missile.mdl":
			return model.EFRocket
		case "progs/grenade.mdl":
			return model.EFGrenade
		default:
			return 0
		}
	}

	c.RelinkEntities()

	// Should have 2 trail events (rocket + grenade), none for player.
	if len(c.TrailEvents) != 2 {
		t.Fatalf("TrailEvents count = %d, want 2", len(c.TrailEvents))
	}

	// Find rocket and grenade trails (map iteration order is non-deterministic).
	var gotRocket, gotGrenade bool
	for _, te := range c.TrailEvents {
		switch te.Type {
		case 0: // rocket
			gotRocket = true
			if te.Start != [3]float32{90, 190, 290} {
				t.Errorf("rocket trail start = %v, want [90 190 290]", te.Start)
			}
		case 1: // grenade
			gotGrenade = true
			if te.Start != [3]float32{40, 50, 60} {
				t.Errorf("grenade trail start = %v, want [40 50 60]", te.Start)
			}
		default:
			t.Errorf("unexpected trail type %d", te.Type)
		}
	}
	if !gotRocket {
		t.Error("missing rocket trail event")
	}
	if !gotGrenade {
		t.Error("missing grenade trail event")
	}

	// After relink, TrailOrigin should be updated to current origin.
	ent1 := c.Entities[1]
	if ent1.TrailOrigin != ent1.Origin {
		t.Errorf("entity 1 TrailOrigin = %v, want %v", ent1.TrailOrigin, ent1.Origin)
	}
}
