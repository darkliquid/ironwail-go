// Package csqc provides pure image/rect math helpers for the CSQC draw
// hooks. These helpers have no *Game dependencies, so they live in a
// subpackage and the game root keeps thin delegators.
//
// # Original C lineage
//
//   - csqc_draw.c — CSQC_DrawPic / CSQC_DrawFill / CSQC_DrawCharacter rect
//     clipping and palette-index resolution.
package csqc

import (
	"math"

	qimage "github.com/darkliquid/ironwail-go/internal/image"
)

// ClipRect is a normalized clip area applied to CSQC draw calls.
type ClipRect struct {
	Enabled bool
	X       float32
	Y       float32
	Width   float32
	Height  float32
}

// ClampUnitFloat32 clamps v into the [0, 1] unit range.
func ClampUnitFloat32(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// NearestPaletteIndex finds the palette index whose RGB triple is closest to
// the given unit-range color.
func NearestPaletteIndex(r, g, b float32, palette []byte) byte {
	if len(palette) < 3 {
		return 0
	}

	targetR := int(ClampUnitFloat32(r)*255 + 0.5)
	targetG := int(ClampUnitFloat32(g)*255 + 0.5)
	targetB := int(ClampUnitFloat32(b)*255 + 0.5)

	bestIdx := 0
	bestDist := math.MaxInt
	for i := 0; i+2 < len(palette); i += 3 {
		dr := targetR - int(palette[i])
		dg := targetG - int(palette[i+1])
		db := targetB - int(palette[i+2])
		dist := dr*dr + dg*dg + db*db
		if dist < bestDist {
			bestDist = dist
			bestIdx = i / 3
		}
	}

	return byte(bestIdx)
}

// ClipDrawRect clips a draw rect against a clip area, returning both the
// clipped destination rect and the normalized source rect.
func ClipDrawRect(clip ClipRect, x, y, width, height float32) (drawX, drawY, drawW, drawH, srcX, srcY, srcW, srcH float32, ok bool) {
	if width <= 0 || height <= 0 {
		return 0, 0, 0, 0, 0, 0, 0, 0, false
	}

	drawX, drawY, drawW, drawH = x, y, width, height
	srcX, srcY, srcW, srcH = 0, 0, 1, 1
	if !clip.Enabled {
		return drawX, drawY, drawW, drawH, srcX, srcY, srcW, srcH, true
	}

	left := max(x, clip.X)
	top := max(y, clip.Y)
	right := min(x+width, clip.X+clip.Width)
	bottom := min(y+height, clip.Y+clip.Height)
	if right <= left || bottom <= top {
		return 0, 0, 0, 0, 0, 0, 0, 0, false
	}

	drawX = left
	drawY = top
	drawW = right - left
	drawH = bottom - top
	srcX = (left - x) / width
	srcY = (top - y) / height
	srcW = drawW / width
	srcH = drawH / height
	return drawX, drawY, drawW, drawH, srcX, srcY, srcW, srcH, true
}

// SubPicFromNormalizedRect extracts a sub-pic from normalized [0,1] source
// bounds.
func SubPicFromNormalizedRect(pic *qimage.QPic, srcX, srcY, srcW, srcH float32) *qimage.QPic {
	if pic == nil || pic.Width == 0 || pic.Height == 0 {
		return nil
	}

	startX := ClampUnitFloat32(srcX)
	startY := ClampUnitFloat32(srcY)
	endX := ClampUnitFloat32(srcX + srcW)
	endY := ClampUnitFloat32(srcY + srcH)
	if endX <= startX || endY <= startY {
		return &qimage.QPic{}
	}

	picWidth := float64(pic.Width)
	picHeight := float64(pic.Height)
	x1 := int(math.Floor(float64(startX) * picWidth))
	y1 := int(math.Floor(float64(startY) * picHeight))
	x2 := int(math.Ceil(float64(endX) * picWidth))
	y2 := int(math.Ceil(float64(endY) * picHeight))
	return pic.SubPic(x1, y1, x2-x1, y2-y1)
}

// ScaleQPic nearest-neighbor scales a pic to the target dimensions, returning
// the original when no scaling is needed.
func ScaleQPic(pic *qimage.QPic, width, height int) *qimage.QPic {
	if pic == nil || width <= 0 || height <= 0 || pic.Width == 0 || pic.Height == 0 {
		return nil
	}
	if int(pic.Width) == width && int(pic.Height) == height {
		return pic
	}

	srcW := int(pic.Width)
	srcH := int(pic.Height)
	scaled := &qimage.QPic{
		Width:  uint32(width),
		Height: uint32(height),
		Pixels: make([]byte, width*height),
	}
	for y := range height {
		srcY := y * srcH / height
		for x := range width {
			srcX := x * srcW / width
			scaled.Pixels[y*width+x] = pic.Pixels[srcY*srcW+srcX]
		}
	}
	return scaled
}

// PreparePic clips, sub-pics, and scales a pic for drawing at the given
// destination rect. It returns the final pixel-space draw position and pic.
func PreparePic(pic *qimage.QPic, posX, posY, sizeX, sizeY, srcX, srcY, srcW, srcH float32, clip ClipRect) (int, int, *qimage.QPic, bool) {
	drawX, drawY, drawW, drawH, clipSrcX, clipSrcY, clipSrcW, clipSrcH, ok := ClipDrawRect(clip, posX, posY, sizeX, sizeY)
	if !ok {
		return 0, 0, nil, false
	}

	srcX += srcW * clipSrcX
	srcY += srcH * clipSrcY
	srcW *= clipSrcW
	srcH *= clipSrcH

	subPic := SubPicFromNormalizedRect(pic, srcX, srcY, srcW, srcH)
	if subPic == nil || subPic.Width == 0 || subPic.Height == 0 {
		return 0, 0, nil, false
	}

	drawPic := ScaleQPic(subPic, int(drawW), int(drawH))
	if drawPic == nil || drawPic.Width == 0 || drawPic.Height == 0 {
		return 0, 0, nil, false
	}

	return int(drawX), int(drawY), drawPic, true
}
