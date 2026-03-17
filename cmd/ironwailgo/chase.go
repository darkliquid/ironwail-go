package main

import "math"

const chaseCrosshairTraceDistance = 4096.0

type chaseTraceFunc func(start, end [3]float32) [3]float32

func chaseUpdate(origin, angles [3]float32, chaseBack, chaseUp, chaseRight float32, traceFn chaseTraceFunc) ([3]float32, [3]float32) {
	forward, right, _ := runtimeAngleVectors(angles)

	ideal := origin
	for i := range ideal {
		ideal[i] = origin[i] - forward[i]*chaseBack + right[i]*chaseRight
	}
	// Match Ironwail chase.c: chase_up is world-up offset, not camera-up.
	ideal[2] += chaseUp

	if traceFn != nil {
		ideal = traceFn(origin, ideal)
	}

	crosshair := [3]float32{
		origin[0] + forward[0]*chaseCrosshairTraceDistance,
		origin[1] + forward[1]*chaseCrosshairTraceDistance,
		origin[2] + forward[2]*chaseCrosshairTraceDistance,
	}
	if traceFn != nil {
		crosshair = traceFn(origin, crosshair)
	}

	lookDir := [3]float32{
		crosshair[0] - ideal[0],
		crosshair[1] - ideal[1],
		crosshair[2] - ideal[2],
	}

	return ideal, vectorAngles(lookDir)
}

// vectorAngles mirrors Quake's VectorAngles behavior from mathlib.c.
func vectorAngles(forward [3]float32) [3]float32 {
	var yaw, pitch float32

	if forward[0] == 0 && forward[1] == 0 {
		yaw = 0
		if forward[2] > 0 {
			pitch = -90
		} else {
			pitch = 90
		}
	} else {
		yaw = float32(math.Atan2(float64(forward[1]), float64(forward[0])) * (180.0 / math.Pi))
		if yaw < 0 {
			yaw += 360
		}
		tmp := float32(math.Sqrt(float64(forward[0]*forward[0] + forward[1]*forward[1])))
		pitch = -float32(math.Atan2(float64(forward[2]), float64(tmp)) * (180.0 / math.Pi))
	}

	return [3]float32{pitch, yaw, 0}
}
