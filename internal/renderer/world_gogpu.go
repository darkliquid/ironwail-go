package renderer

import (
	"encoding/binary"
	"math"
	"sync"

	"github.com/darkliquid/ironwail-go/internal/model"
	worldgogpu "github.com/darkliquid/ironwail-go/internal/renderer/world/gogpu"
)

type gogpuOpaqueBrushEntityDraw struct {
	hasLitWater bool
	alpha       float32
	frame       int
	vertices    []WorldVertex
	indices     []uint32
	faces       []WorldFace
	centers     [][3]float32
	lightmaps   []*gpuWorldTexture
}

type gogpuClassifiedBrushEntityDraw struct {
	alpha            float32
	frame            int
	vertices         []WorldVertex
	opaqueIndices    []uint32
	opaqueFaces      []WorldFace
	opaqueCenters    [][3]float32
	alphaTestIndices []uint32
	alphaTestFaces   []WorldFace
	alphaTestCenters [][3]float32
	lightmaps        []*gpuWorldTexture
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

const goGPUWorldVertexStrideBytes = 11 * 4

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

func buildGoGPUClassifiedBrushEntityDraw(entity BrushEntity, geom *WorldGeometry) *gogpuClassifiedBrushEntityDraw {
	draw := worldgogpu.BuildClassifiedBrushEntityDraw(gogpuBrushEntityParams(entity), geom, classifyGoGPUBrushEntityFace)
	if draw == nil {
		return nil
	}
	return &gogpuClassifiedBrushEntityDraw{
		alpha:            draw.Alpha,
		frame:            draw.Frame,
		vertices:         draw.Vertices,
		opaqueIndices:    draw.OpaqueIndices,
		opaqueFaces:      draw.OpaqueFaces,
		opaqueCenters:    draw.OpaqueCenters,
		alphaTestIndices: draw.AlphaTestIndices,
		alphaTestFaces:   draw.AlphaTestFaces,
		alphaTestCenters: draw.AlphaTestCenters,
	}
}

func buildGoGPUSkyBrushEntityDraw(entity BrushEntity, geom *WorldGeometry) *gogpuOpaqueBrushEntityDraw {
	return buildGoGPUBrushEntityDraw(entity, geom, shouldDrawGoGPUSkyBrushFace)
}

func buildGoGPUOpaqueLiquidBrushEntityDraw(entity BrushEntity, geom *WorldGeometry, liquidAlpha worldLiquidAlphaSettings) *gogpuOpaqueBrushEntityDraw {
	return buildGoGPUBrushEntityDraw(entity, geom, func(face WorldFace, entityAlpha float32) bool {
		return shouldDrawGoGPUOpaqueLiquidBrushFace(face, entityAlpha, liquidAlpha)
	})
}

type gogpuTranslucentLiquidBrushEntityDraw struct {
	frame     int
	vertices  []WorldVertex
	indices   []uint32
	faces     []gogpuTranslucentLiquidFaceDraw
	lightmaps []*gpuWorldTexture
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
	frame            int
	vertices         []WorldVertex
	indices          []uint32
	alphaTestFaces   []WorldFace
	alphaTestCenters [][3]float32
	translucentFaces []gogpuTranslucentLiquidFaceDraw
	liquidFaces      []gogpuTranslucentLiquidFaceDraw
	lightmaps        []*gpuWorldTexture
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
