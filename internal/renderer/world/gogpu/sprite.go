//go:build gogpu && !cgo
// +build gogpu,!cgo

package gogpu

import (
	"encoding/binary"
	"math"

	"github.com/ironwail/ironwail-go/internal/model"
	worldimpl "github.com/ironwail/ironwail-go/internal/renderer/world"
	"github.com/ironwail/ironwail-go/pkg/types"
)

const SpriteUniformBufferSize = 96

// SpriteQuadVertex is the GoGPU-local DTO for expanded sprite quad vertices.
type SpriteQuadVertex struct {
	Position [3]float32
	TexCoord [2]float32
}

// SpriteDrawParams captures root-owned sprite entity state needed to plan a GoGPU draw.
type SpriteDrawParams struct {
	ModelID    string
	SpriteData *model.MSprite
	Frame      int
	Origin     [3]float32
	Angles     [3]float32
	Alpha      float32
	Scale      float32
}

// ResolvedSpriteModel carries a caller-owned sprite handle plus the frame count needed for draw planning.
type ResolvedSpriteModel[Sprite any] struct {
	Handle     Sprite
	FrameCount int
}

// SpriteDraw describes a prepared GoGPU sprite draw for a caller-owned sprite handle.
type SpriteDraw[Sprite any] struct {
	Sprite Sprite
	Frame  int
	Origin [3]float32
	Angles [3]float32
	Alpha  float32
	Scale  float32
}

// SpriteUniformBytes packs the GoGPU sprite draw uniform layout.
func SpriteUniformBytes(vp types.Mat4, cameraOrigin [3]float32, alpha float32, fogColor [3]float32, fogDensity float32) []byte {
	data := make([]byte, SpriteUniformBufferSize)
	matrixBytes := types.Mat4ToBytes(vp)
	copy(data[:64], matrixBytes[:])
	putFloat32Slice(data[64:76], cameraOrigin[:])
	binary.LittleEndian.PutUint32(data[76:80], math.Float32bits(worldimpl.FogUniformDensity(fogDensity)))
	putFloat32Slice(data[80:92], fogColor[:])
	binary.LittleEndian.PutUint32(data[92:96], math.Float32bits(alpha))
	return data
}

// BuildSpriteDraw applies GoGPU sprite draw planning while leaving sprite cache/upload ownership to the caller.
func BuildSpriteDraw[Sprite any](params SpriteDrawParams, resolve func(modelID string, spriteData *model.MSprite) (ResolvedSpriteModel[Sprite], bool)) (SpriteDraw[Sprite], bool) {
	if params.ModelID == "" || params.SpriteData == nil || resolve == nil {
		return SpriteDraw[Sprite]{}, false
	}
	resolved, ok := resolve(params.ModelID, params.SpriteData)
	if !ok || resolved.FrameCount <= 0 {
		return SpriteDraw[Sprite]{}, false
	}
	frame := params.Frame
	if frame < 0 || frame >= resolved.FrameCount {
		frame = 0
	}
	alpha := clamp01(params.Alpha)
	if alpha <= 0 {
		return SpriteDraw[Sprite]{}, false
	}
	return SpriteDraw[Sprite]{
		Sprite: resolved.Handle,
		Frame:  frame,
		Origin: params.Origin,
		Angles: params.Angles,
		Alpha:  alpha,
		Scale:  params.Scale,
	}, true
}

// ProjectSpriteQuadVerticesToWorldVertices projects caller-owned sprite quad DTOs into shared world vertices.
func ProjectSpriteQuadVerticesToWorldVertices[Vertex any](vertices []Vertex, project func(Vertex) SpriteQuadVertex) []worldimpl.WorldVertex {
	out := make([]worldimpl.WorldVertex, len(vertices))
	for i, vertex := range vertices {
		projected := project(vertex)
		out[i] = worldimpl.WorldVertex{
			Position:      projected.Position,
			TexCoord:      projected.TexCoord,
			LightmapCoord: [2]float32{},
			Normal:        [3]float32{0, 0, 1},
		}
	}
	return out
}

// SpriteQuadVerticesToWorldVertices converts expanded sprite triangles to the shared world vertex layout.
func SpriteQuadVerticesToWorldVertices(vertices []SpriteQuadVertex) []worldimpl.WorldVertex {
	return ProjectSpriteQuadVerticesToWorldVertices(vertices, func(vertex SpriteQuadVertex) SpriteQuadVertex {
		return vertex
	})
}
