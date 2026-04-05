//go:build gogpu && !cgo
// +build gogpu,!cgo

// Package renderer provides GPU-accelerated rendering for the Ironwail-Go engine.
// It wraps the gogpu library to provide a Quake-specific rendering interface
// with support for window management, video modes, and basic drawing operations.
//
// The renderer follows the Quake engine architecture where rendering is tightly
// integrated with the host system and console variable (cvar) system. Video
// settings like resolution, fullscreen mode, and vsync are controlled via cvars.
//
// Architecture Overview:
//
//	The renderer is built on top of gogpu, which provides:
//	- Pure Go WebGPU implementation (no CGO required)
//	- Cross-platform windowing (Linux/X11/Wayland, macOS/Metal, Windows/DX12/Vulkan)
//	- Event-driven rendering with game loop support
//	- Hardware-accelerated graphics via Vulkan/Metal/DX12/GLES
//
// Key Components:
//
//   - Renderer: Main rendering context that owns the gogpu App and window
//   - Config: Video configuration derived from cvars and system capabilities
//   - DrawContext: Frame-specific drawing operations (passed to OnDraw callbacks)
//
// Usage:
//
//	// Create renderer with video cvars
//	r, err := renderer.New()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer r.Shutdown()
//
//	// Set up render callback
//	r.OnDraw(func(dc *renderer.DrawContext) {
//	    dc.Clear(color.Black)
//	    // ... draw game world ...
//	})
//
//	// Run the main loop
//	if err := r.Run(); err != nil {
//	    log.Fatal(err)
//	}
package renderer

import (
	"fmt"
	stdimage "image"
	"image/color"
	"image/png"
	"log/slog"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/pkg/types"
	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gmath" // retained only for gogpu API boundary (Color type)
	"github.com/gogpu/gogpu/input"
	"github.com/gogpu/wgpu"
)

// DrawContext provides frame-specific rendering operations.
// It is passed to the OnDraw callback and provides access to
// the current frame's rendering state.
//
// DrawContext wraps gogpu.Context and adds Quake-specific functionality
// like clear colors, gamma correction, and convenient drawing methods.
type DrawContext struct {
	// ctx is the underlying gogpu rendering context.
	ctx *gogpu.Context

	// gamma is the current gamma correction value.
	gamma float32

	// renderer is the parent Renderer instance.
	renderer *Renderer

	// Canvas coordinate system state.
	canvas            CanvasState
	canvasParams      CanvasTransformParams
	sceneRenderActive bool
	sceneRenderTarget *wgpu.TextureView

	// overlay is the CPU-side 2D compositor buffer. All 2D draw calls
	// (DrawPic, DrawFill, DrawCharacter, DrawString) composite into this
	// buffer at screen resolution instead of issuing individual GPU
	// submissions through gogpu's 2D API. The overlay is flushed as a
	// single texture upload + draw at the end of the 2D overlay phase.
	overlay *overlay2D
}

func validatedGoGPURenderPipeline(device *wgpu.Device, desc *wgpu.RenderPipelineDescriptor) (*wgpu.RenderPipeline, error) {
	if device == nil {
		return nil, fmt.Errorf("nil device")
	}
	if desc == nil {
		return nil, fmt.Errorf("nil render pipeline descriptor")
	}
	slog.Debug("Creating GPU Render Pipeline", "label", desc.Label, "vertex shader", fmt.Sprintf("%p", desc.Vertex.Module), "fragment shader", fmt.Sprintf("%p", desc.Fragment))
	return device.CreateRenderPipeline(desc)
}

var halOnlyFrameConsumed atomic.Bool

// overlay2D is a CPU-side RGBA compositor buffer at screen resolution.
// Instead of issuing one GPU command encoder + render pass + submit per 2D
// draw call (which is what gogpu's DrawTextureScaled/DrawTextureEx does),
// all 2D draws composite into this buffer on the CPU. The buffer is then
// uploaded as a single GPU texture and drawn in one submit at the end of
// the 2D overlay phase.
type overlay2D struct {
	pixels []byte // RGBA at screen resolution (straight alpha)
	width  int
	height int
	dirty  bool
}

// ensureOverlay initializes or resets the CPU overlay compositor at the
// current screen resolution. Called lazily on the first 2D draw each frame.
// Reuses a pooled pixel buffer from the Renderer to avoid ~4.5MB allocation
// per frame.
func (dc *DrawContext) ensureOverlay() *overlay2D {
	if dc.overlay != nil {
		return dc.overlay
	}
	w, h := dc.renderer.Size()
	if w <= 0 || h <= 0 {
		return nil
	}
	needed := w * h * 4
	r := dc.renderer
	r.mu.Lock()
	// Reuse pooled buffer if dimensions match, otherwise allocate.
	if r.overlayBufWidth == w && r.overlayBufHeight == h && len(r.overlayPixelBuf) == needed {
		clear(r.overlayPixelBuf)
	} else {
		r.overlayPixelBuf = make([]byte, needed)
		r.overlayBufWidth = w
		r.overlayBufHeight = h
	}
	buf := r.overlayPixelBuf
	r.mu.Unlock()
	dc.overlay = &overlay2D{
		pixels: buf,
		width:  w,
		height: h,
	}
	return dc.overlay
}

// flush2DOverlay uploads the CPU overlay buffer as a single GPU texture and
// draws it onto the current surface in one submit. Reuses a cached GPU
// texture when dimensions match to avoid per-frame CreateTexture overhead.
func (dc *DrawContext) flush2DOverlay() {
	ov := dc.overlay
	if ov == nil || !ov.dirty {
		dc.overlay = nil
		return
	}
	r := dc.renderer
	r.mu.Lock()
	// Reuse cached GPU texture if dimensions match.
	if r.overlayTexture != nil && r.overlayTextureWidth == ov.width && r.overlayTextureHeight == ov.height {
		tex := r.overlayTexture
		r.mu.Unlock()
		if err := tex.UpdateData(ov.pixels); err != nil {
			slog.Error("flush2DOverlay: texture update failed", "error", err)
			dc.overlay = nil
			return
		}
		if err := dc.ctx.DrawTextureScaled(tex, 0, 0, float32(ov.width), float32(ov.height)); err != nil {
			slog.Error("flush2DOverlay: draw failed", "error", err)
		}
	} else {
		// Dimensions changed or first frame — create new texture.
		if r.overlayTexture != nil {
			r.overlayTexture.Destroy()
			r.overlayTexture = nil
		}
		r.mu.Unlock()
		tex, err := dc.ctx.Renderer().NewTextureFromRGBA(ov.width, ov.height, ov.pixels)
		if err != nil {
			slog.Error("flush2DOverlay: texture upload failed", "error", err)
			dc.overlay = nil
			return
		}
		r.mu.Lock()
		r.overlayTexture = tex
		r.overlayTextureWidth = ov.width
		r.overlayTextureHeight = ov.height
		r.mu.Unlock()
		if err := dc.ctx.DrawTextureScaled(tex, 0, 0, float32(ov.width), float32(ov.height)); err != nil {
			slog.Error("flush2DOverlay: draw failed", "error", err)
		}
	}
	dc.overlay = nil
}

// overlayFillRect fills a rectangle in the overlay buffer with an RGBA color.
func (ov *overlay2D) fillRect(x, y, w, h int, r, g, b, a byte) {
	// Clip to overlay bounds.
	if x < 0 {
		w += x
		x = 0
	}
	if y < 0 {
		h += y
		y = 0
	}
	if x+w > ov.width {
		w = ov.width - x
	}
	if y+h > ov.height {
		h = ov.height - y
	}
	if w <= 0 || h <= 0 {
		return
	}
	ov.dirty = true
	stride := ov.width * 4
	if a == 255 {
		// Fast path: opaque fill, no blending needed.
		for dy := 0; dy < h; dy++ {
			off := (y+dy)*stride + x*4
			for dx := 0; dx < w; dx++ {
				ov.pixels[off] = r
				ov.pixels[off+1] = g
				ov.pixels[off+2] = b
				ov.pixels[off+3] = 255
				off += 4
			}
		}
	} else {
		// Alpha blend (source over).
		sa := uint32(a)
		invSA := 255 - sa
		for dy := 0; dy < h; dy++ {
			off := (y+dy)*stride + x*4
			for dx := 0; dx < w; dx++ {
				dr := uint32(ov.pixels[off])
				dg := uint32(ov.pixels[off+1])
				db := uint32(ov.pixels[off+2])
				da := uint32(ov.pixels[off+3])
				ov.pixels[off] = byte((uint32(r)*sa + dr*invSA) / 255)
				ov.pixels[off+1] = byte((uint32(g)*sa + dg*invSA) / 255)
				ov.pixels[off+2] = byte((uint32(b)*sa + db*invSA) / 255)
				ov.pixels[off+3] = byte((sa*255 + da*invSA) / 255)
				off += 4
			}
		}
	}
}

// overlayBlitRGBA blits an RGBA source image into the overlay with
// nearest-neighbor scaling and alpha compositing.
func (ov *overlay2D) blitRGBA(srcRGBA []byte, srcW, srcH int, dstX, dstY, dstW, dstH int, alpha float32) {
	if len(srcRGBA) < srcW*srcH*4 || dstW <= 0 || dstH <= 0 {
		return
	}
	// Clip destination to overlay bounds.
	sx0, sy0 := 0, 0
	if dstX < 0 {
		sx0 = -dstX * srcW / dstW
		dstW += dstX
		dstX = 0
	}
	if dstY < 0 {
		sy0 = -dstY * srcH / dstH
		dstH += dstY
		dstY = 0
	}
	if dstX+dstW > ov.width {
		dstW = ov.width - dstX
	}
	if dstY+dstH > ov.height {
		dstH = ov.height - dstY
	}
	if dstW <= 0 || dstH <= 0 {
		return
	}
	ov.dirty = true
	stride := ov.width * 4
	globalAlpha := uint32(alpha * 255)
	if globalAlpha > 255 {
		globalAlpha = 255
	}
	for dy := 0; dy < dstH; dy++ {
		srcY := sy0 + dy*srcH/dstH
		if srcY >= srcH {
			srcY = srcH - 1
		}
		dstOff := (dstY+dy)*stride + dstX*4
		for dx := 0; dx < dstW; dx++ {
			srcX := sx0 + dx*srcW/dstW
			if srcX >= srcW {
				srcX = srcW - 1
			}
			srcOff := (srcY*srcW + srcX) * 4
			sr := uint32(srcRGBA[srcOff])
			sg := uint32(srcRGBA[srcOff+1])
			sb := uint32(srcRGBA[srcOff+2])
			sa := uint32(srcRGBA[srcOff+3]) * globalAlpha / 255

			if sa == 0 {
				dstOff += 4
				continue
			}
			if sa == 255 {
				ov.pixels[dstOff] = byte(sr)
				ov.pixels[dstOff+1] = byte(sg)
				ov.pixels[dstOff+2] = byte(sb)
				ov.pixels[dstOff+3] = 255
			} else {
				invSA := 255 - sa
				dr := uint32(ov.pixels[dstOff])
				dg := uint32(ov.pixels[dstOff+1])
				db := uint32(ov.pixels[dstOff+2])
				da := uint32(ov.pixels[dstOff+3])
				ov.pixels[dstOff] = byte((sr*sa + dr*invSA) / 255)
				ov.pixels[dstOff+1] = byte((sg*sa + dg*invSA) / 255)
				ov.pixels[dstOff+2] = byte((sb*sa + db*invSA) / 255)
				ov.pixels[dstOff+3] = byte((sa*255 + da*invSA) / 255)
			}
			dstOff += 4
		}
	}
}

// Clear fills the screen with the specified RGBA color.
// This is typically called at the start of each frame to clear
// the previous frame's content.
//
// In Quake, the clear color is typically a dark gray or black,
// but can be adjusted for different visual effects.
func (dc *DrawContext) Clear(r, g, b, a float32) {
	// Use gogpu's proper clear operation
	// Convert to gmath.Color at the gogpu API boundary.
	dc.ctx.ClearColor(gmath.Color{R: r, G: g, B: b, A: a})
}

// DrawTriangle renders a simple colored triangle.
// This is primarily useful for testing the rendering pipeline.
// In a full implementation, this would be replaced with proper
// 3D geometry rendering using shaders.
func (dc *DrawContext) DrawTriangle(r, g, b, a float32) {
	// Convert to gmath.Color at the gogpu API boundary.
	dc.ctx.DrawTriangleColor(gmath.Color{R: r, G: g, B: b, A: a})
}

// SurfaceView returns the current frame's GPU texture view.
// This enables zero-copy rendering for advanced use cases
// like post-processing effects and compositing.
//
// In Quake, this would be used for:
//   - Rendering the 3D world to a texture
//   - Post-processing effects (bloom, motion blur)
//   - UI overlay rendering
func (dc *DrawContext) SurfaceView() interface{} {
	return dc.ctx.SurfaceView()
}

// Gamma returns the current gamma correction value.
func (dc *DrawContext) Gamma() float32 {
	return dc.gamma
}

// SetCanvas switches the active 2D canvas coordinate system.
func (dc *DrawContext) SetCanvas(ct CanvasType) {
	if dc == nil {
		return
	}
	params := dc.canvasParams
	if dc.renderer != nil {
		screenW, screenH := dc.renderer.Size()
		if params.GUIWidth <= 0 {
			params.GUIWidth = float32(screenW)
		}
		if params.GUIHeight <= 0 {
			params.GUIHeight = float32(screenH)
		}
		if params.GLWidth <= 0 {
			params.GLWidth = float32(screenW)
		}
		if params.GLHeight <= 0 {
			params.GLHeight = float32(screenH)
		}
	}
	if params.ConWidth <= 0 {
		params.ConWidth = params.GUIWidth
	}
	if params.ConHeight <= 0 {
		params.ConHeight = params.GUIHeight
	}
	if params.GUIWidth <= 0 || params.GUIHeight <= 0 || params.GLWidth <= 0 || params.GLHeight <= 0 {
		dc.canvas.Type = ct
		return
	}
	SetCanvas(&dc.canvas, ct, params)
}

// Canvas returns the current canvas state.
func (dc *DrawContext) Canvas() CanvasState {
	return dc.canvas
}

// SetCanvasParams updates the per-frame canvas transform parameters.
func (dc *DrawContext) SetCanvasParams(p CanvasTransformParams) {
	dc.canvasParams = p
	dc.canvas.Type = CanvasNone
}

// 2D Drawing API implementation
//
// All 2D draws composite into a CPU overlay buffer at screen resolution.
// The buffer is flushed as a single GPU texture draw at the end of the
// 2D overlay phase, reducing ~80 GPU submissions to 1.

// DrawPic renders a QPic image at the specified screen-space position.
func (dc *DrawContext) DrawPic(x, y int, pic *image.QPic) {
	dc.DrawPicAlpha(x, y, pic, 1)
}

// DrawPicAlpha renders a QPic image at a screen-space position with explicit alpha.
func (dc *DrawContext) DrawPicAlpha(x, y int, pic *image.QPic, alpha float32) {
	if pic == nil || alpha <= 0 {
		return
	}
	ov := dc.ensureOverlay()
	if ov == nil {
		return
	}
	rgba := dc.renderer.getOrCreatePicRGBA(pic)
	if rgba == nil {
		return
	}
	sx, sy, sw, sh := dc.canvasRectToScreen(x, y, int(pic.Width), int(pic.Height))
	ov.blitRGBA(rgba, int(pic.Width), int(pic.Height), sx, sy, sw, sh, alpha)
}

// DrawMenuPic renders a QPic image in 320x200 menu-space coordinates.
func (dc *DrawContext) DrawMenuPic(x, y int, pic *image.QPic) {
	dc.DrawPicAlpha(x, y, pic, 1)
}

// DrawFill fills a rectangle with a Quake palette color.
func (dc *DrawContext) DrawFill(x, y, w, h int, color byte) {
	dc.DrawFillAlpha(x, y, w, h, color, 1)
}

// DrawFillAlpha fills a rectangle with a Quake palette color and explicit alpha.
func (dc *DrawContext) DrawFillAlpha(x, y, w, h int, color byte, alpha float32) {
	if w <= 0 || h <= 0 || alpha <= 0 {
		return
	}
	ov := dc.ensureOverlay()
	if ov == nil {
		return
	}
	r, g, b := GetPaletteColor(color, dc.renderer.palette)
	if alpha > 1 {
		alpha = 1
	}
	sx, sy, sw, sh := dc.canvasRectToScreen(x, y, w, h)
	ov.fillRect(sx, sy, sw, sh, r, g, b, byte(alpha*255))
}

// DrawCharacter renders a single 8×8 character from the conchars font.
func (dc *DrawContext) DrawCharacter(x, y int, num int) {
	dc.DrawCharacterAlpha(x, y, num, 1)
}

// DrawCharacterAlpha renders a single 8×8 character from the conchars font with explicit alpha.
func (dc *DrawContext) DrawCharacterAlpha(x, y int, num int, alpha float32) {
	if num < 0 || num > 255 || alpha <= 0 {
		return
	}
	ov := dc.ensureOverlay()
	if ov == nil {
		return
	}
	pic := dc.renderer.getCharPic(num)
	if pic == nil {
		return
	}
	rgba := dc.renderer.getOrCreateCharRGBA(pic)
	if rgba == nil {
		return
	}
	sx, sy, sw, sh := dc.canvasRectToScreen(x, y, 8, 8)
	ov.blitRGBA(rgba, 8, 8, sx, sy, sw, sh, alpha)
}

// DrawString renders an entire line of text by compositing all characters
// directly into the CPU overlay buffer. No GPU submission occurs.
func (dc *DrawContext) DrawString(x, y int, text []byte) {
	if len(text) == 0 {
		return
	}
	ov := dc.ensureOverlay()
	if ov == nil {
		return
	}
	dc.renderer.mu.RLock()
	conchars := dc.renderer.concharsData
	palette := dc.renderer.palette
	dc.renderer.mu.RUnlock()

	if len(conchars) < 128*128 || len(palette) < 768 {
		// Conchars not loaded — fall back to per-character.
		for i, ch := range text {
			dc.DrawCharacter(x+i*8, y, int(ch))
		}
		return
	}

	// Composite all characters into a single pixel buffer (palette indices).
	w := len(text) * 8
	pixels := make([]byte, w*8) // filled with 0 = transparent in conchars
	for i, ch := range text {
		num := int(ch) & 0xFF
		col := num % 16
		row := num / 16
		for py := 0; py < 8; py++ {
			srcOff := (row*8+py)*128 + col*8
			dstOff := py*w + i*8
			copy(pixels[dstOff:dstOff+8], conchars[srcOff:srcOff+8])
		}
	}

	// Convert to RGBA (index 0 → transparent via ConvertConcharsToRGBA).
	rgba := ConvertConcharsToRGBA(pixels, palette)
	sx, sy, sw, sh := dc.canvasRectToScreen(x, y, w, 8)
	ov.blitRGBA(rgba, w, 8, sx, sy, sw, sh, 1)
}

// DrawMenuCharacter renders a single 8×8 character in menu-space coordinates.
func (dc *DrawContext) DrawMenuCharacter(x, y int, num int) {
	dc.DrawCharacterAlpha(x, y, num, 1)
}

// SetConchars stores the raw 128×128 conchars pixel data and clears the
// per-character texture cache so that DrawCharacter uses the real font.
func (r *Renderer) SetConchars(data []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(data) < 128*128 {
		return
	}
	r.concharsData = data
	r.charCache = [256]*image.QPic{}
}

type pixelRect struct {
	x float32
	y float32
	w float32
	h float32
}

func (dc *DrawContext) screenPicRect(x, y, w, h int) pixelRect {
	screenX, screenY, screenW, screenH := dc.canvasRectToScreen(x, y, w, h)
	return pixelRect{
		x: float32(screenX),
		y: float32(screenY),
		w: float32(screenW),
		h: float32(screenH),
	}
}

func (dc *DrawContext) canvasRectToScreen(x, y, w, h int) (screenX, screenY, screenW, screenH int) {
	if dc == nil || w <= 0 || h <= 0 || dc.canvas.Type == CanvasNone {
		return x, y, w, h
	}
	params := dc.canvasParams
	if dc.renderer != nil {
		rendererW, rendererH := dc.renderer.Size()
		if params.GUIWidth <= 0 {
			params.GUIWidth = float32(rendererW)
		}
		if params.GUIHeight <= 0 {
			params.GUIHeight = float32(rendererH)
		}
	}
	if params.GUIWidth <= 0 || params.GUIHeight <= 0 {
		return x, y, w, h
	}
	left, top := transformCanvasPointToScreen(dc.canvas.Transform, params.GUIWidth, params.GUIHeight, float32(x), float32(y))
	right, bottom := transformCanvasPointToScreen(dc.canvas.Transform, params.GUIWidth, params.GUIHeight, float32(x+w), float32(y+h))
	if left > right {
		left, right = right, left
	}
	if top > bottom {
		top, bottom = bottom, top
	}
	return int(left), int(top), int(right - left), int(bottom - top)
}

func transformCanvasPointToScreen(transform DrawTransform, screenW, screenH, x, y float32) (screenX, screenY float32) {
	ndcX := x*transform.Scale[0] + transform.Offset[0]
	ndcY := y*transform.Scale[1] + transform.Offset[1]
	screenX = (ndcX + 1) * 0.5 * screenW
	screenY = (1 - (ndcY+1)*0.5) * screenH
	return screenX, screenY
}

// getCharPic returns (or lazily creates) an 8×8 QPic for character num extracted
// from the 128×128 conchars bitmap (16 chars per row).
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

// charCacheKey is a cache key for character textures (separate from pic-based cache).
type charCacheKey struct{ num int }

// getOrCreateCharTexture returns a GPU texture for a character, uploading it if needed.
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

type cacheKey struct {
	pic *image.QPic
}

type cachedTexture struct {
	texture *gogpu.Texture
	width   int
	height  int
}

// Renderer is the main rendering context for the Ironwail-Go engine.
// It manages the gogpu application window, handles the game loop,
// and provides rendering callbacks for the game logic.
//
// Thread Safety:
//
//	Renderer is thread-safe for configuration changes via SetConfig.
//	OnUpdate runs on gogpu's main-thread event loop, while OnDraw and OnClose
//	run on gogpu's dedicated locked render thread.
//
// Lifecycle:
//
//  1. Create with New() or NewWithConfig()
//  2. Set up callbacks with OnDraw() and OnUpdate()
//  3. Run the main loop with Run()
//  4. Shutdown() is called automatically or manually for cleanup
type Renderer struct {
	mu sync.RWMutex

	// app is the gogpu application instance.
	app *gogpu.App

	// config is the current video configuration.
	config Config

	// drawCallback is called each frame to render the scene.
	drawCallback func(dc RenderContext)
	// updateCallback is called each frame for game logic updates.
	updateCallback func(dt float64)

	// closeCallback is called when the window is closed.
	closeCallback func()

	// running indicates if the main loop is active.
	running bool

	// textureCache stores uploaded textures to avoid re-uploading
	textureCache map[cacheKey]*cachedTexture

	// colorTextures stores 1x1 textures for solid colors
	colorTextures [256]*gogpu.Texture

	// palette is the current Quake palette (768 bytes)
	palette []byte

	// concharsData is the raw 128×128 indexed-pixel data for the console font.
	concharsData []byte

	// picRGBACache caches RGBA conversions of QPic images for CPU overlay compositing.
	// Keyed by QPic pointer identity (same as textureCache).
	picRGBACache map[*image.QPic][]byte
	// charCache caches per-character 8×8 QPic objects extracted from concharsData.
	charCache [256]*image.QPic

	// Camera state and matrices for view/projection
	// cameraState holds the current camera position and orientation.
	cameraState CameraState
	// viewMatrices caches computed view and projection matrices.
	viewMatrices ViewMatrixData

	// worldData holds GPU-side resources for BSP world rendering.
	// Set via UploadWorld() when a map is loaded.
	worldData *WorldRenderData
	// worldFirstFrameStatsLogged gates one-shot first-world-frame diagnostics per upload.
	worldFirstFrameStatsLogged atomic.Bool
	// worldVisibleFacesScratch reuses visibility marking/storage across world passes.
	worldVisibleFacesScratch worldVisibilityScratch
	worldSkyFacesScratch     []WorldFace
	worldOpaqueDrawsScratch  []gogpuWorldFaceDraw
	worldAlphaDrawsScratch   []gogpuWorldFaceDraw
	worldLiquidDrawsScratch  []gogpuWorldFaceDraw

	// GPU resources for world rendering
	worldVertexBuffer                 *wgpu.Buffer
	worldIndexBuffer                  *wgpu.Buffer
	worldIndexCount                   uint32
	worldPipeline                     *wgpu.RenderPipeline
	worldTranslucentPipeline          *wgpu.RenderPipeline
	worldTurbulentPipeline            *wgpu.RenderPipeline
	worldTranslucentTurbulentPipeline *wgpu.RenderPipeline
	worldSkyPipeline                  *wgpu.RenderPipeline
	worldSkyExternalPipeline          *wgpu.RenderPipeline
	worldPipelineLayout               *wgpu.PipelineLayout
	worldSkyExternalPipelineLayout    *wgpu.PipelineLayout
	worldBindGroup                    *wgpu.BindGroup
	worldShader                       *wgpu.ShaderModule
	uniformBuffer                     *wgpu.Buffer
	uniformBindGroup                  *wgpu.BindGroup
	uniformBindGroupLayout            *wgpu.BindGroupLayout
	textureBindGroupLayout            *wgpu.BindGroupLayout
	worldSkyExternalBindGroupLayout   *wgpu.BindGroupLayout
	worldTextureSampler               *wgpu.Sampler
	worldTextures                     map[int32]*gpuWorldTexture
	worldFullbrightTextures           map[int32]*gpuWorldTexture
	worldSkySolidTextures             map[int32]*gpuWorldTexture
	worldSkyAlphaTextures             map[int32]*gpuWorldTexture
	worldTextureAnimations            []*SurfaceTexture
	worldSkyExternalTextures          [6]*wgpu.Texture
	worldSkyExternalViews             [6]*wgpu.TextureView
	worldSkyExternalBindGroup         *wgpu.BindGroup
	worldSkyExternalFaces             [6]externalSkyboxFace
	worldSkyExternalLoaded            int
	worldSkyExternalMode              externalSkyboxRenderMode
	worldSkyExternalName              string
	worldSkyExternalRequestID         uint64
	whiteTextureBindGroup             *wgpu.BindGroup
	transparentTexture                *wgpu.Texture
	transparentTextureView            *wgpu.TextureView
	transparentBindGroup              *wgpu.BindGroup
	worldLightmapSampler              *wgpu.Sampler
	worldLightmapPages                []*gpuWorldTexture
	whiteLightmapBindGroup            *wgpu.BindGroup
	worldLightStyleValues             [64]float32

	// 1x1 white texture for fallback
	whiteTexture          *wgpu.Texture
	whiteTextureView      *wgpu.TextureView
	worldDepthTexture     *wgpu.Texture
	worldDepthTextureView *wgpu.TextureView
	worldDepthWidth       int
	worldDepthHeight      int

	// Offscreen render target for world rendering
	worldRenderTexture            *wgpu.Texture
	worldRenderTextureView        *wgpu.TextureView
	worldRenderTextureGogpu       *gogpu.Texture // gogpu-wrapped version for compositing
	worldRenderWidth              int
	worldRenderHeight             int
	sceneCompositePipeline        *wgpu.RenderPipeline
	sceneCompositePipelineLayout  *wgpu.PipelineLayout
	sceneCompositeVertexShader    *wgpu.ShaderModule
	sceneCompositeFragmentShader  *wgpu.ShaderModule
	sceneCompositeBindGroupLayout *wgpu.BindGroupLayout
	sceneCompositeSampler         *wgpu.Sampler
	sceneCompositeUniformBuffer   *wgpu.Buffer
	sceneCompositeBindGroup       *wgpu.BindGroup

	// Alias-model resources for the gogpu backend.
	lightPool                      *glLightPool
	brushModelGeometry             map[int]*WorldGeometry
	brushModelLightmaps            map[int][]*gpuWorldTexture
	aliasModels                    map[string]*gpuAliasModel
	spriteModels                   map[string]*gpuSpriteModel
	aliasEntityStates              map[int]*AliasEntity
	viewModelAliasState            *AliasEntity
	aliasShadowSkin                *gpuAliasSkin
	aliasScratchBuffer             *wgpu.Buffer
	aliasScratchBufferSize         uint64
	aliasPipeline                  *wgpu.RenderPipeline
	aliasShadowPipeline            *wgpu.RenderPipeline
	aliasPipelineLayout            *wgpu.PipelineLayout
	aliasVertexShader              *wgpu.ShaderModule
	aliasFragmentShader            *wgpu.ShaderModule
	aliasUniformBuffer             *wgpu.Buffer
	aliasUniformBindGroup          *wgpu.BindGroup
	aliasUniformBindGroupLayout    *wgpu.BindGroupLayout
	aliasTextureBindGroupLayout    *wgpu.BindGroupLayout
	aliasSampler                   *wgpu.Sampler
	spriteUniformBuffer            *wgpu.Buffer
	spriteUniformBindGroup         *wgpu.BindGroup
	spritePipeline                 *wgpu.RenderPipeline
	spriteVertexShader             *wgpu.ShaderModule
	spriteFragmentShader           *wgpu.ShaderModule
	particleOpaquePipeline         *wgpu.RenderPipeline
	particleTranslucentPipeline    *wgpu.RenderPipeline
	particlePipelineLayout         *wgpu.PipelineLayout
	particleVertexShader           *wgpu.ShaderModule
	particleFragmentShader         *wgpu.ShaderModule
	particleUniformBuffer          *wgpu.Buffer
	particleUniformBindGroup       *wgpu.BindGroup
	particleUniformBindGroupLayout *wgpu.BindGroupLayout
	decalPipeline                  *wgpu.RenderPipeline
	decalPipelineLayout            *wgpu.PipelineLayout
	decalVertexShader              *wgpu.ShaderModule
	decalFragmentShader            *wgpu.ShaderModule
	decalUniformBuffer             *wgpu.Buffer
	decalUniformBindGroup          *wgpu.BindGroup
	decalUniformLayout             *wgpu.BindGroupLayout
	decalAtlasTextureHAL           *wgpu.Texture
	decalAtlasView                 *wgpu.TextureView
	decalBindGroup                 *wgpu.BindGroup
	polyBlendPipeline              *wgpu.RenderPipeline
	polyBlendPipelineLayout        *wgpu.PipelineLayout
	polyBlendVertexShader          *wgpu.ShaderModule
	polyBlendFragmentShader        *wgpu.ShaderModule
	polyBlendUniformBuffer         *wgpu.Buffer
	polyBlendBindGroupLayout       *wgpu.BindGroupLayout
	polyBlendBindGroup             *wgpu.BindGroup

	// Cached overlay texture for 2D compositing — avoids creating a new
	// GPU texture every frame. Recreated only when screen dimensions change.
	overlayTexture       *gogpu.Texture
	overlayTextureWidth  int
	overlayTextureHeight int
	// Pooled CPU pixel buffer — avoids ~4.5MB allocation per frame.
	overlayPixelBuf  []byte
	overlayBufWidth  int
	overlayBufHeight int
}

// New creates a new Renderer with configuration from cvars.
// This is the standard way to create a renderer in Ironwail-Go,
// as it respects user-configurable video settings.
//
// Example:
//
//	r, err := renderer.New()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer r.Shutdown()
func New() (*Renderer, error) {
	return NewWithConfig(ConfigFromCvars())
}

// NewWithConfig creates a new Renderer with the specified configuration.
// Use this when you need explicit control over video settings,
// such as for testing or custom launch modes.
//
// Example:
//
//	cfg := renderer.DefaultConfig()
//	cfg.Width = 1280
//	cfg.Height = 720
//	cfg.Fullscreen = false
//	r, err := renderer.NewWithConfig(cfg)
func NewWithConfig(cfg Config) (*Renderer, error) {
	gogpu.SetLogger(slog.Default())
	slog.Debug("Creating renderer", "width", cfg.Width, "height", cfg.Height, "fullscreen", cfg.Fullscreen)

	// Build gogpu configuration
	gpuCfg := gogpu.DefaultConfig()
	gpuCfg.Title = cfg.Title
	gpuCfg.Width = cfg.Width
	gpuCfg.Height = cfg.Height
	gpuCfg.Fullscreen = cfg.Fullscreen

	// Configure continuous rendering for game loop
	// Quake engines typically run at max FPS, not event-driven
	gpuCfg = gpuCfg.WithContinuousRender(true)

	// Apply VSync setting from engine config
	gpuCfg = gpuCfg.WithVSync(cfg.VSync)

	// Log GPU preference — gogpu doesn't yet support PowerPreference in its
	// Config, so this only affects our headless Core path. On the runtime
	// path, adapter selection falls back to the first non-CPU GPU enumerated
	// by the driver. Use DRI_PRIME=1 (Linux) to force discrete GPU selection
	// at the Vulkan loader level.
	switch cfg.GPUPreference {
	case GPUPreferHighPerformance:
		slog.Info("GPU preference: high-performance (discrete)")
	case GPUPreferLowPower:
		slog.Info("GPU preference: low-power (integrated)")
	default:
		slog.Info("GPU preference: auto")
	}
	applyGPUPreferenceRuntimeEnv(cfg.GPUPreference)

	// Use Pure Go backend (no CGO required)
	gpuCfg = gpuCfg.WithBackend(gogpu.BackendGo)

	// Create the gogpu application
	app := gogpu.NewApp(gpuCfg)

	r := &Renderer{
		app:                 app,
		config:              cfg,
		textureCache:        make(map[cacheKey]*cachedTexture),
		lightPool:           NewGLLightPool(512),
		brushModelGeometry:  make(map[int]*WorldGeometry),
		brushModelLightmaps: make(map[int][]*gpuWorldTexture),
		aliasModels:         make(map[string]*gpuAliasModel),
		spriteModels:        make(map[string]*gpuSpriteModel),
		aliasEntityStates:   make(map[int]*AliasEntity),
	}

	slog.Info("Renderer created",
		"width", cfg.Width,
		"height", cfg.Height,
		"fullscreen", cfg.Fullscreen,
		"vsync", cfg.VSync,
		"maxfps", cfg.MaxFPS,
	)

	return r, nil
}

func applyGPUPreferenceRuntimeEnv(pref GPUPreference) {
	if pref != GPUPreferHighPerformance {
		return
	}
	if _, exists := os.LookupEnv("DRI_PRIME"); exists {
		return
	}
	if err := os.Setenv("DRI_PRIME", "1"); err != nil {
		slog.Warn("failed to apply GPU preference override", "env", "DRI_PRIME", "value", "1", "error", err)
		return
	}
	slog.Info("Applied GPU preference override", "env", "DRI_PRIME", "value", "1")
}

// SetPalette sets the Quake palette used for rendering.
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
//	})
func (r *Renderer) OnDraw(callback func(dc RenderContext)) {
	r.mu.Lock()
	r.drawCallback = callback
	r.mu.Unlock()

	var printed bool
	r.app.OnDraw(func(ctx *gogpu.Context) {
		// Get DeviceProvider (available after first frame initialization)
		provider := r.app.DeviceProvider()
		if provider == nil {
			return // Not ready yet
		}

		// Log device info once
		if !printed {
			slog.Debug(
				"DeviceProvider",
				slog.String("device", fmt.Sprintf("%T", provider.Device())),
				slog.String("queue", fmt.Sprintf("%T", provider.Queue())),
				slog.String("surface_format", provider.SurfaceFormat().String()),
			)
			printed = true
		}

		r.mu.RLock()
		callback := r.drawCallback
		gamma := r.config.Gamma
		r.mu.RUnlock()

		if callback != nil {
			dc := &DrawContext{
				ctx:      ctx,
				gamma:    gamma,
				renderer: r,
			}
			callback(dc)
		}
	})
}

// OnUpdate sets the callback for game logic updates.
// The callback is called each frame with the delta time in seconds.
// This is where physics, AI, and game state updates should occur.
//
// Example:
//
//	r.OnUpdate(func(dt float64) {
//	    player.Update(dt)
//	    world.Simulate(dt)
//	})
func (r *Renderer) OnUpdate(callback func(dt float64)) {
	r.mu.Lock()
	r.updateCallback = callback
	r.mu.Unlock()

	r.app.OnUpdate(func(dt float64) {
		r.mu.RLock()
		callback := r.updateCallback
		r.mu.RUnlock()

		if callback != nil {
			callback(dt)
		}
	})
}

// OnClose sets the callback for window close events.
// This is called when the user closes the window or the
// application requests shutdown.
//
// Use this to save game state and release resources.
func (r *Renderer) OnClose(callback func()) {
	r.mu.Lock()
	r.closeCallback = callback
	r.mu.Unlock()

	r.app.OnClose(func() {
		r.mu.RLock()
		cb := r.closeCallback
		r.mu.RUnlock()

		if cb != nil {
			cb()
		}
	})
}

// Input returns the input state for keyboard and mouse polling.
// Use this in the OnUpdate callback to check for key presses
// and mouse movements.
//
// Example:
//
//	r.OnUpdate(func(dt float64) {
//	    inp := r.Input()
//	    if inp.Keyboard().Pressed(input.KeyW) {
//	        player.MoveForward(dt)
//	    }
//	    if inp.Keyboard().JustPressed(input.KeySpace) {
//	        player.Jump()
//	    }
//	})
func (r *Renderer) Input() *input.State {
	return r.app.Input()
}

// Size returns the current window size in pixels.
// This accounts for DPI scaling on high-DPI displays.
func (r *Renderer) Size() (width, height int) {
	width, height = r.app.Size()
	scale := r.app.ScaleFactor()
	if scale > 0 && scale != 1.0 {
		width = int(float64(width)*scale + 0.5)
		height = int(float64(height)*scale + 0.5)
	}
	return width, height
}

// ScaleFactor returns the DPI scale factor.
// A value of 1.0 indicates standard DPI, 2.0 indicates Retina/HiDPI.
func (r *Renderer) ScaleFactor() float64 {
	return r.app.ScaleFactor()
}

// Config returns the current video configuration.
func (r *Renderer) Config() Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// SetConfig updates the video configuration.
// Some changes (like resolution) may require a video mode change
// which could cause a brief pause.
func (r *Renderer) SetConfig(cfg Config) {
	r.mu.Lock()
	r.config = cfg
	r.mu.Unlock()
}

// CaptureScreenshot exports a minimal deterministic PNG for GoGPU builds.
// Full swapchain readback is intentionally deferred until the backend exposes
// a stable cross-platform texture readback path.
func (r *Renderer) CaptureScreenshot(filename string) error {
	width, height := r.Size()
	if width <= 0 {
		width = r.config.Width
	}
	if height <= 0 {
		height = r.config.Height
	}
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}

	img := stdimage.NewNRGBA(stdimage.Rect(0, 0, width, height))
	fill := color.NRGBA{R: 20, G: 20, B: 46, A: 255}
	for y := 0; y < height; y++ {
		rowStart := y * img.Stride
		row := img.Pix[rowStart : rowStart+width*4]
		for x := 0; x < width; x++ {
			idx := x * 4
			row[idx+0] = fill.R
			row[idx+1] = fill.G
			row[idx+2] = fill.B
			row[idx+3] = fill.A
		}
	}

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("capture screenshot: create file: %w", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("capture screenshot: encode png: %w", err)
	}
	return nil
}

// Run starts the main rendering loop.
// This function blocks until the window is closed or Stop() is called.
//
// Run handles:
//   - Window event processing
//   - Frame timing and VSync
//   - Calling OnDraw and OnUpdate callbacks
//   - GPU resource management
//
// Example:
//
//	if err := r.Run(); err != nil {
//	    log.Fatal(err)
//	}
func (r *Renderer) Run() error {
	r.mu.Lock()
	r.running = true
	r.mu.Unlock()

	slog.Info("Starting render loop")

	err := r.app.Run()

	r.mu.Lock()
	r.running = false
	r.mu.Unlock()

	if err != nil {
		return fmt.Errorf("render loop error: %w", err)
	}
	return nil
}

// Stop requests the renderer to stop the main loop.
// This is typically called from an input handler or menu option.
func (r *Renderer) Stop() {
	r.app.Quit()
}

// Shutdown releases all GPU resources and destroys the window.
// This is called automatically when the main loop exits,
// but can be called manually for explicit cleanup.
func (r *Renderer) Shutdown() {
	slog.Debug("Renderer shutting down")
	r.mu.Lock()
	r.brushModelGeometry = nil
	r.destroyAliasResourcesLocked()
	r.destroySpriteResourcesLocked()
	r.destroyParticleResourcesLocked()
	r.destroyDecalResourcesLocked()
	r.destroyPolyBlendResourcesLocked()
	r.destroyWorldRenderTargetLocked()
	r.destroySceneCompositeResourcesLocked()
	if r.overlayTexture != nil {
		r.overlayTexture.Destroy()
		r.overlayTexture = nil
	}
	r.overlayPixelBuf = nil
	r.mu.Unlock()
	// gogpu.App handles cleanup automatically
}

// IsRunning returns true if the render loop is active.
func (r *Renderer) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// Frame Pipeline Methods
// These methods orchestrate the rendering phases in the correct order.

// RenderFrameState contains all data needed to render a frame.
type RenderFrameState struct {
	// ClearColor is the RGBA clear color (typically dark gray/black)
	ClearColor [4]float32

	// DrawWorld enables 3D world rendering
	DrawWorld bool

	// DrawEntities enables entity rendering
	DrawEntities bool

	// BrushEntities contains inline BSP submodels for parity with the OpenGL path.
	BrushEntities []BrushEntity

	// AliasEntities contains world-space MDL entities for parity with the OpenGL path.
	AliasEntities []AliasModelEntity

	// SpriteEntities contains sprite (billboard) entities for parity with the OpenGL path.
	SpriteEntities []SpriteEntity

	// DecalMarks contains projected world-space mark entities for parity with the OpenGL path.
	DecalMarks []DecalMarkEntity

	// ViewModel contains the first-person weapon model when active.
	ViewModel *AliasModelEntity

	// LightStyles contains evaluated lightstyle scalars for the current frame.
	LightStyles [64]float32

	// FogColor and FogDensity mirror the authoritative renderer state for parity tracking.
	FogColor   [3]float32
	FogDensity float32

	// DrawParticles enables particle rendering
	DrawParticles bool

	// Draw2DOverlay enables 2D overlay (HUD, menu, console)
	Draw2DOverlay bool

	// MenuActive indicates if menu is currently displayed
	MenuActive bool

	// CSQCDrawHud indicates CSQC is drawing HUD this frame.
	CSQCDrawHud bool

	// Particles is the active particle system to render
	Particles *ParticleSystem

	// Palette for color conversion
	Palette []byte

	// WaterWarp, WaterWarpTime, ForceUnderwater: see stubs_opengl.go for semantics.
	// GoGPU uses these for scene-target waterwarp/composite behavior and related runtime parity checks.
	WaterWarp       bool
	WaterWarpTime   float32
	ForceUnderwater bool

	// VBlend is the composite RGBA screen tint from client color shifts.
	// Applied after the 3D scene and entity passes, before the 2D overlay.
	VBlend [4]float32
}

// RenderFrame executes the complete frame pipeline in order:
// 1. Clear screen
// 2. Draw 3D world / scene target
// 3. Draw entities
// 4. Draw particles / viewmodel / polyblend
// 5. Draw 2D overlay (HUD, menu, console)
func (dc *DrawContext) RenderFrame(state *RenderFrameState, draw2DOverlay func(dc RenderContext)) {
	if state == nil {
		return
	}

	slog.Debug("RenderFrame called", "draw_world", state.DrawWorld, "draw_particles", state.DrawParticles, "draw_2d_overlay", state.Draw2DOverlay)
	slog.Debug("RenderFrame: surface view (start)", "id", debugSurfaceViewID(dc.ctx.SurfaceView()))
	if frameCleared, hasPendingClear, ok := dc.getGoGPUFrameStateForDebug(); ok {
		slog.Debug("RenderFrame: gogpu frame state (start)", "frameCleared", frameCleared, "hasPendingClear", hasPendingClear)
	}

	// Phase 1: Clear screen
	// Skip clear when world rendering is active - the world render pass will handle clearing,
	// and gogpu will use LoadOpLoad to preserve our world rendering when drawing the overlay.
	sceneTargetActive := shouldUseSceneRenderTarget(state) && dc.enableSceneRenderTarget()
	if !state.DrawWorld && !sceneTargetActive {
		// When the in-game menu is up without an active world pass, preserve the
		// previously rendered scene behind the menu instead of force-clearing to black.
		// This matches Quake-style "menu over frozen gameplay" behavior.
		if !state.MenuActive {
			dc.Clear(state.ClearColor[0], state.ClearColor[1], state.ClearColor[2], state.ClearColor[3])
		}
	} else if sceneTargetActive && !state.DrawWorld {
		dc.clearCurrentHALRenderTarget(state.ClearColor)
	}

	// Phase 2: Draw 3D world directly to surface view (zero-copy)
	// HAL renders to dc.ctx.SurfaceView() which is the current frame's swapchain texture.
	// Then gogpu draws 2D overlay on top with LoadOpLoad to preserve the world.
	if state.DrawWorld {
		dc.renderer.setGoGPUWorldLightStyleValues(state.LightStyles)
		slog.Debug("RenderFrame: rendering world to surface")
		dc.renderWorld(state)
		slog.Debug("RenderFrame: surface view (after world)", "id", debugSurfaceViewID(dc.ctx.SurfaceView()))
		if !sceneTargetActive && dc.markGoGPUFrameContentForOverlay() {
			slog.Debug("RenderFrame: marked gogpu frame as pre-populated (HAL world rendered)")
			if frameCleared, hasPendingClear, ok := dc.getGoGPUFrameStateForDebug(); ok {
				slog.Debug("RenderFrame: gogpu frame state (after mark)", "frameCleared", frameCleared, "hasPendingClear", hasPendingClear)
			}
		} else if !sceneTargetActive {
			slog.Warn("RenderFrame: unable to mark gogpu frame state; first 2D draw may clear world")
		}
	}

	// Nuclear debug option: skip all gogpu-side drawing for exactly one frame.
	// This isolates "HAL world render + present" from all 2D/overlay paths.
	if state.DrawWorld && shouldRunHALOnlyFrame() {
		slog.Warn("RenderFrame: HAL-only debug frame enabled; skipping entities/particles/2D overlay")
		dc.logPrePresentState("hal-only")
		return
	}

	var translucentBrushEntities []BrushEntity
	var translucentAliasEntities []AliasModelEntity
	if state.DrawEntities {
		_, translucentBrushEntities = splitBrushEntitiesByAlpha(state.BrushEntities)
		_, translucentAliasEntities = splitAliasEntitiesByAlpha(state.AliasEntities)
	}
	lateTranslucency := shouldRunLateTranslucencyBlock(lateTranslucencyBlockInputs{
		drawWorld:                   state.DrawWorld,
		hasTranslucentWorld:         state.DrawWorld && dc.renderer != nil && dc.renderer.hasTranslucentWorldLiquidFacesGoGPU(),
		drawEntities:                state.DrawEntities,
		hasSpriteEntities:           len(state.SpriteEntities) > 0,
		drawParticles:               state.DrawParticles,
		hasDecalMarks:               len(state.DecalMarks) > 0,
		hasTranslucentBrushEntities: len(translucentBrushEntities) > 0,
		hasTranslucentAliasEntities: len(translucentAliasEntities) > 0,
	})
	dc.maybeLogGoGPUFirstWorldFrameStats(state, lateTranslucency, translucentBrushEntities, translucentAliasEntities)

	if shouldClearGoGPUSharedDepthStencil(gogpuSharedDepthStencilClearInputs{
		drawWorld:         state.DrawWorld,
		drawEntities:      state.DrawEntities,
		hasBrushEntities:  len(state.BrushEntities) > 0,
		hasAliasEntities:  len(state.AliasEntities) > 0,
		hasSpriteEntities: len(state.SpriteEntities) > 0,
		hasParticles:      state.DrawParticles && particleCount(state.Particles) > 0,
		hasDecalMarks:     len(state.DecalMarks) > 0,
		hasViewModel:      state.ViewModel != nil,
	}) {
		dc.clearGoGPUSharedDepthStencil()
	}

	// Phase 3: Draw entities, decals, and mode-placed particles.
	if lateTranslucency || state.DrawEntities || len(state.DecalMarks) > 0 || (state.DrawParticles && state.Particles != nil) {
		dc.renderEntities(state)
	}

	if state.DrawEntities && state.ViewModel != nil {
		dc.renderViewModelHAL(*state.ViewModel, state.FogColor, state.FogDensity)
	}
	if sceneTargetActive {
		if dc.compositeSceneRenderTarget(state.WaterWarp, state.WaterWarpTime, state.ClearColor) {
			if dc.markGoGPUFrameContentForOverlay() {
				slog.Debug("RenderFrame: marked gogpu frame as pre-populated (scene composite rendered)")
			} else {
				slog.Warn("RenderFrame: unable to mark gogpu frame state after scene composite")
			}
		} else {
			slog.Warn("RenderFrame: failed to composite scene render target")
		}
		dc.disableSceneRenderTarget()
	}
	if state.VBlend[3] > 0 {
		dc.renderPolyBlendHAL(state.VBlend)
	}

	// Phase 5: Draw 2D overlay (HUD, menu, console)
	// Re-enable to show menu + fallback dots
	// IMPORTANT: When we skip dc.Clear() above (because state.DrawWorld=true),
	// gogpu should use LoadOpLoad for its internal 2D render pass, preserving
	// the world rendering we just submitted via HAL. This relies on gogpu's
	// internal behavior to detect that Clear() was not called.
	if state.Draw2DOverlay && draw2DOverlay != nil {
		if frameCleared, hasPendingClear, ok := dc.getGoGPUFrameStateForDebug(); ok {
			slog.Debug("RenderFrame: gogpu frame state (pre-overlay)", "frameCleared", frameCleared, "hasPendingClear", hasPendingClear)
		}
		if shouldDrawWorldFallbackDots() {
			slog.Debug("RenderFrame: debug world fallback dots enabled")
			dc.renderWorldFallbackTopDown()
		}
		slog.Debug("Drawing 2D overlay on top of world", "menu_active", state.MenuActive)
		draw2DOverlay(dc)
		dc.flush2DOverlay()
	}

	dc.logPrePresentState("normal")
}

func (dc *DrawContext) maybeLogGoGPUFirstWorldFrameStats(state *RenderFrameState, lateTranslucency bool, translucentBrushEntities []BrushEntity, translucentAliasEntities []AliasModelEntity) {
	if dc == nil || dc.renderer == nil || state == nil || !state.DrawWorld {
		return
	}
	worldData := dc.renderer.GetWorldData()
	if worldData == nil || worldData.Geometry == nil {
		return
	}
	if !dc.renderer.worldFirstFrameStatsLogged.CompareAndSwap(false, true) {
		return
	}

	camera := dc.renderer.cameraState
	liquidAlpha := worldLiquidAlphaSettingsFromCvars(parseWorldspawnLiquidAlphaOverrides(worldData.Geometry.Tree.Entities), worldData.Geometry.Tree)
	visibleFaces := selectVisibleWorldFaces(
		worldData.Geometry.Tree,
		worldData.Geometry.Faces,
		worldData.Geometry.LeafFaces,
		[3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z},
	)
	visibleStats := summarizeGoGPUWorldFaceStats(visibleFaces, liquidAlpha)

	dynamicLightCount := 0
	dc.renderer.mu.RLock()
	if dc.renderer.lightPool != nil {
		dynamicLightCount = dc.renderer.lightPool.ActiveCount()
	}
	dc.renderer.mu.RUnlock()

	particleTotal := 0
	if state.DrawParticles {
		particleTotal = particleCount(state.Particles)
	}
	opaqueBrushEntities := len(state.BrushEntities) - len(translucentBrushEntities)
	if opaqueBrushEntities < 0 {
		opaqueBrushEntities = 0
	}
	opaqueAliasEntities := len(state.AliasEntities) - len(translucentAliasEntities)
	if opaqueAliasEntities < 0 {
		opaqueAliasEntities = 0
	}

	slog.Info("GoGPU first frame stats",
		"alpha_mode", effectiveGoGPUAlphaMode(GetAlphaMode()).String(),
		"visible_faces", visibleStats.TotalFaces,
		"visible_triangles", visibleStats.TotalTriangles,
		"visible_lightmapped_faces", visibleStats.LightmappedFaces,
		"visible_lit_water_faces", visibleStats.LitWaterFaces,
		"visible_turbulent_faces", visibleStats.TurbulentFaces,
		"visible_sky_faces", visibleStats.SkyFaces,
		"visible_opaque_faces", visibleStats.OpaqueFaces,
		"visible_alpha_test_faces", visibleStats.AlphaTestFaces,
		"visible_opaque_liquid_faces", visibleStats.OpaqueLiquidFaces,
		"visible_translucent_liquid_faces", visibleStats.TranslucentLiquidFaces,
		"world_faces_total", worldData.TotalFaces,
		"world_triangles_total", worldData.TotalIndices/3,
		"lightmap_pages", len(worldData.Geometry.Lightmaps),
		"brush_entities", len(state.BrushEntities),
		"brush_entities_opaque", opaqueBrushEntities,
		"brush_entities_translucent", len(translucentBrushEntities),
		"alias_entities", len(state.AliasEntities),
		"alias_entities_opaque", opaqueAliasEntities,
		"alias_entities_translucent", len(translucentAliasEntities),
		"sprite_entities", len(state.SpriteEntities),
		"decal_marks", len(state.DecalMarks),
		"particles", particleTotal,
		"dynamic_lights", dynamicLightCount,
		"late_translucency", lateTranslucency,
		"menu_active", state.MenuActive,
		"view_model", state.ViewModel != nil,
	)
}

func shouldRunHALOnlyFrame() bool {
	if os.Getenv("IRONWAIL_DEBUG_HAL_ONLY_FRAME") != "1" {
		return false
	}

	return !halOnlyFrameConsumed.Swap(true)
}

func (dc *DrawContext) logPrePresentState(mode string) {
	if dc == nil || dc.ctx == nil {
		return
	}

	if frameCleared, hasPendingClear, ok := dc.getGoGPUFrameStateForDebug(); ok {
		slog.Debug("RenderFrame: gogpu frame state (pre-present)",
			"mode", mode,
			"frameCleared", frameCleared,
			"hasPendingClear", hasPendingClear,
		)
	} else {
		slog.Warn("RenderFrame: unable to read gogpu frame state (pre-present)", "mode", mode)
	}

	slog.Debug("RenderFrame: surface view (pre-present)", "mode", mode, "id", debugSurfaceViewID(dc.ctx.SurfaceView()))
}

func debugSurfaceViewID(view any) string {
	if view == nil {
		return "nil"
	}

	value := reflect.ValueOf(view)
	if value.Kind() == reflect.Pointer || value.Kind() == reflect.UnsafePointer {
		if value.IsNil() {
			return fmt.Sprintf("%T@nil", view)
		}
		return fmt.Sprintf("%T@0x%x", view, value.Pointer())
	}

	return fmt.Sprintf("%T@%p", view, &view)
}

func shouldDrawWorldFallbackDots() bool {
	return os.Getenv("IRONWAIL_DEBUG_WORLD_DOTS") == "1"
}

// markGoGPUFrameContentForOverlay sets gogpu renderer's internal frame state
// so the first 2D draw pass uses LoadOpLoad (preserve existing surface content)
// rather than defaulting to LoadOpClear.
func (dc *DrawContext) markGoGPUFrameContentForOverlay() bool {
	if dc == nil || dc.ctx == nil {
		return false
	}

	ctxValue := reflect.ValueOf(dc.ctx)
	if ctxValue.Kind() != reflect.Pointer || ctxValue.IsNil() {
		return false
	}

	ctxElem := ctxValue.Elem()
	rendererField := ctxElem.FieldByName("renderer")
	if !rendererField.IsValid() || rendererField.Kind() != reflect.Pointer || rendererField.IsNil() {
		return false
	}

	rendererPtr := reflect.NewAt(rendererField.Type(), unsafe.Pointer(rendererField.UnsafeAddr())).Elem()
	rendererElem := rendererPtr.Elem()

	frameClearedField := rendererElem.FieldByName("frameCleared")
	hasPendingClearField := rendererElem.FieldByName("hasPendingClear")
	if !frameClearedField.IsValid() || !hasPendingClearField.IsValid() {
		return false
	}

	frameClearedWritable := reflect.NewAt(frameClearedField.Type(), unsafe.Pointer(frameClearedField.UnsafeAddr())).Elem()
	hasPendingClearWritable := reflect.NewAt(hasPendingClearField.Type(), unsafe.Pointer(hasPendingClearField.UnsafeAddr())).Elem()

	if frameClearedWritable.Kind() != reflect.Bool || hasPendingClearWritable.Kind() != reflect.Bool {
		return false
	}

	frameClearedWritable.SetBool(true)
	hasPendingClearWritable.SetBool(false)

	return true
}

func (dc *DrawContext) getGoGPUFrameStateForDebug() (frameCleared bool, hasPendingClear bool, ok bool) {
	if dc == nil || dc.ctx == nil {
		return false, false, false
	}

	ctxValue := reflect.ValueOf(dc.ctx)
	if ctxValue.Kind() != reflect.Pointer || ctxValue.IsNil() {
		return false, false, false
	}

	ctxElem := ctxValue.Elem()
	rendererField := ctxElem.FieldByName("renderer")
	if !rendererField.IsValid() || rendererField.Kind() != reflect.Pointer || rendererField.IsNil() {
		return false, false, false
	}

	rendererPtr := reflect.NewAt(rendererField.Type(), unsafe.Pointer(rendererField.UnsafeAddr())).Elem()
	rendererElem := rendererPtr.Elem()

	frameClearedField := rendererElem.FieldByName("frameCleared")
	hasPendingClearField := rendererElem.FieldByName("hasPendingClear")
	if !frameClearedField.IsValid() || !hasPendingClearField.IsValid() {
		return false, false, false
	}

	frameClearedReadable := reflect.NewAt(frameClearedField.Type(), unsafe.Pointer(frameClearedField.UnsafeAddr())).Elem()
	hasPendingClearReadable := reflect.NewAt(hasPendingClearField.Type(), unsafe.Pointer(hasPendingClearField.UnsafeAddr())).Elem()
	if frameClearedReadable.Kind() != reflect.Bool || hasPendingClearReadable.Kind() != reflect.Bool {
		return false, false, false
	}

	return frameClearedReadable.Bool(), hasPendingClearReadable.Bool(), true
}

// renderWorldFallbackTopDown draws a sparse top-down projection of world
// vertices using the 2D API. It is used as a visibility fallback while the
// GPU world pass is being stabilized across platforms.
func (dc *DrawContext) renderWorldFallbackTopDown() {
	worldData := dc.renderer.GetWorldData()
	if worldData == nil || worldData.Geometry == nil || len(worldData.Geometry.Vertices) == 0 {
		return
	}

	verts := worldData.Geometry.Vertices
	minX, maxX := verts[0].Position[0], verts[0].Position[0]
	minY, maxY := verts[0].Position[1], verts[0].Position[1]
	for index := 1; index < len(verts); index++ {
		x := verts[index].Position[0]
		y := verts[index].Position[1]
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}

	rangeX := maxX - minX
	rangeY := maxY - minY
	if rangeX < 1 || rangeY < 1 {
		return
	}

	screenW, screenH := dc.renderer.Size()
	if screenW <= 0 || screenH <= 0 {
		return
	}

	const margin = 24.0
	drawW := float32(screenW) - margin*2
	drawH := float32(screenH) - margin*2
	if drawW <= 0 || drawH <= 0 {
		return
	}

	scaleX := drawW / rangeX
	scaleY := drawH / rangeY
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}

	step := len(verts) / 5000
	if step < 1 {
		step = 1
	}

	// Draw a visible frame so non-black background is obvious.
	frameColor := byte(250)
	panelColor := byte(48)
	dc.DrawFill(int(margin), int(margin), int(drawW), int(drawH), panelColor)
	dc.DrawFill(int(margin), int(margin), int(drawW), 2, frameColor)
	dc.DrawFill(int(margin), int(margin+drawH)-2, int(drawW), 2, frameColor)
	dc.DrawFill(int(margin), int(margin), 2, int(drawH), frameColor)
	dc.DrawFill(int(margin+drawW)-2, int(margin), 2, int(drawH), frameColor)

	for index := 0; index < len(verts); index += step {
		v := verts[index]
		sx := int(margin + (v.Position[0]-minX)*scale)
		sy := int(float32(screenH) - (margin + (v.Position[1]-minY)*scale))
		if sx < 0 || sx >= screenW || sy < 0 || sy >= screenH {
			continue
		}
		dc.DrawFill(sx, sy, 2, 2, 251)
	}
}

// renderWorld draws the 3D world geometry (BSP surfaces).
// Implementation delegates to renderWorldInternal in world.go.
func (dc *DrawContext) renderWorld(state *RenderFrameState) {
	// Render world directly to the current surface view (zero-copy approach)
	// The surface view is available from dc.ctx.SurfaceView() per gogpu's design
	dc.renderWorldInternal(state)
}

// Note: ensureWorldRenderTarget removed - we now render directly to the surface view
// provided by gogpu via dc.ctx.SurfaceView() for zero-copy rendering.

// renderEntities draws runtime entities. Alias models, sprites, and decals use
// HAL-backed paths.
func (dc *DrawContext) renderEntities(state *RenderFrameState) {
	if dc == nil || dc.renderer == nil || state == nil {
		return
	}
	hasTranslucentWorld := state.DrawWorld && dc.renderer.hasTranslucentWorldLiquidFacesGoGPU()
	particlePhase, hasParticlePhase := classifyGoGPUParticlePhase(readGoGPUParticleModeCvar(), particleCount(state.Particles))
	plan := planGoGPUEntityDrawOrder(state.DrawEntities, hasTranslucentWorld, state.BrushEntities, state.AliasEntities, state.SpriteEntities, state.DecalMarks, particlePhase, hasParticlePhase)
	var pendingTranslucentRenders []gogpuTranslucentBrushFaceRender
	var pendingTransientBuffers []*wgpu.Buffer
	flushPendingTranslucency := func() {
		if len(pendingTranslucentRenders) == 0 {
			destroyGoGPUTransientBuffers(pendingTransientBuffers)
			pendingTransientBuffers = nil
			return
		}
		sortGoGPUTranslucentBrushFaceRenders(effectiveGoGPUAlphaMode(GetAlphaMode()), pendingTranslucentRenders)
		dc.renderGoGPUSortedTranslucentFaceRendersHAL(pendingTranslucentRenders, state.FogColor, state.FogDensity)
		destroyGoGPUTransientBuffers(pendingTransientBuffers)
		pendingTranslucentRenders = nil
		pendingTransientBuffers = nil
	}
	for _, phase := range plan.phases {
		switch phase {
		case gogpuEntityPhaseTranslucentWorldLiquid, gogpuEntityPhaseTranslucentLiquidBrush, gogpuEntityPhaseTranslucentBrush:
		default:
			flushPendingTranslucency()
		}
		switch phase {
		case gogpuEntityPhaseOpaqueBrush:
			dc.renderOpaqueBrushEntitiesHAL(plan.opaqueBrush, state.FogColor, state.FogDensity)
		case gogpuEntityPhaseOpaqueAlias:
			for _, step := range gogpuOpaqueAliasPassSteps() {
				switch step {
				case gogpuOpaqueAliasStepEntities:
					dc.renderAliasEntitiesHAL(plan.opaqueAlias, state.FogColor, state.FogDensity)
				case gogpuOpaqueAliasStepShadows:
					dc.renderAliasShadowsHAL(plan.opaqueAlias, state.FogColor, state.FogDensity)
				}
			}
		case gogpuEntityPhaseOpaqueParticles:
			if state.DrawParticles && state.Particles != nil {
				dc.renderParticlesHAL(state, false)
			}
		case gogpuEntityPhaseSkyBrush:
			dc.renderSkyBrushEntitiesHAL(plan.skyBrush, state.FogColor, state.FogDensity)
		case gogpuEntityPhaseOpaqueLiquidBrush:
			dc.renderOpaqueLiquidBrushEntitiesHAL(plan.opaqueBrush, state.FogColor, state.FogDensity)
		case gogpuEntityPhaseTranslucentWorldLiquid:
			pendingTranslucentRenders = append(pendingTranslucentRenders, dc.collectGoGPUWorldTranslucentLiquidFaceRenders()...)
		case gogpuEntityPhaseTranslucentLiquidBrush:
			renders, buffers := dc.collectGoGPUTranslucentLiquidBrushFaceRenders(plan.opaqueBrush)
			pendingTranslucentRenders = append(pendingTranslucentRenders, renders...)
			pendingTransientBuffers = append(pendingTransientBuffers, buffers...)
		case gogpuEntityPhaseTranslucentBrush:
			alphaTestRenders, renders, buffers := dc.collectGoGPUTranslucentBrushEntityFaceRenders(plan.translucentBrush)
			dc.renderGoGPUAlphaTestBrushFaceRendersHAL(alphaTestRenders, state.FogColor, state.FogDensity)
			pendingTranslucentRenders = append(pendingTranslucentRenders, renders...)
			pendingTransientBuffers = append(pendingTransientBuffers, buffers...)
		case gogpuEntityPhaseDecals:
			dc.renderDecalMarksHAL(state.DecalMarks)
		case gogpuEntityPhaseTranslucentAlias:
			dc.renderAliasEntitiesHAL(plan.translucentAlias, state.FogColor, state.FogDensity)
		case gogpuEntityPhaseSprites:
			dc.renderSpriteEntitiesHAL(state.SpriteEntities, state.FogColor, state.FogDensity)
		case gogpuEntityPhaseTranslucentParticles:
			if state.DrawParticles && state.Particles != nil {
				dc.renderParticlesHAL(state, true)
			}
		}
	}
	flushPendingTranslucency()
}

func projectWorldPointToScreen(pos [3]float32, vp types.Mat4, screenW, screenH int) (x int, y int, ok bool) {
	if screenW <= 0 || screenH <= 0 {
		return 0, 0, false
	}

	clipPos := TransformVertex(pos, vp)
	if clipPos.W <= 0.001 {
		return 0, 0, false
	}

	invW := float32(1.0) / clipPos.W
	ndcX := clipPos.X * invW
	ndcY := clipPos.Y * invW
	ndcZ := clipPos.Z * invW

	if ndcX < -1 || ndcX > 1 || ndcY < -1 || ndcY > 1 || ndcZ < -1 || ndcZ > 1 {
		return 0, 0, false
	}

	screenX := (ndcX*0.5 + 0.5) * float32(screenW-1)
	screenY := (1 - (ndcY*0.5 + 0.5)) * float32(screenH-1)

	return int(screenX), int(screenY), true
}

type projectedParticleMarker struct {
	x     int
	y     int
	color byte
	size  int
	alpha float32
}

func projectParticleMarkers(particles []Particle, verts []ParticleVertex, vp types.Mat4, screenW, screenH int) []projectedParticleMarker {
	if len(particles) == 0 || len(verts) == 0 {
		return nil
	}
	count := len(particles)
	if len(verts) < count {
		count = len(verts)
	}
	markers := make([]projectedParticleMarker, 0, count)
	for i := 0; i < count; i++ {
		x, y, ok := projectWorldPointToScreen(verts[i].Pos, vp, screenW, screenH)
		if !ok {
			continue
		}
		markers = append(markers, projectedParticleMarker{
			x:     x,
			y:     y,
			color: particles[i].Color,
			size:  4,
			alpha: 1,
		})
	}
	return markers
}

func (r *Renderer) ensureBrushModelGeometry(submodelIndex int) *WorldGeometry {
	if submodelIndex <= 0 {
		return nil
	}
	r.mu.RLock()
	if geom := r.brushModelGeometry[submodelIndex]; geom != nil {
		r.mu.RUnlock()
		return geom
	}
	tree := (*bsp.Tree)(nil)
	if r.worldData != nil && r.worldData.Geometry != nil {
		tree = r.worldData.Geometry.Tree
	}
	r.mu.RUnlock()
	if tree == nil {
		return nil
	}
	geom, err := BuildModelGeometry(tree, submodelIndex)
	if err != nil {
		slog.Debug("GoGPU brush model build skipped", "submodel", submodelIndex, "error", err)
		return nil
	}
	if geom == nil || len(geom.Vertices) == 0 {
		return nil
	}
	r.mu.Lock()
	if r.brushModelGeometry == nil {
		r.brushModelGeometry = make(map[int]*WorldGeometry)
	}
	if existing := r.brushModelGeometry[submodelIndex]; existing != nil {
		r.mu.Unlock()
		return existing
	}
	r.brushModelGeometry[submodelIndex] = geom
	r.mu.Unlock()
	return geom
}

func (r *Renderer) ensureBrushModelLightmaps(submodelIndex int, geom *WorldGeometry) []*gpuWorldTexture {
	if submodelIndex <= 0 || geom == nil || len(geom.Lightmaps) == 0 {
		return nil
	}
	r.mu.RLock()
	if cached := r.brushModelLightmaps[submodelIndex]; len(cached) > 0 {
		r.mu.RUnlock()
		return cached
	}
	sampler := r.worldLightmapSampler
	values := r.worldLightStyleValues
	r.mu.RUnlock()
	device := r.getWGPUDevice()
	queue := r.getWGPUQueue()
	if device == nil || queue == nil || sampler == nil {
		return nil
	}
	uploaded := r.uploadWorldLightmapPages(device, queue, sampler, geom.Lightmaps, values)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.brushModelLightmaps == nil {
		r.brushModelLightmaps = make(map[int][]*gpuWorldTexture)
	}
	if existing := r.brushModelLightmaps[submodelIndex]; len(existing) > 0 {
		for _, page := range uploaded {
			if page == nil {
				continue
			}
			if page.bindGroup != nil {
				page.bindGroup.Release()
			}
			if page.view != nil {
				page.view.Release()
			}
			if page.texture != nil {
				page.texture.Release()
			}
		}
		return existing
	}
	r.brushModelLightmaps[submodelIndex] = uploaded
	return uploaded
}

func readGoGPUParticleModeCvar() int {
	cv := cvar.Get(CvarRParticles)
	if cv == nil {
		return 1
	}
	return cv.Int
}

func shouldDrawGoGPUParticles(mode, activeParticles int) bool {
	return ShouldDrawParticles(mode, false, false, activeParticles) || ShouldDrawParticles(mode, true, false, activeParticles)
}

func particleCount(ps *ParticleSystem) int {
	if ps == nil {
		return 0
	}
	return ps.ActiveCount()
}

// buildParticlePalette converts a 768-byte palette to the [256][4]byte format
// expected by BuildParticleVertices.
func buildParticlePalette(palette []byte) [256][4]byte {
	var p [256][4]byte
	if len(palette) < 768 {
		// Grayscale fallback
		for i := range p {
			p[i] = [4]byte{byte(i), byte(i), byte(i), 255}
		}
		return p
	}
	for i := range p {
		offset := i * 3
		p[i] = [4]byte{
			palette[offset],
			palette[offset+1],
			palette[offset+2],
			255,
		}
	}
	return p
}

// DefaultRenderFrameState returns a sensible default RenderFrameState.
func DefaultRenderFrameState() *RenderFrameState {
	return &RenderFrameState{
		ClearColor:    [4]float32{0.0, 0.0, 0.0, 1.0}, // Black
		DrawWorld:     false,                          // Disabled until M4.3
		DrawEntities:  false,                          // Disabled until M4.4
		DrawParticles: false,                          // Opt-in per frame like the primary renderer
		Draw2DOverlay: true,                           // Always draw 2D overlay
		MenuActive:    true,                           // Menu is active at startup
	}
}

// UpdateCamera updates the camera state and recomputes view/projection matrices.
// This should be called once per frame with the current player position and orientation
// from client prediction.
func (r *Renderer) UpdateCamera(camera CameraState, nearPlane, farPlane float32) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cameraState = camera

	// Compute view matrix from camera state
	r.viewMatrices.View = ComputeViewMatrix(camera)

	// Compute projection matrix (aspect ratio from window size)
	// Use a default aspect ratio if the app is not initialized
	aspect := float32(16.0 / 9.0)
	if r.app != nil {
		w, h := r.Size()
		if w > 0 && h > 0 {
			aspect = float32(w) / float32(h)
		}
	}

	r.viewMatrices.Projection = ComputeProjectionMatrix(projectionFOVForCamera(camera), aspect, nearPlane, farPlane)

	// Log individual matrices before multiplication
	slog.Debug("Camera matrices computed",
		"view_m00", r.viewMatrices.View[0],
		"view_m11", r.viewMatrices.View[5],
		"view_m22", r.viewMatrices.View[10],
		"view_m33", r.viewMatrices.View[15],
		"proj_m00", r.viewMatrices.Projection[0],
		"proj_m11", r.viewMatrices.Projection[5],
		"proj_m22", r.viewMatrices.Projection[10],
		"proj_m33", r.viewMatrices.Projection[15])

	// Compute combined VP matrix
	r.viewMatrices.VP = types.Mat4Multiply(r.viewMatrices.Projection, r.viewMatrices.View)

	// Log VP matrix for debugging
	slog.Debug("Camera updated",
		"position", camera.Origin,
		"angles", camera.Angles,
		"near", nearPlane,
		"far", farPlane,
		"aspect", aspect,
		"fov", camera.FOV,
		"vp_matrix_0_0", r.viewMatrices.VP[0],
		"vp_matrix_3_2", r.viewMatrices.VP[14])
}

// GetViewMatrix returns the currently cached view matrix.
// Thread-safe read.
func (r *Renderer) GetViewMatrix() types.Mat4 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.viewMatrices.View
}

// GetProjectionMatrix returns the currently cached projection matrix.
// Thread-safe read.
func (r *Renderer) GetProjectionMatrix() types.Mat4 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.viewMatrices.Projection
}

// GetViewProjectionMatrix returns the combined View × Projection matrix.
// This is the matrix typically used in vertex shaders for world-to-NDC transformation.
// Thread-safe read.
func (r *Renderer) GetViewProjectionMatrix() types.Mat4 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.viewMatrices.VP
}

// GetCameraState returns the current camera state (position and orientation).
// Thread-safe read. A copy is returned to prevent external modification.
func (r *Renderer) GetCameraState() CameraState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cameraState
}

// HasWorldData reports whether GPU world geometry has been uploaded.
func (r *Renderer) HasWorldData() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.worldData != nil && r.worldVertexBuffer != nil && r.worldIndexBuffer != nil && r.worldIndexCount > 0 && r.worldPipeline != nil
}

func (r *Renderer) SpawnDynamicLight(light DynamicLight) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lightPool == nil {
		return false
	}
	return r.lightPool.SpawnLight(light)
}

func (r *Renderer) SpawnKeyedDynamicLight(light DynamicLight) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lightPool == nil {
		return false
	}
	return r.lightPool.SpawnOrReplaceKeyed(light)
}

func (r *Renderer) UpdateLights(deltaTime float32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lightPool != nil {
		r.lightPool.UpdateAndFilter(deltaTime)
	}
}

func (r *Renderer) ClearDynamicLights() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lightPool != nil {
		r.lightPool.Clear()
	}
}

func (r *Renderer) SetExternalSkybox(name string, loadFile func(string) ([]byte, error)) {
	normalized := normalizeSkyboxBaseName(name)

	r.mu.Lock()
	r.worldSkyExternalRequestID++
	requestID := r.worldSkyExternalRequestID
	if normalized == r.worldSkyExternalName {
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()

	var (
		faces  [6]externalSkyboxFace
		loaded int
	)
	if normalized != "" && loadFile != nil {
		faces, loaded = loadExternalSkyboxFaces(normalized, loadFile)
	}
	renderMode := externalSkyboxRenderEmbedded
	if normalized != "" && loaded > 0 {
		renderMode = externalSkyboxRenderFaces
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if requestID != r.worldSkyExternalRequestID {
		return
	}

	r.destroyGoGPUExternalSkyboxResourcesLocked()
	r.worldSkyExternalFaces = [6]externalSkyboxFace{}
	r.worldSkyExternalLoaded = 0
	r.worldSkyExternalMode = externalSkyboxRenderEmbedded
	r.worldSkyExternalName = ""

	if normalized == "" || renderMode == externalSkyboxRenderEmbedded {
		return
	}

	r.worldSkyExternalFaces = faces
	r.worldSkyExternalLoaded = loaded
	r.worldSkyExternalMode = renderMode
	r.worldSkyExternalName = normalized

	if err := r.ensureGoGPUExternalSkyboxLocked(r.getWGPUDevice(), r.getWGPUQueue()); err != nil {
		slog.Debug("external gogpu skybox upload deferred", "name", normalized, "error", err)
	}
}

// NeedsWorldGPUUpload reports whether CPU world geometry exists but GPU buffers
// are not uploaded yet.
func (r *Renderer) NeedsWorldGPUUpload() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.worldData != nil && (r.worldVertexBuffer == nil || r.worldIndexBuffer == nil || r.worldIndexCount == 0)
}

// getWGPUDevice returns the public WebGPU device exposed by the app provider.
func (r *Renderer) getWGPUDevice() *wgpu.Device {
	if r.app == nil {
		return nil
	}
	provider := r.app.DeviceProvider()
	if provider == nil {
		return nil
	}
	raw := any(provider.Device())
	device, ok := raw.(*wgpu.Device)
	if !ok {
		return nil
	}
	return device
}

func (r *Renderer) getWGPUQueue() *wgpu.Queue {
	device := r.getWGPUDevice()
	if device == nil {
		return nil
	}
	return device.Queue()
}

func (r *Renderer) destroyGoGPUExternalSkyboxResourcesLocked() {
	if r.worldSkyExternalBindGroup != nil {
		r.worldSkyExternalBindGroup.Release()
		r.worldSkyExternalBindGroup = nil
	}
	for i := range r.worldSkyExternalViews {
		if r.worldSkyExternalViews[i] != nil {
			r.worldSkyExternalViews[i].Release()
			r.worldSkyExternalViews[i] = nil
		}
		if r.worldSkyExternalTextures[i] != nil {
			r.worldSkyExternalTextures[i].Release()
			r.worldSkyExternalTextures[i] = nil
		}
	}
}
