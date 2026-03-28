package renderer

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/model"
	inet "github.com/darkliquid/ironwail-go/internal/net"
)

func TestEmitDynamicLightsOnlyExplosionsSpawnLights(t *testing.T) {
	var lights []DynamicLight
	EmitDynamicLights(func(dl DynamicLight) bool {
		lights = append(lights, dl)
		return true
	}, []client.TempEntityEvent{
		{Type: inet.TE_SPIKE, Origin: [3]float32{1, 1, 1}},
		{Type: inet.TE_LIGHTNING1, Origin: [3]float32{2, 2, 2}},
		{Type: inet.TE_EXPLOSION, Origin: [3]float32{3, 3, 3}},
	})
	if got := len(lights); got != 1 {
		t.Fatalf("dynamic lights = %d, want 1", got)
	}
	if got := lights[0].Position; got != [3]float32{3, 3, 3} {
		t.Fatalf("light origin = %v, want explosion origin", got)
	}
}

func TestEmitEntityEffectLightsAddsRocketLightFromModelFlags(t *testing.T) {
	var lights []DynamicLight
	EmitEntityEffectLights(func(dl DynamicLight) bool {
		lights = append(lights, dl)
		return true
	}, []EntityEffectSource{{
		Origin:     [3]float32{4, 5, 6},
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
