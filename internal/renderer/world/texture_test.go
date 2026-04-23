package world

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/model"
)

// fakePalette builds a 256*3 palette where each index maps to a
// deterministic (r,g,b) triple so we can assert pixel outputs.
func fakePalette() []byte {
	p := make([]byte, 256*3)
	for i := 0; i < 256; i++ {
		p[i*3+0] = byte(i)
		p[i*3+1] = byte(i)
		p[i*3+2] = byte(i)
	}
	return p
}

func TestBuildMaterialTextureRGBA_LiquidKeepsFullbrightAlpha(t *testing.T) {
	// Lava textures are almost entirely composed of fullbright texels
	// (palette indices 224..254). The turbulent shader samples sampled.a
	// and multiplies it by uniforms.alpha to produce the final alpha; if
	// we treat lava like a regular lit texture (alpha-mask for fullbrights)
	// the output becomes invisible, matching the bug we're guarding against.
	palette := fakePalette()
	pixels := []byte{224, 240, 254}

	got := BuildMaterialTextureRGBA(pixels, palette, model.TexTypeLava)
	if got.HasFullbright {
		t.Fatalf("liquid textures must not produce a separate fullbright overlay")
	}
	if got.FullbrightRGBA != nil {
		t.Fatalf("liquid fullbright overlay should be nil, got %d bytes", len(got.FullbrightRGBA))
	}
	for i, idx := range pixels {
		base := i * 4
		if got.DiffuseRGBA[base+0] != idx || got.DiffuseRGBA[base+3] != 255 {
			t.Fatalf("liquid pixel %d: want (%d,%d,%d,255), got (%d,%d,%d,%d)",
				i, idx, idx, idx,
				got.DiffuseRGBA[base+0], got.DiffuseRGBA[base+1],
				got.DiffuseRGBA[base+2], got.DiffuseRGBA[base+3])
		}
	}
}

func TestBuildMaterialTextureRGBA_RegularTextureKeepsAlphaLightingMask(t *testing.T) {
	// Regular lit world materials still rely on alpha=0 as a lighting mask
	// for embedded fullbright texels.
	palette := fakePalette()
	pixels := []byte{10, 240}

	got := BuildMaterialTextureRGBA(pixels, palette, model.TexTypeDefault)
	if got.DiffuseRGBA[0*4+3] != 255 {
		t.Fatalf("non-fullbright texel should have alpha 255, got %d", got.DiffuseRGBA[3])
	}
	if got.DiffuseRGBA[1*4+3] != 0 {
		t.Fatalf("fullbright texel should have alpha-mask=0 on default textures, got %d", got.DiffuseRGBA[1*4+3])
	}
}

func TestBuildMaterialTextureRGBA_CutoutSplitsFullbright(t *testing.T) {
	palette := fakePalette()
	pixels := []byte{255, 240}

	got := BuildMaterialTextureRGBA(pixels, palette, model.TexTypeCutout)
	if got.DiffuseRGBA[0*4+3] != 0 {
		t.Fatalf("cutout index 255 should be transparent, got alpha %d", got.DiffuseRGBA[3])
	}
	if !got.HasFullbright || got.FullbrightRGBA == nil {
		t.Fatalf("cutout with fullbright texels must produce a separate overlay")
	}
	if got.FullbrightRGBA[1*4+3] != 255 {
		t.Fatalf("cutout fullbright overlay pixel should be opaque")
	}
}
