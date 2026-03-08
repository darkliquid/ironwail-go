//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"math"
	"strings"
	"testing"

	"github.com/go-gl/gl/v4.6-core/gl"
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

func TestWorldTextureFilters(t *testing.T) {
	minFilter, magFilter := worldTextureFilters(false)
	if minFilter != gl.NEAREST || magFilter != gl.NEAREST {
		t.Fatalf("worldTextureFilters(false) = (%d, %d), want (%d, %d)", minFilter, magFilter, gl.NEAREST, gl.NEAREST)
	}

	minFilter, magFilter = worldTextureFilters(true)
	if minFilter != gl.LINEAR || magFilter != gl.LINEAR {
		t.Fatalf("worldTextureFilters(true) = (%d, %d), want (%d, %d)", minFilter, magFilter, gl.LINEAR, gl.LINEAR)
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
	want := [3]float32{0, 0, 1}
	if !almostEqualVec3(got, want, 1e-6) {
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

func TestExtractEmbeddedSkyLayers_Standard(t *testing.T) {
	palette := make([]byte, 768)
	for i := 0; i < 256; i++ {
		palette[i*3] = byte(i)
		palette[i*3+1] = byte(255 - i)
		palette[i*3+2] = byte(i / 2)
	}
	// 4x2 test sky (left half = front, right half = back)
	// Row0: front [0,2], back [3,4]
	// Row1: front [5,255], back [7,8]
	pixels := []byte{
		0, 2, 3, 4,
		5, 255, 7, 8,
	}
	solid, alpha, w, h, ok := extractEmbeddedSkyLayers(pixels, 4, 2, palette, false)
	if !ok {
		t.Fatalf("extractEmbeddedSkyLayers() failed")
	}
	if w != 2 || h != 2 {
		t.Fatalf("layer size = %dx%d, want 2x2", w, h)
	}
	if len(solid) != 16 || len(alpha) != 16 {
		t.Fatalf("unexpected RGBA sizes: solid=%d alpha=%d", len(solid), len(alpha))
	}
	if solid[0] != 3 || solid[1] != 252 || solid[2] != 1 || solid[3] != 255 {
		t.Fatalf("solid first pixel = %v, want [3 252 1 255]", solid[:4])
	}
	if alpha[3] != 0 {
		t.Fatalf("alpha first pixel alpha = %d, want 0 for front index 0", alpha[3])
	}
	if alpha[4] != 2 || alpha[5] != 253 || alpha[6] != 1 || alpha[7] != 255 {
		t.Fatalf("alpha second pixel = %v, want [2 253 1 255]", alpha[4:8])
	}
	if alpha[15] != 0 {
		t.Fatalf("alpha last pixel alpha = %d, want 0 for front index 255", alpha[15])
	}
}

func TestExtractEmbeddedSkyLayers_Quake64(t *testing.T) {
	palette := make([]byte, 768)
	for i := 0; i < 256; i++ {
		palette[i*3] = byte(i)
		palette[i*3+1] = byte(i + 1)
		palette[i*3+2] = byte(i + 2)
	}
	// 2x4 test sky (top half = front, bottom half = back)
	pixels := []byte{
		1, 2,
		3, 4,
		5, 6,
		7, 8,
	}
	solid, alpha, w, h, ok := extractEmbeddedSkyLayers(pixels, 2, 4, palette, true)
	if !ok {
		t.Fatalf("extractEmbeddedSkyLayers(quake64) failed")
	}
	if w != 2 || h != 2 {
		t.Fatalf("quake64 layer size = %dx%d, want 2x2", w, h)
	}
	if solid[0] != 5 || solid[1] != 6 || solid[2] != 7 || solid[3] != 255 {
		t.Fatalf("quake64 solid first pixel = %v, want [5 6 7 255]", solid[:4])
	}
	if alpha[0] != 1 || alpha[1] != 2 || alpha[2] != 3 || alpha[3] != 128 {
		t.Fatalf("quake64 alpha first pixel = %v, want [1 2 3 128]", alpha[:4])
	}
}

func TestWorldSkyTexturesForFaceUsesAnimatedFrame(t *testing.T) {
	animations, err := BuildTextureAnimations([]string{"+0sky", "+1sky"})
	if err != nil {
		t.Fatalf("BuildTextureAnimations error: %v", err)
	}
	face := WorldFace{TextureIndex: 0}
	solidTextures := map[int32]uint32{0: 100, 1: 101}
	alphaTextures := map[int32]uint32{0: 200, 1: 201}
	solid, alpha := worldSkyTexturesForFace(face, solidTextures, alphaTextures, animations, 900, 901, 0, 0.2)
	if solid != 101 || alpha != 201 {
		t.Fatalf("worldSkyTexturesForFace(animated) = (%d,%d), want (101,201)", solid, alpha)
	}
	solid, alpha = worldSkyTexturesForFace(WorldFace{TextureIndex: 5}, solidTextures, alphaTextures, animations, 900, 901, 0, 0)
	if solid != 900 || alpha != 901 {
		t.Fatalf("worldSkyTexturesForFace(fallback) = (%d,%d), want (900,901)", solid, alpha)
	}
}

func TestShouldSplitAsQuake64Sky(t *testing.T) {
	if !shouldSplitAsQuake64Sky(bsp.BSPVersion_Quake64, 256, 128) {
		t.Fatalf("expected quake64 BSP version to force quake64 split")
	}
	if !shouldSplitAsQuake64Sky(bsp.BSPVersion, 32, 64) {
		t.Fatalf("expected 32x64 textures to use quake64 split")
	}
	if shouldSplitAsQuake64Sky(bsp.BSPVersion, 256, 128) {
		t.Fatalf("unexpected quake64 split for standard sky dimensions")
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

func TestWorldFaceUsesTurb(t *testing.T) {
	if got := worldFaceUsesTurb(0); got {
		t.Fatalf("worldFaceUsesTurb(opaque) = %v, want false", got)
	}
	if got := worldFaceUsesTurb(model.SurfDrawTurb | model.SurfDrawWater); !got {
		t.Fatalf("worldFaceUsesTurb(turb) = %v, want true", got)
	}
	if got := worldFaceUsesTurb(model.SurfDrawTurb | model.SurfDrawSky); got {
		t.Fatalf("worldFaceUsesTurb(sky+turb) = %v, want false", got)
	}
}

func TestWorldFogUniformDensityMatchesIronwailScale(t *testing.T) {
	got := worldFogUniformDensity(1)
	want := float32((1.20112241 * 0.85 / 64.0) * (1.20112241 * 0.85 / 64.0))
	if !almostEqualFloat32(got, want, 1e-9) {
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

func TestParseWorldspawnSkyFogOverride(t *testing.T) {
	entities := []byte(`
{
"classname" "worldspawn"
"skyfog" "0.8"
}
`)
	override := parseWorldspawnSkyFogOverride(entities)
	if !override.hasValue || override.value != 0.8 {
		t.Fatalf("skyfog override = (%v, %v), want (true, 0.8)", override.hasValue, override.value)
	}
}

func TestParseWorldspawnSkyFogOverride_NotWorldspawn(t *testing.T) {
	entities := []byte(`{"classname" "info_player_start" "skyfog" "0.8"}`)
	override := parseWorldspawnSkyFogOverride(entities)
	if override.hasValue {
		t.Fatalf("expected no skyfog override for non-worldspawn, got %+v", override)
	}
}

func TestResolveWorldSkyFogMix(t *testing.T) {
	if got := resolveWorldSkyFogMix(0.5, worldSkyFogOverride{}, 0.2); got != 0.5 {
		t.Fatalf("resolveWorldSkyFogMix(default) = %v, want 0.5", got)
	}
	if got := resolveWorldSkyFogMix(2, worldSkyFogOverride{}, 0.2); got != 1 {
		t.Fatalf("resolveWorldSkyFogMix(clamp cvar) = %v, want 1", got)
	}
	override := worldSkyFogOverride{hasValue: true, value: 0.8}
	if got := resolveWorldSkyFogMix(0.1, override, 0.2); got != 0.8 {
		t.Fatalf("resolveWorldSkyFogMix(override) = %v, want 0.8", got)
	}
	if got := resolveWorldSkyFogMix(0.5, override, 0); got != 0 {
		t.Fatalf("resolveWorldSkyFogMix(no general fog) = %v, want 0", got)
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

	sky, opaque, alphaTest, liquidOpaque, liquidTranslucent, translucent := bucketWorldFaces(faces, textures, nil, lightmaps, fallbackTex, fallbackLM, [3]float32{}, camera, alphaSettings)

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
	if len(liquidOpaque) != 0 {
		t.Fatalf("expected 0 opaque-liquid faces, got %d", len(liquidOpaque))
	}
	if len(liquidTranslucent) != 0 {
		t.Fatalf("expected 0 translucent-liquid faces, got %d", len(liquidTranslucent))
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

	sky, opaque, alphaTest, liquidOpaque, liquidTranslucent, translucent := bucketWorldFaces(faces, textures, nil, lightmaps, fallbackTex, fallbackLM, [3]float32{}, camera, alphaSettings)

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
	if len(liquidOpaque) != 0 {
		t.Fatalf("expected 0 opaque-liquid faces, got %d", len(liquidOpaque))
	}
	if len(liquidTranslucent) != 0 {
		t.Fatalf("expected 0 translucent-liquid faces, got %d", len(liquidTranslucent))
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

	sky, _, _, _, _, _ := bucketWorldFaces(faces, textures, nil, lightmaps, fallbackTex, fallbackLM, [3]float32{}, camera, alphaSettings)

	if len(sky) != 0 {
		t.Fatalf("expected 0 sky faces, got %d", len(sky))
	}
}

func TestBucketWorldFaces_TurbulentCallFlag(t *testing.T) {
	faces := []WorldFace{
		{
			FirstIndex:    0,
			NumIndices:    6,
			TextureIndex:  0,
			LightmapIndex: 0,
			Flags:         model.SurfDrawTurb | model.SurfDrawWater,
			Center:        [3]float32{0, 0, 0},
		},
		{
			FirstIndex:    6,
			NumIndices:    6,
			TextureIndex:  0,
			LightmapIndex: 0,
			Flags:         0,
			Center:        [3]float32{0, 0, 16},
		},
	}
	textures := map[int32]uint32{0: 1}
	lightmaps := []uint32{0}
	fallbackTex := uint32(999)
	fallbackLM := uint32(998)
	camera := CameraState{Origin: gmath.Zero3(), Angles: gmath.Zero3()}
	alphaSettings := worldLiquidAlphaSettings{water: 0.5, lava: 1, slime: 1, tele: 1}

	_, opaque, _, _, liquidTranslucent, translucent := bucketWorldFaces(faces, textures, nil, lightmaps, fallbackTex, fallbackLM, [3]float32{}, camera, alphaSettings)
	if len(liquidTranslucent) != 1 || !liquidTranslucent[0].turbulent {
		t.Fatalf("expected one turbulent translucent liquid call, got %#v", liquidTranslucent)
	}
	if len(translucent) != 0 {
		t.Fatalf("expected 0 non-liquid translucent calls, got %#v", translucent)
	}
	if len(opaque) != 1 || opaque[0].turbulent {
		t.Fatalf("expected one non-turbulent opaque call, got %#v", opaque)
	}
}

func TestBucketWorldFacesWithLights_PropagatesDynamicLight(t *testing.T) {
	faces := []WorldFace{
		{
			FirstIndex:    0,
			NumIndices:    6,
			TextureIndex:  0,
			LightmapIndex: 0,
			Flags:         0,
			Center:        [3]float32{0, 0, 0},
		},
	}
	textures := map[int32]uint32{0: 1}
	lightmaps := []uint32{2}
	camera := CameraState{Origin: gmath.Zero3(), Angles: gmath.Zero3()}
	alphaSettings := worldLiquidAlphaSettings{water: 1, lava: 1, slime: 1, tele: 1}
	pool := NewGLLightPool(4)
	pool.SpawnLight(DynamicLight{
		Position:   [3]float32{0, 0, 0},
		Color:      [3]float32{0.25, 0.5, 0.75},
		Radius:     128,
		Brightness: 1,
		Lifetime:   1,
	})

	_, opaque, _, _, _, _ := bucketWorldFacesWithLights(faces, textures, nil, lightmaps, 999, 998, [3]float32{}, identityModelRotationMatrix, 1, 1, 0, 0, camera, alphaSettings, pool)
	if len(opaque) != 1 {
		t.Fatalf("opaque count = %d, want 1", len(opaque))
	}
	if !almostEqualVec3(opaque[0].light, [3]float32{0.25, 0.5, 0.75}, 1e-5) {
		t.Fatalf("opaque[0].light = %v, want [0.25 0.5 0.75]", opaque[0].light)
	}
}

func TestWorldFragmentShader_DiffuseAndDynamicLightParity(t *testing.T) {
	if strings.Contains(worldFragmentShaderGL, "texture(uLightmap, vLightmapCoord).rgb * 2.0") {
		t.Fatalf("world fragment shader still overbrightens lightmaps")
	}
	if !strings.Contains(worldFragmentShaderGL, "uniform vec3 uDynamicLight;") {
		t.Fatalf("world fragment shader missing uDynamicLight uniform")
	}
	if !strings.Contains(worldFragmentShaderGL, "vec3 light = texture(uLightmap, vLightmapCoord).rgb + uDynamicLight;") {
		t.Fatalf("world fragment shader missing dynamic light accumulation")
	}
}

func TestWorldFaceIsLiquid(t *testing.T) {
	if worldFaceIsLiquid(0) {
		t.Fatalf("expected non-liquid flags to return false")
	}
	if !worldFaceIsLiquid(model.SurfDrawWater) {
		t.Fatalf("expected water flags to return true")
	}
	if !worldFaceIsLiquid(model.SurfDrawLava) {
		t.Fatalf("expected lava flags to return true")
	}
	if !worldFaceIsLiquid(model.SurfDrawSlime) {
		t.Fatalf("expected slime flags to return true")
	}
	if !worldFaceIsLiquid(model.SurfDrawTele) {
		t.Fatalf("expected tele flags to return true")
	}
}

func TestBucketWorldFaces_LiquidBuckets(t *testing.T) {
	faces := []WorldFace{
		{FirstIndex: 0, NumIndices: 6, TextureIndex: 0, LightmapIndex: 0, Flags: model.SurfDrawTurb | model.SurfDrawWater, Center: [3]float32{0, 0, 32}},
		{FirstIndex: 6, NumIndices: 6, TextureIndex: 0, LightmapIndex: 0, Flags: model.SurfDrawTurb | model.SurfDrawLava, Center: [3]float32{0, 0, 96}},
		{FirstIndex: 12, NumIndices: 6, TextureIndex: 0, LightmapIndex: 0, Flags: model.SurfDrawTurb | model.SurfDrawSlime, Center: [3]float32{0, 0, 64}},
		{FirstIndex: 18, NumIndices: 6, TextureIndex: 0, LightmapIndex: 0, Flags: 0, Center: [3]float32{0, 0, 0}},
	}
	textures := map[int32]uint32{0: 1}
	lightmaps := []uint32{0}
	camera := CameraState{Origin: gmath.Zero3(), Angles: gmath.Zero3()}
	alphaSettings := worldLiquidAlphaSettings{water: 1, lava: 0.5, slime: 0.25, tele: 1}

	_, opaque, _, liquidOpaque, liquidTranslucent, translucent := bucketWorldFaces(faces, textures, nil, lightmaps, 999, 998, [3]float32{}, camera, alphaSettings)
	if len(opaque) != 1 {
		t.Fatalf("opaque count = %d, want 1", len(opaque))
	}
	if len(liquidOpaque) != 1 {
		t.Fatalf("opaque-liquid count = %d, want 1", len(liquidOpaque))
	}
	if len(liquidTranslucent) != 2 {
		t.Fatalf("translucent-liquid count = %d, want 2", len(liquidTranslucent))
	}
	if len(translucent) != 0 {
		t.Fatalf("non-liquid translucent count = %d, want 0", len(translucent))
	}
	if liquidTranslucent[0].distanceSq < liquidTranslucent[1].distanceSq {
		t.Fatalf("translucent-liquid draw order not sorted back-to-front: %#v", liquidTranslucent)
	}
}

func almostEqualFloat32(a, b, epsilon float32) bool {
	return float32(math.Abs(float64(a-b))) <= epsilon
}

func almostEqualVec3(a, b [3]float32, epsilon float32) bool {
	return almostEqualFloat32(a[0], b[0], epsilon) &&
		almostEqualFloat32(a[1], b[1], epsilon) &&
		almostEqualFloat32(a[2], b[2], epsilon)
}
