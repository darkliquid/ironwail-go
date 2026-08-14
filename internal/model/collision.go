package model

import (
	"github.com/darkliquid/ironwail-go/pkg/types"
)

func (m *Model) ModelType() int      { return int(m.Type) }
func (m *Model) NumHulls() int       { return len(m.Hulls) }
func (m *Model) Hull(index int) Hull { return m.Hulls[index] }
func (m *Model) CollisionClipNodes() []MClipNode {
	return m.ClipNodes
}
func (m *Model) CollisionPlanes() []MPlane { return m.Planes }
func (m *Model) IsClipBox() bool           { return m.ClipBox }
func (m *Model) CollisionClipMins() types.Vec3 {
	return m.ClipMins
}
func (m *Model) CollisionClipMaxs() types.Vec3 {
	return m.ClipMaxs
}
