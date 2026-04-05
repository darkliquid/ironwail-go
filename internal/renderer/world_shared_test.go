package renderer

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

func TestReadWorldProceduralSkyEnabled(t *testing.T) {
	cvar.Register(CvarRProceduralSky, "0", 0, "")
	cvar.Set(CvarRProceduralSky, "1")
	t.Cleanup(func() {
		cvar.Set(CvarRProceduralSky, "0")
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
