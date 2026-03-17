package renderer

import (
	"testing"
)

// testPalette creates a 768-byte palette where each color index i maps to
// R=i, G=i, B=i for simplicity.
func testPalette() []byte {
	p := make([]byte, 768)
	for i := 0; i < 256; i++ {
		p[i*3+0] = byte(i) // R
		p[i*3+1] = byte(i) // G
		p[i*3+2] = byte(i) // B
	}
	return p
}

func TestConvertPaletteToFullbrightRGBA_NoFullbright(t *testing.T) {
	palette := testPalette()
	// All non-fullbright indices (0-223)
	pixels := []byte{0, 1, 100, 223}

	rgba, hasFB := ConvertPaletteToFullbrightRGBA(pixels, palette)
	if hasFB {
		t.Fatal("expected hasFB=false for non-fullbright pixels")
	}
	if len(rgba) != len(pixels)*4 {
		t.Fatalf("rgba length = %d, want %d", len(rgba), len(pixels)*4)
	}
	// All pixels should be fully transparent
	for i := 0; i < len(pixels); i++ {
		r, g, b, a := rgba[i*4+0], rgba[i*4+1], rgba[i*4+2], rgba[i*4+3]
		if r != 0 || g != 0 || b != 0 || a != 0 {
			t.Fatalf("pixel %d: got (%d,%d,%d,%d), want (0,0,0,0)", i, r, g, b, a)
		}
	}
}

func TestConvertPaletteToFullbrightRGBA_FullbrightPixels(t *testing.T) {
	palette := testPalette()
	// Mix of fullbright and non-fullbright pixels
	pixels := []byte{0, 224, 240, 254}

	rgba, hasFB := ConvertPaletteToFullbrightRGBA(pixels, palette)
	if !hasFB {
		t.Fatal("expected hasFB=true when fullbright pixels present")
	}

	// Pixel 0 (index 0): non-fullbright → transparent
	if rgba[0] != 0 || rgba[1] != 0 || rgba[2] != 0 || rgba[3] != 0 {
		t.Fatalf("pixel 0: got (%d,%d,%d,%d), want (0,0,0,0)", rgba[0], rgba[1], rgba[2], rgba[3])
	}

	// Pixel 1 (index 224): fullbright → opaque with palette color
	if rgba[4] != 224 || rgba[5] != 224 || rgba[6] != 224 || rgba[7] != 255 {
		t.Fatalf("pixel 1: got (%d,%d,%d,%d), want (224,224,224,255)", rgba[4], rgba[5], rgba[6], rgba[7])
	}

	// Pixel 2 (index 240): fullbright → opaque
	if rgba[8] != 240 || rgba[9] != 240 || rgba[10] != 240 || rgba[11] != 255 {
		t.Fatalf("pixel 2: got (%d,%d,%d,%d), want (240,240,240,255)", rgba[8], rgba[9], rgba[10], rgba[11])
	}

	// Pixel 3 (index 254): fullbright → opaque
	if rgba[12] != 254 || rgba[13] != 254 || rgba[14] != 254 || rgba[15] != 255 {
		t.Fatalf("pixel 3: got (%d,%d,%d,%d), want (254,254,254,255)", rgba[12], rgba[13], rgba[14], rgba[15])
	}
}

func TestConvertPaletteToFullbrightRGBA_TransparentIndex(t *testing.T) {
	palette := testPalette()
	// Index 255 is transparent, NOT fullbright
	pixels := []byte{255}

	rgba, hasFB := ConvertPaletteToFullbrightRGBA(pixels, palette)
	if hasFB {
		t.Fatal("expected hasFB=false for transparent pixel (index 255)")
	}
	if rgba[0] != 0 || rgba[1] != 0 || rgba[2] != 0 || rgba[3] != 0 {
		t.Fatalf("transparent pixel: got (%d,%d,%d,%d), want (0,0,0,0)", rgba[0], rgba[1], rgba[2], rgba[3])
	}
}

func TestConvertPaletteToFullbrightRGBA_BoundaryCheck(t *testing.T) {
	palette := testPalette()
	// Test exact boundaries: 223 (not fullbright), 224 (fullbright), 254 (fullbright), 255 (transparent)
	pixels := []byte{223, 224, 254, 255}

	rgba, hasFB := ConvertPaletteToFullbrightRGBA(pixels, palette)
	if !hasFB {
		t.Fatal("expected hasFB=true")
	}

	// Index 223: NOT fullbright
	if rgba[3] != 0 {
		t.Fatalf("index 223 alpha = %d, want 0 (not fullbright)", rgba[3])
	}

	// Index 224: first fullbright index
	if rgba[7] != 255 {
		t.Fatalf("index 224 alpha = %d, want 255 (fullbright)", rgba[7])
	}

	// Index 254: last fullbright index
	if rgba[11] != 255 {
		t.Fatalf("index 254 alpha = %d, want 255 (fullbright)", rgba[11])
	}

	// Index 255: transparent, not fullbright
	if rgba[15] != 0 {
		t.Fatalf("index 255 alpha = %d, want 0 (transparent)", rgba[15])
	}
}

func TestConvertPaletteToFullbrightRGBA_EmptyInput(t *testing.T) {
	palette := testPalette()
	pixels := []byte{}

	rgba, hasFB := ConvertPaletteToFullbrightRGBA(pixels, palette)
	if hasFB {
		t.Fatal("expected hasFB=false for empty input")
	}
	if len(rgba) != 0 {
		t.Fatalf("rgba length = %d, want 0", len(rgba))
	}
}

func TestConvertPaletteToFullbrightRGBA_AllFullbright(t *testing.T) {
	palette := testPalette()
	// All 31 fullbright indices (224-254)
	pixels := make([]byte, 31)
	for i := 0; i < 31; i++ {
		pixels[i] = byte(224 + i)
	}

	rgba, hasFB := ConvertPaletteToFullbrightRGBA(pixels, palette)
	if !hasFB {
		t.Fatal("expected hasFB=true for all fullbright pixels")
	}

	for i := 0; i < 31; i++ {
		idx := 224 + i
		r, g, b, a := rgba[i*4+0], rgba[i*4+1], rgba[i*4+2], rgba[i*4+3]
		if r != byte(idx) || g != byte(idx) || b != byte(idx) || a != 255 {
			t.Fatalf("pixel %d (index %d): got (%d,%d,%d,%d), want (%d,%d,%d,255)",
				i, idx, r, g, b, a, idx, idx, idx)
		}
	}
}
