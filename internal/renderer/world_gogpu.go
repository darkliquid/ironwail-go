package renderer

import (
	"encoding/binary"
	"math"
	"sync"

	"github.com/darkliquid/ironwail-go/internal/model"
	surfacepkg "github.com/darkliquid/ironwail-go/internal/renderer/surface"
	worldgogpu "github.com/darkliquid/ironwail-go/internal/renderer/world/gogpu"
	"github.com/gogpu/wgpu"
)

type gogpuOpaqueBrushEntityDraw struct {
	hasLitWater   bool
	alpha         float32
	frame         int
	vertices      []WorldVertex
	indices       []uint32
	faces         []WorldFace
	centers       [][3]float32
	lightmapArray *gpuWorldTexture
	// Optional per-draw texture overrides used for standalone-BSP brush
	// entities (e.g. b_rock0.bsp). Nil means use the world texture tables.
	textures           *gpuWorldTexture
	fullbrightTextures *gpuWorldTexture
	textureAnimations  []*surfacepkg.SurfaceTexture
	uniformBindGroup   *wgpu.BindGroup
}

type gogpuClassifiedBrushEntityDraw struct {
	alpha              float32
	frame              int
	vertices           []WorldVertex
	opaqueIndices      []uint32
	opaqueFaces        []WorldFace
	opaqueCenters      [][3]float32
	alphaTestIndices   []uint32
	alphaTestFaces     []WorldFace
	alphaTestCenters   [][3]float32
	lightmapArray      *gpuWorldTexture
	textures           *gpuWorldTexture
	fullbrightTextures *gpuWorldTexture
	textureAnimations  []*surfacepkg.SurfaceTexture
	uniformBindGroup   *wgpu.BindGroup
}

type gogpuPreparedClassifiedBrushDraw struct {
	drawIndex            int
	vertexOffset         uint64
	opaqueIndexOffset    uint64
	alphaTestIndexOffset uint64
}

type gogpuPreparedOpaqueBrushDraw struct {
	drawIndex    int
	hasLitWater  bool
	vertexOffset uint64
	indexOffset  uint64
}

type gogpuBrushPrepScratch struct {
	classifiedBuild    []worldgogpu.ClassifiedBrushEntityDraw
	classifiedDraws    []gogpuClassifiedBrushEntityDraw
	classifiedPrepared []gogpuPreparedClassifiedBrushDraw
	opaqueBuild        []worldgogpu.OpaqueBrushEntityDraw
	opaqueDraws        []gogpuOpaqueBrushEntityDraw
	opaquePrepared     []gogpuPreparedOpaqueBrushDraw
	vertexData         []byte
	indexData          []byte
}

var gogpuBrushPrepScratchPool = sync.Pool{
	New: func() any {
		return &gogpuBrushPrepScratch{}
	},
}

// goGPUWorldVertexStrideBytes is the number of bytes between the start of one
// vertex and the start of the next in the flat byte buffer uploaded to the GPU.
//
// This MUST match:
//   - unsafe.Sizeof(WorldVertex{}) in world/types.go (currently 48)
//   - ArrayStride in every pipeline's VertexBufferLayout (currently 48)
//
// If this is too small, the GPU reads past the end of each vertex into the
// next vertex's data, causing materialID and lightmapLayer to contain garbage
// — which produces scrambled textures and "shadow geometry" around moving
// brushes (doors, platforms, triggers). See docs/VERTEX_LAYOUT.md.
const goGPUWorldVertexStrideBytes = 12 * 4 // 12 float32-sized slots = 48 bytes

func shouldDrawGoGPUOpaqueBrushFace(face WorldFace, entityAlpha float32) bool {
	return isFullyOpaqueAlpha(clamp01(entityAlpha)) && shouldDrawGoGPUOpaqueWorldFace(face)
}

func shouldDrawGoGPUAlphaTestBrushFace(face WorldFace, entityAlpha float32) bool {
	return isFullyOpaqueAlpha(clamp01(entityAlpha)) && shouldDrawGoGPUAlphaTestWorldFace(face)
}

func shouldDrawGoGPUSkyBrushFace(face WorldFace, entityAlpha float32) bool {
	return clamp01(entityAlpha) > 0 && shouldDrawGoGPUSkyWorldFace(face)
}

func shouldDrawGoGPUOpaqueLiquidBrushFace(face WorldFace, entityAlpha float32, liquidAlpha worldLiquidAlphaSettings) bool {
	return isFullyOpaqueAlpha(clamp01(entityAlpha)) && shouldDrawGoGPUOpaqueLiquidFace(face, liquidAlpha)
}

func shouldDrawGoGPUTranslucentLiquidBrushFace(face WorldFace, entityAlpha float32, liquidAlpha worldLiquidAlphaSettings) bool {
	return isFullyOpaqueAlpha(clamp01(entityAlpha)) && shouldDrawGoGPUTranslucentLiquidFace(face, liquidAlpha)
}

func shouldDrawGoGPUTranslucentBrushEntityFace(face WorldFace, entityAlpha float32, liquidAlpha worldLiquidAlphaSettings) bool {
	if !(clamp01(entityAlpha) > 0 && clamp01(entityAlpha) < 1) {
		return false
	}
	if face.Flags&model.SurfDrawSky != 0 {
		return false
	}
	pass := worldFacePass(face.Flags, worldFaceAlpha(face.Flags, liquidAlpha)*clamp01(entityAlpha))
	return pass == worldPassAlphaTest || pass == worldPassTranslucent
}

func classifyGoGPUBrushEntityFace(face WorldFace, entityAlpha float32) worldgogpu.BrushEntityFaceClass {
	switch {
	case shouldDrawGoGPUOpaqueBrushFace(face, entityAlpha):
		return worldgogpu.BrushEntityFaceClassOpaque
	case shouldDrawGoGPUAlphaTestBrushFace(face, entityAlpha):
		return worldgogpu.BrushEntityFaceClassAlphaTest
	default:
		return worldgogpu.BrushEntityFaceClassSkip
	}
}

// appendGoGPUWorldVertexBytes packs brush-entity vertices into a flat byte
// array for GPU upload. This is one of four vertex-packing functions that must
// all agree on the byte layout — see docs/VERTEX_LAYOUT.md.
//
// Every field must be written at the correct offset within each 48-byte vertex.
// Missing fields cause the GPU to read uninitialized (zero) bytes, which makes
// materialID default to 0 (wrong texture) and lightmapLayer default to 0
// (wrong lightmap page). A stride that's too small causes the GPU to read into
// the next vertex's data, producing the classic "shadow geometry" symptom
// around moving brushes.
func appendGoGPUWorldVertexBytes(dst []byte, vertices []WorldVertex) []byte {
	if len(vertices) == 0 {
		return dst
	}
	start := len(dst)
	dst = append(dst, make([]byte, len(vertices)*goGPUWorldVertexStrideBytes)...)
	write := start
	for _, vertex := range vertices {
		putGoGPUFloat32Slice(dst[write:write+12], vertex.Position[:])
		putGoGPUFloat32Slice(dst[write+12:write+20], vertex.TexCoord[:])
		putGoGPUFloat32Slice(dst[write+20:write+28], vertex.LightmapCoord[:])
		putGoGPUFloat32Slice(dst[write+28:write+40], vertex.Normal[:])
		putGoGPUFloat32Slice(dst[write+40:write+44], []float32{vertex.LightmapLayer})
		binary.LittleEndian.PutUint32(dst[write+44:write+48], vertex.MaterialID)
		write += goGPUWorldVertexStrideBytes
	}
	return dst
}

func appendGoGPUWorldIndexBytes(dst []byte, indices []uint32) []byte {
	if len(indices) == 0 {
		return dst
	}
	start := len(dst)
	dst = append(dst, make([]byte, len(indices)*4)...)
	write := start
	for _, index := range indices {
		binary.LittleEndian.PutUint32(dst[write:write+4], index)
		write += 4
	}
	return dst
}

func putGoGPUFloat32Slice(dst []byte, values []float32) {
	for i, value := range values {
		binary.LittleEndian.PutUint32(dst[i*4:(i+1)*4], math.Float32bits(value))
	}
}

func buildGoGPUBrushEntityDraw(entity BrushEntity, geom *WorldGeometry, includeFace func(WorldFace, float32) bool) *gogpuOpaqueBrushEntityDraw {
	draw := worldgogpu.BuildBrushEntityDraw(gogpuBrushEntityParams(entity), geom, includeFace)
	if draw == nil {
		return nil
	}
	return &gogpuOpaqueBrushEntityDraw{
		hasLitWater: draw.HasLitWater,
		alpha:       draw.Alpha,
		frame:       draw.Frame,
		vertices:    draw.Vertices,
		indices:     draw.Indices,
		faces:       draw.Faces,
		centers:     draw.Centers,
	}
}

func gogpuBrushEntityParams(entity BrushEntity) worldgogpu.BrushEntityParams {
	return worldgogpu.BrushEntityParams{
		Alpha:  entity.Alpha,
		Frame:  entity.Frame,
		Origin: entity.Origin,
		Angles: entity.Angles,
		Scale:  entity.Scale,
	}
}

func buildGoGPUSkyBrushEntityDraw(entity BrushEntity, geom *WorldGeometry) *gogpuOpaqueBrushEntityDraw {
	return buildGoGPUBrushEntityDraw(entity, geom, shouldDrawGoGPUSkyBrushFace)
}

type gogpuTranslucentLiquidBrushEntityDraw struct {
	frame              int
	vertices           []WorldVertex
	indices            []uint32
	faces              []gogpuTranslucentLiquidFaceDraw
	lightmapArray      *gpuWorldTexture
	textures           *gpuWorldTexture
	fullbrightTextures *gpuWorldTexture
	textureAnimations  []*surfacepkg.SurfaceTexture
	uniformBindGroup   *wgpu.BindGroup
}

func convertGoGPUTranslucentFaceDraws(src []worldgogpu.TranslucentFaceDraw) []gogpuTranslucentLiquidFaceDraw {
	dst := make([]gogpuTranslucentLiquidFaceDraw, 0, len(src))
	for _, face := range src {
		dst = append(dst, gogpuTranslucentLiquidFaceDraw{
			face:       face.Face,
			alpha:      face.Alpha,
			center:     face.Center,
			distanceSq: face.DistanceSq,
		})
	}
	return dst
}

func buildGoGPUTranslucentLiquidBrushEntityDraw(entity BrushEntity, geom *WorldGeometry, liquidAlpha worldLiquidAlphaSettings, camera CameraState) *gogpuTranslucentLiquidBrushEntityDraw {
	draw := worldgogpu.BuildTranslucentLiquidBrushEntityDraw(gogpuBrushEntityParams(entity), geom, func(face WorldFace, entityAlpha float32) (float32, bool) {
		if !shouldDrawGoGPUTranslucentLiquidBrushFace(face, entityAlpha, liquidAlpha) {
			return 0, false
		}
		return worldFaceAlpha(face.Flags, liquidAlpha), true
	}, func(center [3]float32) float32 {
		return worldFaceDistanceSq(center, camera)
	})
	if draw == nil {
		return nil
	}
	return &gogpuTranslucentLiquidBrushEntityDraw{
		frame:    draw.Frame,
		vertices: draw.Vertices,
		indices:  draw.Indices,
		faces:    convertGoGPUTranslucentFaceDraws(draw.Faces),
	}
}

type gogpuTranslucentBrushEntityDraw struct {
	frame              int
	vertices           []WorldVertex
	indices            []uint32
	alphaTestFaces     []WorldFace
	alphaTestCenters   [][3]float32
	translucentFaces   []gogpuTranslucentLiquidFaceDraw
	liquidFaces        []gogpuTranslucentLiquidFaceDraw
	lightmapArray      *gpuWorldTexture
	textures           *gpuWorldTexture
	fullbrightTextures *gpuWorldTexture
	textureAnimations  []*surfacepkg.SurfaceTexture
	uniformBindGroup   *wgpu.BindGroup
}

func buildGoGPUTranslucentBrushEntityDraw(entity BrushEntity, geom *WorldGeometry, liquidAlpha worldLiquidAlphaSettings, camera CameraState) *gogpuTranslucentBrushEntityDraw {
	draw := worldgogpu.BuildTranslucentBrushEntityDraw(gogpuBrushEntityParams(entity), geom, func(face WorldFace, entityAlpha float32) (worldgogpu.TranslucentFacePlan, bool) {
		if !shouldDrawGoGPUTranslucentBrushEntityFace(face, entityAlpha, liquidAlpha) {
			return worldgogpu.TranslucentFacePlan{}, false
		}
		faceAlpha := worldFaceAlpha(face.Flags, liquidAlpha) * entityAlpha
		switch worldFacePass(face.Flags, faceAlpha) {
		case worldPassAlphaTest:
			return worldgogpu.TranslucentFacePlan{
				Pass:  worldgogpu.TranslucentFacePassAlphaTest,
				Alpha: faceAlpha,
			}, true
		case worldPassTranslucent:
			return worldgogpu.TranslucentFacePlan{
				Pass:   worldgogpu.TranslucentFacePassTranslucent,
				Alpha:  faceAlpha,
				Liquid: worldFaceIsLiquid(face.Flags),
			}, true
		default:
			return worldgogpu.TranslucentFacePlan{}, false
		}
	}, func(center [3]float32) float32 {
		return worldFaceDistanceSq(center, camera)
	})
	if draw == nil {
		return nil
	}
	return &gogpuTranslucentBrushEntityDraw{
		frame:            draw.Frame,
		vertices:         draw.Vertices,
		indices:          draw.Indices,
		alphaTestFaces:   draw.AlphaTestFaces,
		alphaTestCenters: draw.AlphaTestCenters,
		translucentFaces: convertGoGPUTranslucentFaceDraws(draw.TranslucentFaces),
		liquidFaces:      convertGoGPUTranslucentFaceDraws(draw.LiquidFaces),
	}
}
