package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/image"
)

// writeTestImageFile writes a PNG fixture (or TGA when ext is .tga) with
// the given RGBA pixels and returns its path.
func writeTestImageFile(t *testing.T, dir, name string, rgba []byte, w, h int) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var werr error
	switch filepath.Ext(name) {
	case ".png":
		werr = image.WritePNG(f, rgba, w, h, 32, false)
	case ".tga":
		werr = image.WriteTGA(f, rgba, w, h, 32, false)
	default:
		t.Fatalf("bad fixture ext %s", name)
	}
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		t.Fatalf("write fixture: %v", werr)
	}
	return path
}

func grayPixels(w, h int, g byte) []byte {
	pix := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		pix[i*4+0] = g
		pix[i*4+1] = g
		pix[i*4+2] = g
		pix[i*4+3] = 255
	}
	return pix
}

func runWadForTest(t *testing.T, args ...string) (string, int) {
	t.Helper()
	var stdout, stderr strings.Builder
	code := runWad(args, &stdout, &stderr)
	t.Logf("qcmod wad %v stderr: %s", args, stderr.String())
	return stdout.String(), code
}

func loadWadFile(t *testing.T, path string) *image.Wad {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read wad: %v", err)
	}
	wad, err := image.LoadWad(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("LoadWad: %v", err)
	}
	return wad
}

func TestWadQPicFromPNG(t *testing.T) {
	dir := t.TempDir()
	png := writeTestImageFile(t, dir, "menu_pic.png", grayPixels(20, 10, 200), 20, 10)
	out := filepath.Join(dir, "gfx.wad")

	got, code := runWadForTest(t, "-o", out, "-type", "qpic", png)
	if code != 0 {
		t.Fatalf("wad exit = %d (%s)", code, got)
	}

	wad := loadWadFile(t, out)
	lump, ok := wad.Lumps["menu_pic"]
	if !ok {
		t.Fatal("wad missing lump menu_pic")
	}
	if lump.Type != image.TypQPic {
		t.Errorf("lump type = %v, want TypQPic", lump.Type)
	}
	pic, err := image.ParseQPic(lump.Data)
	if err != nil {
		t.Fatalf("ParseQPic: %v", err)
	}
	if pic.Width != 20 || pic.Height != 10 {
		t.Errorf("qpic = %dx%d, want 20x10", pic.Width, pic.Height)
	}
	// Default palette is the real Quake palette (not grayscale); gray 200
	// is nearest to index 13 (RGB 0xcb). This documents the quantisation
	// against the embedded palette.lmp.
	for i, idx := range pic.Pixels {
		if idx != 13 {
			t.Fatalf("pixel %d index = %d, want 13 (gray 200 in Quake palette)", i, idx)
		}
	}
}

func TestWadMipTexAutoAndForced(t *testing.T) {
	dir := t.TempDir()
	png := writeTestImageFile(t, dir, "wall_brick.png", grayPixels(64, 64, 128), 64, 64)

	// auto: 64x64 qualifies as a texture -> miptex.
	out := filepath.Join(dir, "textures.wad")
	got, code := runWadForTest(t, "-o", out, png)
	if code != 0 {
		t.Fatalf("wad exit = %d (%s)", code, got)
	}
	wad := loadWadFile(t, out)
	lump := wad.Lumps["wall_brick"]
	if lump.Type != image.TypMipTex {
		t.Fatalf("auto lump type = %v, want TypMipTex", lump.Type)
	}
	mt, err := image.ParseMipTex(lump.Data)
	if err != nil {
		t.Fatalf("ParseMipTex: %v", err)
	}
	for level := 0; level < 4; level++ {
		if _, lw, lh, err := mt.MipLevel(level); err != nil {
			t.Fatalf("MipLevel(%d): %v", level, err)
		} else if lw != 64>>uint(level) || lh != 64>>uint(level) {
			t.Errorf("mip %d = %dx%d", level, lw, lh)
		}
	}

	// Forcing miptex on a non-multiple-of-16 image must fail cleanly.
	smallPng := writeTestImageFile(t, dir, "small.png", grayPixels(20, 10, 128), 20, 10)
	if _, code := runWadForTest(t, "-o", out, "-type", "miptex", smallPng); code != 1 {
		t.Errorf("forced miptex on 20x10: exit = %d, want 1", code)
	}
}

func TestWadTGAInput(t *testing.T) {
	dir := t.TempDir()
	tga := writeTestImageFile(t, dir, "icon.tga", grayPixels(10, 10, 64), 10, 10)
	out := filepath.Join(dir, "gfx.wad")
	got, code := runWadForTest(t, "-o", out, tga)
	if code != 0 {
		t.Fatalf("wad exit = %d (%s)", code, got)
	}
	wad := loadWadFile(t, out)
	lump, ok := wad.Lumps["icon"]
	if !ok {
		t.Fatal("wad missing tga-derived lump icon")
	}
	// 10x10 is not a texture size -> auto picks qpic.
	if lump.Type != image.TypQPic {
		t.Errorf("lump type = %v, want TypQPic", lump.Type)
	}
}

func TestWadPaletteFlag(t *testing.T) {
	dir := t.TempDir()
	// Custom palette: index 0 black, index 1 white, rest black.
	pal := make([]byte, 768)
	pal[3], pal[4], pal[5] = 255, 255, 255
	palPath := filepath.Join(dir, "palette.lmp")
	if err := os.WriteFile(palPath, pal, 0o644); err != nil {
		t.Fatalf("write palette: %v", err)
	}

	png := writeTestImageFile(t, dir, "logo.png", grayPixels(8, 8, 255), 8, 8) // pure white
	out := filepath.Join(dir, "gfx.wad")
	got, code := runWadForTest(t, "-o", out, "-type", "qpic", "-palette", palPath, png)
	if code != 0 {
		t.Fatalf("wad exit = %d (%s)", code, got)
	}
	wad := loadWadFile(t, out)
	pic, err := image.ParseQPic(wad.Lumps["logo"].Data)
	if err != nil {
		t.Fatalf("ParseQPic: %v", err)
	}
	if pic.Pixels[0] != 1 {
		t.Errorf("white pixel indexed %d, want 1 (custom palette)", pic.Pixels[0])
	}
}

func TestWadErrors(t *testing.T) {
	dir := t.TempDir()
	// No images.
	if _, code := runWadForTest(t, "-o", filepath.Join(dir, "x.wad")); code != 2 {
		t.Errorf("no images: exit = %d, want 2", code)
	}
	// Unknown flag.
	png := writeTestImageFile(t, dir, "a.png", grayPixels(8, 8, 0), 8, 8)
	if _, code := runWadForTest(t, "-bogus", png); code != 2 {
		t.Errorf("unknown flag: exit = %d, want 2", code)
	}
	// Unsupported format.
	bmp := filepath.Join(dir, "bad.bmp")
	if err := os.WriteFile(bmp, []byte("BM...."), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, code := runWadForTest(t, "-o", filepath.Join(dir, "x.wad"), bmp); code != 1 {
		t.Errorf("unsupported format: exit = %d, want 1", code)
	}
	// Missing palette file.
	if _, code := runWadForTest(t, "-o", filepath.Join(dir, "x.wad"), "-palette", filepath.Join(dir, "nope.lmp"), png); code != 1 {
		t.Errorf("missing palette: exit = %d, want 1", code)
	}
	// Bad -type.
	if _, code := runWadForTest(t, "-o", filepath.Join(dir, "x.wad"), "-type", "gif", png); code != 2 {
		t.Errorf("bad type: exit = %d, want 2", code)
	}
}