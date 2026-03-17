package model

func (m *Model) ModelType() int      { return int(m.Type) }
func (m *Model) NumHulls() int       { return len(m.Hulls) }
func (m *Model) Hull(index int) Hull { return m.Hulls[index] }
func (m *Model) CollisionClipNodes() []MClipNode {
	return m.ClipNodes
}
func (m *Model) CollisionPlanes() []MPlane { return m.Planes }
func (m *Model) IsClipBox() bool           { return m.ClipBox }
func (m *Model) CollisionClipMins() [3]float32 {
	return m.ClipMins
}
func (m *Model) CollisionClipMaxs() [3]float32 {
	return m.ClipMaxs
}
