// trace.go defines TraceResult for collision trace queries.
package types

// TraceResult contains the result of a collision trace (ray or hull trace).
type TraceResult struct {
	AllSolid    bool
	StartSolid  bool
	Fraction    float32
	EndPos      [3]float32
	PlaneNormal [3]float32
	PlaneDist   float32
	Entity      *Edict
	InOpen      bool
	InWater     bool
}
