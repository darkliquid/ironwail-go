// This file belongs to the world/gogpu renderer subsystem: pure WebGPU
// device/queue resource creation helpers for the BSP world pipeline.
//
// These are device-only helpers that require no *Renderer state, so they live
// in the world/gogpu subpackage rather than the renderer root. The renderer
// root keeps thin delegators that forward the device/queue handles.

package gogpu

import (
	"encoding/binary"
	"fmt"
	"log/slog"

	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// CreateWorldVertexBuffer uploads vertex data to GPU.
//
// This is one of four vertex-packing functions that must all agree on the byte
// layout — see docs/VERTEX_LAYOUT.md. The stride (48 bytes) must match the
// pipeline's ArrayStride. Missing fields cause wrong textures/lighting; a
// stride that's too small causes the GPU to read into adjacent vertices,
// scrambling all rendering.
func CreateWorldVertexBuffer(device *wgpu.Device, queue *wgpu.Queue, vertices []worldimpl.WorldVertex) (*wgpu.Buffer, error) {
	if len(vertices) == 0 {
		return nil, fmt.Errorf("no vertices to upload")
	}

	// Calculate size. WorldVertex is 48 bytes: 3+2+2+3 floats + 1 float + 1 uint32.
	vertexSize := uint64(len(vertices)) * worldVertexStrideBytes

	slog.Debug("Creating world vertex buffer",
		"vertexCount", len(vertices),
		"sizeBytes", vertexSize)

	// Create GPU buffer
	buffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "World Vertices",
		Size:             vertexSize,
		Usage:            gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return nil, fmt.Errorf("create vertex buffer: %w", err)
	}

	_ = queue.WriteBuffer(buffer, 0, VertexBytes(vertices))

	slog.Debug("World vertex buffer uploaded", "vertices", len(vertices))

	return buffer, nil
}

// CreateWorldIndexBuffer uploads index data to GPU.
func CreateWorldIndexBuffer(device *wgpu.Device, queue *wgpu.Queue, indices []uint32) (*wgpu.Buffer, uint32, error) {
	if len(indices) == 0 {
		return nil, 0, fmt.Errorf("no indices to upload")
	}

	indexSize := uint64(len(indices)) * 4 // uint32 = 4 bytes

	slog.Debug("Creating world index buffer",
		"indexCount", len(indices),
		"sizeBytes", indexSize)

	// Create GPU buffer
	buffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "World Indices",
		Size:             indexSize,
		Usage:            gputypes.BufferUsageIndex | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("create index buffer: %w", err)
	}

	_ = queue.WriteBuffer(buffer, 0, IndexBytes(indices))

	slog.Debug("World index buffer uploaded", "indices", len(indices))

	return buffer, uint32(len(indices)), nil
}

// CreateWorldSolidTexture creates a 1x1 RGBA texture for use as a solid-color
// fallback or placeholder.
func CreateWorldSolidTexture(device *wgpu.Device, queue *wgpu.Queue, label string, pixel [4]byte) (*wgpu.Texture, *wgpu.TextureView, error) {
	if device == nil || queue == nil {
		return nil, nil, fmt.Errorf("invalid device or queue")
	}

	// Create 1x1 RGBA texture descriptor
	textureDesc := &wgpu.TextureDescriptor{
		Label:         label,
		Size:          wgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	}

	// Create the texture
	texture, err := device.CreateTexture(textureDesc)
	if err != nil {
		return nil, nil, fmt.Errorf("create %s: %w", label, err)
	}

	err = queue.WriteTexture(
		&wgpu.ImageCopyTexture{
			Texture:  texture,
			MipLevel: 0,
			Origin:   wgpu.Origin3D{X: 0, Y: 0, Z: 0},
			Aspect:   gputypes.TextureAspectAll,
		},
		pixel[:],
		&wgpu.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  4, // 1 pixel × 4 bytes
			RowsPerImage: 1,
		},
		&wgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
	)
	if err != nil {
		texture.Release()
		return nil, nil, fmt.Errorf("write %s data: %w", label, err)
	}

	// Create texture view
	textureViewDesc := &wgpu.TextureViewDescriptor{
		Label:         label + " View",
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Dimension:     gputypes.TextureViewDimension2D,
		Aspect:        gputypes.TextureAspectAll,
		BaseMipLevel:  0,
		MipLevelCount: 1,
	}

	textureView, err := device.CreateTextureView(texture, textureViewDesc)
	if err != nil {
		texture.Release()
		return nil, nil, fmt.Errorf("create %s view: %w", label, err)
	}

	slog.Debug("World solid texture created", "label", label)
	return texture, textureView, nil
}

// CreateWorldSolidTextureArray creates a 1x1 RGBA texture array with all
// layers filled with the same solid pixel.
func CreateWorldSolidTextureArray(device *wgpu.Device, queue *wgpu.Queue, label string, pixel [4]byte, layers int) (*wgpu.Texture, *wgpu.TextureView, error) {
	if device == nil || queue == nil {
		return nil, nil, fmt.Errorf("invalid device or queue")
	}
	if layers <= 0 {
		layers = 1
	}

	textureDesc := &wgpu.TextureDescriptor{
		Label:         label,
		Size:          wgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: uint32(layers)},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	}

	texture, err := device.CreateTexture(textureDesc)
	if err != nil {
		return nil, nil, fmt.Errorf("create %s: %w", label, err)
	}

	for i := 0; i < layers; i++ {
		err = queue.WriteTexture(
			&wgpu.ImageCopyTexture{
				Texture:  texture,
				MipLevel: 0,
				Origin:   wgpu.Origin3D{X: 0, Y: 0, Z: uint32(i)},
				Aspect:   gputypes.TextureAspectAll,
			},
			pixel[:],
			&wgpu.ImageDataLayout{
				Offset:       0,
				BytesPerRow:  4,
				RowsPerImage: 1,
			},
			&wgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
		)
		if err != nil {
			texture.Release()
			return nil, nil, fmt.Errorf("write %s data layer %d: %w", label, i, err)
		}
	}

	textureViewDesc := &wgpu.TextureViewDescriptor{
		Label:           label + " View",
		Format:          gputypes.TextureFormatRGBA8Unorm,
		Dimension:       gputypes.TextureViewDimension2DArray,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: uint32(layers),
	}

	textureView, err := device.CreateTextureView(texture, textureViewDesc)
	if err != nil {
		texture.Release()
		return nil, nil, fmt.Errorf("create %s view: %w", label, err)
	}

	slog.Debug("World solid texture array created", "label", label, "layers", layers)
	return texture, textureView, nil
}

// CreateWorldWhiteTexture creates a simple 1x1 white texture for fallback.
// Used when actual textures are not yet available for rendering.
func CreateWorldWhiteTexture(device *wgpu.Device, queue *wgpu.Queue) (*wgpu.Texture, *wgpu.TextureView, error) {
	return CreateWorldSolidTexture(device, queue, "World White Texture", [4]byte{255, 255, 255, 255})
}

// CreateWorldTextureSampler creates the shared world texture sampler.
func CreateWorldTextureSampler(device *wgpu.Device) (*wgpu.Sampler, error) {
	return device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:        "World Texture Sampler",
		AddressModeU: gputypes.AddressModeRepeat,
		AddressModeV: gputypes.AddressModeRepeat,
		AddressModeW: gputypes.AddressModeRepeat,
		MagFilter:    gputypes.FilterModeLinear,
		MinFilter:    gputypes.FilterModeLinear,
		MipmapFilter: gputypes.FilterModeLinear,
		Anisotropy:   16,
		LodMinClamp:  0,
		LodMaxClamp:  0,
	})
}

// CreateWorldAtlasSampler creates a sampler for atlas-packed textures.
// ClampToEdge prevents bleeding between atlas sub-rects; the shader handles
// UV wrapping via fract() before remapping into the atlas.
// Linear filtering is used with textureSampleLevel in the shader; the
// half-texel UV inset on atlas bounds prevents inter-texture bleeding.
func CreateWorldAtlasSampler(device *wgpu.Device) (*wgpu.Sampler, error) {
	return device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:        "World Atlas Sampler",
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		AddressModeW: gputypes.AddressModeClampToEdge,
		MagFilter:    gputypes.FilterModeLinear,
		MinFilter:    gputypes.FilterModeLinear,
		MipmapFilter: gputypes.FilterModeLinear,
		Anisotropy:   0,
		LodMinClamp:  0,
		LodMaxClamp:  0,
	})
}

// CreateWorldLightmapSampler creates the sampler used for lightmap textures.
func CreateWorldLightmapSampler(device *wgpu.Device) (*wgpu.Sampler, error) {
	return device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:        "World Lightmap Sampler",
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		AddressModeW: gputypes.AddressModeClampToEdge,
		MagFilter:    gputypes.FilterModeLinear,
		MinFilter:    gputypes.FilterModeLinear,
		MipmapFilter: gputypes.FilterModeLinear,
		Anisotropy:   16,
		LodMinClamp:  0,
		LodMaxClamp:  0,
	})
}

// WorldLightmapFallbackView returns the white view when available so world
// faces without lightmap data sample full-bright and stay visible. The black
// fallback is only appropriate for brush entity faces where missing lightmaps
// should stay dark.
func WorldLightmapFallbackView(blackView, whiteView *wgpu.TextureView) *wgpu.TextureView {
	if whiteView != nil {
		return whiteView
	}
	return blackView
}

// maxTextureUploadChunkSize is the maximum bytes per WriteTexture call.
// The Vulkan backend creates a staging buffer for each WriteTexture call,
// and the driver silently caps staging buffer allocations at 64 MiB
// (67,108,864 bytes). Data exceeding this limit is silently truncated,
// corrupting the texture. We use 60 MiB to stay safely under the cap.
const maxTextureUploadChunkSize = 60 * 1024 * 1024

// WriteTextureChunked uploads a 2D RGBA texture to the GPU, splitting
// the data into vertical chunks when the total size exceeds the GPU's
// staging buffer limit. Each chunk is uploaded via a separate
// WriteTexture call targeting a horizontal slice of the destination
// texture.
func WriteTextureChunked(queue *wgpu.Queue, texture *wgpu.Texture, rgba []byte, width, height int, label string) error {
	bytesPerRow := width * 4
	totalBytes := len(rgba)

	if totalBytes <= maxTextureUploadChunkSize {
		return queue.WriteTexture(
			&wgpu.ImageCopyTexture{
				Texture:  texture,
				MipLevel: 0,
				Aspect:   gputypes.TextureAspectAll,
			},
			rgba,
			&wgpu.ImageDataLayout{
				BytesPerRow:  uint32(bytesPerRow),
				RowsPerImage: uint32(height),
			},
			&wgpu.Extent3D{
				Width:              uint32(width),
				Height:             uint32(height),
				DepthOrArrayLayers: 1,
			},
		)
	}

	rowsPerChunk := maxTextureUploadChunkSize / bytesPerRow
	if rowsPerChunk == 0 {
		rowsPerChunk = 1
	}

	uploadedRows := 0
	for uploadedRows < height {
		rowsRemaining := height - uploadedRows
		chunkRows := rowsRemaining
		if chunkRows > rowsPerChunk {
			chunkRows = rowsPerChunk
		}

		chunkOffset := uploadedRows * bytesPerRow
		chunkData := rgba[chunkOffset : chunkOffset+chunkRows*bytesPerRow]

		if err := queue.WriteTexture(
			&wgpu.ImageCopyTexture{
				Texture:  texture,
				MipLevel: 0,
				Origin: wgpu.Origin3D{
					X: 0,
					Y: uint32(uploadedRows),
					Z: 0,
				},
				Aspect: gputypes.TextureAspectAll,
			},
			chunkData,
			&wgpu.ImageDataLayout{
				BytesPerRow:  uint32(bytesPerRow),
				RowsPerImage: uint32(chunkRows),
			},
			&wgpu.Extent3D{
				Width:              uint32(width),
				Height:             uint32(chunkRows),
				DepthOrArrayLayers: 1,
			},
		); err != nil {
			return fmt.Errorf("write texture chunk (label=%s, offset_row=%d, chunk_rows=%d): %w", label, uploadedRows, chunkRows, err)
		}

		uploadedRows += chunkRows
	}

	return nil
}

// Encode indices into a little-endian uint32 payload (kept for parity with
// callers that inline the loop).
func EncodeIndexBytes(indices []uint32) []byte {
	data := make([]byte, len(indices)*4)
	for i, idx := range indices {
		binary.LittleEndian.PutUint32(data[i*4:], idx)
	}
	return data
}
