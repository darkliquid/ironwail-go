//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"encoding/binary"
	"math"
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

	data := worldSceneUniformBytes(vp, cameraOrigin, fogColor, fogDensity)
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
}

func TestWorldShadersIncludeFogMix(t *testing.T) {
	vertexChecks := []string{
		"cameraOrigin",
		"fogDensity",
		"fogColor",
		"worldPos",
	}
	for _, check := range vertexChecks {
		if !strings.Contains(worldVertexShaderWGSL, check) {
			t.Fatalf("world vertex shader missing %q", check)
		}
	}

	fragmentChecks := []string{
		"cameraOrigin",
		"fogDensity",
		"fogColor",
		"worldPos",
		"exp2(",
		"mix(uniforms.fogColor, debugColor, fog)",
	}
	for _, check := range fragmentChecks {
		if !strings.Contains(worldFragmentShaderWGSL, check) {
			t.Fatalf("world fragment shader missing %q", check)
		}
	}
}

func almostEqualWorldFloat32(a, b, epsilon float32) bool {
	return float32(math.Abs(float64(a-b))) <= epsilon
}
