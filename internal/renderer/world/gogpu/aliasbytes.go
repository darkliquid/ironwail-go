// aliasbytes.go implements the pure alias-model vertex/uniform packing
// helpers extracted from renderer_gogpu_world_alias.go. They need no
// *Renderer state, so they live in the world/gogpu subpackage; the renderer
// root keeps thin delegators.
package gogpu

import (
	"encoding/binary"
	"math"

	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

// Alias pipeline byte-layout constants (mirrored from the renderer root).
const (
	// AliasSceneUniformBufferSize is the byte size of one alias scene uniform.
	AliasSceneUniformBufferSize = 96
	// AliasVertexStride is the byte stride of one alias vertex. Must match
	// WorldVertex and every pipeline's ArrayStride — see docs/VERTEX_LAYOUT.md.
	AliasVertexStride = 48
	// WorldUniformAlign is the uniform-buffer alignment (256).
	WorldUniformAlign = 256
)

// PutFloat32s writes little-endian float32 values into dst.
func PutFloat32s(dst []byte, values []float32) {
	for i, value := range values {
		binary.LittleEndian.PutUint32(dst[i*4:(i+1)*4], math.Float32bits(value))
	}
}

// AppendAliasSceneUniformBytes appends one alias scene uniform block to dst,
// growing/positioning to targetOffset with worldUniformAlign alignment.
func AppendAliasSceneUniformBytes(dst []byte, targetOffset uint32, vp types.Mat4, cameraOrigin [3]float32, alpha float32, fogColor types.Vec3, fogDensity float32) []byte {
	requiredLen := int(targetOffset) + int(WorldUniformAlign)
	if cap(dst) < requiredLen {
		newCap := requiredLen * 2
		buf := make([]byte, requiredLen, newCap)
		copy(buf, dst)
		dst = buf
	} else if len(dst) < requiredLen {
		dst = dst[:requiredLen]
	}
	data := dst[targetOffset : targetOffset+AliasSceneUniformBufferSize]
	matrixBytes := types.Mat4ToBytes(vp)
	copy(data[:64], matrixBytes[:])
	PutFloat32s(data[64:76], cameraOrigin[:])
	binary.LittleEndian.PutUint32(data[76:80], math.Float32bits(worldimpl.FogUniformDensity(fogDensity)))
	PutFloat32s(data[80:92], fogColor.Slice())
	binary.LittleEndian.PutUint32(data[92:96], math.Float32bits(alpha))
	return dst
}

// AliasVertexBytes packs alias-model vertices into a fresh flat byte array.
func AliasVertexBytes(vertices []worldimpl.WorldVertex) []byte {
	return AppendAliasVertexBytes(nil, vertices)
}

// AliasVertexBytesInto packs alias-model vertices into dst, reusing the
// caller's buffer capacity when possible.
func AliasVertexBytesInto(dst []byte, vertices []worldimpl.WorldVertex) []byte {
	return AppendAliasVertexBytes(dst[:0], vertices)
}

// AppendAliasVertexBytes appends alias-model vertices to dst in the shared
// 48-byte vertex layout — see docs/VERTEX_LAYOUT.md.
func AppendAliasVertexBytes(dst []byte, vertices []worldimpl.WorldVertex) []byte {
	required := len(vertices) * AliasVertexStride
	start := len(dst)
	total := start + required
	if cap(dst) < total {
		newCap := total * 2
		buf := make([]byte, total, newCap)
		copy(buf, dst)
		dst = buf
	} else {
		dst = dst[:total]
	}
	data := dst[start:]
	for i, v := range vertices {
		offset := i * AliasVertexStride
		PutFloat32s(data[offset:offset+12], []float32{v.Position.X, v.Position.Y, v.Position.Z})
		PutFloat32s(data[offset+12:offset+20], v.TexCoord[:])
		PutFloat32s(data[offset+20:offset+28], v.LightmapCoord[:])
		PutFloat32s(data[offset+28:offset+40], []float32{v.Normal.X, v.Normal.Y, v.Normal.Z})
		PutFloat32s(data[offset+40:offset+44], []float32{v.LightmapLayer})
		binary.LittleEndian.PutUint32(data[offset+44:offset+48], v.MaterialID)
	}
	return dst
}
