//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/ironwail/ironwail-go/internal/model"
	"github.com/ironwail/ironwail-go/pkg/types"
)

func TestSetupAliasFrameInterpolationBlendsMultiPoseFrame(t *testing.T) {
	frames := []model.AliasFrameDesc{{FirstPose: 2, NumPoses: 2, Interval: 0.2}}
	result := setupAliasFrameInterpolation(0, frames, 0.1, true, 0)
	if result.Pose1 != 2 || result.Pose2 != 3 {
		t.Fatalf("poses = (%d,%d), want (2,3)", result.Pose1, result.Pose2)
	}
	if math.Abs(float64(result.Blend-0.5)) > 0.001 {
		t.Fatalf("blend = %f, want 0.5", result.Blend)
	}
}

func TestSetupAliasFrameInterpolationHonorsModNoLerp(t *testing.T) {
	frames := []model.AliasFrameDesc{{FirstPose: 2, NumPoses: 2, Interval: 0.2}}
	result := setupAliasFrameInterpolation(0, frames, 0.1, true, ModNoLerp)
	if result.Blend != 0 {
		t.Fatalf("blend with ModNoLerp = %f, want 0", result.Blend)
	}
}

func TestBuildAliasVerticesInterpolatedAppliesYawRotation(t *testing.T) {
	mdl := &model.Model{AliasHeader: &model.AliasHeader{Scale: [3]float32{1, 1, 1}, ScaleOrigin: [3]float32{0, 0, 0}}}
	alias := &gpuAliasModel{
		refs: []gpuAliasVertexRef{{vertexIndex: 0, texCoord: [2]float32{0.25, 0.75}}},
		poses: [][]model.TriVertX{
			{{V: [3]byte{1, 0, 0}}},
			{{V: [3]byte{1, 0, 0}}},
		},
	}

	vertices := buildAliasVerticesInterpolated(alias, mdl, 0, 1, 0, [3]float32{4, 5, 6}, [3]float32{0, 90, 0}, 1, false)
	if len(vertices) != 1 {
		t.Fatalf("vertex count = %d, want 1", len(vertices))
	}
	got := vertices[0]
	if math.Abs(float64(got.Position[0]-4)) > 0.001 || math.Abs(float64(got.Position[1]-6)) > 0.001 || math.Abs(float64(got.Position[2]-6)) > 0.001 {
		t.Fatalf("rotated position = %v, want [4 6 6]", got.Position)
	}
	if got.TexCoord != [2]float32{0.25, 0.75} {
		t.Fatalf("texcoord = %v, want [0.25 0.75]", got.TexCoord)
	}
}

func TestResolveAliasSkinSlotUsesGroupedSkinTimingGoGPU(t *testing.T) {
	hdr := &model.AliasHeader{
		Skins: make([][]byte, 4),
		SkinDescs: []model.AliasSkinDesc{
			{FirstFrame: 0, NumFrames: 1},
			{FirstFrame: 1, NumFrames: 3, Intervals: []float32{0.1, 0.2, 0.3}},
		},
	}

	if got := resolveAliasSkinSlot(hdr, 1, 0.05, 4); got != 1 {
		t.Fatalf("slot at t=0.05 = %d, want 1", got)
	}
	if got := resolveAliasSkinSlot(hdr, 1, 0.15, 4); got != 2 {
		t.Fatalf("slot at t=0.15 = %d, want 2", got)
	}
	if got := resolveAliasSkinSlot(hdr, 1, 0.25, 4); got != 3 {
		t.Fatalf("slot at t=0.25 = %d, want 3", got)
	}
	if got := resolveAliasSkinSlot(hdr, 1, 0.35, 4); got != 1 {
		t.Fatalf("slot at t=0.35 = %d, want 1", got)
	}
}

func TestAliasSceneUniformBytesEncodesAlphaAndFog(t *testing.T) {
	vp := types.Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
	cameraOrigin := [3]float32{1, 2, 3}
	fogColor := [3]float32{0.2, 0.4, 0.6}
	fogDensity := float32(0.75)
	alpha := float32(0.5)

	data := aliasSceneUniformBytes(vp, cameraOrigin, alpha, fogColor, fogDensity)
	if len(data) != aliasSceneUniformBufferSize {
		t.Fatalf("len(aliasSceneUniformBytes()) = %d, want %d", len(data), aliasSceneUniformBufferSize)
	}
	for i, want := range cameraOrigin {
		got := math.Float32frombits(binary.LittleEndian.Uint32(data[64+i*4 : 68+i*4]))
		if !almostEqualAliasFloat32(got, want, 1e-6) {
			t.Fatalf("cameraOrigin[%d] = %v, want %v", i, got, want)
		}
	}
	gotFogDensity := math.Float32frombits(binary.LittleEndian.Uint32(data[76:80]))
	wantFogDensity := worldFogUniformDensity(fogDensity)
	if !almostEqualAliasFloat32(gotFogDensity, wantFogDensity, 1e-6) {
		t.Fatalf("fog density = %v, want %v", gotFogDensity, wantFogDensity)
	}
	for i, want := range fogColor {
		got := math.Float32frombits(binary.LittleEndian.Uint32(data[80+i*4 : 84+i*4]))
		if !almostEqualAliasFloat32(got, want, 1e-6) {
			t.Fatalf("fogColor[%d] = %v, want %v", i, got, want)
		}
	}
	gotAlpha := math.Float32frombits(binary.LittleEndian.Uint32(data[92:96]))
	if !almostEqualAliasFloat32(gotAlpha, alpha, 1e-6) {
		t.Fatalf("alpha = %v, want %v", gotAlpha, alpha)
	}
}

func TestAliasFragmentShaderIncludesFogMix(t *testing.T) {
	checks := []string{
		"cameraOrigin",
		"fogDensity",
		"fogColor",
		"worldPosition",
		"exp2(",
		"mix(uniforms.fogColor, lit, fog)",
	}
	for _, check := range checks {
		if !strings.Contains(aliasFragmentShaderWGSL, check) && !strings.Contains(aliasVertexShaderWGSL, check) {
			t.Fatalf("alias shaders missing %q", check)
		}
	}
}

func TestAliasFragmentShaderAvoidsDirectionalDiffuseHack(t *testing.T) {
	disallowed := []string{
		"lightDir",
		"dot(normal, lightDir)",
		"sampled.rgb * diffuse",
	}
	for _, check := range disallowed {
		if strings.Contains(aliasFragmentShaderWGSL, check) {
			t.Fatalf("alias fragment shader still contains %q", check)
		}
	}
	required := []string{
		"let lit = sampled.rgb + fullbright.rgb * fullbright.a;",
	}
	for _, check := range required {
		if !strings.Contains(aliasFragmentShaderWGSL, check) {
			t.Fatalf("alias fragment shader missing %q", check)
		}
	}
}

func TestAliasShadowUniformBytesEncodesFog(t *testing.T) {
	vp := types.Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
	cameraOrigin := [3]float32{4, 5, 6}
	fogColor := [3]float32{0.2, 0.3, 0.4}
	fogDensity := float32(0.75)
	alpha := float32(0.5)

	data := aliasShadowUniformBytes(vp, cameraOrigin, alpha, fogColor, fogDensity)
	if len(data) != aliasSceneUniformBufferSize {
		t.Fatalf("len(aliasShadowUniformBytes()) = %d, want %d", len(data), aliasSceneUniformBufferSize)
	}
	gotFogDensity := math.Float32frombits(binary.LittleEndian.Uint32(data[76:80]))
	wantFogDensity := worldFogUniformDensity(fogDensity)
	if !almostEqualAliasFloat32(gotFogDensity, wantFogDensity, 1e-6) {
		t.Fatalf("shadow fog density = %v, want %v", gotFogDensity, wantFogDensity)
	}
	for i, want := range cameraOrigin {
		gotOrigin := math.Float32frombits(binary.LittleEndian.Uint32(data[64+i*4 : 68+i*4]))
		if !almostEqualAliasFloat32(gotOrigin, want, 1e-6) {
			t.Fatalf("cameraOrigin[%d] = %v, want %v", i, gotOrigin, want)
		}
	}
	for i, want := range fogColor {
		gotFogColor := math.Float32frombits(binary.LittleEndian.Uint32(data[80+i*4 : 84+i*4]))
		if !almostEqualAliasFloat32(gotFogColor, want, 1e-6) {
			t.Fatalf("fogColor[%d] = %v, want %v", i, gotFogColor, want)
		}
	}
	gotAlpha := math.Float32frombits(binary.LittleEndian.Uint32(data[92:96]))
	if !almostEqualAliasFloat32(gotAlpha, alpha, 1e-6) {
		t.Fatalf("alpha = %v, want %v", gotAlpha, alpha)
	}
}

func almostEqualAliasFloat32(a, b, epsilon float32) bool {
	return float32(math.Abs(float64(a-b))) <= epsilon
}
