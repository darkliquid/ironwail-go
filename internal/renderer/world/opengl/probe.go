//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package opengl

import (
	"fmt"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
)

// FaceProbeStats captures opt-in world extraction diagnostics for a bbox.
type FaceProbeStats struct {
	ModelIndex int
	BoundsMin  [3]float32
	BoundsMax  [3]float32

	RawInBBox       int
	ExtractedInBBox int

	PassSky         int
	PassOpaque      int
	PassAlphaTest   int
	PassTranslucent int

	Faces []FaceProbeEntry
}

// FaceProbeEntry captures one suspect face from raw BSP through extraction and pass routing.
type FaceProbeEntry struct {
	SourceFaceIndex int
	NumEdges        int32
	TexInfoIndex    int32
	TexInfoFlags    int32
	TextureIndex    int32
	TextureName     string
	DerivedFlags    int32
	Center          [3]float32
	Normal          [3]float32
	InBBox          bool
	Extracted       bool
	ExtractError    string
	VertexCount     int
	Pass            worldimpl.RenderPass
}

func RenderPassName(pass worldimpl.RenderPass) string {
	switch pass {
	case worldimpl.PassSky:
		return "sky"
	case worldimpl.PassOpaque:
		return "opaque"
	case worldimpl.PassAlphaTest:
		return "alpha-test"
	case worldimpl.PassTranslucent:
		return "translucent"
	default:
		return fmt.Sprintf("unknown(%d)", pass)
	}
}

// ProbeFacesInBBox inspects world/model faces in an opt-in diagnostic path.
func ProbeFacesInBBox(tree *bsp.Tree, modelIndex int, boundsMin, boundsMax [3]float32, liquidAlpha worldimpl.LiquidAlphaSettings) (*FaceProbeStats, error) {
	if tree == nil {
		return nil, fmt.Errorf("nil BSP tree")
	}
	if len(tree.Models) == 0 {
		return nil, fmt.Errorf("BSP has no models")
	}
	if modelIndex < 0 || modelIndex >= len(tree.Models) {
		return nil, fmt.Errorf("model index %d out of range", modelIndex)
	}

	meta := worldimpl.ParseTextureMeta(tree)
	modelDef := tree.Models[modelIndex]
	out := &FaceProbeStats{
		ModelIndex: modelIndex,
		BoundsMin:  boundsMin,
		BoundsMax:  boundsMax,
		Faces:      make([]FaceProbeEntry, 0, 64),
	}

	for localFace := 0; localFace < int(modelDef.NumFaces); localFace++ {
		faceIndex := int(modelDef.FirstFace) + localFace
		if faceIndex < 0 || faceIndex >= len(tree.Faces) {
			break
		}
		face := &tree.Faces[faceIndex]
		center, normal, err := faceCenterAndNormal(tree, face)
		if err != nil || !pointInBounds(center, boundsMin, boundsMax) {
			continue
		}

		entry := FaceProbeEntry{
			SourceFaceIndex: faceIndex,
			NumEdges:        face.NumEdges,
			TexInfoIndex:    face.Texinfo,
			Center:          center,
			Normal:          normal,
			InBBox:          true,
		}
		out.RawInBBox++

		if int(face.Texinfo) >= 0 && int(face.Texinfo) < len(tree.Texinfo) {
			texInfo := tree.Texinfo[face.Texinfo]
			entry.TexInfoFlags = texInfo.Flags
			entry.TextureIndex = texInfo.Miptex
			if int(texInfo.Miptex) >= 0 && int(texInfo.Miptex) < len(meta) {
				entry.TextureName = meta[texInfo.Miptex].Name
				entry.DerivedFlags = worldimpl.DeriveFaceFlags(meta[texInfo.Miptex].Type, texInfo.Flags)
			} else {
				entry.DerivedFlags = worldimpl.DeriveFaceFlags(model.TexTypeDefault, texInfo.Flags)
			}
		}

		vertexCount, extractErr := extractFaceVertexCount(tree, face)
		if extractErr != nil {
			entry.ExtractError = extractErr.Error()
			out.Faces = append(out.Faces, entry)
			continue
		}
		entry.Extracted = true
		entry.VertexCount = vertexCount
		out.ExtractedInBBox++
		entry.Pass = worldimpl.FacePass(entry.DerivedFlags, worldimpl.FaceAlpha(entry.DerivedFlags, liquidAlpha))
		switch entry.Pass {
		case worldimpl.PassSky:
			out.PassSky++
		case worldimpl.PassOpaque:
			out.PassOpaque++
		case worldimpl.PassAlphaTest:
			out.PassAlphaTest++
		case worldimpl.PassTranslucent:
			out.PassTranslucent++
		}

		out.Faces = append(out.Faces, entry)
	}

	return out, nil
}

func faceCenterAndNormal(tree *bsp.Tree, face *bsp.TreeFace) ([3]float32, [3]float32, error) {
	var center [3]float32
	var normal [3]float32
	if face == nil {
		return center, normal, fmt.Errorf("nil face")
	}
	if int(face.PlaneNum) < 0 || int(face.PlaneNum) >= len(tree.Planes) {
		return center, normal, fmt.Errorf("invalid plane num %d", face.PlaneNum)
	}
	normal = tree.Planes[face.PlaneNum].Normal
	if face.Side != 0 {
		normal[0] = -normal[0]
		normal[1] = -normal[1]
		normal[2] = -normal[2]
	}
	if face.NumEdges <= 0 {
		return center, normal, fmt.Errorf("face has no edges")
	}
	for i := int32(0); i < face.NumEdges; i++ {
		surfEdgeIdx := int(face.FirstEdge + i)
		if surfEdgeIdx < 0 || surfEdgeIdx >= len(tree.Surfedges) {
			return center, normal, fmt.Errorf("surfedge index %d out of range", surfEdgeIdx)
		}
		surfEdge := tree.Surfedges[surfEdgeIdx]
		var vertIdx uint32
		if surfEdge >= 0 {
			if int(surfEdge) >= len(tree.Edges) {
				return center, normal, fmt.Errorf("edge index %d out of range", surfEdge)
			}
			vertIdx = tree.Edges[surfEdge].V[0]
		} else {
			edgeIdx := -surfEdge
			if int(edgeIdx) >= len(tree.Edges) {
				return center, normal, fmt.Errorf("edge index %d out of range", edgeIdx)
			}
			vertIdx = tree.Edges[edgeIdx].V[1]
		}
		if int(vertIdx) >= len(tree.Vertexes) {
			return center, normal, fmt.Errorf("vertex index %d out of range", vertIdx)
		}
		v := tree.Vertexes[vertIdx]
		center[0] += v.Point[0]
		center[1] += v.Point[1]
		center[2] += v.Point[2]
	}
	inv := 1.0 / float32(face.NumEdges)
	center[0] *= inv
	center[1] *= inv
	center[2] *= inv
	return center, normal, nil
}

func pointInBounds(point, boundsMin, boundsMax [3]float32) bool {
	return point[0] >= boundsMin[0] && point[0] <= boundsMax[0] &&
		point[1] >= boundsMin[1] && point[1] <= boundsMax[1] &&
		point[2] >= boundsMin[2] && point[2] <= boundsMax[2]
}

func extractFaceVertexCount(tree *bsp.Tree, face *bsp.TreeFace) (int, error) {
	if int(face.NumEdges) < 3 {
		return 0, fmt.Errorf("face has < 3 edges")
	}
	for i := int32(0); i < face.NumEdges; i++ {
		surfEdgeIdx := int(face.FirstEdge) + int(i)
		if surfEdgeIdx >= len(tree.Surfedges) {
			return 0, fmt.Errorf("surfedge index %d out of range", surfEdgeIdx)
		}
		surfEdge := tree.Surfedges[surfEdgeIdx]
		var vertIdx uint32
		if surfEdge >= 0 {
			if int(surfEdge) >= len(tree.Edges) {
				return 0, fmt.Errorf("edge index %d out of range", surfEdge)
			}
			vertIdx = tree.Edges[surfEdge].V[0]
		} else {
			edgeIdx := -surfEdge
			if int(edgeIdx) >= len(tree.Edges) {
				return 0, fmt.Errorf("edge index %d out of range", edgeIdx)
			}
			vertIdx = tree.Edges[edgeIdx].V[1]
		}
		if int(vertIdx) >= len(tree.Vertexes) {
			return 0, fmt.Errorf("vertex index %d out of range", vertIdx)
		}
	}
	return int(face.NumEdges), nil
}
