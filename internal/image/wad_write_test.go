package image

import (
	"bytes"
	"image/color"
	"io"
	"testing"
)

// grayscalePalette builds a deterministic 256-entry grayscale palette.
func grayscalePalette() Palette {
	var p Palette
	for i := 0; i < 256; i++ {
		p[i] = color.RGBA{R: byte(i), G: byte(i), B: byte(i), A: 255}
	}
	return p
}

// solidRGBA returns width×height RGBA pixels painted in a single colour.
func solidRGBA(width, height int, r, g, b, a byte) []byte {
	pix := make([]byte, width*height*4)
	for i := 0; i < width*height; i++ {
		pix[i*4+0] = r
		pix[i*4+1] = g
		pix[i*4+2] = b
		pix[i*4+3] = a
	}
	return pix
}

func TestWriteWadQPicRoundTrip(t *testing.T) {
	pal := grayscalePalette()
	rgba := solidRGBA(16, 8, 200, 200, 200, 255) // pure gray: exact palette index 200
	lumpData, err := WriteQPicLump(rgba, 16, 8, pal)
	if err != nil {
		t.Fatalf("WriteQPicLump: %v", err)
	}

	var buf bytes.Buffer
	if err := WriteWad(&buf, []WadLump{{Name: "testpic", Type: TypQPic, Data: lumpData}}); err != nil {
		t.Fatalf("WriteWad: %v", err)
	}

	wad, err := LoadWad(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("LoadWad: %v", err)
	}
	lump, ok := wad.Lumps["testpic"]
	if !ok {
		t.Fatal("wad missing lump testpic")
	}
	if lump.Type != TypQPic {
		t.Errorf("lump type = %v, want TypQPic", lump.Type)
	}
	pic, err := ParseQPic(lump.Data)
	if err != nil {
		t.Fatalf("ParseQPic: %v", err)
	}
	if pic.Width != 16 || pic.Height != 8 {
		t.Errorf("qpic size = %dx%d, want 16x8", pic.Width, pic.Height)
	}
	// Every pixel must quantise to palette index 200 (grayscale: R=200).
	for i, idx := range pic.Pixels {
		if idx != 200 {
			t.Fatalf("pixel %d = index %d, want 200", i, idx)
		}
	}
}

func TestWriteWadMipTexRoundTrip(t *testing.T) {
	pal := grayscalePalette()
	const w, h = 64, 64
	rgba := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			v := byte((x + y) * 2) // pure-gray gradient: exact palette indices
			rgba[i+0] = v
			rgba[i+1] = v
			rgba[i+2] = v
			rgba[i+3] = 255
		}
	}

	lumpData, err := WriteMipTexLump("wall_gray", rgba, w, h, pal)
	if err != nil {
		t.Fatalf("WriteMipTexLump: %v", err)
	}
	// Lump length must match header + all four mip sizes.
	if want := 40 + 64*64 + 32*32 + 16*16 + 8*8; len(lumpData) != want {
		t.Fatalf("miptex lump length = %d, want %d", len(lumpData), want)
	}

	mt, err := ParseMipTex(lumpData)
	if err != nil {
		t.Fatalf("ParseMipTex: %v", err)
	}
	if mt.Name != "wall_gray" {
		t.Errorf("miptex name = %q, want wall_gray", mt.Name)
	}
	if mt.Width != w || mt.Height != h {
		t.Errorf("miptex size = %dx%d, want %dx%d", mt.Width, mt.Height, w, h)
	}

	for level := 0; level < 4; level++ {
		pixels, lw, lh, err := mt.MipLevel(level)
		if err != nil {
			t.Fatalf("MipLevel(%d): %v", level, err)
		}
		wantW, wantH := w>>uint(level), h>>uint(level)
		if lw != wantW || lh != wantH {
			t.Errorf("mip %d size = %dx%d, want %dx%d", level, lw, lh, wantW, wantH)
		}
		if len(pixels) != wantW*wantH {
			t.Errorf("mip %d pixel count = %d, want %d", level, len(pixels), wantW*wantH)
		}
	}

	// Mip 0 must reproduce the source after palette round-tripping.
	mip0, _, _, _ := mt.MipLevel(0)
	back := RGBAFromPalette(mip0, pal, w, h)
	for i := 0; i < w*h; i++ {
		if back[i*4+0] != rgba[i*4+0] {
			t.Fatalf("mip0 pixel %d red = %d, want %d (palette round-trip)", i, back[i*4+0], rgba[i*4+0])
		}
	}
}

func TestWriteWadEmptyAndDuplicates(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteWad(&buf, nil); err != nil {
		t.Fatalf("WriteWad(empty): %v", err)
	}
	wad, err := LoadWad(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("LoadWad(empty): %v", err)
	}
	if len(wad.Lumps) != 0 {
		t.Errorf("empty wad has %d lumps", len(wad.Lumps))
	}

	// Duplicate names after CleanupName must be rejected.
	err = WriteWad(io.Discard, []WadLump{
		{Name: "Pic", Type: TypQPic, Data: []byte("x")},
		{Name: "pic", Type: TypQPic, Data: []byte("y")},
	})
	if err == nil {
		t.Error("WriteWad accepted duplicate cleaned names")
	}
}

func TestWriteMipTexRejectsBadInput(t *testing.T) {
	pal := grayscalePalette()
	if _, err := WriteMipTexLump("t", solidRGBA(64, 63, 1, 2, 3, 255), 64, 63, pal); err == nil {
		t.Error("WriteMipTexLump accepted 64x63 (not multiple of 16)")
	}
	if _, err := WriteMipTexLump("t", nil, 0, 0, pal); err == nil {
		t.Error("WriteMipTexLump accepted zero size")
	}
	if _, err := WriteQPicLump(solidRGBA(4, 4, 1, 2, 3, 255), 4, 0, pal); err == nil {
		t.Error("WriteQPicLump accepted zero height")
	}
	if _, err := WriteQPicLump(make([]byte, 4), 8, 8, pal); err == nil {
		t.Error("WriteQPicLump accepted undersized rgba buffer")
	}
}

func TestWriteMipTexNameCleanupAndTruncation(t *testing.T) {
	pal := grayscalePalette()
	longName := "This_Is_A_Very_Long_Texture_Name" // > 15 bytes
	lump, err := WriteMipTexLump(longName, solidRGBA(16, 16, 5, 5, 5, 255), 16, 16, pal)
	if err != nil {
		t.Fatalf("WriteMipTexLump: %v", err)
	}
	mt, err := ParseMipTex(lump)
	if err != nil {
		t.Fatalf("ParseMipTex: %v", err)
	}
	if len(mt.Name) > 15 {
		t.Errorf("miptex name %q not truncated to 15 bytes", mt.Name)
	}
	if mt.Name != CleanupName(longName[:15]) {
		t.Errorf("miptex name = %q, want %q", mt.Name, CleanupName(longName[:15]))
	}
}

func TestEncodePalettedTransparencyAndNearest(t *testing.T) {
	var pal Palette
	pal[0] = color.RGBA{R: 0, G: 0, B: 0, A: 255}
	pal[10] = color.RGBA{R: 100, G: 100, B: 100, A: 255}
	pal[255] = color.RGBA{R: 255, G: 0, B: 255, A: 255}

	// Transparent pixels map to index 255.
	rgba := []byte{0, 0, 0, 0, 99, 99, 99, 255, 100, 100, 100, 200}
	got := EncodePaletted(rgba, 3, 1, pal)
	if got[0] != 255 {
		t.Errorf("transparent pixel index = %d, want 255", got[0])
	}
	if got[1] != 10 || got[2] != 10 {
		t.Errorf("opaque pixel indices = %d,%d want 10,10 (nearest)", got[1], got[2])
	}
}

func TestLoadPaletteLmp(t *testing.T) {
	data := make([]byte, 768)
	data[0], data[1], data[2] = 10, 20, 30
	pal, err := LoadPaletteLmp(data)
	if err != nil {
		t.Fatalf("LoadPaletteLmp: %v", err)
	}
	if pal[0] != (color.RGBA{R: 10, G: 20, B: 30, A: 255}) {
		t.Errorf("pal[0] = %v, want RGBA{10,20,30,255}", pal[0])
	}
	if _, err := LoadPaletteLmp(make([]byte, 767)); err == nil {
		t.Error("LoadPaletteLmp accepted 767 bytes")
	}
}