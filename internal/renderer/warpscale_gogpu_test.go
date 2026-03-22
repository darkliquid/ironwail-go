//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestSceneCompositeUniformBytes(t *testing.T) {
	got := sceneCompositeUniformBytes(true, 12.5)
	if len(got) != sceneCompositeUniformBufferSize {
		t.Fatalf("uniform byte len = %d, want %d", len(got), sceneCompositeUniformBufferSize)
	}
	values := [4]float32{}
	for i := range values {
		values[i] = math.Float32frombits(binary.LittleEndian.Uint32(got[i*4:]))
	}
	if values[0] != 1 || values[1] != 1 {
		t.Fatalf("uv scale = (%v,%v), want (1,1)", values[0], values[1])
	}
	if math.Abs(float64(values[2]-(1.0/256.0))) > 0.000001 {
		t.Fatalf("warp amp = %v, want %v", values[2], 1.0/256.0)
	}
	if values[3] != 12.5 {
		t.Fatalf("warp time = %v, want 12.5", values[3])
	}

	got = sceneCompositeUniformBytes(false, 3)
	values[2] = math.Float32frombits(binary.LittleEndian.Uint32(got[8:12]))
	if values[2] != 0 {
		t.Fatalf("warp amp without warp = %v, want 0", values[2])
	}
}

func TestShouldUseSceneRenderTarget(t *testing.T) {
	tests := []struct {
		name  string
		state *RenderFrameState
		want  bool
	}{
		{
			name:  "nil state",
			state: nil,
			want:  false,
		},
		{
			name: "waterwarp disabled",
			state: &RenderFrameState{
				DrawWorld: true,
			},
			want: false,
		},
		{
			name: "world scene enables target",
			state: &RenderFrameState{
				WaterWarp: true,
				DrawWorld: true,
			},
			want: true,
		},
		{
			name: "entity only scene enables target",
			state: &RenderFrameState{
				WaterWarp:    true,
				DrawEntities: true,
			},
			want: true,
		},
		{
			name: "particles only scene enables target",
			state: &RenderFrameState{
				WaterWarp:     true,
				DrawParticles: true,
				Particles:     &ParticleSystem{},
			},
			want: true,
		},
		{
			name: "empty particle flag does not enable target",
			state: &RenderFrameState{
				WaterWarp:     true,
				DrawParticles: true,
			},
			want: false,
		},
		{
			name: "decal only scene enables target",
			state: &RenderFrameState{
				WaterWarp:  true,
				DecalMarks: []DecalMarkEntity{{}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldUseSceneRenderTarget(tt.state); got != tt.want {
				t.Fatalf("shouldUseSceneRenderTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}
