package renderer

import (
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gmath"
)

type overlay2D struct {
	pixels []byte // RGBA at screen resolution (straight alpha)
	width  int
	height int
	dirty  bool
	minX   int
	minY   int
	maxX   int
	maxY   int
}

type overlayDirtyRect struct {
	x int
	y int
	w int
	h int
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
	currentDirty := ov.dirtyRect()
	uploadRect := currentDirty
	if r.overlayTextureDirtyValid {
		uploadRect = unionOverlayDirtyRects(uploadRect, overlayDirtyRect{
			x: r.overlayTextureDirtyX,
			y: r.overlayTextureDirtyY,
			w: r.overlayTextureDirtyW,
			h: r.overlayTextureDirtyH,
		})
	}
	// Reuse cached GPU texture if dimensions match.
	if r.overlayTexture != nil && r.overlayTextureWidth == ov.width && r.overlayTextureHeight == ov.height {
		tex := r.overlayTexture
		uploadPixels := r.overlayUploadRegionPixelsLocked(ov.pixels, ov.width, uploadRect)
		r.overlayTextureDirtyX = currentDirty.x
		r.overlayTextureDirtyY = currentDirty.y
		r.overlayTextureDirtyW = currentDirty.w
		r.overlayTextureDirtyH = currentDirty.h
		r.overlayTextureDirtyValid = true
		r.mu.Unlock()
		var err error
		if uploadRect.x == 0 && uploadRect.y == 0 && uploadRect.w == ov.width && uploadRect.h == ov.height {
			err = tex.UpdateData(ov.pixels)
		} else {
			err = tex.UpdateRegion(uploadRect.x, uploadRect.y, uploadRect.w, uploadRect.h, uploadPixels)
		}
		if err != nil {
			slog.Error("flush2DOverlay: texture update failed", "error", err)
			dc.overlay = nil
			return
		}
		if !dc.renderOverlayTextureHAL(tex) {
			slog.Error("flush2DOverlay: HAL overlay composite failed")
			dc.overlay = nil
			return
		}
	} else {
		// Dimensions changed or first frame — create new texture.
		if r.overlayTexture != nil {
			r.overlayTexture.Destroy()
			r.overlayTexture = nil
		}
		r.overlayTextureDirtyX = currentDirty.x
		r.overlayTextureDirtyY = currentDirty.y
		r.overlayTextureDirtyW = currentDirty.w
		r.overlayTextureDirtyH = currentDirty.h
		r.overlayTextureDirtyValid = true
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
		if !dc.renderOverlayTextureHAL(tex) {
			slog.Error("flush2DOverlay: HAL overlay composite failed")
			dc.overlay = nil
			return
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
	ov.markDirtyRect(x, y, w, h)
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
	ov.markDirtyRect(dstX, dstY, dstW, dstH)
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

func (ov *overlay2D) blitConcharsString(conchars, palette, text []byte, dstX, dstY, dstW, dstH int) {
	if len(text) == 0 || len(conchars) < 128*128 || len(palette) < 768 || dstW <= 0 || dstH <= 0 {
		return
	}
	srcW := len(text) * 8
	srcH := 8
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
	ov.markDirtyRect(dstX, dstY, dstW, dstH)
	stride := ov.width * 4
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
			charIndex := srcX / 8
			glyphX := srcX % 8
			num := int(text[charIndex]) & 0xFF
			col := num % 16
			row := num / 16
			srcOff := (row*8+srcY)*128 + col*8 + glyphX
			pixel := conchars[srcOff]
			if pixel != 0 {
				paletteOff := int(pixel) * 3
				ov.pixels[dstOff] = palette[paletteOff]
				ov.pixels[dstOff+1] = palette[paletteOff+1]
				ov.pixels[dstOff+2] = palette[paletteOff+2]
				ov.pixels[dstOff+3] = 255
			}
			dstOff += 4
		}
	}
}

func (ov *overlay2D) markDirtyRect(x, y, w, h int) {
	if w <= 0 || h <= 0 {
		return
	}
	rect := overlayDirtyRect{x: x, y: y, w: w, h: h}
	if !ov.dirty {
		ov.dirty = true
		ov.minX = rect.x
		ov.minY = rect.y
		ov.maxX = rect.x + rect.w
		ov.maxY = rect.y + rect.h
		return
	}
	if rect.x < ov.minX {
		ov.minX = rect.x
	}
	if rect.y < ov.minY {
		ov.minY = rect.y
	}
	if rect.x+rect.w > ov.maxX {
		ov.maxX = rect.x + rect.w
	}
	if rect.y+rect.h > ov.maxY {
		ov.maxY = rect.y + rect.h
	}
}

func (ov *overlay2D) dirtyRect() overlayDirtyRect {
	if ov == nil || !ov.dirty {
		return overlayDirtyRect{}
	}
	return overlayDirtyRect{
		x: ov.minX,
		y: ov.minY,
		w: ov.maxX - ov.minX,
		h: ov.maxY - ov.minY,
	}
}

func unionOverlayDirtyRects(a, b overlayDirtyRect) overlayDirtyRect {
	if a.w <= 0 || a.h <= 0 {
		return b
	}
	if b.w <= 0 || b.h <= 0 {
		return a
	}
	minX := a.x
	if b.x < minX {
		minX = b.x
	}
	minY := a.y
	if b.y < minY {
		minY = b.y
	}
	maxX := a.x + a.w
	if b.x+b.w > maxX {
		maxX = b.x + b.w
	}
	maxY := a.y + a.h
	if b.y+b.h > maxY {
		maxY = b.y + b.h
	}
	return overlayDirtyRect{x: minX, y: minY, w: maxX - minX, h: maxY - minY}
}

func (r *Renderer) overlayUploadRegionPixelsLocked(src []byte, srcWidth int, rect overlayDirtyRect) []byte {
	needed := rect.w * rect.h * 4
	if needed <= 0 {
		return nil
	}
	if cap(r.overlayUploadBuf) < needed {
		r.overlayUploadBuf = make([]byte, needed)
	}
	buf := r.overlayUploadBuf[:needed]
	rowBytes := rect.w * 4
	srcStride := srcWidth * 4
	for row := 0; row < rect.h; row++ {
		srcOff := (rect.y+row)*srcStride + rect.x*4
		dstOff := row * rowBytes
		copy(buf[dstOff:dstOff+rowBytes], src[srcOff:srcOff+rowBytes])
	}
	return buf
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
	tex := dc.renderer.getOrCreateTexture(dc.ctx, pic)
	if tex == nil {
		return
	}
	sx, sy, sw, sh := dc.canvasRectToScreen(x, y, int(pic.Width), int(pic.Height))
	if sw <= 0 || sh <= 0 {
		return
	}
	if err := dc.ctx.DrawTextureEx(tex, gogpu.DrawTextureOptions{
		X:      float32(sx),
		Y:      float32(sy),
		Width:  float32(sw),
		Height: float32(sh),
		Alpha:  alpha,
	}); err != nil {
		slog.Error("DrawPicAlpha: draw failed", "error", err)
	}
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
	tex := dc.renderer.getOrCreateColorTexture(dc.ctx, color)
	if tex == nil {
		return
	}
	if alpha > 1 {
		alpha = 1
	}
	sx, sy, sw, sh := dc.canvasRectToScreen(x, y, w, h)
	if sw <= 0 || sh <= 0 {
		return
	}
	if err := dc.ctx.DrawTextureEx(tex, gogpu.DrawTextureOptions{
		X:      float32(sx),
		Y:      float32(sy),
		Width:  float32(sw),
		Height: float32(sh),
		Alpha:  alpha,
	}); err != nil {
		slog.Error("DrawFillAlpha: draw failed", "error", err)
	}
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
	pic := dc.renderer.getCharPic(num)
	if pic == nil {
		return
	}
	tex := dc.renderer.getOrCreateCharTexture(dc.ctx, num, pic)
	if tex == nil {
		return
	}
	sx, sy, sw, sh := dc.canvasRectToScreen(x, y, 8, 8)
	if sw <= 0 || sh <= 0 {
		return
	}
	if err := dc.ctx.DrawTextureEx(tex, gogpu.DrawTextureOptions{
		X:      float32(sx),
		Y:      float32(sy),
		Width:  float32(sw),
		Height: float32(sh),
		Alpha:  alpha,
	}); err != nil {
		slog.Error("DrawCharacterAlpha: draw failed", "num", num, "error", err)
	}
}

// DrawString renders an entire line of text by compositing all characters
// directly into the CPU overlay buffer. No GPU submission occurs.
func (dc *DrawContext) DrawString(x, y int, text []byte) {
	if len(text) == 0 {
		return
	}
	for i, ch := range text {
		dc.DrawCharacter(x+i*8, y, int(ch))
	}
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
