package gogpu

import (
	"encoding/binary"
	"fmt"
	"math"

	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// worldVertexStrideBytes is the number of bytes per vertex in the flat byte
// buffer uploaded to the GPU. Must match WorldVertex (world/types.go) and
// every pipeline's ArrayStride. See docs/VERTEX_LAYOUT.md.
const worldVertexStrideBytes = 12 * 4 // 48 bytes

// VertexBytes packs shared world vertices into the GoGPU brush vertex layout.
// This is one of four vertex-packing functions that must all agree on the byte
// layout — see docs/VERTEX_LAYOUT.md.
//
// If a field is missing or the stride is wrong, the GPU reads vertex data at
// incorrect offsets. The classic symptom is "shadow geometry" — solid-coloured
// phantom triangles appearing around moving brush entities (doors, platforms)
// while the brush itself is invisible.
func VertexBytes(vertices []worldimpl.WorldVertex) []byte {
	data := make([]byte, len(vertices)*worldVertexStrideBytes)
	for i, v := range vertices {
		offset := i * worldVertexStrideBytes
		putFloat32Slice(data[offset:offset+12], []float32{v.Position.X, v.Position.Y, v.Position.Z})
		putFloat32Slice(data[offset+12:offset+20], v.TexCoord[:])
		putFloat32Slice(data[offset+20:offset+28], v.LightmapCoord[:])
		putFloat32Slice(data[offset+28:offset+40], []float32{v.Normal.X, v.Normal.Y, v.Normal.Z})
		putFloat32Slice(data[offset+40:offset+44], []float32{v.LightmapLayer})
		binary.LittleEndian.PutUint32(data[offset+44:offset+48], v.MaterialID)
	}
	return data
}

// IndexBytes packs brush indices into a little-endian index buffer payload.
func IndexBytes(indices []uint32) []byte {
	data := make([]byte, len(indices)*4)
	for i, idx := range indices {
		binary.LittleEndian.PutUint32(data[i*4:], idx)
	}
	return data
}

// AppendVertexBytes appends brush-entity vertices onto dst in the shared
// 48-byte vertex layout, growing dst as needed. This is one of four
// vertex-packing functions that must all agree on the byte layout — see
// docs/VERTEX_LAYOUT.md.
func AppendVertexBytes(dst []byte, vertices []worldimpl.WorldVertex) []byte {
	if len(vertices) == 0 {
		return dst
	}
	start := len(dst)
	dst = append(dst, make([]byte, len(vertices)*worldVertexStrideBytes)...)
	write := start
	for _, vertex := range vertices {
		putFloat32Slice(dst[write:write+12], []float32{vertex.Position.X, vertex.Position.Y, vertex.Position.Z})
		putFloat32Slice(dst[write+12:write+20], vertex.TexCoord[:])
		putFloat32Slice(dst[write+20:write+28], vertex.LightmapCoord[:])
		putFloat32Slice(dst[write+28:write+40], []float32{vertex.Normal.X, vertex.Normal.Y, vertex.Normal.Z})
		putFloat32Slice(dst[write+40:write+44], []float32{vertex.LightmapLayer})
		binary.LittleEndian.PutUint32(dst[write+44:write+48], vertex.MaterialID)
		write += worldVertexStrideBytes
	}
	return dst
}

// AppendIndexBytes appends brush indices onto dst as little-endian uint32s.
func AppendIndexBytes(dst []byte, indices []uint32) []byte {
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

// CreateBrushBuffer allocates a GoGPU buffer suitable for queued brush uploads.
func CreateBrushBuffer(device *wgpu.Device, label string, usage gputypes.BufferUsage, data []byte) (*wgpu.Buffer, error) {
	if device == nil || len(data) == 0 {
		return nil, fmt.Errorf("invalid brush buffer upload")
	}
	buffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            label,
		Size:             uint64(len(data)),
		Usage:            usage | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return nil, err
	}
	return buffer, nil
}

func putFloat32Slice(dst []byte, values []float32) {
	for i, value := range values {
		binary.LittleEndian.PutUint32(dst[i*4:(i+1)*4], math.Float32bits(value))
	}
}
