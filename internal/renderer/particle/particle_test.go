package particle

import (
	"math"
	"math/rand"
	"sync"
	"testing"
	"unsafe"

	"github.com/darkliquid/ironwail-go/internal/compatrand"
	"github.com/darkliquid/ironwail-go/pkg/types"
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
	p0.Vel = types.Vec3{X: 0, Y: 0, Z: 10}

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
	if got.Vel.Z <= 10 {
		t.Fatalf("fire vel.z = %f, want > 10", got.Vel.Z)
	}
}

func TestRunParticleEffectRocketExplosion(t *testing.T) {
	ps := NewParticleSystem(2048)
	rng := rand.New(rand.NewSource(1))
	ps.RunParticleEffect(types.Vec3{X: 1, Y: 2, Z: 3}, types.Vec3{X: 1, Y: 0, Z: 0}, 100, 1024, rng, 5)

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
	ps.RocketTrail(types.Vec3{X: 0, Y: 0, Z: 0}, types.Vec3{X: 9, Y: 0, Z: 0}, 3, rng, 1)

	a := ps.ActiveParticles()
	if len(a) < 2 {
		t.Fatalf("need at least 2 tracer particles, got %d", len(a))
	}
	if a[0].Type != ParticleStatic || a[1].Type != ParticleStatic {
		t.Fatalf("tracer type mismatch: %d %d", a[0].Type, a[1].Type)
	}
	if a[0].Vel.Y == a[1].Vel.Y {
		t.Fatalf("expected alternating tracer side velocity, got %f and %f", a[0].Vel.Y, a[1].Vel.Y)
	}
}

func TestBlobExplosionAddsBlobParticles(t *testing.T) {
	ps := NewParticleSystem(2048)
	rng := rand.New(rand.NewSource(3))
	ps.BlobExplosion(types.Vec3{X: 1, Y: 2, Z: 3}, rng, 4)

	if ps.ActiveCount() != 1024 {
		t.Fatalf("ActiveCount = %d, want 1024", ps.ActiveCount())
	}
	a := ps.ActiveParticles()
	if a[0].Type != ParticleBlob2 || a[1].Type != ParticleBlob {
		t.Fatalf("types = (%d,%d), want (blob2,blob)", a[0].Type, a[1].Type)
	}
}

func TestEntityParticlesMatchQuakeCountAndStyle(t *testing.T) {
	t.Cleanup(func() { compatrand.ResetShared(1) })
	entityParticleAngularVelOnce = sync.Once{}
	entityParticleAngularVelocities = [len(entityParticleNormals)]types.Vec3{}
	compatrand.ResetShared(1)

	ps := NewParticleSystem(2048)
	ps.EntityParticles(types.Vec3{X: 10, Y: 20, Z: 30}, 1)

	if got := ps.ActiveCount(); got != len(entityParticleNormals) {
		t.Fatalf("ActiveCount = %d, want %d", got, len(entityParticleNormals))
	}
	for _, p := range ps.ActiveParticles() {
		if p.Type != ParticleExplode {
			t.Fatalf("particle type = %d, want explode", p.Type)
		}
		if p.Color != 0x6f {
			t.Fatalf("particle color = %d, want 0x6f", p.Color)
		}
		if p.Die != 1.01 {
			t.Fatalf("particle die = %v, want 1.01", p.Die)
		}
	}
	first := ps.ActiveParticles()[0]
	want := types.Vec3{X: -26.924153, Y: 27.55703, Z: 70.72488}
	if math.Abs(float64(first.Org.X-want.X)) > 0.0001 ||
		math.Abs(float64(first.Org.Y-want.Y)) > 0.0001 ||
		math.Abs(float64(first.Org.Z-want.Z)) > 0.0001 {
		t.Fatalf("first particle org = %v, want %v", first.Org, want)
	}
}

func TestEntityParticlesUsesCompatRandForAngularVelocitySeed(t *testing.T) {
	t.Cleanup(func() { compatrand.ResetShared(1) })

	run := func() types.Vec3 {
		entityParticleAngularVelOnce = sync.Once{}
		entityParticleAngularVelocities = [len(entityParticleNormals)]types.Vec3{}

		ps := NewParticleSystem(2048)
		ps.EntityParticles(types.Vec3{X: 10, Y: 20, Z: 30}, 1)
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

func TestSplashEffectsAddExpectedCounts(t *testing.T) {
	ps := NewParticleSystem(4096)
	rng := rand.New(rand.NewSource(5))
	ps.LavaSplash(types.Vec3{X: 0, Y: 0, Z: 0}, rng, 1)
	if ps.ActiveCount() != 1024 {
		t.Fatalf("LavaSplash count = %d, want 1024", ps.ActiveCount())
	}
	ps.Clear()
	ps.TeleportSplash(types.Vec3{X: 0, Y: 0, Z: 0}, rng, 1)
	if ps.ActiveCount() != 896 {
		t.Fatalf("TeleportSplash count = %d, want 896", ps.ActiveCount())
	}
}

func TestBuildParticleVertices(t *testing.T) {
	palette := [256][4]byte{}
	palette[3] = [4]byte{10, 20, 30, 40}

	verts := BuildParticleVertices([]Particle{{Org: types.Vec3{X: 1, Y: 2, Z: 3}, Color: 3}}, palette, false)
	if len(verts) != 1 {
		t.Fatalf("len = %d, want 1", len(verts))
	}
	if verts[0].Color != [4]byte{10, 20, 30, 40} {
		t.Fatalf("color = %v, want [10 20 30 40]", verts[0].Color)
	}

	verts = BuildParticleVertices([]Particle{{Org: types.Vec3{X: 1, Y: 2, Z: 3}, Color: 3}}, palette, true)
	if verts[0].Color != [4]byte{255, 255, 255, 255} {
		t.Fatalf("showtris color = %v, want white", verts[0].Color)
	}
}

func TestParticleVertexPtr(t *testing.T) {
	if ptr := ParticleVertexPtr(nil); ptr != nil {
		t.Fatalf("ParticleVertexPtr(nil) = %v, want nil", ptr)
	}

	verts := []ParticleVertex{{Pos: types.Vec3{X: 1, Y: 2, Z: 3}, Color: [4]byte{4, 5, 6, 7}}}
	if ptr := ParticleVertexPtr(verts); ptr != unsafe.Pointer(&verts[0]) {
		t.Fatalf("ParticleVertexPtr returned %v, want %v", ptr, unsafe.Pointer(&verts[0]))
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
