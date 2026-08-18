package theme

import (
	"image"
	"image/color"

	"github.com/darkliquid/ironwail-go/internal/draw"
	qimage "github.com/darkliquid/ironwail-go/internal/image"
)

// QPicToImage converts a palette-indexed Quake QPic into an RGBA image
// suitable for canvas.DrawImage (spec §3.1, R1.3). Palette index 255 is
// treated as fully transparent, matching Quake's masked-pic convention and
// the renderer's ConvertPaletteToRGBA.
//
// A nil palette falls back to the standard Quake palette so callers can
// bridge pics before the draw manager is initialized.
func QPicToImage(pic *qimage.QPic, palette []byte) *image.RGBA {
	if pic == nil {
		return nil
	}
	if len(palette) < 768 {
		palette = draw.DefaultQuakePalette()
	}

	w := int(pic.Width)
	h := int(pic.Height)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i, idx := range pic.Pixels {
		if i >= w*h {
			break
		}
		var c color.RGBA
		if idx == 255 {
			c = color.RGBA{A: 0}
		} else {
			off := int(idx) * 3
			c = color.RGBA{
				R: palette[off],
				G: palette[off+1],
				B: palette[off+2],
				A: 255,
			}
		}
		img.Set(i%w, i/w, c)
	}
	return img
}
