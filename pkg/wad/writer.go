package wad

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// WadLump represents a single lump to be written into a WAD archive.
type WadLump struct {
	Name string
	Type LumpType
	Data []byte
}

// Writer provides sequential construction of a WAD2 archive.
type Writer struct {
	w      io.Writer
	lumps  []WadLump
	closed bool
}

// NewWriter creates a new Writer writing to w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		w:     w,
		lumps: make([]WadLump, 0),
	}
}

// AddLump adds a lump to the writer.
func (w *Writer) AddLump(name string, lType LumpType, data []byte) error {
	if w.closed {
		return fmt.Errorf("wad: writer closed")
	}
	clean := CleanupName(name)
	if clean == "" {
		return fmt.Errorf("wad: empty lump name")
	}
	w.lumps = append(w.lumps, WadLump{
		Name: clean,
		Type: lType,
		Data: data,
	})
	return nil
}

// Close finalizes and writes the WAD2 archive.
func (w *Writer) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	return WriteWad(w.w, w.lumps)
}

// WriteWad writes a complete WAD2 archive containing lumps to w:
// the 12-byte header ("WAD2", NumLumps, InfoTableOfs), each lump's data,
// then the 32-byte-per-lump info table.
func WriteWad(w io.Writer, lumps []WadLump) error {
	if len(lumps) == 0 {
		return WriteEmptyWad(w)
	}

	type placed struct {
		lump     WadLump
		filePos  int
		diskSize int
	}
	placedLumps := make([]placed, len(lumps))
	var pos int64 = 12
	seen := make(map[string]struct{}, len(lumps))
	for i, l := range lumps {
		name := CleanupName(l.Name)
		if name == "" {
			return fmt.Errorf("empty lump name")
		}
		if _, dup := seen[name]; dup {
			return fmt.Errorf("duplicate wad lump %q", name)
		}
		seen[name] = struct{}{}
		placedLumps[i] = placed{
			lump:     WadLump{Name: name, Type: l.Type, Data: l.Data},
			filePos:  int(pos),
			diskSize: len(l.Data),
		}
		pos += int64(len(l.Data))
	}

	totalSize := pos + int64(len(placedLumps))*32
	if totalSize > math.MaxInt32 {
		return fmt.Errorf("wad exceeds 2 GiB format limit")
	}

	tableOfs := pos
	var header Header
	copy(header.Identification[:], Magic)
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

// WriteEmptyWad writes a structurally valid WAD2 with no lumps.
func WriteEmptyWad(w io.Writer) error {
	var header Header
	copy(header.Identification[:], Magic)
	header.NumLumps = 0
	header.InfoTableOfs = 12

	if err := binary.Write(w, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("write empty wad header: %w", err)
	}
	return nil
}

// WritePlaceholderWad writes a minimal WAD with a grayscale palette and dummy
// console UI lumps, useful for tests and tooling that need a valid WAD
// without shipping game assets.
func WritePlaceholderWad(w io.Writer) error {
	palette := make([]byte, 768)
	for i := 0; i < 256; i++ {
		palette[i*3+0] = byte(i)
		palette[i*3+1] = byte(i)
		palette[i*3+2] = byte(i)
	}

	createQPic := func(width, height uint32, color byte) []byte {
		data := make([]byte, 8+width*height)
		binary.LittleEndian.PutUint32(data[0:4], width)
		binary.LittleEndian.PutUint32(data[4:8], height)
		for i := uint32(0); i < width*height; i++ {
			data[8+i] = color
		}
		return data
	}

	lumps := []WadLump{
		{Name: "palette.lmp", Type: TypPalette, Data: palette},
		{Name: "gfx/qplaque.lmp", Type: TypConsolePic, Data: createQPic(320, 20, 50)},
		{Name: "gfx/mainmenu.lmp", Type: TypConsolePic, Data: createQPic(320, 180, 100)},
		{Name: "gfx/m_surfs.lmp", Type: TypConsolePic, Data: createQPic(24, 20, 200)},
	}

	return WriteWad(w, lumps)
}


// WriteQPicLump serializes a QPic lump: uint32 width, uint32 height, then
// width×height palette-index pixels.
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

// WriteMipTexLump serializes a MipTex lump: the 40-byte header (16-byte name,
// uint32 width, uint32 height, 4 uint32 mip offsets) followed by four
// palette-index mip levels.
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

	cleanName := CleanupName(name)
	if cleanName == "" {
		return nil, fmt.Errorf("miptex name is empty")
	}
	if len(cleanName) > 15 {
		cleanName = cleanName[:15]
	}

	var offsets [4]uint32
	curOff := uint32(40) // header size
	var mipSizes [4]int

	w, h := width, height
	for i := 0; i < 4; i++ {
		offsets[i] = curOff
		mipSizes[i] = w * h
		curOff += uint32(w * h)
		w >>= 1
		h >>= 1
	}

	out := make([]byte, curOff)
	copy(out[:16], cleanName)
	binary.LittleEndian.PutUint32(out[16:20], uint32(width))
	binary.LittleEndian.PutUint32(out[20:24], uint32(height))
	for i := 0; i < 4; i++ {
		binary.LittleEndian.PutUint32(out[24+i*4:28+i*4], offsets[i])
	}

	// Level 0: full resolution
	lvl0 := EncodePaletted(rgba, width, height, pal)
	copy(out[offsets[0]:offsets[0]+uint32(mipSizes[0])], lvl0)

	// Levels 1..3: box downsample
	srcRGBA := rgba
	sw, sh := width, height
	for i := 1; i < 4; i++ {
		dw, dh := sw/2, sh/2
		downsampled := boxDownsample2x(srcRGBA, sw, sh)
		lvl := EncodePaletted(downsampled, dw, dh, pal)
		copy(out[offsets[i]:offsets[i]+uint32(mipSizes[i])], lvl)
		srcRGBA = downsampled
		sw, sh = dw, dh
	}

	return out, nil
}

func boxDownsample2x(src []byte, sw, sh int) []byte {
	dw, dh := sw/2, sh/2
	dst := make([]byte, dw*dh*4)
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			sx := x * 2
			sy := y * 2
			var r, g, b, a int
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					idx := ((sy+dy)*sw + (sx + dx)) * 4
					r += int(src[idx+0])
					g += int(src[idx+1])
					b += int(src[idx+2])
					a += int(src[idx+3])
				}
			}
			di := (y*dw + x) * 4
			dst[di+0] = byte(r / 4)
			dst[di+1] = byte(g / 4)
			dst[di+2] = byte(b / 4)
			dst[di+3] = byte(a / 4)
		}
	}
	return dst
}
