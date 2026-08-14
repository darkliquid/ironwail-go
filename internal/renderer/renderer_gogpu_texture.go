package renderer

import (
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/draw"
	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/gogpu/gogpu"
)

func (r *Renderer) effectivePalette() []byte {
	if r == nil {
		return draw.DefaultQuakePalette()
	}
	// Lock-free snapshot. Callers may already hold r.mu (e.g. alias skin
	// creation runs under the renderer write lock during a browser draw), so
	// taking the RWMutex here would self-deadlock. The palette is 768 bytes
	// and only replaced wholesale under the same lock; the atomic snapshot
	// is either the old or the new buffer, both complete.
	if p := r.paletteAtomic.Load(); p != nil {
		pal := *p
		if len(pal) >= 768 && !isAllZeros(pal) {
			return pal
		}
	}
	return draw.DefaultQuakePalette()
}

func isAllZeros(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

// uploadRGBAThruGogpu uploads RGBA pixels through gogpu's texture path.
func uploadRGBAThruGogpu(ctx *gogpu.Context, width, height int, rgba []byte) (*gogpu.Texture, error) {
	return ctx.Renderer().NewTextureFromRGBA(width, height, rgba)
}

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
// Uses ConvertConcharsToRGBA so index-0 pixels are transparent.
func (r *Renderer) getOrCreateCharTexture(ctx *gogpu.Context, num int, pic *image.QPic) *gogpu.Texture {
	if num < 0 || num > 255 {
		return nil
	}
	r.mu.RLock()
	tex := r.charTextures[num]
	r.mu.RUnlock()
	palette := r.effectivePalette()

	if tex != nil {
		return tex
	}

	rgba := ConvertConcharsToRGBA(pic.Pixels, palette)
	newTex, err := uploadRGBAThruGogpu(ctx, int(pic.Width), int(pic.Height), rgba)
	if err != nil {
		slog.Error("getOrCreateCharTexture: upload failed", "num", num, "error", err)
		return nil
	}

	r.mu.Lock()
	if r.charTextures[num] != nil {
		newTex.Destroy()
		tex = r.charTextures[num]
	} else {
		r.charTextures[num] = newTex
		tex = newTex
	}
	r.mu.Unlock()

	return tex
}

func (r *Renderer) SetPalette(palette []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.palette = make([]byte, len(palette))
	copy(r.palette, palette)
	r.paletteAtomic.Store(&r.palette)

	// Destroy and invalidate texture cache entries to prevent GPU memory leaks
	for _, entry := range r.textureCache {
		if entry != nil && entry.texture != nil {
			entry.texture.Destroy()
		}
	}
	r.textureCache = make(map[cacheKey]*cachedTexture)

	for i, tex := range r.colorTextures {
		if tex != nil {
			tex.Destroy()
			r.colorTextures[i] = nil
		}
	}

	for i, tex := range r.charTextures {
		if tex != nil {
			tex.Destroy()
			r.charTextures[i] = nil
		}
	}

	r.clearAliasModelsLocked()
	r.clearSpriteModelsLocked()
}

func (r *Renderer) getOrCreateTexture(ctx *gogpu.Context, pic *image.QPic) *gogpu.Texture {
	r.mu.RLock()
	cached, ok := r.textureCache[cacheKey{pic: pic}]
	r.mu.RUnlock()
	palette := r.effectivePalette()

	if ok && cached != nil {
		return cached.texture
	}

	// Convert palette to RGBA
	rgba := ConvertPaletteToRGBA(pic.Pixels, palette)

	// Create texture
	tex, err := uploadRGBAThruGogpu(ctx, int(pic.Width), int(pic.Height), rgba)
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
	r.mu.RUnlock()
	palette := r.effectivePalette()

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

	newTex, err := uploadRGBAThruGogpu(ctx, 1, 1, rgba)
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
