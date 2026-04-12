package renderer

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"unsafe"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

func (r *Renderer) createWorldLightmapPageTexture(device *wgpu.Device, queue *wgpu.Queue, sampler *wgpu.Sampler, page *WorldLightmapPage, values [64]float32) (*gpuWorldTexture, error) {
	if device == nil || queue == nil || sampler == nil || page == nil {
		return nil, fmt.Errorf("invalid world lightmap upload inputs")
	}
	rgba := buildWorldLightmapPageRGBA(page, values)
	if len(rgba) == 0 {
		return nil, fmt.Errorf("empty world lightmap page")
	}
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "World Lightmap Texture",
		Size:          wgpu.Extent3D{Width: uint32(page.Width), Height: uint32(page.Height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("create world lightmap texture: %w", err)
	}
	if err := queue.WriteTexture(&wgpu.ImageCopyTexture{
		Texture:  texture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, rgba, &wgpu.ImageDataLayout{BytesPerRow: uint32(page.Width * 4), RowsPerImage: uint32(page.Height)}, &wgpu.Extent3D{Width: uint32(page.Width), Height: uint32(page.Height), DepthOrArrayLayers: 1}); err != nil {
		texture.Release()
		return nil, fmt.Errorf("write world lightmap texture: %w", err)
	}
	view, err := device.CreateTextureView(texture, &wgpu.TextureViewDescriptor{
		Label:           "World Lightmap Texture View",
		Format:          gputypes.TextureFormatRGBA8Unorm,
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	})
	if err != nil {
		texture.Release()
		return nil, fmt.Errorf("create world lightmap view: %w", err)
	}
	bindGroup, err := r.createWorldTextureBindGroup(device, sampler, view)
	if err != nil {
		view.Release()
		texture.Release()
		return nil, fmt.Errorf("create world lightmap bind group: %w", err)
	}
	return &gpuWorldTexture{texture: texture, view: view, bindGroup: bindGroup}, nil
}

func (r *Renderer) uploadWorldLightmapPages(device *wgpu.Device, queue *wgpu.Queue, sampler *wgpu.Sampler, pages []WorldLightmapPage, values [64]float32) []*gpuWorldTexture {
	if device == nil || queue == nil || sampler == nil || len(pages) == 0 {
		return nil
	}
	out := make([]*gpuWorldTexture, len(pages))
	for i := range pages {
		pageTexture, err := r.createWorldLightmapPageTexture(device, queue, sampler, &pages[i], values)
		if err != nil {
			slog.Warn("failed to upload world lightmap page", "page", i, "error", err)
			continue
		}
		out[i] = pageTexture
	}
	return out
}

func lightmapDirtyBounds(page WorldLightmapPage) (x, y, w, h int) {
	minX, minY := page.Width, page.Height
	maxX, maxY := 0, 0
	found := false
	for _, surface := range page.Surfaces {
		if !surface.Dirty || surface.Width <= 0 || surface.Height <= 0 {
			continue
		}
		if surface.X < minX {
			minX = surface.X
		}
		if surface.Y < minY {
			minY = surface.Y
		}
		if surface.X+surface.Width > maxX {
			maxX = surface.X + surface.Width
		}
		if surface.Y+surface.Height > maxY {
			maxY = surface.Y + surface.Height
		}
		found = true
	}
	if !found {
		return 0, 0, 0, 0
	}
	return minX, minY, maxX - minX, maxY - minY
}

func extractLightmapRegionRGBA(dst, rgba []byte, pageWidth, x, y, w, h int) []byte {
	if len(rgba) == 0 || pageWidth <= 0 || w <= 0 || h <= 0 {
		return nil
	}
	size := w * h * 4
	if cap(dst) < size {
		dst = make([]byte, size)
	} else {
		dst = dst[:size]
	}
	srcStride := pageWidth * 4
	dstStride := w * 4
	for row := 0; row < h; row++ {
		srcStart := ((y + row) * srcStride) + x*4
		srcEnd := srcStart + dstStride
		dstStart := row * dstStride
		copy(dst[dstStart:dstStart+dstStride], rgba[srcStart:srcEnd])
	}
	return dst
}

func updateUploadedLightmapsLocked(queue *wgpu.Queue, uploaded []*gpuWorldTexture, pages []WorldLightmapPage, values [64]float32) {
	if queue == nil || len(pages) == 0 || len(uploaded) == 0 {
		return
	}
	count := len(uploaded)
	if len(pages) < count {
		count = len(pages)
	}
	for i := 0; i < count; i++ {
		if !pages[i].Dirty || uploaded[i] == nil || uploaded[i].texture == nil {
			continue
		}
		rgba := buildWorldLightmapPageRGBA(&pages[i], values)
		if len(rgba) == 0 {
			continue
		}
		x, y, w, h := lightmapDirtyBounds(pages[i])
		if w == 0 || h == 0 {
			continue
		}
		region := extractLightmapRegionRGBA(pages[i].CachedRegionRGBA, rgba, pages[i].Width, x, y, w, h)
		if len(region) == 0 {
			continue
		}
		pages[i].CachedRegionRGBA = region
		if err := queue.WriteTexture(&wgpu.ImageCopyTexture{
			Texture:  uploaded[i].texture,
			MipLevel: 0,
			Aspect:   gputypes.TextureAspectAll,
			Origin:   wgpu.Origin3D{X: uint32(x), Y: uint32(y)},
		}, region, &wgpu.ImageDataLayout{BytesPerRow: uint32(w * 4), RowsPerImage: uint32(h)}, &wgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}); err != nil {
			slog.Warn("failed to update world lightmap page", "page", i, "error", err)
		}
	}
	clearDirtyFlags(pages)
}

func (r *Renderer) setGoGPUWorldLightStyleValues(values [64]float32) {
	queue := r.getWGPUQueue()
	if queue == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	changed := lightStylesChanged(r.worldLightStyleValues, values)
	if !anyLightStyleChanged(changed) {
		return
	}
	if r.worldData != nil && r.worldData.Geometry != nil {
		markDirtyLightmapPages(r.worldData.Geometry.Lightmaps, changed)
		updateUploadedLightmapsLocked(queue, r.worldLightmapPages, r.worldData.Geometry.Lightmaps, values)
	}
	for submodelIndex, geom := range r.brushModelGeometry {
		if geom == nil || len(geom.Lightmaps) == 0 {
			continue
		}
		markDirtyLightmapPages(geom.Lightmaps, changed)
		updateUploadedLightmapsLocked(queue, r.brushModelLightmaps[submodelIndex], geom.Lightmaps, values)
	}
	r.worldLightStyleValues = values
}

func defaultWorldLightStyleValues() [64]float32 {
	var values [64]float32
	values[0] = 1
	return values
}

func worldLightstyleScale(values [64]float32, style uint8) float32 {
	if int(style) >= len(values) {
		return 0
	}
	return values[style]
}

func compositeWorldLightmapSurfaceRGBA(rgba []byte, pageWidth int, surface WorldLightmapSurface, values [64]float32) {
	if surface.Width <= 0 || surface.Height <= 0 {
		return
	}
	styleCount := 0
	for _, style := range surface.Styles {
		if style == 255 {
			break
		}
		styleCount++
	}
	if styleCount == 0 {
		styleCount = 1
	}
	faceSize := surface.Width * surface.Height * 3
	if len(surface.Samples) < faceSize*styleCount {
		return
	}
	for y := 0; y < surface.Height; y++ {
		for x := 0; x < surface.Width; x++ {
			sampleIndex := (y*surface.Width + x) * 3
			var rSum, gSum, bSum float32
			for styleIndex := 0; styleIndex < styleCount; styleIndex++ {
				offset := styleIndex*faceSize + sampleIndex
				scale := worldLightstyleScale(values, surface.Styles[styleIndex])
				rSum += float32(surface.Samples[offset]) * scale
				gSum += float32(surface.Samples[offset+1]) * scale
				bSum += float32(surface.Samples[offset+2]) * scale
			}
			dst := ((surface.Y+y)*pageWidth + (surface.X + x)) * 4
			rgba[dst] = byte(clamp01(rSum/255.0) * 255)
			rgba[dst+1] = byte(clamp01(gSum/255.0) * 255)
			rgba[dst+2] = byte(clamp01(bSum/255.0) * 255)
			rgba[dst+3] = 255
		}
	}
}

func buildWorldLightmapPageRGBA(page *WorldLightmapPage, values [64]float32) []byte {
	if page.Width <= 0 || page.Height <= 0 {
		return nil
	}
	size := page.Width * page.Height * 4
	if len(page.CachedRGBA) != size {
		page.CachedRGBA = make([]byte, size)
		for i := 0; i < len(page.CachedRGBA); i += 4 {
			page.CachedRGBA[i+3] = 255
		}
		for _, surface := range page.Surfaces {
			compositeWorldLightmapSurfaceRGBA(page.CachedRGBA, page.Width, surface, values)
		}
		return page.CachedRGBA
	}
	if page.Dirty {
		recompositeDirtySurfaces(page.CachedRGBA, *page, values)
	}
	return page.CachedRGBA
}

func lightStylesChanged(old, new_ [64]float32) [64]bool {
	var changed [64]bool
	for i := range old {
		if old[i] != new_[i] {
			changed[i] = true
		}
	}
	return changed
}

func anyLightStyleChanged(changed [64]bool) bool {
	for _, dirty := range changed {
		if dirty {
			return true
		}
	}
	return false
}

func markDirtyLightmapPages(pages []WorldLightmapPage, changed [64]bool) {
	for i := range pages {
		pageDirty := false
		for j := range pages[i].Surfaces {
			surf := &pages[i].Surfaces[j]
			for _, style := range surf.Styles {
				if style == 255 {
					break
				}
				if style < 64 && changed[style] {
					surf.Dirty = true
					pageDirty = true
					break
				}
			}
		}
		if pageDirty {
			pages[i].Dirty = true
		}
	}
}

func clearDirtyFlags(pages []WorldLightmapPage) {
	for i := range pages {
		if !pages[i].Dirty {
			continue
		}
		for j := range pages[i].Surfaces {
			pages[i].Surfaces[j].Dirty = false
		}
		pages[i].Dirty = false
	}
}

func recompositeDirtySurfaces(rgba []byte, page WorldLightmapPage, values [64]float32) bool {
	recomposited := false
	for _, surface := range page.Surfaces {
		if !surface.Dirty {
			continue
		}
		compositeWorldLightmapSurfaceRGBA(rgba, page.Width, surface, values)
		recomposited = true
	}
	return recomposited
}

// Helper functions to convert Go types to byte slices
func float32ToBytes(f []float32) []byte {
	result := make([]byte, len(f)*4)
	for i, v := range f {
		binary.LittleEndian.PutUint32(result[i*4:i*4+4], math.Float32bits(v))
	}
	return result
}

// uint32ToBytes expands packed integer data into byte form for uploads to APIs expecting byte-addressable buffers/textures.
func uint32ToBytes(u uint32) []byte {
	result := make([]byte, 4)
	binary.LittleEndian.PutUint32(result, u)
	return result
}

func uint32SliceToBytes(values []uint32) []byte {
	if len(values) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(values))), len(values)*4)
}
