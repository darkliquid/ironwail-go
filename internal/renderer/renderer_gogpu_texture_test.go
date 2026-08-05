package renderer

import (
	"testing"
)

func TestRendererCharTexturesArrayZeroState(t *testing.T) {
	r := &Renderer{}
	if len(r.charTextures) != 256 {
		t.Fatalf("len(r.charTextures) = %d, want 256", len(r.charTextures))
	}
	for i, tex := range r.charTextures {
		if tex != nil {
			t.Errorf("charTextures[%d] should be nil initially", i)
		}
	}
}

func TestRendererSetPaletteClearsCharTextures(t *testing.T) {
	r := &Renderer{
		textureCache: make(map[cacheKey]*cachedTexture),
	}
	r.SetPalette(make([]byte, 768))

	for i, tex := range r.charTextures {
		if tex != nil {
			t.Errorf("charTextures[%d] should be cleared after SetPalette", i)
		}
	}
}
