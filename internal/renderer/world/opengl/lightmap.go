//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package opengl

import (
	"github.com/go-gl/gl/v4.6-core/gl"
	worldimpl "github.com/ironwail/ironwail-go/internal/renderer/world"
	"unsafe"
)

type LightmapTextureUploader func(width, height int, rgba []byte) uint32

func LightStylesChanged(old, new_ [64]float32) [64]bool {
	var changed [64]bool
	for i := range old {
		if old[i] != new_[i] {
			changed[i] = true
		}
	}
	return changed
}

func MarkDirtyLightmapPages(pages []worldimpl.WorldLightmapPage, changed [64]bool) {
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

func ClearDirtyFlags(pages []worldimpl.WorldLightmapPage) {
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

func CompositeSurfaceRGBA(rgba []byte, pageWidth int, surface worldimpl.WorldLightmapSurface, values [64]float32) {
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
				scale := lightstyleScale(values, surface.Styles[styleIndex])
				rSum += float32(surface.Samples[offset]) * scale
				gSum += float32(surface.Samples[offset+1]) * scale
				bSum += float32(surface.Samples[offset+2]) * scale
			}
			dst := ((surface.Y+y)*pageWidth + (surface.X + x)) * 4
			rgba[dst] = byte(clamp01(rSum/255.0) * 255)
			rgba[dst+1] = byte(clamp01(gSum/255.0) * 255)
			rgba[dst+2] = byte(clamp01(bSum/255.0) * 255)
		}
	}
}

func BuildLightmapPageRGBA(page *worldimpl.WorldLightmapPage, values [64]float32) []byte {
	if page.Width <= 0 || page.Height <= 0 {
		return nil
	}
	rgba := make([]byte, page.Width*page.Height*4)
	for i := 0; i < len(rgba); i += 4 {
		rgba[i] = 255
		rgba[i+1] = 255
		rgba[i+2] = 255
		rgba[i+3] = 255
	}

	for _, surface := range page.Surfaces {
		CompositeSurfaceRGBA(rgba, page.Width, surface, values)
	}
	return rgba
}

func RecompositeDirtySurfaces(rgba []byte, page worldimpl.WorldLightmapPage, values [64]float32) bool {
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

func DirtyBounds(page worldimpl.WorldLightmapPage) (x, y, w, h int) {
	minX, minY := page.Width, page.Height
	maxX, maxY := 0, 0
	found := false
	for _, s := range page.Surfaces {
		if !s.Dirty || s.Width <= 0 || s.Height <= 0 {
			continue
		}
		if s.X < minX {
			minX = s.X
		}
		if s.Y < minY {
			minY = s.Y
		}
		if s.X+s.Width > maxX {
			maxX = s.X + s.Width
		}
		if s.Y+s.Height > maxY {
			maxY = s.Y + s.Height
		}
		found = true
	}
	if !found {
		return 0, 0, 0, 0
	}
	return minX, minY, maxX - minX, maxY - minY
}

func UploadLightmapPages(pages []worldimpl.WorldLightmapPage, values [64]float32, uploadTexture LightmapTextureUploader) []uint32 {
	textures := make([]uint32, 0, len(pages))
	for i := range pages {
		rgba := BuildLightmapPageRGBA(&pages[i], values)
		if len(rgba) == 0 {
			continue
		}
		textures = append(textures, uploadTexture(pages[i].Width, pages[i].Height, rgba))
	}
	return textures
}

func UpdateLightmapTextures(textures []uint32, pages []worldimpl.WorldLightmapPage, values [64]float32) {
	count := len(textures)
	if len(pages) < count {
		count = len(pages)
	}
	for i := 0; i < count; i++ {
		if textures[i] == 0 || !pages[i].Dirty {
			continue
		}

		rgba := BuildLightmapPageRGBA(&pages[i], values)
		if len(rgba) == 0 {
			continue
		}
		gl.BindTexture(gl.TEXTURE_2D, textures[i])
		withPinnedPixelData(rgba, func(ptr unsafe.Pointer) {
			gl.TexSubImage2D(gl.TEXTURE_2D, 0, 0, 0, int32(pages[i].Width), int32(pages[i].Height), gl.RGBA, gl.UNSIGNED_BYTE, ptr)
		})
	}
	if count > 0 {
		gl.BindTexture(gl.TEXTURE_2D, 0)
	}
	ClearDirtyFlags(pages)
}

func lightstyleScale(values [64]float32, style uint8) float32 {
	if int(style) >= len(values) {
		return 0
	}
	return values[style]
}

func clamp01(v float32) float32 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}
