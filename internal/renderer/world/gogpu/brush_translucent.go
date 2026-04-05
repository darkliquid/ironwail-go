package gogpu

import worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"

type TranslucentFacePass uint8

const (
	TranslucentFacePassSkip TranslucentFacePass = iota
	TranslucentFacePassAlphaTest
	TranslucentFacePassTranslucent
)

type TranslucentFacePlan struct {
	Pass   TranslucentFacePass
	Alpha  float32
	Liquid bool
}

type TranslucentFaceDraw struct {
	Face       worldimpl.WorldFace
	Alpha      float32
	Center     [3]float32
	DistanceSq float32
}

type TranslucentLiquidBrushEntityDraw struct {
	Frame    int
	Vertices []worldimpl.WorldVertex
	Indices  []uint32
	Faces    []TranslucentFaceDraw
}

type TranslucentBrushEntityDraw struct {
	Frame            int
	Vertices         []worldimpl.WorldVertex
	Indices          []uint32
	AlphaTestFaces   []worldimpl.WorldFace
	AlphaTestCenters [][3]float32
	TranslucentFaces []TranslucentFaceDraw
	LiquidFaces      []TranslucentFaceDraw
}

func BuildTranslucentLiquidBrushEntityDraw(entity BrushEntityParams, geom *worldimpl.WorldGeometry, planFace func(worldimpl.WorldFace, float32) (float32, bool), distanceSq func([3]float32) float32) *TranslucentLiquidBrushEntityDraw {
	if planFace == nil || distanceSq == nil {
		return nil
	}
	alpha, ok := translucentLiquidBrushEntityAlpha(entity)
	if !ok {
		return nil
	}
	vertices, rotation := prepareTranslucentBrushEntityGeometry(entity, geom)
	if len(vertices) == 0 {
		return nil
	}
	faces := make([]TranslucentFaceDraw, 0, len(geom.Faces))
	indices := make([]uint32, 0, len(geom.Indices))
	for _, face := range geom.Faces {
		faceAlpha, ok := planFace(face, alpha)
		if !ok {
			continue
		}
		drawFace, nextIndices, ok := appendBrushFaceIndices(face, geom.Indices, indices)
		if !ok {
			continue
		}
		indices = nextIndices
		center := worldimpl.TransformModelSpacePoint(face.Center, entity.Origin, rotation, entity.Scale)
		faces = append(faces, TranslucentFaceDraw{
			Face:       drawFace,
			Alpha:      faceAlpha,
			Center:     center,
			DistanceSq: distanceSq(center),
		})
	}
	if len(faces) == 0 || len(indices) == 0 {
		return nil
	}
	return &TranslucentLiquidBrushEntityDraw{
		Frame:    entity.Frame,
		Vertices: vertices,
		Indices:  indices,
		Faces:    faces,
	}
}

func BuildTranslucentBrushEntityDraw(entity BrushEntityParams, geom *worldimpl.WorldGeometry, planFace func(worldimpl.WorldFace, float32) (TranslucentFacePlan, bool), distanceSq func([3]float32) float32) *TranslucentBrushEntityDraw {
	if planFace == nil || distanceSq == nil {
		return nil
	}
	alpha, ok := translucentBrushEntityAlpha(entity)
	if !ok {
		return nil
	}
	vertices, rotation := prepareTranslucentBrushEntityGeometry(entity, geom)
	if len(vertices) == 0 {
		return nil
	}
	alphaTestFaces := make([]worldimpl.WorldFace, 0, len(geom.Faces))
	alphaTestCenters := make([][3]float32, 0, len(geom.Faces))
	translucentFaces := make([]TranslucentFaceDraw, 0, len(geom.Faces))
	liquidFaces := make([]TranslucentFaceDraw, 0, len(geom.Faces))
	indices := make([]uint32, 0, len(geom.Indices))
	for _, face := range geom.Faces {
		plan, ok := planFace(face, alpha)
		if !ok {
			continue
		}
		if plan.Pass == TranslucentFacePassSkip {
			continue
		}
		drawFace, nextIndices, ok := appendBrushFaceIndices(face, geom.Indices, indices)
		if !ok {
			continue
		}
		indices = nextIndices
		center := worldimpl.TransformModelSpacePoint(face.Center, entity.Origin, rotation, entity.Scale)
		switch plan.Pass {
		case TranslucentFacePassAlphaTest:
			alphaTestFaces = append(alphaTestFaces, drawFace)
			alphaTestCenters = append(alphaTestCenters, center)
		case TranslucentFacePassTranslucent:
			draw := TranslucentFaceDraw{
				Face:       drawFace,
				Alpha:      plan.Alpha,
				Center:     center,
				DistanceSq: distanceSq(center),
			}
			if plan.Liquid {
				liquidFaces = append(liquidFaces, draw)
				continue
			}
			translucentFaces = append(translucentFaces, draw)
		}
	}
	if len(alphaTestFaces) == 0 && len(translucentFaces) == 0 && len(liquidFaces) == 0 {
		return nil
	}
	return &TranslucentBrushEntityDraw{
		Frame:            entity.Frame,
		Vertices:         vertices,
		Indices:          indices,
		AlphaTestFaces:   alphaTestFaces,
		AlphaTestCenters: alphaTestCenters,
		TranslucentFaces: translucentFaces,
		LiquidFaces:      liquidFaces,
	}
}

func translucentLiquidBrushEntityAlpha(entity BrushEntityParams) (float32, bool) {
	alpha := clamp01(entity.Alpha)
	return alpha, isFullyOpaqueAlpha(alpha)
}

func translucentBrushEntityAlpha(entity BrushEntityParams) (float32, bool) {
	alpha := clamp01(entity.Alpha)
	return alpha, alpha > 0 && alpha < 1
}

func prepareTranslucentBrushEntityGeometry(entity BrushEntityParams, geom *worldimpl.WorldGeometry) ([]worldimpl.WorldVertex, [16]float32) {
	if geom == nil || len(geom.Vertices) == 0 || len(geom.Indices) == 0 || len(geom.Faces) == 0 {
		return nil, [16]float32{}
	}
	rotation := worldimpl.BuildBrushRotationMatrix(entity.Angles)
	vertices := make([]worldimpl.WorldVertex, len(geom.Vertices))
	for i, vertex := range geom.Vertices {
		vertices[i] = vertex
		vertices[i].Position = worldimpl.TransformModelSpacePoint(vertex.Position, entity.Origin, rotation, entity.Scale)
	}
	return vertices, rotation
}

func appendBrushFaceIndices(face worldimpl.WorldFace, geomIndices, indices []uint32) (worldimpl.WorldFace, []uint32, bool) {
	first := int(face.FirstIndex)
	last := first + int(face.NumIndices)
	if first < 0 {
		first = 0
	}
	if last > len(geomIndices) {
		last = len(geomIndices)
	}
	if first >= last {
		return worldimpl.WorldFace{}, indices, false
	}
	drawFace := face
	drawFace.FirstIndex = uint32(len(indices))
	drawFace.NumIndices = uint32(last - first)
	indices = append(indices, geomIndices[first:last]...)
	return drawFace, indices, true
}

func isFullyOpaqueAlpha(alpha float32) bool {
	return alpha >= 0.999
}
