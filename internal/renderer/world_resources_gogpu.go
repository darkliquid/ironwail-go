package renderer

import (
	"encoding/binary"
	"fmt"
	stdimage "image"
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/model"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"

	surfacepkg "github.com/darkliquid/ironwail-go/internal/renderer/surface"
)

// createWorldVertexBuffer uploads vertex data to GPU.
//
// This is one of four vertex-packing functions that must all agree on the byte
// layout — see docs/VERTEX_LAYOUT.md. The stride (48 bytes) must match the
// pipeline's ArrayStride. Missing fields cause wrong textures/lighting; a
// stride that's too small causes the GPU to read into adjacent vertices,
// scrambling all rendering.
func (r *Renderer) createWorldVertexBuffer(device *wgpu.Device, queue *wgpu.Queue, vertices []WorldVertex) (*wgpu.Buffer, error) {
	if len(vertices) == 0 {
		return nil, fmt.Errorf("no vertices to upload")
	}

	// Calculate size. WorldVertex is 48 bytes: 3+2+2+3 floats + 1 float + 1 uint32.
	vertexSize := uint64(len(vertices)) * 48

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

	// Write vertex data to buffer
	vertexData := make([]byte, vertexSize)
	for i, v := range vertices {
		offset := uint64(i) * 48

		// Write position (3 float32 = 12 bytes)
		posBytes := float32ToBytes(v.Position[:])
		copy(vertexData[offset:offset+12], posBytes)

		// Write texCoord (2 float32 = 8 bytes)
		texBytes := float32ToBytes(v.TexCoord[:])
		copy(vertexData[offset+12:offset+20], texBytes)

		// Write lightmapCoord (2 float32 = 8 bytes)
		lightBytes := float32ToBytes(v.LightmapCoord[:])
		copy(vertexData[offset+20:offset+28], lightBytes)

		// Write normal (3 float32 = 12 bytes)
		normBytes := float32ToBytes(v.Normal[:])
		copy(vertexData[offset+28:offset+40], normBytes)

		// Write lightmapLayer (1 float32 = 4 bytes)
		layerBytes := float32ToBytes([]float32{v.LightmapLayer})
		copy(vertexData[offset+40:offset+44], layerBytes)

		// Write materialID (1 uint32 = 4 bytes)
		matIDBytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(matIDBytes, v.MaterialID)
		copy(vertexData[offset+44:offset+48], matIDBytes)
	}

	_ = queue.WriteBuffer(buffer, 0, vertexData)

	slog.Debug("World vertex buffer uploaded", "vertices", len(vertices))

	return buffer, nil
}

// createWorldIndexBuffer uploads index data to GPU
func (r *Renderer) createWorldIndexBuffer(device *wgpu.Device, queue *wgpu.Queue, indices []uint32) (*wgpu.Buffer, uint32, error) {
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

	// Write index data to buffer
	indexData := make([]byte, indexSize)
	for i, idx := range indices {
		offset := uint64(i) * 4
		binary.LittleEndian.PutUint32(indexData[offset:offset+4], idx)
	}

	_ = queue.WriteBuffer(buffer, 0, indexData)

	slog.Debug("World index buffer uploaded", "indices", len(indices))

	return buffer, uint32(len(indices)), nil
}

func (r *Renderer) ensureGoGPUWorldDynamicIndexBuffer(device *wgpu.Device, size uint64) (*wgpu.Buffer, error) {
	if size == 0 {
		return nil, nil
	}
	if r.worldDynamicIndexBuffer != nil && r.worldDynamicIndexBufferSize >= size {
		return r.worldDynamicIndexBuffer, nil
	}
	if r.worldDynamicIndexBuffer != nil {
		r.worldDynamicIndexBuffer.Release()
		r.worldDynamicIndexBuffer = nil
		r.worldDynamicIndexBufferSize = 0
	}
	allocSize := size
	if allocSize < 4096 {
		allocSize = 4096
	}
	buffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "World Dynamic Indices",
		Size:             allocSize,
		Usage:            gputypes.BufferUsageIndex | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return nil, fmt.Errorf("create dynamic world index buffer: %w", err)
	}
	r.worldDynamicIndexBuffer = buffer
	r.worldDynamicIndexBufferSize = allocSize
	return buffer, nil
}

// createWorldRenderTarget ensures the GoGPU world scene target exists for the current framebuffer size.
func (r *Renderer) createWorldRenderTarget() error {
	width, height := r.Size()
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid window size: %dx%d", width, height)
	}
	device := r.getWGPUDevice()
	if device == nil {
		return fmt.Errorf("nil wgpu device")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ensureWorldRenderTargetLocked(device, width, height)
}

func (r *Renderer) createWorldSolidTexture(device *wgpu.Device, queue *wgpu.Queue, label string, pixel [4]byte) (*wgpu.Texture, *wgpu.TextureView, error) {
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

func (r *Renderer) createWorldSolidTextureArray(device *wgpu.Device, queue *wgpu.Queue, label string, pixel [4]byte, layers int) (*wgpu.Texture, *wgpu.TextureView, error) {
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

// createWorldWhiteTexture creates a simple 1x1 white texture for fallback.
// Used when actual textures are not yet available for rendering.
func (r *Renderer) createWorldWhiteTexture(device *wgpu.Device, queue *wgpu.Queue) (*wgpu.Texture, *wgpu.TextureView, error) {
	return r.createWorldSolidTexture(device, queue, "World White Texture", [4]byte{255, 255, 255, 255})
}

func worldLightmapFallbackView(blackView, whiteView *wgpu.TextureView) *wgpu.TextureView {
	// World faces without lightmap data should sample white (full-bright)
	// so they remain visible. The black fallback is only appropriate for
	// brush entity faces where missing lightmaps should stay dark.
	if whiteView != nil {
		return whiteView
	}
	return blackView
}

func (r *Renderer) createWorldTextureFromRGBA(device *wgpu.Device, queue *wgpu.Queue, sampler *wgpu.Sampler, label string, rgba []byte, width, height int) (*gpuWorldTexture, error) {
	if device == nil || queue == nil || sampler == nil {
		return nil, fmt.Errorf("invalid world texture upload inputs")
	}
	if width <= 0 || height <= 0 || len(rgba) != width*height*4 {
		return nil, fmt.Errorf("invalid world texture size/data %dx%d (%d bytes)", width, height, len(rgba))
	}
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         label,
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("create world texture: %w", err)
	}
	if err := queue.WriteTexture(&wgpu.ImageCopyTexture{
		Texture:  texture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, rgba, &wgpu.ImageDataLayout{BytesPerRow: uint32(width * 4), RowsPerImage: uint32(height)}, &wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}); err != nil {
		texture.Release()
		return nil, fmt.Errorf("write world texture: %w", err)
	}
	view, err := device.CreateTextureView(texture, &wgpu.TextureViewDescriptor{
		Label:         label + " View",
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Dimension:     gputypes.TextureViewDimension2D,
		Aspect:        gputypes.TextureAspectAll,
		BaseMipLevel:  0,
		MipLevelCount: 1,
	})
	if err != nil {
		texture.Release()
		return nil, fmt.Errorf("create world texture view: %w", err)
	}
	bindGroup, err := r.createWorldTextureBindGroup(device, sampler, view)
	if err != nil {
		texture.Release()
		view.Release()
		return nil, fmt.Errorf("create world texture bind group: %w", err)
	}
	return &gpuWorldTexture{
		texture:   texture,
		view:      view,
		bindGroup: bindGroup,
		width:     uint32(width),
		height:    uint32(height),
		layers:    1,
	}, nil
}

// createWorldTexture2DFromRGBA creates a single 2D texture (not an array)
// from RGBA pixel data. This is used for the vertically-stacked atlas
// workaround where all atlas layers are packed into one tall 2D texture,
// avoiding the gogpu Vulkan backend bug where WriteTexture hardcodes
// BaseArrayLayer=0 for array textures.
//
// The bind group layout for texture binding uses TextureViewDimension2D,
// so the shader must declare the texture as texture_2d (not texture_2d_array).
func (r *Renderer) createWorldTexture2DFromRGBA(device *wgpu.Device, queue *wgpu.Queue, sampler *wgpu.Sampler, label string, rgba []byte, width, height int) (*gpuWorldTexture, error) {
	if device == nil || queue == nil || sampler == nil {
		return nil, fmt.Errorf("invalid world texture 2D upload inputs")
	}
	if width <= 0 || height <= 0 || len(rgba) != width*height*4 {
		return nil, fmt.Errorf("invalid world texture 2D size/data %dx%d (%d bytes)", width, height, len(rgba))
	}
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         label,
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("create world texture 2D: %w", err)
	}
	if err := writeTextureChunked(queue, texture, rgba, width, height, label); err != nil {
		texture.Release()
		return nil, fmt.Errorf("write world texture 2D: %w", err)
	}
	view, err := device.CreateTextureView(texture, &wgpu.TextureViewDescriptor{
		Label:         label + " View",
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Dimension:     gputypes.TextureViewDimension2D,
		Aspect:        gputypes.TextureAspectAll,
		BaseMipLevel:  0,
		MipLevelCount: 1,
	})
	if err != nil {
		texture.Release()
		return nil, fmt.Errorf("create world texture 2D view: %w", err)
	}
	bindGroup, err := r.createWorldTextureBindGroup(device, sampler, view)
	if err != nil {
		texture.Release()
		view.Release()
		return nil, fmt.Errorf("create world texture 2D bind group: %w", err)
	}
	return &gpuWorldTexture{
		texture:   texture,
		view:      view,
		bindGroup: bindGroup,
		width:     uint32(width),
		height:    uint32(height),
		layers:    1,
	}, nil
}

func (r *Renderer) createWorldTextureArrayFromRGBA(device *wgpu.Device, queue *wgpu.Queue, sampler *wgpu.Sampler, label string, images []*stdimage.RGBA, width, height int) (*gpuWorldTexture, error) {
	if device == nil || queue == nil || sampler == nil {
		return nil, fmt.Errorf("invalid world texture array upload inputs")
	}
	layers := len(images)
	if width <= 0 || height <= 0 || layers == 0 {
		return nil, fmt.Errorf("invalid world texture array size %dx%dx%d", width, height, layers)
	}
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         label,
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: uint32(layers)},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("create world texture array: %w", err)
	}

	for i, img := range images {
		if err := queue.WriteTexture(&wgpu.ImageCopyTexture{
			Texture:  texture,
			MipLevel: 0,
			Aspect:   gputypes.TextureAspectAll,
			Origin:   wgpu.Origin3D{X: 0, Y: 0, Z: uint32(i)},
		}, img.Pix, &wgpu.ImageDataLayout{BytesPerRow: uint32(width * 4), RowsPerImage: uint32(height)}, &wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}); err != nil {
			texture.Release()
			return nil, fmt.Errorf("write world texture array layer %d: %w", i, err)
		}
	}

	view, err := device.CreateTextureView(texture, &wgpu.TextureViewDescriptor{
		Label:           label + " View",
		Format:          gputypes.TextureFormatRGBA8Unorm,
		Dimension:       gputypes.TextureViewDimension2DArray,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: uint32(layers),
	})
	if err != nil {
		texture.Release()
		return nil, fmt.Errorf("create world texture array view: %w", err)
	}
	bindGroup, err := r.createWorldTextureBindGroup(device, sampler, view)
	if err != nil {
		texture.Release()
		view.Release()
		return nil, fmt.Errorf("create world texture array bind group: %w", err)
	}
	return &gpuWorldTexture{
		texture:   texture,
		view:      view,
		bindGroup: bindGroup,
		width:     uint32(width),
		height:    uint32(height),
		layers:    uint32(layers),
	}, nil
}

func shouldDrawGoGPUOpaqueWorldFace(face WorldFace) bool {
	if face.NumIndices == 0 {
		return false
	}
	if face.Flags&(model.SurfDrawSky|model.SurfDrawTurb|model.SurfDrawFence) != 0 {
		return false
	}
	return true
}

func shouldDrawGoGPUAlphaTestWorldFace(face WorldFace) bool {
	return face.NumIndices > 0 && worldFacePass(face.Flags, 1) == worldPassAlphaTest
}

func shouldDrawGoGPUSkyWorldFace(face WorldFace) bool {
	return face.NumIndices > 0 && face.Flags&model.SurfDrawSky != 0
}

func shouldDrawGoGPUOpaqueLiquidFace(face WorldFace, liquidAlpha worldLiquidAlphaSettings) bool {
	return face.NumIndices > 0 && worldFaceIsLiquid(face.Flags) && worldFacePass(face.Flags, worldFaceAlpha(face.Flags, liquidAlpha)) == worldPassOpaque
}

func shouldDrawGoGPUTranslucentLiquidFace(face WorldFace, liquidAlpha worldLiquidAlphaSettings) bool {
	return face.NumIndices > 0 && worldFaceIsLiquid(face.Flags) && worldFacePass(face.Flags, worldFaceAlpha(face.Flags, liquidAlpha)) == worldPassTranslucent
}

func (r *Renderer) createWorldTextureSampler(device *wgpu.Device) (*wgpu.Sampler, error) {
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

// createWorldAtlasSampler creates a sampler for atlas-packed textures.
// ClampToEdge prevents bleeding between atlas sub-rects; the shader handles
// UV wrapping via fract() before remapping into the atlas.
// Linear filtering is used with textureSampleLevel in the shader; the
// half-texel UV inset on atlas bounds prevents inter-texture bleeding.
func (r *Renderer) createWorldAtlasSampler(device *wgpu.Device) (*wgpu.Sampler, error) {
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

func (r *Renderer) createWorldLightmapSampler(device *wgpu.Device) (*wgpu.Sampler, error) {
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

func (r *Renderer) createWorldTextureBindGroup(device *wgpu.Device, sampler *wgpu.Sampler, view *wgpu.TextureView) (*wgpu.BindGroup, error) {
	if device == nil || sampler == nil || view == nil || r.resources.TextureBindGroupLayout == nil {
		return nil, fmt.Errorf("missing world texture bind group resources")
	}
	return device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "World Texture BG",
		Layout: r.resources.TextureBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Sampler: sampler},
			{Binding: 1, TextureView: view},
		},
	})
}

func (r *Renderer) createWorldLightmapBindGroup(device *wgpu.Device, sampler *wgpu.Sampler, view *wgpu.TextureView) (*wgpu.BindGroup, error) {
	if device == nil || sampler == nil || view == nil || r.resources.LightmapBindGroupLayout == nil {
		return nil, fmt.Errorf("missing world lightmap bind group resources")
	}
	return device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "World Lightmap Array BG",
		Layout: r.resources.LightmapBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Sampler: sampler},
			{Binding: 1, TextureView: view},
		},
	})
}

func (r *Renderer) uploadWorldMaterialTextures(device *wgpu.Device, queue *wgpu.Queue, sampler *wgpu.Sampler, tree *bsp.Tree) (*gpuWorldTexture, *gpuWorldTexture, []*surfacepkg.SurfaceTexture, []WorldMaterialData) {
	if tree == nil || device == nil || queue == nil || sampler == nil || len(tree.TextureData) < 4 {
		return nil, nil, nil, nil
	}
	textureCount := int(binary.LittleEndian.Uint32(tree.TextureData[:4]))
	if textureCount <= 0 || len(tree.TextureData) < 4+textureCount*4 {
		return nil, nil, nil, nil
	}

	// Diagnostic: Check if we're dealing with a very large texture count
	if textureCount > 50000 {
		slog.Debug("Extremely large texture count detected",
			"texture_count", textureCount,
			"bsp_version", tree.Version,
		)
	}

	// Phase 1 diagnostic: warn if material count exceeds the fixed GPU
	// buffer capacity (256 entries). This is the suspected root cause of
	// texture corruption on large BSP2 maps like qbj2 start.
	diagMaterialBufferCapacity("uploadWorldMaterialTextures", textureCount+2)

	atlas := NewWorldTextureAtlas(2048, 2048)
	fbAtlas := NewWorldTextureAtlas(2048, 2048)
	fbHasData := false

	baseMaterials := make([]WorldMaterialData, textureCount+2)
	textureNames := make([]string, textureCount)

	// transparent 1x1 black image used as fullbright placeholder for
	// textures that have no fullbright pixels.
	fbBlank := &stdimage.RGBA{Pix: []byte{0, 0, 0, 0}, Stride: 4, Rect: stdimage.Rect(0, 0, 1, 1)}

	// Insert dummy textures first so their atlas positions are known upfront.
	// Faces with missing/invalid miptex indices are remapped to these slots
	// by worldFaceTextureIndex (textureCount = white, textureCount+1 = transparent).
	dummyWhite := &stdimage.RGBA{Pix: []byte{255, 255, 255, 255}, Stride: 4, Rect: stdimage.Rect(0, 0, 1, 1)}
	dummyTransparent := &stdimage.RGBA{Pix: []byte{0, 0, 0, 0}, Stride: 4, Rect: stdimage.Rect(0, 0, 1, 1)}
	dummyWhiteIns, du, dv, dw, dh, dErr := atlas.InsertWithRect(dummyWhite)
	if dErr != nil {
		slog.Warn("failed to insert dummy white texture into atlas", "error", dErr)
	} else {
		baseMaterials[textureCount] = WorldMaterialData{
			AtlasBounds: [4]float32{du, dv, dw, dh},
			Layer:       float32(dummyWhiteIns.Layer),
		}
		fbAtlas.EnsureLayerCount(len(atlas.layers))
		fbAtlas.DrawAt(fbBlank, dummyWhiteIns)
	}
	dummyTransparentIns, dtu, dtv, dtw, dth, dtErr := atlas.InsertWithRect(dummyTransparent)
	if dtErr != nil {
		slog.Warn("failed to insert dummy transparent texture into atlas", "error", dtErr)
	} else {
		baseMaterials[textureCount+1] = WorldMaterialData{
			AtlasBounds: [4]float32{dtu, dtv, dtw, dth},
			Layer:       float32(dummyTransparentIns.Layer),
		}
		fbAtlas.EnsureLayerCount(len(atlas.layers))
		fbAtlas.DrawAt(fbBlank, dummyTransparentIns)
	}

	for i := 0; i < textureCount; i++ {
		offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[4+i*4:])))
		if offset <= 0 || offset >= len(tree.TextureData) {
			// Point failed textures at the white dummy so faces still render.
			baseMaterials[i] = baseMaterials[textureCount]
			continue
		}
		miptex, err := image.ParseMipTex(tree.TextureData[offset:])
		if err != nil {
			baseMaterials[i] = baseMaterials[textureCount]
			continue
		}
		textureNames[i] = miptex.Name
		pixels, width, height, err := miptex.MipLevel(0)
		if err != nil || width <= 0 || height <= 0 {
			baseMaterials[i] = baseMaterials[textureCount]
			continue
		}
		textureType := classifyWorldTextureName(miptex.Name)
		materialRGBA := worldimpl.BuildMaterialTextureRGBA(pixels, r.palette, textureType)

		// Apply alpha edge fix for cutout textures (matching the old
		// createWorldDiffuseTexture path that called AlphaEdgeFix).
		if textureType == model.TexTypeCutout {
			cutout := &stdimage.RGBA{
				Pix:    materialRGBA.DiffuseRGBA,
				Stride: int(miptex.Width) * 4,
				Rect:   stdimage.Rect(0, 0, int(miptex.Width), int(miptex.Height)),
			}
			image.AlphaEdgeFix(cutout)
		}

		diffuseImg := &stdimage.RGBA{
			Pix:    materialRGBA.DiffuseRGBA,
			Stride: int(miptex.Width) * 4,
			Rect:   stdimage.Rect(0, 0, int(miptex.Width), int(miptex.Height)),
		}
		ins, u, v, w, h, err := atlas.InsertWithRect(diffuseImg)
		if err != nil {
			slog.Warn("failed to insert world diffuse texture into atlas", "texture", miptex.Name, "error", err)
			continue
		}

		baseMaterials[i] = WorldMaterialData{
			AtlasBounds: [4]float32{u, v, w, h},
			Layer:       float32(ins.Layer),
		}

		// Diagnostic: Check for invalid layer assignment
		if ins.Layer < 0 || ins.Layer > 100 {
			slog.Warn("Suspicious texture layer assignment",
				"texture_index", i,
				"layer", ins.Layer,
				"texture_name", miptex.Name,
			)
		}

		// Insert the fullbright companion at the same layer/rect so both
		// atlases have identical layouts. Use a transparent placeholder
		// for textures without fullbright data so the shader can safely
		// sample the fullbright atlas at every material's position.
		fbAtlas.EnsureLayerCount(len(atlas.layers))
		var fbImg *stdimage.RGBA
		if materialRGBA.HasFullbright {
			fbHasData = true
			fbImg = &stdimage.RGBA{
				Pix:    materialRGBA.FullbrightRGBA,
				Stride: int(miptex.Width) * 4,
				Rect:   stdimage.Rect(0, 0, int(miptex.Width), int(miptex.Height)),
			}
		} else {
			fbImg = &stdimage.RGBA{
				Pix:    make([]byte, int(miptex.Width)*int(miptex.Height)*4),
				Stride: int(miptex.Width) * 4,
				Rect:   stdimage.Rect(0, 0, int(miptex.Width), int(miptex.Height)),
			}
		}
		fbAtlas.DrawAt(fbImg, ins)
	}

	slog.Debug("World texture atlas built",
		"texture_count", textureCount,
		"atlas_layers", len(atlas.layers),
		"has_fullbright", fbHasData,
		"base_materials", len(baseMaterials),
		"bsp_version", tree.Version,
		"texture_data_size", len(tree.TextureData),
	)

	// Phase 3 diagnostic: log per-layer atlas distribution and validate
	// that all material bounds and layer indices are within valid ranges.
	diagAtlasLayerDistribution(atlas, baseMaterials)

	// Build a map name for diagnostic file output. The tree doesn't carry
	// the original filename, so we use the BSP version and texture count.
	mapName := fmt.Sprintf("bsp_v%d_textures_%d", tree.Version, textureCount)

	// Diagnostic logging for large maps to identify issues
	if textureCount > 10000 && len(atlas.layers) > 1 {
		slog.Debug("Large map with multiple atlas layers detected",
			"texture_count", textureCount,
			"atlas_layers", len(atlas.layers),
		)
	}

	// Always build the fullbright atlas with the same layer count as the
	// diffuse atlas. The shader samples both at the same V-offset, so the
	// fullbright atlas must have the same vertical layout as the diffuse atlas.
	fbAtlas.EnsureLayerCount(len(atlas.layers))

	// Workaround for gogpu Vulkan backend bug: WriteTexture hardcodes
	// BaseArrayLayer=0, so texture array layers > 0 are never written.
	// Instead of a texture_2d_array, we pack all atlas layers vertically
	// into a single tall 2D texture. The material "Layer" field is
	// repurposed from an array layer index to a V-offset fraction that the
	// shader adds to the atlas V coordinate.
	atlasLayerCount := len(atlas.layers)
	diffuseImage := atlas.FlattenVertical()
	fbImage := fbAtlas.FlattenVertical()

	// Repurpose the Layer field as a V-offset and rescale the V/H atlas
	// bounds to account for the vertical stacking with padding.
	//
	// FlattenVertical adds 1-pixel padding above and below each layer,
	// so each layer occupies (atlasHeight + 2) rows. Layer i's content
	// starts at row (i * (atlasHeight + 2) + 1) in the tall texture.
	//
	// totalTallHeight = numLayers * (atlasHeight + 2)
	//
	// The atlas bounds were computed relative to a single 2048-tall layer:
	//   bounds.v = offsetY / 2048
	//   bounds.h = texHeight / 2048
	//
	// We need them normalized to the full tall texture:
	//   bounds.v = offsetY / totalTallHeight
	//   bounds.h = texHeight / totalTallHeight
	//
	// The Layer field stores the layer's content-start V-offset:
	//   layer = (layerIdx * (atlasHeight + 2) + 1) / totalTallHeight
	//
	// The shader computes: atlasV = bounds.v + fract(texCoord.y) * bounds.h + layer
	// which maps the wrapped texture coordinate into the correct sub-rect
	// of the correct layer within the tall texture.
	atlasHeight := 2048
	atlasWidth := 2048
	rowsPerLayer := atlasHeight + 2
	totalTallHeight := atlasLayerCount * rowsPerLayer

	// Half-texel inset to prevent linear filtering from bleeding across
	// texture boundaries within the atlas. The shader computes:
	//   atlasUV = fract(texCoord) * bounds.zw + bounds.xy + layer
	// By insetting bounds by half a texel on each side, the sampled point
	// never reaches the exact edge of the texture sub-rect, so linear
	// filtering always samples within the correct texture.
	halfTexelU := 0.5 / float32(atlasWidth)
	halfTexelV := 0.5 / float32(totalTallHeight)

	for i := range baseMaterials {
		layerIdx := int(baseMaterials[i].Layer)
		if atlasLayerCount > 0 {
			baseMaterials[i].Layer = float32(layerIdx*rowsPerLayer+1) / float32(totalTallHeight)
			// Rescale V and H from single-layer normalization to full tall texture.
			baseMaterials[i].AtlasBounds[1] = baseMaterials[i].AtlasBounds[1] * float32(atlasHeight) / float32(totalTallHeight) // V
			baseMaterials[i].AtlasBounds[3] = baseMaterials[i].AtlasBounds[3] * float32(atlasHeight) / float32(totalTallHeight) // H
			// Apply half-texel inset to prevent inter-texture bleeding.
			baseMaterials[i].AtlasBounds[0] += halfTexelU     // U: shift right by half texel
			baseMaterials[i].AtlasBounds[1] += halfTexelV     // V: shift down by half texel
			baseMaterials[i].AtlasBounds[2] -= 2 * halfTexelU // W: shrink by one texel
			baseMaterials[i].AtlasBounds[3] -= 2 * halfTexelV // H: shrink by one texel
		}
	}

	var diffuseWorldTexture *gpuWorldTexture
	if diffuseImage != nil {
		diffuseWorldTexture, _ = r.createWorldTexture2DFromRGBA(device, queue, sampler, "World Diffuse Atlas", diffuseImage.Pix, diffuseImage.Bounds().Dx(), diffuseImage.Bounds().Dy())
	}

	var fbWorldTexture *gpuWorldTexture
	if fbImage != nil {
		fbWorldTexture, _ = r.createWorldTexture2DFromRGBA(device, queue, sampler, "World Fullbright Atlas", fbImage.Pix, fbImage.Bounds().Dx(), fbImage.Bounds().Dy())
	}

	animations, err := surfacepkg.BuildTextureAnimations(textureNames)
	if err != nil {
		slog.Warn("failed to build world texture animations", "error", err)
	}

	// Phase 4 diagnostic: audit animation chains for out-of-range indices.
	diagAnimationChains(animations, len(baseMaterials))

	// Phase 5 diagnostic: dump the full material table including animation
	// chain info when IRONWAIL_DEBUG_MATERIAL_DUMP=1.
	diagMaterialTableDump(baseMaterials, textureNames, animations, atlas, mapName)

	return diffuseWorldTexture, fbWorldTexture, animations, baseMaterials
}

func shouldSplitAsQuake64Sky(treeVersion int32, width, height int) bool {
	return worldimpl.ShouldSplitAsQuake64Sky(treeVersion, width, height)
}

func extractEmbeddedSkyLayers(pixels []byte, width, height int, palette []byte, quake64 bool) (solidRGBA, alphaRGBA []byte, layerWidth, layerHeight int, ok bool) {
	return worldimpl.ExtractEmbeddedSkyLayers(pixels, width, height, palette, quake64)
}

func (r *Renderer) uploadWorldEmbeddedSkyTextures(device *wgpu.Device, queue *wgpu.Queue, sampler *wgpu.Sampler, tree *bsp.Tree) (map[int32]*gpuWorldTexture, map[int32]*gpuWorldTexture) {
	if tree == nil || device == nil || queue == nil || sampler == nil || len(tree.TextureData) < 4 {
		return nil, nil
	}
	textureCount := int(binary.LittleEndian.Uint32(tree.TextureData[:4]))
	if textureCount <= 0 || len(tree.TextureData) < 4+textureCount*4 {
		return nil, nil
	}
	solid := make(map[int32]*gpuWorldTexture)
	alpha := make(map[int32]*gpuWorldTexture)
	for i := 0; i < textureCount; i++ {
		offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[4+i*4:])))
		if offset <= 0 || offset >= len(tree.TextureData) {
			continue
		}
		miptex, err := image.ParseMipTex(tree.TextureData[offset:])
		if err != nil || classifyWorldTextureName(miptex.Name) != model.TexTypeSky {
			continue
		}
		pixels, width, height, err := miptex.MipLevel(0)
		if err != nil {
			continue
		}
		solidRGBA, alphaRGBA, layerWidth, layerHeight, ok := extractEmbeddedSkyLayers(pixels, width, height, r.palette, shouldSplitAsQuake64Sky(tree.Version, width, height))
		if !ok {
			continue
		}
		solidTexture, err := r.createWorldTextureFromRGBA(device, queue, sampler, "World Sky Solid Texture", solidRGBA, layerWidth, layerHeight)
		if err != nil {
			slog.Warn("failed to upload world sky solid texture", "texture", miptex.Name, "error", err)
			continue
		}
		alphaTexture, err := r.createWorldTextureFromRGBA(device, queue, sampler, "World Sky Alpha Texture", alphaRGBA, layerWidth, layerHeight)
		if err != nil {
			if solidTexture.bindGroup != nil {
				solidTexture.bindGroup.Release()
			}
			if solidTexture.view != nil {
				solidTexture.view.Release()
			}
			if solidTexture.texture != nil {
				solidTexture.texture.Release()
			}
			slog.Warn("failed to upload world sky alpha texture", "texture", miptex.Name, "error", err)
			continue
		}
		solid[int32(i)] = solidTexture
		alpha[int32(i)] = alphaTexture
	}
	return solid, alpha
}

// maxTextureUploadChunkSize is the maximum bytes per WriteTexture call.
// The Vulkan backend creates a staging buffer for each WriteTexture call,
// and the driver silently caps staging buffer allocations at 64 MiB
// (67,108,864 bytes). Data exceeding this limit is silently truncated,
// corrupting the texture. We use 60 MiB to stay safely under the cap.
const maxTextureUploadChunkSize = 60 * 1024 * 1024

// writeTextureChunked uploads a 2D RGBA texture to the GPU, splitting
// the data into vertical chunks when the total size exceeds the GPU's
// staging buffer limit. Each chunk is uploaded via a separate
// WriteTexture call targeting a horizontal slice of the destination
// texture.
func writeTextureChunked(queue *wgpu.Queue, texture *wgpu.Texture, rgba []byte, width, height int, label string) error {
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
