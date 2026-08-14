// trace.go defines TraceResult for collision trace queries.
package types

import (
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

// TraceResult contains the result of a collision trace (ray or hull trace).
type TraceResult struct {
	AllSolid    bool
	StartSolid  bool
	Fraction    float32
	EndPos      qtypes.Vec3
	PlaneNormal qtypes.Vec3
	PlaneDist   float32
	Entity      *Edict
	InOpen      bool
	InWater     bool
}
