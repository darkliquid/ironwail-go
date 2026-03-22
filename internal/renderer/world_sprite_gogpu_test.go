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

func TestSpriteUniformBytesEncodesAlphaAndFog(t *testing.T) {
	vp := types.Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
	cameraOrigin := [3]float32{1, 2, 3}
	fogColor := [3]float32{0.25, 0.5, 0.75}
	fogDensity := float32(0.6)
	alpha := float32(0.4)

	data := spriteUniformBytes(vp, cameraOrigin, alpha, fogColor, fogDensity)
	if len(data) != spriteUniformBufferSize {
		t.Fatalf("len(spriteUniformBytes()) = %d, want %d", len(data), spriteUniformBufferSize)
	}
	for i, want := range cameraOrigin {
		got := math.Float32frombits(binary.LittleEndian.Uint32(data[64+i*4 : 68+i*4]))
		if !almostEqualSpriteFloat32(got, want, 1e-6) {
			t.Fatalf("cameraOrigin[%d] = %v, want %v", i, got, want)
		}
	}
	gotFogDensity := math.Float32frombits(binary.LittleEndian.Uint32(data[76:80]))
	wantFogDensity := worldFogUniformDensity(fogDensity)
	if !almostEqualSpriteFloat32(gotFogDensity, wantFogDensity, 1e-6) {
		t.Fatalf("fog density = %v, want %v", gotFogDensity, wantFogDensity)
	}
	for i, want := range fogColor {
		got := math.Float32frombits(binary.LittleEndian.Uint32(data[80+i*4 : 84+i*4]))
		if !almostEqualSpriteFloat32(got, want, 1e-6) {
			t.Fatalf("fogColor[%d] = %v, want %v", i, got, want)
		}
	}
	gotAlpha := math.Float32frombits(binary.LittleEndian.Uint32(data[92:96]))
	if !almostEqualSpriteFloat32(gotAlpha, alpha, 1e-6) {
		t.Fatalf("alpha = %v, want %v", gotAlpha, alpha)
	}
}

func TestSpriteFragmentShaderIncludesFogMix(t *testing.T) {
	checks := []string{
		"cameraOrigin",
		"fogDensity",
		"fogColor",
		"worldPosition",
		"exp2(",
		"mix(uniforms.fogColor, sampled.rgb, fog)",
	}
	for _, check := range checks {
		if !strings.Contains(spriteFragmentShaderWGSL, check) && !strings.Contains(spriteVertexShaderWGSL, check) {
			t.Fatalf("sprite shaders missing %q", check)
		}
	}
}

func TestBuildSpriteDrawLockedFallsBackToModelSpriteData(t *testing.T) {
	r := &Renderer{
		spriteModels: map[string]*gpuSpriteModel{
			"progs/flame.spr": {frames: []gpuSpriteFrame{{}}},
		},
	}
	entity := SpriteEntity{
		ModelID: "progs/flame.spr",
		Model: &model.Model{
			Type: model.ModSprite,
			Mins: [3]float32{-16, -4, -6},
			Maxs: [3]float32{16, 4, 10},
		},
		Frame: 0,
		Alpha: 1,
		Scale: 1,
	}

	draw := r.buildSpriteDrawLocked(nil, nil, entity)
	if draw == nil {
		t.Fatal("buildSpriteDrawLocked returned nil")
	}
	if draw.sprite == nil {
		t.Fatal("buildSpriteDrawLocked should reuse cached sprite model")
	}
}

func almostEqualSpriteFloat32(a, b, epsilon float32) bool {
	return float32(math.Abs(float64(a-b))) <= epsilon
}
