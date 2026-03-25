package renderer

import (
	"github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/compatrand"
	"github.com/ironwail/ironwail-go/internal/cvar"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"math"
	"math/rand"
	"sync"
	"testing"
	"unsafe"
)

func TestParticleSystemCapacityAndAlloc(t *testing.T) {
	ps := NewParticleSystem(4)
	if ps.Capacity() != AbsoluteMinParticles {
		t.Fatalf("Capacity = %d, want %d", ps.Capacity(), AbsoluteMinParticles)
	}

	ps = NewParticleSystem(2)
	for i := 0; i < ps.Capacity(); i++ {
		if ps.AllocParticle(1.0) == nil {
			t.Fatalf("AllocParticle returned nil at %d", i)
		}
	}
	if ps.AllocParticle(1.0) != nil {
		t.Fatalf("AllocParticle should fail at capacity")
	}

	ps.Clear()
	if ps.ActiveCount() != 0 {
		t.Fatalf("ActiveCount after Clear = %d, want 0", ps.ActiveCount())
	}
}

func TestParticleTextureAndDrawMode(t *testing.T) {
	uv, scale := ParticleTexture(1)
	if uv != 1 || scale != 1.27 {
		t.Fatalf("ParticleTexture(1) = (%v,%v), want (1,1.27)", uv, scale)
	}

	uv, scale = ParticleTexture(2)
	if uv != 0.25 || scale != 1.0 {
		t.Fatalf("ParticleTexture(2) = (%v,%v), want (0.25,1.0)", uv, scale)
	}

	if !ShouldDrawParticles(1, true, false, 10) {
		t.Fatalf("mode 1 alpha pass expected true")
	}
	if ShouldDrawParticles(1, false, false, 10) {
		t.Fatalf("mode 1 opaque pass expected false")
	}
	if !ShouldDrawParticles(2, false, false, 10) {
		t.Fatalf("mode 2 opaque pass expected true")
	}
	if ShouldDrawParticles(2, true, false, 10) {
		t.Fatalf("mode 2 alpha pass expected false")
	}
}

func TestRunParticlesCompactsAndUpdates(t *testing.T) {
	ps := NewParticleSystem(512)
	p0 := ps.AllocParticle(0.0)
	p0.Die = 10
	p0.Spawn = -1
	p0.Type = ParticleFire
	p0.Ramp = 0
	p0.Vel = [3]float32{0, 0, 10}

	p1 := ps.AllocParticle(0.0)
	p1.Die = -1
	p1.Spawn = -1
	p1.Type = ParticleStatic

	p2 := ps.AllocParticle(0.0)
	p2.Die = 10
	p2.Spawn = 2
	p2.Type = ParticleStatic

	ps.RunParticles(1.0, 0.0, 800)
	if ps.ActiveCount() != 1 {
		t.Fatalf("ActiveCount = %d, want 1", ps.ActiveCount())
	}
	got := ps.ActiveParticles()[0]
	if got.Color != ramp3[5] {
		t.Fatalf("fire color = %d, want %d", got.Color, ramp3[5])
	}
	if got.Vel[2] <= 10 {
		t.Fatalf("fire vel.z = %f, want > 10", got.Vel[2])
	}
}

func TestRunParticleEffectRocketExplosion(t *testing.T) {
	ps := NewParticleSystem(2048)
	rng := rand.New(rand.NewSource(1))
	ps.RunParticleEffect([3]float32{1, 2, 3}, [3]float32{1, 0, 0}, 100, 1024, rng, 5)

	if ps.ActiveCount() != 1024 {
		t.Fatalf("ActiveCount = %d, want 1024", ps.ActiveCount())
	}
	a := ps.ActiveParticles()
	if a[0].Type != ParticleExplode2 || a[1].Type != ParticleExplode {
		t.Fatalf("types = (%d,%d), want (explode2,explode)", a[0].Type, a[1].Type)
	}
	if a[0].Die != 10 {
		t.Fatalf("die = %f, want 10", a[0].Die)
	}
}

func TestRocketTrailTracerAlternatesVelocity(t *testing.T) {
	ps := NewParticleSystem(1024)
	rng := rand.New(rand.NewSource(2))
	ps.RocketTrail([3]float32{0, 0, 0}, [3]float32{9, 0, 0}, 3, rng, 1)

	a := ps.ActiveParticles()
	if len(a) < 2 {
		t.Fatalf("need at least 2 tracer particles, got %d", len(a))
	}
	if a[0].Type != ParticleStatic || a[1].Type != ParticleStatic {
		t.Fatalf("tracer type mismatch: %d %d", a[0].Type, a[1].Type)
	}
	if a[0].Vel[1] == a[1].Vel[1] {
		t.Fatalf("expected alternating tracer side velocity, got %f and %f", a[0].Vel[1], a[1].Vel[1])
	}
}

func TestRocketTrailTracerAlternatesAcrossCalls(t *testing.T) {
	ps := NewParticleSystem(1024)
	rng := rand.New(rand.NewSource(11))

	ps.RocketTrail([3]float32{0, 0, 0}, [3]float32{3, 0, 0}, 3, rng, 1)
	first := ps.ActiveParticles()
	if len(first) != 1 {
		t.Fatalf("first call particles = %d, want 1", len(first))
	}

	ps.RocketTrail([3]float32{10, 0, 0}, [3]float32{13, 0, 0}, 3, rng, 1)
	all := ps.ActiveParticles()
	if len(all) != 2 {
		t.Fatalf("total particles = %d, want 2", len(all))
	}

	if all[0].Vel[1] == all[1].Vel[1] {
		t.Fatalf("expected alternating tracer side velocity across calls, got %f and %f", all[0].Vel[1], all[1].Vel[1])
	}
}

func TestBlobExplosionAddsBlobParticles(t *testing.T) {
	ps := NewParticleSystem(2048)
	rng := rand.New(rand.NewSource(3))
	ps.BlobExplosion([3]float32{1, 2, 3}, rng, 4)

	if ps.ActiveCount() != 1024 {
		t.Fatalf("ActiveCount = %d, want 1024", ps.ActiveCount())
	}
	a := ps.ActiveParticles()
	if a[0].Type != ParticleBlob2 || a[1].Type != ParticleBlob {
		t.Fatalf("types = (%d,%d), want (blob2,blob)", a[0].Type, a[1].Type)
	}
}

func TestParticleExplosion2UsesColorRange(t *testing.T) {
	ps := NewParticleSystem(1024)
	rng := rand.New(rand.NewSource(4))
	ps.ParticleExplosion2([3]float32{0, 0, 0}, 32, 5, rng, 2)

	if ps.ActiveCount() != 512 {
		t.Fatalf("ActiveCount = %d, want 512", ps.ActiveCount())
	}
	for _, p := range ps.ActiveParticles() {
		if p.Type != ParticleBlob {
			t.Fatalf("particle type = %d, want blob", p.Type)
		}
		if p.Color < 32 || p.Color >= 37 {
			t.Fatalf("particle color = %d, want in [32,37)", p.Color)
		}
	}
}

func TestEntityParticlesMatchQuakeCountAndStyle(t *testing.T) {
	ps := NewParticleSystem(2048)
	ps.EntityParticles([3]float32{10, 20, 30}, 1)

	if got := ps.ActiveCount(); got != len(entityParticleNormals) {
		t.Fatalf("ActiveCount = %d, want %d", got, len(entityParticleNormals))
	}
	for _, particle := range ps.ActiveParticles() {
		if particle.Type != ParticleExplode {
			t.Fatalf("particle type = %d, want explode", particle.Type)
		}
		if particle.Color != 0x6f {
			t.Fatalf("particle color = %d, want 0x6f", particle.Color)
		}
		if particle.Die != 1.01 {
			t.Fatalf("particle die = %v, want 1.01", particle.Die)
		}
	}
	first := ps.ActiveParticles()[0]
	want := [3]float32{-7.647106, 20.041887, 84.34951}
	for i := range want {
		if math.Abs(float64(first.Org[i]-want[i])) > 0.0001 {
			t.Fatalf("first particle org[%d] = %v, want %v", i, first.Org[i], want[i])
		}
	}
}

func TestSplashEffectsAddExpectedCounts(t *testing.T) {
	ps := NewParticleSystem(4096)
	rng := rand.New(rand.NewSource(5))
	ps.LavaSplash([3]float32{0, 0, 0}, rng, 1)
	if ps.ActiveCount() != 1024 {
		t.Fatalf("LavaSplash count = %d, want 1024", ps.ActiveCount())
	}
	ps.Clear()
	ps.TeleportSplash([3]float32{0, 0, 0}, rng, 1)
	if ps.ActiveCount() != 896 {
		t.Fatalf("TeleportSplash count = %d, want 896", ps.ActiveCount())
	}
}

func TestRunParticleEffectNilRNGUsesCompatRandStream(t *testing.T) {
	t.Cleanup(func() { compatrand.ResetShared(1) })

	capture := func() []Particle {
		ps := NewParticleSystem(64)
		ps.RunParticleEffect([3]float32{1, 2, 3}, [3]float32{0, 1, 0}, 32, 16, nil, 0)
		return ps.ActiveParticles()
	}

	compatrand.ResetShared(1)
	base := capture()

	compatrand.ResetShared(1)
	_ = compatrand.Int() // advance global stream; nil RNG path should observe this
	shifted := capture()

	if len(base) == 0 || len(shifted) == 0 {
		t.Fatalf("expected generated particles for nil RNG path")
	}
	if base[0].Org == shifted[0].Org && base[0].Color == shifted[0].Color {
		t.Fatalf("nil RNG particle output did not change after compatrand stream advance")
	}
}

func TestEntityParticlesUsesCompatRandForAngularVelocitySeed(t *testing.T) {
	t.Cleanup(func() { compatrand.ResetShared(1) })

	run := func() [3]float32 {
		entityParticleAngularVelOnce = sync.Once{}
		entityParticleAngularVelocities = [len(entityParticleNormals)][3]float32{}

		ps := NewParticleSystem(2048)
		ps.EntityParticles([3]float32{10, 20, 30}, 1)
		a := ps.ActiveParticles()
		if len(a) == 0 {
			t.Fatalf("expected entity particles")
		}
		return a[0].Org
	}

	compatrand.ResetShared(1)
	base := run()

	compatrand.ResetShared(1)
	_ = compatrand.Int() // alter shared stream before velocity table init
	shifted := run()

	if base == shifted {
		t.Fatalf("entity particle origin unchanged after compatrand stream advance")
	}
}

func TestBuildParticleVertices(t *testing.T) {
	palette := [256][4]byte{}
	palette[3] = [4]byte{10, 20, 30, 40}

	verts := BuildParticleVertices([]Particle{{Org: [3]float32{1, 2, 3}, Color: 3}}, palette, false)
	if len(verts) != 1 {
		t.Fatalf("len = %d, want 1", len(verts))
	}
	if verts[0].Color != [4]byte{10, 20, 30, 40} {
		t.Fatalf("color = %v, want [10 20 30 40]", verts[0].Color)
	}

	verts = BuildParticleVertices([]Particle{{Org: [3]float32{1, 2, 3}, Color: 3}}, palette, true)
	if verts[0].Color != [4]byte{255, 255, 255, 255} {
		t.Fatalf("showtris color = %v, want white", verts[0].Color)
	}
}

func TestParticleVertexPtr(t *testing.T) {
	if ptr := particleVertexPtr(nil); ptr != nil {
		t.Fatalf("particleVertexPtr(nil) = %v, want nil", ptr)
	}

	verts := []ParticleVertex{{Pos: [3]float32{1, 2, 3}, Color: [4]byte{4, 5, 6, 7}}}
	if ptr := particleVertexPtr(verts); ptr != unsafe.Pointer(&verts[0]) {
		t.Fatalf("particleVertexPtr returned %v, want %v", ptr, unsafe.Pointer(&verts[0]))
	}
}

func TestParticleVertexLayout(t *testing.T) {
	if got := unsafe.Sizeof(ParticleVertex{}); got != 16 {
		t.Fatalf("unsafe.Sizeof(ParticleVertex{}) = %d, want 16", got)
	}
	if got := unsafe.Offsetof(ParticleVertex{}.Color); got != 12 {
		t.Fatalf("unsafe.Offsetof(ParticleVertex{}.Color) = %d, want 12", got)
	}
}

func TestEmitDynamicLightsHonorsRDynamicGate(t *testing.T) {
	if cvar.Get(CvarRDynamic) == nil {
		cvar.Register(CvarRDynamic, "1", cvar.FlagArchive, "")
	}
	cvar.Set(CvarRDynamic, "0")
	t.Cleanup(func() {
		cvar.Set(CvarRDynamic, "1")
	})

	var spawned int
	EmitDynamicLights(func(DynamicLight) bool {
		spawned++
		return true
	}, []client.TempEntityEvent{{Type: inet.TE_EXPLOSION, Origin: [3]float32{1, 2, 3}}})

	if spawned != 0 {
		t.Fatalf("spawned lights = %d, want 0 when r_dynamic=0", spawned)
	}
}

func TestEvaluateDynamicLightsAtPointHonorsRDynamicGate(t *testing.T) {
	if cvar.Get(CvarRDynamic) == nil {
		cvar.Register(CvarRDynamic, "1", cvar.FlagArchive, "")
	}
	lights := []DynamicLight{{
		Position:   [3]float32{0, 0, 0},
		Radius:     100,
		Color:      [3]float32{1, 1, 1},
		Brightness: 1,
		Lifetime:   1,
	}}

	cvar.Set(CvarRDynamic, "1")
	on := evaluateDynamicLightsAtPoint(lights, [3]float32{0, 0, 0})
	if on == [3]float32{} {
		t.Fatalf("expected non-zero contribution when r_dynamic=1")
	}

	cvar.Set(CvarRDynamic, "0")
	t.Cleanup(func() {
		cvar.Set(CvarRDynamic, "1")
	})
	off := evaluateDynamicLightsAtPoint(lights, [3]float32{0, 0, 0})
	if off != ([3]float32{}) {
		t.Fatalf("contribution when r_dynamic=0 = %v, want zero", off)
	}
}
