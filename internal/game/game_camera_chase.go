package game

import (
	cameralib "github.com/darkliquid/ironwail-go/internal/game/camera"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

type chaseTraceFunc = cameralib.ChaseTraceFunc

func (g *Game) chaseUpdate(origin, angles types.Vec3, chaseBack, chaseUp, chaseRight float32, traceFn chaseTraceFunc) (types.Vec3, types.Vec3) {
	return cameralib.ChaseUpdate(origin, angles, chaseBack, chaseUp, chaseRight, traceFn)
}
