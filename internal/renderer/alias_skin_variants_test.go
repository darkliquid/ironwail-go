package renderer

import "testing"

func TestAliasSkinVariantRGBATranslatesPlayerColorsBeforeFullbrightMask(t *testing.T) {
	palette := make([]byte, 256*3)
	for i := 0; i < 256; i++ {
		palette[i*3+0] = byte(i)
		palette[i*3+1] = byte(i)
		palette[i*3+2] = byte(i)
	}

	base, fullbright := aliasSkinVariantRGBA([]byte{16, 96, 224, 5}, palette, 0x49, true)
	if len(base) != 16 || len(fullbright) != 16 {
		t.Fatalf("rgba lens = (%d,%d), want 16", len(base), len(fullbright))
	}
	if base[0] != 64 || base[4] != 159 {
		t.Fatalf("translated base = [%d %d], want [64 159]", base[0], base[4])
	}
	if fullbright[8] != 224 || fullbright[9] != 224 || fullbright[10] != 224 || fullbright[11] != 255 {
		t.Fatalf("fullbright pixel = %v, want opaque 224 mask", fullbright[8:12])
	}
	if fullbright[0] != 0 || fullbright[4] != 0 || fullbright[15] != 0 {
		t.Fatalf("non-fullbright pixels should stay transparent, got %v", fullbright)
	}
}
