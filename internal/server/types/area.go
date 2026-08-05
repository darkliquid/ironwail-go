// area.go defines AreaNode for spatial partitioning.
package types

type AreaNode struct {
	Axis          int
	Dist          float32
	Children      [2]*AreaNode
	TriggerEdicts Edict
	SolidEdicts   Edict
}
