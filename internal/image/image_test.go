package image

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"path/filepath"
	"testing"

	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/testutil"
)

func TestWad(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	f := fs.NewFileSystem()
	err := f.Init(baseDir, "id1")
	testutil.AssertNoError(t, err)
	defer f.Close()

	data, err := f.LoadFile("gfx.wad")
	testutil.AssertNoError(t, err)

	wad, err := LoadWad(bytes.NewReader(data))
	testutil.AssertNoError(t, err)

	if len(wad.Lumps) == 0 {
		t.Fatal("WAD has no lumps")
	}

	// Verify conchars exists; in the real Quake gfx.wad it is stored as TypMipTex
	// (raw 8-bit pixel data), not TypQPic.
	_, ok := wad.Lumps["conchars"]
	if !ok {
		t.Fatal("conchars lump not found in gfx.wad")
	}

	// Find a QPic lump we can parse. "help" is a typical QPic lump in gfx.wad.
	var qpicLump *Lump
	for _, name := range []string{"help", "ttl_main", "loading", "pause"} {
		if l, found := wad.Lumps[name]; found && l.Type == TypQPic {
			lump := l
			qpicLump = &lump
			break
		}
	}
	if qpicLump == nil {
		t.Skip("no known QPic lump found in gfx.wad; skipping parse check")
	}

	qpic, err := ParseQPic(qpicLump.Data)
	testutil.AssertNoError(t, err)

	if qpic.Width == 0 || qpic.Height == 0 {
		t.Errorf("invalid qpic dimensions: %dx%d", qpic.Width, qpic.Height)
	}
}

func TestPalette(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	f := fs.NewFileSystem()
	err := f.Init(baseDir, "id1")
	testutil.AssertNoError(t, err)
	defer f.Close()

	data, err := f.LoadFile("gfx/palette.lmp")
	testutil.AssertNoError(t, err)

	pal, err := LoadPalette(bytes.NewReader(data))
	testutil.AssertNoError(t, err)

	// Check some known colors if possible, or just that it loaded
	if pal[0].A != 255 {
		t.Errorf("expected alpha 255, got %d", pal[0].A)
	}
}

func TestPNG(t *testing.T) {
	// Create a simple PNG in memory
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	err := png.Encode(&buf, img)
	testutil.AssertNoError(t, err)

	decoded, err := LoadPNG(&buf)
	testutil.AssertNoError(t, err)

	if decoded.Bounds().Dx() != 1 || decoded.Bounds().Dy() != 1 {
		t.Errorf("expected 1x1 image, got %dx%d", decoded.Bounds().Dx(), decoded.Bounds().Dy())
	}
}

func TestAlphaEdgeFix(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3, 3))
	// Set center pixel to transparent
	// Set neighbors to some color
	for i := 0; i < 9; i++ {
		if i == 4 {
			continue
		}
		img.Pix[i*4+0] = 255
		img.Pix[i*4+1] = 0
		img.Pix[i*4+2] = 0
		img.Pix[i*4+3] = 255
	}

	AlphaEdgeFix(img)

	if img.Pix[4*4+0] != 255 || img.Pix[4*4+1] != 0 || img.Pix[4*4+2] != 0 {
		t.Errorf("AlphaEdgeFix failed to fill transparent pixel color")
	}
}

func TestDecodeTGA_UncompressedTrueColor(t *testing.T) {
	var buf bytes.Buffer
	header := make([]byte, 18)
	header[2] = 2 // uncompressed truecolor
	binary.LittleEndian.PutUint16(header[12:14], 2)
	binary.LittleEndian.PutUint16(header[14:16], 1)
	header[16] = 24
	header[17] = 0x20 // top-left origin
	buf.Write(header)
	// BGR pixels: red, green
	buf.Write([]byte{0, 0, 255, 0, 255, 0})

	img, err := DecodeTGA(buf.Bytes())
	testutil.AssertNoError(t, err)
	if img.Bounds().Dx() != 2 || img.Bounds().Dy() != 1 {
		t.Fatalf("unexpected bounds %v", img.Bounds())
	}
	r0 := img.RGBAAt(0, 0)
	r1 := img.RGBAAt(1, 0)
	if r0.R != 255 || r0.G != 0 || r0.B != 0 || r0.A != 255 {
		t.Fatalf("pixel 0 = %#v, want red", r0)
	}
	if r1.R != 0 || r1.G != 255 || r1.B != 0 || r1.A != 255 {
		t.Fatalf("pixel 1 = %#v, want green", r1)
	}
}

func TestDecodeTGA_RLETrueColor(t *testing.T) {
	var buf bytes.Buffer
	header := make([]byte, 18)
	header[2] = 10 // RLE truecolor
	binary.LittleEndian.PutUint16(header[12:14], 2)
	binary.LittleEndian.PutUint16(header[14:16], 1)
	header[16] = 24
	header[17] = 0x20 // top-left origin
	buf.Write(header)
	// One run-length packet with 2 pixels of the same blue color in BGR order.
	buf.Write([]byte{0x81, 255, 0, 0})

	img, err := DecodeTGA(buf.Bytes())
	testutil.AssertNoError(t, err)
	if img.Bounds().Dx() != 2 || img.Bounds().Dy() != 1 {
		t.Fatalf("unexpected bounds %v", img.Bounds())
	}
	for x := 0; x < 2; x++ {
		pixel := img.RGBAAt(x, 0)
		if pixel.R != 0 || pixel.G != 0 || pixel.B != 255 || pixel.A != 255 {
			t.Fatalf("pixel %d = %#v, want blue", x, pixel)
		}
	}
}
