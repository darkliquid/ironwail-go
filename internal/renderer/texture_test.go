package renderer

import (
	"testing"
)

func TestConvertPaletteToFullbrightRGBA(t *testing.T) {
	// Create a simple palette with 256 colors (768 bytes: R,G,B for each)
	palette := make([]byte, 768)
	for i := 0; i < 256; i++ {
		palette[i*3+0] = byte(i)       // R
		palette[i*3+1] = byte(255 - i) // G
		palette[i*3+2] = byte(i / 2)   // B
	}

	tests := []struct {
		name           string
		pixels         []byte
		wantFullbright bool
		checkPixels    map[int]struct{ r, g, b, a byte }
	}{
		{
			name:           "no fullbright pixels",
			pixels:         []byte{0, 1, 2, 100, 200, 223},
			wantFullbright: false,
			checkPixels: map[int]struct{ r, g, b, a byte }{
				0: {0, 0, 0, 0}, // All should be transparent
				1: {0, 0, 0, 0},
				5: {0, 0, 0, 0},
			},
		},
		{
			name:           "all fullbright pixels",
			pixels:         []byte{224, 225, 226, 250, 254},
			wantFullbright: true,
			checkPixels: map[int]struct{ r, g, b, a byte }{
				0: {224, 31, 112, 255}, // index 224
				1: {225, 30, 112, 255}, // index 225
				2: {226, 29, 113, 255}, // index 226
				3: {250, 5, 125, 255},  // index 250
				4: {254, 1, 127, 255},  // index 254
			},
		},
		{
			name:           "mixed fullbright and normal pixels",
			pixels:         []byte{0, 224, 100, 254, 223, 225},
			wantFullbright: true,
			checkPixels: map[int]struct{ r, g, b, a byte }{
				0: {0, 0, 0, 0},        // index 0 - transparent
				1: {224, 31, 112, 255}, // index 224 - fullbright
				2: {0, 0, 0, 0},        // index 100 - transparent
				3: {254, 1, 127, 255},  // index 254 - fullbright
				4: {0, 0, 0, 0},        // index 223 - transparent
				5: {225, 30, 112, 255}, // index 225 - fullbright
			},
		},
		{
			name:           "palette index 255 is transparent not fullbright",
			pixels:         []byte{255, 224, 255},
			wantFullbright: true,
			checkPixels: map[int]struct{ r, g, b, a byte }{
				0: {0, 0, 0, 0},        // index 255 - transparent (not fullbright)
				1: {224, 31, 112, 255}, // index 224 - fullbright
				2: {0, 0, 0, 0},        // index 255 - transparent (not fullbright)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rgba, hasFullbright := ConvertPaletteToFullbrightRGBA(tt.pixels, palette)

			if hasFullbright != tt.wantFullbright {
				t.Errorf("ConvertPaletteToFullbrightRGBA() hasFullbright = %v, want %v", hasFullbright, tt.wantFullbright)
			}

			if len(rgba) != len(tt.pixels)*4 {
				t.Errorf("ConvertPaletteToFullbrightRGBA() returned wrong length: got %d, want %d", len(rgba), len(tt.pixels)*4)
			}

			for pixelIdx, want := range tt.checkPixels {
				offset := pixelIdx * 4
				if offset+3 >= len(rgba) {
					t.Errorf("pixel index %d is out of bounds", pixelIdx)
					continue
				}
				got := struct{ r, g, b, a byte }{
					rgba[offset+0],
					rgba[offset+1],
					rgba[offset+2],
					rgba[offset+3],
				}
				if got != want {
					t.Errorf("pixel %d: got RGBA(%d,%d,%d,%d), want RGBA(%d,%d,%d,%d)",
						pixelIdx, got.r, got.g, got.b, got.a, want.r, want.g, want.b, want.a)
				}
			}
		})
	}
}

func TestConvertPaletteToFullbrightRGBA_InvalidPalette(t *testing.T) {
	// Test with invalid palette (too short)
	shortPalette := make([]byte, 100)
	pixels := []byte{224, 225, 226}

	rgba, hasFullbright := ConvertPaletteToFullbrightRGBA(pixels, shortPalette)

	// Even with invalid palette, it should not crash
	// GetPaletteColor will return grayscale values for invalid palettes
	if len(rgba) != len(pixels)*4 {
		t.Errorf("ConvertPaletteToFullbrightRGBA() with invalid palette returned wrong length")
	}

	// Should still detect fullbright pixels even if colors are wrong
	if !hasFullbright {
		t.Errorf("ConvertPaletteToFullbrightRGBA() with invalid palette should still detect fullbright pixels")
	}
}

func TestFullbrightRange(t *testing.T) {
	// Verify that indices 224-254 are fullbright, 255 is transparent
	palette := make([]byte, 768)
	for i := 0; i < 256; i++ {
		palette[i*3+0] = byte(i)
		palette[i*3+1] = byte(i)
		palette[i*3+2] = byte(i)
	}

	// Test boundary cases
	testCases := []struct {
		index          byte
		shouldBeOpaque bool
	}{
		{223, false}, // Just before fullbright range
		{224, true},  // Start of fullbright range
		{225, true},
		{250, true},
		{254, true},  // End of fullbright range
		{255, false}, // Transparent, not fullbright
	}

	for _, tc := range testCases {
		pixels := []byte{tc.index}
		rgba, hasFullbright := ConvertPaletteToFullbrightRGBA(pixels, palette)

		if tc.shouldBeOpaque {
			if !hasFullbright {
				t.Errorf("index %d should be detected as fullbright", tc.index)
			}
			if rgba[3] != 255 {
				t.Errorf("index %d should be opaque, got alpha=%d", tc.index, rgba[3])
			}
		} else {
			if rgba[3] != 0 {
				t.Errorf("index %d should be transparent, got alpha=%d", tc.index, rgba[3])
			}
		}
	}
}
