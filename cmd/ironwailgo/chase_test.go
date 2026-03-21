package main

import (
	"math"
	"testing"
)

func nearlyEqual(a, b, eps float32) bool {
	return float32(math.Abs(float64(a-b))) <= eps
}

func TestVectorAnglesVerticalDirections(t *testing.T) {
	angles := vectorAngles([3]float32{0, 0, 1})
	if !nearlyEqual(angles[0], -90, 0.001) || !nearlyEqual(angles[1], 0, 0.001) || !nearlyEqual(angles[2], 0, 0.001) {
		t.Fatalf("vectorAngles(up) = %v, want {-90 0 0}", angles)
	}

	angles = vectorAngles([3]float32{0, 0, -1})
	if !nearlyEqual(angles[0], 90, 0.001) || !nearlyEqual(angles[1], 0, 0.001) || !nearlyEqual(angles[2], 0, 0.001) {
		t.Fatalf("vectorAngles(down) = %v, want {90 0 0}", angles)
	}
}

func TestVectorAnglesNormalizesYaw(t *testing.T) {
	angles := vectorAngles([3]float32{0, -1, 0})
	if !nearlyEqual(angles[1], 270, 0.001) {
		t.Fatalf("vectorAngles yaw = %v, want 270", angles[1])
	}
}

func TestChaseUpdateBasicPosition(t *testing.T) {
	origin := [3]float32{0, 0, 0}
	angles := [3]float32{0, 0, 0}

	cameraOrigin, cameraAngles := chaseUpdate(origin, angles, 100, 16, 0, nil)

	if !nearlyEqual(cameraOrigin[0], -100, 0.001) || !nearlyEqual(cameraOrigin[1], 0, 0.001) || !nearlyEqual(cameraOrigin[2], 16, 0.001) {
		t.Fatalf("chaseUpdate origin = %v, want {-100 0 16}", cameraOrigin)
	}
	if !nearlyEqual(cameraAngles[1], 0, 0.001) {
		t.Fatalf("chaseUpdate yaw = %v, want 0", cameraAngles[1])
	}
	if cameraAngles[0] <= 0 {
		t.Fatalf("chaseUpdate pitch = %v, want slight down-looking positive pitch", cameraAngles[0])
	}
}

func TestChaseUpdateClipsToTraceEndPosition(t *testing.T) {
	origin := [3]float32{0, 0, 0}
	angles := [3]float32{0, 0, 0}

	traceFn := func(start, end [3]float32) [3]float32 {
		if nearlyEqual(end[0], -100, 0.001) {
			return [3]float32{-40, 0, 4}
		}
		return end
	}

	cameraOrigin, _ := chaseUpdate(origin, angles, 100, 16, 0, traceFn)
	if !nearlyEqual(cameraOrigin[0], -40, 0.001) || !nearlyEqual(cameraOrigin[1], 0, 0.001) || !nearlyEqual(cameraOrigin[2], 4, 0.001) {
		t.Fatalf("chaseUpdate clipped origin = %v, want {-40 0 4}", cameraOrigin)
	}
}

func TestChaseUpdateUsesCanonicalCrosshairTraceDistance(t *testing.T) {
	origin := [3]float32{0, 0, 0}
	angles := [3]float32{0, 0, 0}
	traceCalls := 0

	traceFn := func(start, end [3]float32) [3]float32 {
		traceCalls++
		if traceCalls == 2 {
			if !nearlyEqual(end[0], chaseCrosshairTraceDistance, 0.001) || !nearlyEqual(end[1], 0, 0.001) || !nearlyEqual(end[2], 0, 0.001) {
				t.Fatalf("crosshair trace end = %v, want {%v 0 0}", end, chaseCrosshairTraceDistance)
			}
		}
		return end
	}

	chaseUpdate(origin, angles, 100, 16, 0, traceFn)

	if traceCalls != 2 {
		t.Fatalf("trace call count = %d, want 2", traceCalls)
	}
}

func TestChaseUpdatePreservesYawForVerticalLookDir(t *testing.T) {
	origin := [3]float32{0, 0, 0}
	angles := [3]float32{-90, 123, 0}

	_, cameraAngles := chaseUpdate(origin, angles, 0, 16, 0, nil)

	if !nearlyEqual(cameraAngles[0], -90, 0.001) {
		t.Fatalf("chaseUpdate pitch = %v, want -90", cameraAngles[0])
	}
	if !nearlyEqual(cameraAngles[1], 123, 0.001) {
		t.Fatalf("chaseUpdate yaw = %v, want preserved yaw 123", cameraAngles[1])
	}
}
