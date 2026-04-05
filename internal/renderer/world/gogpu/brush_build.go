//go:build gogpu && !cgo
// +build gogpu,!cgo

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
	Alpha    float32
	Frame    int
	Vertices []worldimpl.WorldVertex
	Indices  []uint32
	Faces    []worldimpl.WorldFace
	Centers  [][3]float32
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

func BuildBrushEntityDraw(entity BrushEntityParams, geom *worldimpl.WorldGeometry, includeFace func(worldimpl.WorldFace, float32) bool) *OpaqueBrushEntityDraw {
	if geom == nil || len(geom.Vertices) == 0 || len(geom.Indices) == 0 || len(geom.Faces) == 0 {
		return nil
	}
	alpha := clamp01(entity.Alpha)
	if alpha <= 0 {
		return nil
	}
	rotation := worldimpl.BuildBrushRotationMatrix(entity.Angles)
	vertices := make([]worldimpl.WorldVertex, len(geom.Vertices))
	for i, vertex := range geom.Vertices {
		vertices[i] = vertex
		vertices[i].Position = worldimpl.TransformModelSpacePoint(vertex.Position, entity.Origin, rotation, entity.Scale)
	}
	faces := make([]worldimpl.WorldFace, 0, len(geom.Faces))
	centers := make([][3]float32, 0, len(geom.Faces))
	indices := make([]uint32, 0, len(geom.Indices))
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
		drawFace.FirstIndex = uint32(len(indices))
		drawFace.NumIndices = uint32(last - first)
		faces = append(faces, drawFace)
		centers = append(centers, worldimpl.TransformModelSpacePoint(face.Center, entity.Origin, rotation, entity.Scale))
		indices = append(indices, geom.Indices[first:last]...)
	}
	if len(faces) == 0 || len(indices) == 0 {
		return nil
	}
	return &OpaqueBrushEntityDraw{
		Alpha:    alpha,
		Frame:    entity.Frame,
		Vertices: vertices,
		Indices:  indices,
		Faces:    faces,
		Centers:  centers,
	}
}

func BuildClassifiedBrushEntityDraw(entity BrushEntityParams, geom *worldimpl.WorldGeometry, classifyFace func(worldimpl.WorldFace, float32) BrushEntityFaceClass) *ClassifiedBrushEntityDraw {
	if geom == nil || len(geom.Vertices) == 0 || len(geom.Indices) == 0 || len(geom.Faces) == 0 || classifyFace == nil {
		return nil
	}
	alpha := clamp01(entity.Alpha)
	if alpha <= 0 {
		return nil
	}
	rotation := worldimpl.BuildBrushRotationMatrix(entity.Angles)
	vertices := make([]worldimpl.WorldVertex, len(geom.Vertices))
	for i, vertex := range geom.Vertices {
		vertices[i] = vertex
		vertices[i].Position = worldimpl.TransformModelSpacePoint(vertex.Position, entity.Origin, rotation, entity.Scale)
	}
	draw := &ClassifiedBrushEntityDraw{
		Alpha:            alpha,
		Frame:            entity.Frame,
		Vertices:         vertices,
		OpaqueFaces:      make([]worldimpl.WorldFace, 0, len(geom.Faces)),
		OpaqueCenters:    make([][3]float32, 0, len(geom.Faces)),
		OpaqueIndices:    make([]uint32, 0, len(geom.Indices)),
		AlphaTestFaces:   make([]worldimpl.WorldFace, 0, len(geom.Faces)),
		AlphaTestCenters: make([][3]float32, 0, len(geom.Faces)),
		AlphaTestIndices: make([]uint32, 0, len(geom.Indices)),
	}
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
			appendClassifiedBrushEntityFace(&draw.OpaqueFaces, &draw.OpaqueCenters, &draw.OpaqueIndices, face, center, geom.Indices[first:last])
		case BrushEntityFaceClassAlphaTest:
			appendClassifiedBrushEntityFace(&draw.AlphaTestFaces, &draw.AlphaTestCenters, &draw.AlphaTestIndices, face, center, geom.Indices[first:last])
		}
	}
	if len(draw.OpaqueFaces) == 0 && len(draw.AlphaTestFaces) == 0 {
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

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
