package game

import cameralib "github.com/darkliquid/ironwail-go/internal/game/camera"


type chaseTraceFunc = cameralib.ChaseTraceFunc

func (g *Game) chaseUpdate(origin, angles [3]float32, chaseBack, chaseUp, chaseRight float32, traceFn chaseTraceFunc) ([3]float32, [3]float32) {
	return cameralib.ChaseUpdate(origin, angles, chaseBack, chaseUp, chaseRight, traceFn)
}

// vectorAngles mirrors Quake's VectorAngles behavior from mathlib.c.
func (g *Game) vectorAngles(forward [3]float32) [3]float32 {
	return cameralib.VectorAngles(forward)
}
