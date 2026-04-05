package renderer

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/model"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
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

func TestQuantizeGoGPUWorldDynamicLightScalar(t *testing.T) {
	if got := quantizeGoGPUWorldDynamicLightScalar(0.001); got != 0 {
		t.Fatalf("quantizeGoGPUWorldDynamicLightScalar(0.001) = %v, want 0", got)
	}
	if got := quantizeGoGPUWorldDynamicLightScalar(0.27); got != 0.28125 {
		t.Fatalf("quantizeGoGPUWorldDynamicLightScalar(0.27) = %v, want 0.28125", got)
	}
}

func TestGoGPUWorldDynamicLightSignatureIgnoresTinyFadeDrift(t *testing.T) {
	base := DynamicLight{
		Position:   [3]float32{10, 20, 30},
		Radius:     200,
		Color:      [3]float32{1, 0.5, 0.25},
		Brightness: 1,
		Lifetime:   10,
		Age:        1,
		Type:       2,
		EntityKey:  99,
	}
	drifted := base
	drifted.Age = 1.01
	if got, want := gogpuWorldDynamicLightSignature([]DynamicLight{base}), gogpuWorldDynamicLightSignature([]DynamicLight{drifted}); got != want {
		t.Fatalf("signature changed for tiny fade drift: base=%d drifted=%d", got, want)
	}
}

func TestGoGPUWorldDynamicLightSignatureTracksVisualLightChanges(t *testing.T) {
	base := DynamicLight{
		Position:   [3]float32{10, 20, 30},
		Radius:     200,
		Color:      [3]float32{1, 0.5, 0.25},
		Brightness: 1,
		Lifetime:   10,
		Age:        1,
		Type:       2,
	}
	changed := base
	changed.Age = 2
	if got, want := gogpuWorldDynamicLightSignature([]DynamicLight{base}), gogpuWorldDynamicLightSignature([]DynamicLight{changed}); got == want {
		t.Fatalf("signature did not change for visible fade step: base=%d changed=%d", got, want)
	}
}

func TestBuildMaterialTextureRGBA_SuppressesRegularFullbrightInDiffuse(t *testing.T) {
	palette := make([]byte, 768)
	palette[5*3+0], palette[5*3+1], palette[5*3+2] = 10, 20, 30
	palette[224*3+0], palette[224*3+1], palette[224*3+2] = 200, 150, 100
	palette[255*3+0], palette[255*3+1], palette[255*3+2] = 7, 8, 9

	got := worldimpl.BuildMaterialTextureRGBA([]byte{5, 224, 255}, palette, model.TexTypeDefault)
	if got.HasFullbright {
		t.Fatal("did not expect separate fullbright texture for regular world material")
	}
	wantDiffuse := []byte{
		10, 20, 30, 255,
		200, 150, 100, 0,
		7, 8, 9, 255,
	}
	if string(got.DiffuseRGBA) != string(wantDiffuse) {
		t.Fatalf("diffuse = %v, want %v", got.DiffuseRGBA, wantDiffuse)
	}
	if got.FullbrightRGBA != nil {
		t.Fatalf("fullbright = %v, want nil", got.FullbrightRGBA)
	}
}

func TestExpandLightmapSamplesRGBUsesTripledOffset(t *testing.T) {
	lighting := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8}
	got := worldimpl.ExpandLightmapSamples(lighting, true, 1, 2)
	want := []byte{3, 4, 5, 6, 7, 8}
	if string(got) != string(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBuildMaterialTextureRGBA_PreservesCutoutTransparency(t *testing.T) {
	palette := make([]byte, 768)
	palette[5*3+0], palette[5*3+1], palette[5*3+2] = 10, 20, 30
	palette[224*3+0], palette[224*3+1], palette[224*3+2] = 200, 150, 100

	got := worldimpl.BuildMaterialTextureRGBA([]byte{5, 224, 255}, palette, model.TexTypeCutout)
	if !got.HasFullbright {
		t.Fatal("expected separate fullbright texture for cutout material")
	}
	wantDiffuse := []byte{
		10, 20, 30, 255,
		0, 0, 0, 255,
		0, 0, 0, 0,
	}
	if string(got.DiffuseRGBA) != string(wantDiffuse) {
		t.Fatalf("diffuse = %v, want %v", got.DiffuseRGBA, wantDiffuse)
	}
	wantFullbright := []byte{
		0, 0, 0, 0,
		200, 150, 100, 255,
		0, 0, 0, 0,
	}
	if string(got.FullbrightRGBA) != string(wantFullbright) {
		t.Fatalf("fullbright = %v, want %v", got.FullbrightRGBA, wantFullbright)
	}
}
