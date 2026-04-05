package gogpu

import worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"

type BrushEntityParams struct {
	Alpha  float32
	Frame  int
	Origin [3]float32
	Angles [3]float32
	Scale  float32
}

type OpaqueBrushEntityDraw struct {
	HasLitWater bool
	Alpha       float32
	Frame       int
	Vertices    []worldimpl.WorldVertex
	Indices     []uint32
	Faces       []worldimpl.WorldFace
	Centers     [][3]float32
}

type BrushEntityFaceClass uint8

const (
	BrushEntityFaceClassSkip BrushEntityFaceClass = iota
	BrushEntityFaceClassOpaque
	BrushEntityFaceClassAlphaTest
)

type ClassifiedBrushEntityDraw struct {
	Alpha            float32
	Frame            int
	Vertices         []worldimpl.WorldVertex
	OpaqueIndices    []uint32
	OpaqueFaces      []worldimpl.WorldFace
	OpaqueCenters    [][3]float32
	AlphaTestIndices []uint32
	AlphaTestFaces   []worldimpl.WorldFace
	AlphaTestCenters [][3]float32
}

func FillBrushEntityDraw(dst *OpaqueBrushEntityDraw, entity BrushEntityParams, geom *worldimpl.WorldGeometry, includeFace func(worldimpl.WorldFace, float32) bool) bool {
	if dst == nil || geom == nil || len(geom.Vertices) == 0 || len(geom.Indices) == 0 || len(geom.Faces) == 0 {
		return false
	}
	alpha := clamp01(entity.Alpha)
	if alpha <= 0 {
		return false
	}
	rotation := worldimpl.BuildBrushRotationMatrix(entity.Angles)
	dst.HasLitWater = geom.HasLitWater
	dst.Alpha = alpha
	dst.Frame = entity.Frame
	dst.Vertices = resizeWorldVertices(dst.Vertices, len(geom.Vertices))
	for i, vertex := range geom.Vertices {
		dst.Vertices[i] = vertex
		dst.Vertices[i].Position = worldimpl.TransformModelSpacePoint(vertex.Position, entity.Origin, rotation, entity.Scale)
	}
	dst.Faces = dst.Faces[:0]
	dst.Centers = dst.Centers[:0]
	dst.Indices = dst.Indices[:0]
	for _, face := range geom.Faces {
		if !includeFace(face, alpha) {
			continue
		}
		first := int(face.FirstIndex)
		last := first + int(face.NumIndices)
		if first < 0 {
			first = 0
		}
		if last > len(geom.Indices) {
			last = len(geom.Indices)
		}
		if first >= last {
			continue
		}
		drawFace := face
		drawFace.FirstIndex = uint32(len(dst.Indices))
		drawFace.NumIndices = uint32(last - first)
		dst.Faces = append(dst.Faces, drawFace)
		dst.Centers = append(dst.Centers, worldimpl.TransformModelSpacePoint(face.Center, entity.Origin, rotation, entity.Scale))
		dst.Indices = append(dst.Indices, geom.Indices[first:last]...)
	}
	return len(dst.Faces) > 0 && len(dst.Indices) > 0
}

func BuildBrushEntityDraw(entity BrushEntityParams, geom *worldimpl.WorldGeometry, includeFace func(worldimpl.WorldFace, float32) bool) *OpaqueBrushEntityDraw {
	draw := &OpaqueBrushEntityDraw{}
	if !FillBrushEntityDraw(draw, entity, geom, includeFace) {
		return nil
	}
	return draw
}

func FillClassifiedBrushEntityDraw(dst *ClassifiedBrushEntityDraw, entity BrushEntityParams, geom *worldimpl.WorldGeometry, classifyFace func(worldimpl.WorldFace, float32) BrushEntityFaceClass) bool {
	if dst == nil || geom == nil || len(geom.Vertices) == 0 || len(geom.Indices) == 0 || len(geom.Faces) == 0 || classifyFace == nil {
		return false
	}
	alpha := clamp01(entity.Alpha)
	if alpha <= 0 {
		return false
	}
	rotation := worldimpl.BuildBrushRotationMatrix(entity.Angles)
	dst.Alpha = alpha
	dst.Frame = entity.Frame
	dst.Vertices = resizeWorldVertices(dst.Vertices, len(geom.Vertices))
	for i, vertex := range geom.Vertices {
		dst.Vertices[i] = vertex
		dst.Vertices[i].Position = worldimpl.TransformModelSpacePoint(vertex.Position, entity.Origin, rotation, entity.Scale)
	}
	dst.OpaqueFaces = dst.OpaqueFaces[:0]
	dst.OpaqueCenters = dst.OpaqueCenters[:0]
	dst.OpaqueIndices = dst.OpaqueIndices[:0]
	dst.AlphaTestFaces = dst.AlphaTestFaces[:0]
	dst.AlphaTestCenters = dst.AlphaTestCenters[:0]
	dst.AlphaTestIndices = dst.AlphaTestIndices[:0]
	for _, face := range geom.Faces {
		first := int(face.FirstIndex)
		last := first + int(face.NumIndices)
		if first < 0 {
			first = 0
		}
		if last > len(geom.Indices) {
			last = len(geom.Indices)
		}
		if first >= last {
			continue
		}
		center := worldimpl.TransformModelSpacePoint(face.Center, entity.Origin, rotation, entity.Scale)
		switch classifyFace(face, alpha) {
		case BrushEntityFaceClassOpaque:
			appendClassifiedBrushEntityFace(&dst.OpaqueFaces, &dst.OpaqueCenters, &dst.OpaqueIndices, face, center, geom.Indices[first:last])
		case BrushEntityFaceClassAlphaTest:
			appendClassifiedBrushEntityFace(&dst.AlphaTestFaces, &dst.AlphaTestCenters, &dst.AlphaTestIndices, face, center, geom.Indices[first:last])
		}
	}
	return len(dst.OpaqueFaces) > 0 || len(dst.AlphaTestFaces) > 0
}

func BuildClassifiedBrushEntityDraw(entity BrushEntityParams, geom *worldimpl.WorldGeometry, classifyFace func(worldimpl.WorldFace, float32) BrushEntityFaceClass) *ClassifiedBrushEntityDraw {
	draw := &ClassifiedBrushEntityDraw{}
	if !FillClassifiedBrushEntityDraw(draw, entity, geom, classifyFace) {
		return nil
	}
	return draw
}

func appendClassifiedBrushEntityFace(dstFaces *[]worldimpl.WorldFace, dstCenters *[][3]float32, dstIndices *[]uint32, face worldimpl.WorldFace, center [3]float32, indices []uint32) {
	drawFace := face
	drawFace.FirstIndex = uint32(len(*dstIndices))
	drawFace.NumIndices = uint32(len(indices))
	*dstFaces = append(*dstFaces, drawFace)
	*dstCenters = append(*dstCenters, center)
	*dstIndices = append(*dstIndices, indices...)
}

func resizeWorldVertices(vertices []worldimpl.WorldVertex, size int) []worldimpl.WorldVertex {
	if cap(vertices) < size {
		return make([]worldimpl.WorldVertex, size)
	}
	return vertices[:size]
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
