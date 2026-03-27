//go:build gogpu && !cgo
// +build gogpu,!cgo

package gogpu

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	worldimpl "github.com/ironwail/ironwail-go/internal/renderer/world"
)

const worldVertexStrideBytes = 11 * 4

// VertexBytes packs shared world vertices into the GoGPU brush vertex layout.
func VertexBytes(vertices []worldimpl.WorldVertex) []byte {
	data := make([]byte, len(vertices)*worldVertexStrideBytes)
	for i, v := range vertices {
		offset := i * worldVertexStrideBytes
		putFloat32Slice(data[offset:offset+12], v.Position[:])
		putFloat32Slice(data[offset+12:offset+20], v.TexCoord[:])
		putFloat32Slice(data[offset+20:offset+28], v.LightmapCoord[:])
		putFloat32Slice(data[offset+28:offset+40], v.Normal[:])
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
