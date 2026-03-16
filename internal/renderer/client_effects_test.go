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

func TestEmitEntityEffectLightsMuzzleFlashUsesForwardOffset(t *testing.T) {
	var lights []DynamicLight
	EmitEntityEffectLights(func(light DynamicLight) bool {
		lights = append(lights, light)
		return true
	}, []EntityEffectSource{{
		Origin:  [3]float32{1, 2, 3},
		Angles:  [3]float32{0, 90, 0},
		Effects: inet.EF_MUZZLEFLASH,
	}})

	if got := len(lights); got != 1 {
		t.Fatalf("light count = %d, want 1", got)
	}
	if lights[0].Position != [3]float32{1, 20, 19} {
		t.Fatalf("muzzle flash position = %#v, want [1 20 19]", lights[0].Position)
	}
	if lights[0].Lifetime != 0.1 || lights[0].Radius != 216 {
		t.Fatalf("muzzle flash light = %#v, want lifetime 0.1 and radius 216", lights[0])
	}
}

func TestEmitEntityEffectLightsBrightAndDimShareLiftedOrigin(t *testing.T) {
	var lights []DynamicLight
	EmitEntityEffectLights(func(light DynamicLight) bool {
		lights = append(lights, light)
		return true
	}, []EntityEffectSource{{
		Origin:  [3]float32{4, 5, 6},
		Effects: inet.EF_BRIGHTLIGHT | inet.EF_DIMLIGHT | inet.EF_BRIGHTFIELD,
	}})

	if got := len(lights); got != 2 {
		t.Fatalf("light count = %d, want 2", got)
	}
	for i, wantRadius := range []float32{416, 216} {
		if lights[i].Radius != wantRadius {
			t.Fatalf("light %d radius = %v, want %v", i, lights[i].Radius, wantRadius)
		}
		if lights[i].Lifetime != 0.001 {
			t.Fatalf("light %d lifetime = %v, want 0.001", i, lights[i].Lifetime)
		}
	}
	if lights[0].Position != [3]float32{4, 5, 22} {
		t.Fatalf("bright light position = %#v, want [4 5 22]", lights[0].Position)
	}
	if lights[1].Position != [3]float32{4, 5, 6} {
		t.Fatalf("dim light position = %#v, want [4 5 6]", lights[1].Position)
	}
}

func TestEmitEntityEffectParticlesBrightField(t *testing.T) {
	ps := NewParticleSystem(2048)
	EmitEntityEffectParticles(ps, []EntityEffectSource{
		{Origin: [3]float32{10, 20, 30}, Effects: inet.EF_BRIGHTFIELD},
		{Origin: [3]float32{1, 2, 3}, Effects: inet.EF_DIMLIGHT},
	}, 2)

	if got := ps.ActiveCount(); got != len(entityParticleNormals) {
		t.Fatalf("ActiveCount = %d, want %d", got, len(entityParticleNormals))
	}
	first := ps.ActiveParticles()[0]
	if first.Type != ParticleExplode || first.Color != 0x6f || first.Die != 2.01 {
		t.Fatalf("first brightfield particle = %#v, want explode type, 0x6f color, and 2.01 die", first)
	}
}

func TestEmitEntityEffectLightsQuadAndPentaColors(t *testing.T) {
	var lights []DynamicLight
	EmitEntityEffectLights(func(light DynamicLight) bool {
		lights = append(lights, light)
		return true
	}, []EntityEffectSource{{
		Origin:  [3]float32{8, 9, 10},
		Effects: inet.EF_QUADLIGHT | inet.EF_PENTALIGHT,
	}})

	if got := len(lights); got != 2 {
		t.Fatalf("light count = %d, want 2", got)
	}
	if lights[0].Position != [3]float32{8, 9, 26} || lights[0].Color != [3]float32{0.25, 0.25, 1.0} {
		t.Fatalf("quad light = %#v, want lifted blue light", lights[0])
	}
	if lights[1].Position != [3]float32{8, 9, 26} || lights[1].Color != [3]float32{1.0, 0.25, 0.25} {
		t.Fatalf("penta light = %#v, want lifted red light", lights[1])
	}
}

func TestEmitEntityEffectLightsCandlelight(t *testing.T) {
	var lights []DynamicLight
	EmitEntityEffectLights(func(light DynamicLight) bool {
		lights = append(lights, light)
		return true
	}, []EntityEffectSource{{
		Origin:  [3]float32{1, 2, 3},
		Effects: inet.EF_CANDLELIGHT,
	}})

	if got := len(lights); got != 1 {
		t.Fatalf("light count = %d, want 1", got)
	}
	if lights[0].Position != [3]float32{1, 2, 3} || lights[0].Color != [3]float32{1.0, 0.75, 0.2} {
		t.Fatalf("candle light = %#v, want unlifted yellow light", lights[0])
	}
}

func TestEmitDecalMarksMapsImpactAndExplosion(t *testing.T) {
	ms := NewDecalMarkSystem()
	rng := rand.New(rand.NewSource(9))

	EmitDecalMarks(ms,
		[]cl.TempEntityEvent{
			{Type: inet.TE_GUNSHOT, Origin: [3]float32{1, 2, 3}},
			{Type: inet.TE_EXPLOSION, Origin: [3]float32{4, 5, 6}},
			{Type: inet.TE_BEAM, Start: [3]float32{0, 0, 0}, End: [3]float32{10, 0, 0}},
		},
		rng,
		2,
	)

	if got := ms.ActiveCount(); got != 2 {
		t.Fatalf("ActiveCount = %d, want 2", got)
	}
	marks := ms.ActiveMarks()
	if marks[0].Size != 8 {
		t.Fatalf("impact mark size = %v, want 8", marks[0].Size)
	}
	if marks[0].Variant != DecalVariantBullet {
		t.Fatalf("impact mark variant = %v, want %v", marks[0].Variant, DecalVariantBullet)
	}
	if marks[1].Size != 24 {
		t.Fatalf("explosion mark size = %v, want 24", marks[1].Size)
	}
	if marks[1].Variant != DecalVariantScorch {
		t.Fatalf("explosion mark variant = %v, want %v", marks[1].Variant, DecalVariantScorch)
	}
}

func TestDecalMarkSystemRunExpiresMarks(t *testing.T) {
	ms := NewDecalMarkSystem()
	ms.AddMark(DecalMarkEntity{Origin: [3]float32{0, 0, 0}, Normal: [3]float32{0, 0, 1}, Size: 8, Alpha: 1}, 1.0, 5.0)
	ms.AddMark(DecalMarkEntity{Origin: [3]float32{1, 0, 0}, Normal: [3]float32{0, 0, 1}, Size: 8, Alpha: 1}, 5.0, 5.0)

	if got := ms.ActiveCount(); got != 2 {
		t.Fatalf("ActiveCount before run = %d, want 2", got)
	}

	ms.Run(6.1)
	if got := ms.ActiveCount(); got != 1 {
		t.Fatalf("ActiveCount after run = %d, want 1", got)
	}
}

func TestEmitDynamicLightsMapsExplosionAndBeam(t *testing.T) {
	var lights []DynamicLight
	EmitDynamicLights(func(light DynamicLight) bool {
		lights = append(lights, light)
		return true
	}, []cl.TempEntityEvent{
		{Type: inet.TE_EXPLOSION, Origin: [3]float32{4, 5, 6}},
		{Type: inet.TE_BEAM, Start: [3]float32{0, 0, 0}, End: [3]float32{10, 20, 30}},
		{Type: inet.TE_LAVASPLASH, Origin: [3]float32{9, 9, 9}},
	})

	if got := len(lights); got != 2 {
		t.Fatalf("light count = %d, want 2", got)
	}
	if lights[0].Position != [3]float32{4, 5, 6} || lights[0].Radius != 320 {
		t.Fatalf("explosion light = %#v, want origin light with radius 320", lights[0])
	}
	if lights[1].Position != [3]float32{5, 10, 15} || lights[1].Radius != 160 {
		t.Fatalf("beam light = %#v, want midpoint light with radius 160", lights[1])
	}
}
