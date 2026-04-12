package renderer

import (
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/gogpu/gogpu"
)

func (r *Renderer) getCharPic(num int) *image.QPic {
	r.mu.RLock()
	if len(r.concharsData) < 128*128 {
		r.mu.RUnlock()
		return nil
	}
	if r.charCache[num] != nil {
		pic := r.charCache[num]
		r.mu.RUnlock()
		return pic
	}
	r.mu.RUnlock()

	col := num % 16
	row := num / 16
	pixels := make([]byte, 8*8)
	for y := 0; y < 8; y++ {
		src := (row*8+y)*128 + col*8
		copy(pixels[y*8:y*8+8], r.concharsData[src:src+8])
	}
	pic := &image.QPic{Width: 8, Height: 8, Pixels: pixels}

	r.mu.Lock()
	r.charCache[num] = pic
	r.mu.Unlock()
	return pic
}

// getOrCreateCharTexture returns a GPU texture for a character, uploading it if needed.
// Character textures are cached using the character pic via the shared texture cache.
// Uses ConvertConcharsToRGBA so index-0 pixels are transparent.
func (r *Renderer) getOrCreateCharTexture(ctx *gogpu.Context, num int, pic *image.QPic) *gogpu.Texture {
	key := cacheKey{pic: pic}
	r.mu.RLock()
	if entry, ok := r.textureCache[key]; ok {
		r.mu.RUnlock()
		return entry.texture
	}
	r.mu.RUnlock()

	rgba := ConvertConcharsToRGBA(pic.Pixels, r.palette)
	tex, err := ctx.Renderer().NewTextureFromRGBA(int(pic.Width), int(pic.Height), rgba)
	if err != nil {
		slog.Error("getOrCreateCharTexture: upload failed", "num", num, "error", err)
		return nil
	}

	r.mu.Lock()
	r.textureCache[key] = &cachedTexture{texture: tex, width: int(pic.Width), height: int(pic.Height)}
	r.mu.Unlock()
	return tex
}

// getOrCreatePicRGBA returns cached RGBA pixel data for a QPic, converting
// from the Quake palette on first access. Used by the CPU overlay compositor.
func (r *Renderer) getOrCreatePicRGBA(pic *image.QPic) []byte {
	r.mu.RLock()
	if rgba, ok := r.picRGBACache[pic]; ok {
		r.mu.RUnlock()
		return rgba
	}
	r.mu.RUnlock()

	if r.palette == nil || len(pic.Pixels) == 0 {
		return nil
	}
	rgba := ConvertPaletteToRGBA(pic.Pixels, r.palette)

	r.mu.Lock()
	if r.picRGBACache == nil {
		r.picRGBACache = make(map[*image.QPic][]byte)
	}
	r.picRGBACache[pic] = rgba
	r.mu.Unlock()
	return rgba
}

// getOrCreateCharRGBA returns cached RGBA pixel data for a conchars character,
// using ConvertConcharsToRGBA (index 0 = transparent). Used by the CPU overlay.
func (r *Renderer) getOrCreateCharRGBA(pic *image.QPic) []byte {
	r.mu.RLock()
	if rgba, ok := r.picRGBACache[pic]; ok {
		r.mu.RUnlock()
		return rgba
	}
	r.mu.RUnlock()

	if r.palette == nil || len(pic.Pixels) == 0 {
		return nil
	}
	rgba := ConvertConcharsToRGBA(pic.Pixels, r.palette)

	r.mu.Lock()
	if r.picRGBACache == nil {
		r.picRGBACache = make(map[*image.QPic][]byte)
	}
	r.picRGBACache[pic] = rgba
	r.mu.Unlock()
	return rgba
}

func (r *Renderer) SetPalette(palette []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.palette = make([]byte, len(palette))
	copy(r.palette, palette)

	// Invalidate texture cache since palette changed
	r.textureCache = make(map[cacheKey]*cachedTexture)
	for i := range r.colorTextures {
		r.colorTextures[i] = nil
	}
	r.clearAliasModelsLocked()
	r.clearSpriteModelsLocked()
}

func (r *Renderer) getOrCreateTexture(ctx *gogpu.Context, pic *image.QPic) *gogpu.Texture {
	r.mu.RLock()
	cached, ok := r.textureCache[cacheKey{pic: pic}]
	palette := r.palette
	r.mu.RUnlock()

	if ok && cached != nil {
		return cached.texture
	}

	// Convert palette to RGBA
	rgba := ConvertPaletteToRGBA(pic.Pixels, palette)

	// Create texture
	tex, err := ctx.Renderer().NewTextureFromRGBA(int(pic.Width), int(pic.Height), rgba)
	if err != nil {
		slog.Error("Failed to create texture", "error", err)
		return nil
	}

	r.mu.Lock()
	r.textureCache[cacheKey{pic: pic}] = &cachedTexture{
		texture: tex,
		width:   int(pic.Width),
		height:  int(pic.Height),
	}
	r.mu.Unlock()

	return tex
}

func (r *Renderer) getOrCreateColorTexture(ctx *gogpu.Context, color byte) *gogpu.Texture {
	r.mu.RLock()
	tex := r.colorTextures[color]
	palette := r.palette
	r.mu.RUnlock()

	if tex != nil {
		return tex
	}

	// Create 1x1 RGBA texture
	rgba := make([]byte, 4)
	if IsTransparentIndex(color) {
		rgba[0], rgba[1], rgba[2], rgba[3] = 0, 0, 0, 0
	} else {
		pr, pg, pb := GetPaletteColor(color, palette)
		rgba[0], rgba[1], rgba[2], rgba[3] = pr, pg, pb, 255
	}

	newTex, err := ctx.Renderer().NewTextureFromRGBA(1, 1, rgba)
	if err != nil {
		slog.Error("Failed to create color texture", "error", err)
		return nil
	}

	r.mu.Lock()
	r.colorTextures[color] = newTex
	r.mu.Unlock()

	return newTex
}

// OnDraw sets the callback for frame rendering.
// The callback is called each frame with a DrawContext for drawing operations.
//
// Example:
//
//	r.OnDraw(func(dc *renderer.DrawContext) {
//	    dc.Clear(0.1, 0.1, 0.1, 1.0)
//	    // Draw world geometry...
