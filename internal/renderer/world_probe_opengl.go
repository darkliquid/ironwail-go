//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"fmt"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/model"
)

// WorldFaceProbeStats captures opt-in world extraction diagnostics for a bbox.
type WorldFaceProbeStats struct {
	ModelIndex int
	BoundsMin  [3]float32
	BoundsMax  [3]float32

	RawInBBox       int
	ExtractedInBBox int

	PassSky         int
	PassOpaque      int
	PassAlphaTest   int
	PassTranslucent int

	Faces []WorldFaceProbeEntry
}

// WorldFaceProbeEntry captures one suspect face from raw BSP through extraction and pass routing.
type WorldFaceProbeEntry struct {
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
	Pass            worldRenderPass
}

func worldRenderPassName(pass worldRenderPass) string {
	switch pass {
	case worldPassSky:
		return "sky"
	case worldPassOpaque:
		return "opaque"
	case worldPassAlphaTest:
		return "alpha-test"
	case worldPassTranslucent:
		return "translucent"
	default:
		return fmt.Sprintf("unknown(%d)", pass)
	}
}

// ProbeWorldFacesInBBox inspects world/model faces in an opt-in diagnostic path.
// It reports raw suspect faces in bbox, extraction results, and render pass routing.
func ProbeWorldFacesInBBox(tree *bsp.Tree, modelIndex int, boundsMin, boundsMax [3]float32, liquidAlpha worldLiquidAlphaSettings) (*WorldFaceProbeStats, error) {
	if tree == nil {
		return nil, fmt.Errorf("nil BSP tree")
	}
	if len(tree.Models) == 0 {
		return nil, fmt.Errorf("BSP has no models")
	}
	if modelIndex < 0 || modelIndex >= len(tree.Models) {
		return nil, fmt.Errorf("model index %d out of range", modelIndex)
	}

	meta := parseWorldTextureMeta(tree)
	modelDef := tree.Models[modelIndex]
	out := &WorldFaceProbeStats{
		ModelIndex: modelIndex,
		BoundsMin:  boundsMin,
		BoundsMax:  boundsMax,
		Faces:      make([]WorldFaceProbeEntry, 0, 64),
	}

	for localFace := 0; localFace < int(modelDef.NumFaces); localFace++ {
		faceIndex := int(modelDef.FirstFace) + localFace
		if faceIndex < 0 || faceIndex >= len(tree.Faces) {
			break
		}
		face := &tree.Faces[faceIndex]
		center, normal, err := worldFaceCenterAndNormal(tree, face)
		if err != nil || !pointInBounds(center, boundsMin, boundsMax) {
			continue
		}

		entry := WorldFaceProbeEntry{
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
				entry.DerivedFlags = deriveWorldFaceFlags(meta[texInfo.Miptex].Type, texInfo.Flags)
			} else {
				entry.DerivedFlags = deriveWorldFaceFlags(model.TexTypeDefault, texInfo.Flags)
			}
		}

		verts, _, extractErr := extractFaceVertices(tree, face, meta, nil, nil)
		if extractErr != nil {
			entry.ExtractError = extractErr.Error()
			out.Faces = append(out.Faces, entry)
			continue
		}
		entry.Extracted = true
		entry.VertexCount = len(verts)
		out.ExtractedInBBox++
		entry.Pass = worldFacePass(entry.DerivedFlags, worldFaceAlpha(entry.DerivedFlags, liquidAlpha))
		switch entry.Pass {
		case worldPassSky:
			out.PassSky++
		case worldPassOpaque:
			out.PassOpaque++
		case worldPassAlphaTest:
			out.PassAlphaTest++
		case worldPassTranslucent:
			out.PassTranslucent++
		}

		out.Faces = append(out.Faces, entry)
	}

	return out, nil
}

func worldFaceCenterAndNormal(tree *bsp.Tree, face *bsp.TreeFace) ([3]float32, [3]float32, error) {
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
		if int(vertIdx) < 0 || int(vertIdx) >= len(tree.Vertexes) {
			return center, normal, fmt.Errorf("vertex index %d out of range", vertIdx)
		}
		p := tree.Vertexes[vertIdx].Point
		center[0] += p[0]
		center[1] += p[1]
		center[2] += p[2]
	}
	scale := 1 / float32(face.NumEdges)
	center[0] *= scale
	center[1] *= scale
	center[2] *= scale
	return center, normal, nil
}

func pointInBounds(point, boundsMin, boundsMax [3]float32) bool {
	return point[0] >= boundsMin[0] && point[0] <= boundsMax[0] &&
		point[1] >= boundsMin[1] && point[1] <= boundsMax[1] &&
		point[2] >= boundsMin[2] && point[2] <= boundsMax[2]
}
