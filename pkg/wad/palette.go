package wad

import (
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"io"
)

// Palette represents the Quake 256-color palette as an array of RGBA colors.
type Palette [256]color.RGBA

// StandardQuakePaletteHex is the exact 768-byte RGB palette from Quake's gfx/palette.lmp.
const StandardQuakePaletteHex = "0000000f0f0f1f1f1f2f2f2f3f3f3f4b4b4b5b5b5b6b6b6b7b7b7b8b8b8b9b9b9babababbbbbbbcbcbcbdbdbdbebebeb0f0b07170f0b1f170b271b0f2f2313372b173f2f174b371b533b1b5b431f634b1f6b531f73571f7b5f238367238f6f230b0b0f13131b1b1b272727332f2f3f37374b3f3f574747674f4f735b5b7f63638b6b6b977373a37b7baf8383bb8b8bcb0000000707000b0b001313001b1b002323002b2b072f2f073737073f3f074747074b4b0b53530b5b5b0b63630b6b6b0f0700000f00001700001f00002700002f00003700003f00004700004f00005700005f00006700006f00007700007f00001313001b1b002323002f2b00372f004337004b3b075743075f47076b4b0b77530f8357138b5b13975f1ba3631faf67232313072f170b3b1f0f4b2313572b17632f1f7337237f3b2b8f43339f4f33af632fbf772fcf8f2bdfab27efcb1ffff31b0b07001b13002b230f372b1347331b533723633f2b6f47337f533f8b5f479b6b53a77b5fb7876bc3937bd3a38be3b397ab8ba39f7f979373878b677b7f5b6f7753636b4b575f3f4b5737434b2f3743272f371f232b171b231313170b0b0f0707bb739faf6b8fa35f839757778b4f6b7f4b5f7343536b3b4b5f333f532b3747232b3b1f232f171b231313170b0b0f0707dbc3bbcbb3a7bfa39baf978ba3877b977b6f876f5f7b63536b57475f4b3b533f33433327372b1f271f171b130f0f0b076f837b677b6f5f7367576b5f4f6357475b4f3f5347374b3f2f43372b3b2f2333271f2b1f1723170f1b130b130b070b07fff31befdf17dbcb13cbb70fbba70fab970b9b83078b73077b63076b53005b47004b37003b2b002b1f001b0f000b07000000ff0b0bef1313df1b1bcf2323bf2b2baf2f2f9f2f2f8f2f2f7f2f2f6f2f2f5f2b2b4f23233f1b1b2f13131f0b0b0f2b00003b00004b07005f07006f0f007f1707931f07a3270bb7330fc34b1bcf632bdb7f3be3974fe7ab5fefbf77f7d38ba77b3bb79b37c7c337e7e3577fbfffabe7ffd7ffff6700008b0000b30000d70000ff0000fff393fff7c7ffffff9f5b53"

// DefaultQuakePalette returns the decoded 768-byte RGB Quake palette.
func DefaultQuakePalette() []byte {
	pal, err := hex.DecodeString(StandardQuakePaletteHex)
	if err != nil || len(pal) != 768 {
		panic("invalid default quake palette hex")
	}
	return pal
}

// DefaultPalette returns the default Quake Palette.
func DefaultPalette() Palette {
	p, err := LoadPaletteBytes(DefaultQuakePalette())
	if err != nil {
		panic(err)
	}
	return p
}

// LoadPalette reads a 768-byte Quake palette from r and returns Palette.
func LoadPalette(r io.Reader) (Palette, error) {
	data := make([]byte, 768)
	if _, err := io.ReadFull(r, data); err != nil {
		return Palette{}, fmt.Errorf("read palette: %w", err)
	}
	return LoadPaletteBytes(data)
}

// LoadPaletteBytes loads a 768-byte palette from memory.
func LoadPaletteBytes(data []byte) (Palette, error) {
	var p Palette
	if len(data) < 768 {
		return p, fmt.Errorf("palette data too short: %d bytes (want 768)", len(data))
	}
	for i := 0; i < 256; i++ {
		p[i] = color.RGBA{
			R: data[i*3+0],
			G: data[i*3+1],
			B: data[i*3+2],
			A: 255,
		}
	}
	return p, nil
}

// ToRGBA converts palette-indexed pixel data into an image.RGBA.
// If transparent is true, index 255 is treated as alpha = 0.
func (p Palette) ToRGBA(data []byte, width, height int, transparent bool) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i, idx := range data {
		if i >= width*height {
			break
		}
		c := p[idx]
		if transparent && idx == 255 {
			c.A = 0
		}
		img.Pix[i*4+0] = c.R
		img.Pix[i*4+1] = c.G
		img.Pix[i*4+2] = c.B
		img.Pix[i*4+3] = c.A
	}
	return img
}

// NearestColor finds the palette index closest in Euclidean RGB distance to c.
func (p Palette) NearestColor(c color.RGBA) byte {
	if c.A < 128 {
		return 255 // transparent in Quake
	}
	bestIdx := 0
	bestDist := int(^uint(0) >> 1)
	cr, cg, cb := int(c.R), int(c.G), int(c.B)
	for i := 0; i < 256; i++ {
		pr, pg, pb := int(p[i].R), int(p[i].G), int(p[i].B)
		dr := cr - pr
		dg := cg - pg
		db := cb - pb
		dist := dr*dr + dg*dg + db*db
		if dist < bestDist {
			bestDist = dist
			bestIdx = i
		}
	}
	return byte(bestIdx)
}

// EncodePaletted quantizes an RGBA pixel buffer into single-byte palette indices.
func EncodePaletted(rgba []byte, width, height int, pal Palette) []byte {
	out := make([]byte, width*height)
	for i := 0; i < width*height; i++ {
		c := color.RGBA{
			R: rgba[i*4+0],
			G: rgba[i*4+1],
			B: rgba[i*4+2],
			A: rgba[i*4+3],
		}
		out[i] = pal.NearestColor(c)
	}
	return out
}
