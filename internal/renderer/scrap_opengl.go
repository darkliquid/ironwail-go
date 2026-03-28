//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"unsafe"

	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/go-gl/gl/v4.6-core/gl"
)

// initScrapAtlas creates the atlas (256×256 pages) used for small UI textures.
func (r *Renderer) initScrapAtlas() {
	r.scrapAtlas = NewScrapAtlas(256, 256)
	r.scrapEntries = make(map[glCacheKey]*ScrapEntry)
}

// tryScrapAlloc attempts to pack a small QPic into the scrap atlas using regular palette conversion.
func (r *Renderer) tryScrapAlloc(dc *glDrawContext, pic *image.QPic) (*ScrapEntry, uint32, bool) {
	return r.tryScrapAllocWithConverter(dc, pic, ConvertPaletteToRGBA)
}

// tryScrapAllocConchars attempts to pack a small conchars QPic into the scrap atlas.
func (r *Renderer) tryScrapAllocConchars(dc *glDrawContext, pic *image.QPic) (*ScrapEntry, uint32, bool) {
	return r.tryScrapAllocWithConverter(dc, pic, ConvertConcharsToRGBA)
}

func (r *Renderer) tryScrapAllocWithConverter(dc *glDrawContext, pic *image.QPic, convert func([]byte, []byte) []byte) (*ScrapEntry, uint32, bool) {
	if pic == nil || r.scrapAtlas == nil {
		return nil, 0, false
	}
	if pic.Width > 128 || pic.Height > 128 {
		return nil, 0, false
	}

	key := glCacheKey{pic: pic}
	r.mu.RLock()
	entry, ok := r.scrapEntries[key]
	palette := append([]byte(nil), r.palette...)
	r.mu.RUnlock()
	if ok && entry != nil {
		tex := r.ensureScrapPageTexture(dc, entry.PageIndex)
		if tex == 0 {
			return nil, 0, false
		}
		return entry, tex, true
	}

	entry, err := r.scrapAtlas.Alloc(int(pic.Width), int(pic.Height))
	if err != nil {
		return nil, 0, false
	}

	rgba := convert(pic.Pixels, palette)
	r.scrapAtlas.Upload(entry, rgba)

	r.mu.Lock()
	if existing := r.scrapEntries[key]; existing != nil {
		r.mu.Unlock()
		tex := r.ensureScrapPageTexture(dc, existing.PageIndex)
		if tex == 0 {
			return nil, 0, false
		}
		return existing, tex, true
	}
	r.scrapEntries[key] = entry
	r.mu.Unlock()

	tex := r.ensureScrapPageTexture(dc, entry.PageIndex)
	if tex == 0 {
		return nil, 0, false
	}
	return entry, tex, true
}

// ensureScrapPageTexture creates or updates a GL texture for the atlas page.
func (r *Renderer) ensureScrapPageTexture(dc *glDrawContext, pageIndex int) uint32 {
	if dc == nil || r.scrapAtlas == nil {
		return 0
	}
	for len(r.scrapTextures) <= pageIndex {
		r.scrapTextures = append(r.scrapTextures, 0)
	}

	page := r.scrapAtlas.Page(pageIndex)
	if page == nil || len(page.Pixels) == 0 {
		return 0
	}

	if r.scrapTextures[pageIndex] == 0 {
		var tex uint32
		gl.GenTextures(1, &tex)
		gl.BindTexture(gl.TEXTURE_2D, tex)
		gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
		gl.TexImage2D(
			gl.TEXTURE_2D,
			0,
			gl.RGBA,
			int32(page.Width),
			int32(page.Height),
			0,
			gl.RGBA,
			gl.UNSIGNED_BYTE,
			unsafe.Pointer(&page.Pixels[0]),
		)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
		r.scrapTextures[pageIndex] = tex
		r.scrapAtlas.ClearDirty(pageIndex)
		return tex
	}

	if page.Dirty {
		gl.BindTexture(gl.TEXTURE_2D, r.scrapTextures[pageIndex])
		gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
		gl.TexSubImage2D(
			gl.TEXTURE_2D,
			0,
			0,
			0,
			int32(page.Width),
			int32(page.Height),
			gl.RGBA,
			gl.UNSIGNED_BYTE,
			unsafe.Pointer(&page.Pixels[0]),
		)
		r.scrapAtlas.ClearDirty(pageIndex)
	}

	return r.scrapTextures[pageIndex]
}

func (r *Renderer) scrapTextureForPage(pageIndex int) uint32 {
	if pageIndex >= 0 && pageIndex < len(r.scrapTextures) {
		return r.scrapTextures[pageIndex]
	}
	return 0
}

// uploadDirtyScrapPages uploads any modified scrap pages to the GPU.
func (r *Renderer) uploadDirtyScrapPages(dc *glDrawContext) {
	if r.scrapAtlas == nil || dc == nil {
		return
	}
	for _, idx := range r.scrapAtlas.DirtyPages() {
		r.ensureScrapPageTexture(dc, idx)
	}
}

// destroyScrapAtlas deletes all GL textures used by the scrap atlas.
func (r *Renderer) destroyScrapAtlas() {
	for i, tex := range r.scrapTextures {
		if tex != 0 {
			gl.DeleteTextures(1, &tex)
			r.scrapTextures[i] = 0
		}
	}
	r.scrapTextures = nil
	r.scrapAtlas = nil
	r.scrapEntries = nil
}
