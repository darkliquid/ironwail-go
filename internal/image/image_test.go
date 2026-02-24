package image

import (
	"bytes"
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

	// Try to find a known lump, e.g., "conchars"
	lump, ok := wad.Lumps["conchars"]
	if !ok {
		t.Fatal("conchars lump not found in gfx.wad")
	}

	if lump.Type != TypQPic {
		t.Errorf("expected conchars to be TypQPic, got %d", lump.Type)
	}

	qpic, err := ParseQPic(lump.Data)
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