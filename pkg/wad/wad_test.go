package wad_test

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"testing"

	"github.com/darkliquid/ironwail-go/pkg/wad"
)

func TestPalette(t *testing.T) {
	raw := wad.DefaultQuakePalette()
	if len(raw) != 768 {
		t.Fatalf("DefaultQuakePalette() length = %d, want 768", len(raw))
	}

	pal, err := wad.LoadPalette(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("LoadPalette failed: %v", err)
	}

	// First color in standard Quake palette is black (0, 0, 0)
	if pal[0].R != 0 || pal[0].G != 0 || pal[0].B != 0 || pal[0].A != 255 {
		t.Errorf("pal[0] = %+v, want black with A=255", pal[0])
	}

	// Index 255 is transparent in Quake masked textures
	pixels := []byte{0, 255}
	rgba := pal.ToRGBA(pixels, 2, 1, true)
	if rgba.Pix[3] != 255 {
		t.Errorf("pixel 0 alpha = %d, want 255", rgba.Pix[3])
	}
	if rgba.Pix[7] != 0 {
		t.Errorf("pixel 1 (index 255) alpha = %d, want 0 when transparent=true", rgba.Pix[7])
	}

	// Nearest color lookup
	idx := pal.NearestColor(color.RGBA{R: 0, G: 0, B: 0, A: 255})
	if idx != 0 {
		t.Errorf("NearestColor(black) = %d, want 0", idx)
	}
}

func TestMipTexRoundTrip(t *testing.T) {
	pal, err := wad.LoadPalette(bytes.NewReader(wad.DefaultQuakePalette()))
	if err != nil {
		t.Fatalf("LoadPalette: %v", err)
	}

	// Generate a 32x32 test pattern
	w, h := 32, 32
	rgba := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		rgba[i*4+0] = 128
		rgba[i*4+1] = 64
		rgba[i*4+2] = 32
		rgba[i*4+3] = 255
	}

	lumpData, err := wad.WriteMipTexLump("wall_test", rgba, w, h, pal)
	if err != nil {
		t.Fatalf("WriteMipTexLump failed: %v", err)
	}

	mt, err := wad.ParseMipTex(lumpData)
	if err != nil {
		t.Fatalf("ParseMipTex failed: %v", err)
	}

	if mt.Name != "wall_test" {
		t.Errorf("mt.Name = %q, want wall_test", mt.Name)
	}
	if mt.Width != 32 || mt.Height != 32 {
		t.Errorf("dimensions = %dx%d, want 32x32", mt.Width, mt.Height)
	}

	// Test all 4 mip levels
	expectedDims := [][2]int{{32, 32}, {16, 16}, {8, 8}, {4, 4}}
	for level, want := range expectedDims {
		data, mw, mh, err := mt.MipLevel(level)
		if err != nil {
			t.Fatalf("MipLevel(%d) failed: %v", level, err)
		}
		if mw != want[0] || mh != want[1] {
			t.Errorf("MipLevel(%d) dims = %dx%d, want %dx%d", level, mw, mh, want[0], want[1])
		}
		if len(data) != mw*mh {
			t.Errorf("MipLevel(%d) data len = %d, want %d", level, len(data), mw*mh)
		}

		// Convert to RGBA
		img := mt.ToRGBA(pal, level)
		if img.Bounds().Dx() != mw || img.Bounds().Dy() != mh {
			t.Errorf("ToRGBA(%d) bounds = %v, want %dx%d", level, img.Bounds(), mw, mh)
		}
	}
}

func TestWadWriterAndReader(t *testing.T) {
	pal, err := wad.LoadPalette(bytes.NewReader(wad.DefaultQuakePalette()))
	if err != nil {
		t.Fatalf("LoadPalette: %v", err)
	}

	var buf bytes.Buffer
	writer := wad.NewWriter(&buf)

	// Add a dummy QPic lump
	qpicData, err := wad.WriteQPicLump(make([]byte, 16*16*4), 16, 16, pal)
	if err != nil {
		t.Fatalf("WriteQPicLump failed: %v", err)
	}
	if err := writer.AddLump("pic_demo", wad.TypQPic, qpicData); err != nil {
		t.Fatalf("AddLump(pic_demo) failed: %v", err)
	}

	// Add a MipTex lump
	mipData, err := wad.WriteMipTexLump("tex_wall", make([]byte, 16*16*4), 16, 16, pal)
	if err != nil {
		t.Fatalf("WriteMipTexLump failed: %v", err)
	}
	if err := writer.AddLump("tex_wall", wad.TypMipTex, mipData); err != nil {
		t.Fatalf("AddLump(tex_wall) failed: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read back
	archive, err := wad.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes failed: %v", err)
	}

	if len(archive.Lumps) != 2 {
		t.Fatalf("got %d lumps, want 2", len(archive.Lumps))
	}

	lump1, ok := archive.Get("pic_demo")
	if !ok {
		t.Fatalf("Get(pic_demo) not found")
	}
	if lump1.Type != wad.TypQPic {
		t.Errorf("lump1.Type = %v, want TypQPic", lump1.Type)
	}

	lump2, ok := archive.Get("TEX_WALL") // case-insensitive lookup
	if !ok {
		t.Fatalf("Get(TEX_WALL) not found")
	}
	if lump2.Type != wad.TypMipTex {
		t.Errorf("lump2.Type = %v, want TypMipTex", lump2.Type)
	}
}

func TestWadErrors(t *testing.T) {
	// Corrupt header
	_, err := wad.OpenBytes([]byte("NOT_A_WAD"))
	if err == nil {
		t.Errorf("expected error for bad header magic")
	}

	// Duplicate lumps
	var buf bytes.Buffer
	lumps := []wad.WadLump{
		{Name: "same", Type: wad.TypQPic, Data: []byte{1, 2, 3}},
		{Name: "SAME", Type: wad.TypQPic, Data: []byte{4, 5, 6}},
	}
	err = wad.WriteWad(&buf, lumps)
	if err == nil {
		t.Errorf("expected error for duplicate lump names")
	}
}

func TestQPic(t *testing.T) {
	pal, err := wad.LoadPalette(bytes.NewReader(wad.DefaultQuakePalette()))
	if err != nil {
		t.Fatalf("LoadPalette: %v", err)
	}

	// 1. Invalid short data
	if _, err := wad.ParseQPic([]byte{1, 2, 3}); err == nil {
		t.Errorf("ParseQPic short data: expected error, got nil")
	}

	// 2. Valid QPic
	rgba := make([]byte, 8*8*4)
	for i := range rgba {
		rgba[i] = 128
	}
	lumpData, err := wad.WriteQPicLump(rgba, 8, 8, pal)
	if err != nil {
		t.Fatalf("WriteQPicLump failed: %v", err)
	}

	pic, err := wad.ParseQPic(lumpData)
	if err != nil {
		t.Fatalf("ParseQPic failed: %v", err)
	}
	if pic.Width != 8 || pic.Height != 8 {
		t.Errorf("pic dimensions = %dx%d, want 8x8", pic.Width, pic.Height)
	}
	if len(pic.Pixels) != 64 {
		t.Errorf("pic.Pixels len = %d, want 64", len(pic.Pixels))
	}

	// 3. SubPic
	sub := pic.SubPic(2, 2, 4, 4)
	if sub.Width != 4 || sub.Height != 4 {
		t.Errorf("sub dimensions = %dx%d, want 4x4", sub.Width, sub.Height)
	}
	if len(sub.Pixels) != 16 {
		t.Errorf("sub.Pixels len = %d, want 16", len(sub.Pixels))
	}

	// 4. SubPic out of bounds clamping
	subOOB := pic.SubPic(6, 6, 8, 8)
	if subOOB.Width != 2 || subOOB.Height != 2 {
		t.Errorf("subOOB dimensions = %dx%d, want 2x2", subOOB.Width, subOOB.Height)
	}

	// 5. SubPic completely out of bounds
	subEmpty := pic.SubPic(10, 10, 4, 4)
	if subEmpty.Width != 0 || subEmpty.Height != 0 {
		t.Errorf("subEmpty dimensions = %dx%d, want 0x0", subEmpty.Width, subEmpty.Height)
	}

	// 6. ToRGBA
	img := pic.ToRGBA(pal, false)
	if img.Bounds().Dx() != 8 || img.Bounds().Dy() != 8 {
		t.Errorf("img bounds = %v, want 8x8", img.Bounds())
	}
}

func TestPlaceholderWad(t *testing.T) {
	var buf bytes.Buffer
	if err := wad.WritePlaceholderWad(&buf); err != nil {
		t.Fatalf("WritePlaceholderWad failed: %v", err)
	}

	archive, err := wad.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes on placeholder wad failed: %v", err)
	}

	expectedLumps := []string{"palette.lmp", "gfx/qplaque.lmp", "gfx/mainmenu.lmp", "gfx/m_surfs.lmp"}
	for _, name := range expectedLumps {
		lump, ok := archive.Get(name)
		if !ok {
			t.Errorf("placeholder wad missing lump %q", name)
			continue
		}
		if len(lump.Data) == 0 {
			t.Errorf("placeholder lump %q has empty data", name)
		}
	}
}

func TestWriteMipTexErrors(t *testing.T) {
	pal := wad.DefaultPalette()

	// Empty name
	if _, err := wad.WriteMipTexLump("", make([]byte, 16*16*4), 16, 16, pal); err == nil {
		t.Errorf("expected error for empty miptex name")
	}

	// Non-multiple of 16
	if _, err := wad.WriteMipTexLump("bad", make([]byte, 15*16*4), 15, 16, pal); err == nil {
		t.Errorf("expected error for non-multiple-of-16 width")
	}

	// Buffer too small
	if _, err := wad.WriteMipTexLump("short", make([]byte, 10), 16, 16, pal); err == nil {
		t.Errorf("expected error for buffer too small")
	}
}

func TestHardeningEdgeCases(t *testing.T) {
	// 1. QPic 32-bit dimension overflow rejection
	data := make([]byte, 16)
	binary.LittleEndian.PutUint32(data[0:4], 65536)
	binary.LittleEndian.PutUint32(data[4:8], 65536)
	if _, err := wad.ParseQPic(data); err == nil {
		t.Errorf("expected error for overflowing QPic dimensions")
	}

	// 2. MipLevel offset out of bounds rejection
	m := &wad.MipTex{
		Width:   16,
		Height:  16,
		Offsets: [4]uint32{0, 256, 320, 336},
		Pixels:  make([]byte, 100), // less than 256
	}
	if _, _, _, err := m.MipLevel(0); err == nil {
		t.Errorf("expected error for MipLevel offset out of bounds")
	}

	// 3. AlphaEdgeFix with zero dimensions does not panic
	zeroImg := image.NewRGBA(image.Rect(0, 0, 0, 0))
	wad.AlphaEdgeFix(zeroImg) // should not panic

	// 4. LoadWad with bytes.Reader detects size
	var buf bytes.Buffer
	if err := wad.WriteEmptyWad(&buf); err != nil {
		t.Fatalf("WriteEmptyWad failed: %v", err)
	}
	archive, err := wad.LoadWad(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("LoadWad with bytes.Reader failed: %v", err)
	}
	if len(archive.LumpList) != 0 {
		t.Errorf("expected 0 lumps, got %d", len(archive.LumpList))
	}
}

