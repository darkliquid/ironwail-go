package renderer

import (
	"log/slog"
	"unsafe"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"

	"github.com/darkliquid/ironwail-go/internal/renderer/lightmap"
)

func (r *Renderer) uploadWorldLightmapArray(device *wgpu.Device, queue *wgpu.Queue, sampler *wgpu.Sampler, pages []WorldLightmapPage, values [256]float32) *gpuWorldTexture {
	if device == nil || queue == nil || sampler == nil || len(pages) == 0 {
		return nil
	}

	width := uint32(pages[0].Width)

	// Workaround for gogpu Vulkan backend bug: WriteTexture hardcodes
	// BaseArrayLayer=0, so array layers > 0 are never written. Instead
	// of a texture_2d_array, we stack all lightmap pages vertically into
	// a single tall 2D texture. The shader uses texture_2d and applies
	// a V-offset per page.
	//
	// Each page gets 1-pixel padding above and below to prevent linear
	// filter bleeding between adjacent pages in the tall texture.
	rowsPerPage := pages[0].Height + 2
	totalHeight := rowsPerPage * len(pages)

	// Build a single tall RGBA buffer with all pages stacked vertically.
	rgba := lightmap.StackPages(pages, values, int(width))
	if len(rgba) == 0 {
		return nil
	}

	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "World Lightmap Texture",
		Size:          wgpu.Extent3D{Width: width, Height: uint32(totalHeight), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		slog.Warn("failed to create world lightmap texture", "error", err)
		return nil
	}

	if err := writeTextureChunked(queue, texture, rgba, int(width), int(totalHeight), "World Lightmap Texture"); err != nil {
		texture.Release()
		slog.Warn("failed to write world lightmap texture", "error", err)
		return nil
	}

	view, err := device.CreateTextureView(texture, &wgpu.TextureViewDescriptor{
		Label:         "World Lightmap Texture View",
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Dimension:     gputypes.TextureViewDimension2D,
		Aspect:        gputypes.TextureAspectAll,
		BaseMipLevel:  0,
		MipLevelCount: 1,
	})
	if err != nil {
		texture.Release()
		slog.Warn("failed to create world lightmap view", "error", err)
		return nil
	}

	bindGroup, err := r.createWorldLightmapBindGroup(device, sampler, view)
	if err != nil {
		view.Release()
		texture.Release()
		slog.Warn("failed to create world lightmap bind group", "error", err)
		return nil
	}

	return &gpuWorldTexture{
		texture:   texture,
		view:      view,
		bindGroup: bindGroup,
		width:     width,
		height:    uint32(totalHeight),
		layers:    1,
	}
}

func updateUploadedLightmapsLocked(queue *wgpu.Queue, uploaded *gpuWorldTexture, pages []WorldLightmapPage, values [256]float32) {
	if queue == nil || len(pages) == 0 || uploaded == nil || uploaded.texture == nil {
		return
	}
	// Each page is stacked vertically in the 2D texture with 1px padding
	// above and below. Page i's content starts at row (i * (pageHeight + 2) + 1).
	// Dirty region updates write at the page's content Y offset.
	pageHeight := uint32(pages[0].Height)
	rowsPerPage := pageHeight + 2
	count := len(pages)
	for i := 0; i < count; i++ {
		if !pages[i].Dirty {
			continue
		}
		rgba := lightmap.CompositePageRGBA(&pages[i], values)
		if len(rgba) == 0 {
			continue
		}
		x, y, w, h := lightmap.DirtyBounds(pages[i])
		if w == 0 || h == 0 {
			continue
		}
		region := lightmap.ExtractRegionRGBA(pages[i].CachedRegionRGBA, rgba, pages[i].Width, x, y, w, h)
		if len(region) == 0 {
			continue
		}
		pages[i].CachedRegionRGBA = region
		contentY := uint32(i)*rowsPerPage + 1
		if err := queue.WriteTexture(&wgpu.ImageCopyTexture{
			Texture:  uploaded.texture,
			MipLevel: 0,
			Aspect:   gputypes.TextureAspectAll,
			Origin:   wgpu.Origin3D{X: uint32(x), Y: contentY + uint32(y), Z: 0},
		}, region, &wgpu.ImageDataLayout{BytesPerRow: uint32(w * 4), RowsPerImage: uint32(h)}, &wgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}); err != nil {
			slog.Warn("failed to update world lightmap page", "page", i, "error", err)
		}
	}
	lightmap.ClearDirtyFlags(pages)
}

// setGoGPUWorldLightStyleValues stores the current lightstyle values for
// potential future use. Unlike the previous implementation, it does NOT
// re-composite lightmap surfaces on the CPU every frame — lightmap textures
// are built once at level load with all styles at scale 1.0 (matching C
// Ironwail behavior). Dynamic lightstyle animation (flickering torches) is
// handled by the dynamic light cluster system, not by modifying lightmap
// textures. This eliminates the 9.69% CPU overhead from
// compositeWorldLightmapSurfaceRGBA per-frame re-compositing.
func (r *Renderer) setGoGPUWorldLightStyleValues(values [256]float32) {
	r.mu.Lock()
	r.worldLightStyleValues = values
	r.mu.Unlock()
}

// Helper functions to convert Go types to byte slices
func float32ToBytes(f []float32) []byte {
	if len(f) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(f))), len(f)*4)
}

func uint32SliceToBytes(values []uint32) []byte {
	if len(values) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(values))), len(values)*4)
}
