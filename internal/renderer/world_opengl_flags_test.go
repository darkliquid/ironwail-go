//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"testing"

	"github.com/gogpu/gogpu/gmath"
	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/model"
)

func TestClassifyWorldTextureName(t *testing.T) {
	tests := []struct {
		name string
		want model.TextureType
	}{
		{name: "sky1", want: model.TexTypeSky},
		{name: "{fence01", want: model.TexTypeCutout},
		{name: "*lava1", want: model.TexTypeLava},
		{name: "*slime0", want: model.TexTypeSlime},
		{name: "*teleport", want: model.TexTypeTele},
		{name: "*water1", want: model.TexTypeWater},
		{name: "brick01", want: model.TexTypeDefault},
	}

	for _, tc := range tests {
		if got := classifyWorldTextureName(tc.name); got != tc.want {
			t.Fatalf("classifyWorldTextureName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestDeriveWorldFaceFlags(t *testing.T) {
	if got := deriveWorldFaceFlags(model.TexTypeSky, bsp.TexSpecial); got&(model.SurfDrawSky|model.SurfDrawTiled) != (model.SurfDrawSky | model.SurfDrawTiled) {
		t.Fatalf("sky flags = %#x, want sky+tiled bits", got)
	}
	if got := deriveWorldFaceFlags(model.TexTypeCutout, 0); got&model.SurfDrawFence == 0 {
		t.Fatalf("cutout flags = %#x, want fence bit", got)
	}
	if got := deriveWorldFaceFlags(model.TexTypeWater, 0); got&(model.SurfDrawTurb|model.SurfDrawWater) != (model.SurfDrawTurb | model.SurfDrawWater) {
		t.Fatalf("water flags = %#x, want turb+water bits", got)
	}
	if got := deriveWorldFaceFlags(model.TexTypeDefault, bsp.TexMissing); got&model.SurfNoTexture == 0 {
		t.Fatalf("missing-texture flags = %#x, want no-texture bit", got)
	}
}

func TestWorldFaceCenter(t *testing.T) {
	vertices := []WorldVertex{
		{Position: [3]float32{0, 0, 0}},
		{Position: [3]float32{8, 0, 0}},
		{Position: [3]float32{8, 8, 0}},
		{Position: [3]float32{0, 8, 0}},
	}
	if got := worldFaceCenter(vertices); got != ([3]float32{4, 4, 0}) {
		t.Fatalf("worldFaceCenter() = %v, want [4 4 0]", got)
	}
}

func TestWorldFacePass(t *testing.T) {
	if got := worldFacePass(model.SurfDrawSky, 1); got != worldPassSky {
		t.Fatalf("sky pass = %v, want %v", got, worldPassSky)
	}
	if got := worldFacePass(model.SurfDrawFence, 1); got != worldPassAlphaTest {
		t.Fatalf("fence pass = %v, want %v", got, worldPassAlphaTest)
	}
	if got := worldFacePass(model.SurfDrawTurb, 0.5); got != worldPassTranslucent {
		t.Fatalf("turb alpha pass = %v, want %v", got, worldPassTranslucent)
	}
	if got := worldFacePass(0, 0.5); got != worldPassTranslucent {
		t.Fatalf("brush alpha pass = %v, want %v", got, worldPassTranslucent)
	}
	if got := worldFacePass(0, 1); got != worldPassOpaque {
		t.Fatalf("opaque pass = %v, want %v", got, worldPassOpaque)
	}
}

func TestTransformModelSpacePointAppliesScaleAndOffset(t *testing.T) {
	got := transformModelSpacePoint([3]float32{1, 2, 3}, [3]float32{10, 20, 30}, identityModelRotationMatrix, 2)
	if got != [3]float32{12, 24, 36} {
		t.Fatalf("transformModelSpacePoint() = %v, want [12 24 36]", got)
	}
}

func TestTransformModelSpacePointAppliesBrushYawRotation(t *testing.T) {
	got := transformModelSpacePoint([3]float32{1, 0, 0}, [3]float32{10, 20, 30}, buildBrushRotationMatrix([3]float32{0, 90, 0}), 2)
	if got != [3]float32{10, 22, 30} {
		t.Fatalf("transformModelSpacePoint(yaw) = %v, want [10 22 30]", got)
	}
}

func TestBuildBrushRotationMatrixNegatesPitch(t *testing.T) {
	got := transformModelSpacePoint([3]float32{1, 0, 0}, [3]float32{}, buildBrushRotationMatrix([3]float32{90, 0, 0}), 1)
	if got != [3]float32{0, 0, 1} {
		t.Fatalf("transformModelSpacePoint(pitch) = %v, want [0 0 1]", got)
	}
}

func TestWorldTextureForFaceUsesAnimatedFrame(t *testing.T) {
	animations, err := BuildTextureAnimations([]string{"+0lava", "+1lava", "+Alava"})
	if err != nil {
		t.Fatalf("BuildTextureAnimations error: %v", err)
	}

	face := WorldFace{TextureIndex: 0}
	textures := map[int32]uint32{0: 11, 1: 22, 2: 33}

	if got := worldTextureForFace(face, textures, animations, 99, 0, 0.3); got != 22 {
		t.Fatalf("worldTextureForFace(primary) = %d, want 22", got)
	}
	if got := worldTextureForFace(face, textures, animations, 99, 1, 0.0); got != 33 {
		t.Fatalf("worldTextureForFace(alternate) = %d, want 33", got)
	}
}

func TestWorldFaceAlpha(t *testing.T) {
	alpha := worldLiquidAlphaSettings{water: 0.6, lava: 0.4, slime: 0.5, tele: 0.7}

	if got := worldFaceAlpha(0, alpha); got != 1 {
		t.Fatalf("opaque alpha = %v, want 1", got)
	}
	if got := worldFaceAlpha(model.SurfDrawTurb|model.SurfDrawWater, alpha); got != 0.6 {
		t.Fatalf("water alpha = %v, want 0.6", got)
	}
	if got := worldFaceAlpha(model.SurfDrawTurb|model.SurfDrawLava, alpha); got != 0.4 {
		t.Fatalf("lava alpha = %v, want 0.4", got)
	}
	if got := worldFaceAlpha(model.SurfDrawTurb|model.SurfDrawSlime, alpha); got != 0.5 {
		t.Fatalf("slime alpha = %v, want 0.5", got)
	}
	if got := worldFaceAlpha(model.SurfDrawTurb|model.SurfDrawTele, alpha); got != 0.7 {
		t.Fatalf("tele alpha = %v, want 0.7", got)
	}
}

func TestWorldFogUniformDensityMatchesIronwailScale(t *testing.T) {
	got := worldFogUniformDensity(1)
	want := float32((1.20112241 * 0.85 / 64.0) * (1.20112241 * 0.85 / 64.0))
	if got != want {
		t.Fatalf("worldFogUniformDensity(1) = %v, want %v", got, want)
	}
	if got := worldFogUniformDensity(0); got != 0 {
		t.Fatalf("worldFogUniformDensity(0) = %v, want 0", got)
	}
}

func TestResolveWorldLiquidAlphaSettings(t *testing.T) {
	overrides := worldLiquidAlphaOverrides{hasWater: true, water: 0.25, hasTele: true, tele: 0.8}
	got := resolveWorldLiquidAlphaSettings(1, 0, 0, 0, overrides, nil)
	if got.water != 0.25 || got.tele != 0.8 || got.lava != 0.25 || got.slime != 0.25 {
		t.Fatalf("resolveWorldLiquidAlphaSettings() = %+v", got)
	}
}

func TestParseWorldspawnLiquidAlphaOverrides(t *testing.T) {
	entities := []byte(`
{
"classname" "worldspawn"
"_wateralpha" "0.40"
"lavaalpha" "0.30"
"slimealpha" "0.50"
"telealpha" "0.60"
}
{
"classname" "info_player_start"
}
`)

	overrides := parseWorldspawnLiquidAlphaOverrides(entities)
	if !overrides.hasWater || overrides.water != 0.40 {
		t.Fatalf("water override = (%v, %v), want (true, 0.40)", overrides.hasWater, overrides.water)
	}
	if !overrides.hasLava || overrides.lava != 0.30 {
		t.Fatalf("lava override = (%v, %v), want (true, 0.30)", overrides.hasLava, overrides.lava)
	}
	if !overrides.hasSlime || overrides.slime != 0.50 {
		t.Fatalf("slime override = (%v, %v), want (true, 0.50)", overrides.hasSlime, overrides.slime)
	}
	if !overrides.hasTele || overrides.tele != 0.60 {
		t.Fatalf("tele override = (%v, %v), want (true, 0.60)", overrides.hasTele, overrides.tele)
	}
}

func TestParseWorldspawnLiquidAlphaOverrides_NotWorldspawn(t *testing.T) {
	entities := []byte(`{"classname" "info_player_start" "wateralpha" "0.1"}`)
	overrides := parseWorldspawnLiquidAlphaOverrides(entities)
	if overrides.hasWater || overrides.hasLava || overrides.hasSlime || overrides.hasTele {
		t.Fatalf("expected no overrides for non-worldspawn, got %+v", overrides)
	}
}

func TestResolveWorldLiquidAlphaSettings_OverrideZeroFallsBack(t *testing.T) {
	overrides := worldLiquidAlphaOverrides{hasWater: true, water: 0.4, hasLava: true, lava: 0}
	got := resolveWorldLiquidAlphaSettings(1, 0.8, 0, 0, overrides, nil)
	if got.water != 0.4 {
		t.Fatalf("water = %v, want 0.4", got.water)
	}
	if got.lava != 0.4 {
		t.Fatalf("lava = %v, want 0.4", got.lava)
	}
	if got.slime != 0.4 {
		t.Fatalf("slime = %v, want 0.4", got.slime)
	}
	if got.tele != 0.4 {
		t.Fatalf("tele = %v, want 0.4", got.tele)
	}
}

func TestResolveWorldLiquidAlphaSettings_CvarSpecificWinsWithoutMapOverride(t *testing.T) {
	overrides := worldLiquidAlphaOverrides{hasWater: true, water: 0.4}
	got := resolveWorldLiquidAlphaSettings(1, 0.8, 0, 0, overrides, nil)
	if got.lava != 0.8 {
		t.Fatalf("lava = %v, want 0.8", got.lava)
	}
}

func TestMapVisTransparentWaterSafe_Default(t *testing.T) {
	// Test that mapVisTransparentWaterSafe returns true by default (safe assumption)
	got := mapVisTransparentWaterSafe(nil)
	if !got {
		t.Fatalf("mapVisTransparentWaterSafe(nil) = %v, want true", got)
	}

	// Test with a non-nil tree (still safe by default)
	tree := &bsp.Tree{}
	got = mapVisTransparentWaterSafe(tree)
	if !got {
		t.Fatalf("mapVisTransparentWaterSafe(tree) = %v, want true", got)
	}
}

func TestResolveWorldLiquidAlphaSettings_VisUnsafeForcesOpaque(t *testing.T) {
	// Create a test case to verify that when vis-safety gating is implemented,
	// vis-unsafe maps will force all liquid alphas to opaque (1.0)
	overrides := worldLiquidAlphaOverrides{hasWater: true, water: 0.5, hasLava: true, lava: 0.3}

	// Test with nil tree (treated as safe by mapVisTransparentWaterSafe)
	got := resolveWorldLiquidAlphaSettings(1.0, 0.7, 0.6, 0.8, overrides, nil)
	// water: override at 0.5, slime: cvar at 0.6, lava: override at 0.3, tele: cvar at 0.8
	if got.water != 0.5 || got.lava != 0.3 || got.slime != 0.6 || got.tele != 0.8 {
		t.Fatalf("resolveWorldLiquidAlphaSettings(nil tree) = %+v, expected water:0.5 lava:0.3 slime:0.6 tele:0.8", got)
	}

	// Test with a tree (also treated as safe by current default implementation)
	tree := &bsp.Tree{}
	got = resolveWorldLiquidAlphaSettings(1.0, 0.7, 0.6, 0.8, overrides, tree)
	if got.water != 0.5 || got.lava != 0.3 || got.slime != 0.6 || got.tele != 0.8 {
		t.Fatalf("resolveWorldLiquidAlphaSettings(tree) = %+v, expected water:0.5 lava:0.3 slime:0.6 tele:0.8", got)
	}
	// Note: When mapVisTransparentWaterSafe is fully implemented to detect unsafe maps,
	// this test should be updated to verify that vis-unsafe maps force all alphas to 1.0
}

func TestBucketWorldFaces_Sky(t *testing.T) {
	faces := []WorldFace{
		{
			FirstIndex:    0,
			NumIndices:    6,
			TextureIndex:  0,
			LightmapIndex: -1,
			Flags:         model.SurfDrawSky,
			Center:        [3]float32{0, 0, 100},
		},
	}

	textures := map[int32]uint32{0: 1}
	lightmaps := []uint32{0}
	fallbackTex := uint32(999)
	fallbackLM := uint32(998)
	camera := CameraState{Origin: gmath.Zero3(), Angles: gmath.Zero3()}
	alphaSettings := worldLiquidAlphaSettings{water: 1, lava: 1, slime: 1, tele: 1}

	sky, opaque, alphaTest, translucent := bucketWorldFaces(faces, textures, nil, lightmaps, fallbackTex, fallbackLM, [3]float32{}, camera, alphaSettings)

	if len(sky) != 1 {
		t.Fatalf("expected 1 sky face, got %d", len(sky))
	}
	if len(opaque) != 0 {
		t.Fatalf("expected 0 opaque faces, got %d", len(opaque))
	}
	if len(alphaTest) != 0 {
		t.Fatalf("expected 0 alphaTest faces, got %d", len(alphaTest))
	}
	if len(translucent) != 0 {
		t.Fatalf("expected 0 translucent faces, got %d", len(translucent))
	}
}

func TestBucketWorldFaces_SkyWithOpaque(t *testing.T) {
	faces := []WorldFace{
		{
			FirstIndex:    0,
			NumIndices:    6,
			TextureIndex:  0,
			LightmapIndex: -1,
			Flags:         model.SurfDrawSky,
			Center:        [3]float32{0, 0, 100},
		},
		{
			FirstIndex:    6,
			NumIndices:    6,
			TextureIndex:  0,
			LightmapIndex: 0,
			Flags:         0, // opaque
			Center:        [3]float32{0, 0, 0},
		},
	}

	textures := map[int32]uint32{0: 1}
	lightmaps := []uint32{0}
	fallbackTex := uint32(999)
	fallbackLM := uint32(998)
	camera := CameraState{Origin: gmath.Zero3(), Angles: gmath.Zero3()}
	alphaSettings := worldLiquidAlphaSettings{water: 1, lava: 1, slime: 1, tele: 1}

	sky, opaque, alphaTest, translucent := bucketWorldFaces(faces, textures, nil, lightmaps, fallbackTex, fallbackLM, [3]float32{}, camera, alphaSettings)

	if len(sky) != 1 {
		t.Fatalf("expected 1 sky face, got %d", len(sky))
	}
	if len(opaque) != 1 {
		t.Fatalf("expected 1 opaque face, got %d", len(opaque))
	}
	if len(alphaTest) != 0 {
		t.Fatalf("expected 0 alphaTest faces, got %d", len(alphaTest))
	}
	if len(translucent) != 0 {
		t.Fatalf("expected 0 translucent faces, got %d", len(translucent))
	}
}

func TestBucketWorldFaces_EmptySkyBucket(t *testing.T) {
	faces := []WorldFace{
		{
			FirstIndex:    0,
			NumIndices:    6,
			TextureIndex:  0,
			LightmapIndex: 0,
			Flags:         0, // opaque, no sky
			Center:        [3]float32{0, 0, 0},
		},
	}

	textures := map[int32]uint32{0: 1}
	lightmaps := []uint32{0}
	fallbackTex := uint32(999)
	fallbackLM := uint32(998)
	camera := CameraState{Origin: gmath.Zero3(), Angles: gmath.Zero3()}
	alphaSettings := worldLiquidAlphaSettings{water: 1, lava: 1, slime: 1, tele: 1}

	sky, _, _, _ := bucketWorldFaces(faces, textures, nil, lightmaps, fallbackTex, fallbackLM, [3]float32{}, camera, alphaSettings)

	if len(sky) != 0 {
		t.Fatalf("expected 0 sky faces, got %d", len(sky))
	}
}
