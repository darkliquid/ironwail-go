package client

import (
	"math"
	"testing"
)

func TestStartPitchDriftUsesCenterSpeed(t *testing.T) {
	c := NewClient()
	c.Time = 10
	c.LastStop = 0
	c.NoDrift = true
	c.PitchVel = 0
	c.CenterSpeed = 720

	c.StartPitchDrift()

	if math.Abs(float64(c.PitchVel-720)) > 0.0001 {
		t.Fatalf("PitchVel = %.3f, want 720", c.PitchVel)
	}
	if c.NoDrift {
		t.Fatal("NoDrift = true, want false after StartPitchDrift")
	}
}

func TestDriftPitchUsesCenterMoveThreshold(t *testing.T) {
	c := NewClient()
	c.Time = 1
	c.LastStop = 0
	c.OnGround = true
	c.NoDrift = true
	c.LookSpring = true
	c.ForwardSpeed = 200
	c.CenterMove = 0.4
	c.PendingCmd.Forward = 200

	c.DriftPitch(0.2, c.PendingCmd.Forward)
	if !c.NoDrift {
		t.Fatal("drift started before center move threshold")
	}

	c.DriftPitch(0.21, c.PendingCmd.Forward)
	if c.NoDrift {
		t.Fatal("drift did not start after center move threshold")
	}
}

func TestDriftPitchUsesCenterSpeedAcceleration(t *testing.T) {
	c := NewClient()
	c.OnGround = true
	c.NoDrift = false
	c.PitchVel = 100
	c.CenterSpeed = 300
	c.IdealPitch = 30
	c.ViewAngles[0] = 0

	c.DriftPitch(0.1, 0)

	if math.Abs(float64(c.PitchVel-130)) > 0.0001 {
		t.Fatalf("PitchVel = %.3f, want 130", c.PitchVel)
	}
	if math.Abs(float64(c.ViewAngles[0]-10)) > 0.0001 {
		t.Fatalf("ViewAngles[0] = %.3f, want 10", c.ViewAngles[0])
	}
}
