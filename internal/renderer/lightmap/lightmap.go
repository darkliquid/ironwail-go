// Package lightmap implements the CPU-side lightmap atlas helpers for the
// renderer: page compositing, dirtied-region tracking, region extraction and
// the vertical page-stacking layout used to work around the gogpu Vulkan
// WriteTexture BaseArrayLayer bug.
//
// Original C lineage: GL_PackLitSurfaces / GL_FillSurfaceLightmap in
// gl_lightmap.c, gl_rsurf.c (Ironwail). The Go renderer splits lightmap work
// from world geometry; this package is the leaf that knows nothing about
// wgpu resources.
package lightmap

import (
	"unsafe"

	"github.com/darkliquid/ironwail-go/internal/renderer/world"
)

// StackPages builds a single tall RGBA buffer with every page stacked
// vertically. pageSize is the page width/height in pixels (constant
// worldLightmapPageSize in the parent renderer). Each page is padded with a
// 1-pixel replicated border above and below so linear filtering does not
// bleed between adjacent pages inside the packed 2D texture.
func StackPages(pages []world.WorldLightmapPage, values [256]float32, pageSize int) []byte {
	if len(pages) == 0 || pageSize <= 0 {
		return nil
	}
	height := pages[0].Height
	if height <= 0 {
		return nil
	}

	rowsPerPage := height + 2
	totalHeight := rowsPerPage * len(pages)
	totalPixels := pageSize * totalHeight * 4
	rgba := make([]byte, totalPixels)
	srcStride := pageSize * 4
	for i := range pages {
		pageRGBA := CompositePageRGBA(&pages[i], values)
		if len(pageRGBA) == 0 {
			continue
		}
		// Page content starts at row (i * rowsPerPage + 1).
		yOffset := rowsPerPage * i
		contentY := yOffset + 1
		for row := 0; row < height; row++ {
			srcStart := row * srcStride
			dstStart := (contentY + row) * srcStride
			copy(rgba[dstStart:dstStart+srcStride], pageRGBA[srcStart:srcStart+srcStride])
		}
		// Replicate top edge row into padding row above.
		if height > 0 {
			topRowStart := 0
			padRowStart := yOffset * srcStride
			copy(rgba[padRowStart:padRowStart+srcStride], pageRGBA[topRowStart:topRowStart+srcStride])
			// Replicate bottom edge row into padding row below.
			botRowStart := (height - 1) * srcStride
			padBotStart := (contentY + height) * srcStride
			copy(rgba[padBotStart:padBotStart+srcStride], pageRGBA[botRowStart:botRowStart+srcStride])
		}
	}
	return rgba
}

// DirtyBounds returns the bounding box of all dirty surfaces on a page, or
// an empty (0,0,0,0) box when nothing is dirty.
func DirtyBounds(page world.WorldLightmapPage) (x, y, w, h int) {
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

// ExtractRegionRGBA copies the (x,y,w,h) region from a full page RGBA buffer
// into dst, reusing dst when its capacity already matches. It returns nil
// when the region is empty.
func ExtractRegionRGBA(dst []byte, rgba []byte, srcWidth, x, y, w, h int) []byte {
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

// MarkDirtyLightmapPages flags every page (and the surfaces on it) whose
// lightstyles changed, so the next upload recomposites only those surfaces.
func MarkDirtyLightmapPages(pages []world.WorldLightmapPage, changed [256]bool) {
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

// ClearDirtyFlags resets the dirty flags on all pages and surfaces after an
// upload has consumed them.
func ClearDirtyFlags(pages []world.WorldLightmapPage) {
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

// RecompositeDirtySurfaces re-composites only the dirty surfaces of a page
// into an existing RGBA buffer. It reports whether any surface was rewritten.
func RecompositeDirtySurfaces(rgba []byte, page world.WorldLightmapPage, values [256]float32) bool {
	recomposited := false
	for _, surface := range page.Surfaces {
		if !surface.Dirty {
			continue
		}
		CompositeSurfaceRGBA(rgba, page.Width, surface, values)
		recomposited = true
	}
	return recomposited
}

// CompositePageRGBA renders a full page's lightmap into an RGBA buffer,
// reusing the cached buffer when present. A fresh buffer is zero-filled with
// alpha=255 and all non-dirty surfaces composited; a dirty page only
// recomposites its dirty surfaces in place.
func CompositePageRGBA(page *world.WorldLightmapPage, values [256]float32) []byte {
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
			CompositeSurfaceRGBA(page.CachedRGBA, page.Width, surface, values)
		}
		return page.CachedRGBA
	}
	if page.Dirty {
		RecompositeDirtySurfaces(page.CachedRGBA, *page, values)
	}
	return page.CachedRGBA
}

// CompositeSurfaceRGBA composites one lightmap surface's samples into the
// page buffer at the surface's position, applying lightstyle scales with
// fast paths for 1, 2, and 3-4 style counts. Samples are RGB triplets; the
// buffer is RGBA.
func CompositeSurfaceRGBA(rgba []byte, pageWidth int, surface world.WorldLightmapSurface, values [256]float32) {
	if surface.Width <= 0 || surface.Height <= 0 {
		return
	}
	styleCount := 0
	var scales [4]float32
	for i, style := range surface.Styles {
		if style == 255 {
			break
		}
		if int(style) < len(values) {
			scales[i] = values[style]
		}
		styleCount++
	}
	if styleCount == 0 {
		styleCount = 1
		scales[0] = values[0]
	}

	faceSize := surface.Width * surface.Height * 3
	if len(surface.Samples) < faceSize*styleCount {
		return
	}

	// Fast path 1: Single style, scale == 1.0 (static lighting, 90%+ of surfaces)
	if styleCount == 1 && scales[0] == 1.0 {
		for y := 0; y < surface.Height; y++ {
			srcRow := (y * surface.Width) * 3
			dstRow := ((surface.Y+y)*pageWidth + surface.X) * 4
			for x := 0; x < surface.Width; x++ {
				src := srcRow + x*3
				dst := dstRow + x*4
				rgba[dst] = surface.Samples[src]
				rgba[dst+1] = surface.Samples[src+1]
				rgba[dst+2] = surface.Samples[src+2]
				rgba[dst+3] = 255
			}
		}
		return
	}

	// Fast path 2: Single style, scale != 1.0
	if styleCount == 1 {
		s0 := scales[0]
		for y := 0; y < surface.Height; y++ {
			srcRow := (y * surface.Width) * 3
			dstRow := ((surface.Y+y)*pageWidth + surface.X) * 4
			for x := 0; x < surface.Width; x++ {
				src := srcRow + x*3
				dst := dstRow + x*4
				rVal := float32(surface.Samples[src]) * s0
				gVal := float32(surface.Samples[src+1]) * s0
				bVal := float32(surface.Samples[src+2]) * s0
				if rVal > 255 {
					rVal = 255
				}
				if gVal > 255 {
					gVal = 255
				}
				if bVal > 255 {
					bVal = 255
				}
				rgba[dst] = byte(rVal)
				rgba[dst+1] = byte(gVal)
				rgba[dst+2] = byte(bVal)
				rgba[dst+3] = 255
			}
		}
		return
	}

	// Fast path 3: 2 styles
	if styleCount == 2 {
		s0, s1 := scales[0], scales[1]
		for y := 0; y < surface.Height; y++ {
			srcRow := (y * surface.Width) * 3
			dstRow := ((surface.Y+y)*pageWidth + surface.X) * 4
			for x := 0; x < surface.Width; x++ {
				src0 := srcRow + x*3
				src1 := src0 + faceSize
				dst := dstRow + x*4
				rVal := float32(surface.Samples[src0])*s0 + float32(surface.Samples[src1])*s1
				gVal := float32(surface.Samples[src0+1])*s0 + float32(surface.Samples[src1+1])*s1
				bVal := float32(surface.Samples[src0+2])*s0 + float32(surface.Samples[src1+2])*s1
				if rVal > 255 {
					rVal = 255
				}
				if gVal > 255 {
					gVal = 255
				}
				if bVal > 255 {
					bVal = 255
				}
				rgba[dst] = byte(rVal)
				rgba[dst+1] = byte(gVal)
				rgba[dst+2] = byte(bVal)
				rgba[dst+3] = 255
			}
		}
		return
	}

	// General path: 3 or 4 styles
	for y := 0; y < surface.Height; y++ {
		srcRow := (y * surface.Width) * 3
		dstRow := ((surface.Y+y)*pageWidth + surface.X) * 4
		for x := 0; x < surface.Width; x++ {
			sampleIndex := srcRow + x*3
			var rSum, gSum, bSum float32
			for styleIndex := 0; styleIndex < styleCount; styleIndex++ {
				offset := styleIndex*faceSize + sampleIndex
				scale := scales[styleIndex]
				rSum += float32(surface.Samples[offset]) * scale
				gSum += float32(surface.Samples[offset+1]) * scale
				bSum += float32(surface.Samples[offset+2]) * scale
			}
			dst := dstRow + x*4
			if rSum > 255 {
				rSum = 255
			}
			if gSum > 255 {
				gSum = 255
			}
			if bSum > 255 {
				bSum = 255
			}
			rgba[dst] = byte(rSum)
			rgba[dst+1] = byte(gSum)
			rgba[dst+2] = byte(bSum)
			rgba[dst+3] = 255
		}
	}
}

// DefaultStyleValues returns a lightstyle table where only style 0 is lit.
func DefaultStyleValues() [256]float32 {
	var values [256]float32
	values[0] = 1
	return values
}

// LightstyleScale returns the scale for a lightstyle index, clamping out of
// range styles to 0.
func LightstyleScale(values [256]float32, style uint8) float32 {
	if int(style) >= len(values) {
		return 0
	}
	return values[style]
}

// StylesChanged returns per-index flags for every lightstyle whose value
// differs between the old and new tables.
func StylesChanged(old, new_ [256]float32) [256]bool {
	var changed [256]bool
	for i := range old {
		if old[i] != new_[i] {
			changed[i] = true
		}
	}
	return changed
}

// AnyStyleChanged reports whether at least one lightstyle changed.
func AnyStyleChanged(changed [256]bool) bool {
	for _, dirty := range changed {
		if dirty {
			return true
		}
	}
	return false
}

// Float32ToBytes reinterprets a []float32 as bytes without copying.
func Float32ToBytes(f []float32) []byte {
	if len(f) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(f))), len(f)*4)
}

// Uint32SliceToBytes reinterprets a []uint32 as bytes without copying.
func Uint32SliceToBytes(values []uint32) []byte {
	if len(values) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(values))), len(values)*4)
}
