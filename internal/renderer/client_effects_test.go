package renderer

import (
	"math/rand"
	"testing"

	cl "github.com/ironwail/ironwail-go/internal/client"
	inet "github.com/ironwail/ironwail-go/internal/net"
)

func TestEmitClientEffectsMapsEvents(t *testing.T) {
	ps := NewParticleSystem(4096)
	rng := rand.New(rand.NewSource(6))

	EmitClientEffects(ps,
		[]cl.ParticleEvent{{Origin: [3]float32{1, 2, 3}, Dir: [3]float32{1, 0, 0}, Count: 12, Color: 99}},
		[]cl.TempEntityEvent{
			{Type: inet.TE_EXPLOSION, Origin: [3]float32{4, 5, 6}},
			{Type: inet.TE_TAREXPLOSION, Origin: [3]float32{7, 8, 9}},
			{Type: inet.TE_BEAM, Entity: 1, Start: [3]float32{0, 0, 0}, End: [3]float32{9, 0, 0}},
		},
		rng,
		3,
	)

	if ps.ActiveCount() != 2063 {
		t.Fatalf("ActiveCount = %d, want 2063", ps.ActiveCount())
	}
	a := ps.ActiveParticles()
	if a[0].Type != ParticleSlowGrav {
		t.Fatalf("first particle type = %d, want slowgrav", a[0].Type)
	}
	if a[12].Type != ParticleExplode2 || a[13].Type != ParticleExplode {
		t.Fatalf("explosion types = (%d,%d), want (explode2,explode)", a[12].Type, a[13].Type)
	}
	if a[len(a)-1].Type != ParticleStatic {
		t.Fatalf("last particle type = %d, want static beam particle", a[len(a)-1].Type)
	}
}
