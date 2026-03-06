//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/image"
)

type worldTextureMeta struct {
	Width  int
	Height int
	Name   string
}

// WorldGeometry holds BSP world data prepared for rendering.
type WorldGeometry struct {
	Vertices []WorldVertex
	Indices  []uint32
	Faces    []WorldFace
	Tree     *bsp.Tree
}

// WorldVertex matches the packed vertex layout used by the OpenGL world path.
type WorldVertex struct {
	Position      [3]float32
	TexCoord      [2]float32
	LightmapCoord [2]float32
	Normal        [3]float32
}

// WorldFace stores rendering metadata for a BSP face.
type WorldFace struct {
	FirstIndex    uint32
	NumIndices    uint32
	TextureIndex  int32
	LightmapIndex int32
	Flags         int32
}

// WorldRenderData holds CPU-side world data and bounds.
type WorldRenderData struct {
	Geometry      *WorldGeometry
	BoundsMin     [3]float32
	BoundsMax     [3]float32
	TotalVertices int
	TotalIndices  int
	TotalFaces    int
}

// BuildWorldGeometry extracts renderable geometry from BSP data.
func BuildWorldGeometry(tree *bsp.Tree) (*WorldGeometry, error) {
	if tree == nil {
		return nil, fmt.Errorf("nil BSP tree")
	}
	if len(tree.Models) == 0 {
		return nil, fmt.Errorf("BSP has no models")
	}

	worldModel := tree.Models[0]
	geom := &WorldGeometry{
		Vertices: make([]WorldVertex, 0, 4096),
		Indices:  make([]uint32, 0, 16384),
		Faces:    make([]WorldFace, 0, 256),
		Tree:     tree,
	}

	textureMeta := parseWorldTextureMeta(tree)

	numFaces := int(worldModel.NumFaces)
	firstFace := int(worldModel.FirstFace)
	for faceIdx := 0; faceIdx < numFaces; faceIdx++ {
		globalFaceIdx := firstFace + faceIdx
		if globalFaceIdx >= len(tree.Faces) {
			break
		}

		face := &tree.Faces[globalFaceIdx]
		textureIndex := int32(-1)
		textureFlags := int32(0)
		if int(face.Texinfo) >= 0 && int(face.Texinfo) < len(tree.Texinfo) {
			textureIndex = tree.Texinfo[face.Texinfo].Miptex
			textureFlags = tree.Texinfo[face.Texinfo].Flags
		}
		faceData := WorldFace{
			FirstIndex:    uint32(len(geom.Indices)),
			TextureIndex:  textureIndex,
			LightmapIndex: -1,
			Flags:         textureFlags,
		}

		faceVerts, err := extractFaceVertices(tree, face, textureMeta)
		if err != nil {
			slog.Warn("OpenGL world: failed to extract face", "face", globalFaceIdx, "error", err)
			continue
		}
		if len(faceVerts) < 3 {
			continue
		}

		baseVertIdx := uint32(len(geom.Vertices))
		geom.Vertices = append(geom.Vertices, faceVerts...)
		for i := 1; i < len(faceVerts)-1; i++ {
			geom.Indices = append(geom.Indices,
				baseVertIdx,
				baseVertIdx+uint32(i),
				baseVertIdx+uint32(i+1),
			)
		}

		faceData.NumIndices = uint32((len(faceVerts) - 2) * 3)
		geom.Faces = append(geom.Faces, faceData)
	}

	return geom, nil
}

func parseWorldTextureMeta(tree *bsp.Tree) []worldTextureMeta {
	if tree == nil || len(tree.TextureData) < 4 {
		return nil
	}

	count := int(binary.LittleEndian.Uint32(tree.TextureData[:4]))
	if count <= 0 || len(tree.TextureData) < 4+count*4 {
		return nil
	}

	textures := make([]worldTextureMeta, count)
	for i := 0; i < count; i++ {
		offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[4+i*4:])))
		if offset <= 0 || offset >= len(tree.TextureData) {
			continue
		}
		miptex, err := image.ParseMipTex(tree.TextureData[offset:])
		if err != nil {
			continue
		}
		textures[i] = worldTextureMeta{
			Width:  int(miptex.Width),
			Height: int(miptex.Height),
			Name:   miptex.Name,
		}
	}
	return textures
}

func extractFaceVertices(tree *bsp.Tree, face *bsp.TreeFace, textureMeta []worldTextureMeta) ([]WorldVertex, error) {
	if int(face.NumEdges) < 3 {
		return nil, fmt.Errorf("face has < 3 edges")
	}

	vertices := make([]WorldVertex, 0, int(face.NumEdges))
	var normal [3]float32
	if int(face.PlaneNum) < len(tree.Planes) {
		normal = tree.Planes[face.PlaneNum].Normal
		if face.Side != 0 {
			normal[0] = -normal[0]
			normal[1] = -normal[1]
			normal[2] = -normal[2]
		}
	}
	if n := float32(math.Sqrt(float64(normal[0]*normal[0] + normal[1]*normal[1] + normal[2]*normal[2]))); n > 1e-6 {
		normal[0] /= n
		normal[1] /= n
		normal[2] /= n
	} else {
		normal = [3]float32{0, 0, 1}
	}

	var texInfo *bsp.Texinfo
	textureWidth := float32(1)
	textureHeight := float32(1)
	if int(face.Texinfo) >= 0 && int(face.Texinfo) < len(tree.Texinfo) {
		texInfo = &tree.Texinfo[face.Texinfo]
		if int(texInfo.Miptex) >= 0 && int(texInfo.Miptex) < len(textureMeta) {
			meta := textureMeta[texInfo.Miptex]
			if meta.Width > 0 {
				textureWidth = float32(meta.Width)
			}
			if meta.Height > 0 {
				textureHeight = float32(meta.Height)
			}
		}
	}

	for i := int32(0); i < face.NumEdges; i++ {
		surfEdgeIdx := int(face.FirstEdge) + int(i)
		if surfEdgeIdx >= len(tree.Surfedges) {
			return nil, fmt.Errorf("surfedge index %d out of range", surfEdgeIdx)
		}
		surfEdge := tree.Surfedges[surfEdgeIdx]

		var vertIdx uint32
		if surfEdge >= 0 {
			if int(surfEdge) >= len(tree.Edges) {
				return nil, fmt.Errorf("edge index %d out of range", surfEdge)
			}
			vertIdx = tree.Edges[surfEdge].V[0]
		} else {
			edgeIdx := -surfEdge
			if int(edgeIdx) >= len(tree.Edges) {
				return nil, fmt.Errorf("edge index %d out of range", edgeIdx)
			}
			vertIdx = tree.Edges[edgeIdx].V[1]
		}

		if int(vertIdx) >= len(tree.Vertexes) {
			return nil, fmt.Errorf("vertex index %d out of range", vertIdx)
		}
		position := tree.Vertexes[vertIdx].Point

		texCoord := [2]float32{0, 0}
		if texInfo != nil {
			u := position[0]*texInfo.Vecs[0][0] + position[1]*texInfo.Vecs[0][1] + position[2]*texInfo.Vecs[0][2] + texInfo.Vecs[0][3]
			v := position[0]*texInfo.Vecs[1][0] + position[1]*texInfo.Vecs[1][1] + position[2]*texInfo.Vecs[1][2] + texInfo.Vecs[1][3]
			texCoord[0] = u / textureWidth
			texCoord[1] = v / textureHeight
		}

		vertices = append(vertices, WorldVertex{
			Position:      position,
			TexCoord:      texCoord,
			LightmapCoord: [2]float32{0, 0},
			Normal:        normal,
		})
	}

	return vertices, nil
}

func buildWorldRenderData(tree *bsp.Tree) (*WorldRenderData, error) {
	geom, err := BuildWorldGeometry(tree)
	if err != nil {
		return nil, err
	}

	renderData := &WorldRenderData{
		Geometry:      geom,
		TotalVertices: len(geom.Vertices),
		TotalIndices:  len(geom.Indices),
		TotalFaces:    len(geom.Faces),
	}
	if len(geom.Vertices) == 0 {
		return renderData, nil
	}

	boundsMin := geom.Vertices[0].Position
	boundsMax := geom.Vertices[0].Position
	for i := 1; i < len(geom.Vertices); i++ {
		position := geom.Vertices[i].Position
		if position[0] < boundsMin[0] {
			boundsMin[0] = position[0]
		}
		if position[1] < boundsMin[1] {
			boundsMin[1] = position[1]
		}
		if position[2] < boundsMin[2] {
			boundsMin[2] = position[2]
		}
		if position[0] > boundsMax[0] {
			boundsMax[0] = position[0]
		}
		if position[1] > boundsMax[1] {
			boundsMax[1] = position[1]
		}
		if position[2] > boundsMax[2] {
			boundsMax[2] = position[2]
		}
	}
	renderData.BoundsMin = boundsMin
	renderData.BoundsMax = boundsMax
	return renderData, nil
}
