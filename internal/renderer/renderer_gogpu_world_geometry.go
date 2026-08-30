package renderer

import (
	"fmt"
	"log/slog"
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
	surfacepkg "github.com/darkliquid/ironwail-go/internal/renderer/surface"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
	"github.com/darkliquid/ironwail-go/pkg/types"
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
	lightmapAllocator, err := surfacepkg.NewLightmapAllocator(worldLightmapPageSize, worldLightmapPageSize, false)
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

	// Debug: Report texture index ranges for large maps if needed
	if numFaces > 1000 {
		debugTextureIndexRange(tree, tree.Faces)
	}

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
			slog.Debug("lit water face detected",
				"face_index", globalFaceIdx,
				"flags", faceData.Flags,
				"lightmap_index", faceData.LightmapIndex,
			)
		} else if faceData.Flags&model.SurfDrawTurb != 0 && faceData.Flags&model.SurfDrawSky == 0 {
			slog.Debug("turbulent face NOT lit water",
				"face_index", globalFaceIdx,
				"flags", faceData.Flags,
				"has_tiled", faceData.Flags&model.SurfDrawTiled != 0,
				"has_lightmap_surface", lightmapSurface != nil,
				"lightmap_index", faceData.LightmapIndex,
			)
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

	// Diagnostic: count faces by render pass classification.
	fenceCount := 0
	skyCount := 0
	turbCount := 0
	opaqueCount := 0
	for _, face := range geom.Faces {
		if face.NumIndices == 0 {
			continue
		}
		if face.Flags&model.SurfDrawSky != 0 {
			skyCount++
		} else if face.Flags&model.SurfDrawFence != 0 {
			fenceCount++
		} else if face.Flags&model.SurfDrawTurb != 0 {
			turbCount++
		} else {
			opaqueCount++
		}
		// Dump flags for dopefish texture (miptex 37)
		if face.TextureIndex == 37 {
			slog.Debug("Dopefish face flags",
				"texture_index", face.TextureIndex,
				"flags", face.Flags,
				"has_fence", face.Flags&model.SurfDrawFence != 0,
				"has_sky", face.Flags&model.SurfDrawSky != 0,
				"has_turb", face.Flags&model.SurfDrawTurb != 0,
				"has_tiled", face.Flags&model.SurfDrawTiled != 0,
				"num_indices", face.NumIndices,
				"lightmap_index", face.LightmapIndex,
			)
		}
	}
	slog.Debug("Face classification",
		"total_faces", len(geom.Faces),
		"opaque", opaqueCount,
		"fence", fenceCount,
		"sky", skyCount,
		"turb", turbCount,
	)

	// Lightmap diagnostic: count how many faces got lightmaps vs not,
	// and log the reasons for non-lightmapped faces.
	lightmappedCount := 0
	noLightmapCount := 0
	noLightOfsCount := 0
	noLightingDataCount := 0
	noSamplesCount := 0
	for _, face := range geom.Faces {
		if face.LightmapIndex >= 0 {
			lightmappedCount++
		} else {
			noLightmapCount++
		}
	}
	// Sample first N faces to understand why lightmaps are missing.
	if noLightmapCount > 0 && lightmappedCount == 0 {
		sampleCount := 0
		for globalFaceIdx := firstFace; globalFaceIdx < firstFace+numFaces && sampleCount < 20; globalFaceIdx++ {
			if globalFaceIdx >= len(tree.Faces) {
				break
			}
			face := &tree.Faces[globalFaceIdx]
			if face.LightOfs < 0 {
				noLightOfsCount++
			} else if len(tree.Lighting) == 0 {
				noLightingDataCount++
			} else {
				// Check if expandLightmapSamples returns nil
				texInfo := worldFaceTexInfo(tree, face)
				if texInfo == nil {
					noSamplesCount++
				}
			}
			sampleCount++
		}
		slog.Warn("Lightmap diagnostic: no faces received lightmaps",
			"total_faces", len(geom.Faces),
			"lightmapped_faces", lightmappedCount,
			"no_lightmap_faces", noLightmapCount,
			"no_lightofs_in_sample", noLightOfsCount,
			"no_lighting_data_in_sample", noLightingDataCount,
			"no_samples_in_sample", noSamplesCount,
			"lighting_data_len", len(tree.Lighting),
			"bsp_version", tree.Version,
		)
	} else {
		// Count how many faces in the BSP have LightOfs < 0 (no lightmap data).
		noLightOfsTotal := 0
		noLightOfsSky := 0
		noLightOfsTurb := 0
		noLightOfsTiled := 0
		noLightOfsOther := 0
		for globalFaceIdx := firstFace; globalFaceIdx < firstFace+numFaces && globalFaceIdx < len(tree.Faces); globalFaceIdx++ {
			bface := &tree.Faces[globalFaceIdx]
			if bface.LightOfs < 0 {
				noLightOfsTotal++
				// Check if the face is sky/turb/tiled (expected to have no lightmap)
				texInfo := worldFaceTexInfo(tree, bface)
				if texInfo != nil {
					if texInfo.Flags&bsp.TexSpecial != 0 {
						noLightOfsTiled++
					}
					textureMeta := parseWorldTextureMeta(tree)
					textureType := classifyWorldTextureName("")
					if int(texInfo.Miptex) >= 0 && int(texInfo.Miptex) < len(textureMeta) {
						textureType = textureMeta[texInfo.Miptex].Type
					}
					flags := deriveWorldFaceFlags(textureType, texInfo.Flags)
					if flags&model.SurfDrawSky != 0 {
						noLightOfsSky++
					} else if flags&model.SurfDrawTurb != 0 {
						noLightOfsTurb++
					} else if noLightOfsTiled == 0 || (flags&(model.SurfDrawSky|model.SurfDrawTurb) == 0 && texInfo.Flags&bsp.TexSpecial == 0) {
						noLightOfsOther++
					}
				} else {
					noLightOfsOther++
				}
			}
		}
		slog.Debug("Lightmap diagnostic",
			"total_faces", len(geom.Faces),
			"lightmapped_faces", lightmappedCount,
			"no_lightmap_faces", noLightmapCount,
			"no_lightofs_total", noLightOfsTotal,
			"no_lightofs_sky", noLightOfsSky,
			"no_lightofs_turb", noLightOfsTurb,
			"no_lightofs_tiled", noLightOfsTiled,
			"no_lightofs_other", noLightOfsOther,
			"lightmap_pages", len(lightmapPages),
			"bsp_version", tree.Version,
		)
	}

	// Phase 2 diagnostic: validate materialID range against the material
	// buffer capacity (len(tree.Textures) + 2 dummy slots, matching the
	// world material table size). Faces with textureIndex >= capacity will
	// produce out-of-bounds reads in the WGSL materials[] storage array.
	materialCapacity := 0
	if tc, ok := worldimpl.TextureCount(tree); ok {
		materialCapacity = int(tc) + 2
	}
	diagMaterialIDRange(geom, materialCapacity)
	diagMaterialIDFaceAudit(geom, 50, materialCapacity)

	// Convert lightmap layer from page index to V-offset for the
	// vertically-stacked lightmap texture. Each page occupies
	// (pageSize + 2) rows (1px padding top + 1px padding bottom).
	// Page i's content starts at row (i * (pageSize + 2) + 1).
	// The V-offset = (i * (pageSize + 2) + 1) / (numPages * (pageSize + 2)).
	if len(lightmapPages) > 0 {
		pageSize := float32(worldLightmapPageSize)
		rowsPerPage := pageSize + 2
		totalTallHeight := float32(len(lightmapPages)) * rowsPerPage
		for i := range geom.Vertices {
			pageIdx := geom.Vertices[i].LightmapLayer
			if pageIdx > 0 || geom.Vertices[i].LightmapCoord[0] != 0 || geom.Vertices[i].LightmapCoord[1] != 0 {
				geom.Vertices[i].LightmapLayer = (pageIdx*rowsPerPage + 1) / totalTallHeight
				// Rescale lightmap V to account for vertical stacking.
				geom.Vertices[i].LightmapCoord[1] = geom.Vertices[i].LightmapCoord[1] * pageSize / totalTallHeight
			}
		}
	}

	geom.LeafFaces = buildWorldLeafFaceLookup(tree, faceLookup)
	geom.Lightmaps = lightmapPages
	return geom, nil
}

// extractFaceVertices extracts all vertices for a BSP face.
// It follows the edge/surfedge indirection to get vertex positions,
// then computes texture/lightmap coords and normals.
func extractFaceVertices(tree *bsp.Tree, face *bsp.TreeFace, allocator *surfacepkg.LightmapAllocator, pages *[]WorldLightmapPage) ([]WorldVertex, *faceLightmapSurface, error) {
	numEdges := int(face.NumEdges)
	if numEdges < 3 {
		return nil, nil, fmt.Errorf("face has < 3 edges")
	}

	vertices := make([]WorldVertex, 0, numEdges)
	rawLightmapCoords := make([][2]float64, 0, numEdges)

	// Get plane normal for this face
	var normal types.Vec3
	if int(face.PlaneNum) < len(tree.Planes) {
		normal = tree.Planes[face.PlaneNum].Normal
		// If face is on back side of plane, flip normal
		if face.Side != 0 {
			normal = normal.Neg()
		}
	} else {
		// Invalid plane number - log warning
		slog.Warn("Invalid plane number for face",
			"planeNum", face.PlaneNum,
			"numPlanes", len(tree.Planes))
	}

	// Check if normal is valid (not all zeros)
	normalLen := normal.Len()
	if normalLen < 0.01 {
		slog.Warn("Invalid normal for face",
			"faceIdx", face,
			"normalLen", normalLen)
	}

	texInfo := worldFaceTexInfo(tree, face)
	textureWidth, textureHeight := worldTextureDimensions(tree, texInfo)

	// Compute the material ID once per face using the same remapping logic
	// as worldFaceTextureIndex, which handles missing/invalid miptex indices
	// by mapping them to dummy texture slots (textureCount / textureCount+1).
	textureIndex := worldFaceTextureIndex(tree, face)
	var materialID uint32
	if textureIndex >= 0 {
		materialID = uint32(textureIndex)
	}

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
			if texInfo.Flags&model.SurfDrawTurb != 0 {
				// Match C Ironwail (r_brush.c:637-641): turbulent surfaces use 1/128.0 scale without offset
				uTurb := float64(position.X)*float64(texInfo.Vecs[0][0]) + float64(position.Y)*float64(texInfo.Vecs[0][1]) + float64(position.Z)*float64(texInfo.Vecs[0][2])
				vTurb := float64(position.X)*float64(texInfo.Vecs[1][0]) + float64(position.Y)*float64(texInfo.Vecs[1][1]) + float64(position.Z)*float64(texInfo.Vecs[1][2])
				texCoord = [2]float32{float32(uTurb) / 128.0, float32(vTurb) / 128.0}
			} else {
				texCoord = [2]float32{float32(u) / textureWidth, float32(v) / textureHeight}
			}
			rawLightmapCoords = append(rawLightmapCoords, [2]float64{u, v})
		}

		vertices = append(vertices, WorldVertex{
			Position:      position,
			TexCoord:      texCoord,
			LightmapCoord: lightmapCoord,
			Normal:        normal,
			MaterialID:    materialID,
		})
	}

	lightmapSurface, err := assignFaceLightmap(vertices, rawLightmapCoords, face, tree, allocator, pages)
	if err != nil {
		return nil, nil, err
	}
	return vertices, lightmapSurface, nil
}

// worldFaceTexInfo resolves the texture-info record for a BSP face, which maps
// geometric vertices into texture/lightmap UV space. Lives in world/texture.go.
func worldFaceTexInfo(tree *bsp.Tree, face *bsp.TreeFace) *bsp.Texinfo {
	return worldimpl.FaceTexInfo(tree, face)
}

// worldFaceTextureIndex resolves the diffuse texture atlas slot for a face.
// Lives in world/texture.go.
func worldFaceTextureIndex(tree *bsp.Tree, face *bsp.TreeFace) int32 {
	return worldimpl.FaceTextureIndex(tree, face)
}

// Debug function to check the range of texture indices we're seeing
func debugTextureIndexRange(tree *bsp.Tree, faces []bsp.TreeFace) {
	// Report on min, max, and unique texture indices as a sanity check
	textureIndices := make(map[int32]bool)
	maxIndex := int32(-1)
	minIndex := int32(1000000)

	// If we have a large set of faces, sample them for showing stats
	sampleSize := len(faces)
	if sampleSize > 1000 {
		sampleSize = 1000
	}

	for i := 0; i < sampleSize; i++ {
		face := &faces[i]
		texInfo := worldFaceTexInfo(tree, face)
		if texInfo == nil {
			continue
		}
		index := texInfo.Miptex
		if index >= 0 {
			textureIndices[index] = true
			if index > maxIndex {
				maxIndex = index
			}
			if index < minIndex {
				minIndex = index
			}
		}
	}

	if len(textureIndices) > 0 {
		slog.Debug("Texture index range",
			"min_index", minIndex,
			"max_index", maxIndex,
			"unique_indices", len(textureIndices),
			"sample_size", sampleSize,
		)
	}
}

// worldFaceFlags exposes per-face material/render flags (sky, liquid, turbulent,
// etc.) that drive pass routing and shader behavior. Lives in world/texture.go.
func worldFaceFlags(textureMeta []worldTextureMeta, tree *bsp.Tree, face *bsp.TreeFace) int32 {
	if len(textureMeta) == 0 {
		return worldimpl.FaceFlags(nil, tree, face)
	}
	meta := make([]worldimpl.TextureMeta, len(textureMeta))
	for i, m := range textureMeta {
		meta[i] = worldimpl.TextureMeta{Width: m.Width, Height: m.Height, Name: m.Name, Type: m.Type}
	}
	return worldimpl.FaceFlags(meta, tree, face)
}

// worldTextureDimensions fetches source texture dimensions for texel-density
// and UV conversion computations. Lives in world/texture.go.
func worldTextureDimensions(tree *bsp.Tree, texInfo *bsp.Texinfo) (float32, float32) {
	return worldimpl.TextureDimensions(tree, texInfo)
}

func worldFaceCenter(vertices []WorldVertex) types.Vec3 {
	if len(vertices) == 0 {
		return types.Vec3{}
	}
	var center types.Vec3
	for _, vertex := range vertices {
		center.X += vertex.Position.X
		center.Y += vertex.Position.Y
		center.Z += vertex.Position.Z
	}
	scale := 1 / float32(len(vertices))
	center.X *= scale
	center.Y *= scale
	center.Z *= scale
	return center
}

func worldLiquidAlphaSettingsForGeometry(geom *WorldGeometry) worldLiquidAlphaSettings {
	if geom == nil {
		return worldLiquidAlphaSettingsFromCvars(worldLiquidAlphaOverrides{}, nil)
	}
	cvarWater := worldimpl.ReadAlphaCvar(CvarRWaterAlpha, 1)
	overrides := worldLiquidAlphaOverridesFromWorld(geom.LiquidAlphaOverrides)
	settings := resolveWorldLiquidAlphaSettings(
		cvarWater,
		worldimpl.ReadAlphaCvar(CvarRLavaAlpha, 0),
		worldimpl.ReadAlphaCvar(CvarRSlimeAlpha, 0),
		worldimpl.ReadAlphaCvar(CvarRTeleAlpha, 0),
		overrides,
		nil,
	)
	if !geom.TransparentWaterSafe && !overrides.hasWater && cvarWater >= 1 {
		settings.water = 1
		settings.lava = 1
		settings.slime = 1
		settings.tele = 1
	}
	return settings
}

func assignFaceLightmap(vertices []WorldVertex, rawCoords [][2]float64, face *bsp.TreeFace, tree *bsp.Tree, allocator *surfacepkg.LightmapAllocator, pages *[]WorldLightmapPage) (*faceLightmapSurface, error) {
	if face == nil || tree == nil || allocator == nil || len(vertices) == 0 || len(rawCoords) != len(vertices) {
		return nil, nil
	}
	if face.LightOfs < 0 || len(tree.Lighting) == 0 {
		slog.Debug("assignFaceLightmap: skipping face without lightmap data",
			"light_ofs", face.LightOfs,
			"lighting_len", len(tree.Lighting),
			"styles", face.Styles,
		)
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
		// Clamp lightmap coordinates to the interior of the allocated block
		// to prevent linear filtering from bleeding into adjacent blocks.
		// The block spans [x, x+smax] x [y, y+tmax] in page pixels.
		// Inset by 0.5 pixels on each side so the sample never reaches
		// the block edge.
		blockMinS := float32(x) + 0.5
		blockMaxS := float32(x+smax) - 0.5
		blockMinT := float32(y) + 0.5
		blockMaxT := float32(y+tmax) - 0.5
		if lightS < blockMinS {
			lightS = blockMinS
		}
		if lightS > blockMaxS {
			lightS = blockMaxS
		}
		if lightT < blockMinT {
			lightT = blockMinT
		}
		if lightT > blockMaxT {
			lightT = blockMaxT
		}
		vertices[i].LightmapCoord = [2]float32{
			lightS / float32(worldLightmapPageSize),
			lightT / float32(worldLightmapPageSize),
		}
		vertices[i].LightmapLayer = float32(texNum)
	}

	return &faceLightmapSurface{pageIndex: texNum}, nil
}

func worldTexCoordDouble(position types.Vec3, vec [4]float32) float64 {
	return worldimpl.TexCoordDouble(position, vec)
}
