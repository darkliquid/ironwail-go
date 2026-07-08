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
		putFloat32Slice(data[offset:offset+12], v.Position[:])
		putFloat32Slice(data[offset+12:offset+20], v.TexCoord[:])
		putFloat32Slice(data[offset+20:offset+28], v.LightmapCoord[:])
		putFloat32Slice(data[offset+28:offset+40], v.Normal[:])
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
