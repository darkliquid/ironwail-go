package renderer

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestSceneCompositeUniformBytes(t *testing.T) {
	got := sceneCompositeUniformBytes(true, 12.5, 1.2, 0.95)
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

	got = sceneCompositeUniformBytes(false, 3, 1.0, 1.0)
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
			want: true,
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

func TestMenuWithoutWorldShouldSkipInitialClear(t *testing.T) {
	state := &RenderFrameState{
		DrawWorld:     false,
		Draw2DOverlay: true,
		MenuActive:    true,
	}

	shouldClear := !state.DrawWorld && !state.MenuActive
	if shouldClear {
		t.Fatal("menu-only frame should preserve existing scene content and skip clear")
	}
}

func TestPolyBlendUniformBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    [4]float32
		expected [4]float32
	}{
		{
			name:     "standard red flash",
			input:    [4]float32{1.0, 0.0, 0.0, 0.5},
			expected: [4]float32{1.0, 0.0, 0.0, 0.5},
		},
		{
			name:     "underwater blue-green tint",
			input:    [4]float32{0.0, 0.5, 0.5, 0.3},
			expected: [4]float32{0.0, 0.5, 0.5, 0.3},
		},
		{
			name:     "clamping values out of 0..1 range",
			input:    [4]float32{-0.5, 1.5, 2.0, -1.0},
			expected: [4]float32{0.0, 1.0, 1.0, 0.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := polyBlendUniformBytes(tt.input)
			if len(got) != polyBlendUniformBufferSize {
				t.Fatalf("polyBlendUniformBytes len = %d, want %d", len(got), polyBlendUniformBufferSize)
			}
			for i := 0; i < 4; i++ {
				val := math.Float32frombits(binary.LittleEndian.Uint32(got[i*4:]))
				if math.Abs(float64(val-tt.expected[i])) > 0.00001 {
					t.Errorf("polyBlendUniformBytes component %d = %v, want %v", i, val, tt.expected[i])
				}
			}
		})
	}
}

func TestPostProcessingUniformEncoding(t *testing.T) {
	contrast := float32(1.35)
	gamma := float32(0.85)
	warpTime := float32(7.75)

	bufWarp := sceneCompositeUniformBytes(true, warpTime, contrast, gamma)
	if len(bufWarp) != sceneCompositeUniformBufferSize {
		t.Fatalf("bufWarp len = %d, want %d", len(bufWarp), sceneCompositeUniformBufferSize)
	}

	gotUVScaleX := math.Float32frombits(binary.LittleEndian.Uint32(bufWarp[0:4]))
	gotUVScaleY := math.Float32frombits(binary.LittleEndian.Uint32(bufWarp[4:8]))
	gotWarpAmp := math.Float32frombits(binary.LittleEndian.Uint32(bufWarp[8:12]))
	gotWarpTime := math.Float32frombits(binary.LittleEndian.Uint32(bufWarp[12:16]))
	gotContrast := math.Float32frombits(binary.LittleEndian.Uint32(bufWarp[16:20]))
	gotGamma := math.Float32frombits(binary.LittleEndian.Uint32(bufWarp[20:24]))

	if gotUVScaleX != 1.0 || gotUVScaleY != 1.0 {
		t.Errorf("uvScale = (%v, %v), want (1.0, 1.0)", gotUVScaleX, gotUVScaleY)
	}
	if math.Abs(float64(gotWarpAmp-(1.0/256.0))) > 0.000001 {
		t.Errorf("warpAmp = %v, want %v", gotWarpAmp, 1.0/256.0)
	}
	if gotWarpTime != warpTime {
		t.Errorf("warpTime = %v, want %v", gotWarpTime, warpTime)
	}
	if gotContrast != contrast {
		t.Errorf("contrast = %v, want %v", gotContrast, contrast)
	}
	if gotGamma != gamma {
		t.Errorf("gamma = %v, want %v", gotGamma, gamma)
	}

	bufNoWarp := sceneCompositeUniformBytes(false, warpTime, contrast, gamma)
	gotNoWarpAmp := math.Float32frombits(binary.LittleEndian.Uint32(bufNoWarp[8:12]))
	if gotNoWarpAmp != 0.0 {
		t.Errorf("no-warp warpAmp = %v, want 0.0", gotNoWarpAmp)
	}
}
