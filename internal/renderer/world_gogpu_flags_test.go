//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"encoding/binary"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/model"
	"github.com/ironwail/ironwail-go/pkg/types"
)

func TestClassifyWorldTextureNameGoGPU(t *testing.T) {
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

func TestDeriveWorldFaceFlagsGoGPU(t *testing.T) {
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

func TestGoGPUWorldClearColorUsesStateColor(t *testing.T) {
	t.Setenv("IRONWAIL_DEBUG_WORLD_CLEAR_GREEN", "")

	got := gogpuWorldClearColor([4]float32{0.1, 0.2, 0.3, 0.4})
	want := gputypes.Color{R: 0.1, G: 0.2, B: 0.3, A: 0.4}
	if math.Abs(got.R-want.R) > 1e-6 || math.Abs(got.G-want.G) > 1e-6 || math.Abs(got.B-want.B) > 1e-6 || math.Abs(got.A-want.A) > 1e-6 {
		t.Fatalf("gogpuWorldClearColor() = %#v, want %#v", got, want)
	}
}

func TestGoGPUWorldClearColorDebugOverride(t *testing.T) {
	t.Setenv("IRONWAIL_DEBUG_WORLD_CLEAR_GREEN", "1")

	got := gogpuWorldClearColor([4]float32{0.1, 0.2, 0.3, 0.4})
	want := gputypes.Color{R: 0.0, G: 1.0, B: 0.0, A: 1.0}
	if got != want {
		t.Fatalf("gogpuWorldClearColor() with debug override = %#v, want %#v", got, want)
	}
}

func TestGoGPUSharedDepthStencilClearAttachmentForView(t *testing.T) {
	attachment := gogpuSharedDepthStencilClearAttachmentForView(hal.TextureView(&wgpuTextureViewStub{}))
	if attachment == nil {
		t.Fatal("gogpuSharedDepthStencilClearAttachmentForView() = nil")
	}
	if attachment.DepthLoadOp != gputypes.LoadOpClear {
		t.Fatalf("DepthLoadOp = %v, want %v", attachment.DepthLoadOp, gputypes.LoadOpClear)
	}
	if attachment.DepthStoreOp != gputypes.StoreOpStore {
		t.Fatalf("DepthStoreOp = %v, want %v", attachment.DepthStoreOp, gputypes.StoreOpStore)
	}
	if attachment.StencilLoadOp != gputypes.LoadOpClear {
		t.Fatalf("StencilLoadOp = %v, want %v", attachment.StencilLoadOp, gputypes.LoadOpClear)
	}
	if attachment.StencilStoreOp != gputypes.StoreOpStore {
		t.Fatalf("StencilStoreOp = %v, want %v", attachment.StencilStoreOp, gputypes.StoreOpStore)
	}
	if attachment.StencilReadOnly {
		t.Fatal("StencilReadOnly = true, want false")
	}
}

func TestWorldSceneUniformBytesEncodesFog(t *testing.T) {
	vp := types.Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
	cameraOrigin := [3]float32{1, 2, 3}
	fogColor := [3]float32{0.2, 0.4, 0.6}
	fogDensity := float32(0.75)
	timeValue := float32(12.5)
	alphaValue := float32(0.35)
	dynamicLight := [3]float32{0.7, 0.5, 0.3}
	litWater := float32(1)

	data := worldSceneUniformBytes(vp, cameraOrigin, fogColor, fogDensity, timeValue, alphaValue, dynamicLight, litWater)
	if len(data) != worldUniformBufferSize {
		t.Fatalf("len(worldSceneUniformBytes()) = %d, want %d", len(data), worldUniformBufferSize)
	}
	for i, want := range cameraOrigin {
		got := math.Float32frombits(binary.LittleEndian.Uint32(data[64+i*4 : 68+i*4]))
		if !almostEqualWorldFloat32(got, want, 1e-6) {
			t.Fatalf("cameraOrigin[%d] = %v, want %v", i, got, want)
		}
	}
	gotFogDensity := math.Float32frombits(binary.LittleEndian.Uint32(data[76:80]))
	wantFogDensity := worldFogUniformDensity(fogDensity)
	if !almostEqualWorldFloat32(gotFogDensity, wantFogDensity, 1e-6) {
		t.Fatalf("fog density = %v, want %v", gotFogDensity, wantFogDensity)
	}
	for i, want := range fogColor {
		got := math.Float32frombits(binary.LittleEndian.Uint32(data[80+i*4 : 84+i*4]))
		if !almostEqualWorldFloat32(got, want, 1e-6) {
			t.Fatalf("fogColor[%d] = %v, want %v", i, got, want)
		}
	}
	gotTime := math.Float32frombits(binary.LittleEndian.Uint32(data[92:96]))
	if !almostEqualWorldFloat32(gotTime, timeValue, 1e-6) {
		t.Fatalf("time = %v, want %v", gotTime, timeValue)
	}
	gotAlpha := math.Float32frombits(binary.LittleEndian.Uint32(data[96:100]))
	if !almostEqualWorldFloat32(gotAlpha, alphaValue, 1e-6) {
		t.Fatalf("alpha = %v, want %v", gotAlpha, alphaValue)
	}
	for i, want := range dynamicLight {
		got := math.Float32frombits(binary.LittleEndian.Uint32(data[112+i*4 : 116+i*4]))
		if !almostEqualWorldFloat32(got, want, 1e-6) {
			t.Fatalf("dynamicLight[%d] = %v, want %v", i, got, want)
		}
	}
	gotLitWater := math.Float32frombits(binary.LittleEndian.Uint32(data[124:128]))
	if !almostEqualWorldFloat32(gotLitWater, litWater, 1e-6) {
		t.Fatalf("litWater = %v, want %v", gotLitWater, litWater)
	}
}

func TestGoGPUWorldUniformInputsUseCameraTimeAndRawFogDensity(t *testing.T) {
	state := &RenderFrameState{
		FogDensity:    0.75,
		WaterWarpTime: 99,
	}
	camera := CameraState{}
	camera.Origin = types.Vec3{X: 1, Y: 2, Z: 3}
	camera.Time = 12.5

	cameraOrigin, fogDensity, timeValue := gogpuWorldUniformInputs(state, camera)
	if cameraOrigin != [3]float32{1, 2, 3} {
		t.Fatalf("cameraOrigin = %#v, want [3]float32{1, 2, 3}", cameraOrigin)
	}
	if !almostEqualWorldFloat32(fogDensity, state.FogDensity, 1e-6) {
		t.Fatalf("fogDensity = %v, want %v", fogDensity, state.FogDensity)
	}
	if !almostEqualWorldFloat32(timeValue, camera.Time, 1e-6) {
		t.Fatalf("timeValue = %v, want %v", timeValue, camera.Time)
	}
	if almostEqualWorldFloat32(timeValue, state.WaterWarpTime, 1e-6) {
		t.Fatalf("timeValue = %v unexpectedly matched WaterWarpTime %v", timeValue, state.WaterWarpTime)
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

func TestGoGPUWorldSkyFogDensityUsesWorldspawnOverride(t *testing.T) {
	entities := []byte(`{"classname" "worldspawn" "skyfog" "0.8"}`)
	if got := gogpuWorldSkyFogDensity(entities, 0.2); got != 0.8 {
		t.Fatalf("gogpuWorldSkyFogDensity(override) = %v, want 0.8", got)
	}
	if got := gogpuWorldSkyFogDensity(entities, 0); got != 0 {
		t.Fatalf("gogpuWorldSkyFogDensity(no general fog) = %v, want 0", got)
	}
}

func TestWorldShadersIncludeFogMix(t *testing.T) {
	vertexChecks := []string{
		"cameraOrigin",
		"fogDensity",
		"fogColor",
		"worldPos",
		"alpha: f32",
	}
	for _, check := range vertexChecks {
		if !strings.Contains(worldVertexShaderWGSL, check) {
			t.Fatalf("world vertex shader missing %q", check)
		}
	}

	fragmentChecks := []string{
		"worldSampler",
		"worldTexture",
		"worldLightmapSampler",
		"worldLightmap",
		"worldFullbrightSampler",
		"worldFullbrightTexture",
		"textureSample(worldTexture, worldSampler, input.texCoord)",
		"textureSample(worldLightmap, worldLightmapSampler, input.lightmapCoord)",
		"textureSample(worldFullbrightTexture, worldFullbrightSampler, input.texCoord)",
		"if (sampled.a < 0.5)",
		"discard;",
		"cameraOrigin",
		"fogDensity",
		"fogColor",
		"worldPos",
		"exp2(",
		"dynamicLight",
		"sampled.rgb * (lightmap + uniforms.dynamicLight) + fullbright.rgb*fullbright.a",
		"mix(uniforms.fogColor, lit, fog)",
		"sampled.a * uniforms.alpha",
	}
	for _, check := range fragmentChecks {
		if !strings.Contains(worldFragmentShaderWGSL, check) {
			t.Fatalf("world fragment shader missing %q", check)
		}
	}

	skyChecks := []string{
		"skySolidTexture",
		"skyAlphaTexture",
		"uniforms.time / 16.0",
		"uniforms.time / 8.0",
		"189.0 / 64.0",
		"mix(result.rgb, layer.rgb, layer.a)",
		"mix(result.rgb, uniforms.fogColor, uniforms.fogDensity)",
	}
	for _, check := range skyChecks {
		if !strings.Contains(worldSkyFragmentShaderWGSL, check) {
			t.Fatalf("world sky fragment shader missing %q", check)
		}
	}

	externalSkyChecks := []string{
		"sampleExternalSky",
		"var skyRT: texture_2d<f32>;",
		"var skyBK: texture_2d<f32>;",
		"var skyLF: texture_2d<f32>;",
		"var skyFT: texture_2d<f32>;",
		"var skyUP: texture_2d<f32>;",
		"var skyDN: texture_2d<f32>;",
		"dir.x > 0.0",
		"dir.y > 0.0",
		"dir.z > 0.0",
		"mix(result.rgb, uniforms.fogColor, uniforms.fogDensity)",
	}
	for _, check := range externalSkyChecks {
		if !strings.Contains(worldSkyExternalFaceFragmentShaderWGSL, check) {
			t.Fatalf("world external sky fragment shader missing %q", check)
		}
	}

	turbChecks := []string{
		"input.texCoord * 2.0 + 0.125 * sin",
		"uniforms.time",
		"worldLightmapSampler",
		"worldLightmap",
		"worldFullbrightTexture",
		"uniforms.litWater > 0.5",
		"vec3<f32>(0.5)",
		"textureSample(worldLightmap, worldLightmapSampler, input.lightmapCoord).rgb",
		"sampled.rgb * (lightmap + uniforms.dynamicLight) + fullbright.rgb*fullbright.a",
		"sampled.a * uniforms.alpha",
	}
	for _, check := range turbChecks {
		if !strings.Contains(worldTurbulentFragmentShaderWGSL, check) {
			t.Fatalf("world turbulent fragment shader missing %q", check)
		}
	}
}

func TestShouldDrawGoGPUOpaqueWorldFace(t *testing.T) {
	tests := []struct {
		name string
		face WorldFace
		want bool
	}{
		{name: "opaque", face: WorldFace{NumIndices: 3}, want: true},
		{name: "empty", face: WorldFace{}, want: false},
		{name: "sky", face: WorldFace{NumIndices: 3, Flags: model.SurfDrawSky}, want: false},
		{name: "turbulent", face: WorldFace{NumIndices: 3, Flags: model.SurfDrawTurb}, want: false},
		{name: "fence", face: WorldFace{NumIndices: 3, Flags: model.SurfDrawFence}, want: true},
	}
	for _, tc := range tests {
		if got := shouldDrawGoGPUOpaqueWorldFace(tc.face); got != tc.want {
			t.Fatalf("%s: shouldDrawGoGPUOpaqueWorldFace(%#v) = %v, want %v", tc.name, tc.face, got, tc.want)
		}
	}
}

func TestSortGoGPUTranslucentLiquidFacesHonorsAlphaMode(t *testing.T) {
	faces := []gogpuTranslucentLiquidFaceDraw{
		{distanceSq: 1, face: WorldFace{TextureIndex: 1}},
		{distanceSq: 9, face: WorldFace{TextureIndex: 2}},
		{distanceSq: 4, face: WorldFace{TextureIndex: 3}},
	}
	sortGoGPUTranslucentLiquidFaces(AlphaModeSorted, faces)
	if got := []int32{faces[0].face.TextureIndex, faces[1].face.TextureIndex, faces[2].face.TextureIndex}; !reflect.DeepEqual(got, []int32{2, 3, 1}) {
		t.Fatalf("sorted liquid face order = %v, want [2 3 1]", got)
	}

	faces = []gogpuTranslucentLiquidFaceDraw{
		{distanceSq: 1, face: WorldFace{TextureIndex: 1}},
		{distanceSq: 9, face: WorldFace{TextureIndex: 2}},
		{distanceSq: 4, face: WorldFace{TextureIndex: 3}},
	}
	sortGoGPUTranslucentLiquidFaces(AlphaModeOIT, faces)
	if got := []int32{faces[0].face.TextureIndex, faces[1].face.TextureIndex, faces[2].face.TextureIndex}; !reflect.DeepEqual(got, []int32{1, 2, 3}) {
		t.Fatalf("oit liquid face order = %v, want [1 2 3]", got)
	}
}

func TestSortGoGPUTranslucentBrushFaceRendersHonorsAlphaMode(t *testing.T) {
	renders := []gogpuTranslucentBrushFaceRender{
		{face: gogpuTranslucentLiquidFaceDraw{distanceSq: 1, face: WorldFace{TextureIndex: 1}}},
		{face: gogpuTranslucentLiquidFaceDraw{distanceSq: 9, face: WorldFace{TextureIndex: 2}}},
		{face: gogpuTranslucentLiquidFaceDraw{distanceSq: 4, face: WorldFace{TextureIndex: 3}}},
	}
	sortGoGPUTranslucentBrushFaceRenders(AlphaModeSorted, renders)
	if got := []int32{renders[0].face.face.TextureIndex, renders[1].face.face.TextureIndex, renders[2].face.face.TextureIndex}; !reflect.DeepEqual(got, []int32{2, 3, 1}) {
		t.Fatalf("sorted brush face order = %v, want [2 3 1]", got)
	}

	renders = []gogpuTranslucentBrushFaceRender{
		{face: gogpuTranslucentLiquidFaceDraw{distanceSq: 1, face: WorldFace{TextureIndex: 1}}},
		{face: gogpuTranslucentLiquidFaceDraw{distanceSq: 9, face: WorldFace{TextureIndex: 2}}},
		{face: gogpuTranslucentLiquidFaceDraw{distanceSq: 4, face: WorldFace{TextureIndex: 3}}},
	}
	sortGoGPUTranslucentBrushFaceRenders(AlphaModeOIT, renders)
	if got := []int32{renders[0].face.face.TextureIndex, renders[1].face.face.TextureIndex, renders[2].face.face.TextureIndex}; !reflect.DeepEqual(got, []int32{1, 2, 3}) {
		t.Fatalf("oit brush face order = %v, want [1 2 3]", got)
	}
}

func TestSortGoGPUTranslucentBrushFaceRendersCrossSortsWorldAndBrushFaces(t *testing.T) {
	renders := []gogpuTranslucentBrushFaceRender{
		{liquid: true, face: gogpuTranslucentLiquidFaceDraw{distanceSq: 4, face: WorldFace{TextureIndex: 1}}},
		{liquid: false, face: gogpuTranslucentLiquidFaceDraw{distanceSq: 9, face: WorldFace{TextureIndex: 2}}},
		{liquid: true, face: gogpuTranslucentLiquidFaceDraw{distanceSq: 1, face: WorldFace{TextureIndex: 3}}},
	}
	sortGoGPUTranslucentBrushFaceRenders(AlphaModeSorted, renders)
	if got := []int32{renders[0].face.face.TextureIndex, renders[1].face.face.TextureIndex, renders[2].face.face.TextureIndex}; !reflect.DeepEqual(got, []int32{2, 1, 3}) {
		t.Fatalf("mixed translucent face order = %v, want [2 1 3]", got)
	}
}

func TestEffectiveGoGPUAlphaModeFallsBackFromOITToSorted(t *testing.T) {
	if got := effectiveGoGPUAlphaMode(AlphaModeBasic); got != AlphaModeBasic {
		t.Fatalf("effectiveGoGPUAlphaMode(Basic) = %v, want %v", got, AlphaModeBasic)
	}
	if got := effectiveGoGPUAlphaMode(AlphaModeSorted); got != AlphaModeSorted {
		t.Fatalf("effectiveGoGPUAlphaMode(Sorted) = %v, want %v", got, AlphaModeSorted)
	}
	if got := effectiveGoGPUAlphaMode(AlphaModeOIT); got != AlphaModeSorted {
		t.Fatalf("effectiveGoGPUAlphaMode(OIT) = %v, want %v", got, AlphaModeSorted)
	}
}

func TestShouldDrawGoGPUOpaqueBrushFace(t *testing.T) {
	if !shouldDrawGoGPUOpaqueBrushFace(WorldFace{NumIndices: 3}, 1) {
		t.Fatal("opaque brush face should draw")
	}
	if shouldDrawGoGPUOpaqueBrushFace(WorldFace{NumIndices: 3, Flags: model.SurfDrawSky}, 1) {
		t.Fatal("sky brush face should not draw in opaque pass")
	}
	if shouldDrawGoGPUOpaqueBrushFace(WorldFace{NumIndices: 3, Flags: model.SurfDrawTurb | model.SurfDrawWater}, 1) {
		t.Fatal("liquid brush face should not draw in opaque pass")
	}
	if shouldDrawGoGPUOpaqueBrushFace(WorldFace{NumIndices: 3}, 0.5) {
		t.Fatal("translucent brush entity should not draw in opaque pass")
	}
}

func TestBuildGoGPUOpaqueBrushEntityDrawTransformsVerticesAndKeepsOpaqueFaces(t *testing.T) {
	geom := &WorldGeometry{
		Vertices: []WorldVertex{
			{Position: [3]float32{0, 0, 0}, TexCoord: [2]float32{0, 0}, LightmapCoord: [2]float32{0, 0}, Normal: [3]float32{0, 0, 1}},
			{Position: [3]float32{1, 0, 0}, TexCoord: [2]float32{1, 0}, LightmapCoord: [2]float32{1, 0}, Normal: [3]float32{0, 0, 1}},
			{Position: [3]float32{0, 1, 0}, TexCoord: [2]float32{0, 1}, LightmapCoord: [2]float32{0, 1}, Normal: [3]float32{0, 0, 1}},
		},
		Indices: []uint32{0, 1, 2, 0, 2, 1},
		Faces: []WorldFace{
			{FirstIndex: 0, NumIndices: 3, TextureIndex: 4, LightmapIndex: 2},
			{FirstIndex: 3, NumIndices: 3, TextureIndex: 7, LightmapIndex: 1, Flags: model.SurfDrawSky},
		},
	}
	entity := BrushEntity{
		Origin: [3]float32{10, 20, 30},
		Alpha:  1,
		Scale:  2,
	}
	draw := buildGoGPUOpaqueBrushEntityDraw(entity, geom)
	if draw == nil {
		t.Fatal("buildGoGPUOpaqueBrushEntityDraw() = nil")
	}
	if len(draw.faces) != 1 {
		t.Fatalf("len(draw.faces) = %d, want 1", len(draw.faces))
	}
	if len(draw.indices) != 3 {
		t.Fatalf("len(draw.indices) = %d, want 3", len(draw.indices))
	}
	if draw.faces[0].TextureIndex != 4 || draw.faces[0].LightmapIndex != 2 {
		t.Fatalf("opaque face metadata = %+v, want texture 4 lightmap 2", draw.faces[0])
	}
	if got := draw.vertices[1].Position; got != [3]float32{12, 20, 30} {
		t.Fatalf("vertex 1 position = %v, want [12 20 30]", got)
	}
	if got := draw.vertices[2].Position; got != [3]float32{10, 22, 30} {
		t.Fatalf("vertex 2 position = %v, want [10 22 30]", got)
	}
	if len(draw.centers) != 1 || draw.centers[0] != [3]float32{10, 20, 30} {
		t.Fatalf("face centers = %v, want [[10 20 30]]", draw.centers)
	}
}

func TestShouldDrawGoGPUSkyWorldFace(t *testing.T) {
	if !shouldDrawGoGPUSkyWorldFace(WorldFace{NumIndices: 3, Flags: model.SurfDrawSky}) {
		t.Fatal("sky face should draw in sky pass")
	}
	if shouldDrawGoGPUSkyWorldFace(WorldFace{NumIndices: 3}) {
		t.Fatal("non-sky face should not draw in sky pass")
	}
	if shouldDrawGoGPUSkyWorldFace(WorldFace{Flags: model.SurfDrawSky}) {
		t.Fatal("empty sky face should not draw in sky pass")
	}
}

func TestShouldDrawGoGPUSkyBrushFace(t *testing.T) {
	if !shouldDrawGoGPUSkyBrushFace(WorldFace{NumIndices: 3, Flags: model.SurfDrawSky}, 1) {
		t.Fatal("sky brush face should draw in sky pass")
	}
	if shouldDrawGoGPUSkyBrushFace(WorldFace{NumIndices: 3}, 1) {
		t.Fatal("non-sky brush face should not draw in sky pass")
	}
	if shouldDrawGoGPUSkyBrushFace(WorldFace{NumIndices: 3, Flags: model.SurfDrawSky}, 0) {
		t.Fatal("hidden brush entity should not draw in sky pass")
	}
}

func TestBuildGoGPUSkyBrushEntityDrawKeepsOnlySkyFaces(t *testing.T) {
	geom := &WorldGeometry{
		Vertices: []WorldVertex{
			{Position: [3]float32{0, 0, 0}},
			{Position: [3]float32{1, 0, 0}},
			{Position: [3]float32{0, 1, 0}},
		},
		Indices: []uint32{0, 1, 2, 0, 2, 1},
		Faces: []WorldFace{
			{FirstIndex: 0, NumIndices: 3, TextureIndex: 9, Flags: model.SurfDrawSky},
			{FirstIndex: 3, NumIndices: 3, TextureIndex: 5},
		},
	}
	draw := buildGoGPUSkyBrushEntityDraw(BrushEntity{Alpha: 1, Scale: 1}, geom)
	if draw == nil {
		t.Fatal("buildGoGPUSkyBrushEntityDraw() = nil")
	}
	if len(draw.faces) != 1 {
		t.Fatalf("len(draw.faces) = %d, want 1", len(draw.faces))
	}
	if draw.faces[0].TextureIndex != 9 || draw.faces[0].Flags&model.SurfDrawSky == 0 {
		t.Fatalf("sky face metadata = %+v, want sky texture 9", draw.faces[0])
	}
}

func TestShouldDrawGoGPUOpaqueLiquidFace(t *testing.T) {
	alpha := worldLiquidAlphaSettings{water: 1, lava: 1, slime: 1, tele: 1}
	if !shouldDrawGoGPUOpaqueLiquidFace(WorldFace{NumIndices: 3, Flags: model.SurfDrawTurb | model.SurfDrawWater}, alpha) {
		t.Fatal("opaque liquid face should draw in turbulent pass")
	}
	if shouldDrawGoGPUOpaqueLiquidFace(WorldFace{NumIndices: 3, Flags: model.SurfDrawSky | model.SurfDrawTurb}, alpha) {
		t.Fatal("sky face should not draw in turbulent pass")
	}
	if shouldDrawGoGPUOpaqueLiquidFace(WorldFace{NumIndices: 3}, alpha) {
		t.Fatal("non-liquid face should not draw in turbulent pass")
	}
}

func TestShouldDrawGoGPUOpaqueLiquidBrushFace(t *testing.T) {
	alpha := worldLiquidAlphaSettings{water: 1, lava: 1, slime: 1, tele: 1}
	if !shouldDrawGoGPUOpaqueLiquidBrushFace(WorldFace{NumIndices: 3, Flags: model.SurfDrawTurb | model.SurfDrawWater}, 1, alpha) {
		t.Fatal("opaque liquid brush face should draw")
	}
	if shouldDrawGoGPUOpaqueLiquidBrushFace(WorldFace{NumIndices: 3, Flags: model.SurfDrawTurb | model.SurfDrawWater}, 0.5, alpha) {
		t.Fatal("translucent brush entity should not draw in opaque liquid pass")
	}
	if shouldDrawGoGPUOpaqueLiquidBrushFace(WorldFace{NumIndices: 3}, 1, alpha) {
		t.Fatal("non-liquid brush face should not draw in opaque liquid pass")
	}
}

func TestShouldDrawGoGPUTranslucentLiquidBrushFace(t *testing.T) {
	alpha := worldLiquidAlphaSettings{water: 0.5, lava: 1, slime: 1, tele: 1}
	if !shouldDrawGoGPUTranslucentLiquidBrushFace(WorldFace{NumIndices: 3, Flags: model.SurfDrawTurb | model.SurfDrawWater}, 1, alpha) {
		t.Fatal("translucent liquid brush face should draw")
	}
	if shouldDrawGoGPUTranslucentLiquidBrushFace(WorldFace{NumIndices: 3, Flags: model.SurfDrawTurb | model.SurfDrawWater}, 0.5, alpha) {
		t.Fatal("translucent brush entity should not draw in opaque-entity translucent liquid pass")
	}
	if shouldDrawGoGPUTranslucentLiquidBrushFace(WorldFace{NumIndices: 3, Flags: model.SurfDrawTurb | model.SurfDrawLava}, 1, alpha) {
		t.Fatal("opaque liquid brush face should not draw in translucent liquid pass")
	}
}

func TestShouldDrawGoGPUTranslucentBrushEntityFace(t *testing.T) {
	alpha := worldLiquidAlphaSettings{water: 0.5, lava: 1, slime: 1, tele: 1}
	if !shouldDrawGoGPUTranslucentBrushEntityFace(WorldFace{NumIndices: 3}, 0.5, alpha) {
		t.Fatal("translucent non-liquid brush face should draw")
	}
	if !shouldDrawGoGPUTranslucentBrushEntityFace(WorldFace{NumIndices: 3, Flags: model.SurfDrawTurb | model.SurfDrawWater}, 0.5, alpha) {
		t.Fatal("translucent liquid brush face should draw")
	}
	if !shouldDrawGoGPUTranslucentBrushEntityFace(WorldFace{NumIndices: 3, Flags: model.SurfDrawFence}, 0.5, alpha) {
		t.Fatal("alpha-test translucent brush face should draw")
	}
	if shouldDrawGoGPUTranslucentBrushEntityFace(WorldFace{NumIndices: 3, Flags: model.SurfDrawSky}, 0.5, alpha) {
		t.Fatal("sky translucent brush face should not draw here")
	}
	if shouldDrawGoGPUTranslucentBrushEntityFace(WorldFace{NumIndices: 3}, 1, alpha) {
		t.Fatal("opaque brush entity should not use translucent brush pass")
	}
}

func TestBuildGoGPUOpaqueLiquidBrushEntityDrawKeepsOnlyOpaqueLiquidFaces(t *testing.T) {
	alpha := worldLiquidAlphaSettings{water: 1, lava: 1, slime: 0.5, tele: 1}
	geom := &WorldGeometry{
		Vertices: []WorldVertex{
			{Position: [3]float32{0, 0, 0}},
			{Position: [3]float32{1, 0, 0}},
			{Position: [3]float32{0, 1, 0}},
		},
		Indices: []uint32{0, 1, 2, 0, 2, 1},
		Faces: []WorldFace{
			{FirstIndex: 0, NumIndices: 3, TextureIndex: 11, Flags: model.SurfDrawTurb | model.SurfDrawWater},
			{FirstIndex: 3, NumIndices: 3, TextureIndex: 12, Flags: model.SurfDrawTurb | model.SurfDrawSlime},
		},
	}
	draw := buildGoGPUOpaqueLiquidBrushEntityDraw(BrushEntity{Alpha: 1, Scale: 1}, geom, alpha)
	if draw == nil {
		t.Fatal("buildGoGPUOpaqueLiquidBrushEntityDraw() = nil")
	}
	if len(draw.faces) != 1 {
		t.Fatalf("len(draw.faces) = %d, want 1", len(draw.faces))
	}
	if draw.faces[0].TextureIndex != 11 {
		t.Fatalf("opaque liquid face metadata = %+v, want texture 11", draw.faces[0])
	}
}

func TestBuildGoGPUTranslucentLiquidBrushEntityDrawKeepsOnlyTranslucentLiquidFaces(t *testing.T) {
	alpha := worldLiquidAlphaSettings{water: 0.5, lava: 1, slime: 1, tele: 1}
	geom := &WorldGeometry{
		Vertices: []WorldVertex{
			{Position: [3]float32{0, 0, 0}},
			{Position: [3]float32{1, 0, 0}},
			{Position: [3]float32{0, 1, 0}},
		},
		Indices: []uint32{0, 1, 2, 0, 2, 1},
		Faces: []WorldFace{
			{FirstIndex: 0, NumIndices: 3, TextureIndex: 21, Flags: model.SurfDrawTurb | model.SurfDrawWater, Center: [3]float32{0, 0, 8}},
			{FirstIndex: 3, NumIndices: 3, TextureIndex: 22, Flags: model.SurfDrawTurb | model.SurfDrawLava, Center: [3]float32{0, 0, 2}},
		},
	}
	draw := buildGoGPUTranslucentLiquidBrushEntityDraw(BrushEntity{Alpha: 1, Scale: 1}, geom, alpha, CameraState{})
	if draw == nil {
		t.Fatal("buildGoGPUTranslucentLiquidBrushEntityDraw() = nil")
	}
	if len(draw.faces) != 1 {
		t.Fatalf("len(draw.faces) = %d, want 1", len(draw.faces))
	}
	if draw.faces[0].face.TextureIndex != 21 {
		t.Fatalf("translucent liquid face metadata = %+v, want texture 21", draw.faces[0].face)
	}
	if !almostEqualWorldFloat32(draw.faces[0].alpha, 0.5, 1e-6) {
		t.Fatalf("translucent liquid face alpha = %v, want 0.5", draw.faces[0].alpha)
	}
}

func TestBuildGoGPUTranslucentBrushEntityDrawBucketsFaces(t *testing.T) {
	alpha := worldLiquidAlphaSettings{water: 0.5, lava: 1, slime: 1, tele: 1}
	geom := &WorldGeometry{
		Vertices: []WorldVertex{
			{Position: [3]float32{0, 0, 0}},
			{Position: [3]float32{1, 0, 0}},
			{Position: [3]float32{0, 1, 0}},
		},
		Indices: []uint32{0, 1, 2, 0, 2, 1, 0, 1, 2},
		Faces: []WorldFace{
			{FirstIndex: 0, NumIndices: 3, TextureIndex: 31, LightmapIndex: 2},
			{FirstIndex: 3, NumIndices: 3, TextureIndex: 32, Flags: model.SurfDrawTurb | model.SurfDrawWater, Center: [3]float32{0, 0, 4}},
			{FirstIndex: 6, NumIndices: 3, TextureIndex: 33, Flags: model.SurfDrawFence},
		},
	}
	draw := buildGoGPUTranslucentBrushEntityDraw(BrushEntity{Alpha: 0.5, Scale: 1}, geom, alpha, CameraState{})
	if draw == nil {
		t.Fatal("buildGoGPUTranslucentBrushEntityDraw() = nil")
	}
	if len(draw.translucentFaces) != 1 || draw.translucentFaces[0].face.TextureIndex != 31 {
		t.Fatalf("translucentFaces = %+v, want texture 31", draw.translucentFaces)
	}
	if draw.translucentFaces[0].center != [3]float32{0, 0, 0} {
		t.Fatalf("translucent face center = %v, want [0 0 0]", draw.translucentFaces[0].center)
	}
	if len(draw.liquidFaces) != 1 || draw.liquidFaces[0].face.TextureIndex != 32 {
		t.Fatalf("liquidFaces = %+v, want texture 32", draw.liquidFaces)
	}
	if draw.liquidFaces[0].center != [3]float32{0, 0, 4} {
		t.Fatalf("liquid face center = %v, want [0 0 4]", draw.liquidFaces[0].center)
	}
	if len(draw.alphaTestFaces) != 1 || draw.alphaTestFaces[0].TextureIndex != 33 {
		t.Fatalf("alphaTestFaces = %+v, want texture 33", draw.alphaTestFaces)
	}
	if len(draw.alphaTestCenters) != 1 || draw.alphaTestCenters[0] != [3]float32{0, 0, 0} {
		t.Fatalf("alphaTestCenters = %v, want [[0 0 0]]", draw.alphaTestCenters)
	}
}

func TestShouldDrawGoGPUTranslucentLiquidFace(t *testing.T) {
	alpha := worldLiquidAlphaSettings{water: 0.5, lava: 1, slime: 1, tele: 1}
	if !shouldDrawGoGPUTranslucentLiquidFace(WorldFace{NumIndices: 3, Flags: model.SurfDrawTurb | model.SurfDrawWater}, alpha) {
		t.Fatal("translucent liquid face should draw in translucent turbulent pass")
	}
	if shouldDrawGoGPUTranslucentLiquidFace(WorldFace{NumIndices: 3, Flags: model.SurfDrawTurb | model.SurfDrawLava}, alpha) {
		t.Fatal("opaque liquid face should not draw in translucent turbulent pass")
	}
}

func TestGoGPUWorldTextureForFace(t *testing.T) {
	fallback := &gpuWorldTexture{}
	base := &gpuWorldTexture{}
	animated := &gpuWorldTexture{}
	textures := map[int32]*gpuWorldTexture{1: base, 2: animated}
	animations := []*SurfaceTexture{
		nil,
		{TextureIndex: 1, AnimTotal: 4, AnimMin: 0, AnimMax: 2, AnimNext: &SurfaceTexture{TextureIndex: 2, AnimTotal: 4, AnimMin: 2, AnimMax: 4}},
		nil,
	}
	animations[1].AnimNext.AnimNext = animations[1]
	got := gogpuWorldTextureForFace(WorldFace{TextureIndex: 1}, textures, animations, fallback, 0, 0.3)
	if got != animated {
		t.Fatalf("animated world texture = %p, want %p", got, animated)
	}
	if got := gogpuWorldTextureForFace(WorldFace{TextureIndex: 99}, textures, animations, fallback, 0, 0); got != fallback {
		t.Fatalf("fallback world texture = %p, want %p", got, fallback)
	}
}

func TestExtractEmbeddedSkyLayersClassic(t *testing.T) {
	palette := make([]byte, 256*3)
	palette[3] = 255
	palette[6] = 64
	palette[7] = 128
	pixels := []byte{
		1, 0, 2, 2,
		1, 255, 2, 2,
	}
	solid, alpha, width, height, ok := extractEmbeddedSkyLayers(pixels, 4, 2, palette, false)
	if !ok {
		t.Fatal("extractEmbeddedSkyLayers() = not ok")
	}
	if width != 2 || height != 2 {
		t.Fatalf("layer size = %dx%d, want 2x2", width, height)
	}
	if got := solid[0]; got != 64 {
		t.Fatalf("solid first pixel red = %d, want 64", got)
	}
	if got := alpha[3]; got != 255 {
		t.Fatalf("alpha first pixel alpha = %d, want 255", got)
	}
	if got := alpha[7]; got != 0 {
		t.Fatalf("transparent sky pixel alpha = %d, want 0", got)
	}
}

func TestGoGPULightStylesChanged(t *testing.T) {
	var old, new_ [64]float32
	for i := range old {
		old[i] = 1
		new_[i] = 1
	}
	new_[5] = 0.5
	new_[10] = 2

	changed := lightStylesChanged(old, new_)
	if !changed[5] || !changed[10] {
		t.Fatalf("changed = %v, want style 5 and 10 marked", changed)
	}
	if changed[0] {
		t.Fatal("style 0 should not be marked changed")
	}
}

func TestGoGPUMarkDirtyLightmapPages(t *testing.T) {
	pages := []WorldLightmapPage{
		{
			Width: 64, Height: 64,
			Surfaces: []WorldLightmapSurface{
				{X: 0, Y: 0, Width: 4, Height: 4, Styles: [4]uint8{0, 255, 255, 255}},
				{X: 4, Y: 0, Width: 4, Height: 4, Styles: [4]uint8{5, 255, 255, 255}},
			},
		},
	}
	var changed [64]bool
	changed[5] = true
	markDirtyLightmapPages(pages, changed)
	if pages[0].Surfaces[0].Dirty {
		t.Fatal("style 0 surface should stay clean")
	}
	if !pages[0].Surfaces[1].Dirty || !pages[0].Dirty {
		t.Fatal("style 5 surface/page should be dirty")
	}
	clearDirtyFlags(pages)
	if pages[0].Dirty || pages[0].Surfaces[1].Dirty {
		t.Fatal("dirty flags should clear after clearDirtyFlags")
	}
}

func TestGoGPURecompositeDirtySurfaces(t *testing.T) {
	page := WorldLightmapPage{
		Width: 4, Height: 4,
		Surfaces: []WorldLightmapSurface{
			{
				X: 0, Y: 0, Width: 2, Height: 2,
				Styles:  [4]uint8{0, 255, 255, 255},
				Samples: []byte{128, 128, 128, 128, 128, 128, 128, 128, 128, 128, 128, 128},
				Dirty:   true,
			},
			{
				X: 2, Y: 0, Width: 2, Height: 2,
				Styles:  [4]uint8{1, 255, 255, 255},
				Samples: []byte{200, 200, 200, 200, 200, 200, 200, 200, 200, 200, 200, 200},
				Dirty:   false,
			},
		},
	}
	var values [64]float32
	values[0] = 1
	values[1] = 1
	rgba := buildWorldLightmapPageRGBA(&page, values)
	surface1Pixel := append([]byte(nil), rgba[8:12]...)
	values[0] = 0.5
	if !recompositeDirtySurfaces(rgba, page, values) {
		t.Fatal("recompositeDirtySurfaces should report work")
	}
	if rgba[0] >= 128 {
		t.Fatalf("surface 0 pixel = %d, want darkened value", rgba[0])
	}
	for i := 0; i < 4; i++ {
		if rgba[8+i] != surface1Pixel[i] {
			t.Fatalf("surface 1 pixel[%d] changed: got %d want %d", i, rgba[8+i], surface1Pixel[i])
		}
	}
}

func almostEqualWorldFloat32(a, b, epsilon float32) bool {
	return float32(math.Abs(float64(a-b))) <= epsilon
}
