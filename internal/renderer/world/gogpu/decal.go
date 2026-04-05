package gogpu

import (
	"encoding/binary"
	"math"

	"github.com/darkliquid/ironwail-go/pkg/types"
)

const DecalUniformBufferSize = 80

// DecalDrawParams captures the root-owned decal geometry/color policy inputs needed for GoGPU draw prep.
type DecalDrawParams struct {
	Corners [4][3]float32
	Color   [4]float32
	Variant int
}

// DecalMarkParams captures the root-owned decal mark geometry needed immediately before quad construction.
type DecalMarkParams struct {
	Origin   [3]float32
	Normal   [3]float32
	Size     float32
	Rotation float32
	Variant  int
}

// PreparedDecalDraw is the GoGPU-ready packed decal payload for one draw call.
type PreparedDecalDraw struct {
	VertexCount uint32
	VertexBytes []byte
}

// DecalPreparedMark captures the caller-owned mark inputs that remain after root policy decisions.
type DecalPreparedMark struct {
	Params DecalMarkParams
	Color  [4]float32
}

// DecalVertex is the GoGPU-local packed decal vertex DTO.
type DecalVertex struct {
	Position [3]float32
	TexCoord [2]float32
	Color    [4]float32
}

// DecalUniformBytes packs the GoGPU decal uniform layout.
func DecalUniformBytes(vp types.Mat4, alpha float32) []byte {
	data := make([]byte, DecalUniformBufferSize)
	matrixBytes := types.Mat4ToBytes(vp)
	copy(data[:64], matrixBytes[:])
	binary.LittleEndian.PutUint32(data[64:68], math.Float32bits(alpha))
	return data
}

// BuildDecalVertices expands a validated decal quad into two triangles in GoGPU vertex form.
func BuildDecalVertices(corners [4][3]float32, color [4]float32, variant int) []DecalVertex {
	baseX := float32(variant%2) * 0.5
	baseY := float32(variant/2) * 0.5
	uv := [4][2]float32{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	indices := [6]int{0, 1, 2, 0, 2, 3}

	out := make([]DecalVertex, 0, len(indices))
	for _, idx := range indices {
		coord := uv[idx]
		out = append(out, DecalVertex{
			Position: corners[idx],
			TexCoord: [2]float32{baseX + coord[0]*0.5, baseY + coord[1]*0.5},
			Color:    color,
		})
	}
	return out
}

// DecalVertexBytes packs GoGPU decal vertices into the HAL upload layout.
func DecalVertexBytes(vertices []DecalVertex) []byte {
	data := make([]byte, len(vertices)*36)
	for i, v := range vertices {
		offset := i * 36
		putFloat32Slice(data[offset:offset+12], v.Position[:])
		putFloat32Slice(data[offset+12:offset+20], v.TexCoord[:])
		putFloat32Slice(data[offset+20:offset+36], v.Color[:])
	}
	return data
}

// PrepareDecalDraw converts caller-owned decal geometry/color inputs into a packed GoGPU draw payload.
func PrepareDecalDraw(params DecalDrawParams) PreparedDecalDraw {
	vertices := BuildDecalVertices(params.Corners, params.Color, params.Variant)
	if len(vertices) == 0 {
		return PreparedDecalDraw{}
	}
	return PreparedDecalDraw{
		VertexCount: uint32(len(vertices)),
		VertexBytes: DecalVertexBytes(vertices),
	}
}

// PrepareDecalDrawFromMark delegates quad construction to the caller while keeping packed draw shaping in the GoGPU subpackage.
func PrepareDecalDrawFromMark(params DecalMarkParams, color [4]float32, buildQuad func(DecalMarkParams) ([4][3]float32, bool)) PreparedDecalDraw {
	if buildQuad == nil {
		return PreparedDecalDraw{}
	}
	corners, ok := buildQuad(params)
	if !ok {
		return PreparedDecalDraw{}
	}
	return PrepareDecalDraw(DecalDrawParams{
		Corners: corners,
		Color:   color,
		Variant: params.Variant,
	})
}

// PrepareDecalDraws batches mark-local GoGPU draw preparation while leaving policy, HAL, and quad building adapters to the caller.
func PrepareDecalDraws(marks []DecalPreparedMark, buildQuad func(DecalMarkParams) ([4][3]float32, bool)) []PreparedDecalDraw {
	if len(marks) == 0 || buildQuad == nil {
		return nil
	}
	draws := make([]PreparedDecalDraw, 0, len(marks))
	for _, mark := range marks {
		draw := PrepareDecalDrawFromMark(mark.Params, mark.Color, buildQuad)
		if draw.VertexCount == 0 {
			continue
		}
		draws = append(draws, draw)
	}
	return draws
}

// PrepareDecalDrawsWithAdapter batches caller-owned mark adaptation and packed GoGPU draw preparation while keeping policy and quad building in the caller.
func PrepareDecalDrawsWithAdapter[Mark any](marks []Mark, adapt func(Mark) (DecalPreparedMark, bool), buildQuad func(DecalMarkParams) ([4][3]float32, bool)) []PreparedDecalDraw {
	if len(marks) == 0 || adapt == nil || buildQuad == nil {
		return nil
	}
	draws := make([]PreparedDecalDraw, 0, len(marks))
	for _, mark := range marks {
		preparedMark, ok := adapt(mark)
		if !ok {
			continue
		}
		draw := PrepareDecalDrawFromMark(preparedMark.Params, preparedMark.Color, buildQuad)
		if draw.VertexCount == 0 {
			continue
		}
		draws = append(draws, draw)
	}
	return draws
}
