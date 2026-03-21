//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/model"
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
