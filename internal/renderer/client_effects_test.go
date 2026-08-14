package renderer

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/model"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

func TestEmitDynamicLightsOnlyExplosionsSpawnLights(t *testing.T) {
	var lights []DynamicLight
	EmitDynamicLights(func(dl DynamicLight) bool {
		lights = append(lights, dl)
		return true
	}, []client.TempEntityEvent{
		{Type: inet.TE_SPIKE, Origin: types.Vec3{X: 1, Y: 1, Z: 1}},
		{Type: inet.TE_LIGHTNING1, Origin: types.Vec3{X: 2, Y: 2, Z: 2}},
		{Type: inet.TE_EXPLOSION, Origin: types.Vec3{X: 3, Y: 3, Z: 3}},
	})
	if got := len(lights); got != 1 {
		t.Fatalf("dynamic lights = %d, want 1", got)
	}
	if got := lights[0].Position; got != (types.Vec3{X: 3, Y: 3, Z: 3}) {
		t.Fatalf("light origin = %v, want explosion origin", got)
	}
}

func TestEmitEntityEffectLightsAddsRocketLightFromModelFlags(t *testing.T) {
	var lights []DynamicLight
	EmitEntityEffectLights(func(dl DynamicLight) bool {
		lights = append(lights, dl)
		return true
	}, []EntityEffectSource{{
		Origin:     types.Vec3{X: 4, Y: 5, Z: 6},
		ModelFlags: model.EFRocket,
		EntityNum:  7,
	}})
	if got := len(lights); got != 1 {
		t.Fatalf("effect lights = %d, want 1", got)
	}
	if got := lights[0].Radius; got != 200 {
		t.Fatalf("rocket light radius = %v, want 200", got)
	}
	if got := lights[0].EntityKey; got != 7 {
		t.Fatalf("rocket light key = %d, want 7", got)
	}
}

func TestEmitEntityEffectLightsMuzzleFlashSetsMinLight(t *testing.T) {
	var lights []DynamicLight
	EmitEntityEffectLights(func(dl DynamicLight) bool {
		lights = append(lights, dl)
		return true
	}, []EntityEffectSource{{
		Origin:    types.Vec3{X: 4, Y: 5, Z: 6},
		Effects:   inet.EF_MUZZLEFLASH,
		EntityNum: 7,
	}})
	if got := len(lights); got != 1 {
		t.Fatalf("effect lights = %d, want 1", got)
	}
	if got := lights[0].MinLight; got != 32 {
		t.Fatalf("muzzle flash minlight = %v, want 32", got)
	}
}
