package renderer

import (
	"reflect"
	"testing"

	surfacepkg "github.com/darkliquid/ironwail-go/internal/renderer/surface"
)

func TestAnimateWorldMaterials_SwapsLayerAndBounds(t *testing.T) {
	// 1. Setup base materials with different layers and atlas bounds.
	// Frame 0 has Layer 0, Bounds [0.1, 0.1, 0.2, 0.2]
	// Frame 1 has Layer 1, Bounds [0.5, 0.5, 0.2, 0.2]
	baseMaterials := []WorldMaterialData{
		{
			AtlasBounds: [4]float32{0.1, 0.1, 0.2, 0.2},
			Layer:       0,
		},
		{
			AtlasBounds: [4]float32{0.5, 0.5, 0.2, 0.2},
			Layer:       1,
		},
	}

	// 2. Setup the texture animation chain matching Quake's BuildTextureAnimations logic.
	// For a 2-frame animation, AnimTotal is 4, and each frame has a window of size 2.
	anim1 := &surfacepkg.SurfaceTexture{
		TextureIndex: 1,
		AnimTotal:    4,
		AnimMin:      2,
		AnimMax:      4,
	}
	anim0 := &surfacepkg.SurfaceTexture{
		TextureIndex: 0,
		AnimTotal:    4,
		AnimMin:      0,
		AnimMax:      2,
		AnimNext:     anim1,
	}
	anim1.AnimNext = anim0

	animations := []*surfacepkg.SurfaceTexture{
		anim0,
		nil, // frame 1 doesn't initiate its own animation chain typically
	}

	// 3. We animate at a timeValue that resolves to frame 1.
	// timeValue=0.3 -> relative = int(0.3 * 10) % 4 = 3, which lands on anim1's window [2, 4).
	animated := animateWorldMaterials(baseMaterials, animations, 0, 0.3)

	if len(animated) != len(baseMaterials) {
		t.Fatalf("Expected %d animated materials, got %d", len(baseMaterials), len(animated))
	}

	// Verify that the animated material at index 0 got BOTH the Layer and AtlasBounds of anim1.
	wantMaterial := WorldMaterialData{
		AtlasBounds: [4]float32{0.5, 0.5, 0.2, 0.2},
		Layer:       1,
	}

	if animated[0].Layer != wantMaterial.Layer {
		t.Errorf("animated[0].Layer = %f, want %f", animated[0].Layer, wantMaterial.Layer)
	}

	if !reflect.DeepEqual(animated[0].AtlasBounds, wantMaterial.AtlasBounds) {
		t.Errorf("animated[0].AtlasBounds = %v, want %v", animated[0].AtlasBounds, wantMaterial.AtlasBounds)
	}
}
