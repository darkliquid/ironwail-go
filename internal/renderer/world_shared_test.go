package renderer

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestReadWorldProceduralSkyEnabled(t *testing.T) {
	testCV.Register(CvarRProceduralSky, "0", 0, "")
	testCV.Set(CvarRProceduralSky, "1")
	t.Cleanup(func() {
		testCV.Set(CvarRProceduralSky, "0")
	})

	if !readWorldProceduralSkyEnabled() {
		t.Fatal("readWorldProceduralSkyEnabled() = false, want true")
	}
}

func TestProceduralSkyGradientColorsDeterministic(t *testing.T) {
	horizon, zenith := proceduralSkyGradientColors()
	if horizon != ([3]float32{0.40, 0.53, 0.78}) {
		t.Fatalf("horizon = %v, want [0.4 0.53 0.78]", horizon)
	}
	if zenith != ([3]float32{0.07, 0.10, 0.23}) {
		t.Fatalf("zenith = %v, want [0.07 0.10 0.23]", zenith)
	}
}

func TestShouldUseProceduralSky(t *testing.T) {
	tests := []struct {
		name        string
		fastSky     bool
		procedural  bool
		external    externalSkyboxRenderMode
		wantEnabled bool
	}{
		{name: "embedded fast sky enabled", fastSky: true, procedural: true, external: externalSkyboxRenderEmbedded, wantEnabled: true},
		{name: "disabled cvar", fastSky: true, procedural: false, external: externalSkyboxRenderEmbedded, wantEnabled: false},
		{name: "not fast sky", fastSky: false, procedural: true, external: externalSkyboxRenderEmbedded, wantEnabled: false},
		{name: "cubemap external sky", fastSky: true, procedural: true, external: externalSkyboxRenderCubemap, wantEnabled: false},
		{name: "external faces sky", fastSky: true, procedural: true, external: externalSkyboxRenderFaces, wantEnabled: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldUseProceduralSky(tt.fastSky, tt.procedural, tt.external); got != tt.wantEnabled {
				t.Fatalf("shouldUseProceduralSky(%v, %v, %v) = %v, want %v", tt.fastSky, tt.procedural, tt.external, got, tt.wantEnabled)
			}
		})
	}
}

func TestQuantizeGoGPUWorldDynamicLight(t *testing.T) {
	tests := []struct {
		name  string
		input [3]float32
		want  [3]float32
	}{
		{
			name:  "tiny contributions are dropped",
			input: [3]float32{0.001, -0.002, 0.0},
			want:  [3]float32{0, 0, 0},
		},
		{
			name:  "values quantize to 1 over 32 steps",
			input: [3]float32{0.12, 0.27, 0.49},
			want:  [3]float32{0.125, 0.28125, 0.5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := quantizeGoGPUWorldDynamicLight(tt.input)
			if got != tt.want {
				t.Fatalf("quantizeGoGPUWorldDynamicLight(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestQuantizeGoGPUWorldDynamicLightScalar(t *testing.T) {
	if got := quantizeGoGPUWorldDynamicLightScalar(0.001); got != 0 {
		t.Fatalf("quantizeGoGPUWorldDynamicLightScalar(0.001) = %v, want 0", got)
	}
	if got := quantizeGoGPUWorldDynamicLightScalar(0.27); got != 0.28125 {
		t.Fatalf("quantizeGoGPUWorldDynamicLightScalar(0.27) = %v, want 0.28125", got)
	}
}

func TestEncodeGoGPUWorldDynamicLights(t *testing.T) {
	lights := []DynamicLight{{
		Position:   [3]float32{10, 20, 30},
		Radius:     200,
		Color:      [3]float32{1, 0.5, 0.25},
		Brightness: 2,
		MinLight:   32,
		Lifetime:   10,
		Age:        2.5,
	}}
	got := encodeGoGPUWorldDynamicLights(lights)
	if len(got) != gogpuWorldDynamicLightHeaderSize+gogpuWorldDynamicLightBufferStride {
		t.Fatalf("encoded byte length = %d, want %d", len(got), gogpuWorldDynamicLightHeaderSize+gogpuWorldDynamicLightBufferStride)
	}
	if count := binary.LittleEndian.Uint32(got[:4]); count != 1 {
		t.Fatalf("encoded count = %d, want 1", count)
	}
	base := gogpuWorldDynamicLightHeaderSize
	if radius := math.Float32frombits(binary.LittleEndian.Uint32(got[base+12 : base+16])); radius != 200 {
		t.Fatalf("encoded radius = %v, want 200", radius)
	}
	if minLight := math.Float32frombits(binary.LittleEndian.Uint32(got[base+28 : base+32])); minLight != 32 {
		t.Fatalf("encoded minlight = %v, want 32", minLight)
	}
	wantMul := float32(1.5)
	if colorR := math.Float32frombits(binary.LittleEndian.Uint32(got[base+16 : base+20])); colorR != 1*wantMul {
		t.Fatalf("encoded red = %v, want %v", colorR, 1*wantMul)
	}
	if colorG := math.Float32frombits(binary.LittleEndian.Uint32(got[base+20 : base+24])); colorG != 0.5*wantMul {
		t.Fatalf("encoded green = %v, want %v", colorG, 0.5*wantMul)
	}
	if colorB := math.Float32frombits(binary.LittleEndian.Uint32(got[base+24 : base+28])); colorB != 0.25*wantMul {
		t.Fatalf("encoded blue = %v, want %v", colorB, 0.25*wantMul)
	}
}
