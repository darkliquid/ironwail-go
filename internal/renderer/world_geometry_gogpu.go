package renderer

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/model"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
)

const worldLightmapPageSize = 1024

// BuildWorldGeometry extracts renderable geometry from a BSP tree.
// This converts the BSP's face/edge/vertex structure into a simple
// vertex buffer + index buffer suitable for GPU rendering.
//
// The function:
// - Iterates all faces in the world model (model 0)
// - Extracts vertices via the edge/surfedge indirection
// - Computes texture coordinates from TexInfo
// - Triangulates faces using fan triangulation
// - Computes normals from plane data
//
// For MVP implementation, this processes ALL faces without culling.
// Future optimization: PVS culling, frustum culling, face sorting.
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
		Vertices:             make([]WorldVertex, 0, 4096),
		Indices:              make([]uint32, 0, 16384),
		Faces:                make([]WorldFace, 0, 256),
		LiquidAlphaOverrides: worldimpl.ParseWorldspawnLiquidAlphaOverrides(tree.Entities),
		TransparentWaterSafe: worldimpl.MapVisTransparentWaterSafe(tree),
		Tree:                 tree,
	}
	lightmapAllocator, err := NewLightmapAllocator(worldLightmapPageSize, worldLightmapPageSize, false)
	if err != nil {
		return nil, fmt.Errorf("create lightmap allocator: %w", err)
	}
	lightmapPages := make([]WorldLightmapPage, 0, 4)
	textureMeta := parseWorldTextureMeta(tree)

	// Process all faces in the selected model.
	numFaces := int(worldModel.NumFaces)
	firstFace := int(worldModel.FirstFace)
	faceLookup := make(map[int]int, numFaces)

	slog.Debug("Building world geometry",
		"numFaces", numFaces,
		"numVertices", len(tree.Vertexes),
		"numEdges", len(tree.Edges))

	for faceIdx := 0; faceIdx < numFaces; faceIdx++ {
		globalFaceIdx := firstFace + faceIdx
		if globalFaceIdx >= len(tree.Faces) {
			break
		}

		face := &tree.Faces[globalFaceIdx]

		// Extract face metadata
		faceData := WorldFace{
			FirstIndex:    uint32(len(geom.Indices)),
			NumIndices:    0, // Will be computed during triangulation
			TextureIndex:  worldFaceTextureIndex(tree, face),
			LightmapIndex: -1,
			Flags:         worldFaceFlags(textureMeta, tree, face),
		}

		// Extract vertices for this face
		faceVerts, lightmapSurface, err := extractFaceVertices(tree, face, lightmapAllocator, &lightmapPages)
		if err != nil {
			slog.Warn("Failed to extract face vertices",
				"faceIdx", globalFaceIdx,
				"error", err)
			continue
		}

		if len(faceVerts) < 3 {
			// Skip degenerate faces
			continue
		}
		if lightmapSurface != nil {
			faceData.LightmapIndex = int32(lightmapSurface.pageIndex)
		}
		faceData.Center = worldFaceCenter(faceVerts)
		if worldFaceHasLitWater(faceData.Flags, lightmapSurface) {
			geom.HasLitWater = true
		}
		if faceData.Flags&model.SurfDrawTurb != 0 {
			geom.LiquidFaceTypes |= faceData.Flags & (model.SurfDrawLava | model.SurfDrawSlime | model.SurfDrawTele | model.SurfDrawWater)
		}

		// Triangulate face using fan triangulation
		// Face with N vertices becomes (N-2) triangles
		baseVertIdx := uint32(len(geom.Vertices))

		// Add all vertices for this face
		geom.Vertices = append(geom.Vertices, faceVerts...)

		// Generate triangle indices (fan triangulation around vertex 0)
		for i := 1; i < len(faceVerts)-1; i++ {
			geom.Indices = append(geom.Indices,
				baseVertIdx,             // Vertex 0 (fan center)
				baseVertIdx+uint32(i),   // Vertex i
				baseVertIdx+uint32(i+1)) // Vertex i+1
		}

		faceData.NumIndices = uint32((len(faceVerts) - 2) * 3)
		geom.Faces = append(geom.Faces, faceData)
		faceLookup[globalFaceIdx] = len(geom.Faces) - 1
	}

	slog.Debug("World geometry built",
		"vertices", len(geom.Vertices),
		"indices", len(geom.Indices),
		"faces", len(geom.Faces),
		"triangles", len(geom.Indices)/3)

	geom.LeafFaces = buildWorldLeafFaceLookup(tree, faceLookup)
	geom.Lightmaps = lightmapPages
	return geom, nil
}

// extractFaceVertices extracts all vertices for a BSP face.
// It follows the edge/surfedge indirection to get vertex positions,
// then computes texture/lightmap coords and normals.
func extractFaceVertices(tree *bsp.Tree, face *bsp.TreeFace, allocator *LightmapAllocator, pages *[]WorldLightmapPage) ([]WorldVertex, *faceLightmapSurface, error) {
	numEdges := int(face.NumEdges)
	if numEdges < 3 {
		return nil, nil, fmt.Errorf("face has < 3 edges")
	}

	vertices := make([]WorldVertex, 0, numEdges)
	rawLightmapCoords := make([][2]float64, 0, numEdges)

	// Get plane normal for this face
	var normal [3]float32
	if int(face.PlaneNum) < len(tree.Planes) {
		planeNormal := tree.Planes[face.PlaneNum].Normal
		normal = planeNormal
		// If face is on back side of plane, flip normal
		if face.Side != 0 {
			normal[0] = -normal[0]
			normal[1] = -normal[1]
			normal[2] = -normal[2]
		}
	} else {
		// Invalid plane number - log warning
		slog.Warn("Invalid plane number for face",
			"planeNum", face.PlaneNum,
			"numPlanes", len(tree.Planes))
	}

	// Check if normal is valid (not all zeros)
	normalLen := float32(math.Sqrt(float64(normal[0]*normal[0] + normal[1]*normal[1] + normal[2]*normal[2])))
	if normalLen < 0.01 {
		slog.Warn("Invalid normal for face",
			"faceIdx", face,
			"normalLen", normalLen)
	}

	texInfo := worldFaceTexInfo(tree, face)
	textureWidth, textureHeight := worldTextureDimensions(tree, texInfo)

	// Iterate through edges to extract vertex positions
	for i := int32(0); i < face.NumEdges; i++ {
		surfEdgeIdx := int(face.FirstEdge) + int(i)
		if surfEdgeIdx >= len(tree.Surfedges) {
			return nil, nil, fmt.Errorf("surfedge index %d out of range", surfEdgeIdx)
		}

		surfEdge := tree.Surfedges[surfEdgeIdx]

		// Surfedge is signed: positive = use edge V[0], negative = use edge V[1]
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

		texCoord := [2]float32{0.0, 0.0}
		lightmapCoord := [2]float32{0.0, 0.0}
		if texInfo != nil {
			u := worldTexCoordDouble(position, texInfo.Vecs[0])
			v := worldTexCoordDouble(position, texInfo.Vecs[1])
			texCoord = [2]float32{float32(u) / textureWidth, float32(v) / textureHeight}
			rawLightmapCoords = append(rawLightmapCoords, [2]float64{u, v})
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

// worldFaceTexInfo resolves the texture-info record for a BSP face, which maps geometric vertices into texture/lightmap UV space.
func worldFaceTexInfo(tree *bsp.Tree, face *bsp.TreeFace) *bsp.Texinfo {
	if tree == nil || face == nil {
		return nil
	}
	if int(face.Texinfo) < 0 || int(face.Texinfo) >= len(tree.Texinfo) {
		return nil
	}
	return &tree.Texinfo[face.Texinfo]
}

// worldFaceTextureIndex resolves the diffuse texture atlas slot for a face so world pass shaders can sample the correct base map.
func worldFaceTextureIndex(tree *bsp.Tree, face *bsp.TreeFace) int32 {
	texInfo := worldFaceTexInfo(tree, face)
	if texInfo == nil || texInfo.Miptex < 0 {
		return -1
	}
	return texInfo.Miptex
}

// worldFaceLightmapIndex returns the lightmap atlas page/index used for static lighting lookup during world shading.
func worldFaceLightmapIndex(face *bsp.TreeFace) int32 {
	if face == nil || face.LightOfs < 0 || face.Styles[0] == 255 {
		return -1
	}
	// gogpu path does not allocate lightmap pages yet; keep a stable "present" sentinel.
	return 0
}

// worldFaceFlags exposes per-face material/render flags (sky, liquid, turbulent, etc.) that drive pass routing and shader behavior.
func worldFaceFlags(textureMeta []worldTextureMeta, tree *bsp.Tree, face *bsp.TreeFace) int32 {
	texInfo := worldFaceTexInfo(tree, face)
	if texInfo == nil {
		return 0
	}
	textureType := classifyWorldTextureName("")
	if int(texInfo.Miptex) >= 0 && int(texInfo.Miptex) < len(textureMeta) {
		textureType = textureMeta[texInfo.Miptex].Type
	}
	return deriveWorldFaceFlags(textureType, texInfo.Flags)
}

// worldTextureDimensions fetches source texture dimensions for texel-density and UV conversion computations.
func worldTextureDimensions(tree *bsp.Tree, texInfo *bsp.Texinfo) (float32, float32) {
	textureWidth := float32(1)
	textureHeight := float32(1)
	if tree == nil || texInfo == nil || texInfo.Miptex < 0 || len(tree.TextureData) < 4 {
		return textureWidth, textureHeight
	}

	textureCount := int(int32(binary.LittleEndian.Uint32(tree.TextureData[:4])))
	miptexIndex := int(texInfo.Miptex)
	if miptexIndex < 0 || miptexIndex >= textureCount {
		return textureWidth, textureHeight
	}
	offsetTableEnd := 4 + textureCount*4
	if len(tree.TextureData) < offsetTableEnd {
		return textureWidth, textureHeight
	}

	offsetPos := 4 + miptexIndex*4
	offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[offsetPos : offsetPos+4])))
	if offset <= 0 || offset >= len(tree.TextureData) {
		return textureWidth, textureHeight
	}

	miptex, err := image.ParseMipTex(tree.TextureData[offset:])
	if err != nil {
		return textureWidth, textureHeight
	}

	if miptex.Width > 0 {
		textureWidth = float32(miptex.Width)
	}
	if miptex.Height > 0 {
		textureHeight = float32(miptex.Height)
	}
	return textureWidth, textureHeight
}

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

func worldLiquidAlphaSettingsForGeometry(geom *WorldGeometry) worldLiquidAlphaSettings {
	if geom == nil {
		return worldLiquidAlphaSettingsFromCvars(worldLiquidAlphaOverrides{}, nil)
	}
	settings := resolveWorldLiquidAlphaSettings(
		worldimpl.ReadAlphaCvar(CvarRWaterAlpha, 1),
		worldimpl.ReadAlphaCvar(CvarRLavaAlpha, 0),
		worldimpl.ReadAlphaCvar(CvarRSlimeAlpha, 0),
		worldimpl.ReadAlphaCvar(CvarRTeleAlpha, 0),
		worldLiquidAlphaOverridesFromWorld(geom.LiquidAlphaOverrides),
		nil,
	)
	if !geom.TransparentWaterSafe {
		settings.water = 1
		settings.lava = 1
		settings.slime = 1
		settings.tele = 1
	}
	return settings
}

func assignFaceLightmap(vertices []WorldVertex, rawCoords [][2]float64, face *bsp.TreeFace, tree *bsp.Tree, allocator *LightmapAllocator, pages *[]WorldLightmapPage) (*faceLightmapSurface, error) {
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

	textureMinU := math.Floor(minU/16.0) * 16.0
	textureMinV := math.Floor(minV/16.0) * 16.0
	extentU := int(math.Ceil(maxU/16.0)*16.0 - textureMinU)
	extentV := int(math.Ceil(maxV/16.0)*16.0 - textureMinV)
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

	sampleSize8 := smax * tmax * styleCount
	samples := expandLightmapSamples(tree.Lighting, tree.LightingRGB, int(face.LightOfs), sampleSize8)
	if samples == nil {
		return nil, nil
	}

	(*pages)[texNum].Surfaces = append((*pages)[texNum].Surfaces, WorldLightmapSurface{
		X:       x,
		Y:       y,
		Width:   smax,
		Height:  tmax,
		Styles:  face.Styles,
		Samples: samples,
	})

	for i := range vertices {
		lightS := float32((rawCoords[i][0]-textureMinU)/16.0 + float64(x) + 0.5)
		lightT := float32((rawCoords[i][1]-textureMinV)/16.0 + float64(y) + 0.5)
		vertices[i].LightmapCoord = [2]float32{
			lightS / float32(worldLightmapPageSize),
			lightT / float32(worldLightmapPageSize),
		}
	}

	return &faceLightmapSurface{pageIndex: texNum}, nil
}

func worldTexCoordDouble(position [3]float32, vec [4]float32) float64 {
	return float64(position[0])*float64(vec[0]) +
		float64(position[1])*float64(vec[1]) +
		float64(position[2])*float64(vec[2]) +
		float64(vec[3])
}
