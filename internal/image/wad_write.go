package image

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// WadLump is a single lump for WriteWad. Name is the directory key (cleaned
// via CleanupName at write time); Type selects the lump's interpretation.
type WadLump struct {
	Name string
	Type LumpType
	Data []byte
}

// WriteWad writes a WAD2 archive to w: the 12-byte header ("WAD2",
// NumLumps, InfoTableOfs), each lump's data sequentially, then the
// 32-byte-per-lump info table. The layout mirrors what LoadWad parses (and
// Quake's W_LoadWadFile): each LumpInfo is FilePos | DiskSize | Size |
// Type | Compression | Pad1 | Pad2 | Name[16], all little-endian.
//
// Names are normalized with CleanupName (lowercase, NUL/space stripped);
// duplicate normalized names are rejected, and lump data must stay within
// the format's 2 GiB offset range.
func WriteWad(w io.Writer, lumps []WadLump) error {
	if len(lumps) == 0 {
		return writeEmptyWad(w)
	}

	type placed struct {
		lump     WadLump
		filePos  int
		diskSize int
	}
	placedLumps := make([]placed, len(lumps))
	seen := make(map[string]struct{}, len(lumps))
	pos := 12 // header size
	for i, l := range lumps {
		name := CleanupName(l.Name)
		placedLumps[i] = placed{lump: WadLump{Name: name, Type: l.Type, Data: l.Data}, filePos: pos, diskSize: len(l.Data)}
		pos += len(l.Data)
	}
	for _, p := range placedLumps {
		if p.lump.Name == "" {
			return fmt.Errorf("wad lump with empty name")
		}
		if _, dup := seen[p.lump.Name]; dup {
			return fmt.Errorf("duplicate wad lump %q", p.lump.Name)
		}
		seen[p.lump.Name] = struct{}{}
	}
	if int64(pos) > int64(maxInt32) {
		return fmt.Errorf("wad exceeds the 2 GiB format limit")
	}

	tableOfs := pos
	var header struct {
		Identification [4]byte
		NumLumps       int32
		InfoTableOfs   int32
	}
	copy(header.Identification[:], Wad2Id)
	header.NumLumps = int32(len(placedLumps))
	header.InfoTableOfs = int32(tableOfs)
	if err := binary.Write(w, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("write wad header: %w", err)
	}

	for _, p := range placedLumps {
		if _, err := w.Write(p.lump.Data); err != nil {
			return fmt.Errorf("write wad lump %q: %w", p.lump.Name, err)
		}
	}

	for _, p := range placedLumps {
		info := &LumpInfo{
			FilePos:  int32(p.filePos),
			DiskSize: int32(p.diskSize),
			Size:     int32(p.diskSize),
			Type:     p.lump.Type,
		}
		copy(info.Name[:], p.lump.Name)
		if err := binary.Write(w, binary.LittleEndian, info); err != nil {
			return fmt.Errorf("write wad info for %q: %w", p.lump.Name, err)
		}
	}
	return nil
}

const maxInt32 = int(^uint32(0) >> 1)

// writeEmptyWad writes a structurally valid WAD2 with no lumps.
func writeEmptyWad(w io.Writer) error {
	var header struct {
		Identification [4]byte
		NumLumps       int32
		InfoTableOfs   int32
	}
	copy(header.Identification[:], Wad2Id)
	if err := binary.Write(w, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("write wad header: %w", err)
	}
	return nil
}

// WriteQPicLump serialises a QPic lump: uint32 width, uint32 height, then
// width×height palette-index pixels (Quake's qpic_t layout, parsed by
// ParseQPic). Source pixels are RGBA (4 bytes per pixel, row-major).
func WriteQPicLump(rgba []byte, width, height int, pal Palette) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("qpic size must be positive, got %dx%d", width, height)
	}
	if len(rgba) < width*height*4 {
		return nil, fmt.Errorf("qpic rgba buffer too small: %d bytes for %dx%d", len(rgba), width, height)
	}
	out := make([]byte, 8+width*height)
	binary.LittleEndian.PutUint32(out[0:4], uint32(width))
	binary.LittleEndian.PutUint32(out[4:8], uint32(height))
	copy(out[8:], EncodePaletted(rgba, width, height, pal))
	return out, nil
}

// WriteMipTexLump serialises a MipTex lump: the 40-byte header (16-byte
// name, uint32 width, uint32 height, 4 uint32 mip offsets) followed by four
// palette-index mip levels (parsed by ParseMipTex / MipLevel). Mip level n
// is box-downsampled 2^n times from the full-resolution RGBA source.
//
// Classic Quake textures are square power-of-two with width/height
// divisible by 16, which keeps every level's pixel count integral; inputs
// violating that are rejected.
func WriteMipTexLump(name string, rgba []byte, width, height int, pal Palette) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("miptex size must be positive, got %dx%d", width, height)
	}
	if width%16 != 0 || height%16 != 0 {
		return nil, fmt.Errorf("miptex dimensions must be multiples of 16, got %dx%d", width, height)
	}
	if len(rgba) < width*height*4 {
		return nil, fmt.Errorf("miptex rgba buffer too small: %d bytes for %dx%d", len(rgba), width, height)
	}

	clean := CleanupName(name)
	if clean == "" {
		return nil, fmt.Errorf("miptex name is empty")
	}
	if len(clean) > 15 {
		clean = clean[:15] // 16-byte field, NUL-terminated
	}

	// Precompute each mip level's dimensions and byte size.
	var mipW, mipH, mipSize [4]int
	for i := 0; i < 4; i++ {
		mipW[i], mipH[i] = width>>i, height>>i
		if mipW[i] < 1 {
			mipW[i] = 1
		}
		if mipH[i] < 1 {
			mipH[i] = 1
		}
		mipSize[i] = mipW[i] * mipH[i]
	}

	out := make([]byte, 40+mipSize[0]+mipSize[1]+mipSize[2]+mipSize[3])
	copy(out[0:16], clean)
	binary.LittleEndian.PutUint32(out[16:20], uint32(width))
	binary.LittleEndian.PutUint32(out[20:24], uint32(height))

	off := 40
	for i := 0; i < 4; i++ {
		binary.LittleEndian.PutUint32(out[24+i*4:28+i*4], uint32(off))
		off += mipSize[i]
	}

	// Mip 0 uses the source directly; deeper levels are box-averaged from
	// the previous level's RGBA (each 2×2 block becomes one pixel).
	dst := 40
	curRGBA := rgba
	curW, curH := width, height
	for level := 0; level < 4; level++ {
		levelSize := mipSize[level]
		indexed := EncodePaletted(curRGBA, curW, curH, pal)
		copy(out[dst:dst+levelSize], indexed)
		dst += levelSize
		if level < 3 {
			curRGBA, curW, curH = downsampleRGBA(curRGBA, curW, curH)
		}
	}
	return out[:dst], nil
}

// downsampleRGBA halves both dimensions by averaging each 2×2 block.
func downsampleRGBA(rgba []byte, w, h int) ([]byte, int, int) {
	nw, nh := w/2, h/2
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	out := make([]byte, nw*nh*4)
	for y := 0; y < nh; y++ {
		for x := 0; x < nw; x++ {
			var r, g, b, a int
			count := 0
			for dy := 0; dy < 2; dy++ {
				sy := y*2 + dy
				if sy >= h {
					continue
				}
				for dx := 0; dx < 2; dx++ {
					sx := x*2 + dx
					if sx >= w {
						continue
					}
					i := (sy*w + sx) * 4
					r += int(rgba[i+0])
					g += int(rgba[i+1])
					b += int(rgba[i+2])
					a += int(rgba[i+3])
					count++
				}
			}
			if count == 0 {
				count = 1
			}
			o := (y*nw + x) * 4
			out[o+0] = byte(r / count)
			out[o+1] = byte(g / count)
			out[o+2] = byte(b / count)
			out[o+3] = byte(a / count)
		}
	}
	return out, nw, nh
}

// EncodePaletted quantises RGBA pixels (4 bytes each, row-major) to Quake
// palette indices via nearest-colour matching (squared RGB distance).
// Pixels with alpha below 128 map to index 255, Quake's transparent entry.
func EncodePaletted(rgba []byte, width, height int, pal Palette) []byte {
	out := make([]byte, width*height)
	for i := 0; i < width*height; i++ {
		src := i * 4
		if rgba[src+3] < 128 {
			out[i] = 255
			continue
		}
		out[i] = nearestPaletteIndex(pal, rgba[src], rgba[src+1], rgba[src+2])
	}
	return out
}

// nearestPaletteIndex returns the palette index closest to r, g, b.
func nearestPaletteIndex(pal Palette, r, g, b byte) byte {
	best := byte(0)
	bestDist := int64(1) << 62
	for i, c := range pal {
		dr := int64(int(r) - int(c.R))
		dg := int64(int(g) - int(c.G))
		db := int64(int(b) - int(c.B))
		dist := dr*dr + dg*dg + db*db
		if dist < bestDist {
			bestDist = dist
			best = byte(i)
		}
	}
	return best
}

// LoadPaletteLmp parses a 768-byte Quake palette.lmp payload (256 RGB
// triplets) into a Palette.
func LoadPaletteLmp(data []byte) (Palette, error) {
	if len(data) != 768 {
		return Palette{}, fmt.Errorf("palette.lmp must be 768 bytes (RGB triplets), got %d", len(data))
	}
	return LoadPalette(bytes.NewReader(data))
}

// DecodeQuakeImage loads an image file, dispatching on extension: .png via
// LoadPNG, .tga via LoadTGA. Everything else is rejected, matching the
// engine's supported source set for asset conversion.
func DecodeQuakeImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		img, err := LoadPNG(f)
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}
		return img, nil
	case ".tga":
		img, err := LoadTGA(f)
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}
		return img, nil
	default:
		return nil, fmt.Errorf("unsupported image format %q (supported: .png, .tga)", filepath.Ext(path))
	}
}

// RGBAFromImage extracts row-major RGBA bytes (4 per pixel) from any
// image.Image, converting non-RGBA colour models as needed. The height of
// the returned buffer is the image's bounds height (top-left origin).
func RGBAFromImage(img image.Image) ([]byte, int, int) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([]byte, w*h*4)
	// Fast path for plain *image.RGBA (as produced by png.Decode for
	// 8-bit RGBA inputs and by LoadTGA).
	if rgba, ok := img.(*image.RGBA); ok {
		copy(out, rgba.Pix[:w*h*4])
		return out, w, h
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.RGBA)
			i := (y*w + x) * 4
			out[i+0], out[i+1], out[i+2], out[i+3] = c.R, c.G, c.B, c.A
		}
	}
	return out, w, h
}