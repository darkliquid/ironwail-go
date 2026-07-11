package renderer

import (
	"log/slog"
	"unsafe"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

func (r *Renderer) uploadWorldLightmapArray(device *wgpu.Device, queue *wgpu.Queue, sampler *wgpu.Sampler, pages []WorldLightmapPage, values [256]float32) *gpuWorldTexture {
	if device == nil || queue == nil || sampler == nil || len(pages) == 0 {
		return nil
	}

	width := uint32(pages[0].Width)
	height := uint32(pages[0].Height)
	numPages := uint32(len(pages))

	// Workaround for gogpu Vulkan backend bug: WriteTexture hardcodes
	// BaseArrayLayer=0, so array layers > 0 are never written. Instead
	// of a texture_2d_array, we stack all lightmap pages vertically into
	// a single tall 2D texture. The shader uses texture_2d and applies
	// a V-offset per page.
	//
	// Each page gets 1-pixel padding above and below to prevent linear
	// filter bleeding between adjacent pages in the tall texture.
	rowsPerPage := height + 2
	totalHeight := rowsPerPage * numPages

	// Build a single tall RGBA buffer with all pages stacked vertically.
	totalPixels := int(width * totalHeight * 4)
	rgba := make([]byte, totalPixels)
	srcStride := int(width) * 4
	for i := range pages {
		pageRGBA := buildWorldLightmapPageRGBA(&pages[i], values)
		if len(pageRGBA) == 0 {
			continue
		}
		// Page content starts at row (i * rowsPerPage + 1).
		yOffset := int(rowsPerPage) * i
		contentY := yOffset + 1
		for row := 0; row < int(height); row++ {
			srcStart := row * srcStride
			dstStart := (contentY + row) * srcStride
			copy(rgba[dstStart:dstStart+srcStride], pageRGBA[srcStart:srcStart+srcStride])
		}
		// Replicate top edge row into padding row above.
		if int(height) > 0 {
			topRowStart := 0
			padRowStart := yOffset * srcStride
			copy(rgba[padRowStart:padRowStart+srcStride], pageRGBA[topRowStart:topRowStart+srcStride])
			// Replicate bottom edge row into padding row below.
			botRowStart := int(height-1) * srcStride
			padBotStart := (contentY + int(height)) * srcStride
			copy(rgba[padBotStart:padBotStart+srcStride], pageRGBA[botRowStart:botRowStart+srcStride])
		}
	}

	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "World Lightmap Texture",
		Size:          wgpu.Extent3D{Width: width, Height: totalHeight, DepthOrArrayLayers: 1},
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

	if err := queue.WriteTexture(&wgpu.ImageCopyTexture{
		Texture:  texture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, rgba, &wgpu.ImageDataLayout{BytesPerRow: width * 4, RowsPerImage: totalHeight}, &wgpu.Extent3D{Width: width, Height: totalHeight, DepthOrArrayLayers: 1}); err != nil {
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
		height:    totalHeight,
		layers:    1,
	}
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

func extractLightmapRegionRGBA(dst []byte, rgba []byte, srcWidth, x, y, w, h int) []byte {
	if w <= 0 || h <= 0 || len(rgba) == 0 {
		return nil
	}
	size := w * h * 4
	if len(dst) != size {
		dst = make([]byte, size)
	}
	srcStride := srcWidth * 4
	dstStride := w * 4
	for row := 0; row < h; row++ {
		srcStart := ((y + row) * srcStride) + x*4
		srcEnd := srcStart + dstStride
		dstStart := row * dstStride
		copy(dst[dstStart:dstStart+dstStride], rgba[srcStart:srcEnd])
	}
	return dst
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
	clearDirtyFlags(pages)
}

func (r *Renderer) setGoGPUWorldLightStyleValues(values [256]float32) {
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
		updateUploadedLightmapsLocked(queue, r.worldLightmapArray, r.worldData.Geometry.Lightmaps, values)
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

func defaultWorldLightStyleValues() [256]float32 {
	var values [256]float32
	values[0] = 1
	return values
}

func worldLightstyleScale(values [256]float32, style uint8) float32 {
	if int(style) >= len(values) {
		return 0
	}
	return values[style]
}

func compositeWorldLightmapSurfaceRGBA(rgba []byte, pageWidth int, surface WorldLightmapSurface, values [256]float32) {
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

func buildWorldLightmapPageRGBA(page *WorldLightmapPage, values [256]float32) []byte {
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

func lightStylesChanged(old, new_ [256]float32) [256]bool {
	var changed [256]bool
	for i := range old {
		if old[i] != new_[i] {
			changed[i] = true
		}
	}
	return changed
}

func anyLightStyleChanged(changed [256]bool) bool {
	for _, dirty := range changed {
		if dirty {
			return true
		}
	}
	return false
}

func markDirtyLightmapPages(pages []WorldLightmapPage, changed [256]bool) {
	for i := range pages {
		pageDirty := false
		for j := range pages[i].Surfaces {
			surf := &pages[i].Surfaces[j]
			for _, style := range surf.Styles {
				if style == 255 {
					break
				}
				if int(style) < 256 && changed[style] {
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

func recompositeDirtySurfaces(rgba []byte, page WorldLightmapPage, values [256]float32) bool {
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
