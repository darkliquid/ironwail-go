//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/model"
)

// worldTextureMeta holds parsed texture metadata (name, dimensions, classified type)
// from the BSP miptex lump entries.
type worldTextureMeta struct {
	Width  int
	Height int
	Name   string
	Type   model.TextureType
}

// WorldGeometry holds BSP world data prepared for rendering.
type WorldGeometry struct {
	Vertices    []WorldVertex
	Indices     []uint32
	Faces       []WorldFace
	Lightmaps   []WorldLightmapPage
	HasLitWater bool
	Tree        *bsp.Tree
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
	Center        [3]float32
}

// WorldLightmapSurface describes a single face's lightmap data within an atlas page,
// including its position, dimensions, lightstyle references, and raw light samples.
type WorldLightmapSurface struct {
	X       int
	Y       int
	Width   int
	Height  int
	Styles  [bsp.MaxLightmaps]uint8
	Samples []byte
	Dirty   bool // true when lightstyle values have changed since last composite
}

// WorldLightmapPage represents a shared lightmap atlas texture page containing multiple
// surface lightmaps packed together to minimize texture binds during rendering.
type WorldLightmapPage struct {
	Width    int
	Height   int
	Surfaces []WorldLightmapSurface
	Dirty    bool   // true when any surface in this page is dirty
	rgba     []byte // cached composited RGBA buffer for partial re-upload
}

// WorldRenderData holds CPU-side world data and bounds.
type WorldRenderData struct {
	Geometry      *WorldGeometry
	Lightmaps     []WorldLightmapPage
	HasLitWater   bool
	BoundsMin     [3]float32
	BoundsMax     [3]float32
	TotalVertices int
	TotalIndices  int
	TotalFaces    int
}

const worldLightmapPageSize = 1024

// BuildWorldGeometry extracts renderable geometry from BSP data.
func BuildWorldGeometry(tree *bsp.Tree) (*WorldGeometry, error) {
	return BuildModelGeometry(tree, 0)
}

// BuildModelGeometry extracts renderable geometry for a specific BSP model index.
func BuildModelGeometry(tree *bsp.Tree, modelIndex int) (*WorldGeometry, error) {
	if tree == nil {
		return nil, fmt.Errorf("nil BSP tree")
	}
	if len(tree.Models) == 0 {
		return nil, fmt.Errorf("BSP has no models")
	}
	if modelIndex < 0 || modelIndex >= len(tree.Models) {
		return nil, fmt.Errorf("model index %d out of range", modelIndex)
	}

	worldModel := tree.Models[modelIndex]
	geom := &WorldGeometry{
		Vertices: make([]WorldVertex, 0, 4096),
		Indices:  make([]uint32, 0, 16384),
		Faces:    make([]WorldFace, 0, 256),
		Tree:     tree,
	}
	lightmapAllocator, err := NewLightmapAllocator(worldLightmapPageSize, worldLightmapPageSize, false)
	if err != nil {
		return nil, fmt.Errorf("create lightmap allocator: %w", err)
	}
	lightmapPages := make([]WorldLightmapPage, 0, 4)

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
		textureType := model.TexTypeDefault
		if int(face.Texinfo) >= 0 && int(face.Texinfo) < len(tree.Texinfo) {
			textureIndex = tree.Texinfo[face.Texinfo].Miptex
			if int(textureIndex) >= 0 && int(textureIndex) < len(textureMeta) {
				textureType = textureMeta[textureIndex].Type
			}
			textureFlags = deriveWorldFaceFlags(textureType, tree.Texinfo[face.Texinfo].Flags)
		}
		faceData := WorldFace{
			FirstIndex:    uint32(len(geom.Indices)),
			TextureIndex:  textureIndex,
			LightmapIndex: -1,
			Flags:         textureFlags,
		}

		faceVerts, lightmapSurface, err := extractFaceVertices(tree, face, textureMeta, lightmapAllocator, &lightmapPages)
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
		faceData.Center = worldFaceCenter(faceVerts)
		if lightmapSurface != nil {
			faceData.LightmapIndex = int32(lightmapSurface.pageIndex)
		}
		if worldFaceHasLitWater(textureFlags, lightmapSurface) {
			geom.HasLitWater = true
		}
		geom.Faces = append(geom.Faces, faceData)
	}
	geom.Lightmaps = lightmapPages

	return geom, nil
}

// parseWorldTextureMeta parses the BSP miptex lump to extract texture names and dimensions.
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
			Type:   classifyWorldTextureName(miptex.Name),
		}
	}
	return textures
}

// classifyWorldTextureName classifies a texture by its name prefix convention: '{' = fence/cutout,
// 'sky' = sky surface, '*lava/*slime/*tele/*' = liquid types. This naming convention dates back
// to Quake's original map tools.
func classifyWorldTextureName(name string) model.TextureType {
	name = strings.TrimRight(strings.ToLower(name), "\x00")
	switch {
	case strings.HasPrefix(name, "{"):
		return model.TexTypeCutout
	case strings.HasPrefix(name, "sky"):
		return model.TexTypeSky
	case strings.HasPrefix(name, "*lava"):
		return model.TexTypeLava
	case strings.HasPrefix(name, "*slime"):
		return model.TexTypeSlime
	case strings.HasPrefix(name, "*tele"):
		return model.TexTypeTele
	case strings.HasPrefix(name, "*"):
		return model.TexTypeWater
	default:
		return model.TexTypeDefault
	}
}

// deriveWorldFaceFlags converts texture type and texinfo flags into surface rendering flags
// that control how the face is drawn (tiled, turbulent warp for liquids, sky, fence alpha test).
func deriveWorldFaceFlags(textureType model.TextureType, texinfoFlags int32) int32 {
	flags := int32(0)
	if texinfoFlags&bsp.TexMissing != 0 {
		flags |= model.SurfNoTexture
	}
	if texinfoFlags&bsp.TexSpecial != 0 {
		flags |= model.SurfDrawTiled
	}

	switch textureType {
	case model.TexTypeCutout:
		flags |= model.SurfDrawFence
	case model.TexTypeSky:
		flags |= model.SurfDrawSky | model.SurfDrawTiled
	case model.TexTypeLava:
		flags |= model.SurfDrawTurb | model.SurfDrawLava | model.SurfDrawTiled
	case model.TexTypeSlime:
		flags |= model.SurfDrawTurb | model.SurfDrawSlime | model.SurfDrawTiled
	case model.TexTypeTele:
		flags |= model.SurfDrawTurb | model.SurfDrawTele | model.SurfDrawTiled
	case model.TexTypeWater:
		flags |= model.SurfDrawTurb | model.SurfDrawWater | model.SurfDrawTiled
	}

	return flags
}

// worldFaceCenter computes the centroid of a face's vertices, used for distance-based
// sorting of translucent faces.
func worldFaceCenter(vertices []WorldVertex) [3]float32 {
	if len(vertices) == 0 {
		return [3]float32{}
	}
	var center [3]float32
	for _, vertex := range vertices {
		center[0] += vertex.Position[0]
		center[1] += vertex.Position[1]
		center[2] += vertex.Position[2]
	}
	scale := 1 / float32(len(vertices))
	center[0] *= scale
	center[1] *= scale
	center[2] *= scale
	return center
}

func worldFaceHasLitWater(textureFlags int32, lightmapSurface *faceLightmapSurface) bool {
	return textureFlags&model.SurfDrawTurb != 0 &&
		textureFlags&model.SurfDrawSky == 0 &&
		lightmapSurface != nil
}

// faceLightmapSurface is an internal result type tracking which atlas page a face's
// lightmap was allocated to during geometry building.
type faceLightmapSurface struct {
	pageIndex int
}

// extractFaceVertices extracts vertices for a BSP face by walking the surfedge table.
// Positive surfedge index means first vertex of edge, negative means second vertex (reversed
// winding). Computes both diffuse texture UVs and raw lightmap UVs.
func extractFaceVertices(tree *bsp.Tree, face *bsp.TreeFace, textureMeta []worldTextureMeta, allocator *LightmapAllocator, pages *[]WorldLightmapPage) ([]WorldVertex, *faceLightmapSurface, error) {
	if int(face.NumEdges) < 3 {
		return nil, nil, fmt.Errorf("face has < 3 edges")
	}

	vertices := make([]WorldVertex, 0, int(face.NumEdges))
	rawLightmapCoords := make([][2]float32, 0, int(face.NumEdges))
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
			return nil, nil, fmt.Errorf("surfedge index %d out of range", surfEdgeIdx)
		}
		surfEdge := tree.Surfedges[surfEdgeIdx]

		var vertIdx uint32
		if surfEdge >= 0 {
			if int(surfEdge) >= len(tree.Edges) {
				return nil, nil, fmt.Errorf("edge index %d out of range", surfEdge)
			}
			vertIdx = tree.Edges[surfEdge].V[0]
		} else {
			edgeIdx := -surfEdge
			if int(edgeIdx) >= len(tree.Edges) {
				return nil, nil, fmt.Errorf("edge index %d out of range", edgeIdx)
			}
			vertIdx = tree.Edges[edgeIdx].V[1]
		}

		if int(vertIdx) >= len(tree.Vertexes) {
			return nil, nil, fmt.Errorf("vertex index %d out of range", vertIdx)
		}
		position := tree.Vertexes[vertIdx].Point

		texCoord := [2]float32{0, 0}
		lightmapCoord := [2]float32{0, 0}
		if texInfo != nil {
			u := position[0]*texInfo.Vecs[0][0] + position[1]*texInfo.Vecs[0][1] + position[2]*texInfo.Vecs[0][2] + texInfo.Vecs[0][3]
			v := position[0]*texInfo.Vecs[1][0] + position[1]*texInfo.Vecs[1][1] + position[2]*texInfo.Vecs[1][2] + texInfo.Vecs[1][3]
			texCoord[0] = u / textureWidth
			texCoord[1] = v / textureHeight
			rawLightmapCoords = append(rawLightmapCoords, [2]float32{u, v})
		}

		vertices = append(vertices, WorldVertex{
			Position:      position,
			TexCoord:      texCoord,
			LightmapCoord: lightmapCoord,
			Normal:        normal,
		})
	}

	lightmapSurface, err := assignFaceLightmap(vertices, rawLightmapCoords, face, tree, allocator, pages)
	if err != nil {
		return nil, nil, err
	}

	return vertices, lightmapSurface, nil
}

// assignFaceLightmap allocates a lightmap rectangle for a face in the atlas and computes
// lightmap texture coordinates. Each face gets a unique region in a shared lightmap page.
func assignFaceLightmap(vertices []WorldVertex, rawCoords [][2]float32, face *bsp.TreeFace, tree *bsp.Tree, allocator *LightmapAllocator, pages *[]WorldLightmapPage) (*faceLightmapSurface, error) {
	if face == nil || tree == nil || allocator == nil || len(vertices) == 0 || len(rawCoords) != len(vertices) || face.LightOfs < 0 || len(tree.Lighting) == 0 {
		return nil, nil
	}

	minU, maxU := rawCoords[0][0], rawCoords[0][0]
	minV, maxV := rawCoords[0][1], rawCoords[0][1]
	for i := 1; i < len(rawCoords); i++ {
		if rawCoords[i][0] < minU {
			minU = rawCoords[i][0]
		}
		if rawCoords[i][0] > maxU {
			maxU = rawCoords[i][0]
		}
		if rawCoords[i][1] < minV {
			minV = rawCoords[i][1]
		}
		if rawCoords[i][1] > maxV {
			maxV = rawCoords[i][1]
		}
	}

	textureMinU := float32(math.Floor(float64(minU/16.0))) * 16.0
	textureMinV := float32(math.Floor(float64(minV/16.0))) * 16.0
	extentU := int(math.Ceil(float64(maxU/16.0))*16.0 - float64(textureMinU))
	extentV := int(math.Ceil(float64(maxV/16.0))*16.0 - float64(textureMinV))
	if extentU < 0 {
		extentU = 0
	}
	if extentV < 0 {
		extentV = 0
	}
	smax := extentU/16 + 1
	tmax := extentV/16 + 1
	if smax <= 0 || tmax <= 0 {
		return nil, nil
	}

	texNum, x, y, err := allocator.AllocBlock(smax, tmax)
	if err != nil {
		return nil, fmt.Errorf("alloc face lightmap: %w", err)
	}
	for len(*pages) <= texNum {
		*pages = append(*pages, WorldLightmapPage{Width: worldLightmapPageSize, Height: worldLightmapPageSize})
	}

	styleCount := 0
	for _, style := range face.Styles {
		if style == 255 {
			break
		}
		styleCount++
	}
	if styleCount == 0 {
		styleCount = 1
	}
	sampleSize := smax * tmax * 3 * styleCount
	if int(face.LightOfs) < 0 || int(face.LightOfs)+sampleSize > len(tree.Lighting) {
		return nil, nil
	}
	samples := append([]byte(nil), tree.Lighting[face.LightOfs:int(face.LightOfs)+sampleSize]...)
	(*pages)[texNum].Surfaces = append((*pages)[texNum].Surfaces, WorldLightmapSurface{
		X:       x,
		Y:       y,
		Width:   smax,
		Height:  tmax,
		Styles:  face.Styles,
		Samples: samples,
	})

	for i := range vertices {
		lightS := (rawCoords[i][0]-textureMinU)/16.0 + float32(x) + 0.5
		lightT := (rawCoords[i][1]-textureMinV)/16.0 + float32(y) + 0.5
		vertices[i].LightmapCoord = [2]float32{
			lightS / float32(worldLightmapPageSize),
			lightT / float32(worldLightmapPageSize),
		}
	}

	return &faceLightmapSurface{pageIndex: texNum}, nil
}

// buildWorldRenderData builds complete CPU-side render data: geometry, lightmaps, and bounding box.
func buildWorldRenderData(tree *bsp.Tree) (*WorldRenderData, error) {
	return buildModelRenderData(tree, 0)
}

// buildModelRenderData builds render data for a specific BSP submodel index (used for brush entities like doors).
func buildModelRenderData(tree *bsp.Tree, modelIndex int) (*WorldRenderData, error) {
	geom, err := BuildModelGeometry(tree, modelIndex)
	if err != nil {
		return nil, err
	}

	renderData := &WorldRenderData{
		Geometry:      geom,
		Lightmaps:     append([]WorldLightmapPage(nil), geom.Lightmaps...),
		HasLitWater:   geom.HasLitWater,
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
