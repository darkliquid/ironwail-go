package renderer

import (
	"context"
	"log/slog"
	"reflect"
	"strings"
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

// TestDiagMaterialIDRangeDetectsOverflow verifies the Phase 2 diagnostic
// reliably distinguishes in-range materialIDs from those exceeding the
// material buffer capacity. The GPU buffers are sized to the material table
// (textureCount+2), so a face referencing a slot beyond capacity is a real
// out-of-bounds risk that must surface as a warning.
func TestDiagMaterialIDRangeDetectsOverflow(t *testing.T) {
	makeGeom := func(maxID uint32) *WorldGeometry {
		geom := &WorldGeometry{}
		for i := uint32(0); i < 10; i++ {
			id := i % (maxID + 1)
			geom.Faces = append(geom.Faces, WorldFace{
				TextureIndex: int32(id),
				NumIndices:   3,
			})
		}
		return geom
	}

	capture := &slogCapture{}
	old := slog.Default()
	slog.SetDefault(slog.New(capture))
	defer slog.SetDefault(old)

	// In-range: capacity 64, max materialID 9 — must not warn.
	capture.reset()
	diagMaterialIDRange(makeGeom(9), 64)
	if capture.hasSubstr("MATERIALID EXCEEDS") {
		t.Fatalf("in-range geometry wrongly warned: %v", capture.messages())
	}

	// Over-capacity: capacity 8, max materialID 9 — must warn.
	capture.reset()
	diagMaterialIDRange(makeGeom(9), 8)
	if !capture.hasSubstr("MATERIALID EXCEEDS") {
		t.Fatalf("over-capacity geometry did not warn: %v", capture.messages())
	}
}

// TestDiagMaterialIDFaceAuditUsesCapacity verifies the per-face audit flags
// faces against the actual material count, not a fixed 256-slot constant.
func TestDiagMaterialIDFaceAuditUsesCapacity(t *testing.T) {
	t.Setenv("IRONWAIL_DEBUG_MATERIAL_AUDIT", "1")

	geom := &WorldGeometry{}
	geom.Faces = append(geom.Faces, WorldFace{TextureIndex: 5, NumIndices: 3})

	capture := &slogCapture{}
	old := slog.Default()
	slog.SetDefault(slog.New(capture))
	defer slog.SetDefault(old)

	diagMaterialIDFaceAudit(geom, 50, 8)
	if !capture.hasSubstr("Per-face materialID audit") {
		t.Fatalf("env-gated audit did not run: %v", capture.messages())
	}
}

// slogCapture is a minimal slog.Handler that records messages for assertions.
type slogCapture struct {
	msgs []string
}

func (c *slogCapture) Enabled(context.Context, slog.Level) bool { return true }
func (c *slogCapture) Handle(_ context.Context, r slog.Record) error {
	c.msgs = append(c.msgs, r.Message)
	return nil
}
func (c *slogCapture) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *slogCapture) WithGroup(string) slog.Handler      { return c }
func (c *slogCapture) reset()                             { c.msgs = nil }
func (c *slogCapture) hasSubstr(s string) bool {
	for _, m := range c.msgs {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}
func (c *slogCapture) messages() []string { return c.msgs }

// TestFaceMaterialIDSentinelClampedToZero guards the materialID diagnostic
// against misreading the -1 missing-texture sentinel as a huge unsigned ID
// (which would flood MATERIALID EXCEEDS warnings on any map with dummy slots).
func TestFaceMaterialIDSentinelClampedToZero(t *testing.T) {
	if got := faceMaterialID(WorldFace{TextureIndex: -1}); got != 0 {
		t.Fatalf("faceMaterialID(-1) = %d, want 0", got)
	}
	if got := faceMaterialID(WorldFace{TextureIndex: 7}); got != 7 {
		t.Fatalf("faceMaterialID(7) = %d, want 7", got)
	}
}
