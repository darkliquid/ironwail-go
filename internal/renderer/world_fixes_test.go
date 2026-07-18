package renderer

import (
	"reflect"
	"testing"

	surfacepkg "github.com/darkliquid/ironwail-go/internal/renderer/surface"
)

// TestAnimateWorldMaterialsFrame1 verifies that frame=1 selects the alternate
// animation chain (used by pressed buttons and activated switches). QuakeC
// sets self.frame=1 when a button is pressed, which should switch textures
// from the primary chain (+0button, +1button) to the alternate chain
// (+Abutton, +Bbutton).
func TestAnimateWorldMaterialsFrame1(t *testing.T) {
	// Base materials: index 0 = primary texture, index 1 = alternate texture
	baseMaterials := []WorldMaterialData{
		{AtlasBounds: [4]float32{0.0, 0.0, 0.5, 0.5}, Layer: 0},
		{AtlasBounds: [4]float32{0.5, 0.5, 0.5, 0.5}, Layer: 1},
	}

	// Primary chain: anim0 -> anim0 (no time animation, just a base texture)
	anim0 := &surfacepkg.SurfaceTexture{
		TextureIndex: 0,
	}
	// Alternate chain: animAlt points to index 1
	animAlt := &surfacepkg.SurfaceTexture{
		TextureIndex: 1,
	}
	anim0.AlternateAnims = animAlt

	animations := []*surfacepkg.SurfaceTexture{anim0}

	// Frame 0: should use primary chain (index 0)
	animated0 := animateWorldMaterials(baseMaterials, animations, 0, 0.0)
	if !reflect.DeepEqual(animated0[0], baseMaterials[0]) {
		t.Errorf("frame=0: animated[0] = %v, want %v (primary)", animated0[0], baseMaterials[0])
	}

	// Frame 1: should use alternate chain (index 1)
	animated1 := animateWorldMaterials(baseMaterials, animations, 1, 0.0)
	if !reflect.DeepEqual(animated1[0], baseMaterials[1]) {
		t.Errorf("frame=1: animated[0] = %v, want %v (alternate)", animated1[0], baseMaterials[1])
	}
}
